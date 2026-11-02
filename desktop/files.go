package main

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/jameskbb/casebook/internal/app"
)

// Export, import, backup and restore glue (D-06, D-07). File paths come only
// from native dialogs shown by Go; the page never passes a path to read or
// write, except RestoreBackup, which accepts only the path PickBackupToRestore
// validated and pinned.

// FileResult is the result of a save dialog based action. Cancelled is true
// when the user closed the dialog; nothing was written then.
type FileResult struct {
	Cancelled bool   `json:"cancelled"`
	Path      string `json:"path"`
	FileName  string `json:"file_name"`
}

// ImportOutcome is the result of ImportInvestigation. Title is the imported
// investigation's title (also in Result.Title).
type ImportOutcome struct {
	Cancelled bool             `json:"cancelled"`
	FileName  string           `json:"file_name"`
	Title     string           `json:"title"`
	Result    app.ImportResult `json:"result"`
}

// BackupOutcome is the result of BackUp.
type BackupOutcome struct {
	Cancelled bool           `json:"cancelled"`
	Info      app.BackupInfo `json:"info"`
}

// RestorePick is the validated backup the user chose, shown in the restore
// confirmation before RestoreBackup runs.
type RestorePick struct {
	Cancelled bool           `json:"cancelled"`
	Path      string         `json:"path"`
	FileName  string         `json:"file_name"`
	Info      app.BackupInfo `json:"info"`
}

var (
	jsonFilter = []runtime.FileFilter{{DisplayName: "Casebook case (*.json)", Pattern: "*.json"}}
	dbFilter   = []runtime.FileFilter{{DisplayName: "Casebook backup (*.db)", Pattern: "*.db"}}
)

// ExportInvestigation asks where to save and writes the investigation as
// casebook.investigation JSON, owner-only (0600).
func (d *Desktop) ExportInvestigation(invRef string) (FileResult, error) {
	if err := d.ready(); err != nil {
		return FileResult{}, err
	}
	inv, err := d.app.GetInvestigation(d.ctx, invRef)
	if err != nil {
		return FileResult{}, mapErr(err)
	}
	path, err := d.native.SaveFile(runtime.SaveDialogOptions{
		Title:                "Export case",
		DefaultFilename:      exportFileName(inv.Ref, inv.Title),
		Filters:              jsonFilter,
		CanCreateDirectories: true,
	})
	if err != nil {
		return FileResult{}, nativeErr("dialog", err)
	}
	if path == "" {
		return FileResult{Cancelled: true}, nil
	}
	if d.isCasebookFile(path) {
		return FileResult{}, errExportOverData
	}
	data, err := d.app.ExportInvestigation(d.ctx, inv.Ref)
	if err != nil {
		return FileResult{}, mapErr(err)
	}
	if err := writePrivateFile(path, data); err != nil {
		return FileResult{}, mapErr(err)
	}
	return FileResult{Path: path, FileName: filepath.Base(path)}, nil
}

var mdFilter = []runtime.FileFilter{{DisplayName: "Markdown document (*.md)", Pattern: "*.md"}}

// SaveReport writes the report (app.Report) to a Markdown file chosen in
// the native save dialog, for sending to the person who asked the question.
func (d *Desktop) SaveReport(invRef string) (FileResult, error) {
	if err := d.ready(); err != nil {
		return FileResult{}, err
	}
	inv, err := d.app.GetInvestigation(d.ctx, invRef)
	if err != nil {
		return FileResult{}, mapErr(err)
	}
	text, err := d.app.Report(d.ctx, inv.Ref, app.ReportOptions{})
	if err != nil {
		return FileResult{}, mapErr(err)
	}
	path, err := d.native.SaveFile(runtime.SaveDialogOptions{
		Title:                "Save summary",
		DefaultFilename:      inv.Ref + "-summary.md",
		Filters:              mdFilter,
		CanCreateDirectories: true,
	})
	if err != nil {
		return FileResult{}, nativeErr("dialog", err)
	}
	if path == "" {
		return FileResult{Cancelled: true}, nil
	}
	if d.isCasebookFile(path) {
		return FileResult{}, errExportOverData
	}
	if err := writePrivateFile(path, []byte(text)); err != nil {
		return FileResult{}, mapErr(err)
	}
	return FileResult{Path: path, FileName: filepath.Base(path)}, nil
}

// errExportOverData refuses an export destination that is one of
// Casebook's own files.
var errExportOverData = uiErr(kindConflict, "", "Casebook cannot export over its own data files. Choose another location.")

// isCasebookFile reports whether path is the live database, one of its
// SQLite side files, or anything inside the backups folder next to it
// (where Restore keeps its safety copies). Writing there would corrupt the
// open database or destroy a safety copy.
func (d *Desktop) isCasebookFile(path string) bool {
	db := d.app.DBPath()
	for _, p := range []string{db, db + "-wal", db + "-shm", db + "-journal"} {
		if samePath(path, p) {
			return true
		}
	}
	return isWithin(path, filepath.Join(filepath.Dir(db), "backups"))
}

// writePrivateFile writes data to path with mode 0600 through a temporary
// file in the same directory (os.CreateTemp creates it 0600) and a rename,
// so a failure part way never leaves a half-written file and never damages
// the file being replaced.
func writePrivateFile(path string, data []byte) (err error) {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".casebook-export-*.tmp")
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tmp.Close()
			os.Remove(tmp.Name())
		}
	}()
	if _, err = tmp.Write(data); err != nil {
		return err
	}
	if err = tmp.Sync(); err != nil {
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// ImportInvestigation asks for an export file and imports it. Files larger
// than app.MaxImportBytes are refused before they are read; the content is
// passed unchanged to the engine, which validates everything.
func (d *Desktop) ImportInvestigation() (ImportOutcome, error) {
	if err := d.ready(); err != nil {
		return ImportOutcome{}, err
	}
	path, err := d.native.OpenFile(runtime.OpenDialogOptions{
		Title:   "Import case",
		Filters: jsonFilter,
	})
	if err != nil {
		return ImportOutcome{}, nativeErr("dialog", err)
	}
	if path == "" {
		return ImportOutcome{Cancelled: true}, nil
	}
	f, err := app.OpenImportFile(path)
	if err != nil {
		return ImportOutcome{}, mapErr(err)
	}
	defer f.Close()
	res, err := d.app.ImportInvestigationFrom(d.ctx, d.actor, f)
	if err != nil {
		return ImportOutcome{}, mapErr(err)
	}
	return ImportOutcome{FileName: filepath.Base(path), Title: res.Title, Result: res}, nil
}

// BackUp asks where to save and writes a consistent backup of all data. An
// existing destination is never overwritten (kind conflict).
func (d *Desktop) BackUp() (BackupOutcome, error) {
	if err := d.ready(); err != nil {
		return BackupOutcome{}, err
	}
	path, err := d.native.SaveFile(runtime.SaveDialogOptions{
		Title:                "Back up all Casebook data",
		DefaultFilename:      backupFileName(d.now()),
		Filters:              dbFilter,
		CanCreateDirectories: true,
	})
	if err != nil {
		return BackupOutcome{}, nativeErr("dialog", err)
	}
	if path == "" {
		return BackupOutcome{Cancelled: true}, nil
	}
	info, err := d.app.Backup(d.ctx, path)
	if err != nil {
		return BackupOutcome{}, mapErr(err)
	}
	return BackupOutcome{Info: info}, nil
}

// PickBackupToRestore is step one of restore (D-07): the user picks a file,
// it is validated read-only, and its path is pinned for RestoreBackup. A
// cancelled or invalid pick clears any earlier pin.
func (d *Desktop) PickBackupToRestore() (RestorePick, error) {
	if err := d.ready(); err != nil {
		return RestorePick{}, err
	}
	d.setPendingRestore("")
	path, err := d.native.OpenFile(runtime.OpenDialogOptions{
		Title:   "Restore from backup",
		Filters: dbFilter,
	})
	if err != nil {
		return RestorePick{}, nativeErr("dialog", err)
	}
	if path == "" {
		return RestorePick{Cancelled: true}, nil
	}
	info, err := d.app.ValidateBackup(d.ctx, path)
	if err != nil {
		return RestorePick{}, mapErr(err)
	}
	d.setPendingRestore(path)
	return RestorePick{Path: path, FileName: filepath.Base(path), Info: info}, nil
}

// RestoreBackup is step two of restore: it replaces all data with the backup
// pinned by the last PickBackupToRestore, after the engine saves a safety
// copy. Any other path is refused, and every call uses up the pin.
func (d *Desktop) RestoreBackup(path string) (app.RestoreResult, error) {
	if err := d.ready(); err != nil {
		return app.RestoreResult{}, err
	}
	d.mu.Lock()
	pinned := d.pendingRestore
	d.pendingRestore = ""
	d.mu.Unlock()
	if pinned == "" || path != pinned {
		return app.RestoreResult{}, mapErr(app.NewValidationError("path", "choose the backup again"))
	}
	res, err := d.app.Restore(d.ctx, d.actor, pinned)
	return res, mapErr(err)
}

func (d *Desktop) setPendingRestore(path string) {
	d.mu.Lock()
	d.pendingRestore = path
	d.mu.Unlock()
}

// exportFileName is the default export file name: the ref plus a slug of the
// title ("INV-0001-september-cloud-bill-up-38.json"), or the ref alone when
// the title has no ASCII letters or digits.
func exportFileName(ref, title string) string {
	if s := slugify(title, 60); s != "" {
		return ref + "-" + s + ".json"
	}
	return ref + ".json"
}

// backupFileName is the default backup file name for the local date of t.
func backupFileName(t time.Time) string {
	return "casebook-backup-" + t.Format("2006-01-02") + ".db"
}

// slugify keeps lowercase ASCII letters and digits, turns every other run of
// characters into one hyphen, trims hyphens at both ends, and cuts the
// result to at most max bytes without a trailing hyphen.
func slugify(s string, max int) string {
	var b strings.Builder
	hyphen := false
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			if hyphen && b.Len() > 0 {
				b.WriteByte('-')
			}
			hyphen = false
			b.WriteRune(r)
			continue
		}
		hyphen = true
	}
	out := b.String()
	if len(out) > max {
		out = strings.TrimRight(out[:max], "-")
	}
	return out
}
