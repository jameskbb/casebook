package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/jameskbb/casebook/internal/domain"
	"github.com/jameskbb/casebook/internal/paths"
	"github.com/jameskbb/casebook/internal/store"
)

// safetyCopyLayout is the UTC timestamp in safety copy names.
const safetyCopyLayout = "20060102T150405Z"

// maxSafetyCopyAttempts bounds the -2, -3 ... suffix search.
const maxSafetyCopyAttempts = 1000

// Restore replaces the whole database with the backup at srcPath (D-22,
// DATA-04), safely while other Apps and processes keep the database open:
//
//  1. ValidateBackup checks the source first; any failure (corrupt, foreign,
//     tampered or newer backup) returns before anything is written.
//  2. The source is copied into a private 0600 temp file in
//     <datadir>/backups, the copy is validated again, and it is migrated
//     forward there if it is older.
//  3. A safety copy of the current database is written with VACUUM INTO to
//     <datadir>/backups/casebook-pre-restore-<UTC timestamp>.db (-2, -3 ...
//     appended when the name exists).
//  4. Inside one IMMEDIATE transaction on the live database every row is
//     replaced with the copy's rows, the search index is rebuilt and a
//     global restore activity row is recorded. Other connections see the old
//     or the new contents only; the file itself is never swapped.
//
// The temp copy is always removed. Restoring the live database onto itself
// is ErrConflict. The result's Counts are read after the restore (so
// Activity includes the restore row).
func (a *App) Restore(ctx context.Context, actor Actor, srcPath string) (RestoreResult, error) {
	const op = "restore"
	actor, err := actor.Validate()
	if err != nil {
		return RestoreResult{}, a.fail(op, err)
	}
	if strings.TrimSpace(srcPath) == "" {
		return RestoreResult{}, domain.NewValidationError("path", "is required")
	}
	abs, err := filepath.Abs(srcPath)
	if err != nil {
		return RestoreResult{}, domain.NewValidationError("path", "is not a usable file path")
	}
	if a.isLiveDB(abs) {
		return RestoreResult{}, fmt.Errorf("%w: cannot restore the live database onto itself", domain.ErrConflict)
	}
	if _, err := a.ValidateBackup(ctx, abs); err != nil {
		return RestoreResult{}, err
	}

	backupsDir := filepath.Join(filepath.Dir(a.DBPath()), "backups")
	if err := paths.EnsureDir(backupsDir); err != nil {
		return RestoreResult{}, fmt.Errorf("create backups directory: %w", err)
	}
	tmp, err := copyToTemp(abs, backupsDir)
	if tmp != "" {
		defer removeWithSidecars(tmp)
	}
	if err != nil {
		return RestoreResult{}, fmt.Errorf("copy backup %s: %w", filepath.Base(abs), unwrapPath(err))
	}
	// The private copy is what gets restored: validate it too, so a source
	// file changed between the first check and the copy is still refused.
	if _, err := a.ValidateBackup(ctx, tmp); err != nil {
		return RestoreResult{}, err
	}
	from, _, err := store.MigrateFile(ctx, tmp, nil)
	if err != nil {
		return RestoreResult{}, a.restoreFailure(op, err)
	}

	safety, err := a.writeSafetyCopy(ctx, backupsDir)
	if err != nil {
		return RestoreResult{}, err
	}

	summary := fmt.Sprintf("Restored backup %s (safety copy %s)", filepath.Base(abs), filepath.Base(safety))
	err = a.db.ReplaceAll(ctx, tmp, func(tx *sql.Tx) error {
		return a.recordActivity(ctx, tx, actor, 0, op, "", summary, "", a.timestamp())
	})
	if err != nil {
		return RestoreResult{}, a.fail(op, err)
	}

	res := RestoreResult{SourcePath: abs, SafetyCopyPath: safety, SourceSchemaVersion: from}
	err = a.db.WithReadTx(ctx, func(tx *sql.Tx) error {
		c, err := store.CountAll(ctx, tx)
		res.Counts = c
		return err
	})
	if err != nil {
		return RestoreResult{}, a.fail(op, err)
	}
	a.log.Info(op, "source", filepath.Base(abs), "safety_copy", filepath.Base(safety), "from_version", from)
	return res, nil
}

// restoreFailure maps a MigrateFile failure: a snapshot problem is the usual
// invalid backup ErrConflict; anything else goes through fail.
func (a *App) restoreFailure(op string, err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	if reason := store.SnapshotReason(err); reason != "" {
		return invalidBackup(reason)
	}
	a.log.Warn(op, "stage", "migrate", "err", err)
	return invalidBackup("cannot upgrade the backup to this Casebook version")
}

// writeSafetyCopy snapshots the live database into backupsDir as
// casebook-pre-restore-<UTC timestamp>.db, appending -2, -3 ... while the
// name is taken. It returns the path.
func (a *App) writeSafetyCopy(ctx context.Context, backupsDir string) (string, error) {
	base := "casebook-pre-restore-" + a.now().UTC().Format(safetyCopyLayout)
	p, err := writeSnapshot(ctx, a.db, backupsDir, base)
	if err == nil {
		return p, nil
	}
	var pe *fs.PathError
	switch {
	case errors.Is(err, errTooManyCopies):
		return "", fmt.Errorf("%w: too many safety copies named %s", domain.ErrConflict, base)
	case errors.As(err, &pe):
		return "", fmt.Errorf("write safety copy %s: %w", filepath.Base(p), pe.Err)
	}
	return "", a.fail("restore.safety_copy", err)
}

// errTooManyCopies means every name writeSnapshot tried was taken.
var errTooManyCopies = errors.New("too many copies with the same name")

// writeSnapshot writes a VACUUM INTO snapshot of db into backupsDir as
// <base>.db, or <base>-2.db, <base>-3.db ... while the name is taken (by an
// earlier copy or a concurrent writer: VacuumInto never overwrites). It
// returns the path written, or on failure the path it tried last, with the
// raw error (errTooManyCopies when every name was taken).
func writeSnapshot(ctx context.Context, db *store.DB, backupsDir, base string) (string, error) {
	var p string
	for i := 1; i <= maxSafetyCopyAttempts; i++ {
		name := base
		if i > 1 {
			name += fmt.Sprintf("-%d", i)
		}
		p = filepath.Join(backupsDir, name+".db")
		err := db.VacuumInto(ctx, p)
		if errors.Is(err, store.ErrDestinationExists) {
			continue
		}
		return p, err
	}
	return p, errTooManyCopies
}

// copyToTemp copies src into a new 0600 temp file in dir, fsyncs it and
// returns its path (also on a copy failure, so the caller can remove it).
func copyToTemp(src, dir string) (string, error) {
	in, err := os.Open(src)
	if err != nil {
		return "", err
	}
	defer in.Close()
	out, err := os.CreateTemp(dir, "restore-*.db")
	if err != nil {
		return "", err
	}
	name := out.Name()
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return name, err
	}
	if err := out.Sync(); err != nil {
		out.Close()
		return name, err
	}
	return name, out.Close()
}

// removeWithSidecars removes p and any -journal, -wal or -shm files next to
// it.
func removeWithSidecars(p string) {
	for _, s := range []string{"", "-journal", "-wal", "-shm"} {
		os.Remove(p + s)
	}
}

// unwrapPath drops the full path from a *fs.PathError.
func unwrapPath(err error) error {
	var pe *fs.PathError
	if errors.As(err, &pe) {
		return pe.Err
	}
	return err
}
