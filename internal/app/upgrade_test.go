package app_test

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/app/apptest"
	"github.com/jameskbb/casebook/internal/store"
)

// oldDatabase writes a database at schema version LatestVersion()-1 (the
// newest migration hidden) to <new temp dir>/casebook.db, as an older
// Casebook left it, and returns its path.
func oldDatabase(t *testing.T) (string, int) {
	t.Helper()
	old := store.LatestVersion() - 1
	if old < 1 {
		t.Skip("needs at least two migrations")
	}
	sub, err := fs.Sub(store.Migrations, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	entries, err := fs.ReadDir(sub, ".")
	if err != nil {
		t.Fatal(err)
	}
	mfs := fstest.MapFS{}
	for _, e := range entries {
		var v int
		if _, err := fmt.Sscanf(e.Name(), "%04d_", &v); err != nil || v > old {
			continue
		}
		b, err := fs.ReadFile(sub, e.Name())
		if err != nil {
			t.Fatal(err)
		}
		mfs[e.Name()] = &fstest.MapFile{Data: b}
	}
	path := filepath.Join(t.TempDir(), "casebook.db")
	if _, to, err := store.MigrateFile(context.Background(), path, mfs); err != nil || to != old {
		t.Fatalf("build schema %d database: to %d, %v", old, to, err)
	}
	return path, old
}

// preUpgradeCopies lists the pre-upgrade backups next to dbPath.
func preUpgradeCopies(t *testing.T, dbPath string) []string {
	t.Helper()
	m, err := filepath.Glob(filepath.Join(filepath.Dir(dbPath), "backups", app.PreUpgradePrefix+"*.db"))
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestOpenBacksUpBeforeUpgrade(t *testing.T) {
	ctx := context.Background()
	path, old := oldDatabase(t)
	a := apptest.Reopen(t, path)

	st, err := a.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if st.SchemaVersion != store.LatestVersion() {
		t.Fatalf("schema after open = %d, want %d", st.SchemaVersion, store.LatestVersion())
	}
	copies := preUpgradeCopies(t, path)
	if len(copies) != 1 {
		t.Fatalf("pre-upgrade copies = %v, want 1", copies)
	}
	want := fmt.Sprintf("%sschema%d-%s.db", app.PreUpgradePrefix, old, apptest.FixedClock()().UTC().Format("20060102T150405Z"))
	if filepath.Base(copies[0]) != want {
		t.Fatalf("copy name = %s, want %s", filepath.Base(copies[0]), want)
	}
	// The copy is a valid backup at the old schema: an older Casebook can
	// restore it.
	info, err := a.ValidateBackup(ctx, copies[0])
	if err != nil {
		t.Fatalf("ValidateBackup(pre-upgrade copy): %v", err)
	}
	if info.SchemaVersion != old {
		t.Fatalf("copy schema = %d, want %d", info.SchemaVersion, old)
	}

	// Reopening the now up-to-date database takes no further copy.
	apptest.Reopen(t, path)
	if n := len(preUpgradeCopies(t, path)); n != 1 {
		t.Fatalf("pre-upgrade copies after an up-to-date reopen = %d, want 1", n)
	}
}

func TestOpenFreshDatabaseTakesNoCopy(t *testing.T) {
	a := apptest.Open(t)
	apptest.Reopen(t, a.DBPath())
	if _, err := os.Stat(filepath.Join(filepath.Dir(a.DBPath()), "backups")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("backups folder after fresh opens: %v", err)
	}
}

func TestOpenRefusesUpgradeWhenBackupFails(t *testing.T) {
	path, old := oldDatabase(t)
	// A file where the backups folder belongs makes the copy impossible.
	if err := os.WriteFile(filepath.Join(filepath.Dir(path), "backups"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	a, err := app.Open(path)
	if err == nil {
		a.Close()
		t.Fatal("Open upgraded without a pre-upgrade backup")
	}
	if !strings.Contains(err.Error(), "cannot back up the database before upgrading it") {
		t.Fatalf("Open error = %v", err)
	}
	if errors.Is(err, app.ErrInternal) {
		t.Fatalf("a filesystem failure should name its reason, got %v", err)
	}
	// The file keeps its old schema.
	if err := os.Remove(filepath.Join(filepath.Dir(path), "backups")); err != nil {
		t.Fatal(err)
	}
	info, err := apptest.Open(t).ValidateBackup(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if info.SchemaVersion != old {
		t.Fatalf("schema after a refused upgrade = %d, want %d", info.SchemaVersion, old)
	}
}

func TestOpenConcurrentUpgrades(t *testing.T) {
	path, _ := oldDatabase(t)
	const n = 6
	var wg sync.WaitGroup
	errs := make([]error, n)
	apps := make([]*app.App, n)
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			apps[i], errs[i] = app.Open(path, app.WithClock(apptest.FixedClock()))
		}()
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("open %d: %v", i, err)
		}
		st, err := apps[i].Status(context.Background())
		if err != nil || st.SchemaVersion != store.LatestVersion() {
			t.Fatalf("open %d: schema %d, %v", i, st.SchemaVersion, err)
		}
		apps[i].Close()
	}
	if c := preUpgradeCopies(t, path); len(c) < 1 || len(c) > n {
		t.Fatalf("pre-upgrade copies = %d, want 1..%d", len(c), n)
	}
}
