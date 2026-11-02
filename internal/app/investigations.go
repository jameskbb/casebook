package app

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/jameskbb/casebook/internal/domain"
	"github.com/jameskbb/casebook/internal/store"
)

// CreateInvestigation creates an investigation with the next INV ref and
// records investigation.create (plus active.set, or active.bind where a
// directory binding applies, when MakeActive is true).
// Status defaults to new and must not be archived.
func (a *App) CreateInvestigation(ctx context.Context, actor Actor, in CreateInvestigationInput) (Investigation, error) {
	const op = "investigation.create"
	actor, err := actor.Validate()
	if err != nil {
		return Investigation{}, a.fail(op, err)
	}
	title, err := domain.SanitizeLine("title", in.Title, domain.MaxTitleBytes, true)
	if err != nil {
		return Investigation{}, a.fail(op, err)
	}
	problem, err := domain.SanitizeText("problem_statement", in.ProblemStatement, domain.MaxProblemStatementBytes, false)
	if err != nil {
		return Investigation{}, a.fail(op, err)
	}
	status := in.Status
	if status == "" {
		status = domain.InvestigationStatusNew
	}
	if !status.Valid() || status == domain.InvestigationStatusArchived {
		return Investigation{}, a.fail(op, domain.NewValidationError("status",
			"must be one of new, investigating, blocked, resolved, got %q", string(status)))
	}

	var out Investigation
	err = a.db.WithWriteTx(ctx, func(tx *sql.Tx) error {
		now := a.timestamp()
		num, err := store.AllocateInvestigationNum(ctx, tx)
		if err != nil {
			return err
		}
		ref := domain.FormatInvestigationRef(num)
		id, err := store.InsertInvestigation(ctx, tx, store.NewInvestigation{
			RefNum:           num,
			Title:            title,
			ProblemStatement: problem,
			Status:           status,
			CreatedAt:        now,
			CreatedBy:        actor,
		})
		if err != nil {
			return err
		}
		if err := a.recordActivity(ctx, tx, actor, id, "investigation.create", ref,
			fmt.Sprintf("Created investigation %s %q", ref, title), "", now); err != nil {
			return err
		}
		if in.MakeActive {
			created := domain.Investigation{ID: id, Ref: ref}
			if err := a.makeActiveTx(ctx, tx, actor, created, false, now); err != nil {
				return err
			}
		}
		inv, err := a.getInvestigationByID(ctx, tx, id)
		if err != nil {
			return err
		}
		out = inv
		return nil
	})
	if err != nil {
		return Investigation{}, a.fail(op, err)
	}
	a.log.Debug(op, "ref", out.Ref)
	return out, nil
}

// GetInvestigation returns one investigation. An empty invRef means the
// active investigation.
func (a *App) GetInvestigation(ctx context.Context, invRef string) (Investigation, error) {
	var out Investigation
	err := a.db.WithReadTx(ctx, func(tx *sql.Tx) error {
		inv, err := a.resolveInvestigation(ctx, tx, invRef)
		if err != nil {
			return err
		}
		out = inv
		return nil
	})
	if err != nil {
		return Investigation{}, a.fail("investigation.get", err)
	}
	return out, nil
}

// statusChange describes how a requested status moves an investigation.
type statusChange int

const (
	statusSame      statusChange = iota // nothing to write
	statusSet                           // plain status change, not archived either side
	statusArchive                       // moves into archived
	statusUnarchive                     // leaves archived
)

// planStatus works out the status write that moves inv to target.
func planStatus(inv domain.Investigation, target domain.InvestigationStatus, now string) (statusChange, store.InvestigationStatusUpdate) {
	archived := inv.Status == domain.InvestigationStatusArchived || inv.Archived
	switch {
	case target == domain.InvestigationStatusArchived && archived:
		return statusSame, store.InvestigationStatusUpdate{}
	case target == domain.InvestigationStatusArchived:
		return statusArchive, store.InvestigationStatusUpdate{
			Status:           domain.InvestigationStatusArchived,
			PreArchiveStatus: inv.Status,
			ArchivedAt:       now,
			UpdatedAt:        now,
		}
	case archived:
		return statusUnarchive, store.InvestigationStatusUpdate{Status: target, UpdatedAt: now}
	case target == inv.Status:
		return statusSame, store.InvestigationStatusUpdate{}
	default:
		return statusSet, store.InvestigationStatusUpdate{Status: target, UpdatedAt: now}
	}
}

// UpdateInvestigation applies a patch: nil fields stay unchanged. Status
// archived archives (like ArchiveInvestigation); a non-archived status on an
// archived investigation unarchives it to that status. A patch that changes
// nothing returns the current investigation and records no activity;
// otherwise one investigation.update row (investigation.archive when the
// patch archives) lists the changed field names, never their values.
func (a *App) UpdateInvestigation(ctx context.Context, actor Actor, invRef string, p InvestigationPatch) (Investigation, error) {
	const op = "investigation.update"
	actor, err := actor.Validate()
	if err != nil {
		return Investigation{}, a.fail(op, err)
	}
	var title, problem *string
	if p.Title != nil {
		v, err := domain.SanitizeLine("title", *p.Title, domain.MaxTitleBytes, true)
		if err != nil {
			return Investigation{}, a.fail(op, err)
		}
		title = &v
	}
	if p.ProblemStatement != nil {
		v, err := domain.SanitizeText("problem_statement", *p.ProblemStatement, domain.MaxProblemStatementBytes, false)
		if err != nil {
			return Investigation{}, a.fail(op, err)
		}
		problem = &v
	}
	if p.Status != nil {
		if err := p.Status.Validate("status"); err != nil {
			return Investigation{}, a.fail(op, err)
		}
	}

	var out Investigation
	err = a.db.WithWriteTx(ctx, func(tx *sql.Tx) error {
		now := a.timestamp()
		inv, err := a.resolveInvestigation(ctx, tx, invRef)
		if err != nil {
			return err
		}
		var changed []string
		newTitle, newProblem := inv.Title, inv.ProblemStatement
		if title != nil && *title != inv.Title {
			newTitle = *title
			changed = append(changed, "title")
		}
		if problem != nil && *problem != inv.ProblemStatement {
			newProblem = *problem
			changed = append(changed, "problem_statement")
		}
		textChanged := len(changed) > 0

		change := statusSame
		var su store.InvestigationStatusUpdate
		if p.Status != nil {
			change, su = planStatus(inv, *p.Status, now)
			if change != statusSame {
				changed = append(changed, "status")
			}
		}
		if len(changed) == 0 {
			out = inv
			return nil
		}

		if textChanged {
			if err := store.UpdateInvestigationText(ctx, tx, inv.ID, newTitle, newProblem, now); err != nil {
				return err
			}
		}
		if change != statusSame {
			if err := store.SetInvestigationStatus(ctx, tx, inv.ID, su); err != nil {
				return err
			}
		}

		action := "investigation.update"
		summary := fmt.Sprintf("Updated investigation %s: %s", inv.Ref, strings.Join(changed, ", "))
		switch change {
		case statusArchive:
			action = "investigation.archive"
			summary = fmt.Sprintf("Archived investigation %s", inv.Ref)
			if textChanged {
				summary += fmt.Sprintf(" and updated %s", strings.Join(changed[:len(changed)-1], ", "))
			}
		case statusUnarchive:
			summary += fmt.Sprintf(" (unarchived to %s)", su.Status)
		case statusSet:
			summary += fmt.Sprintf(" (status %s)", su.Status)
		}
		if err := a.recordActivity(ctx, tx, actor, inv.ID, action, inv.Ref, summary, "", now); err != nil {
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

// ArchiveInvestigation sets archived_at and status archived, remembering the
// prior status for UnarchiveInvestigation, and records investigation.archive.
// Archiving an archived investigation is a no-op with no activity. Archiving
// the active investigation leaves it active.
func (a *App) ArchiveInvestigation(ctx context.Context, actor Actor, invRef string) (Investigation, error) {
	return a.moveStatus(ctx, actor, invRef, "investigation.archive", true)
}

// UnarchiveInvestigation restores the status the investigation had before it
// was archived (investigating when none is recorded), clears archived_at and
// records investigation.unarchive. Unarchiving an investigation that is not
// archived is a no-op with no activity.
func (a *App) UnarchiveInvestigation(ctx context.Context, actor Actor, invRef string) (Investigation, error) {
	return a.moveStatus(ctx, actor, invRef, "investigation.unarchive", false)
}

func (a *App) moveStatus(ctx context.Context, actor Actor, invRef, op string, archive bool) (Investigation, error) {
	actor, err := actor.Validate()
	if err != nil {
		return Investigation{}, a.fail(op, err)
	}
	var out Investigation
	err = a.db.WithWriteTx(ctx, func(tx *sql.Tx) error {
		now := a.timestamp()
		inv, err := a.resolveInvestigation(ctx, tx, invRef)
		if err != nil {
			return err
		}
		archived := inv.Status == domain.InvestigationStatusArchived || inv.Archived
		target := domain.InvestigationStatusArchived
		if !archive {
			if !archived {
				out = inv
				return nil
			}
			pre, err := store.GetPreArchiveStatus(ctx, tx, inv.ID)
			if err != nil {
				return err
			}
			target = pre
			if !target.Valid() || target == domain.InvestigationStatusArchived {
				target = domain.InvestigationStatusInvestigating
			}
		}
		change, su := planStatus(inv, target, now)
		if change == statusSame {
			out = inv
			return nil
		}
		if err := store.SetInvestigationStatus(ctx, tx, inv.ID, su); err != nil {
			return err
		}
		summary := fmt.Sprintf("Archived investigation %s", inv.Ref)
		if !archive {
			summary = fmt.Sprintf("Unarchived investigation %s (status %s)", inv.Ref, su.Status)
		}
		if err := a.recordActivity(ctx, tx, actor, inv.ID, op, inv.Ref, summary, "", now); err != nil {
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

// UpdateConclusion replaces the conclusion, its confidence (empty means
// unknown) and rationale, and records conclusion.update. An empty conclusion
// clears it; the confidence must then be empty or unknown and the rationale
// empty (ValidationError on confidence or rationale otherwise). Replacing it
// with identical values is a no-op with no activity.
func (a *App) UpdateConclusion(ctx context.Context, actor Actor, invRef string, in ConclusionInput) (Investigation, error) {
	const op = "conclusion.update"
	actor, err := actor.Validate()
	if err != nil {
		return Investigation{}, a.fail(op, err)
	}
	c, err := prepareConclusion(in)
	if err != nil {
		return Investigation{}, a.fail(op, err)
	}
	var out Investigation
	err = a.db.WithWriteTx(ctx, func(tx *sql.Tx) error {
		inv, _, err := a.conclusionTx(ctx, tx, actor, invRef, c)
		out = inv
		return err
	})
	if err != nil {
		return Investigation{}, a.fail(op, err)
	}
	a.log.Debug(op, "ref", out.Ref)
	return out, nil
}

// preparedConclusion is a validated ConclusionInput.
type preparedConclusion struct {
	conclusion, rationale     string
	confidence                domain.Confidence
	verification              domain.Verification
	supporting, contradicting []string
	review                    bool
	expectedReviewToken       string
	reviewBasis               string
}

// prepareConclusion validates and sanitizes in.
func prepareConclusion(in ConclusionInput) (preparedConclusion, error) {
	var (
		c   preparedConclusion
		err error
	)
	if c.conclusion, err = domain.SanitizeText("conclusion", in.Conclusion, domain.MaxConclusionBytes, false); err != nil {
		return c, err
	}
	if c.rationale, err = domain.SanitizeText("rationale", in.Rationale, domain.MaxConclusionRationaleBytes, false); err != nil {
		return c, err
	}
	c.confidence = in.Confidence
	if c.confidence == "" {
		c.confidence = domain.ConfidenceUnknown
	}
	if err := c.confidence.Validate("confidence"); err != nil {
		return c, err
	}
	if c.conclusion == "" {
		// Clearing the conclusion clears its confidence and rationale too, so
		// a pack never shows "(not yet set)" next to a confidence.
		if c.confidence != domain.ConfidenceUnknown {
			return c, domain.NewValidationError("confidence", "must be empty or unknown when the conclusion is empty")
		}
		if c.rationale != "" {
			return c, domain.NewValidationError("rationale", "must be empty when the conclusion is empty")
		}
	}
	c.verification = in.Verification
	if c.verification != "" {
		if err := c.verification.Validate("verification"); err != nil {
			return c, err
		}
		if c.verification == domain.VerificationSuperseded {
			return c, domain.NewValidationError("verification", "superseded is not a conclusion verification")
		}
	}
	c.supporting, c.contradicting = in.SupportingClaims, in.ContradictingClaims
	c.review = in.Review
	c.expectedReviewToken = in.ExpectedReviewToken
	if c.reviewBasis, err = domain.SanitizeText("review_basis", in.ReviewBasis, domain.MaxConclusionRationaleBytes, false); err != nil {
		return c, err
	}
	if c.reviewBasis != "" && !c.review {
		return c, domain.NewValidationError("review_basis", "requires review to be true")
	}
	if c.review && c.reviewBasis == "" {
		return c, domain.NewValidationError("review_basis", "explain what you checked when recording a review")
	}
	if c.conclusion == "" && (c.review || c.reviewBasis != "" || len(c.supporting) > 0 || len(c.contradicting) > 0 || c.verification != "") {
		return c, domain.NewValidationError("conclusion", "an empty conclusion cannot carry verification, claims or a review")
	}
	return c, nil
}

// conclusionTx writes a prepared conclusion inside tx and reports whether it
// changed anything; identical values write nothing.
func (a *App) conclusionTx(ctx context.Context, tx *sql.Tx, actor domain.Actor, invRef string,
	c preparedConclusion) (Investigation, bool, error) {
	now := a.timestamp()
	inv, err := a.resolveInvestigation(ctx, tx, invRef)
	if err != nil {
		return Investigation{}, false, err
	}
	if err := checkVerifier(actor.Kind, c.verification); err != nil {
		return Investigation{}, false, err
	}
	if c.review && actor.Kind != domain.ActorHuman {
		return Investigation{}, false, domain.NewValidationError("review", "only a person can record a conclusion review")
	}
	next, snapshot, changed, err := a.prepareConclusionAccountability(ctx, tx, actor, inv, c, now)
	if err != nil {
		return Investigation{}, false, err
	}
	if !changed {
		return inv, false, nil
	}
	if err := store.UpdateInvestigationConclusion(ctx, tx, inv.ID, c.conclusion, c.confidence, c.rationale, now); err != nil {
		return Investigation{}, false, err
	}
	if err := store.SetConclusionAccountability(ctx, tx, next, snapshot); err != nil {
		return Investigation{}, false, err
	}
	summary := fmt.Sprintf("Updated conclusion of %s (confidence %s)", inv.Ref, c.confidence)
	if inv.ConclusionVerification != next.ConclusionVerification && next.ConclusionVerification == domain.VerificationAIGenerated && actor.Kind != domain.ActorHuman && inv.Conclusion != c.conclusion {
		summary += resetNote(actor.Kind)
	}
	if c.conclusion == "" {
		summary = fmt.Sprintf("Cleared conclusion of %s", inv.Ref)
	}
	if err := a.recordActivity(ctx, tx, actor, inv.ID, "conclusion.update", inv.Ref, summary, "", now); err != nil {
		return Investigation{}, false, err
	}
	got, err := a.getInvestigationByID(ctx, tx, inv.ID)
	return got, true, err
}
