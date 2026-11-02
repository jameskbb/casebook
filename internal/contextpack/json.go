package contextpack

import (
	"bytes"
	"encoding/json"
)

// RenderJSON renders the pack as indented JSON (two spaces) with a trailing
// newline. The document starts with "format": "casebook.context" and
// "version": 1, and every list is present ([] when empty, never null, even
// for a Pack not made by Build). Characters such as <, > and & are written
// as is, not HTML-escaped.
func RenderJSON(p Pack) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(nonNil(p)); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// nonNil returns a copy of p whose nil slices are empty.
func nonNil(p Pack) Pack {
	p.Conclusion.SupportingClaims = orEmpty(p.Conclusion.SupportingClaims)
	p.Conclusion.ContradictingClaims = orEmpty(p.Conclusion.ContradictingClaims)
	p.EstablishedClaims = claimsNonNil(p.EstablishedClaims)
	p.DisputedClaims = claimsNonNil(p.DisputedClaims)
	hyps := orEmpty(p.Hypotheses)
	if len(hyps) > 0 {
		hyps = append([]PackHypothesis(nil), hyps...)
		for i := range hyps {
			hyps[i].SupportingEvidence = orEmpty(hyps[i].SupportingEvidence)
			hyps[i].ContradictingEvidence = orEmpty(hyps[i].ContradictingEvidence)
		}
	}
	p.Hypotheses = hyps
	p.OpenQuestions = orEmpty(p.OpenQuestions)
	p.DeferredQuestions = orEmpty(p.DeferredQuestions)
	disproven := orEmpty(p.DisprovenHypotheses)
	if len(disproven) > 0 {
		disproven = append([]PackOneLine(nil), disproven...)
		for i := range disproven {
			disproven[i].ContradictingEvidence = orEmpty(disproven[i].ContradictingEvidence)
		}
	}
	p.DisprovenHypotheses = disproven
	p.EvidenceIndex = orEmpty(p.EvidenceIndex)
	p.EvidenceVerification = orEmpty(p.EvidenceVerification)
	p.RecentChanges = orEmpty(p.RecentChanges)
	p.OmittedSections = orEmpty(p.OmittedSections)
	return p
}

func claimsNonNil(cs []PackClaim) []PackClaim {
	if len(cs) == 0 {
		return []PackClaim{}
	}
	out := append([]PackClaim(nil), cs...)
	for i := range out {
		out[i].SupportingEvidence = orEmpty(out[i].SupportingEvidence)
		out[i].ContradictingEvidence = orEmpty(out[i].ContradictingEvidence)
	}
	return out
}

func orEmpty[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}
