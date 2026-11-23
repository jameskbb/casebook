package contextpack

import (
	"fmt"
	"strings"
	"time"

	"github.com/jameskbb/casebook/internal/domain"
)

// RenderMarkdown renders the pack in its fixed Markdown layout: LF line endings, one blank line between sections and a
// trailing newline. It says question, answer, finding and source: a claim
// (C ref) and a hypothesis (H ref) are both findings, evidence (E ref) is a
// source, and an open question (Q ref) is a follow-up question. Refs and the
// JSON rendering keep the engine's names. Confidence is not printed. Empty
// findings and follow-up questions print "(none)"; an unset question or
// answer prints "(not yet set)"; the other optional sections are left out
// when empty. A trimmed pack ends with a line naming the budget and the
// omitted sections. A record copied from an imported file shows "(imported)"
// after its ref, like an imported activity row.
//
// Every user text value (title, question, answer, why, statements, follow-up
// questions, notes, source titles and labels, activity summaries, client
// names) is rendered on one line with oneLine, so stored text can never
// start a new line and forge a section header, a finding line or a check
// label: the only line breaks in the output are the layout's own.
func RenderMarkdown(p Pack) string {
	var sections []string
	add := func(lines ...string) { sections = append(sections, strings.Join(lines, "\n")) }

	add("Question: "+oneLine(p.Investigation.Ref),
		"Title: "+oneLine(p.Investigation.Title),
		"Status: "+questionStatusWord(p.Investigation.Status))
	add("The question:", orNotSet(oneLine(p.Investigation.PrimaryQuestion)))
	add("Answer so far:", orNotSet(oneLine(p.Conclusion.Text)))
	if p.Conclusion.Text != "" {
		add(conclusionAccountabilityLines(p.Conclusion, basisBrackets)...)
	}
	if rationale := oneLine(p.Conclusion.Rationale); rationale != "" {
		add("Why this answer:", rationale)
	}

	lines := []string{"Findings:"}
	for _, c := range p.EstablishedClaims {
		lines = append(lines, fmt.Sprintf("%s [%s]", refTag(c.Ref, c.Imported), verificationLabel(c.Verification)), oneLine(c.Statement))
		lines = append(lines, whyLine(c.Rationale)...)
		lines = append(lines, p.evidenceLines(c.SupportingEvidence, c.ContradictingEvidence)...)
	}
	add(listTail(lines, len(p.EstablishedClaims), p.Omitted.EstablishedClaims, "finding", "findings")...)

	if len(p.DisputedClaims) > 0 || p.Omitted.DisputedClaims > 0 {
		lines := []string{"Findings where sources disagree:"}
		for _, c := range p.DisputedClaims {
			lines = append(lines, fmt.Sprintf("%s [%s] %s", refTag(c.Ref, c.Imported), verificationLabel(c.Verification), oneLine(c.Statement)))
			lines = append(lines, p.evidenceLines(c.SupportingEvidence, c.ContradictingEvidence)...)
		}
		add(listTail(lines, len(p.DisputedClaims), p.Omitted.DisputedClaims,
			"finding where sources disagree", "findings where sources disagree")...)
	}

	lines = []string{"Findings still being tested:"}
	for _, h := range p.Hypotheses {
		lines = append(lines, fmt.Sprintf("%s [%s, %s]", refTag(h.Ref, h.Imported), h.Status.PlainLabel(), verificationLabel(h.Verification)),
			oneLine(h.Statement))
		lines = append(lines, whyLine(h.Rationale)...)
		lines = append(lines, p.evidenceLines(h.SupportingEvidence, h.ContradictingEvidence)...)
	}
	add(listTail(lines, len(p.Hypotheses), p.Omitted.Hypotheses,
		"finding still being tested", "findings still being tested")...)

	lines = []string{"Follow-up questions:"}
	for _, q := range p.OpenQuestions {
		lines = append(lines, refTag(q.Ref, q.Imported), oneLine(q.Question))
	}
	add(listTail(lines, len(p.OpenQuestions), p.Omitted.OpenQuestions, "follow-up question", "follow-up questions")...)

	if len(p.DeferredQuestions) > 0 {
		lines := []string{"Deferred follow-up questions:"}
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
		lines := []string{"Findings ruled out:"}
		for _, h := range p.DisprovenHypotheses {
			lines = append(lines, refTag(h.Ref, h.Imported)+" "+oneLine(h.Text)+" ["+verificationLabel(h.Verification)+"]")
			if len(h.ContradictingEvidence) > 0 {
				lines = append(lines, "Ruled out by: "+p.evidenceRefs(h.ContradictingEvidence))
			}
		}
		add(listTail(lines, len(p.DisprovenHypotheses), p.Omitted.DisprovenHypotheses,
			"finding ruled out", "findings ruled out")...)
	}

	if len(p.EvidenceIndex) > 0 {
		lines := []string{"Sources:"}
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
		lines := []string{"Recent changes:"}
		for _, c := range p.RecentChanges {
			who := string(c.Actor.Kind) + " " + oneLine(c.Actor.Client)
			if c.Imported {
				who += " (imported)"
			}
			lines = append(lines, fmt.Sprintf("- %s %s: %s", c.At, who,
				timelinePhrase(domain.Activity{Action: c.Action, TargetRef: c.TargetRef, Summary: c.Summary}, verificationLabel)))
		}
		add(lines...)
	}

	if p.Omitted.EvidenceOutsideFindings > 0 {
		add(fmt.Sprintf("Sources outside the included findings: %d omitted.", p.Omitted.EvidenceOutsideFindings))
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

// whyLine renders a finding's rationale as "Why: ...", or nothing when it
// is empty.
func whyLine(rationale string) []string {
	if r := oneLine(rationale); r != "" {
		return []string{"Why: " + r}
	}
	return nil
}

// evidenceLines renders the source ref lines of a finding; each is left out
// when its list is empty.
func (p Pack) evidenceLines(supporting, contradicting []string) []string {
	var out []string
	if len(supporting) > 0 {
		out = append(out, "Sources: "+p.evidenceRefs(supporting))
	}
	if len(contradicting) > 0 {
		out = append(out, "Sources against: "+p.evidenceRefs(contradicting))
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

// evidenceRefs keeps the check label adjacent to each citation, including
// after the source list's titles and labels were omitted by the budget.
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
	return checkUnknown
}

// checkUnknown stands in for a check label that is not available.
const checkUnknown = "Check not known"

// verificationLabel is the check label in everything that leaves the app: a
// shared answer and a pack an AI client reads. Its reader is not the person
// who did the checking, so it never says "You".
func verificationLabel(v domain.Verification) string {
	switch v {
	case "":
		return checkUnknown
	case domain.VerificationHumanVerified:
		return "A person checked this"
	}
	return v.PlainLabel()
}

// basisStyle picks how conclusionAccountabilityLines writes a finding
// behind the answer. Documents for people (the shared answer) use
// parentheses joined by semicolons; packs keep square brackets and commas.
type basisStyle int

const (
	basisBrackets basisStyle = iota
	basisParens
)

// claimBasisLabel names a basis finding's state without repeating a word: a
// current finding shows only its check label, a finding whose status and
// check agree (Sources disagree, Replaced) shows the words once, and any
// other finding shows its status, then its check label, joined by a
// semicolon inside parentheses and by a comma inside brackets.
func claimBasisLabel(claim PackConclusionClaim, style basisStyle) string {
	if claim.Missing {
		return "missing finding"
	}
	verification := verificationLabel(claim.Verification)
	sep := "; "
	if style == basisBrackets {
		sep = ", "
	}
	status := claim.Status.PlainLabel()
	if claim.Status == domain.ClaimStatusActive || claim.Status == "" || status == verification {
		return verification
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

// These labels are mandatory: a budget may shorten prose, never a check
// label or the fact that the findings behind the answer need a person's
// review.
func conclusionAccountabilityLines(c PackConclusion, style basisStyle) []string {
	verification := c.Verification
	if verification == "" {
		verification = domain.VerificationAIGenerated
	}
	open, closing, sep := " [", "]", ", "
	if style == basisParens {
		open, closing, sep = " (", ")", "; "
	}
	lines := []string{"Answer marked: " + verificationLabel(verification)}
	for _, group := range []struct {
		label  string
		claims []PackConclusionClaim
	}{{"Findings behind the answer", c.SupportingClaims}, {"Findings against the answer", c.ContradictingClaims}} {
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
		lines = append(lines, "Answer review: not reviewed yet.")
	} else {
		lines = append(lines, "Answer reviewed: "+readableTime(c.ReviewedAt))
		if c.ReviewBasis != "" {
			lines = append(lines, "What was checked: "+oneLine(c.ReviewBasis))
		}
		if c.Verification == domain.VerificationDisputed {
			lines = append(lines, "The answer needs a new review: sources disagree about it.")
		} else if c.NeedsReview {
			lines = append(lines, "The answer needs a new review: its findings or their sources changed since the last review.")
		}
	}
	return lines
}

// sourcesCheckedLine counts the sources that are not replaced: "1 of 3
// sources checked by a person. 1 checked by AI, 1 not checked." It is "" when
// there are none. The count stands where a confidence rating used to.
func sourcesCheckedLine(evidence []domain.Evidence) string {
	total, byPerson, byAI, notChecked := 0, 0, 0, 0
	for _, e := range evidence {
		switch e.Verification {
		case domain.VerificationSuperseded:
			continue
		case domain.VerificationHumanVerified:
			byPerson++
		case domain.VerificationSourceVerified:
			byAI++
		case domain.VerificationAIGenerated:
			notChecked++
		}
		total++
	}
	if total == 0 {
		return ""
	}
	noun := "sources"
	if total == 1 {
		noun = "source"
	}
	line := fmt.Sprintf("%d of %d %s checked by a person.", byPerson, total, noun)
	var rest []string
	if byAI > 0 {
		rest = append(rest, fmt.Sprintf("%d checked by AI", byAI))
	}
	if notChecked > 0 {
		rest = append(rest, fmt.Sprintf("%d not checked", notChecked))
	}
	if len(rest) > 0 {
		line += " " + strings.Join(rest, ", ") + "."
	}
	return line
}
