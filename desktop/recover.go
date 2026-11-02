package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/jameskbb/casebook/internal/app"
)

// Recovery from the startup error screen (AR-11). When the database failed
// to open, the screen offers Open data folder, Copy path and Restore from
// backup. None of these needs an open engine:
//
//   - OpenDataFolder shows the folder that holds the data file;
//   - CopyDataLocation copies the data file path;
//   - PickBackupToRecover and RecoverFromBackup restore a backup: the
//     backup is validated through a scratch engine in a temporary folder,
//     the data file that failed to open is moved aside (never deleted) into
//     the backups folder next to it, a new database is opened in its place
//     and app.Restore fills it from the backup. On any failure the moved
//     file is put back.
//
// Recovery is offered only for the two failures a backup fixes: a file
// SQLite cannot open as a Casebook database (app.ErrInternal) and a database
// saved by a newer Casebook (app.ErrSchemaTooNew). Updating Casebook is the
// usual fix for the second, but a person going back to this version restores
// the pre-upgrade backup the newer version wrote, and the newer file is moved
// aside like any other (docs/updating.md). Lock contention, permissions,
// a full disk or an unknown data path are not fixed by a restore.
//
// Recovery is refused while the engine is open, and while another program
// holds the data file: both steps ask app.CheckNotInUse at the time of the
// call, not only what the startup error said. A file another program has
// open (a newer Casebook's MCP server, say) must not be moved aside under
// it.

// errRecoverOpen refuses recovery once the database is open.
var errRecoverOpen = uiErr(kindConflict, "", "Casebook already has its data open. Use Restore from backup in the Data menu.")

// errRecoverNotAllowed refuses recovery for errors a restore does not fix.
var errRecoverNotAllowed = uiErr(kindConflict, "", "A backup cannot fix this problem. Close Casebook and try again.")

// errRecoverSameFile refuses restoring the file that failed to open.
var errRecoverSameFile = uiErr(kindConflict, "", "That is the data file Casebook could not open. Choose a backup file instead.")

// errRecoverInUse refuses recovery while another program has the data file
// open.
var errRecoverInUse = uiErr(kindConflict, "", "Another program is using the data file, such as an AI client running Casebook. Close it, then try again.")

// recoverable reports whether a restore can replace a database that failed
// to open with err: one SQLite could not open as a Casebook database, or one
// from a newer Casebook. ErrBusy never wraps ErrInternal.
func recoverable(err error) bool {
	return errors.Is(err, app.ErrInternal) || errors.Is(err, app.ErrSchemaTooNew)
}

// OpenDataFolder opens the folder that holds the data file in the system
// file manager. It needs no open database.
func (d *Desktop) OpenDataFolder() error {
	if d.dbPath == "" {
		return uiErr(kindOther, "folder", "Casebook does not know where its data folder is")
	}
	return nativeErr("folder", d.native.OpenFolder(filepath.Dir(d.dbPath)))
}

// CopyDataLocation copies the data file path to the clipboard. It needs no
// open database.
func (d *Desktop) CopyDataLocation() error {
	if d.dbPath == "" {
		return uiErr(kindOther, "clipboard", "Casebook does not know where its data file is")
	}
	return nativeErr("clipboard", d.native.SetClipboard(d.dbPath))
}

// recoveryAllowed checks the engine state for both recovery steps, and
// checks now that no other program holds the data file.
func (d *Desktop) recoveryAllowed() error {
	d.mu.Lock()
	open := d.app != nil
	openErr := d.openErr
	d.mu.Unlock()
	if open {
		return errRecoverOpen
	}
	if !recoverable(openErr) || d.dbPath == "" {
		return errRecoverNotAllowed
	}
	if err := app.CheckNotInUse(d.ctx, d.dbPath); errors.Is(err, app.ErrBusy) {
		return errRecoverInUse
	} else if err != nil {
		return mapErr(err)
	}
	return nil
}

// PickBackupToRecover is step one of recovery: the user picks a backup, it
// is validated read-only through a scratch engine, and its path is pinned
// for RecoverFromBackup.
func (d *Desktop) PickBackupToRecover() (RestorePick, error) {
	if err := d.recoveryAllowed(); err != nil {
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
	if samePath(path, d.dbPath) {
		return RestorePick{}, errRecoverSameFile
	}
	info, err := validateWithScratch(d, path)
	if err != nil {
		return RestorePick{}, mapErr(err)
	}
	d.setPendingRestore(path)
	return RestorePick{Path: path, FileName: filepath.Base(path), Info: info}, nil
}

// validateWithScratch validates a backup through an engine opened on an
// empty database in a temporary folder, which is removed afterwards.
func validateWithScratch(d *Desktop, path string) (app.BackupInfo, error) {
	dir, err := os.MkdirTemp("", "casebook-recover-*")
	if err != nil {
		return app.BackupInfo{}, err
	}
	defer os.RemoveAll(dir)
	scratch, err := app.Open(filepath.Join(dir, "scratch.db"))
	if err != nil {
		return app.BackupInfo{}, err
	}
	defer scratch.Close()
	return scratch.ValidateBackup(d.ctx, path)
}

// RecoverFromBackup is step two of recovery: it moves the data file that
// failed to open aside, opens a new database in its place and restores the
// backup pinned by the last PickBackupToRecover into it. On success the
// engine is open (StartupState reports ok) and SafetyCopyPath names the
// moved file. Every call uses up the pin.
func (d *Desktop) RecoverFromBackup(path string) (app.RestoreResult, error) {
	if err := d.recoveryAllowed(); err != nil {
		return app.RestoreResult{}, err
	}
	d.mu.Lock()
	pinned := d.pendingRestore
	d.pendingRestore = ""
	d.mu.Unlock()
	if pinned == "" || path != pinned {
		return app.RestoreResult{}, mapErr(app.NewValidationError("path", "choose the backup again"))
	}
	moved, err := moveAside(d.dbPath, d.now())
	if err != nil {
		return app.RestoreResult{}, uiErr(kindOther, "", fmt.Sprintf("Casebook could not move the damaged data file aside: %v", err))
	}
	putBack := func() {
		if moved != "" {
			removeDBFiles(d.dbPath)
			_ = moveFiles(moved, d.dbPath)
		}
	}
	a, err := app.Open(d.dbPath)
	if err != nil {
		putBack()
		return app.RestoreResult{}, mapErr(err)
	}
	res, err := a.Restore(d.ctx, d.actor, pinned)
	if err != nil {
		_ = a.Close()
		putBack()
		return app.RestoreResult{}, mapErr(err)
	}
	if moved != "" {
		res.SafetyCopyPath = moved
	}
	d.mu.Lock()
	d.app = a
	d.openErr = nil
	d.mu.Unlock()
	return res, nil
}

// dbSuffixes are the SQLite files that belong to one database.
var dbSuffixes = []string{"", "-wal", "-shm", "-journal"}

// moveAside moves the database at dbPath and its side files into the
// backups folder next to it as casebook-unopened-<UTC timestamp>.db (with
// -2, -3 ... when taken). It returns "" when there was no file to move.
func moveAside(dbPath string, now time.Time) (string, error) {
	if _, err := os.Lstat(dbPath); errors.Is(err, os.ErrNotExist) {
		return "", nil
	} else if err != nil {
		return "", err
	}
	dir := filepath.Join(filepath.Dir(dbPath), "backups")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	base := "casebook-unopened-" + now.UTC().Format("20060102T150405Z")
	for i := 1; i <= 1000; i++ {
		name := base
		if i > 1 {
			name = fmt.Sprintf("%s-%d", base, i)
		}
		dest := filepath.Join(dir, name+".db")
		if anyExists(dest) {
			continue
		}
		if err := moveFiles(dbPath, dest); err != nil {
			return "", err
		}
		return dest, nil
	}
	return "", errors.New("too many earlier copies in the backups folder")
}

// anyExists reports whether p or one of its side files exists.
func anyExists(p string) bool {
	for _, s := range dbSuffixes {
		if _, err := os.Lstat(p + s); err == nil {
			return true
		}
	}
	return false
}

// moveFiles renames from (and each side file that exists) to to. When a
// rename fails, the files already moved are moved back.
func moveFiles(from, to string) error {
	var done []string
	for _, s := range dbSuffixes {
		if _, err := os.Lstat(from + s); errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err := os.Rename(from+s, to+s); err != nil {
			for _, d := range done {
				_ = os.Rename(to+d, from+d)
			}
			return err
		}
		done = append(done, s)
	}
	return nil
}

// removeDBFiles removes a database and its side files (the new database
// recovery created, before the moved file is put back).
func removeDBFiles(p string) {
	for _, s := range dbSuffixes {
		_ = os.Remove(p + s)
	}
}
