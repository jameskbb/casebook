// Package app is Casebook's application API: the only package every surface
// (desktop, CLI, MCP, hooks) calls. It re-exports the DTOs, enums and errors
// surfaces need (see types.go), so surfaces import only internal/app and, to
// locate the database, internal/paths.
package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/jameskbb/casebook/internal/paths"
	"github.com/jameskbb/casebook/internal/store"
)

// App is an open Casebook engine bound to one database file. It is safe for
// concurrent use; several Apps (in one or many processes) may share a file.
type App struct {
	db  *store.DB
	log *slog.Logger
	now func() time.Time

	// workDir is the cleaned WithWorkDir directory ("" when none) and
	// workDirs it and its ancestors, nearest first
	// (ADR-0015).
	workDir  string
	workDirs []string

	closeOnce sync.Once
	closeErr  error

	// tokenMu guards tokenConn and closed. tokenConn is the dedicated
	// long-lived reader connection ChangeToken reads PRAGMA data_version on;
	// it is acquired lazily and released by Close.
	tokenMu   sync.Mutex
	tokenConn *sql.Conn
	closed    bool
}

type options struct {
	logger      *slog.Logger
	clock       func() time.Time
	busyTimeout time.Duration
	retryBudget time.Duration
	workDir     string
}

// Option configures Open.
type Option func(*options)

// WithLogger sets the structured logger. The default discards everything;
// Casebook never writes logs to stdout.
func WithLogger(l *slog.Logger) Option {
	return func(o *options) {
		if l != nil {
			o.logger = l
		}
	}
}

// WithClock sets the clock used for every timestamp (tests and goldens).
func WithClock(now func() time.Time) Option {
	return func(o *options) {
		if now != nil {
			o.clock = now
		}
	}
}

// WithBusyTimeout sets SQLite's busy_timeout per connection (default 5s).
func WithBusyTimeout(d time.Duration) Option {
	return func(o *options) { o.busyTimeout = d }
}

// WithRetryBudget sets the total time a write keeps retrying on lock
// contention before returning ErrBusy (default 10s). The budget is checked
// between attempts, and each attempt can itself wait inside SQLite for up to
// the busy timeout, so the worst-case wait is about the budget plus one busy
// timeout (15s with the defaults).
func WithRetryBudget(d time.Duration) Option {
	return func(o *options) { o.retryBudget = d }
}

// Open opens the database at dbPath, creating its parent directory (0700) and
// the file (0600) when missing, and applies pragmas and migrations. Before it
// migrates an existing database (schema version above 0) that has pending
// migrations, it writes a pre-upgrade backup to
// <dir of dbPath>/backups/casebook-pre-upgrade-schema<old>-<UTC timestamp>.db;
// when that copy cannot be written Open fails and leaves the file at its old
// schema. A fresh or up-to-date database takes no copy. A database written by
// a newer Casebook is refused with ErrSchemaTooNew and left untouched.
func Open(dbPath string, opts ...Option) (*App, error) {
	o := options{
		logger: slog.New(slog.DiscardHandler),
		clock:  time.Now,
	}
	for _, opt := range opts {
		opt(&o)
	}
	db, err := store.Open(context.Background(), dbPath, store.Config{
		BusyTimeout:   o.busyTimeout,
		RetryBudget:   o.retryBudget,
		Logger:        o.logger,
		Now:           o.clock,
		BeforeUpgrade: preUpgradeBackup(o.logger, o.clock),
	})
	if err != nil {
		return nil, openError(o.logger, dbPath, err)
	}
	out := &App{db: db, log: o.logger, now: o.clock}
	if strings.TrimSpace(o.workDir) != "" {
		if d, err := cleanDir(o.workDir); err == nil {
			out.workDir = d
			out.workDirs = lookupDirs(d)
		}
	}
	return out, nil
}

// openError maps an Open failure: lock contention is ErrBusy, a newer schema
// stays ErrSchemaTooNew, any other SQLite failure becomes ErrInternal naming
// the path (the raw text is logged at Warn), and filesystem errors such as
// permission denied pass through so the user can act on them.
func openError(log *slog.Logger, dbPath string, err error) error {
	switch {
	case errors.Is(err, ErrSchemaTooNew), errors.Is(err, ErrBusy):
		return err
	case store.IsBusy(err):
		return ErrBusy
	case store.IsSQLite(err):
		log.Warn("app.open", "path", dbPath, "err", err)
		return fmt.Errorf("%w: cannot open %s as a Casebook database", ErrInternal, dbPath)
	}
	return err
}

// OpenDefault opens the database at paths.DBPath() (CASEBOOK_DB, else
// CASEBOOK_HOME/casebook.db, else the per-OS data directory).
func OpenDefault(opts ...Option) (*App, error) {
	p, err := paths.DBPath()
	if err != nil {
		return nil, err
	}
	return Open(p, opts...)
}

// Close releases the database, including the change token connection. It is
// safe to call more than once.
func (a *App) Close() error {
	a.closeOnce.Do(func() {
		a.tokenMu.Lock()
		a.closed = true
		var cerr error
		if a.tokenConn != nil {
			cerr = a.tokenConn.Close()
			a.tokenConn = nil
		}
		a.tokenMu.Unlock()
		a.closeErr = errors.Join(cerr, a.db.Close())
	})
	return a.closeErr
}

// DBPath returns the absolute path of the open database file.
func (a *App) DBPath() string { return a.db.Path() }

// timestamp returns the current time in TimeLayout.
func (a *App) timestamp() string { return FormatTime(a.now()) }
