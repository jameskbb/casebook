package contextpack

import (
	"fmt"
	"slices"
	"strings"

	"github.com/jameskbb/casebook/internal/domain"
)

// ReportOptions tunes RenderReport. TimelineLimit > 0 keeps only the newest
// TimelineLimit material changes in the timeline (0 keeps all of them).
// MoreActivity says the snapshot's activity is not the full history (the
// caller read a capped number of rows), so earlier changes exist that the
// timeline cannot show.
type ReportOptions struct {
	TimelineLimit int
	MoreActivity  bool
}

// RenderReport renders a snapshot as the full shared answer in Markdown,
// in the words a person reads (question, answer, finding, source): the
// title, the question, the answer with the count of sources checked, its
// check and review labels and why, the findings with their sources (source
// label and ref), where sources disagree, what was ruled out (disproven
// hypotheses and rejected claims with the sources that contradicted them),
// findings still being tested, answered and open follow-up questions, every
// source, and a dated list (UTC) of how the answer was reached, oldest
// first. Confidence is never printed.
//
// Like Build, it is pure and deterministic: inputs are copied and sorted by
// ref number (activity by id), no clock is read and no map is iterated.
// Superseded claims and hypotheses are left out (their replacements carry
// the story); the timeline still records the change. Every user text value
// is rendered on one line, and a value that would start a line with a
// Markdown block marker is escaped, so stored text cannot add headings or
// list items of its own.
func RenderReport(s Snapshot, o ReportOptions) string {
	inv := s.Investigation
	var b strings.Builder
	para := func(lines ...string) {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		for _, l := range lines {
			b.WriteString(l)
			b.WriteString("\n")
		}
	}

	evidence := slices.Clone(s.Evidence)
	sortByRef(evidence, domain.RecordEvidence, func(e domain.Evidence) string { return e.Ref })
	evByRef := func(ref string) (domain.Evidence, bool) {
		for _, e := range evidence {
			if e.Ref == ref {
				return e, true
			}
		}
		return domain.Evidence{}, false
	}
	// evidenceLines renders "Supported by E-001: title (source)" bullets.
	evidenceLines := func(links []domain.EvidenceLink) []string {
		supporting, contradicting := linkRefs(links)
		var out []string
		for _, group := range []struct {
			verb string
			refs []string
		}{{"Supported by", supporting}, {"Contradicted by", contradicting}} {
			for _, ref := range group.refs {
				line := "  - " + group.verb + " " + ref
				if e, ok := evByRef(ref); ok {
					line += ": " + oneLine(e.Title) + " (" + sourceText(e) + "; " + verificationLabel(e.Verification) + ")"
				} else {
					line += " (" + checkUnknown + ")"
				}
				out = append(out, line)
			}
		}
		return out
	}
	why := func(rationale string) []string {
		if r := oneLine(rationale); r != "" {
			return []string{"  - Why: " + r}
		}
		return nil
	}

	para("# " + oneLine(inv.Ref) + ": " + oneLine(inv.Title))
	para("- Opened: "+datePart(inv.CreatedAt),
		"- Last updated: "+datePart(inv.UpdatedAt))

	para("## Question")
	para(orText(mdText(inv.ProblemStatement), "Not recorded."))

	para("## Answer")
	for _, line := range answerLines(s, evidence) {
		para(line)
	}

	claims := slices.Clone(s.Claims)
	sortByRef(claims, domain.RecordClaim, func(c domain.Claim) string { return c.Ref })
	claimItem := func(c domain.Claim, tail string) []string {
		lines := []string{fmt.Sprintf("- **%s** %s (%s)", refTag(c.Ref, c.Imported), oneLine(c.Statement), tail)}
		lines = append(lines, why(c.ConfidenceRationale)...)
		return append(lines, evidenceLines(c.Links)...)
	}
	var established, disputed, rejected []string
	for _, c := range claims {
		switch {
		case c.Status == domain.ClaimStatusRejected:
			rejected = append(rejected, claimItem(c, BriefStateLabel(c.Status.PlainLabel(), c.Verification))...)
		default:
			switch claimState(c) {
			case domain.ClaimStatusActive:
				established = append(established, claimItem(c, verificationLabel(c.Verification))...)
			case domain.ClaimStatusDisputed:
				disputed = append(disputed, claimItem(c, verificationLabel(c.Verification))...)
			}
		}
	}

	hyps := slices.Clone(s.Hypotheses)
	sortByRef(hyps, domain.RecordHypothesis, func(h domain.Hypothesis) string { return h.Ref })
	var standing, disproven []string
	for _, h := range hyps {
		if h.Verification == domain.VerificationSuperseded {
			continue
		}
		head := fmt.Sprintf("- **%s** %s", refTag(h.Ref, h.Imported), oneLine(h.Statement))
		switch h.Status {
		case domain.HypothesisStatusOpen, domain.HypothesisStatusSupported:
			standing = append(standing, head+" ("+BriefStateLabel(h.Status.PlainLabel(), h.Verification)+")")
			standing = append(standing, why(h.Rationale)...)
			standing = append(standing, evidenceLines(h.Links)...)
		case domain.HypothesisStatusDisproven:
			disproven = append(disproven, head+" ("+BriefStateLabel(h.Status.PlainLabel(), h.Verification)+")")
			disproven = append(disproven, why(h.Rationale)...)
			disproven = append(disproven, evidenceLines(h.Links)...)
		}
	}

	para("## Findings")
	if len(established) > 0 {
		para(established...)
	} else {
		para("None yet.")
	}
	if len(disputed) > 0 {
		para("## Where sources disagree")
		para(disputed...)
	}
	if ruledOut := slices.Concat(disproven, rejected); len(ruledOut) > 0 {
		para("## Ruled out")
		para(ruledOut...)
	}
	if len(standing) > 0 {
		para("## Still being tested")
		para(standing...)
	}

	questions := slices.Clone(s.Questions)
	sortByRef(questions, domain.RecordQuestion, func(q domain.Question) string { return q.Ref })
	var answered, open []string
	for _, q := range questions {
		head := fmt.Sprintf("- **%s** %s", refTag(q.Ref, q.Imported), oneLine(q.Question))
		switch q.Status {
		case domain.QuestionStatusResolved:
			answer := "  - Answer: " + orText(oneLine(q.Resolution), "(none recorded)")
			if d := datePart(q.ResolvedAt); d != "" {
				answer += " (" + d + ")"
			}
			answered = append(answered, head, answer)
		case domain.QuestionStatusOpen:
			open = append(open, head)
		case domain.QuestionStatusDeferred:
			line := head + " (deferred"
			if note := oneLine(q.DeferNote); note != "" {
				line += ": " + note
			}
			open = append(open, line+")")
		}
	}
	if len(answered) > 0 {
		para("## Answered follow-up questions")
		para(answered...)
	}
	para("## Open follow-up questions")
	if len(open) > 0 {
		para(open...)
	} else {
		para("None.")
	}

	if len(evidence) > 0 {
		para("## Sources")
		var lines []string
		for _, e := range evidence {
			lines = append(lines, fmt.Sprintf("- **%s** %s", refTag(e.Ref, e.Imported), oneLine(e.Title)))
			detail := "  - " + sourceText(e)
			if d := datePart(e.CapturedAt); d != "" {
				detail += ", captured " + d
			}
			lines = append(lines, detail+", "+verificationLabel(e.Verification))
			if sum := oneLine(e.Summary); sum != "" {
				lines = append(lines, "  - "+sum)
			}
		}
		para(lines...)
	}

	timeline, earlier := reportTimeline(s.Activity, o)
	if len(timeline) > 0 || earlier {
		para("## How we got here (UTC)")
		if earlier {
			para("Earlier changes are not shown.")
		}
		day := ""
		var lines []string
		for _, a := range timeline {
			if d := datePart(a.CreatedAt); d != day {
				if len(lines) > 0 {
					para(lines...)
					lines = nil
				}
				day = d
				para("### " + orText(d, "Undated"))
			}
			who := actorWords(a.Actor)
			if a.Imported {
				who += " (imported)"
			}
			lines = append(lines, fmt.Sprintf("- %s %s: %s", timePart(a.CreatedAt), who, timelinePhrase(a, verificationLabel)))
		}
		if len(lines) > 0 {
			para(lines...)
		}
	}

	para("_Generated by Casebook from " + oneLine(inv.Ref) + "._")
	return b.String()
}

// actorWords names who made a change for a reader of the shared answer:
// "A person", "Claude Code (AI)". The reader is not the person who did the
// checking, so a person is never "You".
func actorWords(a domain.Actor) string {
	client := oneLine(a.Client)
	switch client {
	case "claude-code":
		client = "Claude Code"
	case "codex":
		client = "Codex"
	}
	switch a.Kind {
	case domain.ActorHuman:
		return "A person"
	case domain.ActorAI:
		if client == "" || client == "mcp-client" {
			return "An AI client (AI)"
		}
		return client + " (AI)"
	case domain.ActorImport:
		return "Import"
	case domain.ActorSystem:
		return "Casebook"
	}
	return strings.TrimSpace(string(a.Kind) + " " + client)
}

// reportTimeline returns the material activity rows the timeline shows,
// oldest first, and whether earlier rows exist that it leaves out.
func reportTimeline(rows []domain.Activity, o ReportOptions) ([]domain.Activity, bool) {
	activity := slices.Clone(rows)
	slices.SortStableFunc(activity, func(a, b domain.Activity) int {
		switch {
		case a.ID < b.ID:
			return -1
		case a.ID > b.ID:
			return 1
		}
		return strings.Compare(a.CreatedAt, b.CreatedAt)
	})
	material := activity[:0:0]
	for _, a := range activity {
		if !slices.Contains(nonMaterial, a.Action) {
			material = append(material, a)
		}
	}
	earlier := o.MoreActivity
	if o.TimelineLimit > 0 && len(material) > o.TimelineLimit {
		material = material[len(material)-o.TimelineLimit:]
		earlier = true
	}
	return material, earlier
}

// sourceText renders where evidence came from: its source label (or source
// type label when it has none), then ": " and its source ref when set.
func sourceText(e domain.Evidence) string {
	s := oneLine(e.SourceLabel)
	if s == "" {
		s = e.SourceType.Label()
	}
	if ref := oneLine(e.SourceRef); ref != "" {
		s += ": " + ref
	}
	return s
}

// datePart returns the YYYY-MM-DD prefix of an RFC 3339 timestamp, or ""
// when the value is shorter.
func datePart(ts string) string {
	if len(ts) < 10 {
		return ""
	}
	return ts[:10]
}

// timePart returns the HH:MM of an RFC 3339 UTC timestamp, or "--:--" when
// the value has no time.
func timePart(ts string) string {
	if len(ts) < 16 || ts[10] != 'T' {
		return "--:--"
	}
	return ts[11:16]
}

// orText returns s, or fallback when s is empty.
func orText(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

// mdText renders user text that stands as a paragraph of its own on one
// line, escaping a leading Markdown block marker (heading, quote, list item,
// table row, fence, rule or ordered list number) so the text stays a plain
// paragraph.
func mdText(s string) string {
	s = oneLine(s)
	if s == "" {
		return s
	}
	switch s[0] {
	case '#', '>', '-', '+', '*', '|', '`', '~', '=', '_':
		return `\` + s
	}
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i > 0 && i < len(s) && (s[i] == '.' || s[i] == ')') {
		return s[:i] + `\` + s[i:]
	}
	return s
}
