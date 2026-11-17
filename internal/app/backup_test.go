package app_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/app/apptest"
)

// backupOf backs a up into a fresh temp dir and returns the info.
func backupOf(t *testing.T, a *app.App) app.BackupInfo {
	t.Helper()
	info, err := a.Backup(context.Background(), filepath.Join(t.TempDir(), "casebook-backup.db"))
	if err != nil {
		t.Fatalf("Backup: %v", err)
	}
	return info
}

// execFile runs statements directly on a database file (journal mode
// unchanged), closing it afterwards.
func execFile(t *testing.T, path string, stmts ...string) {
	t.Helper()
	db := rawDB(t, path, false)
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("exec %q: %v", s, err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
}

// copyFile copies src to a new file in a temp dir.
func copyFile(t *testing.T, src string) string {
	t.Helper()
	b, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(t.TempDir(), "copy.db")
	if err := os.WriteFile(dst, b, 0o600); err != nil {
		t.Fatal(err)
	}
	return dst
}

// assertNoSidecars fails when -wal, -shm or -journal files exist next to p.
func assertNoSidecars(t *testing.T, p string) {
	t.Helper()
	for _, suffix := range []string{"-wal", "-shm", "-journal"} {
		if _, err := os.Stat(p + suffix); err == nil {
			t.Fatalf("%s exists", filepath.Base(p+suffix))
		}
	}
}

func TestBackupSeeded(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t)
	apptest.SeedCloudSpend(t, a)
	st, err := a.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	info := backupOf(t, a)
	if !filepath.IsAbs(info.Path) || info.SizeBytes <= 0 {
		t.Fatalf("info = %+v", info)
	}
	if info.SchemaVersion != st.SchemaVersion || info.SchemaVersion < 1 {
		t.Fatalf("schema version = %d, want %d", info.SchemaVersion, st.SchemaVersion)
	}
	if info.Counts != st.Counts {
		t.Fatalf("counts = %+v, want %+v", info.Counts, st.Counts)
	}
	if info.CreatedAt != app.FormatTime(apptest.FixedTime) {
		t.Fatalf("created_at = %q", info.CreatedAt)
	}
	if runtime.GOOS != "windows" {
		assertMode(t, info.Path, 0o600)
	}
	assertNoSidecars(t, info.Path)
	after, err := a.Status(ctx)
	if err != nil || after.Counts != st.Counts {
		t.Fatalf("backup changed the database: %+v, %v", after.Counts, err)
	}
}

func TestBackupRefusals(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t)
	apptest.SeedCloudSpend(t, a)

	existing := filepath.Join(t.TempDir(), "keep.db")
	if err := os.WriteFile(existing, []byte("keep me"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := a.Backup(ctx, existing)
	wantErrIs(t, err, app.ErrConflict)
	if b, _ := os.ReadFile(existing); string(b) != "keep me" {
		t.Fatalf("existing destination changed: %q", b)
	}

	_, err = a.Backup(ctx, filepath.Join(t.TempDir(), "missing", "b.db"))
	wantValidation(t, err, "path")
	_, err = a.Backup(ctx, "  ")
	wantValidation(t, err, "path")

	_, err = a.Backup(ctx, a.DBPath())
	wantErrIs(t, err, app.ErrConflict)
	_, err = a.Backup(ctx, a.DBPath()+"-wal")
	wantErrIs(t, err, app.ErrConflict)
	if err := a.CheckIntegrity(ctx); err != nil {
		t.Fatalf("live database damaged: %v", err)
	}
}

func TestBackupWhileAnotherAppWrites(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t)
	f := apptest.SeedCloudSpend(t, a)
	other := apptest.Reopen(t, a.DBPath())

	stop := make(chan struct{})
	var (
		wg     sync.WaitGroup
		werr   error
		writes int
	)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
			}
			_, err := other.AddEvidence(ctx, apptest.AI, f.InvestigationRef, app.EvidenceInput{
				Title: fmt.Sprintf("Concurrent %d", i), Summary: "written during backup", SourceType: app.SourceTypeFile})
			if err != nil {
				werr = err
				return
			}
			writes++
		}
	}()
	// Let the writer get going, then take several backups while it runs.
	time.Sleep(20 * time.Millisecond)
	var infos []app.BackupInfo
	for i := 0; i < 3; i++ {
		infos = append(infos, backupOf(t, a))
	}
	close(stop)
	wg.Wait()
	if werr != nil {
		t.Fatalf("concurrent writer failed: %v", werr)
	}
	if writes == 0 {
		t.Fatal("writer made no writes")
	}
	for _, info := range infos {
		got, err := a.ValidateBackup(ctx, info.Path)
		if err != nil {
			t.Fatalf("ValidateBackup: %v", err)
		}
		if got.Counts != info.Counts || got.Counts.Evidence < 4 {
			t.Fatalf("validated counts = %+v, backup counts = %+v", got.Counts, info.Counts)
		}
	}
}

func TestValidateBackupMissing(t *testing.T) {
	a := apptest.Open(t)
	_, err := a.ValidateBackup(context.Background(), filepath.Join(t.TempDir(), "nope.db"))
	wantErrIs(t, err, app.ErrNotFound)
	if !strings.Contains(err.Error(), "nope.db") || strings.Contains(err.Error(), t.TempDir()) {
		t.Fatalf("message = %q, want the base name only", err)
	}
}

func TestValidateBackupRejectsBadFiles(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t)
	apptest.SeedCloudSpend(t, a)
	good := backupOf(t, a).Path
	goodBytes, err := os.ReadFile(good)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		make func(t *testing.T) string
	}{
		{"random bytes", func(t *testing.T) string {
			p := filepath.Join(t.TempDir(), "random.db")
			os.WriteFile(p, bytes.Repeat([]byte{0xde, 0xad, 0xbe, 0xef}, 4096), 0o600)
			return p
		}},
		{"empty file", func(t *testing.T) string {
			p := filepath.Join(t.TempDir(), "empty.db")
			os.WriteFile(p, nil, 0o600)
			return p
		}},
		{"truncated copy", func(t *testing.T) string {
			p := filepath.Join(t.TempDir(), "truncated.db")
			os.WriteFile(p, goodBytes[:len(goodBytes)/2], 0o600)
			return p
		}},
		{"plain SQLite without schema_migrations", func(t *testing.T) string {
			p := filepath.Join(t.TempDir(), "plain.db")
			execFile(t, p, `CREATE TABLE notes (body TEXT)`, `INSERT INTO notes VALUES ('hi')`)
			return p
		}},
		{"extra view", func(t *testing.T) string {
			p := copyFile(t, good)
			execFile(t, p, `CREATE VIEW sneaky AS SELECT title FROM investigations`)
			return p
		}},
		{"extra trigger", func(t *testing.T) string {
			p := copyFile(t, good)
			execFile(t, p, `CREATE TRIGGER evil AFTER INSERT ON evidence BEGIN DELETE FROM claims; END`)
			return p
		}},
		{"schema version 99", func(t *testing.T) string {
			p := copyFile(t, good)
			execFile(t, p, `INSERT INTO schema_migrations (version, applied_at) VALUES (99, '2026-01-01T00:00:00.000Z')`)
			return p
		}},
		{"300-byte title", func(t *testing.T) string {
			p := copyFile(t, good)
			execFile(t, p, `UPDATE investigations SET title = '`+strings.Repeat("x", 300)+`'`)
			return p
		}},
		{"directory", func(t *testing.T) string { return t.TempDir() }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := tc.make(t)
			var before []byte
			if fi, err := os.Stat(p); err == nil && fi.Mode().IsRegular() {
				before, _ = os.ReadFile(p)
			}
			_, err := a.ValidateBackup(ctx, p)
			wantErrIs(t, err, app.ErrConflict)
			if !strings.Contains(err.Error(), "not a valid Casebook backup (") {
				t.Fatalf("message = %q", err)
			}
			assertNoRawSQLite(t, err)
			for _, bad := range []string{"malformed", "file is not a database", "out of memory", "no such"} {
				if strings.Contains(err.Error(), bad) {
					t.Fatalf("message leaks SQLite text: %q", err)
				}
			}
			if before != nil {
				after, _ := os.ReadFile(p)
				if !bytes.Equal(before, after) {
					t.Fatal("ValidateBackup modified the file")
				}
				assertNoSidecars(t, p)
			}
		})
	}
}

func TestValidateBackupReadOnly(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t)
	apptest.SeedCloudSpend(t, a)
	info := backupOf(t, a)
	before, err := os.ReadFile(info.Path)
	if err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(info.Path)
	if err != nil {
		t.Fatal(err)
	}
	got, err := a.ValidateBackup(ctx, info.Path)
	if err != nil {
		t.Fatalf("ValidateBackup: %v", err)
	}
	if got.Path != info.Path || got.SizeBytes != info.SizeBytes || got.Counts != info.Counts ||
		got.CreatedAt != app.FormatTime(fi.ModTime()) {
		t.Fatalf("validate = %+v, backup = %+v", got, info)
	}
	after, err := os.ReadFile(info.Path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("ValidateBackup modified the backup")
	}
	assertNoSidecars(t, info.Path)

	// A backup opens as a normal Casebook database too.
	b := apptest.Reopen(t, copyFile(t, info.Path))
	if inv, err := b.GetInvestigation(ctx, "INV-0001"); err != nil || inv.Title != "September cloud bill up 38%" {
		t.Fatalf("backup content = %+v, %v", inv, err)
	}
}
