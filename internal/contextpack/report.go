package contextpack

import (
	"slices"
	"strings"
	"time"

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

// answerFooter ends both versions of a shared answer.
const answerFooter = "_Made with Casebook. These are the saved findings and sources, word for word._"

// RenderReport renders a snapshot as the full shared answer ("Everything")
// in Markdown: the body every shared answer has (see answerDoc.body), then a
// dated list of how the answer was reached, oldest first.
//
// Like Build, it is pure and deterministic: inputs are copied and sorted by
// ref number (activity by id), no clock is read and no map is iterated.
// Every user text value is rendered on one line, and a value that would
// start a line with a Markdown block marker is escaped, so stored text
// cannot add headings or list items of its own.
func RenderReport(s Snapshot, o ReportOptions) string {
	d := answerDoc{full: true}
	d.body(s, s.Evidence)

	timeline, earlier := reportTimeline(s.Activity, o)
	if len(timeline) > 0 || earlier {
		d.para("## How we got here")
		if earlier {
			d.para("Earlier changes are not shown.")
		}
		name := s.names()
		day := "\x00"
		var lines []string
		for _, a := range timeline {
			if on := longDate(a.CreatedAt); on != day {
				d.para(lines...)
				lines, day = nil, on
				d.para("### " + orText(on, "No date"))
			}
			who := actorWords(a.Actor)
			if a.Imported {
				who += " (from a file)"
			}
			phrase := "made a change"
			if _, known := timelineVerbs[a.Action]; known {
				phrase = timelinePhrase(a, name)
			}
			lines = append(lines, "- "+who+" "+phrase)
		}
		d.para(lines...)
	}

	d.para(answerFooter)
	return d.b.String()
}

// answerDoc collects the paragraphs of a shared answer, one blank line
// between them.
type answerDoc struct {
	b strings.Builder
	// full is the version with everything: it also dates each source.
	full bool
}

func (d *answerDoc) para(lines ...string) {
	if len(lines) == 0 {
		return
	}
	if d.b.Len() > 0 {
		d.b.WriteString("\n")
	}
	for _, l := range lines {
		d.b.WriteString(l)
		d.b.WriteString("\n")
	}
}

// section writes a heading and its list, or nothing when the list is empty.
func (d *answerDoc) section(heading string, lines []string) {
	if len(lines) > 0 {
		d.para("## " + heading)
		d.para(lines...)
	}
}

// body writes what both versions of a shared answer have, in the words the
// desktop uses and with no record codes: the question as the title, when it
// was asked, the background, the answer with the count of sources checked
// and who checked it, then the findings by state (each with why and the
// sources that back it up or go against it), the follow-up questions and the
// sources. A section with nothing in it is left out. Confidence is never
// printed. allEvidence is every source of the question, for the count.
func (d *answerDoc) body(s Snapshot, allEvidence []domain.Evidence) {
	inv := s.Investigation
	d.para("# " + oneLine(inv.Title))
	if asked := longDate(inv.CreatedAt); asked != "" {
		line := "Asked on " + asked + "."
		if changed := longDate(inv.UpdatedAt); changed != "" && changed != asked {
			line += " Last changed on " + changed + "."
		}
		d.para(line)
	}
	if background := mdText(inv.ProblemStatement); background != "" {
		d.para("## Background")
		d.para(background)
	}
	d.para("## Answer")
	for _, block := range answerBlocks(s, allEvidence) {
		d.para(block...)
	}

	evidence := slices.Clone(s.Evidence)
	sortByRef(evidence, domain.RecordEvidence, func(e domain.Evidence) string { return e.Ref })
	sources := func(links []domain.EvidenceLink) []string {
		backs, against := linkRefs(links)
		var out []string
		for _, group := range []struct {
			label string
			refs  []string
		}{{"Backed by", backs}, {"Goes against it", against}} {
			for _, ref := range group.refs {
				line := "  - " + group.label + ": a source that is not included here"
				for _, e := range evidence {
					if e.Ref == ref {
						line = "  - " + group.label + ": " + oneLine(e.Title) + " (" + verificationLabel(e.Verification) + ")"
						break
					}
				}
				out = append(out, line)
			}
		}
		return out
	}
	// hide is the state the section heading already says.
	finding := func(statement, why string, imported bool, links []domain.EvidenceLink, hide string, states ...string) []string {
		lines := []string{"- " + oneLine(statement) + stateWords(hide, imported, states...)}
		if w := oneLine(why); w != "" {
			lines = append(lines, "  - Why: "+w)
		}
		return append(lines, sources(links)...)
	}

	// The same groups as the question page: a finding sits under what
	// happened to it, claims before hypotheses, each in ref order.
	var found, disagree, untested, ruledOut, replaced []string
	claims := slices.Clone(s.Claims)
	sortByRef(claims, domain.RecordClaim, func(c domain.Claim) string { return c.Ref })
	for _, c := range claims {
		item := func(hide string) []string {
			return finding(c.Statement, c.ConfidenceRationale, c.Imported, c.Links, hide, c.Status.PlainLabel(), verificationLabel(c.Verification))
		}
		switch {
		case c.Status == domain.ClaimStatusSuperseded || c.Verification == domain.VerificationSuperseded:
			replaced = append(replaced, item("Replaced")...)
		case c.Status == domain.ClaimStatusRejected:
			ruledOut = append(ruledOut, item("Ruled out")...)
		case c.Status == domain.ClaimStatusDisputed || c.Verification == domain.VerificationDisputed:
			disagree = append(disagree, item("Sources disagree")...)
		default:
			found = append(found, item("")...)
		}
	}
	hyps := slices.Clone(s.Hypotheses)
	sortByRef(hyps, domain.RecordHypothesis, func(h domain.Hypothesis) string { return h.Ref })
	for _, h := range hyps {
		item := func(hide string) []string {
			return finding(h.Statement, h.Rationale, h.Imported, h.Links, hide, h.Status.PlainLabel(), verificationLabel(h.Verification))
		}
		switch {
		case h.Status == domain.HypothesisStatusSuperseded || h.Verification == domain.VerificationSuperseded:
			replaced = append(replaced, item("Replaced")...)
		case h.Status == domain.HypothesisStatusDisproven:
			ruledOut = append(ruledOut, item("Ruled out")...)
		case h.Verification == domain.VerificationDisputed:
			disagree = append(disagree, item("Sources disagree")...)
		case h.Status == domain.HypothesisStatusOpen:
			untested = append(untested, item("Not tested yet")...)
		default:
			found = append(found, item("")...)
		}
	}
	d.section("Findings", found)
	d.section("Where sources disagree", disagree)
	d.section("Not tested yet", untested)
	d.section("Ruled out", ruledOut)
	d.section("Replaced", replaced)

	questions := slices.Clone(s.Questions)
	sortByRef(questions, domain.RecordQuestion, func(q domain.Question) string { return q.Ref })
	var followUps []string
	for _, q := range questions {
		followUps = append(followUps, "- "+oneLine(q.Question)+stateWords("", q.Imported, q.Status.PlainLabel()))
		if answer := oneLine(q.Resolution); answer != "" {
			followUps = append(followUps, "  - Answer: "+answer)
		}
		if note := oneLine(q.DeferNote); note != "" {
			followUps = append(followUps, "  - Why it can wait: "+note)
		}
	}
	d.section("Follow-up questions", followUps)

	var list []string
	for _, e := range evidence {
		list = append(list, "- "+oneLine(e.Title)+stateWords("", e.Imported, verificationLabel(e.Verification)))
		from := oneLine(e.SourceLabel)
		if from == "" {
			from = e.SourceType.PlainLabel()
		}
		if on := longDate(e.CapturedAt); d.full && on != "" {
			from += ", dated " + on
		}
		list = append(list, "  - "+from)
		if where := oneLine(e.SourceRef); where != "" {
			list = append(list, "  - Where to find it: "+where)
		}
		if sum := oneLine(e.Summary); sum != "" {
			list = append(list, "  - "+sum)
		}
	}
	d.section("Sources", list)
}

// stateWords writes a record's state after its text, " (Looks right so far;
// Not checked)": each state once, without Current and without hide, then
// "From a file" for a record copied from an imported file. It is "" when
// nothing is left to say.
func stateWords(hide string, imported bool, states ...string) string {
	var words []string
	for _, s := range states {
		if s != "" && s != "Current" && s != hide && !slices.Contains(words, s) {
			words = append(words, s)
		}
	}
	if imported {
		words = append(words, "From a file")
	}
	if len(words) == 0 {
		return ""
	}
	return " (" + strings.Join(words, "; ") + ")"
}

// answerBlocks renders the paragraphs of the Answer section: the answer,
// the count of sources checked (in place of a confidence rating), who
// checked the answer and what they checked, why, and the findings the
// answer rests on, named by their own text. evidence is every source of the
// question.
func answerBlocks(s Snapshot, evidence []domain.Evidence) [][]string {
	inv := s.Investigation
	if inv.Conclusion == "" {
		return [][]string{{"No answer yet."}}
	}
	out := [][]string{{mdText(inv.Conclusion)}}
	if count := sourcesCheckedLine(evidence); count != "" {
		out = append(out, []string{count})
	}
	out = append(out, []string{answerCheckLine(inv)})
	if basis := oneLine(inv.ConclusionReviewBasis); basis != "" && inv.ConclusionReviewedAt != "" {
		out = append(out, []string{"What was checked: " + basis})
	}
	if why := oneLine(inv.ConclusionRationale); why != "" {
		out = append(out, []string{"Why: " + why})
	}
	for _, group := range []struct {
		heading string
		refs    []string
	}{{"Findings that back the answer:", inv.ConclusionSupportingClaims}, {"Findings that go against the answer:", inv.ConclusionContradictingClaims}} {
		if len(group.refs) == 0 {
			continue
		}
		lines := []string{group.heading}
		for _, ref := range group.refs {
			line := "- A finding that is no longer here"
			for _, c := range s.Claims {
				if c.Ref == ref {
					line = "- " + oneLine(c.Statement) + stateWords("", false, c.Status.PlainLabel(), verificationLabel(c.Verification))
					break
				}
			}
			lines = append(lines, line)
		}
		out = append(out, lines)
	}
	return out
}

// answerCheckLine says who has checked the answer, from its check label and
// the day a person last checked it. It always states the label, so a reader
// never takes an unchecked answer for a checked one.
func answerCheckLine(inv domain.Investigation) string {
	on := longDate(inv.ConclusionReviewedAt)
	last := ""
	if on != "" {
		last = " A person last checked it on " + on + "."
	}
	switch inv.ConclusionVerification {
	case domain.VerificationHumanVerified:
		line := "A person checked this answer."
		if on != "" {
			line = "A person checked this answer on " + on + "."
		}
		if inv.ConclusionNeedsReview && on != "" { // without a dated check, NeedsReview only means none was recorded
			line += " Something changed after that. It needs checking again."
		}
		return line
	case domain.VerificationDisputed:
		return "Sources disagree about this answer." + last
	case domain.VerificationSourceVerified:
		if on == "" {
			return "An AI compared this answer with its sources. No person has checked it yet."
		}
		return "An AI compared this answer with its sources." + last
	case domain.VerificationSuperseded:
		return "This answer was replaced." + last
	}
	if on != "" {
		return "A person checked this answer on " + on + ". It changed after that, and nobody has checked it since."
	}
	return "Nobody has checked this answer yet."
}

// names returns the refNamer of a shared answer: a record is named by its
// own text in quotes, cut to a readable length. The map is only looked up.
func (s Snapshot) names() refNamer {
	text := map[string]string{}
	for _, e := range s.Evidence {
		text[e.Ref] = e.Title
	}
	for _, c := range s.Claims {
		text[c.Ref] = c.Statement
	}
	for _, h := range s.Hypotheses {
		text[h.Ref] = h.Statement
	}
	for _, q := range s.Questions {
		text[q.Ref] = q.Question
	}
	return func(ref string) string {
		if t := domain.TruncateLine(text[ref], 90); t != "" {
			return `"` + t + `"`
		}
		return ""
	}
}

// actorWords names who made a change for a reader of the shared answer, in
// the app's words: "A person", "Claude Code", "Your AI". The reader is not
// the person who did the checking, so a person is never "You".
func actorWords(a domain.Actor) string {
	switch a.Kind {
	case domain.ActorHuman:
		return "A person"
	case domain.ActorAI:
		switch client := oneLine(a.Client); client {
		case "claude-code":
			return "Claude Code"
		case "codex":
			return "Codex"
		case "", "mcp-client":
			return "Your AI"
		default:
			return "Your AI (" + client + ")"
		}
	}
	return "Casebook"
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

// longDate writes the day of a stored timestamp (RFC 3339, UTC) as "October
// 30, 2026", or "" when the value has no date. A time of day is never
// printed: the reader's clock is not the one the records are stored in.
func longDate(ts string) string {
	if len(ts) < 10 {
		return ""
	}
	t, err := time.Parse(time.DateOnly, ts[:10])
	if err != nil {
		return ""
	}
	return t.Format("January 2, 2006")
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
