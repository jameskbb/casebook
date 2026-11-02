package mcpserver

import (
	"fmt"
	"strings"

	"github.com/jameskbb/casebook/internal/app"
)

// Compact record projections shared by every tool (D-15). They carry refs,
// states and link refs, never raw detail or long text. Every slice field is
// non-nil so no output contains a JSON null.

// conclusionAccountability is kept on compact headers as well as full reads.
type conclusionAccountability struct {
	ConclusionVerification        app.Verification `json:"conclusion_verification"`
	ConclusionSupportingClaims    []string         `json:"conclusion_supporting_claims"`
	ConclusionContradictingClaims []string         `json:"conclusion_contradicting_claims"`
	ConclusionReviewedAt          string           `json:"conclusion_reviewed_at"`
	ConclusionReviewBasis         string           `json:"conclusion_review_basis"`
	ConclusionNeedsReview         bool             `json:"conclusion_needs_review"`
}

func conclusionAccountabilityOf(inv app.Investigation) conclusionAccountability {
	return conclusionAccountability{inv.ConclusionVerification, nonNilStrings(inv.ConclusionSupportingClaims), nonNilStrings(inv.ConclusionContradictingClaims), inv.ConclusionReviewedAt, inv.ConclusionReviewBasis, inv.ConclusionNeedsReview}
}
func nonNilStrings(refs []string) []string {
	if refs == nil {
		return []string{}
	}
	return refs
}

// investigationRecord is the compact form of one investigation.
type investigationRecord struct {
	conclusionAccountability
	Ref                  string                  `json:"ref"`
	Title                string                  `json:"title"`
	Status               app.InvestigationStatus `json:"status"`
	ConclusionConfidence app.Confidence          `json:"conclusion_confidence"`
	IsActive             bool                    `json:"is_active"`
	Archived             bool                    `json:"archived"`
	UpdatedAt            string                  `json:"updated_at"`
}

func investigationRecordOf(inv app.Investigation) investigationRecord {
	return investigationRecord{
		conclusionAccountability: conclusionAccountabilityOf(inv),
		Ref:                      inv.Ref,
		Title:                    inv.Title,
		Status:                   inv.Status,
		ConclusionConfidence:     inv.ConclusionConfidence,
		IsActive:                 inv.IsActive,
		Archived:                 inv.Archived,
		UpdatedAt:                inv.UpdatedAt,
	}
}

// evidenceRecord is the compact form of one evidence.
type evidenceRecord struct {
	Ref             string           `json:"ref"`
	Title           string           `json:"title"`
	SourceType      app.SourceType   `json:"source_type"`
	SourceLabel     string           `json:"source_label"`
	Verification    app.Verification `json:"verification"`
	HasRawDetail    bool             `json:"has_raw_detail"`
	ClaimLinks      int              `json:"claim_links"`
	HypothesisLinks int              `json:"hypothesis_links"`
	// Imported marks a row copied from an imported file: its author and
	// verification came from that file, not from this database (AR-09).
	Imported bool `json:"imported,omitempty"`
}

func evidenceRecordOf(e app.Evidence) evidenceRecord {
	return evidenceRecord{
		Ref:             e.Ref,
		Title:           e.Title,
		SourceType:      e.SourceType,
		SourceLabel:     e.SourceLabel,
		Verification:    e.Verification,
		HasRawDetail:    e.HasRawDetail,
		ClaimLinks:      len(e.ClaimLinks),
		HypothesisLinks: len(e.HypothesisLinks),
		Imported:        e.Imported,
	}
}

// maxLinkRefs caps each relation list of a statementRecord.
const maxLinkRefs = 100

// statementRecord is the compact form of a claim or hypothesis.
type statementRecord struct {
	Ref          string           `json:"ref"`
	Status       string           `json:"status"`
	Confidence   app.Confidence   `json:"confidence"`
	Verification app.Verification `json:"verification"`
	SupersededBy string           `json:"superseded_by"`
	// Supports and Contradicts list linked evidence refs by relation, in
	// evidence ref order.
	// Each list holds at most maxLinkRefs refs; LinksOmitted counts the
	// refs left out of both lists so a heavily linked record cannot blow the
	// response budget.
	Supports     []string `json:"supports"`
	Contradicts  []string `json:"contradicts"`
	LinksOmitted int      `json:"links_omitted,omitempty"`
	// Text is the statement on one line (at most 240 bytes); read views only.
	Text string `json:"text,omitempty"`
	// Imported: see evidenceRecord.Imported.
	Imported bool `json:"imported,omitempty"`
}

func statementRecordOf(ref, status string, conf app.Confidence, ver app.Verification, supersededBy, statement string,
	links []app.EvidenceLink, withText, imported bool) statementRecord {
	r := statementRecord{
		Imported:     imported,
		Ref:          ref,
		Status:       status,
		Confidence:   conf,
		Verification: ver,
		SupersededBy: supersededBy,
		Supports:     []string{},
		Contradicts:  []string{},
	}
	for _, l := range links {
		switch l.Relation {
		case app.RelationSupports:
			if len(r.Supports) < maxLinkRefs {
				r.Supports = append(r.Supports, l.EvidenceRef)
				continue
			}
		case app.RelationContradicts:
			if len(r.Contradicts) < maxLinkRefs {
				r.Contradicts = append(r.Contradicts, l.EvidenceRef)
				continue
			}
		default:
			continue
		}
		r.LinksOmitted++
	}
	if withText {
		r.Text = oneLine(statement)
	}
	return r
}

func claimRecordOf(c app.Claim, withText bool) statementRecord {
	return statementRecordOf(c.Ref, string(c.Status), c.Confidence, c.Verification, c.SupersededBy, c.Statement,
		c.Links, withText, c.Imported)
}

func hypothesisRecordOf(h app.Hypothesis, withText bool) statementRecord {
	return statementRecordOf(h.Ref, string(h.Status), h.Confidence, h.Verification, h.SupersededBy, h.Statement,
		h.Links, withText, h.Imported)
}

// questionRecord is the compact form of one question.
type questionRecord struct {
	Ref        string             `json:"ref"`
	Status     app.QuestionStatus `json:"status"`
	Text       string             `json:"text,omitempty"`
	Resolution string             `json:"resolution,omitempty"`
	DeferNote  string             `json:"defer_note,omitempty"`
	ResolvedAt string             `json:"resolved_at,omitempty"`
	Imported   bool               `json:"imported,omitempty"`
}

func questionRecordOf(q app.Question, withText bool) questionRecord {
	r := questionRecord{Ref: q.Ref, Status: q.Status, ResolvedAt: q.ResolvedAt, Imported: q.Imported}
	if withText {
		r.Text = oneLine(q.Question)
		r.Resolution = oneLine(q.Resolution)
		r.DeferNote = oneLine(q.DeferNote)
	}
	return r
}

// activityRecord is the compact form of one activity row.
type activityRecord struct {
	Action    string    `json:"action"`
	TargetRef string    `json:"target_ref"`
	Actor     app.Actor `json:"actor"`
	Summary   string    `json:"summary"`
	CreatedAt string    `json:"created_at"`
	Imported  bool      `json:"imported,omitempty"`
}

func activityRecordOf(a app.Activity) activityRecord {
	return activityRecord{
		Imported:  a.Imported,
		Action:    a.Action,
		TargetRef: a.TargetRef,
		Actor:     a.Actor,
		Summary:   a.Summary,
		CreatedAt: a.CreatedAt,
	}
}

// itemList is a possibly truncated list.
type itemList[T any] struct {
	Items     []T  `json:"items"`
	Total     int  `json:"total"`
	Truncated bool `json:"truncated"`
}

// newItemList keeps at most limit of items and records the total.
func newItemList[T any](items []T, limit int) itemList[T] {
	n := min(len(items), limit)
	out := make([]T, n)
	copy(out, items[:n])
	return itemList[T]{Items: out, Total: len(items), Truncated: n < len(items)}
}

// mutationResult is the output of every mutating tool.
type mutationResult[R any] struct {
	Ref              string `json:"ref"`
	InvestigationRef string `json:"investigation_ref"`
	// Action is one of created, updated, unchanged, activated, linked,
	// relinked, unlinked, superseded, resolved, deferred, reopened.
	Action string `json:"action"`
	// Changed lists the changed field names (never their values).
	Changed []string `json:"changed"`
	Record  R        `json:"record"`
	resetFields
}

// resetFields is set on an update whose edit reset a vouched verification
// to ai_generated (ADR-0014); both fields are omitted otherwise.
type resetFields struct {
	// VerificationReset is true when the edit removed a check.
	VerificationReset bool `json:"verification_reset,omitempty"`
	// PreviousVerification is the label the edit replaced.
	PreviousVerification app.Verification `json:"previous_verification,omitempty"`
}

// resetFieldsOf converts an app update outcome.
func resetFieldsOf(o app.UpdateOutcome) resetFields {
	if !o.VerificationReset {
		return resetFields{}
	}
	return resetFields{VerificationReset: true, PreviousVerification: o.PreviousVerification}
}

// recordWord names the record type of a canonical ref for result text:
// claim, hypothesis or evidence.
func recordWord(ref string) string {
	switch {
	case strings.HasPrefix(ref, "C-"):
		return "claim"
	case strings.HasPrefix(ref, "H-"):
		return "hypothesis"
	case strings.HasPrefix(ref, "E-"):
		return "evidence"
	}
	return "record"
}

// resetSentence tells the AI client, in words it can relay, that its edit
// removed a check on ref (language guide section 2).
func resetSentence(ref string, prev app.Verification) string {
	return fmt.Sprintf("This %s was %s; the edit reset it to AI generated. Tell the person and ask them to review it again.",
		recordWord(ref), prev.Label())
}

// changedFields returns, in order, the names whose values differ between
// before and after. The result is never nil.
func changedFields(order []string, before, after map[string]string) []string {
	out := []string{}
	for _, name := range order {
		if before[name] != after[name] {
			out = append(out, name)
		}
	}
	return out
}
