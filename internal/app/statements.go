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

// Claims and hypotheses share their rules (D-12, D-13, D-14): a required
// statement, an enum confidence with a rationale, a status, a verification
// state, superseding by another record of the same kind in the same
// investigation, and supporting or contradicting evidence links. The logic
// lives here once, parameterized by stmtKind; claims.go and hypotheses.go are
// thin typed wrappers. Statements and rationales are never logged.

// stmtKind describes claims or hypotheses for the shared logic.
type stmtKind struct {
	target         store.LinkTarget
	rec            domain.RecordType
	addOp          string
	updateOp       string
	supersedeOp    string
	supersedeFunc  string // name of the Supersede method, for messages
	rationaleField string
	defaultStatus  string
	superseded     string
	// disputed is the status that pairs with verification disputed, or ""
	// when the kind has no disputed status (hypotheses). See pairState.
	disputed    string
	validStatus func(field, s string) error
}

var claimKind = stmtKind{
	target:         store.LinkClaim,
	rec:            domain.RecordClaim,
	addOp:          "claim.add",
	updateOp:       "claim.update",
	supersedeOp:    "claim.supersede",
	supersedeFunc:  "SupersedeClaim",
	rationaleField: "confidence_rationale",
	defaultStatus:  string(domain.ClaimStatusActive),
	superseded:     string(domain.ClaimStatusSuperseded),
	disputed:       string(domain.ClaimStatusDisputed),
	validStatus:    func(f, s string) error { return domain.ClaimStatus(s).Validate(f) },
}

// stmtInput is the shared form of ClaimInput and HypothesisInput.
type stmtInput struct {
	Statement    string
	Confidence   domain.Confidence
	Rationale    string
	Status       string
	Verification domain.Verification
	Links        []domain.EvidenceLinkInput
}

// stmtPatch is the shared form of ClaimPatch and HypothesisPatch.
type stmtPatch struct {
	Statement    *string
	Rationale    *string
	Confidence   *domain.Confidence
	Status       *string
	Verification *domain.Verification
	SupersededBy *string
}

func cleanStatement(s string) (string, error) {
	return domain.SanitizeText("statement", s, domain.MaxStatementBytes, true)
}

func (k stmtKind) cleanRationale(s string) (string, error) {
	return domain.SanitizeText(k.rationaleField, s, domain.MaxRationaleBytes, false)
}

// cleanConfidence validates a confidence; empty means unknown.
func cleanConfidence(c domain.Confidence) (domain.Confidence, error) {
	if c == "" {
		return domain.ConfidenceUnknown, nil
	}
	if err := c.Validate("confidence"); err != nil {
		return "", err
	}
	return c, nil
}

// The status and verification of a claim or hypothesis form one state
// (ADR-0013), enforced here and by the triggers of migration 0002:
//   - verification is superseded exactly when the status is superseded, and
//     only superseding (superseded_by, Supersede*) writes it;
//   - for claims, verification is disputed exactly when the status is
//     disputed. Hypotheses have no disputed status, so a hypothesis that is
//     not superseded may carry verification disputed in any status.

// checkPair returns a ValidationError on verification when status and v
// disagree.
func (k stmtKind) checkPair(status string, v domain.Verification) error {
	if status == k.superseded {
		if v != domain.VerificationSuperseded {
			return domain.NewValidationError("verification",
				"must be superseded or omitted when the status is superseded, got %q", v)
		}
		return nil
	}
	if v == domain.VerificationSuperseded {
		return domain.NewValidationError("verification",
			"superseded is set only by superseding; pass superseded_by naming the replacing %s", k.rec)
	}
	if k.disputed == "" {
		return nil
	}
	if status == k.disputed && v != domain.VerificationDisputed {
		return domain.NewValidationError("verification",
			"must be disputed or omitted when the status is disputed, got %q", v)
	}
	if status != k.disputed && v == domain.VerificationDisputed {
		return domain.NewValidationError("verification",
			"disputed needs status disputed; omit the status or set it to disputed, got status %q", status)
	}
	return nil
}

// repairPair maps a status and verification pair written before ADR-0013
// (an export file from an older Casebook) to the agreeing pair, exactly as
// migration 0002 repairs stored rows: the status wins for superseded and
// disputed, a claim verified as disputed while active becomes disputed, and
// any other leftover disputed or superseded verification becomes the
// author's default verification.
func (k stmtKind) repairPair(status string, v domain.Verification, author domain.ActorKind) (string, domain.Verification) {
	switch {
	case status == k.superseded:
		return status, domain.VerificationSuperseded
	case k.disputed != "" && status == k.disputed:
		return status, domain.VerificationDisputed
	case k.disputed != "" && status == k.defaultStatus && v == domain.VerificationDisputed:
		return k.disputed, domain.VerificationDisputed
	case v == domain.VerificationSuperseded, k.disputed != "" && v == domain.VerificationDisputed:
		return status, domain.DefaultVerification(author)
	}
	return status, v
}

// pairState returns the status and verification a write that does not
// supersede leaves on cur. ps and pv are the requested status and
// verification (nil means not given). When only one half is given the other
// follows: verification disputed alone disputes a claim, any other
// verification alone on a disputed claim makes it active again, status
// disputed alone sets verification disputed, and leaving disputed or
// superseded without a verification resets verification to the actor's
// default. A verification alone on a superseded record is ErrConflict:
// reinstate it with a status first.
func (k stmtKind) pairState(cur store.StatementRow, ps *string, pv *domain.Verification,
	actor domain.ActorKind) (string, domain.Verification, error) {
	status := cur.Status
	switch {
	case ps != nil:
		status = *ps
	case pv != nil && *pv == cur.Verification:
		return cur.Status, cur.Verification, nil
	case pv != nil && cur.Status == k.superseded:
		return "", "", fmt.Errorf("%w: %s is superseded by %s; give it another status before changing its verification",
			domain.ErrConflict, cur.Ref, cur.SupersededByRef)
	case pv != nil && k.disputed != "":
		if *pv == domain.VerificationDisputed {
			status = k.disputed
		} else if cur.Status == k.disputed {
			status = k.defaultStatus
		}
	}
	var v domain.Verification
	switch {
	case pv != nil:
		v = *pv
	case k.disputed != "" && status == k.disputed:
		v = domain.VerificationDisputed
	case status != cur.Status && (cur.Status == k.superseded || (k.disputed != "" && cur.Status == k.disputed)):
		v = domain.DefaultVerification(actor)
	default:
		v = cur.Verification
	}
	return status, v, k.checkPair(status, v)
}

// refieldValidation re-labels a *ValidationError with field; other errors
// pass unchanged.
func refieldValidation(err error, field string) error {
	if ve, ok := domain.IsValidation(err); ok {
		return &domain.ValidationError{Field: field, Message: ve.Message}
	}
	return err
}

// linkRequest is one parsed AddClaim/AddHypothesis link.
type linkRequest struct {
	index    int
	ref      domain.RecordRef
	relation domain.Relation
}

// parseLinkInputs validates every link's evidence ref and relation.
func parseLinkInputs(in []domain.EvidenceLinkInput) ([]linkRequest, error) {
	out := make([]linkRequest, 0, len(in))
	for i, l := range in {
		rr, err := domain.ParseRecordRef(l.EvidenceRef, domain.RecordEvidence)
		if err != nil {
			return nil, refieldValidation(err, fmt.Sprintf("links[%d].evidence_ref", i))
		}
		if err := l.Relation.Validate(fmt.Sprintf("links[%d].relation", i)); err != nil {
			return nil, err
		}
		out = append(out, linkRequest{index: i, ref: rr, relation: l.Relation})
	}
	return out, nil
}

// linkedEvidenceID resolves an evidence ref for a link from a record in inv.
// An unqualified ref looks in inv (ErrNotFound when missing); a qualified ref
// naming another investigation is ErrConflict.
func linkedEvidenceID(ctx context.Context, q store.Querier, inv domain.Investigation, rr domain.RecordRef) (int64, string, error) {
	ref := domain.FormatRecordRef(domain.RecordEvidence, rr.Num)
	if rr.InvestigationNum != 0 {
		if other := domain.FormatInvestigationRef(rr.InvestigationNum); other != inv.Ref {
			return 0, "", fmt.Errorf("%w: cross-investigation link: %s/%s is not in %s", domain.ErrConflict, other, ref, inv.Ref)
		}
	}
	id, err := store.EvidenceID(ctx, q, inv.ID, rr.Num)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, "", recordTarget{Inv: inv, Type: domain.RecordEvidence, Num: rr.Num, Ref: ref}.notFound()
	}
	return id, ref, err
}

// getStatement resolves and loads a claim or hypothesis, mapping a missing
// row to ErrNotFound naming the ref.
func (a *App) getStatement(ctx context.Context, q store.Querier, k stmtKind, invRef, ref string) (recordTarget, store.StatementRow, error) {
	t, err := a.resolveRecord(ctx, q, invRef, ref, k.rec)
	if err != nil {
		return recordTarget{}, store.StatementRow{}, err
	}
	row, err := store.GetStatement(ctx, q, k.target, t.Inv.ID, t.Num)
	if errors.Is(err, sql.ErrNoRows) {
		return recordTarget{}, store.StatementRow{}, t.notFound()
	}
	return t, row, err
}

// resolveReplacement resolves the replacement for superseding cur (in inv).
// The replacement must be another record of the same kind in the same
// investigation that is not itself superseded (which also prevents cycles).
func resolveReplacement(ctx context.Context, q store.Querier, k stmtKind, inv domain.Investigation, cur store.StatementRow,
	rr domain.RecordRef) (store.StatementRow, error) {
	ref := domain.FormatRecordRef(k.rec, rr.Num)
	if rr.InvestigationNum != 0 {
		if other := domain.FormatInvestigationRef(rr.InvestigationNum); other != inv.Ref {
			return store.StatementRow{}, fmt.Errorf("%w: cross-investigation supersede: %s/%s is not in %s",
				domain.ErrConflict, other, ref, inv.Ref)
		}
	}
	if rr.Num == cur.RefNum {
		return store.StatementRow{}, fmt.Errorf("%w: %s cannot supersede itself", domain.ErrConflict, cur.Ref)
	}
	rep, err := store.GetStatement(ctx, q, k.target, inv.ID, rr.Num)
	if errors.Is(err, sql.ErrNoRows) {
		return store.StatementRow{}, recordTarget{Inv: inv, Type: k.rec, Num: rr.Num, Ref: ref}.notFound()
	}
	if err != nil {
		return store.StatementRow{}, err
	}
	if rep.Status == k.superseded {
		return store.StatementRow{}, fmt.Errorf("%w: %s is itself superseded by %s; supersede %s with a current %s",
			domain.ErrConflict, rep.Ref, rep.SupersededByRef, cur.Ref, k.rec)
	}
	return rep, nil
}

// plural renders "1 evidence link" or "2 evidence links".
func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

// preparedAdd is a validated AddClaim or AddHypothesis input.
type preparedAdd struct {
	statement, rationale, status string
	confidence                   domain.Confidence
	verification                 domain.Verification
	links                        []linkRequest
}

// prepareAdd validates and sanitizes in for actor (validated by the caller).
func (k stmtKind) prepareAdd(actor domain.Actor, in stmtInput) (preparedAdd, error) {
	var (
		pa  preparedAdd
		err error
	)
	if pa.statement, err = cleanStatement(in.Statement); err != nil {
		return pa, err
	}
	if pa.confidence, err = cleanConfidence(in.Confidence); err != nil {
		return pa, err
	}
	if pa.rationale, err = k.cleanRationale(in.Rationale); err != nil {
		return pa, err
	}
	pa.status = in.Status
	if pa.status == "" {
		pa.status = k.defaultStatus
	}
	if err := k.validStatus("status", pa.status); err != nil {
		return pa, err
	}
	if pa.status == k.superseded {
		return pa, domain.NewValidationError("status",
			"cannot be superseded when adding a %s; add it, then supersede it with superseded_by", k.rec)
	}
	pa.verification = in.Verification
	if pa.verification != "" {
		if err := pa.verification.Validate("verification"); err != nil {
			return pa, err
		}
		if err := checkVerifier(actor.Kind, pa.verification); err != nil {
			return pa, err
		}
	}
	if k.disputed != "" {
		switch {
		case in.Status == "" && pa.verification == domain.VerificationDisputed:
			pa.status = k.disputed
		case pa.status == k.disputed && pa.verification == "":
			pa.verification = domain.VerificationDisputed
		}
	}
	if pa.verification == "" {
		pa.verification = domain.DefaultVerification(actor.Kind)
	}
	if err := k.checkPair(pa.status, pa.verification); err != nil {
		return pa, err
	}
	if pa.links, err = parseLinkInputs(in.Links); err != nil {
		return pa, err
	}
	return pa, nil
}

// addStatement validates in, then in one write transaction runs
// addStatementTx.
func (a *App) addStatement(ctx context.Context, actor Actor, invRef string, k stmtKind, in stmtInput) (store.StatementRow, error) {
	op := k.addOp
	actor, err := actor.Validate()
	if err != nil {
		return store.StatementRow{}, a.fail(op, err)
	}
	pa, err := k.prepareAdd(actor, in)
	if err != nil {
		return store.StatementRow{}, a.fail(op, err)
	}
	var out store.StatementRow
	err = a.db.WithWriteTx(ctx, func(tx *sql.Tx) error {
		row, err := a.addStatementTx(ctx, tx, actor, invRef, k, pa)
		out = row
		return err
	})
	if err != nil {
		return store.StatementRow{}, a.fail(op, err)
	}
	a.log.Debug(op, "investigation_ref", out.InvestigationRef, "ref", out.Ref, "links", len(out.Links))
	return out, nil
}

// addStatementTx allocates the next ref, inserts the record, resolves and
// inserts every link (a bad link fails the transaction), records one add
// activity row, touches the investigation and reads the record back.
func (a *App) addStatementTx(ctx context.Context, tx *sql.Tx, actor domain.Actor, invRef string, k stmtKind,
	pa preparedAdd) (store.StatementRow, error) {
	now := a.timestamp()
	inv, err := a.resolveInvestigation(ctx, tx, invRef)
	if err != nil {
		return store.StatementRow{}, err
	}
	num, err := store.AllocateRecordNum(ctx, tx, inv.ID, k.rec)
	if err != nil {
		return store.StatementRow{}, err
	}
	ref := domain.FormatRecordRef(k.rec, num)
	id, err := store.InsertStatement(ctx, tx, k.target, store.NewStatement{
		InvestigationID: inv.ID,
		RefNum:          num,
		Statement:       pa.statement,
		Confidence:      pa.confidence,
		Rationale:       pa.rationale,
		Status:          pa.status,
		Verification:    pa.verification,
		CreatedAt:       now,
		CreatedBy:       actor,
	})
	if err != nil {
		return store.StatementRow{}, err
	}
	// Links are resolved after the insert so that a bad evidence ref rolls
	// back the record, earlier links and the allocated ref.
	seen := map[int64]linkRequest{}
	for _, l := range pa.links {
		evID, evRef, err := linkedEvidenceID(ctx, tx, inv, l.ref)
		if err != nil {
			return store.StatementRow{}, err
		}
		if prev, ok := seen[evID]; ok {
			if prev.relation != l.relation {
				return store.StatementRow{}, domain.NewValidationError(fmt.Sprintf("links[%d].relation", l.index),
					"%s is already linked as %s by links[%d]", evRef, prev.relation, prev.index)
			}
			continue
		}
		seen[evID] = l
		if err := store.InsertLink(ctx, tx, k.target, id, evID, l.relation, now, actor); err != nil {
			return store.StatementRow{}, err
		}
	}
	summary := fmt.Sprintf("Added %s %s [%s]", k.rec, ref, pa.confidence)
	if len(seen) > 0 {
		summary += " with " + plural(len(seen), "evidence link")
	}
	if err := a.recordActivity(ctx, tx, actor, inv.ID, k.addOp, ref, summary, "", now); err != nil {
		return store.StatementRow{}, err
	}
	if err := store.TouchInvestigation(ctx, tx, inv.ID, now); err != nil {
		return store.StatementRow{}, err
	}
	return store.GetStatementByID(ctx, tx, k.target, id)
}

// readStatement loads one claim or hypothesis in a read transaction.
func (a *App) readStatement(ctx context.Context, k stmtKind, op, invRef, ref string) (store.StatementRow, error) {
	var out store.StatementRow
	err := a.db.WithReadTx(ctx, func(tx *sql.Tx) error {
		_, row, err := a.getStatement(ctx, tx, k, invRef, ref)
		if err != nil {
			return err
		}
		out = row
		return nil
	})
	if err != nil {
		return store.StatementRow{}, a.fail(op, err)
	}
	return out, nil
}

// listStatements loads every claim or hypothesis of an investigation.
func (a *App) listStatements(ctx context.Context, k stmtKind, op, invRef string) ([]store.StatementRow, error) {
	var out []store.StatementRow
	err := a.db.WithReadTx(ctx, func(tx *sql.Tx) error {
		inv, err := a.resolveInvestigation(ctx, tx, invRef)
		if err != nil {
			return err
		}
		rows, err := store.ListStatements(ctx, tx, k.target, inv.ID)
		if err != nil {
			return err
		}
		out = rows
		return nil
	})
	if err != nil {
		return nil, a.fail(op, err)
	}
	return out, nil
}

// preparedPatch is a validated claim or hypothesis patch.
type preparedPatch struct {
	statement, rationale *string
	confidence           *domain.Confidence
	status               *string
	verification         *domain.Verification
	replacement          *domain.RecordRef
}

// preparePatch validates and sanitizes p for actor (validated by the
// caller). supersedeField names the
// replacement ref field in validation errors (superseded_by for patches,
// replacement_ref for Supersede*).
func (k stmtKind) preparePatch(actor domain.Actor, p stmtPatch, supersedeField string) (preparedPatch, error) {
	var pp preparedPatch
	if p.Statement != nil {
		v, err := cleanStatement(*p.Statement)
		if err != nil {
			return pp, err
		}
		pp.statement = &v
	}
	if p.Rationale != nil {
		v, err := k.cleanRationale(*p.Rationale)
		if err != nil {
			return pp, err
		}
		pp.rationale = &v
	}
	if p.Confidence != nil {
		v, err := cleanConfidence(*p.Confidence)
		if err != nil {
			return pp, err
		}
		pp.confidence = &v
	}
	if p.Status != nil {
		if err := k.validStatus("status", *p.Status); err != nil {
			return pp, err
		}
		pp.status = p.Status
	}
	if p.Verification != nil {
		if err := p.Verification.Validate("verification"); err != nil {
			return pp, err
		}
		if err := checkVerifier(actor.Kind, *p.Verification); err != nil {
			return pp, err
		}
		pp.verification = p.Verification
	}
	if p.SupersededBy != nil {
		rr, err := domain.ParseRecordRef(*p.SupersededBy, k.rec)
		if err != nil {
			return pp, refieldValidation(err, supersedeField)
		}
		pp.replacement = &rr
		if p.Status != nil && *p.Status != k.superseded {
			return pp, domain.NewValidationError("status",
				"must be superseded or omitted when superseded_by is set, got %q", *p.Status)
		}
	}
	superseding := pp.replacement != nil || (p.Status != nil && *p.Status == k.superseded)
	if superseding && p.Verification != nil && *p.Verification != domain.VerificationSuperseded {
		return pp, domain.NewValidationError("verification",
			"must be superseded or omitted when superseding, got %q", *p.Verification)
	}
	return pp, nil
}

// updateStatement applies a patch (see ClaimPatch for the rules) in one
// write transaction. op names the operation for errors and logs.
func (a *App) updateStatement(ctx context.Context, actor Actor, invRef, ref string, k stmtKind, p stmtPatch,
	op, supersedeField string) (store.StatementRow, UpdateOutcome, error) {
	actor, err := actor.Validate()
	if err != nil {
		return store.StatementRow{}, UpdateOutcome{}, a.fail(op, err)
	}
	pp, err := k.preparePatch(actor, p, supersedeField)
	if err != nil {
		return store.StatementRow{}, UpdateOutcome{}, a.fail(op, err)
	}
	var (
		out     store.StatementRow
		outcome UpdateOutcome
	)
	err = a.db.WithWriteTx(ctx, func(tx *sql.Tx) error {
		row, _, oc, err := a.updateStatementTxOutcome(ctx, tx, actor, invRef, ref, k, pp)
		out, outcome = row, oc
		return err
	})
	if err != nil {
		return store.StatementRow{}, UpdateOutcome{}, a.fail(op, err)
	}
	a.log.Debug(op, "investigation_ref", out.InvestigationRef, "ref", out.Ref)
	return out, outcome, nil
}

// updateStatementTx applies a prepared patch inside tx and reports whether
// anything changed. A patch that changes nothing writes no activity. A patch
// that supersedes records one supersede row ("C-003 superseded by C-002");
// anything else records one update row listing the changed field names.
func (a *App) updateStatementTx(ctx context.Context, tx *sql.Tx, actor domain.Actor, invRef, ref string, k stmtKind,
	pp preparedPatch) (store.StatementRow, bool, error) {
	row, changed, _, err := a.updateStatementTxOutcome(ctx, tx, actor, invRef, ref, k, pp)
	return row, changed, err
}

// updateStatementTxOutcome is updateStatementTx that also reports whether
// the write reset a vouched verification, and the label it replaced.
func (a *App) updateStatementTxOutcome(ctx context.Context, tx *sql.Tx, actor domain.Actor, invRef, ref string, k stmtKind,
	pp preparedPatch) (store.StatementRow, bool, UpdateOutcome, error) {
	var (
		out     store.StatementRow
		outcome UpdateOutcome
	)
	changedAny := false
	err := func() error {
		now := a.timestamp()
		t, cur, err := a.getStatement(ctx, tx, k, invRef, ref)
		if err != nil {
			return err
		}
		var (
			set          []store.Assignment
			changed      []string
			supersededTo string // replacement ref when this write supersedes
			reset        bool   // an edit lowered a vouched verification (ADR-0014)
		)
		// add records one assignment; enum fields name their new value in
		// the activity summary ("status disproven", "confidence high"),
		// free-text fields only their name.
		add := func(column, field string, v any) {
			set = append(set, store.Assignment{Column: column, Value: v})
			switch field {
			case "status", "confidence", "verification":
				changed = append(changed, fmt.Sprintf("%s %v", field, v))
			case "verification_reset":
				changed = append(changed, "verification")
			default:
				changed = append(changed, field)
			}
		}
		if pp.statement != nil && *pp.statement != cur.Statement {
			add("statement", "statement", *pp.statement)
		}
		if pp.confidence != nil && *pp.confidence != cur.Confidence {
			add("confidence", "confidence", string(*pp.confidence))
		}
		if pp.rationale != nil && *pp.rationale != cur.Rationale {
			add("rationale", k.rationaleField, *pp.rationale)
		}
		switch {
		case pp.replacement != nil:
			rep, err := resolveReplacement(ctx, tx, k, t.Inv, cur, *pp.replacement)
			if err != nil {
				return err
			}
			if rep.ID != cur.SupersededByID {
				add("superseded_by_id", "superseded_by", rep.ID)
				supersededTo = rep.Ref
			}
			if cur.Status != k.superseded {
				add("status", "status", k.superseded)
				supersededTo = rep.Ref
			}
			if cur.Verification != domain.VerificationSuperseded {
				add("verification", "verification", string(domain.VerificationSuperseded))
			}
		case pp.status != nil && *pp.status == k.superseded:
			if cur.SupersededByID == 0 {
				return domain.NewValidationError("superseded_by",
					"is required to set status superseded; name the replacing %s in superseded_by", k.rec)
			}
			if cur.Status != k.superseded {
				add("status", "status", k.superseded)
				supersededTo = cur.SupersededByRef
			}
			if cur.Verification != domain.VerificationSuperseded {
				add("verification", "verification", string(domain.VerificationSuperseded))
			}
		default:
			status, verification, err := k.pairState(cur, pp.status, pp.verification, actor.Kind)
			if err != nil {
				return err
			}
			if status != cur.Status {
				add("status", "status", status)
			}
			if status != k.superseded && cur.SupersededByID != 0 {
				add("superseded_by_id", "superseded_by", nil)
			}
			statementChanged := pp.statement != nil && *pp.statement != cur.Statement
			if resetsVerification(actor.Kind, verification, statementChanged, pp.verification != nil) {
				verification = domain.VerificationAIGenerated
				reset = true
			}
			if verification != cur.Verification {
				field := "verification"
				if reset {
					field = "verification_reset" // the reset note names the value
				}
				add("verification", field, string(verification))
			}
		}
		if len(set) == 0 {
			out = cur
			return nil
		}
		action := k.updateOp
		summary := fmt.Sprintf("Updated %s %s: %s", k.rec, cur.Ref, strings.Join(changed, ", "))
		if reset {
			summary += resetNote(actor.Kind)
		}
		if supersededTo != "" {
			action = k.supersedeOp
			summary = fmt.Sprintf("%s superseded by %s", cur.Ref, supersededTo)
			var also []string
			for _, c := range changed {
				name, _, _ := strings.Cut(c, " ")
				if name != "status" && name != "verification" && name != "superseded_by" {
					also = append(also, c)
				}
			}
			if len(also) > 0 {
				summary += "; also updated " + strings.Join(also, ", ")
			}
		}
		row, err := a.writeStatementChange(ctx, tx, actor, k, t, cur.ID, set, action, summary, now)
		if err != nil {
			return err
		}
		out = row
		changedAny = true
		if reset {
			outcome = UpdateOutcome{VerificationReset: true, PreviousVerification: cur.Verification}
		}
		return nil
	}()
	return out, changedAny, outcome, err
}

// setStatementVerification sets the verification state with the pairing
// rules of pairState (verification disputed disputes a claim; another
// verification on a disputed claim makes it active; superseded is refused)
// and records the update action ("Set verification of C-001 to disputed",
// plus "; status disputed" when the status follows). Setting the current
// value is a no-op with no activity.
func (a *App) setStatementVerification(ctx context.Context, actor Actor, invRef, ref string, k stmtKind,
	v domain.Verification) (store.StatementRow, error) {
	op := k.updateOp
	actor, err := actor.Validate()
	if err != nil {
		return store.StatementRow{}, a.fail(op, err)
	}
	if err := v.Validate("verification"); err != nil {
		return store.StatementRow{}, a.fail(op, err)
	}
	if err := checkVerifier(actor.Kind, v); err != nil {
		return store.StatementRow{}, a.fail(op, err)
	}
	var out store.StatementRow
	err = a.db.WithWriteTx(ctx, func(tx *sql.Tx) error {
		now := a.timestamp()
		t, cur, err := a.getStatement(ctx, tx, k, invRef, ref)
		if err != nil {
			return err
		}
		if cur.Verification == v {
			out = cur
			return nil
		}
		status, verification, err := k.pairState(cur, nil, &v, actor.Kind)
		if err != nil {
			return err
		}
		set := []store.Assignment{{Column: "verification", Value: string(verification)}}
		summary := fmt.Sprintf("Set verification of %s to %s", cur.Ref, verification)
		if status != cur.Status {
			set = append(set, store.Assignment{Column: "status", Value: status})
			summary += "; status " + status
		}
		row, err := a.writeStatementChange(ctx, tx, actor, k, t, cur.ID, set, op, summary, now)
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

// writeStatementChange applies set to record id (named by t), records one
// activity row, touches the investigation and reads the record back.
func (a *App) writeStatementChange(ctx context.Context, tx *sql.Tx, actor domain.Actor, k stmtKind, t recordTarget,
	id int64, set []store.Assignment, action, summary, now string) (store.StatementRow, error) {
	if err := store.UpdateStatement(ctx, tx, k.target, id, set, now); err != nil {
		return store.StatementRow{}, err
	}
	if err := a.recordActivity(ctx, tx, actor, t.Inv.ID, action, t.Ref, summary, "", now); err != nil {
		return store.StatementRow{}, err
	}
	if err := store.TouchInvestigation(ctx, tx, t.Inv.ID, now); err != nil {
		return store.StatementRow{}, err
	}
	return store.GetStatementByID(ctx, tx, k.target, id)
}
