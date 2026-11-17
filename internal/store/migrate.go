package store

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"strconv"
	"time"

	"github.com/jameskbb/casebook/internal/domain"
)

// Migrations holds the embedded forward-only migration files
// migrations/NNNN_name.sql.
//
//go:embed migrations/*.sql
var Migrations embed.FS

var migrationName = regexp.MustCompile(`^(\d{4})_[a-z0-9_]+\.sql$`)

// goSteps holds the data migrations that SQL cannot express, keyed by the
// migration file they belong to. Each runs inside that file's transaction,
// after its SQL and before its version is recorded, so it is applied exactly
// once and atomically with the version bump. A step only rewrites data the
// file's version already holds, except widenSourceTypeCheck, which loosens
// one CHECK list in place (see migrate_0006.go).
var goSteps = map[string]func(ctx context.Context, tx *sql.Tx) error{
	"0004_resanitize_text.sql": resanitizeText,
	"0006_evernote_links.sql":  widenSourceTypeCheck,
}

type migration struct {
	version int
	name    string
}

// LatestVersion is the newest schema version embedded in this build.
func LatestVersion() int {
	sub, err := fs.Sub(Migrations, "migrations")
	if err != nil {
		panic(err)
	}
	ms, err := listMigrations(sub)
	if err != nil {
		panic(err)
	}
	return ms[len(ms)-1].version
}

// listMigrations returns the migrations in fsys (root holds NNNN_name.sql
// files), requiring contiguous numbering from 1.
func listMigrations(fsys fs.FS) ([]migration, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("read migrations: %w", err)
	}
	var ms []migration
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		m := migrationName.FindStringSubmatch(e.Name())
		if m == nil {
			return nil, fmt.Errorf("migration file %q is not named NNNN_name.sql", e.Name())
		}
		v, _ := strconv.Atoi(m[1])
		ms = append(ms, migration{version: v, name: e.Name()})
	}
	sort.Slice(ms, func(i, j int) bool { return ms[i].version < ms[j].version })
	if len(ms) == 0 {
		return nil, fmt.Errorf("no migrations found")
	}
	for i, m := range ms {
		if m.version != i+1 {
			return nil, fmt.Errorf("migrations must be numbered contiguously from 0001: found %s at position %d", m.name, i+1)
		}
	}
	return ms, nil
}

// Migrate applies the pending migrations in fsys through the writer pool and
// returns the resulting schema version. See the package-level Migrate.
func (db *DB) Migrate(ctx context.Context, fsys fs.FS) (int, error) {
	return Migrate(ctx, db.writer, fsys, db.cfg.Now)
}

// Migrate applies every pending migration in fsys (root holds NNNN_name.sql
// files) to the database behind w, each in its own write transaction that
// re-reads the current version first, so concurrent first opens never apply a
// version twice. It returns the resulting schema version. A database whose
// version is newer than the newest file in fsys is refused with
// domain.ErrSchemaTooNew and left untouched.
func Migrate(ctx context.Context, w *sql.DB, fsys fs.FS, now func() time.Time) (int, error) {
	if now == nil {
		now = time.Now
	}
	ms, err := listMigrations(fsys)
	if err != nil {
		return 0, err
	}
	latest := ms[len(ms)-1].version

	// Check the version before writing anything, so a newer database is
	// refused without even creating schema_migrations.
	have, err := currentVersion(ctx, w)
	if err != nil {
		return 0, err
	}
	if have > latest {
		return have, tooNew(have, latest)
	}
	if _, err := w.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
    version    INTEGER PRIMARY KEY,
    applied_at TEXT NOT NULL
)`); err != nil {
		return 0, fmt.Errorf("create schema_migrations: %w", err)
	}

	for _, m := range ms {
		body, err := fs.ReadFile(fsys, m.name)
		if err != nil {
			return 0, fmt.Errorf("read migration %s: %w", m.name, err)
		}
		tx, err := w.BeginTx(ctx, nil)
		if err != nil {
			return 0, fmt.Errorf("apply migration %s: %w", m.name, err)
		}
		var cur int
		if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&cur); err != nil {
			_ = rollback(tx)
			return 0, fmt.Errorf("apply migration %s: %w", m.name, err)
		}
		if cur > latest {
			_ = rollback(tx)
			return cur, tooNew(cur, latest)
		}
		if cur >= m.version {
			// Already applied (possibly by a concurrent opener).
			if err := rollback(tx); err != nil {
				return 0, err
			}
			continue
		}
		if _, err := tx.ExecContext(ctx, string(body)); err != nil {
			_ = rollback(tx)
			return 0, fmt.Errorf("apply migration %s: %w", m.name, err)
		}
		if step := goSteps[m.name]; step != nil {
			if err := step(ctx, tx); err != nil {
				_ = rollback(tx)
				return 0, fmt.Errorf("apply migration %s: %w", m.name, err)
			}
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`,
			m.version, domain.FormatTime(now())); err != nil {
			_ = rollback(tx)
			return 0, fmt.Errorf("apply migration %s: %w", m.name, err)
		}
		if err := tx.Commit(); err != nil {
			return 0, fmt.Errorf("apply migration %s: %w", m.name, err)
		}
	}
	return currentVersion(ctx, w)
}

func tooNew(have, latest int) error {
	return fmt.Errorf("%w (database schema %d, this Casebook supports up to %d); "+
		"to keep using this version instead, restore the casebook-pre-upgrade backup "+
		"from the backups folder next to the database (nothing has been changed)",
		domain.ErrSchemaTooNew, have, latest)
}

// currentVersion returns max(version) from schema_migrations, or 0 when the
// table does not exist yet.
func currentVersion(ctx context.Context, q Querier) (int, error) {
	var n int
	if err := q.QueryRowContext(ctx,
		`SELECT count(*) FROM sqlite_schema WHERE type = 'table' AND name = 'schema_migrations'`).Scan(&n); err != nil {
		return 0, fmt.Errorf("read schema version: %w", err)
	}
	if n == 0 {
		return 0, nil
	}
	var v int
	if err := q.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&v); err != nil {
		return 0, fmt.Errorf("read schema version: %w", err)
	}
	return v, nil
}

// SchemaVersion returns the database's current schema version.
func (db *DB) SchemaVersion(ctx context.Context) (int, error) {
	return currentVersion(ctx, db.reader)
}
