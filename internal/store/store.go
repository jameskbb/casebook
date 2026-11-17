// Package store owns Casebook's SQLite storage: connection setup, embedded
// forward-only migrations, transactions with busy retry, and the SQL for every
// table. Only internal/app may import it; surfaces never do.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/jameskbb/casebook/internal/paths"

	_ "modernc.org/sqlite" // registers the "sqlite" database/sql driver
)

// Defaults for Config zero values.
const (
	DefaultBusyTimeout = 5 * time.Second
	DefaultRetryBudget = 10 * time.Second
	readerPoolSize     = 8
)

// Config tunes how a DB is opened.
type Config struct {
	// BusyTimeout is SQLite's busy_timeout for every connection.
	BusyTimeout time.Duration
	// RetryBudget bounds the total time WithWriteTx spends retrying a write
	// that keeps failing with SQLITE_BUSY before it returns domain.ErrBusy.
	RetryBudget time.Duration
	// Logger receives Debug and Warn records; nil discards.
	Logger *slog.Logger
	// Now is the clock used for migration timestamps; nil means time.Now.
	Now func() time.Time
	// BeforeUpgrade, when set, runs once during Open before pending
	// migrations are applied to an existing database: one whose schema
	// version is above 0 and below LatestVersion. from is that version. A
	// fresh or up-to-date database never calls it. An error aborts Open
	// before any migration runs, so the file keeps its old schema. Two
	// processes opening the same old file at once may both call it.
	BeforeUpgrade func(ctx context.Context, db *DB, from int) error
}

// Querier is the subset of *sql.DB, *sql.Tx and *sql.Conn the query helpers
// need, so the same SQL runs inside write and read transactions.
type Querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// DB is an open Casebook database: a single-connection BEGIN IMMEDIATE writer
// pool and a small deferred reader pool on the same file.
type DB struct {
	path   string
	cfg    Config
	log    *slog.Logger
	writer *sql.DB
	reader *sql.DB
}

// Open opens (creating when missing) the database at path, applies pragmas
// and embedded migrations, and tightens file permissions.
func Open(ctx context.Context, path string, cfg Config) (*DB, error) {
	if cfg.BusyTimeout <= 0 {
		cfg.BusyTimeout = DefaultBusyTimeout
	}
	if cfg.RetryBudget <= 0 {
		cfg.RetryBudget = DefaultRetryBudget
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.New(slog.DiscardHandler)
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve database path: %w", err)
	}
	if err := paths.EnsureDir(filepath.Dir(abs)); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}
	// Pre-create the file owner-only so SQLite (and its -wal/-shm files,
	// which inherit the main file's mode) never exist with looser permissions.
	f, err := os.OpenFile(abs, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, fmt.Errorf("create database file: %w", err)
	}
	if err := f.Close(); err != nil {
		return nil, fmt.Errorf("create database file: %w", err)
	}

	base := buildDSN(abs, cfg.BusyTimeout)
	writer, err := sql.Open("sqlite", base+"&_txlock=immediate")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	writer.SetMaxOpenConns(1)
	writer.SetMaxIdleConns(1)
	writer.SetConnMaxLifetime(0)
	writer.SetConnMaxIdleTime(0)

	reader, err := sql.Open("sqlite", base)
	if err != nil {
		writer.Close()
		return nil, fmt.Errorf("open database: %w", err)
	}
	reader.SetMaxOpenConns(readerPoolSize)
	reader.SetMaxIdleConns(readerPoolSize)

	db := &DB{path: abs, cfg: cfg, log: cfg.Logger, writer: writer, reader: reader}
	if _, err := db.migrate(ctx); err != nil {
		db.Close()
		return nil, err
	}
	if err := db.tightenPermissions(); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// buildDSN renders path as a file: URI. The path goes through url.URL so
// spaces, '#', '?' and '%' are escaped and cannot inject query parameters; the
// pragma list is a compile-time constant apart from the integer timeout.
func buildDSN(abs string, busy time.Duration) string {
	return fileURI(abs) + fmt.Sprintf(
		"?_pragma=busy_timeout(%d)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=synchronous(NORMAL)",
		busy.Milliseconds())
}

// fileURI renders an absolute path as a file: URI with no query. The path
// goes through url.URL so it cannot inject query parameters.
func fileURI(abs string) string {
	p := filepath.ToSlash(abs)
	if !strings.HasPrefix(p, "/") {
		// Windows drive path C:/x becomes /C:/x in a file URI.
		p = "/" + p
	}
	u := url.URL{Scheme: "file", Path: p}
	return u.String()
}

// migrate applies the embedded migrations through the writer pool, retrying
// on lock contention (another process may be opening or writing the same
// file). Migrate is safe to re-run.
func (db *DB) migrate(ctx context.Context) (int, error) {
	sub, err := fs.Sub(Migrations, "migrations")
	if err != nil {
		return 0, err
	}
	if db.cfg.BeforeUpgrade != nil {
		// A newer database is refused by Migrate below, untouched. The read
		// retries like the migration: a concurrent first open may hold the
		// lock while it switches the file to WAL.
		var from int
		err := db.retryBusy(ctx, "migrate", func() error {
			var err error
			from, err = currentVersion(ctx, db.reader)
			return err
		})
		if err != nil {
			return 0, err
		}
		if from > 0 && from < LatestVersion() {
			if err := db.cfg.BeforeUpgrade(ctx, db, from); err != nil {
				return from, err
			}
		}
	}
	var v int
	err = db.retryBusy(ctx, "migrate", func() error {
		var err error
		v, err = db.Migrate(ctx, sub)
		return err
	})
	return v, err
}

// tightenPermissions makes the DB, -wal and -shm files 0600 on non-Windows.
func (db *DB) tightenPermissions() error {
	if runtime.GOOS == "windows" {
		return nil
	}
	for _, p := range []string{db.path, db.path + "-wal", db.path + "-shm"} {
		if err := os.Chmod(p, 0o600); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("set database permissions: %w", err)
		}
	}
	return nil
}

// Path returns the absolute database file path.
func (db *DB) Path() string { return db.path }

// Logger returns the configured logger.
func (db *DB) Logger() *slog.Logger { return db.log }

// Close closes both pools.
func (db *DB) Close() error {
	werr := db.writer.Close()
	rerr := db.reader.Close()
	return errors.Join(werr, rerr)
}
