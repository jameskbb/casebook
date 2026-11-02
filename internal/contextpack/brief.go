package contextpack

import (
	"fmt"
	"slices"
	"strings"

	"github.com/jameskbb/casebook/internal/domain"
)

// BriefOmitted counts and names the records outside the person's selected
// leadership brief. The ref lists are in ref order; when they are empty but a
// count is not (an older caller), the notice falls back to the counts. The
// complete report remains available, including the activity history.
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
// recommendations. The app selects records and always keeps disputed findings,
// contradicting evidence and the claims behind the answer.
func RenderBrief(s Snapshot, omitted BriefOmitted) string {
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
	para("Leadership brief. Status: " + inv.Status.Label() + ".")
	para("## Question", orText(mdText(inv.ProblemStatement), "Not recorded."))
	para("## Answer")
	if inv.Conclusion == "" {
		para("No conclusion recorded yet.")
	} else {
		para(mdText(inv.Conclusion))
		c := PackConclusion{Verification: inv.ConclusionVerification, SupportingClaims: conclusionClaimBasis(inv.ConclusionSupportingClaims, s.Claims), ContradictingClaims: conclusionClaimBasis(inv.ConclusionContradictingClaims, s.Claims), ReviewedAt: inv.ConclusionReviewedAt, ReviewBasis: inv.ConclusionReviewBasis, NeedsReview: inv.ConclusionNeedsReview}
		for _, line := range conclusionAccountabilityLines(c, basisParens) {
			para(line)
		}
		para(strings.TrimSpace("Confidence: " + briefConfidenceWord(inv.ConclusionConfidence) + ". " + oneLine(inv.ConclusionRationale)))
	}
	links := func(rows []domain.EvidenceLink) {
		support, contra := linkRefs(rows)
		for _, g := range []struct {
			label string
			refs  []string
		}{{"Supported by", support}, {"Contradicted by", contra}} {
			for _, ref := range g.refs {
				line := "  - " + g.label + " " + ref + " [Verification unavailable]"
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
		para(fmt.Sprintf("- **%s** %s (%s)", refTag(h.Ref, h.Imported), oneLine(h.Statement), briefStateTail(h.Status.Label(), h.Confidence, h.Verification, h.Status != domain.HypothesisStatusDisproven)))
		if h.Rationale != "" {
			para("  - Why: " + oneLine(h.Rationale))
		}
		links(h.Links)
	}
	claim := func(c domain.Claim) {
		para(fmt.Sprintf("- **%s** %s (%s)", refTag(c.Ref, c.Imported), oneLine(c.Statement), briefStateTail(c.Status.Label(), c.Confidence, c.Verification, c.Status != domain.ClaimStatusRejected)))
		if c.ConfidenceRationale != "" {
			para("  - Why: " + oneLine(c.ConfidenceRationale))
		}
		links(c.Links)
	}
	para("## Selected findings and what is still in doubt")
	if len(claims) == 0 && len(hyps) == 0 {
		para("None recorded or selected.")
	}
	for _, c := range claims {
		claim(c)
	}
	for _, h := range hyps {
		hypothesis(h)
	}
	para("## Explanations still open or supported")
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
	para("## Selected questions")
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
	para("## Evidence and source caveats")
	if len(evidence) == 0 {
		para("None linked.")
	}
	for _, e := range evidence {
		para("- **" + refTag(e.Ref, e.Imported) + "** " + oneLine(e.Title) + " (" + sourceText(e) + "; " + verificationLabel(e.Verification) + ")")
		if e.Summary != "" {
			para("  - " + oneLine(e.Summary))
		}
	}
	para("## Scope and omissions")
	para(briefLabels)
	para(briefOmissionNotice(omitted))
	para("Disputed findings, contradicting evidence and the claims behind the answer are always included.")
	para("_Prepared by Casebook from the saved records of " + oneLine(inv.Ref) + ". Casebook did not rewrite them or add to them._")
	return b.String()
}

// briefLabels is the fixed legend a recipient without Casebook reads to
// understand every verification label in the brief. It is constant text.
const briefLabels = "Labels: Human verified means a person checked this and vouched for it. Source verified means it was marked as checked against its source, usually by an AI. AI generated means it has not been marked as checked by a person in Casebook. Disputed means evidence contradicts it; it stays on record until it is settled. Superseded means it was replaced by a newer statement."

// BriefStateLabel is briefStateTail for the Share summary selection list, so
// the sender sees each record's state in the words the brief prints.
func BriefStateLabel(status string, confidence domain.Confidence, v domain.Verification, withConfidence bool) string {
	return briefStateTail(status, confidence, v, withConfidence)
}

// briefStateTail writes the state in a brief finding line: status, then
// confidence when it means something (not on a rejected claim or a disproven
// hypothesis), then the verification label unless it repeats the status word.
func briefStateTail(status string, confidence domain.Confidence, v domain.Verification, withConfidence bool) string {
	parts := []string{status}
	if withConfidence {
		parts = append(parts, briefConfidence(confidence))
	}
	if label := verificationLabel(v); label != status {
		parts = append(parts, label)
	}
	return strings.Join(parts, "; ")
}

// briefOmissionNotice says what the brief leaves out: the omitted refs in
// order (claims, hypotheses, questions, evidence), or nonzero counts when no
// refs were supplied, then the activity history and where the recipient can
// get the complete report.
func briefOmissionNotice(o BriefOmitted) string {
	const tail = " Activity history is not included. Ask the person who shared this for the complete report."
	if refs := slices.Concat(o.ClaimRefs, o.HypothesisRefs, o.QuestionRefs, o.EvidenceRefs); len(refs) > 0 {
		return "Not included: " + joinList(refs) + "." + tail
	}
	var counts []string
	for _, n := range []struct {
		count            int
		singular, plural string
	}{{o.Claims, "claim", "claims"}, {o.Hypotheses, "hypothesis", "hypotheses"}, {o.Questions, "question", "questions"}, {o.Evidence, "evidence item", "evidence items"}} {
		switch {
		case n.count == 1:
			counts = append(counts, "1 "+n.singular)
		case n.count > 1:
			counts = append(counts, fmt.Sprintf("%d %s", n.count, n.plural))
		}
	}
	notice := "Every claim, hypothesis, question and evidence item in the case is included."
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

// briefConfidence and briefConfidenceWord say "Not assessed" where the enum
// label says Unknown: a person reads the brief, and the app uses those words.
func briefConfidence(c domain.Confidence) string {
	if c == domain.ConfidenceUnknown || c == "" {
		return "Confidence not assessed"
	}
	return c.Label() + " confidence"
}

func briefConfidenceWord(c domain.Confidence) string {
	if c == domain.ConfidenceUnknown || c == "" {
		return "Not assessed"
	}
	return c.Label()
}
