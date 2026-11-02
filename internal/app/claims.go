package app

import (
	"context"

	"github.com/jameskbb/casebook/internal/store"
)

// AddClaim records a claim under the next C ref of the investigation (empty
// invRef means the active one) and creates its evidence links in the same
// transaction, recording one claim.add row ("Added claim C-001 [high] with 2
// evidence links"). Confidence defaults to unknown, status to active,
// verification by actor kind. Status and verification pair as ADR-0013 says:
// status disputed implies verification disputed and the reverse (either one
// alone sets both; a contradicting pair is a ValidationError on
// verification), and superseded is rejected on either: add the claim, then
// supersede it. A link to a missing evidence ref is ErrNotFound and
// one naming another investigation is ErrConflict; either way nothing is
// written. Duplicate links with the same relation collapse into one; with
// different relations they are a ValidationError on links[i].relation.
func (a *App) AddClaim(ctx context.Context, actor Actor, invRef string, in ClaimInput) (Claim, error) {
	row, err := a.addStatement(ctx, actor, invRef, claimKind, stmtInput{
		Statement:    in.Statement,
		Confidence:   in.Confidence,
		Rationale:    in.ConfidenceRationale,
		Status:       string(in.Status),
		Verification: in.Verification,
		Links:        in.Links,
	})
	if err != nil {
		return Claim{}, err
	}
	return row.Claim(), nil
}

// GetClaim returns one claim with its evidence links (ordered by evidence
// ref). claimRef may be qualified (INV-0001/C-001).
func (a *App) GetClaim(ctx context.Context, invRef, claimRef string) (Claim, error) {
	row, err := a.readStatement(ctx, claimKind, "claim.get", invRef, claimRef)
	if err != nil {
		return Claim{}, err
	}
	return row.Claim(), nil
}

// ListClaims returns the investigation's claims ordered by ref, each with its
// links. Superseded and rejected claims are included. The slice is never nil.
func (a *App) ListClaims(ctx context.Context, invRef string) ([]Claim, error) {
	rows, err := a.listStatements(ctx, claimKind, "claim.list", invRef)
	if err != nil {
		return nil, err
	}
	return store.Claims(rows), nil
}

// UpdateClaim applies a patch: nil fields stay unchanged. SupersededBy
// supersedes the claim exactly like SupersedeClaim (status and verification
// superseded; another verification in the same patch is a ValidationError)
// and records claim.supersede. Status superseded without SupersededBy is a
// ValidationError on superseded_by unless the claim already has a
// replacement. Status and verification otherwise follow ADR-0013: status
// disputed sets verification disputed and verification disputed sets status
// disputed; another verification on a disputed claim makes it active;
// leaving disputed or superseded without a verification resets it to the
// actor's default; verification superseded is a ValidationError; a
// verification alone on a superseded claim is ErrConflict (give it a status
// first). Moving a superseded claim to another status clears superseded_by.
// A patch that changes nothing records no activity; otherwise one
// claim.update row lists the changed field names, never their values.
func (a *App) UpdateClaim(ctx context.Context, actor Actor, invRef, claimRef string, p ClaimPatch) (Claim, error) {
	c, _, err := a.UpdateClaimOutcome(ctx, actor, invRef, claimRef, p)
	return c, err
}

// UpdateClaimOutcome is UpdateClaim that also reports whether the edit reset
// a vouched verification to ai_generated (ADR-0014) and the label it
// replaced, so a surface can tell the person their check is gone.
func (a *App) UpdateClaimOutcome(ctx context.Context, actor Actor, invRef, claimRef string, p ClaimPatch) (Claim, UpdateOutcome, error) {
	var status *string
	if p.Status != nil {
		s := string(*p.Status)
		status = &s
	}
	row, outcome, err := a.updateStatement(ctx, actor, invRef, claimRef, claimKind, stmtPatch{
		Statement:    p.Statement,
		Rationale:    p.ConfidenceRationale,
		Confidence:   p.Confidence,
		Status:       status,
		Verification: p.Verification,
		SupersededBy: p.SupersededBy,
	}, "claim.update", "superseded_by")
	if err != nil {
		return Claim{}, UpdateOutcome{}, err
	}
	return row.Claim(), outcome, nil
}

// SetClaimVerification sets the claim's verification state and records
// claim.update ("Set verification of C-001 to disputed; status disputed").
// The status follows (ADR-0013): disputed disputes the claim, any other value
// on a disputed claim makes it active, superseded is a ValidationError (only
// superseding writes it), and any change on a superseded claim is
// ErrConflict. Setting the current value is a no-op with no activity.
func (a *App) SetClaimVerification(ctx context.Context, actor Actor, invRef, claimRef string, v Verification) (Claim, error) {
	row, err := a.setStatementVerification(ctx, actor, invRef, claimRef, claimKind, v)
	if err != nil {
		return Claim{}, err
	}
	return row.Claim(), nil
}

// SupersedeClaim marks claimRef superseded by replacementRef: status
// superseded, verification superseded, superseded_by the replacement; it
// records claim.supersede ("C-003 superseded by C-002"). The replacement must
// be another claim in the same investigation that is not itself superseded
// (ErrConflict otherwise; a missing replacement is ErrNotFound). Superseding
// again by the same replacement is a no-op.
func (a *App) SupersedeClaim(ctx context.Context, actor Actor, invRef, claimRef, replacementRef string) (Claim, error) {
	row, _, err := a.updateStatement(ctx, actor, invRef, claimRef, claimKind, stmtPatch{SupersededBy: &replacementRef},
		"claim.supersede", "replacement_ref")
	if err != nil {
		return Claim{}, err
	}
	return row.Claim(), nil
}
