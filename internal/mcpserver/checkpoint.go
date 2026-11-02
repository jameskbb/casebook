package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jameskbb/casebook/internal/app"
)

// casebook_checkpoint (D-19): the single call hooks ask the model to make.
// The batch goes to app.CheckpointBatch, which applies it and records the
// checkpoint row in one write transaction (AR-06).

// Checkpoint summary limits.
const (
	// checkpointListCap bounds each list in the checkpoint summary.
	checkpointListCap = 20
	// checkpointTextLines bounds the open-item lines per list in the text.
	checkpointTextLines = 5
)

type checkpointEvidence struct {
	Key string `json:"key,omitempty" jsonschema:"optional key such as new1 (1 to 32 of A-Z a-z 0-9 _ -, matched exactly); claims and hypotheses in this call link it as evidence_ref #new1"`
	evidenceFields
}

type checkpointClaimUpdate struct {
	Ref string `json:"ref" jsonschema:"claim ref such as C-001 (case-insensitive)"`
	claimPatchFields
}

type checkpointHypothesisUpdate struct {
	Ref string `json:"ref" jsonschema:"hypothesis ref such as H-001 (case-insensitive)"`
	hypothesisPatchFields
}

type checkpointQuestionUpdate struct {
	Ref string `json:"ref" jsonschema:"question ref such as Q-001 (case-insensitive)"`
	questionModeFields
}

type checkpointConclusion struct {
	Verification        statementVerification `json:"verification,omitempty" jsonschema:"ai_generated, source_verified or disputed; only a person can human-verify"`
	SupportingClaims    []string              `json:"supporting_claims,omitempty" jsonschema:"selected supporting claim refs; omit to preserve"`
	ContradictingClaims []string              `json:"contradicting_claims,omitempty" jsonschema:"selected contradicting claim refs; omit to preserve"`
	Conclusion          string                `json:"conclusion" jsonschema:"the conclusion; an empty string clears it"`
	Confidence          app.Confidence        `json:"confidence,omitempty" jsonschema:"confidence in the conclusion; omit for unknown"`
	Rationale           string                `json:"rationale,omitempty" jsonschema:"why the conclusion holds, naming the supporting evidence and claims"`
}

type checkpointInput struct {
	Note              string                       `json:"note" jsonschema:"concise summary of what was learned since the last checkpoint (at most 1000 bytes); never a transcript or chain-of-thought"`
	InvestigationRef  string                       `json:"investigation_ref,omitempty" jsonschema:"investigation ref such as INV-0001; omit to use the active investigation"`
	Evidence          []checkpointEvidence         `json:"evidence,omitempty" jsonschema:"new evidence, fields as in casebook_add_evidence, plus an optional key (1 to 32 of A-Z a-z 0-9 _ -) that claims and hypotheses in this call link as evidence_ref #key"`
	Claims            []claimFields                `json:"claims,omitempty" jsonschema:"new claims, fields as in casebook_add_claim; links[].evidence_ref may be an E ref or #key"`
	ClaimUpdates      []checkpointClaimUpdate      `json:"claim_updates,omitempty" jsonschema:"claim patches: ref (C-001) plus at least one field of casebook_update_claim"`
	Hypotheses        []hypothesisFields           `json:"hypotheses,omitempty" jsonschema:"new hypotheses, fields as in casebook_add_hypothesis; links[].evidence_ref may be an E ref or #key"`
	HypothesisUpdates []checkpointHypothesisUpdate `json:"hypothesis_updates,omitempty" jsonschema:"hypothesis patches: ref (H-001) plus at least one field of casebook_update_hypothesis"`
	Questions         []string                     `json:"questions,omitempty" jsonschema:"new open question texts (each at most 2000 bytes)"`
	QuestionUpdates   []checkpointQuestionUpdate   `json:"question_updates,omitempty" jsonschema:"question changes: ref (Q-001) plus exactly one mode of casebook_resolve_question (resolution, defer with optional note, or reopen)"`
	Conclusion        *checkpointConclusion        `json:"conclusion,omitempty" jsonschema:"replaces the conclusion, its confidence and rationale together, as casebook_update_conclusion; applied last"`
}

// checkpointSchema is the casebook_checkpoint input schema: the inferred
// one with the property descriptions inside batch items removed. Every item
// field is described once on its single-record tool, and the top-level
// descriptions point there; repeating them made this one schema about 9 KB
// of every session's tools/list (AR-05). Types, enums and required lists
// stay, so clients still validate every item.
func checkpointSchema() *jsonschema.Schema {
	s := inputSchema[checkpointInput]()
	for _, p := range s.Properties {
		dropDescriptions(p.Items)
		if p.Items == nil {
			// The conclusion object: keep its own description, drop its
			// fields' (they repeat casebook_update_conclusion's).
			for _, f := range p.Properties {
				dropDescriptions(f)
			}
		}
	}
	return s
}

// dropDescriptions removes the description of s and of every schema below
// it.
func dropDescriptions(s *jsonschema.Schema) {
	if s == nil {
		return
	}
	s.Description = ""
	for _, p := range s.Properties {
		dropDescriptions(p)
	}
	dropDescriptions(s.Items)
}

func (in *checkpointInput) setExactJSON(args json.RawMessage) error {
	var raw struct {
		Evidence []struct {
			Metadata json.RawMessage `json:"metadata"`
		} `json:"evidence"`
	}
	if err := json.Unmarshal(args, &raw); err != nil {
		return err
	}
	for i := range in.Evidence {
		if i < len(raw.Evidence) {
			in.Evidence[i].metadataText = string(raw.Evidence[i].Metadata)
		}
	}
	return nil
}

type checkpointChange struct {
	Action    string `json:"action"`
	TargetRef string `json:"target_ref"`
	Summary   string `json:"summary"`
}

type checkpointSummary struct {
	ActivityID int64 `json:"activity_id"`
	// PreviousCheckpointAt is "" on the first checkpoint.
	PreviousCheckpointAt string               `json:"previous_checkpoint_at"`
	Changes              []checkpointChange   `json:"changes"`
	ChangesTotal         int                  `json:"changes_total"`
	ChangesTruncated     bool                 `json:"changes_truncated"`
	OpenQuestions        []app.CheckpointItem `json:"open_questions"`
	OpenQuestionsTotal   int                  `json:"open_questions_total"`
	OpenHypotheses       []app.CheckpointItem `json:"open_hypotheses"`
	OpenHypothesesTotal  int                  `json:"open_hypotheses_total"`
	ClaimsToReview       []app.CheckpointItem `json:"claims_to_review"`
	ClaimsToReviewTotal  int                  `json:"claims_to_review_total"`
}

type checkpointOut struct {
	InvestigationRef string   `json:"investigation_ref"`
	Created          []string `json:"created"`
	Updated          []string `json:"updated"`
	// VerificationResets names the updated claims and hypotheses whose
	// vouched verification this call's edits reset to ai_generated
	// (ADR-0014); omitted when none.
	VerificationResets []app.VerificationReset `json:"verification_resets,omitempty"`
	Checkpoint         checkpointSummary       `json:"checkpoint"`
}

const checkpointDescription = `Save progress in one call: call it before your final reply in a session that worked on a case (an investigation record), before context compaction, or when a hook asks, batching every material discovery not yet recorded (new evidence, claims, hypotheses, questions, updates and the conclusion). ` +
	`note (required, at most 1000 bytes) is a concise summary of what was learned; never paste a transcript or chain-of-thought. ` +
	`Batches apply first, in order evidence, claims, claim_updates, hypotheses, hypothesis_updates, questions, question_updates, conclusion; link evidence by its key as #key from claims or hypotheses. At most 50 items. ` +
	`All or nothing: on any error nothing is written and the error names the failing item (for example claims[1].links[0].evidence_ref), so fix it and resend the whole call. ` +
	`Returns created and updated refs, changes since the previous checkpoint and open items to revisit.`

func init() {
	registerTool("casebook_checkpoint", func(s *mcp.Server, h *handlers) {
		addTool(s, h, &mcp.Tool{
			Name:        "casebook_checkpoint",
			Title:       "Checkpoint",
			Description: checkpointDescription,
			InputSchema: checkpointSchema(),
			Annotations: mutatingTool(false),
		}, h.checkpoint)
	})
}

// toCheckpointInput converts the tool input to the engine batch. The engine
// validates everything (item count, keys, #key links, every field) before
// its one write transaction, so nothing is checked twice here.
func (in checkpointInput) toCheckpointInput() (app.CheckpointInput, error) {
	out := app.CheckpointInput{Note: in.Note}
	for i, e := range in.Evidence {
		ev, err := e.toInput()
		if err != nil {
			return app.CheckpointInput{}, rePath(err, fmt.Sprintf("evidence[%d].", i))
		}
		out.Evidence = append(out.Evidence, app.CheckpointEvidence{Key: e.Key, Evidence: ev})
	}
	for _, c := range in.Claims {
		out.Claims = append(out.Claims, c.toInput())
	}
	for _, u := range in.ClaimUpdates {
		out.ClaimUpdates = append(out.ClaimUpdates, app.ClaimUpdate{Ref: u.Ref, Patch: u.toPatch()})
	}
	for _, h := range in.Hypotheses {
		out.Hypotheses = append(out.Hypotheses, h.toInput())
	}
	for _, u := range in.HypothesisUpdates {
		out.HypothesisUpdates = append(out.HypothesisUpdates, app.HypothesisUpdate{Ref: u.Ref, Patch: u.toPatch()})
	}
	for _, q := range in.Questions {
		out.Questions = append(out.Questions, app.QuestionInput{Question: q})
	}
	for i, u := range in.QuestionUpdates {
		// The tool has no question text field, so a change means exactly
		// one of the modes (the engine allows text with at most one).
		if _, err := u.mode(); err != nil {
			return app.CheckpointInput{}, rePath(err, fmt.Sprintf("question_updates[%d].", i))
		}
		qu := app.QuestionUpdate{Ref: u.Ref, Defer: u.Defer, Note: u.Note, Reopen: u.Reopen}
		if u.Resolution != "" {
			r := u.Resolution
			qu.Resolution = &r
		}
		out.QuestionUpdates = append(out.QuestionUpdates, qu)
	}
	if cc := in.Conclusion; cc != nil {
		out.Conclusion = &app.ConclusionInput{Conclusion: cc.Conclusion, Confidence: cc.Confidence, Rationale: cc.Rationale, Verification: app.Verification(cc.Verification), SupportingClaims: cc.SupportingClaims, ContradictingClaims: cc.ContradictingClaims}
	}
	return out, nil
}

// rePath prefixes the field of a ValidationError with prefix; other errors
// are returned as is.
func rePath(err error, prefix string) error {
	if ve, ok := app.IsValidation(err); ok {
		return app.NewValidationError(prefix+ve.Field, "%s", ve.Message)
	}
	return err
}

// Engine path segments the tool input flattens: the ".patch" of claim and
// hypothesis updates, and the ".question" of new questions (plain strings
// in the tool).
var (
	patchSegment    = regexp.MustCompile(`^((?:claim|hypothesis)_updates\[\d+\])\.patch(\.|$)`)
	questionSegment = regexp.MustCompile(`^(questions\[\d+\])\.question$`)
)

// toolPath rewrites an engine ValidationError path to the tool's input
// shape (claim_updates[0].patch.status becomes claim_updates[0].status,
// questions[1].question becomes questions[1]).
func toolPath(err error) error {
	ve, ok := app.IsValidation(err)
	if !ok {
		return err
	}
	field := patchSegment.ReplaceAllString(ve.Field, "$1$2")
	field = questionSegment.ReplaceAllString(field, "$1")
	if field == ve.Field {
		return err
	}
	return app.NewValidationError(field, "%s", ve.Message)
}

func (h *handlers) checkpoint(ctx context.Context, c call, in checkpointInput) (checkpointOut, string, error) {
	var zero checkpointOut
	batch, err := in.toCheckpointInput()
	if err != nil {
		return zero, "", err
	}
	res, err := h.app.CheckpointBatch(ctx, c.Actor, in.InvestigationRef, batch)
	if err != nil {
		return zero, "", toolPath(err)
	}
	if h.opts.OnCheckpoint != nil {
		h.opts.OnCheckpoint(res.InvestigationRef)
	}
	out := checkpointOut{
		InvestigationRef:   res.InvestigationRef,
		Created:            nonNil(res.Created),
		Updated:            nonNil(res.Updated),
		VerificationResets: res.VerificationResets,
		Checkpoint:         checkpointSummaryOf(res),
	}
	return out, checkpointText(out), nil
}

// nonNil returns list, or an empty slice for nil, so JSON shows [].
func nonNil(list []string) []string {
	if list == nil {
		return []string{}
	}
	return list
}

// capItems keeps at most checkpointListCap items (never nil) and the total.
func capItems(items []app.CheckpointItem) ([]app.CheckpointItem, int) {
	n := min(len(items), checkpointListCap)
	out := make([]app.CheckpointItem, n)
	copy(out, items[:n])
	return out, len(items)
}

func checkpointSummaryOf(res app.CheckpointResult) checkpointSummary {
	s := checkpointSummary{
		ActivityID:           res.ActivityID,
		PreviousCheckpointAt: res.PreviousCheckpointAt,
		Changes:              []checkpointChange{},
		ChangesTotal:         len(res.Changes),
		ChangesTruncated:     res.ChangesTruncated || len(res.Changes) > checkpointListCap,
	}
	for _, ch := range res.Changes[:min(len(res.Changes), checkpointListCap)] {
		s.Changes = append(s.Changes, checkpointChange{Action: ch.Action, TargetRef: ch.TargetRef, Summary: oneLine(ch.Summary)})
	}
	s.OpenQuestions, s.OpenQuestionsTotal = capItems(res.OpenQuestions)
	s.OpenHypotheses, s.OpenHypothesesTotal = capItems(res.OpenHypotheses)
	s.ClaimsToReview, s.ClaimsToReviewTotal = capItems(res.ClaimsToReview)
	return s
}

// checkpointResetText names each claim or hypothesis whose check an edit in
// the batch removed, in the words of resetSentence, or "" when none.
func checkpointResetText(resets []app.VerificationReset) string {
	if len(resets) == 0 {
		return ""
	}
	var b strings.Builder
	for _, r := range resets {
		fmt.Fprintf(&b, " %s %s was %s; the edit reset it to AI generated.",
			upperFirst(string(r.Type)), r.Ref, r.PreviousVerification.Label())
	}
	if len(resets) == 1 {
		b.WriteString(" Tell the person and ask them to review it again.")
	} else {
		b.WriteString(" Tell the person and ask them to review them again.")
	}
	return b.String()
}

// upperFirst capitalises the first ASCII letter of s.
func upperFirst(s string) string {
	if s == "" || s[0] < 'a' || s[0] > 'z' {
		return s
	}
	return string(s[0]-'a'+'A') + s[1:]
}

// checkpointText is the content text of a recorded checkpoint.
func checkpointText(out checkpointOut) string {
	var b strings.Builder
	s := out.Checkpoint
	fmt.Fprintf(&b, "Checkpoint recorded for %s (activity %d).", out.InvestigationRef, s.ActivityID)
	if len(out.Created) > 0 {
		fmt.Fprintf(&b, " Created: %s.", strings.Join(out.Created, ", "))
	}
	if len(out.Updated) > 0 {
		fmt.Fprintf(&b, " Updated: %s.", strings.Join(out.Updated, ", "))
	}
	b.WriteString(checkpointResetText(out.VerificationResets))
	since := s.PreviousCheckpointAt
	if since == "" {
		since = "the start"
	}
	more := ""
	if s.ChangesTruncated {
		more = " or more"
	}
	fmt.Fprintf(&b, "\nSince %s: %d%s changes.", since, s.ChangesTotal, more)
	items := func(title string, list []app.CheckpointItem, total int) {
		if total == 0 {
			return
		}
		fmt.Fprintf(&b, "\n%s (%d):", title, total)
		for _, it := range list[:min(len(list), checkpointTextLines)] {
			fmt.Fprintf(&b, "\n- %s %s", it.Ref, it.Text)
		}
		if shown := min(len(list), checkpointTextLines); total > shown {
			fmt.Fprintf(&b, "\n(%d more)", total-shown)
		}
	}
	items("Open questions", s.OpenQuestions, s.OpenQuestionsTotal)
	items("Open hypotheses", s.OpenHypotheses, s.OpenHypothesesTotal)
	items("Claims to review", s.ClaimsToReview, s.ClaimsToReviewTotal)
	return b.String()
}
