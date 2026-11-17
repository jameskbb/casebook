package doctor

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeDesktop writes a stand-in desktop app next to the fixture's CLI
// (dir(dir(CLI))/Casebook on this test's GOOS) holding data.
func writeDesktop(t *testing.T, f *doctorFixture, data string) string {
	t.Helper()
	name := "Casebook"
	if f.Opts.GOOS == "windows" {
		name += ".exe"
	}
	p := filepath.Join(filepath.Dir(filepath.Dir(f.Opts.CLIPath)), name)
	doctorWrite(t, p, data)
	return p
}

func TestRunBundleVersion(t *testing.T) {
	for _, goos := range []string{"linux", "windows", "darwin"} {
		t.Run(goos, func(t *testing.T) {
			f := newDoctorFixture(t, filepath.Join(t.TempDir(), "casebook.db"))
			f.Opts.GOOS = goos
			run := func() Report { return Run(context.Background(), f.Opts) }

			wantCheck(t, run(), "bundle_version", StatusSkip, "no desktop app next to the CLI")

			// The prefix as a plain string constant, followed by other text,
			// is not a stamp.
			desktop := writeDesktop(t, f, "MZ junk casebook-version=.casebook-backup multipart\x00 more")
			wantCheck(t, run(), "bundle_version", StatusSkip, desktop, "no version stamp")

			writeDesktop(t, f, "MZ casebook-version=.casebook-ba\x01 casebook-version=v9.9.9-test\x00\x00 tail")
			wantCheck(t, run(), "bundle_version", StatusOK, "desktop app and CLI are both v9.9.9-test")

			writeDesktop(t, f, "MZ casebook-version=v9.9.9-test2\x00 tail")
			r := run()
			wantCheck(t, r, "bundle_version", StatusWarn,
				"the desktop app is v9.9.9-test2, this CLI is v9.9.9-test: replace the whole folder when you update")
			if !r.OK {
				t.Fatal("a version mismatch must not fail doctor")
			}

			f.Opts.Version = "dev"
			wantCheck(t, run(), "bundle_version", StatusSkip, "development build")
		})
	}
}

// TestScanStampAcrossChunks: a marker that straddles a read boundary is
// found, and one past the scan limit is not read.
func TestScanStampAcrossChunks(t *testing.T) {
	dir := t.TempDir()
	marker := "casebook-version=v1.2.3\x00"
	for _, off := range []int{0, bundleChunk - 10, bundleChunk - 1, bundleChunk, 2*bundleChunk - 30} {
		p := filepath.Join(dir, "Casebook")
		data := strings.Repeat("\xff", off) + marker + strings.Repeat("\xff", 100)
		if err := os.WriteFile(p, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
		match, other, err := scanStamp(p, "v1.2.3")
		if err != nil || !match || other != "" {
			t.Fatalf("offset %d: match %v other %q err %v", off, match, other, err)
		}
		match, other, err = scanStamp(p, "v1.2.4")
		if err != nil || match || other != "v1.2.3" {
			t.Fatalf("offset %d, other version: match %v other %q err %v", off, match, other, err)
		}
	}
	if _, _, err := scanStamp(filepath.Join(dir, "missing"), "v1"); err == nil {
		t.Fatal("scanStamp of a missing file succeeded")
	}
}

// TestRunNewerDatabase: a database from a newer Casebook fails with what to
// do, and doctor leaves it untouched.
func TestRunNewerDatabase(t *testing.T) {
	db, a := doctorSeeded(t)
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	raw, err := sql.Open("sqlite", db)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(`INSERT INTO schema_migrations (version, applied_at) VALUES (99, '2030-01-01T00:00:00.000Z')`); err != nil {
		t.Fatal(err)
	}
	raw.Close()
	r := Run(context.Background(), newDoctorFixture(t, db).Opts)
	wantCheck(t, r, "database", StatusFail, db, "saved by a newer version of Casebook", "database schema 99",
		"casebook-pre-upgrade backup")
	wantCheck(t, r, "migrations", StatusFail, "newer than this build")
	if _, err := os.Stat(filepath.Join(filepath.Dir(db), "backups")); err == nil {
		t.Fatal("doctor wrote a backup for a newer database")
	}
}
