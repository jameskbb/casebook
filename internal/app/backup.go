package app

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/jameskbb/casebook/internal/domain"
	"github.com/jameskbb/casebook/internal/store"
)

// Backup writes a consistent snapshot of the whole database to destPath with
// SQLite's VACUUM INTO (D-22, DATA-03). It works while other connections and
// processes keep writing. The file is created owner-only (0600 where the OS
// supports it) and an existing file is never overwritten.
//
// Errors: an empty path or a missing parent directory is a ValidationError on
// path; an existing destination or the live database path is ErrConflict.
// The new file is then checked with ValidateBackup; the result's CreatedAt is
// the App clock time. Backup takes no actor and records no activity row: it
// does not change Casebook data.
func (a *App) Backup(ctx context.Context, destPath string) (BackupInfo, error) {
	const op = "backup"
	if strings.TrimSpace(destPath) == "" {
		return BackupInfo{}, domain.NewValidationError("path", "is required")
	}
	abs, err := filepath.Abs(destPath)
	if err != nil {
		return BackupInfo{}, domain.NewValidationError("path", "is not a usable file path")
	}
	if a.isLiveDB(abs) {
		return BackupInfo{}, fmt.Errorf("%w: cannot write a backup over the live database", domain.ErrConflict)
	}
	if fi, err := os.Stat(filepath.Dir(abs)); err != nil || !fi.IsDir() {
		return BackupInfo{}, domain.NewValidationError("path", "parent directory does not exist")
	}
	if err := a.db.VacuumInto(ctx, abs); err != nil {
		if errors.Is(err, store.ErrDestinationExists) {
			return BackupInfo{}, fmt.Errorf("%w: backup destination %s already exists", domain.ErrConflict, filepath.Base(abs))
		}
		var pe *fs.PathError
		if errors.As(err, &pe) {
			// Filesystem problems (permission denied, read-only volume) are
			// actionable by the user; report them without the full path.
			return BackupInfo{}, fmt.Errorf("create backup %s: %w", filepath.Base(abs), pe.Err)
		}
		return BackupInfo{}, a.fail(op, err)
	}
	info, err := a.ValidateBackup(ctx, abs)
	if err != nil {
		os.Remove(abs)
		return BackupInfo{}, err
	}
	info.CreatedAt = a.timestamp()
	a.log.Debug(op, "file", filepath.Base(abs), "bytes", info.SizeBytes)
	return info, nil
}

// ValidateBackup checks that path is a genuine, untampered Casebook backup
// without modifying it: it opens the file read-only and immutable (no -wal or
// -shm files appear), requires PRAGMA integrity_check ok, no foreign key
// violations, a schema version between 1 and this build's version, a schema
// identical to the one the embedded migrations produce at that version (so
// extra triggers or views are refused), and data within Casebook limits.
//
// A missing file is ErrNotFound. Any other failure, including a file that is
// not a database at all, is ErrConflict "not a valid Casebook backup
// (<short reason>)"; the reason names tables or columns only, never raw
// SQLite text or file contents. On success CreatedAt is the file's
// modification time.
func (a *App) ValidateBackup(ctx context.Context, path string) (BackupInfo, error) {
	const op = "backup.validate"
	if strings.TrimSpace(path) == "" {
		return BackupInfo{}, domain.NewValidationError("path", "is required")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return BackupInfo{}, domain.NewValidationError("path", "is not a usable file path")
	}
	fi, err := os.Stat(abs)
	if errors.Is(err, fs.ErrNotExist) {
		return BackupInfo{}, fmt.Errorf("%w: backup file %s", domain.ErrNotFound, filepath.Base(abs))
	}
	if err != nil {
		var pe *fs.PathError
		if errors.As(err, &pe) {
			return BackupInfo{}, fmt.Errorf("read backup %s: %w", filepath.Base(abs), pe.Err)
		}
		return BackupInfo{}, a.fail(op, err)
	}
	if !fi.Mode().IsRegular() {
		return BackupInfo{}, invalidBackup("not a regular file")
	}

	db, err := store.OpenReadOnly(abs)
	if err != nil {
		a.log.Warn(op, "file", filepath.Base(abs), "err", err)
		return BackupInfo{}, invalidBackup("cannot open the file")
	}
	version, counts, verr := store.ValidateSnapshot(ctx, db, store.LatestVersion())
	cerr := db.Close()
	if verr != nil {
		if errors.Is(verr, context.Canceled) || errors.Is(verr, context.DeadlineExceeded) {
			return BackupInfo{}, verr
		}
		reason := store.SnapshotReason(verr)
		if reason == "" {
			a.log.Warn(op, "file", filepath.Base(abs), "err", verr)
			reason = "validation failed"
		}
		return BackupInfo{}, invalidBackup(reason)
	}
	if cerr != nil {
		a.log.Warn(op, "file", filepath.Base(abs), "err", cerr)
	}
	return BackupInfo{
		Path:          abs,
		SizeBytes:     fi.Size(),
		SchemaVersion: version,
		CreatedAt:     domain.FormatTime(fi.ModTime()),
		Counts:        counts,
	}, nil
}

// invalidBackup is the ErrConflict returned for a file that is not a valid
// Casebook backup.
func invalidBackup(reason string) error {
	return fmt.Errorf("%w: not a valid Casebook backup (%s)", domain.ErrConflict, reason)
}

// isLiveDB reports whether abs is the open database file (or one of its
// -wal, -shm or -journal companions).
func (a *App) isLiveDB(abs string) bool {
	live := a.DBPath()
	for _, p := range []string{live, live + "-wal", live + "-shm", live + "-journal"} {
		if abs == p {
			return true
		}
	}
	fi, err := os.Stat(abs)
	if err != nil {
		return false
	}
	li, err := os.Stat(live)
	return err == nil && os.SameFile(fi, li)
}
