package main

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/app/apptest"
)

// brokenDesktop returns a Desktop whose data file (garbage bytes in a temp
// folder) failed to open, plus a valid backup holding one investigation.
func brokenDesktop(t *testing.T) (*Desktop, *fakeNative, string) {
	t.Helper()
	src := apptest.Open(t)
	if _, err := src.CreateInvestigation(context.Background(), apptest.Human, app.CreateInvestigationInput{Title: "Recovered"}); err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(t.TempDir(), "casebook-backup.db")
	if _, err := src.Backup(context.Background(), backup); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "casebook.db")
	if err := os.WriteFile(dbPath, []byte("this is not a database, it is a broken file of sufficient length"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, openErr := app.Open(dbPath)
	if openErr == nil {
		t.Fatal("garbage database opened")
	}
	fake := &fakeNative{}
	d := &Desktop{ctx: context.Background(), native: fake, openErr: openErr, dbPath: dbPath, actor: app.ActorDesktop(), now: func() time.Time { return testNow }}
	t.Cleanup(func() {
		if d.app != nil {
			_ = d.app.Close()
		}
	})
	return d, fake, backup
}

func TestRecoverFromBackupReplacesUnopenedData(t *testing.T) {
	d, fake, backup := brokenDesktop(t)
	if s := d.StartupState(); s.OK || !s.Recoverable {
		t.Fatalf("StartupState = %+v, want a recoverable failure", s)
	}
	fake.openPath = backup
	pick, err := d.PickBackupToRecover()
	if err != nil {
		t.Fatal(err)
	}
	if pick.Path != backup || pick.Info.Counts.Investigations != 1 {
		t.Fatalf("pick = %+v", pick)
	}
	res, err := d.RecoverFromBackup(backup)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(filepath.Dir(d.dbPath), "backups", "casebook-unopened-"+testNow.UTC().Format("20060102T150405Z")+".db")
	if res.SafetyCopyPath != want {
		t.Fatalf("SafetyCopyPath = %q, want %q", res.SafetyCopyPath, want)
	}
	if b, err := os.ReadFile(want); err != nil || len(b) == 0 || string(b[:7]) != "this is" {
		t.Fatalf("moved file = %q, %v", b, err)
	}
	if s := d.StartupState(); !s.OK {
		t.Fatalf("StartupState after recovery = %+v", s)
	}
	list, err := d.ListInvestigations(app.ListOptions{})
	if err != nil || len(list) != 1 || list[0].Title != "Recovered" {
		t.Fatalf("list after recovery = %+v, %v", list, err)
	}
	if ue := decodeUIError(t, func() error { _, err := d.PickBackupToRecover(); return err }()); ue.Kind != kindConflict {
		t.Fatalf("recovery with the data open = %+v, want conflict", ue)
	}
}

func TestRecoverRefusals(t *testing.T) {
	d, fake, backup := brokenDesktop(t)

	// A pin is required, and the data file itself is not a backup.
	if ue := decodeUIError(t, func() error { _, err := d.RecoverFromBackup(backup); return err }()); ue.Kind != kindValidation {
		t.Fatalf("unpinned recovery = %+v", ue)
	}
	fake.openPath = d.dbPath
	if ue := decodeUIError(t, func() error { _, err := d.PickBackupToRecover(); return err }()); ue.Kind != kindConflict {
		t.Fatalf("picking the data file = %+v", ue)
	}
	// An invalid backup is refused before anything moves.
	bad := filepath.Join(t.TempDir(), "bad.db")
	if err := os.WriteFile(bad, []byte("nope"), 0o600); err != nil {
		t.Fatal(err)
	}
	fake.openPath = bad
	if _, err := d.PickBackupToRecover(); err == nil {
		t.Fatal("invalid backup accepted")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(d.dbPath), "backups")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("backups folder created by a refused pick: %v", err)
	}
	// Cancelling the dialog changes nothing.
	fake.openPath = ""
	if pick, err := d.PickBackupToRecover(); err != nil || !pick.Cancelled {
		t.Fatalf("cancelled pick = %+v, %v", pick, err)
	}

	// A busy file is not something a backup fixes.
	d.openErr = app.ErrBusy
	if d.StartupState().Recoverable {
		t.Fatal("busy reported recoverable")
	}
	if ue := decodeUIError(t, func() error { _, err := d.PickBackupToRecover(); return err }()); ue.Kind != kindConflict {
		t.Fatalf("pick while busy = %+v", ue)
	}
}

// newerDesktop is brokenDesktop with a data file from a newer Casebook
// (schema version 99) in place of the garbage one.
func newerDesktop(t *testing.T) (*Desktop, *fakeNative, string) {
	t.Helper()
	d, fake, backup := brokenDesktop(t)
	if err := os.Remove(d.dbPath); err != nil {
		t.Fatal(err)
	}
	newer, err := app.Open(d.dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := newer.Close(); err != nil {
		t.Fatal(err)
	}
	bumpSchema(t, d.dbPath, 99)
	_, d.openErr = app.Open(d.dbPath)
	if !errors.Is(d.openErr, app.ErrSchemaTooNew) {
		t.Fatalf("open newer = %v, want ErrSchemaTooNew", d.openErr)
	}
	return d, fake, backup
}

// TestRecoverRefusedWhileInUse: a data file another program holds when
// recovery runs is not moved aside, whatever the startup error said (here a
// newer Casebook's MCP server could still have it open), and recovery works
// once the program lets go.
func TestRecoverRefusedWhileInUse(t *testing.T) {
	d, fake, backup := newerDesktop(t)
	if !d.StartupState().Recoverable {
		t.Fatal("newer schema not recoverable")
	}
	fake.openPath = backup
	holder := holdOpen(t, d.dbPath)
	if ue := decodeUIError(t, func() error { _, err := d.PickBackupToRecover(); return err }()); ue != errRecoverInUseUI(t) {
		t.Fatalf("pick while held = %+v", ue)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(d.dbPath), "backups")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("backups folder created while held: %v", err)
	}
	if err := holder.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := d.PickBackupToRecover(); err != nil {
		t.Fatalf("pick after release: %v", err)
	}
	holder = holdOpen(t, d.dbPath)
	if ue := decodeUIError(t, func() error { _, err := d.RecoverFromBackup(backup); return err }()); ue != errRecoverInUseUI(t) {
		t.Fatalf("recover while held = %+v", ue)
	}
	if !anyExists(d.dbPath) {
		t.Fatal("data file moved while held")
	}
	if err := holder.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err := d.PickBackupToRecover(); err != nil {
		t.Fatal(err)
	}
	if _, err := d.RecoverFromBackup(backup); err != nil {
		t.Fatalf("recover after release: %v", err)
	}
}

// TestRecoverFromNewerSchema: going back to an older Casebook, the startup
// screen restores the pre-upgrade backup and moves the newer file aside
// (docs/updating.md).
func TestRecoverFromNewerSchema(t *testing.T) {
	d, fake, backup := newerDesktop(t)
	if s := d.StartupState(); s.Kind != "schema_too_new" || !s.Recoverable {
		t.Fatalf("StartupState = %+v, want a recoverable schema_too_new", s)
	}
	fake.openPath = backup
	if _, err := d.PickBackupToRecover(); err != nil {
		t.Fatal(err)
	}
	res, err := d.RecoverFromBackup(backup)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(res.SafetyCopyPath); err != nil {
		t.Fatalf("newer file not moved aside: %v", err)
	}
	list, err := d.ListInvestigations(app.ListOptions{})
	if err != nil || len(list) != 1 || list[0].Title != "Recovered" {
		t.Fatalf("list after recovery = %+v, %v", list, err)
	}
}

func TestOpenDataFolderAndCopyLocation(t *testing.T) {
	d, fake, _ := brokenDesktop(t)
	if err := d.OpenDataFolder(); err != nil {
		t.Fatal(err)
	}
	if len(fake.folders) != 1 || fake.folders[0] != filepath.Dir(d.dbPath) {
		t.Fatalf("folders = %v", fake.folders)
	}
	fake.folderErr = errors.New("no file manager")
	if ue := decodeUIError(t, d.OpenDataFolder()); ue.Kind != kindOther || ue.Field != "folder" {
		t.Fatalf("folder failure = %+v", ue)
	}
	if err := d.CopyDataLocation(); err != nil || fake.clipboard != d.dbPath {
		t.Fatalf("CopyDataLocation = %v, clipboard %q", err, fake.clipboard)
	}
}

func TestMoveAsideRestoresOnFailure(t *testing.T) {
	dir := t.TempDir()
	db := filepath.Join(dir, "casebook.db")
	for _, s := range []string{"", "-wal"} {
		if err := os.WriteFile(db+s, []byte("x"+s), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	moved, err := moveAside(db, testNow)
	if err != nil {
		t.Fatal(err)
	}
	if anyExists(db) {
		t.Fatal("database still in place after moveAside")
	}
	if b, _ := os.ReadFile(moved + "-wal"); string(b) != "x-wal" {
		t.Fatalf("side file not moved: %q", b)
	}
	// A second move in the same second gets a -2 name.
	if err := os.WriteFile(db, []byte("y"), 0o600); err != nil {
		t.Fatal(err)
	}
	second, err := moveAside(db, testNow)
	if err != nil || second == moved || filepath.Base(second) != "casebook-unopened-"+testNow.UTC().Format("20060102T150405Z")+"-2.db" {
		t.Fatalf("second move = %q, %v", second, err)
	}
	// Putting a file back.
	if err := moveFiles(moved, db); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(db); string(b) != "x" {
		t.Fatalf("put back = %q", b)
	}
	if none, err := moveAside(filepath.Join(dir, "missing.db"), testNow); err != nil || none != "" {
		t.Fatalf("missing file = %q, %v", none, err)
	}
}

// holdOpen opens an idle SQLite connection on the database at path, as
// another program with the file open would (this version's engine refuses a
// newer schema, so a plain connection stands in for the newer Casebook).
// Close lets go.
func holdOpen(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec("PRAGMA user_version"); err != nil {
		t.Fatal(err)
	}
	return db
}

// errRecoverInUseUI is errRecoverInUse as the page receives it.
func errRecoverInUseUI(t *testing.T) uiError {
	t.Helper()
	return decodeUIError(t, errRecoverInUse)
}

// bumpSchema records schema version v in the closed database at path, as a
// newer Casebook would.
func bumpSchema(t *testing.T, path string, v int) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`INSERT INTO schema_migrations (version, applied_at) VALUES (?, '2030-01-01T00:00:00.000Z')`, v); err != nil {
		t.Fatal(err)
	}
}
