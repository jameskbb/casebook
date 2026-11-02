package contextpack

import "github.com/jameskbb/casebook/internal/domain"

// shortenLevels are the byte caps statements and questions are cut to, in
// order, once every optional section is gone.
var shortenLevels = []int{240, 120, 60}

// applyBudget trims p until RenderMarkdown(*p) is at most limit bytes,
// re-rendering after every step and stopping at the first step that fits
// (D-25). The steps, in order:
//
//  1. drop recent_changes, the claim and hypothesis rationales (key
//     "statement_rationales"), evidence_index, deferred_questions, then the
//     conclusion rationale (key "rationale");
//  2. shorten established and disputed claim statements, hypothesis
//     statements and open questions to 240, then 120, then 60 bytes, ending
//     in "..." (disproven hypothesis one-liners are cut to the same caps once
//     the cap is below their own 200 bytes);
//  3. drop disputed_claims;
//  4. remove disproven hypotheses from the end one at a time, counting them
//     in p.Omitted (key "disproven_hypotheses" once none is left): they stay
//     as long as possible so a large investigation still says what not to
//     retry;
//  5. remove open questions from the end one at a time, then hypotheses,
//     then established claims, counting them in p.Omitted.
//
// The header (ref, title, status, primary question, conclusion text and
// confidence, verification and review labels), selected conclusion basis
// and mandatory omission warnings are never dropped or shortened. A budget
// smaller than this minimum yields output larger than the budget. The steps are a fixed
// sequence over sorted data, so the result is deterministic.
func applyBudget(p *Pack, limit int) {
	p.MaxBytes = limit
	fits := func() bool {
		pruneEvidenceVerification(p)
		return len(RenderMarkdown(*p)) <= limit
	}
	if fits() {
		return
	}
	p.Truncated = true // the trim footer counts against the budget

	omit := func(key string, n int, clear func()) bool {
		if n == 0 {
			return fits()
		}
		clear()
		p.OmittedSections = append(p.OmittedSections, key)
		return fits()
	}
	if omit("recent_changes", len(p.RecentChanges), func() { p.RecentChanges = []PackChange{} }) ||
		omit("statement_rationales", countRationales(p), func() { clearRationales(p) }) ||
		omit("evidence_index", len(p.EvidenceIndex), func() { p.EvidenceIndex = []PackEvidence{} }) ||
		omit("deferred_questions", len(p.DeferredQuestions), func() { p.DeferredQuestions = []PackQuestion{} }) ||
		omit("rationale", len(p.Conclusion.Rationale), func() { p.Conclusion.Rationale = "" }) {
		return
	}

	for _, n := range shortenLevels {
		shortenTexts(p, n)
		if fits() {
			return
		}
	}

	if omit("disputed_claims", len(p.DisputedClaims), func() {
		p.Omitted.DisputedClaims += len(p.DisputedClaims)
		p.DisputedClaims = []PackClaim{}
	}) {
		return
	}

	for len(p.DisprovenHypotheses) > 0 {
		p.DisprovenHypotheses = p.DisprovenHypotheses[:len(p.DisprovenHypotheses)-1]
		p.Omitted.DisprovenHypotheses++
		if len(p.DisprovenHypotheses) == 0 {
			p.OmittedSections = append(p.OmittedSections, "disproven_hypotheses")
		}
		if fits() {
			return
		}
	}

	for len(p.OpenQuestions) > 0 {
		p.OpenQuestions = p.OpenQuestions[:len(p.OpenQuestions)-1]
		p.Omitted.OpenQuestions++
		if fits() {
			return
		}
	}
	for len(p.Hypotheses) > 0 {
		p.Hypotheses = p.Hypotheses[:len(p.Hypotheses)-1]
		p.Omitted.Hypotheses++
		if fits() {
			return
		}
	}
	for len(p.EstablishedClaims) > 0 {
		p.EstablishedClaims = p.EstablishedClaims[:len(p.EstablishedClaims)-1]
		p.Omitted.EstablishedClaims++
		if fits() {
			return
		}
	}
}

// shortenTexts cuts every claim and hypothesis statement, open question and
// disproven hypothesis one-liner to at most n bytes with domain.TruncateLine.
func shortenTexts(p *Pack, n int) {
	for i := range p.EstablishedClaims {
		p.EstablishedClaims[i].Statement = domain.TruncateLine(p.EstablishedClaims[i].Statement, n)
	}
	for i := range p.DisputedClaims {
		p.DisputedClaims[i].Statement = domain.TruncateLine(p.DisputedClaims[i].Statement, n)
	}
	for i := range p.Hypotheses {
		p.Hypotheses[i].Statement = domain.TruncateLine(p.Hypotheses[i].Statement, n)
	}
	for i := range p.OpenQuestions {
		p.OpenQuestions[i].Question = domain.TruncateLine(p.OpenQuestions[i].Question, n)
	}
	for i := range p.DisprovenHypotheses {
		p.DisprovenHypotheses[i].Text = domain.TruncateLine(p.DisprovenHypotheses[i].Text, n)
	}
}

// countRationales counts the claims and hypotheses that carry a rationale.
func countRationales(p *Pack) int {
	n := 0
	for _, c := range p.EstablishedClaims {
		if c.Rationale != "" {
			n++
		}
	}
	for _, c := range p.DisputedClaims {
		if c.Rationale != "" {
			n++
		}
	}
	for _, h := range p.Hypotheses {
		if h.Rationale != "" {
			n++
		}
	}
	return n
}

// clearRationales empties every claim and hypothesis rationale.
func clearRationales(p *Pack) {
	for i := range p.EstablishedClaims {
		p.EstablishedClaims[i].Rationale = ""
	}
	for i := range p.DisputedClaims {
		p.DisputedClaims[i].Rationale = ""
	}
	for i := range p.Hypotheses {
		p.Hypotheses[i].Rationale = ""
	}
}

// pruneEvidenceVerification removes labels only after all findings citing that
// evidence have gone. A kept reference never loses its verification to budgeting.
func pruneEvidenceVerification(p *Pack) {
	referenced := map[string]bool{}
	add := func(refs []string) {
		for _, ref := range refs {
			referenced[ref] = true
		}
	}
	for _, c := range p.EstablishedClaims {
		add(c.SupportingEvidence)
		add(c.ContradictingEvidence)
	}
	for _, c := range p.DisputedClaims {
		add(c.SupportingEvidence)
		add(c.ContradictingEvidence)
	}
	for _, h := range p.Hypotheses {
		add(h.SupportingEvidence)
		add(h.ContradictingEvidence)
	}
	for _, h := range p.DisprovenHypotheses {
		add(h.ContradictingEvidence)
	}
	kept := p.EvidenceVerification[:0]
	for _, e := range p.EvidenceVerification {
		if referenced[e.Ref] {
			kept = append(kept, e)
		}
	}
	p.EvidenceVerification = kept
}
