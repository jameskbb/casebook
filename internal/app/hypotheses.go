package app

import (
	"context"

	"github.com/jameskbb/casebook/internal/domain"
	"github.com/jameskbb/casebook/internal/store"
)

// Hypotheses follow the same rules as claims (see statements.go): status
// open, supported, disproven or superseded; superseding by another
// hypothesis in the same investigation; supporting or contradicting
// evidence links.

var hypothesisKind = stmtKind{
	target:         store.LinkHypothesis,
	rec:            domain.RecordHypothesis,
	addOp:          "hypothesis.add",
	updateOp:       "hypothesis.update",
	supersedeOp:    "hypothesis.supersede",
	supersedeFunc:  "SupersedeHypothesis",
	rationaleField: "rationale",
	defaultStatus:  string(domain.HypothesisStatusOpen),
	superseded:     string(domain.HypothesisStatusSuperseded),
	validStatus:    func(f, s string) error { return domain.HypothesisStatus(s).Validate(f) },
}

// AddHypothesis records a hypothesis under the next H ref of the
// investigation (empty invRef means the active one) with its evidence links in
// the same transaction, recording one hypothesis.add row. Confidence defaults
// to unknown, status to open, verification by actor kind; status or
// verification superseded is rejected (use SupersedeHypothesis). Verification
// disputed is allowed in any other status (hypotheses have no disputed
// status, ADR-0013). Link rules match AddClaim.
func (a *App) AddHypothesis(ctx context.Context, actor Actor, invRef string, in HypothesisInput) (Hypothesis, error) {
	row, err := a.addStatement(ctx, actor, invRef, hypothesisKind, stmtInput{
		Statement:    in.Statement,
		Confidence:   in.Confidence,
		Rationale:    in.Rationale,
		Status:       string(in.Status),
		Verification: in.Verification,
		Links:        in.Links,
	})
	if err != nil {
		return Hypothesis{}, err
	}
	return row.Hypothesis(), nil
}

// GetHypothesis returns one hypothesis with its evidence links (ordered by
// evidence ref). hypRef may be qualified (INV-0001/H-001).
func (a *App) GetHypothesis(ctx context.Context, invRef, hypRef string) (Hypothesis, error) {
	row, err := a.readStatement(ctx, hypothesisKind, "hypothesis.get", invRef, hypRef)
	if err != nil {
		return Hypothesis{}, err
	}
	return row.Hypothesis(), nil
}

// ListHypotheses returns the investigation's hypotheses ordered by ref, each
// with its links, in every status. The slice is never nil.
func (a *App) ListHypotheses(ctx context.Context, invRef string) ([]Hypothesis, error) {
	rows, err := a.listStatements(ctx, hypothesisKind, "hypothesis.list", invRef)
	if err != nil {
		return nil, err
	}
	return store.Hypotheses(rows), nil
}

// UpdateHypothesis applies a patch with the same rules as UpdateClaim:
// SupersededBy supersedes (hypothesis.supersede), status superseded needs a
// replacement, leaving superseded clears it and resets verification to the
// actor's default unless the patch sets one, verification superseded is
// written only by superseding, no-op patches record nothing, anything else
// records one hypothesis.update row naming the changed fields.
func (a *App) UpdateHypothesis(ctx context.Context, actor Actor, invRef, hypRef string, p HypothesisPatch) (Hypothesis, error) {
	h, _, err := a.UpdateHypothesisOutcome(ctx, actor, invRef, hypRef, p)
	return h, err
}

// UpdateHypothesisOutcome is UpdateHypothesis that also reports whether the
// edit reset a vouched verification and the label it replaced (see
// UpdateClaimOutcome).
func (a *App) UpdateHypothesisOutcome(ctx context.Context, actor Actor, invRef, hypRef string, p HypothesisPatch) (Hypothesis, UpdateOutcome, error) {
	var status *string
	if p.Status != nil {
		s := string(*p.Status)
		status = &s
	}
	row, outcome, err := a.updateStatement(ctx, actor, invRef, hypRef, hypothesisKind, stmtPatch{
		Statement:    p.Statement,
		Rationale:    p.Rationale,
		Confidence:   p.Confidence,
		Status:       status,
		Verification: p.Verification,
		SupersededBy: p.SupersededBy,
	}, "hypothesis.update", "superseded_by")
	if err != nil {
		return Hypothesis{}, UpdateOutcome{}, err
	}
	return row.Hypothesis(), outcome, nil
}

// SetHypothesisVerification sets the verification state and records
// hypothesis.update. Superseded is a ValidationError and any change on a
// superseded hypothesis is ErrConflict (ADR-0013). Setting the current value
// is a no-op with no activity.
func (a *App) SetHypothesisVerification(ctx context.Context, actor Actor, invRef, hypRef string, v Verification) (Hypothesis, error) {
	row, err := a.setStatementVerification(ctx, actor, invRef, hypRef, hypothesisKind, v)
	if err != nil {
		return Hypothesis{}, err
	}
	return row.Hypothesis(), nil
}

// SupersedeHypothesis marks hypRef superseded by replacementRef with the
// same rules as SupersedeClaim and records hypothesis.supersede ("H-001
// superseded by H-002").
func (a *App) SupersedeHypothesis(ctx context.Context, actor Actor, invRef, hypRef, replacementRef string) (Hypothesis, error) {
	row, _, err := a.updateStatement(ctx, actor, invRef, hypRef, hypothesisKind, stmtPatch{SupersededBy: &replacementRef},
		"hypothesis.supersede", "replacement_ref")
	if err != nil {
		return Hypothesis{}, err
	}
	return row.Hypothesis(), nil
}
