package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/jameskbb/casebook/internal/domain"
	"github.com/jameskbb/casebook/internal/store"
)

// The active investigation (ADR-0008) is the settings row
// active_investigation_id, shared by every App and process on the database
// file. Methods that take an invRef treat "" as the active investigation.
//
// Archiving the active investigation leaves it active: archiving is about
// list visibility, while the active investigation is what surfaces default
// to, and the user may still be wrapping it up. Clear or change it explicitly.

// GetActiveInvestigation returns the active investigation as this App
// resolves it (the directory binding of its working directory, else the
// global setting; ADR-0015), or nil, nil when none is set (or the setting
// points at an investigation that no longer exists).
func (a *App) GetActiveInvestigation(ctx context.Context) (*Investigation, error) {
	var out *Investigation
	err := a.db.WithReadTx(ctx, func(tx *sql.Tx) error {
		id, ok, _, err := a.activeID(ctx, tx)
		if err != nil || !ok {
			return err
		}
		inv, err := a.getInvestigationByID(ctx, tx, id)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		out = &inv
		return nil
	})
	if err != nil {
		return nil, a.fail("active.get", err)
	}
	return out, nil
}

// SetActiveInvestigation makes invRef the active investigation where this
// App works and records the change bound to it. When a directory binding
// applies to the App's working directory (ADR-0015), that binding is
// re-pointed (active.bind); otherwise the global setting changes for every
// surface (active.set). invRef is required here (an empty ref is a
// ValidationError on investigation_ref). Setting the already active
// investigation is a no-op with no activity. The result has IsActive true.
func (a *App) SetActiveInvestigation(ctx context.Context, actor Actor, invRef string) (Investigation, error) {
	return a.setActive(ctx, actor, invRef, false)
}

// SetGlobalActiveInvestigation is SetActiveInvestigation that always sets
// the global setting, even where a directory binding applies (the binding
// still wins there).
func (a *App) SetGlobalActiveInvestigation(ctx context.Context, actor Actor, invRef string) (Investigation, error) {
	return a.setActive(ctx, actor, invRef, true)
}

func (a *App) setActive(ctx context.Context, actor Actor, invRef string, global bool) (Investigation, error) {
	const op = "active.set"
	actor, err := actor.Validate()
	if err != nil {
		return Investigation{}, a.fail(op, err)
	}
	if strings.TrimSpace(invRef) == "" {
		return Investigation{}, a.fail(op, domain.NewValidationError("investigation_ref", "is required"))
	}
	var out Investigation
	err = a.db.WithWriteTx(ctx, func(tx *sql.Tx) error {
		now := a.timestamp()
		inv, err := a.resolveInvestigation(ctx, tx, invRef)
		if err != nil {
			return err
		}
		if err := a.makeActiveTx(ctx, tx, actor, inv, global, now); err != nil {
			return err
		}
		got, err := a.getInvestigationByID(ctx, tx, inv.ID)
		if err != nil {
			return err
		}
		out = got
		return nil
	})
	if err != nil {
		return Investigation{}, a.fail(op, err)
	}
	a.log.Debug(op, "ref", out.Ref)
	return out, nil
}

// makeActiveTx makes inv active inside tx: it re-points the directory
// binding that applies to the App's working directory (active.bind) unless
// global is set or none applies, and otherwise sets the global setting
// (active.set). Making the already active investigation active writes
// nothing.
func (a *App) makeActiveTx(ctx context.Context, tx *sql.Tx, actor domain.Actor, inv domain.Investigation, global bool, now string) error {
	if !global {
		_, _, binding, err := a.activeID(ctx, tx)
		if err != nil {
			return err
		}
		if binding != nil {
			if binding.InvestigationID == inv.ID {
				return nil
			}
			if err := store.SetDirectoryBinding(ctx, tx, binding.Dir, inv.ID, now); err != nil {
				return err
			}
			return a.recordActivity(ctx, tx, actor, inv.ID, "active.bind", inv.Ref,
				fmt.Sprintf("Made %s the active investigation for %s", inv.Ref, binding.Dir), "", now)
		}
	}
	if gid, ok, err := store.GetActiveInvestigationID(ctx, tx); err != nil {
		return err
	} else if ok && gid == inv.ID {
		return nil
	}
	if err := store.SetActiveInvestigationID(ctx, tx, inv.ID, now); err != nil {
		return err
	}
	return a.recordActivity(ctx, tx, actor, inv.ID, "active.set", inv.Ref,
		fmt.Sprintf("Set active investigation to %s", inv.Ref), "", now)
}

// ClearActiveInvestigation unsets the active investigation and records a
// global active.clear row (no investigation_ref; target_ref names the
// previously active investigation when it still exists). It is a silent
// no-op when none is set.
func (a *App) ClearActiveInvestigation(ctx context.Context, actor Actor) error {
	_, err := a.clearActive(ctx, actor, "")
	return err
}

// ClearActiveInvestigationIf clears the active investigation only while it
// is still invRef, checking and clearing in one write transaction, so a
// surface showing a stale "Active" state never clears a choice another
// client made since. It reports whether it cleared; false (and no activity
// row) when another investigation, or none, is active. invRef must name an
// existing investigation (ErrNotFound otherwise); a blank invRef is a
// ValidationError.
func (a *App) ClearActiveInvestigationIf(ctx context.Context, actor Actor, invRef string) (bool, error) {
	if strings.TrimSpace(invRef) == "" {
		return false, a.fail("active.clear", domain.NewValidationError("investigation_ref", "is required"))
	}
	return a.clearActive(ctx, actor, invRef)
}

// clearActive clears the active investigation; with expected set, only
// while that investigation is the active one.
func (a *App) clearActive(ctx context.Context, actor Actor, expected string) (bool, error) {
	const op = "active.clear"
	actor, err := actor.Validate()
	if err != nil {
		return false, a.fail(op, err)
	}
	cleared := false
	err = a.db.WithWriteTx(ctx, func(tx *sql.Tx) error {
		now := a.timestamp()
		var expectedID int64
		if expected != "" {
			inv, err := a.resolveInvestigation(ctx, tx, expected)
			if err != nil {
				return err
			}
			expectedID = inv.ID
		}
		id, ok, err := store.GetActiveInvestigationID(ctx, tx)
		if err != nil {
			return err
		}
		if expected != "" && (!ok || id != expectedID) {
			return nil
		}
		prevRef := ""
		if ok {
			inv, err := a.getInvestigationByID(ctx, tx, id)
			switch {
			case err == nil:
				prevRef = inv.Ref
			case !errors.Is(err, sql.ErrNoRows):
				return err
			}
		}
		done, err := store.ClearActiveInvestigation(ctx, tx)
		if err != nil || !done {
			return err
		}
		cleared = true
		summary := "Cleared active investigation"
		if prevRef != "" {
			summary = fmt.Sprintf("Cleared active investigation (was %s)", prevRef)
		}
		return a.recordActivity(ctx, tx, actor, 0, op, prevRef, summary, "", now)
	})
	if err != nil {
		return false, a.fail(op, err)
	}
	a.log.Debug(op, "cleared", cleared)
	return cleared, nil
}
