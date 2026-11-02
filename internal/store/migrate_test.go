package store

import (
	"context"
	"database/sql"
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/jameskbb/casebook/internal/domain"
)

func rawDB(t *testing.T, path string) *sql.DB {
	t.Helper()
	w, err := sql.Open("sqlite", buildDSN(path, 5*time.Second)+"&_txlock=immediate")
	if err != nil {
		t.Fatal(err)
	}
	w.SetMaxOpenConns(1)
	t.Cleanup(func() { w.Close() })
	return w
}

func migrationRows(t *testing.T, q Querier) int {
	t.Helper()
	var n int
	if err := q.QueryRowContext(context.Background(), `SELECT count(*) FROM schema_migrations`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func realInit(t *testing.T) []byte {
	t.Helper()
	b, err := fs.ReadFile(Migrations, "migrations/0001_init.sql")
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestMigrateFreshReachesLatest(t *testing.T) {
	db := openTestDB(t)
	v, err := db.SchemaVersion(context.Background())
	if err != nil || v != LatestVersion() || v < 1 {
		t.Fatalf("SchemaVersion = %d, %v; want LatestVersion %d", v, err, LatestVersion())
	}
	if n := migrationRows(t, db.reader); n != LatestVersion() {
		t.Fatalf("schema_migrations rows = %d, want %d", n, LatestVersion())
	}
}

func TestMigrateIdempotentReopen(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "casebook.db")
	for i := 0; i < 3; i++ {
		db, err := Open(ctx, path, Config{})
		if err != nil {
			t.Fatalf("open %d: %v", i, err)
		}
		if n := migrationRows(t, db.reader); n != LatestVersion() {
			t.Fatalf("open %d: schema_migrations rows = %d", i, n)
		}
		db.Close()
	}
}

func TestMigrateUpgradeKeepsSeededData(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "casebook.db")
	v1 := fstest.MapFS{"0001_init.sql": {Data: realInit(t)}}
	w := rawDB(t, path)
	if v, err := Migrate(ctx, w, v1, nil); err != nil || v != 1 {
		t.Fatalf("migrate to 1 = %d, %v", v, err)
	}
	if err := func() error {
		tx, err := w.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err := insertTestInvestigation(ctx, tx, "seeded under v1"); err != nil {
			tx.Rollback()
			return err
		}
		return tx.Commit()
	}(); err != nil {
		t.Fatal(err)
	}

	v2 := fstest.MapFS{
		"0001_init.sql": {Data: realInit(t)},
		"0002_extra.sql": {Data: []byte(`CREATE TABLE extra (id INTEGER PRIMARY KEY, note TEXT NOT NULL) STRICT;
ALTER TABLE investigations ADD COLUMN priority TEXT NOT NULL DEFAULT 'normal';`)},
	}
	if v, err := Migrate(ctx, w, v2, nil); err != nil || v != 2 {
		t.Fatalf("migrate to 2 = %d, %v", v, err)
	}
	var title, priority string
	if err := w.QueryRow(`SELECT title, priority FROM investigations WHERE ref_num = 1`).Scan(&title, &priority); err != nil {
		t.Fatal(err)
	}
	if title != "seeded under v1" || priority != "normal" {
		t.Fatalf("seeded row = %q/%q", title, priority)
	}
	if n := migrationRows(t, w); n != 2 {
		t.Fatalf("schema_migrations rows = %d, want 2", n)
	}
	// Re-running is a no-op.
	if v, err := Migrate(ctx, w, v2, nil); err != nil || v != 2 || migrationRows(t, w) != 2 {
		t.Fatalf("re-run = %d, %v", v, err)
	}
}

func TestMigrateRefusesNewerSchema(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "casebook.db")
	w := rawDB(t, path)
	if _, err := w.Exec(`CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL);
INSERT INTO schema_migrations VALUES (99, '2030-01-01T00:00:00.000Z');`); err != nil {
		t.Fatal(err)
	}
	w.Close()

	_, err := Open(ctx, path, Config{BeforeUpgrade: func(context.Context, *DB, int) error {
		t.Error("BeforeUpgrade ran for a newer database")
		return nil
	}})
	if !errors.Is(err, domain.ErrSchemaTooNew) {
		t.Fatalf("Open newer DB = %v, want ErrSchemaTooNew", err)
	}
	// The message tells a person what to do: update, or go back with the
	// pre-upgrade backup.
	for _, want := range []string{"update Casebook to open it", "casebook-pre-upgrade backup", "database schema 99"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q lacks %q", err.Error(), want)
		}
	}
	w2 := rawDB(t, path)
	var tables int
	if err := w2.QueryRow(`SELECT count(*) FROM sqlite_schema WHERE type = 'table' AND name = 'investigations'`).Scan(&tables); err != nil {
		t.Fatal(err)
	}
	if tables != 0 || migrationRows(t, w2) != 1 {
		t.Fatalf("newer DB was modified: investigations tables=%d", tables)
	}
}

func TestMigrateRefusesGap(t *testing.T) {
	ctx := context.Background()
	w := rawDB(t, filepath.Join(t.TempDir(), "casebook.db"))
	gap := fstest.MapFS{
		"0001_init.sql":  {Data: realInit(t)},
		"0003_later.sql": {Data: []byte(`CREATE TABLE later (id INTEGER PRIMARY KEY);`)},
	}
	if _, err := Migrate(ctx, w, gap, nil); err == nil {
		t.Fatal("Migrate with a gap succeeded")
	}
	var n int
	if err := w.QueryRow(`SELECT count(*) FROM sqlite_schema WHERE name IN ('investigations', 'later')`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("gap migration applied something: %d, %v", n, err)
	}
	bad := fstest.MapFS{"init.sql": {Data: []byte(`SELECT 1;`)}}
	if _, err := Migrate(ctx, w, bad, nil); err == nil {
		t.Fatal("Migrate with a misnamed file succeeded")
	}
}

func TestMigrateConcurrentFirstOpen(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "fresh", "casebook.db")
	const n = 4
	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		errs []error
		dbs  []*DB
	)
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			db, err := Open(ctx, path, Config{})
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, err)
				return
			}
			dbs = append(dbs, db)
		}()
	}
	close(start)
	wg.Wait()
	for _, db := range dbs {
		defer db.Close()
	}
	if len(errs) != 0 {
		t.Fatalf("concurrent opens failed: %v", errs)
	}
	if got := migrationRows(t, dbs[0].reader); got != LatestVersion() {
		t.Fatalf("schema_migrations rows = %d, want %d", got, LatestVersion())
	}
}
