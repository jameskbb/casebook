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

// Deleting (AR-10). Archive stays the normal way to put an investigation
// away; delete exists for mistakes (a test investigation, a mis-pasted
// record). Surfaces ask for confirmation; the engine only enforces the
// guards and records the deletion:
//
//   - an investigation must be archived first; deleting it removes every
//     record, link and activity row it holds and clears it as the active
//     investigation, and one global investigation.delete row remains;
//   - a record is refused (ErrConflict) while another record depends on it:
//     evidence while any claim or hypothesis links it, a claim or hypothesis
//     while another one names it in superseded_by. A claim's or hypothesis's
//     own evidence links go with it.
//
// Refs are never reused: counters only grow, so a deleted E-003 stays
// unassigned.

// deleteSummaryTitle bounds the title copied into an investigation.delete
// or evidence.delete summary.
const deleteSummaryTitle = 120

// DeleteInvestigation permanently deletes an archived investigation and
// everything in it, and records one global investigation.delete row
// ("Deleted INV-0003 "Title" (4 evidence, 3 claims, 2 hypotheses, 1
// questions)"). An investigation that is not archived is ErrConflict;
// invRef is required (blank is a ValidationError on investigation_ref, so
// a missing active investigation is never deleted by accident). If it was
// the active investigation, the setting is cleared in the same transaction.
func (a *App) DeleteInvestigation(ctx context.Context, actor Actor, invRef string) error {
	const op = "investigation.delete"
	actor, err := actor.Validate()
	if err != nil {
		return a.fail(op, err)
	}
	if strings.TrimSpace(invRef) == "" {
		return a.fail(op, domain.NewValidationError("investigation_ref", "is required to delete an investigation"))
	}
	err = a.db.WithWriteTx(ctx, func(tx *sql.Tx) error {
		now := a.timestamp()
		inv, err := a.resolveInvestigation(ctx, tx, invRef)
		if err != nil {
			return err
		}
		if !inv.Archived {
			return fmt.Errorf("%w: %s is not archived; archive it before deleting it", domain.ErrConflict, inv.Ref)
		}
		counts, err := store.InvestigationCounts(ctx, tx, inv.ID)
		if err != nil {
			return err
		}
		if activeID, ok, err := store.GetActiveInvestigationID(ctx, tx); err != nil {
			return err
		} else if ok && activeID == inv.ID {
			if _, err := store.ClearActiveInvestigation(ctx, tx); err != nil {
				return err
			}
		}
		if err := store.DeleteInvestigation(ctx, tx, inv.ID); err != nil {
			return err
		}
		summary := fmt.Sprintf("Deleted %s %q (%d evidence, %d claims, %d hypotheses, %d questions)", inv.Ref,
			domain.TruncateLine(inv.Title, deleteSummaryTitle), counts.Evidence, counts.Claims, counts.Hypotheses,
			counts.Questions)
		return a.recordActivity(ctx, tx, actor, 0, op, inv.Ref, summary, "", now)
	})
	if err != nil {
		return a.fail(op, err)
	}
	a.log.Debug(op, "ref", strings.ToUpper(strings.TrimSpace(invRef)))
	return nil
}

// DeleteEvidence permanently deletes one evidence record and records
// evidence.delete ("Deleted evidence E-002 "Title""). It is ErrConflict while
// any claim or hypothesis links it: unlink it first.
func (a *App) DeleteEvidence(ctx context.Context, actor Actor, invRef, evidenceRef string) error {
	const op = "evidence.delete"
	return a.deleteRecord(ctx, actor, op, func(tx *sql.Tx) (recordTarget, string, error) {
		t, err := a.resolveRecord(ctx, tx, invRef, evidenceRef, domain.RecordEvidence)
		if err != nil {
			return t, "", err
		}
		ev, err := store.GetEvidence(ctx, tx, t.Inv.ID, t.Num, false)
		if errors.Is(err, sql.ErrNoRows) {
			return t, "", t.notFound()
		}
		if err != nil {
			return t, "", err
		}
		claims, hyps, err := store.EvidenceDependents(ctx, tx, ev.ID)
		if err != nil {
			return t, "", err
		}
		if claims+hyps > 0 {
			return t, "", fmt.Errorf("%w: %s is linked by %s and %s; unlink it first", domain.ErrConflict, t.Ref,
				plural(claims, "claim link"), plural(hyps, "hypothesis link"))
		}
		if err := store.DeleteEvidence(ctx, tx, ev.ID); err != nil {
			return t, "", err
		}
		return t, fmt.Sprintf("Deleted evidence %s %q", t.Ref, domain.TruncateLine(ev.Title, deleteSummaryTitle)), nil
	})
}

// DeleteClaim permanently deletes one claim with its own evidence links and
// records claim.delete ("Deleted claim C-003"). It is ErrConflict while
// another claim names it in superseded_by.
func (a *App) DeleteClaim(ctx context.Context, actor Actor, invRef, claimRef string) error {
	return a.deleteStatement(ctx, actor, invRef, claimRef, claimKind)
}

// DeleteHypothesis permanently deletes one hypothesis with its own evidence
// links and records hypothesis.delete. It is ErrConflict while another
// hypothesis names it in superseded_by.
func (a *App) DeleteHypothesis(ctx context.Context, actor Actor, invRef, hypRef string) error {
	return a.deleteStatement(ctx, actor, invRef, hypRef, hypothesisKind)
}

// DeleteQuestion permanently deletes one question and records
// question.delete ("Deleted question Q-002").
func (a *App) DeleteQuestion(ctx context.Context, actor Actor, invRef, qRef string) error {
	const op = "question.delete"
	return a.deleteRecord(ctx, actor, op, func(tx *sql.Tx) (recordTarget, string, error) {
		t, q, err := a.getQuestion(ctx, tx, invRef, qRef)
		if err != nil {
			return t, "", err
		}
		if err := store.DeleteQuestion(ctx, tx, q.ID); err != nil {
			return t, "", err
		}
		return t, "Deleted question " + t.Ref, nil
	})
}

// deleteStatement deletes a claim or hypothesis unless another record of the
// same kind is superseded by it.
func (a *App) deleteStatement(ctx context.Context, actor Actor, invRef, ref string, k stmtKind) error {
	op := string(k.rec) + ".delete"
	return a.deleteRecord(ctx, actor, op, func(tx *sql.Tx) (recordTarget, string, error) {
		t, cur, err := a.getStatement(ctx, tx, k, invRef, ref)
		if err != nil {
			return t, "", err
		}
		if k.rec == domain.RecordClaim {
			for _, selected := range append(append([]string{}, t.Inv.ConclusionSupportingClaims...), t.Inv.ConclusionContradictingClaims...) {
				if selected == t.Ref {
					return t, "", fmt.Errorf("%w: %s is selected in the conclusion; remove it from the conclusion before deleting it", domain.ErrConflict, t.Ref)
				}
			}
		}
		deps, err := store.SupersededByRefs(ctx, tx, k.target, cur.ID)
		if err != nil {
			return t, "", err
		}
		if len(deps) > 0 {
			names := make([]string, len(deps))
			for i, n := range deps {
				names[i] = domain.FormatRecordRef(k.rec, n)
			}
			return t, "", fmt.Errorf("%w: %s replaces %s; reinstate or delete %s first", domain.ErrConflict, t.Ref,
				strings.Join(names, ", "), strings.Join(names, ", "))
		}
		if err := store.DeleteStatement(ctx, tx, k.target, cur.ID); err != nil {
			return t, "", err
		}
		return t, fmt.Sprintf("Deleted %s %s", k.rec, t.Ref), nil
	})
}

// deleteRecord runs del in a write transaction, then records op on the
// record's investigation and touches it.
func (a *App) deleteRecord(ctx context.Context, actor Actor, op string,
	del func(tx *sql.Tx) (recordTarget, string, error)) error {
	actor, err := actor.Validate()
	if err != nil {
		return a.fail(op, err)
	}
	var ref string
	err = a.db.WithWriteTx(ctx, func(tx *sql.Tx) error {
		now := a.timestamp()
		t, summary, err := del(tx)
		if err != nil {
			return err
		}
		ref = t.Ref
		if err := a.recordActivity(ctx, tx, actor, t.Inv.ID, op, t.Ref, summary, "", now); err != nil {
			return err
		}
		return store.TouchInvestigation(ctx, tx, t.Inv.ID, now)
	})
	if err != nil {
		return a.fail(op, err)
	}
	a.log.Debug(op, "ref", ref)
	return nil
}
