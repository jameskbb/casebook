package app

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/jameskbb/casebook/internal/paths"
	"github.com/jameskbb/casebook/internal/store"
)

// PreUpgradePrefix starts the name of every automatic pre-upgrade backup:
// casebook-pre-upgrade-schema<old version>-<UTC timestamp>.db in the backups
// folder next to the database.
const PreUpgradePrefix = "casebook-pre-upgrade-"

// errPreUpgrade starts the message of a failed pre-upgrade backup.
const errPreUpgrade = "cannot back up the database before upgrading it to this Casebook version, so it was not upgraded (nothing was changed)"

// preUpgradeBackup returns the store.Config BeforeUpgrade hook: before Open
// applies pending migrations to an existing database at schema version
// from, it writes a VACUUM INTO snapshot to
// <dir of the database>/backups/casebook-pre-upgrade-schema<from>-<UTC
// timestamp>.db (-2, -3 ... appended when the name is taken). An older
// Casebook can restore that copy after an upgrade. Any failure refuses the
// upgrade, so the file keeps its old schema. Concurrent first opens may each
// write a copy; the names never collide.
func preUpgradeBackup(log *slog.Logger, now func() time.Time) func(context.Context, *store.DB, int) error {
	return func(ctx context.Context, db *store.DB, from int) error {
		const op = "app.open.pre_upgrade_backup"
		dir := filepath.Join(filepath.Dir(db.Path()), "backups")
		if err := paths.EnsureDir(dir); err != nil {
			return fmt.Errorf("%s: create backups folder %s: %w", errPreUpgrade, dir, unwrapPath(err))
		}
		base := fmt.Sprintf("%sschema%d-%s", PreUpgradePrefix, from, now().UTC().Format(safetyCopyLayout))
		p, err := writeSnapshot(ctx, db, dir, base)
		var pe *fs.PathError
		switch {
		case err == nil:
			log.Info(op, "from_version", from, "to_version", store.LatestVersion(), "copy", filepath.Base(p))
			return nil
		case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
			return err
		case errors.Is(err, errTooManyCopies):
			return fmt.Errorf("%w: %s: too many copies named %s in %s", ErrConflict, errPreUpgrade, base, dir)
		case errors.As(err, &pe):
			return fmt.Errorf("%s: write %s: %w", errPreUpgrade, filepath.Join(dir, filepath.Base(p)), pe.Err)
		case store.IsBusy(err):
			return ErrBusy
		}
		// Raw SQLite text never leaves internal/app.
		log.Warn(op, "path", db.Path(), "err", err)
		return fmt.Errorf("%w: %s", ErrInternal, errPreUpgrade)
	}
}
