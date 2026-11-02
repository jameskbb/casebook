package contextpack

import (
	"fmt"
	"strings"
	"time"

	"github.com/jameskbb/casebook/internal/domain"
)

// RenderMarkdown renders the pack in the layout of the brief's "Context
// Packs" example: LF line endings, one blank line between sections and a
// trailing newline. Labels come from the enum Label methods. Empty
// current claims, hypotheses and open questions print "(none)"; an unset
// primary question or conclusion prints "(not yet set)"; the other optional
// sections are left out when empty. A trimmed pack ends with a line naming
// the budget and the omitted sections. A record copied from an imported
// file shows "(imported)" after its ref, like an imported activity row.
//
// Every user text value (title, primary question, conclusion, rationale,
// statements, questions, notes, evidence titles and labels, activity
// summaries, client names) is rendered on one line with oneLine, so stored
// text can never start a new line and forge a section header, a claim line
// or a verification label: the only line breaks in the output are the
// layout's own.
func RenderMarkdown(p Pack) string {
	var sections []string
	add := func(lines ...string) { sections = append(sections, strings.Join(lines, "\n")) }

	add("Investigation: "+oneLine(p.Investigation.Ref),
		"Title: "+oneLine(p.Investigation.Title),
		"Status: "+p.Investigation.Status.Label())
	add("Primary question:", orNotSet(oneLine(p.Investigation.PrimaryQuestion)))
	add("Current conclusion:", orNotSet(oneLine(p.Conclusion.Text)))
	add("Conclusion confidence:", p.Conclusion.Confidence.Label())
	if p.Conclusion.Text != "" {
		add(conclusionAccountabilityLines(p.Conclusion, basisBrackets)...)
	}
	if rationale := oneLine(p.Conclusion.Rationale); rationale != "" {
		add("Conclusion rationale:", rationale)
	}

	lines := []string{"Current claims:"}
	for _, c := range p.EstablishedClaims {
		lines = append(lines, fmt.Sprintf("%s [%s, %s]", refTag(c.Ref, c.Imported), c.Confidence.Label(), c.Verification.Label()), oneLine(c.Statement))
		lines = append(lines, whyLine(c.Rationale)...)
		lines = append(lines, p.evidenceLines(c.SupportingEvidence, c.ContradictingEvidence)...)
	}
	add(listTail(lines, len(p.EstablishedClaims), p.Omitted.EstablishedClaims, "current claim", "current claims")...)

	if len(p.DisputedClaims) > 0 || p.Omitted.DisputedClaims > 0 {
		lines := []string{"Disputed claims:"}
		for _, c := range p.DisputedClaims {
			lines = append(lines, fmt.Sprintf("%s [%s, %s] %s", refTag(c.Ref, c.Imported), c.Confidence.Label(), c.Verification.Label(), oneLine(c.Statement)))
			lines = append(lines, p.evidenceLines(c.SupportingEvidence, c.ContradictingEvidence)...)
		}
		add(listTail(lines, len(p.DisputedClaims), p.Omitted.DisputedClaims, "disputed claim", "disputed claims")...)
	}

	lines = []string{"Hypotheses:"}
	for _, h := range p.Hypotheses {
		lines = append(lines, fmt.Sprintf("%s [%s, %s, %s]", refTag(h.Ref, h.Imported), h.Confidence.Label(), h.Status.Label(), h.Verification.Label()),
			oneLine(h.Statement))
		lines = append(lines, whyLine(h.Rationale)...)
		lines = append(lines, p.evidenceLines(h.SupportingEvidence, h.ContradictingEvidence)...)
	}
	add(listTail(lines, len(p.Hypotheses), p.Omitted.Hypotheses, "hypothesis", "hypotheses")...)

	lines = []string{"Open questions:"}
	for _, q := range p.OpenQuestions {
		lines = append(lines, refTag(q.Ref, q.Imported), oneLine(q.Question))
	}
	add(listTail(lines, len(p.OpenQuestions), p.Omitted.OpenQuestions, "open question", "open questions")...)

	if len(p.DeferredQuestions) > 0 {
		lines := []string{"Deferred questions:"}
		for _, q := range p.DeferredQuestions {
			line := refTag(q.Ref, q.Imported) + " " + oneLine(q.Question)
			if note := oneLine(q.Note); note != "" {
				line += " (note: " + note + ")"
			}
			lines = append(lines, line)
		}
		add(lines...)
	}

	if len(p.DisprovenHypotheses) > 0 || p.Omitted.DisprovenHypotheses > 0 {
		lines := []string{"Known disproven hypotheses:"}
		for _, h := range p.DisprovenHypotheses {
			lines = append(lines, refTag(h.Ref, h.Imported)+" "+oneLine(h.Text)+" ["+h.Verification.Label()+"]")
			if len(h.ContradictingEvidence) > 0 {
				lines = append(lines, "Contradicted by: "+p.evidenceRefs(h.ContradictingEvidence))
			}
		}
		add(listTail(lines, len(p.DisprovenHypotheses), p.Omitted.DisprovenHypotheses,
			"disproven hypothesis", "disproven hypotheses")...)
	}

	if len(p.EvidenceIndex) > 0 {
		lines := []string{"Evidence index:"}
		for _, e := range p.EvidenceIndex {
			source := oneLine(e.SourceLabel)
			if source == "" {
				source = e.SourceType.Label()
			}
			lines = append(lines, fmt.Sprintf("%s %s (%s) [%s]", refTag(e.Ref, e.Imported), oneLine(e.Title), source, verificationLabel(e.Verification)))
		}
		add(lines...)
	}

	if len(p.RecentChanges) > 0 {
		lines := []string{"Recent material changes:"}
		for _, c := range p.RecentChanges {
			who := string(c.Actor.Kind) + " " + oneLine(c.Actor.Client)
			if c.Imported {
				who += " (imported)"
			}
			lines = append(lines, fmt.Sprintf("- %s %s: %s", c.At, who, oneLine(c.Summary)))
		}
		add(lines...)
	}

	if p.Omitted.EvidenceOutsideFindings > 0 {
		add(fmt.Sprintf("Evidence outside the included findings: %d omitted.", p.Omitted.EvidenceOutsideFindings))
	}

	if p.Truncated {
		footer := fmt.Sprintf("(Trimmed to fit %d bytes", p.MaxBytes)
		if len(p.OmittedSections) > 0 {
			footer += "; omitted: " + strings.Join(p.OmittedSections, ", ")
		}
		add(footer + ")")
	}
	return strings.Join(sections, "\n\n") + "\n"
}

// oneLine renders user text on a single line: invalid UTF-8 and control
// characters are dropped, and every run of whitespace (newlines, tabs,
// spaces and Unicode separators such as U+2028) becomes one space.
func oneLine(s string) string {
	return strings.Join(strings.Fields(domain.TruncateLine(s, 0)), " ")
}

// refTag renders a record ref, followed by " (imported)" when the record
// was copied from an imported file.
func refTag(ref string, imported bool) string {
	if imported {
		return ref + " (imported)"
	}
	return ref
}

// orNotSet returns s, or "(not yet set)" when it is empty.
func orNotSet(s string) string {
	if s == "" {
		return "(not yet set)"
	}
	return s
}

// whyLine renders a claim or hypothesis rationale as "Why: ...", or nothing
// when it is empty.
func whyLine(rationale string) []string {
	if r := oneLine(rationale); r != "" {
		return []string{"Why: " + r}
	}
	return nil
}

// evidenceLines renders the evidence ref lines of a claim or hypothesis;
// each is left out when its list is empty.
func (p Pack) evidenceLines(supporting, contradicting []string) []string {
	var out []string
	if len(supporting) > 0 {
		out = append(out, "Evidence: "+p.evidenceRefs(supporting))
	}
	if len(contradicting) > 0 {
		out = append(out, "Contradicting evidence: "+p.evidenceRefs(contradicting))
	}
	return out
}

// listTail finishes a list section: "(none)" when it has no items and none
// were omitted, and an "(N more ... omitted)" line when the budget removed
// items.
func listTail(lines []string, kept, omitted int, singular, plural string) []string {
	if kept == 0 && omitted == 0 {
		return append(lines, "(none)")
	}
	if omitted > 0 {
		noun := plural
		if omitted == 1 {
			noun = singular
		}
		lines = append(lines, fmt.Sprintf("(%d more %s omitted)", omitted, noun))
	}
	return lines
}

// evidenceRefs keeps verification adjacent to each citation, including after
// the evidence index's titles and sources were omitted by the budget.
func (p Pack) evidenceRefs(refs []string) string {
	labelled := make([]string, 0, len(refs))
	for _, ref := range refs {
		labelled = append(labelled, ref+" ["+p.evidenceVerificationLabel(ref)+"]")
	}
	return strings.Join(labelled, ", ")
}

func (p Pack) evidenceVerificationLabel(ref string) string {
	for _, e := range p.EvidenceVerification {
		if e.Ref == ref {
			return verificationLabel(e.Verification)
		}
	}
	for _, e := range p.EvidenceIndex {
		if e.Ref == ref {
			return verificationLabel(e.Verification)
		}
	}
	return "Verification unavailable"
}

func verificationLabel(v domain.Verification) string {
	if v == "" {
		return "Verification unavailable"
	}
	return v.Label()
}

// basisStyle picks how conclusionAccountabilityLines writes a claim basis.
// Documents for people (brief and report) use parentheses joined by
// semicolons; packs keep square brackets and commas.
type basisStyle int

const (
	basisBrackets basisStyle = iota
	basisParens
)

// claimBasisLabel names a basis claim's state without repeating a word: a
// current claim shows only its verification label, a claim whose status and
// verification agree (Disputed, Superseded) shows the word once, and any
// other claim shows its status, then its verification label, joined by a
// semicolon inside parentheses and by a comma inside brackets.
func claimBasisLabel(claim PackConclusionClaim, style basisStyle) string {
	if claim.Missing {
		return "missing claim"
	}
	verification := verificationLabel(claim.Verification)
	status := claim.Status.Label()
	if claim.Status == domain.ClaimStatusActive || claim.Status == "" || status == verification {
		return verification
	}
	sep := ", "
	if style == basisParens {
		sep = "; "
	}
	return status + sep + verification
}

// readableTime renders an RFC 3339 timestamp as "YYYY-MM-DD HH:MM UTC" for
// readers. A value that does not parse is printed as stored.
func readableTime(ts string) string {
	t, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return oneLine(ts)
	}
	return t.UTC().Format("2006-01-02 15:04") + " UTC"
}

// These labels are mandatory: a budget may shorten prose, never verification
// or the fact that the selected basis needs a person's review.
func conclusionAccountabilityLines(c PackConclusion, style basisStyle) []string {
	verification := c.Verification
	if verification == "" {
		verification = domain.VerificationAIGenerated
	}
	open, closing, sep := " [", "]", ", "
	if style == basisParens {
		open, closing, sep = " (", ")", "; "
	}
	lines := []string{"Conclusion verification: " + verification.Label()}
	for _, group := range []struct {
		label  string
		claims []PackConclusionClaim
	}{{"Supporting claims", c.SupportingClaims}, {"Contradicting claims", c.ContradictingClaims}} {
		refs := []string{}
		for _, claim := range group.claims {
			refs = append(refs, oneLine(claim.Ref)+open+claimBasisLabel(claim, style)+closing)
		}
		if len(refs) > 0 {
			lines = append(lines, group.label+": "+strings.Join(refs, sep))
		} else {
			lines = append(lines, group.label+": none selected.")
		}
	}
	if c.ReviewedAt == "" {
		lines = append(lines, "Conclusion review: not reviewed; needs review.")
	} else {
		lines = append(lines, "Conclusion reviewed: "+readableTime(c.ReviewedAt))
		if c.ReviewBasis != "" {
			lines = append(lines, "Review basis: "+oneLine(c.ReviewBasis))
		}
		if c.Verification == domain.VerificationDisputed {
			lines = append(lines, "Conclusion needs review: the conclusion is disputed.")
		} else if c.NeedsReview {
			lines = append(lines, "Conclusion needs review: selected claims or their evidence changed since the recorded review.")
		}
	}
	return lines
}
