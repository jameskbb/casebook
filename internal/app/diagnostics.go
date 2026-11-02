package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/jameskbb/casebook/internal/domain"
	"github.com/jameskbb/casebook/internal/store"
)

// maxIntegrityProblems caps how many problems CheckIntegrity reports.
const maxIntegrityProblems = 10

// ChangeToken returns a cheap token that changes whenever the database
// changes, for surfaces that poll for external writes (D-17, STORE-07). Equal
// tokens from the same App mean nothing changed; tokens from different Apps
// are not comparable.
//
// The token is "<data_version>.<max activity id>". PRAGMA data_version is read
// outside any transaction on one dedicated long-lived connection, so it moves
// when any other connection (another App, process or tool) commits.
// max(activity.id) also catches this App's own writes. It uses
// max(activity.id) rather than max(updated_at) (as STORE-07 first proposed)
// because every mutation writes an activity row in the same transaction,
// while not every mutation touches an updated_at column. ChangeToken runs two
// tiny queries and never opens a transaction.
func (a *App) ChangeToken(ctx context.Context) (string, error) {
	const op = "change_token"
	a.tokenMu.Lock()
	defer a.tokenMu.Unlock()
	if a.closed {
		return "", a.fail(op, sql.ErrConnDone)
	}
	if a.tokenConn == nil {
		c, err := a.db.OpenTokenConn(ctx)
		if err != nil {
			return "", a.fail(op, err)
		}
		a.tokenConn = c
	}
	dv, err := store.DataVersion(ctx, a.tokenConn)
	if err == nil {
		var maxID int64
		maxID, err = store.MaxActivityID(ctx, a.tokenConn)
		if err == nil {
			return fmt.Sprintf("%d.%d", dv, maxID), nil
		}
	}
	// Drop a failed connection so the next call starts on a fresh one.
	a.tokenConn.Close()
	a.tokenConn = nil
	return "", a.fail(op, err)
}

// Status reports the database path, data directory, schema version, active
// investigation ref ("" when none) and row counts from one read snapshot.
func (a *App) Status(ctx context.Context) (StatusInfo, error) {
	out := StatusInfo{DBPath: a.DBPath(), DataDir: filepath.Dir(a.DBPath())}
	err := a.db.WithReadTx(ctx, func(tx *sql.Tx) error {
		v, err := store.SchemaVersion(ctx, tx)
		if err != nil {
			return err
		}
		out.SchemaVersion = v
		if id, ok, binding, err := a.activeID(ctx, tx); err != nil {
			return err
		} else if ok {
			if binding != nil {
				out.ActiveDir = binding.Dir
			}
			inv, err := a.getInvestigationByID(ctx, tx, id)
			switch {
			case err == nil:
				out.ActiveInvestigationRef = inv.Ref
			case !errors.Is(err, sql.ErrNoRows):
				return err
			}
		}
		c, err := store.CountAll(ctx, tx)
		if err != nil {
			return err
		}
		out.Counts = c
		return nil
	})
	if err != nil {
		return StatusInfo{}, a.fail("status", err)
	}
	return out, nil
}

// CheckIntegrity runs PRAGMA integrity_check and PRAGMA foreign_key_check. It
// returns nil when both are clean, else an *IntegrityError (which wraps
// ErrInternal) listing up to the first 10 problems as sanitized lines naming
// the check, tables, indexes and row ids only (never raw SQLite text or
// column values), with Total counting every problem. Test with
// errors.Is(err, ErrInternal) or IsIntegrity(err).
func (a *App) CheckIntegrity(ctx context.Context) error {
	var problems []string
	err := a.db.WithReadTx(ctx, func(tx *sql.Tx) error {
		p, err := store.IntegrityProblems(ctx, tx)
		problems = p
		return err
	})
	if err != nil {
		return a.fail("integrity_check", err)
	}
	if len(problems) == 0 {
		return nil
	}
	a.log.Warn("integrity_check", "problems", len(problems))
	shown := problems
	if len(shown) > maxIntegrityProblems {
		shown = shown[:maxIntegrityProblems:maxIntegrityProblems]
	}
	return &domain.IntegrityError{Problems: append([]string{}, shown...), Total: len(problems)}
}
