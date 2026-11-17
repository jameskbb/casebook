package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/app/apptest"
)

func TestExportCancelled(t *testing.T) {
	d, fake := newTestDesktop(t)
	f := apptest.SeedCloudSpend(t, d.app)
	res, err := d.ExportInvestigation(f.InvestigationRef)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Cancelled || res.Path != "" || res.FileName != "" {
		t.Fatalf("result = %+v, want cancelled", res)
	}
	if len(fake.saveOpts) != 1 {
		t.Fatalf("save dialog shown %d times", len(fake.saveOpts))
	}
	o := fake.saveOpts[0]
	if o.Title != "Export case" || o.DefaultFilename != "INV-0001-september-cloud-bill-up-38.json" {
		t.Fatalf("dialog options = %+v", o)
	}
	if len(o.Filters) != 1 || o.Filters[0].DisplayName != "Casebook case (*.json)" || o.Filters[0].Pattern != "*.json" {
		t.Fatalf("filters = %+v", o.Filters)
	}
}

func TestExportWritesFile(t *testing.T) {
	d, fake := newTestDesktop(t)
	f := apptest.SeedCloudSpend(t, d.app)
	fake.savePath = filepath.Join(t.TempDir(), "out.json")
	res, err := d.ExportInvestigation(f.InvestigationRef)
	if err != nil {
		t.Fatal(err)
	}
	if res.Cancelled || res.Path != fake.savePath || res.FileName != "out.json" {
		t.Fatalf("result = %+v", res)
	}
	data, err := os.ReadFile(fake.savePath)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Format        string `json:"format"`
		Version       int    `json:"version"`
		Investigation struct {
			Ref string `json:"ref"`
		} `json:"investigation"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("export is not JSON: %v", err)
	}
	if doc.Format != app.ExportFormat || doc.Version != app.ExportVersion || doc.Investigation.Ref != "INV-0001" {
		t.Fatalf("export header = %+v", doc)
	}
	if runtime.GOOS != "windows" {
		st, err := os.Stat(fake.savePath)
		if err != nil {
			t.Fatal(err)
		}
		if st.Mode().Perm() != 0o600 {
			t.Fatalf("export mode = %v, want 0600", st.Mode().Perm())
		}
	}
	if ue := decodeUIError(t, func() error { _, err := d.ExportInvestigation("INV-0042"); return err }()); ue.Kind != kindNotFound {
		t.Fatalf("missing investigation export = %+v", ue)
	}
}

func TestExportRefusesCasebookFiles(t *testing.T) {
	d, fake := newTestDesktop(t)
	f := apptest.SeedCloudSpend(t, d.app)
	db := d.app.DBPath()
	backups := filepath.Join(filepath.Dir(db), "backups")
	if err := os.MkdirAll(backups, 0o700); err != nil {
		t.Fatal(err)
	}
	safety := filepath.Join(backups, "casebook-pre-restore-20261002T120000Z.db")
	if err := os.WriteFile(safety, []byte("safety copy"), 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(db)
	if err != nil {
		t.Fatal(err)
	}
	targets := []string{
		db,
		db + "-wal",
		db + "-shm",
		filepath.Join(filepath.Dir(db), ".", filepath.Base(db)),
		safety,
		filepath.Join(backups, "new.json"),
		filepath.Join(backups, "nested", "deeper.json"),
	}
	if foldCase {
		targets = append(targets, strings.ToUpper(db))
	}
	for _, target := range targets {
		fake.savePath = target
		_, err := d.ExportInvestigation(f.InvestigationRef)
		ue := decodeUIError(t, err)
		if ue.Kind != kindConflict || !strings.Contains(ue.Message, "Choose another location") {
			t.Fatalf("export to %q = %+v, want a conflict", target, ue)
		}
	}
	if got, _ := os.ReadFile(safety); string(got) != "safety copy" {
		t.Fatalf("safety copy changed to %q", got)
	}
	after, err := os.ReadFile(db)
	if err != nil || len(after) != len(before) {
		t.Fatalf("live database changed: %d bytes before, %d after, %v", len(before), len(after), err)
	}
	if _, err := d.ListInvestigations(app.ListOptions{}); err != nil {
		t.Fatalf("database unusable after refused exports: %v", err)
	}

	// A sibling of the database that is not one of its files is fine.
	fake.savePath = filepath.Join(filepath.Dir(db), "export.json")
	if _, err := d.ExportInvestigation(f.InvestigationRef); err != nil {
		t.Fatalf("export next to the database: %v", err)
	}
}

func TestExportReplacesAtomically(t *testing.T) {
	d, fake := newTestDesktop(t)
	f := apptest.SeedCloudSpend(t, d.app)
	dir := t.TempDir()
	fake.savePath = filepath.Join(dir, "out.json")
	if err := os.WriteFile(fake.savePath, []byte("old contents"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := d.ExportInvestigation(f.InvestigationRef); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(fake.savePath)
	if err != nil || !json.Valid(data) {
		t.Fatalf("replaced export is not JSON: %v", err)
	}
	if runtime.GOOS != "windows" {
		if st, _ := os.Stat(fake.savePath); st.Mode().Perm() != 0o600 {
			t.Fatalf("replaced export mode = %v, want 0600", st.Mode().Perm())
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("directory holds %d entries after export, want only out.json (no temp files)", len(entries))
	}

	// A failed write leaves the old file untouched and no temp file behind.
	if err := writePrivateFile(filepath.Join(dir, "missing", "x.json"), []byte("{}")); err == nil {
		t.Fatal("write into a missing directory succeeded")
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Fatalf("failed write left %d entries", len(entries))
	}
}

func TestImportRoundTrip(t *testing.T) {
	d, fake := newTestDesktop(t)
	f := apptest.SeedCloudSpend(t, d.app)
	path := filepath.Join(t.TempDir(), "case.json")
	data, err := d.app.ExportInvestigation(context.Background(), f.InvestigationRef)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	// Cancelled.
	out, err := d.ImportInvestigation()
	if err != nil || !out.Cancelled {
		t.Fatalf("cancelled import = %+v, %v", out, err)
	}
	if o := fake.openOpts[0]; o.Title != "Import case" || len(o.Filters) != 1 || o.Filters[0].Pattern != "*.json" {
		t.Fatalf("open dialog options = %+v", o)
	}

	// Into the same database: the ref is taken, so it changes.
	fake.openPath = path
	out, err = d.ImportInvestigation()
	if err != nil {
		t.Fatal(err)
	}
	if out.Cancelled || out.FileName != "case.json" || out.Title != "September cloud bill up 38%" {
		t.Fatalf("import = %+v", out)
	}
	if out.Result.InvestigationRef != "INV-0002" || !out.Result.RefChanged || out.Result.OriginalRef != "INV-0001" {
		t.Fatalf("import result = %+v", out.Result)
	}
	apptest.AssertNoJSONNulls(t, out)

	// Into a fresh database: the ref is kept.
	d2, fake2 := newTestDesktop(t)
	fake2.openPath = path
	out, err = d2.ImportInvestigation()
	if err != nil {
		t.Fatal(err)
	}
	if out.Result.InvestigationRef != "INV-0001" || out.Result.RefChanged {
		t.Fatalf("fresh import result = %+v", out.Result)
	}

	// An invalid file is a validation error and adds nothing.
	bad := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(bad, []byte(`{"format":"something-else"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	fake2.openPath = bad
	if ue := decodeUIError(t, func() error { _, err := d2.ImportInvestigation(); return err }()); ue.Kind != kindValidation || ue.Field == "" {
		t.Fatalf("bad import = %+v", ue)
	}
}

func TestImportTooLarge(t *testing.T) {
	d, fake := newTestDesktop(t)
	path := filepath.Join(t.TempDir(), "huge.json")
	fh, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := fh.Truncate(app.MaxImportBytes + 1); err != nil {
		t.Fatal(err)
	}
	fh.Close()
	fake.openPath = path
	ue := decodeUIError(t, func() error { _, err := d.ImportInvestigation(); return err }())
	if ue.Kind != kindValidation || ue.Field != "data" {
		t.Fatalf("huge import = %+v, want validation on data", ue)
	}
	list, _ := d.ListInvestigations(app.ListOptions{IncludeArchived: true})
	if len(list) != 0 {
		t.Fatalf("huge import added %d investigations", len(list))
	}
}

func TestImportRefusesNonRegularFile(t *testing.T) {
	d, fake := newTestDesktop(t)
	// A directory stands in for FIFOs and devices on every OS: none of them
	// is a regular file, and reading a FIFO would block forever.
	fake.openPath = t.TempDir()
	ue := decodeUIError(t, func() error { _, err := d.ImportInvestigation(); return err }())
	if ue.Kind != kindValidation || ue.Field != "data" || ue.Message != "must be a regular file" {
		t.Fatalf("import of a directory = %+v, want validation on data", ue)
	}
}

func TestBackUpCancelledAndConflict(t *testing.T) {
	d, fake := newTestDesktop(t)
	apptest.SeedCloudSpend(t, d.app)

	out, err := d.BackUp()
	if err != nil || !out.Cancelled {
		t.Fatalf("cancelled backup = %+v, %v", out, err)
	}
	o := fake.saveOpts[0]
	if o.Title != "Back up all Casebook data" || o.DefaultFilename != "casebook-backup-2026-10-02.db" ||
		len(o.Filters) != 1 || o.Filters[0].DisplayName != "Casebook backup (*.db)" || o.Filters[0].Pattern != "*.db" {
		t.Fatalf("backup dialog options = %+v", o)
	}

	fake.savePath = filepath.Join(t.TempDir(), "backup.db")
	out, err = d.BackUp()
	if err != nil {
		t.Fatal(err)
	}
	if out.Cancelled || out.Info.Path != fake.savePath || out.Info.Counts.Investigations != 1 {
		t.Fatalf("backup = %+v", out)
	}
	apptest.AssertNoJSONNulls(t, out)

	if ue := decodeUIError(t, func() error { _, err := d.BackUp(); return err }()); ue.Kind != kindConflict {
		t.Fatalf("backup over an existing file = %+v, want conflict", ue)
	}
}

// makeBackup writes a backup of a database holding one investigation titled
// title and returns its path.
func makeBackup(t *testing.T, title string) string {
	t.Helper()
	src := apptest.Open(t)
	if _, err := src.CreateInvestigation(context.Background(), app.ActorCLI(), app.CreateInvestigationInput{Title: title}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "casebook-backup.db")
	if _, err := src.Backup(context.Background(), path); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRestoreTwoStep(t *testing.T) {
	d, fake := newTestDesktop(t)
	apptest.SeedCloudSpend(t, d.app)
	backup := makeBackup(t, "From the backup")

	pick, err := d.PickBackupToRestore()
	if err != nil || !pick.Cancelled {
		t.Fatalf("cancelled pick = %+v, %v", pick, err)
	}
	if o := fake.openOpts[0]; o.Title != "Restore from backup" || len(o.Filters) != 1 || o.Filters[0].Pattern != "*.db" {
		t.Fatalf("restore dialog options = %+v", o)
	}

	fake.openPath = backup
	pick, err = d.PickBackupToRestore()
	if err != nil {
		t.Fatal(err)
	}
	if pick.Cancelled || pick.Path != backup || pick.FileName != "casebook-backup.db" || pick.Info.Counts.Investigations != 1 {
		t.Fatalf("pick = %+v", pick)
	}
	apptest.AssertNoJSONNulls(t, pick)

	res, err := d.RestoreBackup(pick.Path)
	if err != nil {
		t.Fatal(err)
	}
	if res.SafetyCopyPath == "" || res.SourcePath != backup {
		t.Fatalf("restore = %+v", res)
	}
	if _, err := os.Stat(res.SafetyCopyPath); err != nil {
		t.Fatalf("safety copy missing: %v", err)
	}
	list, err := d.ListInvestigations(app.ListOptions{IncludeArchived: true})
	if err != nil || len(list) != 1 || list[0].Title != "From the backup" {
		t.Fatalf("after restore list = %+v, %v", list, err)
	}

	// The pin is used up: the same path again needs a new pick.
	if ue := decodeUIError(t, func() error { _, err := d.RestoreBackup(backup); return err }()); ue.Kind != kindValidation || ue.Field != "path" {
		t.Fatalf("second restore = %+v", ue)
	}

	// An invalid file is refused at pick time and pins nothing.
	junk := filepath.Join(t.TempDir(), "junk.db")
	if err := os.WriteFile(junk, []byte("not a database"), 0o600); err != nil {
		t.Fatal(err)
	}
	fake.openPath = junk
	if ue := decodeUIError(t, func() error { _, err := d.PickBackupToRestore(); return err }()); ue.Kind != kindConflict {
		t.Fatalf("junk pick = %+v, want conflict", ue)
	}
	if ue := decodeUIError(t, func() error { _, err := d.RestoreBackup(junk); return err }()); ue.Kind != kindValidation || ue.Field != "path" {
		t.Fatalf("junk restore = %+v", ue)
	}
}

func TestRestoreRefusesUnpickedPath(t *testing.T) {
	d, fake := newTestDesktop(t)
	a := makeBackup(t, "Backup A")
	b := makeBackup(t, "Backup B")

	ue := decodeUIError(t, func() error { _, err := d.RestoreBackup(a); return err }())
	if ue.Kind != kindValidation || ue.Field != "path" || ue.Message != "choose the backup again" {
		t.Fatalf("unpicked restore = %+v", ue)
	}

	fake.openPath = a
	if _, err := d.PickBackupToRestore(); err != nil {
		t.Fatal(err)
	}
	if ue := decodeUIError(t, func() error { _, err := d.RestoreBackup(b); return err }()); ue.Kind != kindValidation || ue.Field != "path" {
		t.Fatalf("restore of another path = %+v", ue)
	}
	list, _ := d.ListInvestigations(app.ListOptions{IncludeArchived: true})
	if len(list) != 0 {
		t.Fatalf("a refused restore changed data: %+v", list)
	}
}

func TestDialogFailure(t *testing.T) {
	d, fake := newTestDesktop(t)
	fake.openErr = os.ErrPermission
	ue := decodeUIError(t, func() error { _, err := d.ImportInvestigation(); return err }())
	if ue.Kind != kindOther || ue.Field != "dialog" {
		t.Fatalf("dialog failure = %+v", ue)
	}
}

func TestExportFileName(t *testing.T) {
	cases := []struct{ ref, title, want string }{
		{"INV-0001", "September cloud bill up 38%", "INV-0001-september-cloud-bill-up-38.json"},
		{"INV-0002", "  Vendor contract renewals: US / EU (Q3)!  ", "INV-0002-vendor-contract-renewals-us-eu-q3.json"},
		{"INV-0003", "Café déjà vu", "INV-0003-caf-d-j-vu.json"},
		{"INV-0004", "!!!", "INV-0004.json"},
		{"INV-0005", "", "INV-0005.json"},
	}
	for _, c := range cases {
		if got := exportFileName(c.ref, c.title); got != c.want {
			t.Errorf("exportFileName(%q, %q) = %q, want %q", c.ref, c.title, got, c.want)
		}
	}
	long := slugify("a very long investigation title that keeps going well past the sixty character limit for slugs", 60)
	if len(long) > 60 || long[len(long)-1] == '-' || long[0] == '-' {
		t.Fatalf("long slug %q (%d)", long, len(long))
	}
	if got := slugify("--Already--hyphenated--", 60); got != "already-hyphenated" {
		t.Fatalf("slugify collapsed to %q", got)
	}
}

func TestBackupFileName(t *testing.T) {
	if got := backupFileName(time.Date(2026, 10, 2, 23, 59, 0, 0, time.Local)); got != "casebook-backup-2026-10-02.db" {
		t.Fatalf("backupFileName = %q", got)
	}
}

func TestSaveReport(t *testing.T) {
	d, fake := newTestDesktop(t)
	inv, err := d.CreateInvestigation(app.CreateInvestigationInput{Title: "Saved"})
	if err != nil {
		t.Fatal(err)
	}
	fake.savePath = "" // cancelled
	res, err := d.SaveReport(inv.Ref)
	if err != nil || !res.Cancelled {
		t.Fatalf("cancel = %+v, %v", res, err)
	}
	fake.savePath = d.dbPath
	if _, err := d.SaveReport(inv.Ref); err == nil || !strings.Contains(err.Error(), "cannot export over its own data files") {
		t.Fatalf("save over data file = %v", err)
	}
	fake.savePath = filepath.Join(t.TempDir(), "saved.md")
	res, err = d.SaveReport(inv.Ref)
	if err != nil || res.Cancelled || res.FileName != "saved.md" {
		t.Fatalf("save = %+v, %v", res, err)
	}
	if got := fake.saveOpts[len(fake.saveOpts)-1].DefaultFilename; got != inv.Ref+"-summary.md" {
		t.Fatalf("default file name = %q", got)
	}
	got, _ := os.ReadFile(fake.savePath)
	want, _ := d.app.Report(d.ctx, inv.Ref, app.ReportOptions{})
	if string(got) != want {
		t.Fatalf("file = %q, want the report", got)
	}
}
