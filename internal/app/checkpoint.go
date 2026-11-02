package app

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/jameskbb/casebook/internal/domain"
	"github.com/jameskbb/casebook/internal/store"
)

// Checkpoint limits.
const (
	maxCheckpointChanges = 50
	checkpointItemBytes  = 200
)

// Checkpoint records a checkpoint with only a note. It is
// CheckpointBatch(ctx, actor, invRef, CheckpointInput{Note: note}).
func (a *App) Checkpoint(ctx context.Context, actor Actor, invRef, note string) (CheckpointResult, error) {
	return a.CheckpointBatch(ctx, actor, invRef, CheckpointInput{Note: note})
}

// CheckpointBatch applies an optional batch of writes and records a
// checkpoint for an investigation (empty invRef means the active one), all in
// one write transaction: either every item and the checkpoint row are
// written, or nothing is.
//
// Items apply in the order evidence, claims, claim updates, hypotheses,
// hypothesis updates, questions, question updates, conclusion, each with the
// rules and the one activity row of the matching single-record method
// (AddEvidence, AddClaim, UpdateClaim, ...). Claims and hypotheses link
// evidence created earlier in the batch with evidence_ref "#<key>". At most
// MaxCheckpointItems items (ValidationError on items). Every input is
// validated before the transaction; a failure names the item path
// (evidence[2].summary, claim_updates[0].status) and a not found or conflict
// error during the transaction is prefixed with the item path
// ("claim_updates[1]: not found: claim C-009 in INV-0001").
//
// The result reports the refs created and the refs actually changed
// (Created, Updated; no-op items are left out), the activity recorded since
// the previous checkpoint of that investigation including this batch's rows
// (newest first, at most 50, ChangesTruncated when more exist;
// PreviousCheckpointAt is "" on the first checkpoint), open questions, open
// hypotheses and claims to review (verification ai_generated or status
// disputed; superseded and rejected claims are skipped). Item texts are one
// line of at most 200 bytes.
//
// The note is required, at most 1000 bytes (ValidationError on note). It is
// stored in full in the checkpoint activity row's note; the row's summary is
// the note cut to one line. The checkpoint row itself does not touch the
// investigation's updated_at (the batch items do) and never appears in a
// context pack's recent changes.
func (a *App) CheckpointBatch(ctx context.Context, actor Actor, invRef string, in CheckpointInput) (CheckpointResult, error) {
	const op = "checkpoint"
	actor, err := actor.Validate()
	if err != nil {
		return CheckpointResult{}, a.fail(op, err)
	}
	b, err := prepareBatch(actor, in)
	if err != nil {
		return CheckpointResult{}, a.fail(op, err)
	}

	var out CheckpointResult
	err = a.db.WithWriteTx(ctx, func(tx *sql.Tx) error {
		inv, err := a.resolveInvestigation(ctx, tx, invRef)
		if err != nil {
			return err
		}
		prevID, prevAt, err := store.LatestActivity(ctx, tx, inv.ID, op)
		if err != nil {
			return err
		}
		created, updated, resets, err := a.applyBatch(ctx, tx, actor, inv.Ref, b)
		if err != nil {
			return err
		}
		changes, err := store.ListActivity(ctx, tx, store.ActivityQuery{
			InvestigationID: inv.ID, AfterID: prevID, Limit: maxCheckpointChanges + 1,
		})
		if err != nil {
			return err
		}
		res := CheckpointResult{
			InvestigationRef:     inv.Ref,
			Note:                 b.note,
			Created:              created,
			Updated:              updated,
			VerificationResets:   resets,
			PreviousCheckpointAt: prevAt,
			Changes:              changes,
		}
		if len(changes) > maxCheckpointChanges {
			res.Changes = changes[:maxCheckpointChanges]
			res.ChangesTruncated = true
		}
		if res.ActivityID, err = store.InsertActivity(ctx, tx, store.NewActivity{
			InvestigationID: inv.ID,
			Actor:           actor,
			Action:          op,
			TargetRef:       inv.Ref,
			Summary:         domain.TruncateLine(b.note, domain.MaxActivitySummaryBytes),
			Note:            b.note,
			CreatedAt:       a.timestamp(),
		}); err != nil {
			return err
		}
		if res.OpenQuestions, res.OpenHypotheses, res.ClaimsToReview, err = openItems(ctx, tx, inv.ID); err != nil {
			return err
		}
		out = res
		return nil
	})
	if err != nil {
		return CheckpointResult{}, a.fail(op, err)
	}
	a.log.Debug(op, "investigation_ref", out.InvestigationRef, "changes", len(out.Changes),
		"created", len(out.Created), "updated", len(out.Updated))
	return out, nil
}

// evidenceKeyPattern is the shape of a checkpoint evidence key.
var evidenceKeyPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,32}$`)

// batchQuestionUpdate is a validated QuestionUpdate.
type batchQuestionUpdate struct {
	ref     string
	text    *string
	op      string // question.resolve, question.defer, question.reopen or ""
	plan    questionPlan
	hasText bool
}

// batchStmt is a claim or hypothesis to add; in keeps the raw links so
// #key refs can be resolved inside the transaction.
type batchStmt struct {
	in stmtInput
	pa preparedAdd
}

// preparedBatch is a fully validated CheckpointInput.
type preparedBatch struct {
	note              string
	evidence          []evidenceFields
	keys              []string // per evidence item, "" when none
	claims            []batchStmt
	claimUpdates      []preparedRefPatch
	hypotheses        []batchStmt
	hypothesisUpdates []preparedRefPatch
	questions         []string
	questionUpdates   []batchQuestionUpdate
	conclusion        *preparedConclusion
}

type preparedRefPatch struct {
	ref string
	pp  preparedPatch
}

// itemError prefixes a ValidationError's field with path, and wraps other
// caller-facing errors with the path; storage errors pass unchanged so
// retries and the error mapping still see them.
func itemError(path string, err error) error {
	if err == nil {
		return nil
	}
	if ve, ok := domain.IsValidation(err); ok {
		return &domain.ValidationError{Field: path + "." + ve.Field, Message: ve.Message}
	}
	if passThrough(err) {
		return fmt.Errorf("%s: %w", path, err)
	}
	return err
}

// keyPlaceholder stands in for a #key evidence ref while links are validated
// before the transaction.
const keyPlaceholder = "E-001"

// prepareBatch validates every item of in before any write.
func prepareBatch(actor domain.Actor, in CheckpointInput) (preparedBatch, error) {
	var (
		b   preparedBatch
		err error
	)
	if b.note, err = domain.SanitizeText("note", in.Note, domain.MaxCheckpointNoteBytes, true); err != nil {
		return b, err
	}
	n := len(in.Evidence) + len(in.Claims) + len(in.ClaimUpdates) + len(in.Hypotheses) +
		len(in.HypothesisUpdates) + len(in.Questions) + len(in.QuestionUpdates)
	if in.Conclusion != nil {
		n++
	}
	if n > domain.MaxCheckpointItems {
		return b, domain.NewValidationError("items",
			"at most %d items per checkpoint (got %d); split the work into several checkpoints", domain.MaxCheckpointItems, n)
	}

	known := map[string]bool{}
	for i, e := range in.Evidence {
		path := fmt.Sprintf("evidence[%d]", i)
		if e.Key != "" {
			if !evidenceKeyPattern.MatchString(e.Key) {
				return b, domain.NewValidationError(path+".key", "must be 1 to 32 of A-Z, a-z, 0-9, _ and -")
			}
			if known[e.Key] {
				return b, domain.NewValidationError(path+".key", "duplicate key %q", e.Key)
			}
			known[e.Key] = true
		}
		f, err := cleanEvidenceInput(actor, e.Evidence)
		if err != nil {
			return b, itemError(path, err)
		}
		b.evidence = append(b.evidence, f)
		b.keys = append(b.keys, e.Key)
	}

	stmts := func(name string, k stmtKind, ins []stmtInput) ([]batchStmt, error) {
		out := make([]batchStmt, 0, len(ins))
		for i, si := range ins {
			path := fmt.Sprintf("%s[%d]", name, i)
			check := si
			check.Links = append([]domain.EvidenceLinkInput(nil), si.Links...)
			for j, l := range check.Links {
				if key, ok := strings.CutPrefix(strings.TrimSpace(l.EvidenceRef), "#"); ok {
					if !known[key] {
						return nil, domain.NewValidationError(fmt.Sprintf("%s.links[%d].evidence_ref", path, j),
							"unknown key %q; define it with evidence[].key in this checkpoint", l.EvidenceRef)
					}
					check.Links[j].EvidenceRef = keyPlaceholder
				}
			}
			pa, err := k.prepareAdd(actor, check)
			if err != nil {
				return nil, itemError(path, err)
			}
			out = append(out, batchStmt{in: si, pa: pa})
		}
		return out, nil
	}
	claims := make([]stmtInput, len(in.Claims))
	for i, c := range in.Claims {
		claims[i] = stmtInput{Statement: c.Statement, Confidence: c.Confidence, Rationale: c.ConfidenceRationale,
			Status: string(c.Status), Verification: c.Verification, Links: c.Links}
	}
	if b.claims, err = stmts("claims", claimKind, claims); err != nil {
		return b, err
	}
	hyps := make([]stmtInput, len(in.Hypotheses))
	for i, h := range in.Hypotheses {
		hyps[i] = stmtInput{Statement: h.Statement, Confidence: h.Confidence, Rationale: h.Rationale,
			Status: string(h.Status), Verification: h.Verification, Links: h.Links}
	}
	if b.hypotheses, err = stmts("hypotheses", hypothesisKind, hyps); err != nil {
		return b, err
	}

	patch := func(path, ref string, k stmtKind, p stmtPatch) (preparedRefPatch, error) {
		if strings.TrimSpace(ref) == "" {
			return preparedRefPatch{}, domain.NewValidationError(path+".ref", "is required")
		}
		if p == (stmtPatch{}) {
			return preparedRefPatch{}, domain.NewValidationError(path+".patch",
				"pass at least one of statement, confidence, rationale, status, verification or superseded_by")
		}
		pp, err := k.preparePatch(actor, p, "superseded_by")
		if err != nil {
			return preparedRefPatch{}, itemError(path+".patch", err)
		}
		return preparedRefPatch{ref: ref, pp: pp}, nil
	}
	for i, u := range in.ClaimUpdates {
		var status *string
		if u.Patch.Status != nil {
			s := string(*u.Patch.Status)
			status = &s
		}
		rp, err := patch(fmt.Sprintf("claim_updates[%d]", i), u.Ref, claimKind, stmtPatch{
			Statement: u.Patch.Statement, Rationale: u.Patch.ConfidenceRationale, Confidence: u.Patch.Confidence,
			Status: status, Verification: u.Patch.Verification, SupersededBy: u.Patch.SupersededBy,
		})
		if err != nil {
			return b, err
		}
		b.claimUpdates = append(b.claimUpdates, rp)
	}
	for i, u := range in.HypothesisUpdates {
		var status *string
		if u.Patch.Status != nil {
			s := string(*u.Patch.Status)
			status = &s
		}
		rp, err := patch(fmt.Sprintf("hypothesis_updates[%d]", i), u.Ref, hypothesisKind, stmtPatch{
			Statement: u.Patch.Statement, Rationale: u.Patch.Rationale, Confidence: u.Patch.Confidence,
			Status: status, Verification: u.Patch.Verification, SupersededBy: u.Patch.SupersededBy,
		})
		if err != nil {
			return b, err
		}
		b.hypothesisUpdates = append(b.hypothesisUpdates, rp)
	}

	for i, q := range in.Questions {
		text, err := cleanQuestion(q.Question)
		if err != nil {
			return b, itemError(fmt.Sprintf("questions[%d]", i), err)
		}
		b.questions = append(b.questions, text)
	}
	for i, u := range in.QuestionUpdates {
		qu, err := prepareQuestionUpdate(u)
		if err != nil {
			return b, itemError(fmt.Sprintf("question_updates[%d]", i), err)
		}
		b.questionUpdates = append(b.questionUpdates, qu)
	}

	if in.Conclusion != nil {
		c, err := prepareConclusion(*in.Conclusion)
		if err != nil {
			return b, itemError("conclusion", err)
		}
		b.conclusion = &c
	}
	return b, nil
}

// prepareQuestionUpdate validates one QuestionUpdate (fields relative to the
// item).
func prepareQuestionUpdate(u QuestionUpdate) (batchQuestionUpdate, error) {
	qu := batchQuestionUpdate{ref: u.Ref}
	if strings.TrimSpace(u.Ref) == "" {
		return qu, domain.NewValidationError("ref", "is required")
	}
	if u.Question != nil {
		text, err := cleanQuestion(*u.Question)
		if err != nil {
			return qu, err
		}
		qu.text, qu.hasText = &text, true
	}
	modes := 0
	if u.Resolution != nil {
		modes++
		res, err := domain.SanitizeText("resolution", *u.Resolution, domain.MaxResolutionBytes, true)
		if err != nil {
			return qu, err
		}
		qu.op, qu.plan = "question.resolve", resolvePlan(res)
	}
	if u.Defer {
		modes++
		note, err := domain.SanitizeText("note", u.Note, domain.MaxDeferNoteBytes, false)
		if err != nil {
			return qu, err
		}
		qu.op, qu.plan = "question.defer", deferPlan(note)
	} else if strings.TrimSpace(u.Note) != "" {
		return qu, domain.NewValidationError("note", "is only used with defer")
	}
	if u.Reopen {
		modes++
		qu.op, qu.plan = "question.reopen", reopenPlan
	}
	if modes > 1 {
		return qu, domain.NewValidationError("resolution", "pass at most one of resolution, defer or reopen")
	}
	if modes == 0 && !qu.hasText {
		return qu, domain.NewValidationError("question", "pass question text and/or one of resolution, defer or reopen")
	}
	return qu, nil
}

// applyBatch writes a prepared batch inside tx for investigation invRef
// (canonical) and returns the refs created, the refs changed and the claims
// and hypotheses whose vouched verification an edit reset (ADR-0014). It
// reads b and never modifies it, so a retried transaction starts clean.
func (a *App) applyBatch(ctx context.Context, tx *sql.Tx, actor domain.Actor, invRef string,
	b preparedBatch) ([]string, []string, []domain.VerificationReset, error) {
	created, updated := []string{}, []string{}
	var resets []domain.VerificationReset
	touched := func(ref string, changed bool) {
		if changed && !slices.Contains(updated, ref) {
			updated = append(updated, ref)
		}
	}
	keyRefs := map[string]string{}
	for i, f := range b.evidence {
		ev, err := a.addEvidenceTx(ctx, tx, actor, invRef, f)
		if err != nil {
			return nil, nil, nil, itemError(fmt.Sprintf("evidence[%d]", i), err)
		}
		created = append(created, ev.Ref)
		if b.keys[i] != "" {
			keyRefs[b.keys[i]] = ev.Ref
		}
	}
	addStmts := func(name string, k stmtKind, list []batchStmt) error {
		for i, st := range list {
			path := fmt.Sprintf("%s[%d]", name, i)
			pa := st.pa
			if hasKeyLink(st.in.Links) {
				in := st.in
				in.Links = make([]domain.EvidenceLinkInput, len(st.in.Links))
				for j, l := range st.in.Links {
					in.Links[j] = l
					if key, ok := strings.CutPrefix(strings.TrimSpace(l.EvidenceRef), "#"); ok {
						in.Links[j].EvidenceRef = keyRefs[key]
					}
				}
				links, err := parseLinkInputs(in.Links)
				if err != nil {
					return itemError(path, err)
				}
				pa.links = links
			}
			row, err := a.addStatementTx(ctx, tx, actor, invRef, k, pa)
			if err != nil {
				return itemError(path, err)
			}
			created = append(created, row.Ref)
		}
		return nil
	}
	updStmts := func(name string, k stmtKind, list []preparedRefPatch) error {
		for i, u := range list {
			row, changed, outcome, err := a.updateStatementTxOutcome(ctx, tx, actor, invRef, u.ref, k, u.pp)
			if err != nil {
				path := fmt.Sprintf("%s[%d]", name, i)
				if _, ok := domain.IsValidation(err); ok {
					path += ".patch" // field errors name the patch field
				}
				return itemError(path, err)
			}
			touched(row.Ref, changed)
			if outcome.VerificationReset {
				resets = append(resets, domain.VerificationReset{
					Type: k.rec, Ref: row.Ref, PreviousVerification: outcome.PreviousVerification,
				})
			}
		}
		return nil
	}
	if err := addStmts("claims", claimKind, b.claims); err != nil {
		return nil, nil, nil, err
	}
	if err := updStmts("claim_updates", claimKind, b.claimUpdates); err != nil {
		return nil, nil, nil, err
	}
	if err := addStmts("hypotheses", hypothesisKind, b.hypotheses); err != nil {
		return nil, nil, nil, err
	}
	if err := updStmts("hypothesis_updates", hypothesisKind, b.hypothesisUpdates); err != nil {
		return nil, nil, nil, err
	}
	for i, text := range b.questions {
		q, err := a.addQuestionTx(ctx, tx, actor, invRef, text)
		if err != nil {
			return nil, nil, nil, itemError(fmt.Sprintf("questions[%d]", i), err)
		}
		created = append(created, q.Ref)
	}
	for i, u := range b.questionUpdates {
		path := fmt.Sprintf("question_updates[%d]", i)
		if u.hasText {
			text := *u.text
			q, changed, err := a.changeQuestionTx(ctx, tx, actor, invRef, u.ref, "question.update",
				func(cur domain.Question, _ string) (questionTarget, string) {
					return questionTarget{question: &text},
						fmt.Sprintf("Updated question %s: %s", cur.Ref, domain.TruncateLine(text, questionSummaryText))
				})
			if err != nil {
				return nil, nil, nil, itemError(path, err)
			}
			touched(q.Ref, changed)
		}
		if u.plan != nil {
			q, changed, err := a.changeQuestionTx(ctx, tx, actor, invRef, u.ref, u.op, u.plan)
			if err != nil {
				return nil, nil, nil, itemError(path, err)
			}
			touched(q.Ref, changed)
		}
	}
	if b.conclusion != nil {
		inv, changed, err := a.conclusionTx(ctx, tx, actor, invRef, *b.conclusion)
		if err != nil {
			return nil, nil, nil, itemError("conclusion", err)
		}
		touched(inv.Ref, changed)
	}
	return created, updated, resets, nil
}

// hasKeyLink reports whether any link names batch evidence by #key.
func hasKeyLink(links []domain.EvidenceLinkInput) bool {
	for _, l := range links {
		if strings.HasPrefix(strings.TrimSpace(l.EvidenceRef), "#") {
			return true
		}
	}
	return false
}

// openItems loads the items a checkpoint asks to revisit, each ordered by
// ref and non-nil: open questions, open hypotheses, and claims that are
// ai_generated or disputed (never superseded or rejected).
func openItems(ctx context.Context, q store.Querier, invID int64) (questions, hyps, claims []CheckpointItem, err error) {
	questions, hyps, claims = []CheckpointItem{}, []CheckpointItem{}, []CheckpointItem{}

	qs, err := store.ListQuestions(ctx, q, invID)
	if err != nil {
		return nil, nil, nil, err
	}
	for _, qq := range qs {
		if qq.Status != domain.QuestionStatusOpen {
			continue
		}
		questions = append(questions, CheckpointItem{
			Type:   domain.RecordQuestion,
			Ref:    qq.Ref,
			Text:   domain.TruncateLine(qq.Question, checkpointItemBytes),
			Status: string(qq.Status),
		})
	}

	hs, err := store.ListStatements(ctx, q, store.LinkHypothesis, invID)
	if err != nil {
		return nil, nil, nil, err
	}
	for _, h := range hs {
		if domain.HypothesisStatus(h.Status) != domain.HypothesisStatusOpen {
			continue
		}
		hyps = append(hyps, statementItem(domain.RecordHypothesis, h))
	}

	cs, err := store.ListStatements(ctx, q, store.LinkClaim, invID)
	if err != nil {
		return nil, nil, nil, err
	}
	for _, c := range cs {
		st := domain.ClaimStatus(c.Status)
		if st == domain.ClaimStatusSuperseded || st == domain.ClaimStatusRejected {
			continue
		}
		if c.Verification != domain.VerificationAIGenerated && st != domain.ClaimStatusDisputed {
			continue
		}
		claims = append(claims, statementItem(domain.RecordClaim, c))
	}
	return questions, hyps, claims, nil
}

// statementItem converts a claim or hypothesis row to a CheckpointItem.
func statementItem(t domain.RecordType, r store.StatementRow) CheckpointItem {
	return CheckpointItem{
		Type:         t,
		Ref:          r.Ref,
		Text:         domain.TruncateLine(r.Statement, checkpointItemBytes),
		Status:       r.Status,
		Confidence:   r.Confidence,
		Verification: r.Verification,
	}
}
