package app

import (
	"cmp"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"slices"

	"github.com/jameskbb/casebook/internal/contextpack"
	"github.com/jameskbb/casebook/internal/domain"
)

// reportActivityRows is how many of the newest activity rows a report
// reads for its timeline. An investigation with more rows gets a timeline
// that says earlier changes are not shown.
const reportActivityRows = 2000

// ReportOptions tunes Report. TimelineLimit > 0 keeps only the newest
// TimelineLimit material changes in the timeline; 0 keeps every change the
// report reads (up to the newest 2000 activity rows).
type ReportOptions struct {
	TimelineLimit int `json:"timeline_limit"`
}

// Report renders a deterministic Markdown write-up of an investigation
// (empty invRef means the active one) for a pull request description, an
// incident document or a postmortem: title, question, conclusion with
// confidence and rationale, established claims with their verification and
// evidence, what was ruled out and what contradicted it, answered and open
// questions, the evidence and a dated timeline of material changes with
// their actors. See contextpack.RenderReport for the layout. The same
// database state always gives the same bytes. A negative TimelineLimit is a
// ValidationError on timeline_limit.
func (a *App) Report(ctx context.Context, invRef string, o ReportOptions) (string, error) {
	const op = "report.render"
	if o.TimelineLimit < 0 {
		return "", a.fail(op, domain.NewValidationError("timeline_limit", "must be 0 or more"))
	}
	var snap contextpack.Snapshot
	err := a.db.WithReadTx(ctx, func(tx *sql.Tx) error {
		s, err := a.readSnapshot(ctx, tx, invRef, reportActivityRows+1)
		snap = s
		return err
	})
	if err != nil {
		return "", a.fail(op, err)
	}
	more := len(snap.Activity) > reportActivityRows
	if more {
		snap.Activity = newestActivity(snap.Activity, reportActivityRows)
	}
	text := contextpack.RenderReport(snap, contextpack.ReportOptions{TimelineLimit: o.TimelineLimit, MoreActivity: more})
	a.log.Debug(op, "investigation_ref", snap.Investigation.Ref, "bytes", len(text))
	return text, nil
}

// ReportHTML renders Report as HTML for a preview or a saved file; see
// contextpack.ReportHTML for what it escapes.
func (a *App) ReportHTML(ctx context.Context, invRef string, o ReportOptions) (string, error) {
	md, err := a.Report(ctx, invRef, o)
	if err != nil {
		return "", err
	}
	return contextpack.ReportHTML(md), nil
}

// newestActivity returns the n rows with the largest ids, newest first.
func newestActivity(rows []domain.Activity, n int) []domain.Activity {
	out := slices.Clone(rows)
	slices.SortFunc(out, func(x, y domain.Activity) int { return cmp.Compare(y.ID, x.ID) })
	return out[:n]
}

// ShareOptions selects existing records for a leadership brief. Nil selections
// choose a small initial set; an empty slice explicitly selects none. Material
// caveats and the conclusion's claim basis are always retained. Ruled-out
// findings remain selectable rather than expanding the brief into a history.
// HypothesisRefs selects open or supported hypotheses (group "hypotheses").
type ShareOptions struct {
	Mode           string   `json:"mode"`
	ClaimRefs      []string `json:"claim_refs"`
	HypothesisRefs []string `json:"hypothesis_refs"`
	QuestionRefs   []string `json:"question_refs"`
	RuledOutRefs   []string `json:"ruled_out_refs"`
}

// ShareCandidate is a selectable existing finding or question. Required records
// retain the answer's basis, disputes and contradictions in every brief. Group
// is claims, hypotheses (open or supported), questions or ruled_out; Label is
// the state the brief prints for the record.
type ShareCandidate struct {
	Ref      string `json:"ref"`
	Text     string `json:"text"`
	Group    string `json:"group"`
	Label    string `json:"label"`
	Required bool   `json:"required"`
	Selected bool   `json:"selected"`
}

// SharePreview contains both formats derived from the same read transaction.
// SnapshotID identifies the exact case content and bytes reviewed by a person.
type SharePreview struct {
	Markdown         string                   `json:"markdown"`
	HTML             string                   `json:"html"`
	SnapshotID       string                   `json:"snapshot_id"`
	InvestigationRef string                   `json:"investigation_ref"`
	Mode             string                   `json:"mode"`
	Candidates       []ShareCandidate         `json:"candidates"`
	Omitted          contextpack.BriefOmitted `json:"omitted"`
}

// SharePreview renders a deterministic leadership brief or complete report from
// one database snapshot. It reads no clock and performs no writes.
func (a *App) SharePreview(ctx context.Context, invRef string, o ShareOptions) (SharePreview, error) {
	const op = "share.preview"
	if o.Mode == "" {
		o.Mode = "brief"
	}
	if o.Mode != "brief" && o.Mode != "report" {
		return SharePreview{}, a.fail(op, domain.NewValidationError("mode", "must be brief or report"))
	}
	var s contextpack.Snapshot
	err := a.db.WithReadTx(ctx, func(tx *sql.Tx) error {
		var err error
		s, err = a.readSnapshot(ctx, tx, invRef, reportActivityRows+1)
		return err
	})
	if err != nil {
		return SharePreview{}, a.fail(op, err)
	}
	out := SharePreview{InvestigationRef: s.Investigation.Ref, Mode: o.Mode, Candidates: []ShareCandidate{}}
	selected := contextpack.Snapshot{Investigation: s.Investigation}
	evidenceRefs := map[string]bool{}
	disputedSources := map[string]bool{}
	for _, e := range s.Evidence {
		if e.Verification == domain.VerificationDisputed {
			evidenceRefs[e.Ref] = true
			disputedSources[e.Ref] = true
		}
	}
	addLinks := func(links []domain.EvidenceLink) {
		for _, l := range links {
			evidenceRefs[l.EvidenceRef] = true
		}
	}
	materialSourceCaveat := func(links []domain.EvidenceLink) bool {
		for _, l := range links {
			if l.Relation == domain.RelationContradicts || disputedSources[l.EvidenceRef] {
				return true
			}
		}
		return false
	}
	wants := func(ref string, refs []string, initial bool) bool {
		if refs == nil {
			return initial
		}
		return slices.Contains(refs, ref)
	}
	slices.SortFunc(s.Claims, func(a, b domain.Claim) int { return briefRefOrder(a.Ref, b.Ref, domain.RecordClaim) })
	slices.SortFunc(s.Hypotheses, func(a, b domain.Hypothesis) int { return briefRefOrder(a.Ref, b.Ref, domain.RecordHypothesis) })
	slices.SortFunc(s.Questions, func(a, b domain.Question) int { return briefRefOrder(a.Ref, b.Ref, domain.RecordQuestion) })
	claimCount, hypothesisCount, ruledCount, questionCount := 0, 0, 0, 0
	for _, c := range s.Claims {
		current := c.Status != domain.ClaimStatusRejected && c.Status != domain.ClaimStatusSuperseded && c.Verification != domain.VerificationSuperseded
		required := (current && (c.Status == domain.ClaimStatusDisputed || c.Verification == domain.VerificationDisputed || materialSourceCaveat(c.Links))) || slices.Contains(s.Investigation.ConclusionSupportingClaims, c.Ref) || slices.Contains(s.Investigation.ConclusionContradictingClaims, c.Ref)
		group := "claims"
		refs := o.ClaimRefs
		initial := claimCount < 3 && c.Status == domain.ClaimStatusActive && c.Verification != domain.VerificationSuperseded
		if c.Status == domain.ClaimStatusRejected {
			group = "ruled_out"
			refs = o.RuledOutRefs
			initial = ruledCount < 2
			ruledCount++
		} else if initial {
			claimCount++
		}
		chosen := required || wants(c.Ref, refs, initial)
		out.Candidates = append(out.Candidates, ShareCandidate{Ref: c.Ref, Text: c.Statement, Group: group, Label: contextpack.BriefStateLabel(c.Status.Label(), c.Confidence, c.Verification, c.Status != domain.ClaimStatusRejected), Required: required, Selected: chosen})
		if chosen {
			selected.Claims = append(selected.Claims, c)
			addLinks(c.Links)
		}
	}
	for _, h := range s.Hypotheses {
		current := h.Status != domain.HypothesisStatusDisproven && h.Status != domain.HypothesisStatusSuperseded && h.Verification != domain.VerificationSuperseded
		required := current && (h.Verification == domain.VerificationDisputed || materialSourceCaveat(h.Links))
		chosen := required
		switch h.Status {
		case domain.HypothesisStatusDisproven:
			chosen = required || wants(h.Ref, o.RuledOutRefs, ruledCount < 2)
			ruledCount++
			out.Candidates = append(out.Candidates, ShareCandidate{Ref: h.Ref, Text: h.Statement, Group: "ruled_out", Label: contextpack.BriefStateLabel(h.Status.Label(), h.Confidence, h.Verification, false), Required: required, Selected: chosen})
		case domain.HypothesisStatusOpen, domain.HypothesisStatusSupported:
			initial := hypothesisCount < 2 && h.Verification != domain.VerificationSuperseded
			if initial {
				hypothesisCount++
			}
			chosen = required || wants(h.Ref, o.HypothesisRefs, initial)
			out.Candidates = append(out.Candidates, ShareCandidate{Ref: h.Ref, Text: h.Statement, Group: "hypotheses", Label: contextpack.BriefStateLabel(h.Status.Label(), h.Confidence, h.Verification, true), Required: required, Selected: chosen})
		}
		if chosen {
			selected.Hypotheses = append(selected.Hypotheses, h)
			addLinks(h.Links)
		}
	}
	for _, q := range s.Questions {
		initial := questionCount < 2 && q.Status != domain.QuestionStatusResolved
		if initial {
			questionCount++
		}
		chosen := wants(q.Ref, o.QuestionRefs, initial)
		out.Candidates = append(out.Candidates, ShareCandidate{Ref: q.Ref, Text: q.Question, Group: "questions", Label: q.Status.Label(), Selected: chosen})
		if chosen {
			selected.Questions = append(selected.Questions, q)
		}
	}
	for _, group := range []struct {
		field string
		refs  []string
		kind  string
	}{{"claim_refs", o.ClaimRefs, "claims"}, {"hypothesis_refs", o.HypothesisRefs, "hypotheses"}, {"question_refs", o.QuestionRefs, "questions"}, {"ruled_out_refs", o.RuledOutRefs, "ruled_out"}} {
		for _, ref := range group.refs {
			found := false
			for _, c := range out.Candidates {
				if c.Ref == ref && c.Group == group.kind {
					found = true
					break
				}
			}
			if !found {
				return SharePreview{}, a.fail(op, domain.NewValidationError(group.field, "%s is not an available selection", ref))
			}
		}
	}
	for _, e := range s.Evidence {
		if evidenceRefs[e.Ref] {
			selected.Evidence = append(selected.Evidence, e)
		}
	}
	out.Omitted = briefOmitted(s, selected)
	if o.Mode == "report" {
		more := len(s.Activity) > reportActivityRows
		if more {
			s.Activity = newestActivity(s.Activity, reportActivityRows)
		}
		out.Markdown = contextpack.RenderReport(s, contextpack.ReportOptions{MoreActivity: more})
		out.Omitted = briefOmitted(s, s)
	} else {
		out.Markdown = contextpack.RenderBrief(selected, out.Omitted)
	}
	out.HTML = contextpack.ReportHTML(out.Markdown)
	raw, _ := json.Marshal(s)
	sum := sha256.Sum256(append(raw, []byte(out.Markdown)...))
	out.SnapshotID = hex.EncodeToString(sum[:])
	return out, nil
}

// briefOmitted counts and names, in ref order, the records of s that selected
// leaves out. The ref lists are never nil so the desktop always reads arrays.
func briefOmitted(s, selected contextpack.Snapshot) contextpack.BriefOmitted {
	o := contextpack.BriefOmitted{ClaimRefs: []string{}, HypothesisRefs: []string{}, QuestionRefs: []string{}, EvidenceRefs: []string{}}
	for _, c := range s.Claims {
		if !slices.ContainsFunc(selected.Claims, func(x domain.Claim) bool { return x.Ref == c.Ref }) {
			o.ClaimRefs = append(o.ClaimRefs, c.Ref)
		}
	}
	for _, h := range s.Hypotheses {
		if !slices.ContainsFunc(selected.Hypotheses, func(x domain.Hypothesis) bool { return x.Ref == h.Ref }) {
			o.HypothesisRefs = append(o.HypothesisRefs, h.Ref)
		}
	}
	for _, q := range s.Questions {
		if !slices.ContainsFunc(selected.Questions, func(x domain.Question) bool { return x.Ref == q.Ref }) {
			o.QuestionRefs = append(o.QuestionRefs, q.Ref)
		}
	}
	for _, e := range s.Evidence {
		if !slices.ContainsFunc(selected.Evidence, func(x domain.Evidence) bool { return x.Ref == e.Ref }) {
			o.EvidenceRefs = append(o.EvidenceRefs, e.Ref)
		}
	}
	slices.SortStableFunc(o.EvidenceRefs, func(a, b string) int { return briefRefOrder(a, b, domain.RecordEvidence) })
	o.Claims, o.Hypotheses, o.Questions, o.Evidence = len(o.ClaimRefs), len(o.HypothesisRefs), len(o.QuestionRefs), len(o.EvidenceRefs)
	return o
}

// Snapshot records already have canonical refs. Compare their numeric part so
// default selection stays in ref order when a case grows beyond 999 records.
func briefRefOrder(a, b string, kind domain.RecordType) int {
	x, _ := domain.ParseRecordRef(a, kind)
	y, _ := domain.ParseRecordRef(b, kind)
	return cmp.Compare(x.Num, y.Num)
}
