package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jameskbb/casebook/internal/domain"
	"github.com/jameskbb/casebook/internal/store"
)

// Evidence links (D-14) live in claim_evidence and hypothesis_evidence. The
// app resolves both ends inside the record's investigation and rejects
// cross-investigation links with ErrConflict; the SQL guard triggers are the
// backstop (their constraint error also maps to ErrConflict through fail).

// LinkEvidenceToClaim links evidence to a claim with a relation (supports or
// contradicts) and returns the claim. A new link records link.add; relinking
// with another relation updates it and records link.update; relinking with the
// same relation is a no-op with no activity. evidenceRef resolves in the
// claim's investigation: a missing ref is ErrNotFound and a qualified ref to
// another investigation is ErrConflict.
func (a *App) LinkEvidenceToClaim(ctx context.Context, actor Actor, invRef, claimRef, evidenceRef string, rel Relation) (Claim, error) {
	row, err := a.linkEvidence(ctx, actor, invRef, claimRef, evidenceRef, rel, claimKind)
	if err != nil {
		return Claim{}, err
	}
	return row.Claim(), nil
}

// UnlinkEvidenceFromClaim removes the link between a claim and evidence,
// records link.remove and returns the claim. A missing link is ErrNotFound
// ("not found: no link between C-001 and E-002").
func (a *App) UnlinkEvidenceFromClaim(ctx context.Context, actor Actor, invRef, claimRef, evidenceRef string) (Claim, error) {
	row, err := a.unlinkEvidence(ctx, actor, invRef, claimRef, evidenceRef, claimKind)
	if err != nil {
		return Claim{}, err
	}
	return row.Claim(), nil
}

// linkEvidence is the shared body of LinkEvidenceTo*.
func (a *App) linkEvidence(ctx context.Context, actor Actor, invRef, recordRef, evidenceRef string, rel Relation,
	k stmtKind) (store.StatementRow, error) {
	const op = "link.add"
	actor, err := actor.Validate()
	if err != nil {
		return store.StatementRow{}, a.fail(op, err)
	}
	if err := rel.Validate("relation"); err != nil {
		return store.StatementRow{}, a.fail(op, err)
	}
	rr, err := domain.ParseRecordRef(evidenceRef, domain.RecordEvidence)
	if err != nil {
		return store.StatementRow{}, a.fail(op, refieldValidation(err, "evidence_ref"))
	}
	var out store.StatementRow
	err = a.db.WithWriteTx(ctx, func(tx *sql.Tx) error {
		now := a.timestamp()
		t, cur, err := a.getStatement(ctx, tx, k, invRef, recordRef)
		if err != nil {
			return err
		}
		evID, evRef, err := linkedEvidenceID(ctx, tx, t.Inv, rr)
		if err != nil {
			return err
		}
		var action, summary string
		existing, err := store.GetLinkRelation(ctx, tx, k.target, cur.ID, evID)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			if err := store.InsertLink(ctx, tx, k.target, cur.ID, evID, rel, now, actor); err != nil {
				return err
			}
			action, summary = "link.add", fmt.Sprintf("Linked %s to %s (%s)", evRef, cur.Ref, rel)
		case err != nil:
			return err
		case existing == rel:
			out = cur
			return nil
		default:
			if err := store.UpdateLinkRelation(ctx, tx, k.target, cur.ID, evID, rel); err != nil {
				return err
			}
			action, summary = "link.update", fmt.Sprintf("Changed link %s to %s from %s to %s", evRef, cur.Ref, existing, rel)
		}
		row, err := a.writeStatementChange(ctx, tx, actor, k, t, cur.ID, nil, action, summary, now)
		if err != nil {
			return err
		}
		out = row
		return nil
	})
	if err != nil {
		return store.StatementRow{}, a.fail(op, err)
	}
	a.log.Debug(op, "investigation_ref", out.InvestigationRef, "ref", out.Ref)
	return out, nil
}

// unlinkEvidence is the shared body of UnlinkEvidenceFrom*.
func (a *App) unlinkEvidence(ctx context.Context, actor Actor, invRef, recordRef, evidenceRef string,
	k stmtKind) (store.StatementRow, error) {
	const op = "link.remove"
	actor, err := actor.Validate()
	if err != nil {
		return store.StatementRow{}, a.fail(op, err)
	}
	rr, err := domain.ParseRecordRef(evidenceRef, domain.RecordEvidence)
	if err != nil {
		return store.StatementRow{}, a.fail(op, refieldValidation(err, "evidence_ref"))
	}
	var out store.StatementRow
	err = a.db.WithWriteTx(ctx, func(tx *sql.Tx) error {
		now := a.timestamp()
		t, cur, err := a.getStatement(ctx, tx, k, invRef, recordRef)
		if err != nil {
			return err
		}
		evID, evRef, err := linkedEvidenceID(ctx, tx, t.Inv, rr)
		if err != nil {
			return err
		}
		ok, err := store.DeleteLink(ctx, tx, k.target, cur.ID, evID)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("%w: no link between %s and %s", domain.ErrNotFound, cur.Ref, evRef)
		}
		row, err := a.writeStatementChange(ctx, tx, actor, k, t, cur.ID, nil, op,
			fmt.Sprintf("Unlinked %s from %s", evRef, cur.Ref), now)
		if err != nil {
			return err
		}
		out = row
		return nil
	})
	if err != nil {
		return store.StatementRow{}, a.fail(op, err)
	}
	a.log.Debug(op, "investigation_ref", out.InvestigationRef, "ref", out.Ref)
	return out, nil
}

// LinkEvidenceToHypothesis links evidence to a hypothesis with the same
// rules as LinkEvidenceToClaim and returns the hypothesis.
func (a *App) LinkEvidenceToHypothesis(ctx context.Context, actor Actor, invRef, hypRef, evidenceRef string, rel Relation) (Hypothesis, error) {
	row, err := a.linkEvidence(ctx, actor, invRef, hypRef, evidenceRef, rel, hypothesisKind)
	if err != nil {
		return Hypothesis{}, err
	}
	return row.Hypothesis(), nil
}

// UnlinkEvidenceFromHypothesis removes the link between a hypothesis and
// evidence, records link.remove and returns the hypothesis. A missing link is
// ErrNotFound.
func (a *App) UnlinkEvidenceFromHypothesis(ctx context.Context, actor Actor, invRef, hypRef, evidenceRef string) (Hypothesis, error) {
	row, err := a.unlinkEvidence(ctx, actor, invRef, hypRef, evidenceRef, hypothesisKind)
	if err != nil {
		return Hypothesis{}, err
	}
	return row.Hypothesis(), nil
}
