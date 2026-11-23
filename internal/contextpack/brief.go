package contextpack

import (
	"fmt"
	"slices"
	"strings"

	"github.com/jameskbb/casebook/internal/domain"
)

// BriefOmitted counts and names the records outside the person's selected
// short answer. The ref lists are in ref order; when they are empty but a
// count is not (an older caller), the notice falls back to the counts. The
// full version remains available, including the history.
type BriefOmitted struct {
	Claims         int      `json:"claims"`
	Hypotheses     int      `json:"hypotheses"`
	Questions      int      `json:"questions"`
	Evidence       int      `json:"evidence"`
	ClaimRefs      []string `json:"claim_refs"`
	HypothesisRefs []string `json:"hypothesis_refs"`
	QuestionRefs   []string `json:"question_refs"`
	EvidenceRefs   []string `json:"evidence_refs"`
}

// RenderBrief renders only supplied records without inventing a summary or
// recommendations. The app selects records and always keeps findings where
// sources disagree, contradicting sources and the findings behind the
// answer. allEvidence is every source of the question, selected or not: the
// "N of M sources checked by you" line counts them all, so leaving a source
// out of the selection cannot improve the count.
func RenderBrief(s Snapshot, omitted BriefOmitted, allEvidence []domain.Evidence) string {
	var b strings.Builder
	para := func(lines ...string) {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		for _, line := range lines {
			b.WriteString(line)
			b.WriteString("\n")
		}
	}
	inv := s.Investigation
	para("# " + oneLine(inv.Ref) + ": " + oneLine(inv.Title))
	para("## Question", orText(mdText(inv.ProblemStatement), "Not recorded."))
	para("## Answer")
	for _, line := range answerLines(s, allEvidence) {
		para(line)
	}
	links := func(rows []domain.EvidenceLink) {
		support, contra := linkRefs(rows)
		for _, g := range []struct {
			label string
			refs  []string
		}{{"Supported by", support}, {"Contradicted by", contra}} {
			for _, ref := range g.refs {
				line := "  - " + g.label + " " + ref + " (" + checkUnknown + ")"
				for _, e := range s.Evidence {
					if e.Ref == ref {
						line = "  - " + g.label + " " + ref + ": " + oneLine(e.Title) + " (" + verificationLabel(e.Verification) + ")"
						break
					}
				}
				para(line)
			}
		}
	}
	// Rejected claims and disproven hypotheses are dead ends: they go under
	// Ruled out, not among the things still in doubt.
	var claims, rejected []domain.Claim
	for _, c := range s.Claims {
		if c.Status == domain.ClaimStatusRejected {
			rejected = append(rejected, c)
		} else {
			claims = append(claims, c)
		}
	}
	sortByRef(claims, domain.RecordClaim, func(c domain.Claim) string { return c.Ref })
	sortByRef(rejected, domain.RecordClaim, func(c domain.Claim) string { return c.Ref })
	var hyps, live, disproven []domain.Hypothesis
	for _, h := range s.Hypotheses {
		switch h.Status {
		case domain.HypothesisStatusOpen, domain.HypothesisStatusSupported:
			live = append(live, h)
		case domain.HypothesisStatusDisproven:
			disproven = append(disproven, h)
		default:
			hyps = append(hyps, h)
		}
	}
	sortByRef(hyps, domain.RecordHypothesis, func(h domain.Hypothesis) string { return h.Ref })
	sortByRef(live, domain.RecordHypothesis, func(h domain.Hypothesis) string { return h.Ref })
	sortByRef(disproven, domain.RecordHypothesis, func(h domain.Hypothesis) string { return h.Ref })
	hypothesis := func(h domain.Hypothesis) {
		para(fmt.Sprintf("- **%s** %s (%s)", refTag(h.Ref, h.Imported), oneLine(h.Statement), BriefStateLabel(h.Status.PlainLabel(), h.Verification)))
		if h.Rationale != "" {
			para("  - Why: " + oneLine(h.Rationale))
		}
		links(h.Links)
	}
	claim := func(c domain.Claim) {
		para(fmt.Sprintf("- **%s** %s (%s)", refTag(c.Ref, c.Imported), oneLine(c.Statement), BriefStateLabel(c.Status.PlainLabel(), c.Verification)))
		if c.ConfidenceRationale != "" {
			para("  - Why: " + oneLine(c.ConfidenceRationale))
		}
		links(c.Links)
	}
	para("## Findings")
	if len(claims) == 0 && len(hyps) == 0 {
		para("None recorded or selected.")
	}
	for _, c := range claims {
		claim(c)
	}
	for _, h := range hyps {
		hypothesis(h)
	}
	para("## Still being tested")
	if len(live) == 0 {
		para("None selected.")
	}
	for _, h := range live {
		hypothesis(h)
	}
	para("## Ruled out")
	if len(rejected) == 0 && len(disproven) == 0 {
		para("None selected.")
	}
	for _, c := range rejected {
		claim(c)
	}
	for _, h := range disproven {
		hypothesis(h)
	}
	questions := slices.Clone(s.Questions)
	sortByRef(questions, domain.RecordQuestion, func(q domain.Question) string { return q.Ref })
	para("## Follow-up questions")
	if len(questions) == 0 {
		para("None selected.")
	}
	for _, q := range questions {
		para("- **" + refTag(q.Ref, q.Imported) + "** " + oneLine(q.Question) + " (" + q.Status.Label() + ")")
		if q.Resolution != "" {
			para("  - Answer: " + oneLine(q.Resolution))
		}
		if q.DeferNote != "" {
			para("  - Deferred: " + oneLine(q.DeferNote))
		}
	}
	evidence := slices.Clone(s.Evidence)
	sortByRef(evidence, domain.RecordEvidence, func(e domain.Evidence) string { return e.Ref })
	para("## Sources")
	if len(evidence) == 0 {
		para("None linked.")
	}
	for _, e := range evidence {
		para("- **" + refTag(e.Ref, e.Imported) + "** " + oneLine(e.Title) + " (" + sourceText(e) + "; " + verificationLabel(e.Verification) + ")")
		if e.Summary != "" {
			para("  - " + oneLine(e.Summary))
		}
	}
	para(BriefScopeHeading)
	para(briefLabels)
	para(briefOmissionNotice(omitted))
	para("Findings where sources disagree, sources that contradict a finding and the findings behind the answer are always included.")
	para("_Prepared by Casebook from the saved records of " + oneLine(inv.Ref) + ". Casebook did not rewrite them or add to them._")
	return b.String()
}

// answerLines renders the body of the Answer section of a shared answer:
// the answer text, the count of sources checked (in place of a confidence
// rating), the check and review labels, and why. evidence is every source
// of the question.
func answerLines(s Snapshot, evidence []domain.Evidence) []string {
	inv := s.Investigation
	if inv.Conclusion == "" {
		return []string{"No answer yet."}
	}
	lines := []string{mdText(inv.Conclusion)}
	if count := sourcesCheckedLine(evidence); count != "" {
		lines = append(lines, count)
	}
	c := PackConclusion{Verification: inv.ConclusionVerification, SupportingClaims: conclusionClaimBasis(inv.ConclusionSupportingClaims, s.Claims), ContradictingClaims: conclusionClaimBasis(inv.ConclusionContradictingClaims, s.Claims), ReviewedAt: inv.ConclusionReviewedAt, ReviewBasis: inv.ConclusionReviewBasis, NeedsReview: inv.ConclusionNeedsReview}
	lines = append(lines, conclusionAccountabilityLines(c, basisParens)...)
	if why := oneLine(inv.ConclusionRationale); why != "" {
		lines = append(lines, "Why: "+why)
	}
	return lines
}

// BriefScopeHeading opens the last section of the short answer, which says
// what was left out. A surface that cuts a long answer keeps everything from
// this heading on.
const BriefScopeHeading = "## What is included"

// briefLabels is the fixed legend a recipient without Casebook reads to
// understand every check label in the shared answer. It is constant text.
const briefLabels = "Labels: You checked this means the person who shared this checked it against its source. AI checked the source means an AI compared it with its source and no person has. Not checked means nobody has checked it. Sources disagree means a source contradicts it; it stays here until someone decides. Replaced means a newer finding took its place."

// BriefStateLabel writes the state in a finding line of a shared answer, and
// in the selection list, so the sender sees each record's state in the words
// the answer prints: the status (left out when it is Current), then the
// check label unless it repeats the status. status is the record's
// PlainLabel.
func BriefStateLabel(status string, v domain.Verification) string {
	label := verificationLabel(v)
	if status == "" || status == "Current" || status == label {
		return label
	}
	return status + "; " + label
}

// briefOmissionNotice says what the short answer leaves out: the omitted refs
// in order (claims, hypotheses, questions, evidence), or nonzero counts when
// no refs were supplied, then the history and where the recipient can get
// the full version.
func briefOmissionNotice(o BriefOmitted) string {
	const tail = " The history is not included. Ask the person who shared this for the full version."
	if refs := slices.Concat(o.ClaimRefs, o.HypothesisRefs, o.QuestionRefs, o.EvidenceRefs); len(refs) > 0 {
		return "Not included: " + joinList(refs) + "." + tail
	}
	var counts []string
	for _, n := range []struct {
		count            int
		singular, plural string
	}{{o.Claims + o.Hypotheses, "finding", "findings"}, {o.Questions, "follow-up question", "follow-up questions"}, {o.Evidence, "source", "sources"}} {
		switch {
		case n.count == 1:
			counts = append(counts, "1 "+n.singular)
		case n.count > 1:
			counts = append(counts, fmt.Sprintf("%d %s", n.count, n.plural))
		}
	}
	notice := "Every finding, follow-up question and source is included."
	if len(counts) > 0 {
		notice = "Not included: " + joinList(counts) + "."
	}
	return notice + tail
}

// joinList joins items as "a", "a and b" or "a, b and c".
func joinList(items []string) string {
	if len(items) <= 1 {
		return strings.Join(items, "")
	}
	return strings.Join(items[:len(items)-1], ", ") + " and " + items[len(items)-1]
}
