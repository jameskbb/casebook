package store

import "github.com/jameskbb/casebook/internal/domain"

// Hypothesis converts a LinkHypothesis row to the Hypothesis DTO.
func (r StatementRow) Hypothesis() domain.Hypothesis {
	links := r.Links
	if links == nil {
		links = []domain.EvidenceLink{}
	}
	return domain.Hypothesis{
		ID:               r.ID,
		Ref:              r.Ref,
		InvestigationRef: r.InvestigationRef,
		Statement:        r.Statement,
		Confidence:       r.Confidence,
		Rationale:        r.Rationale,
		Status:           domain.HypothesisStatus(r.Status),
		Verification:     r.Verification,
		SupersededBy:     r.SupersededByRef,
		CreatedAt:        r.CreatedAt,
		UpdatedAt:        r.UpdatedAt,
		CreatedBy:        r.CreatedBy,
		Imported:         r.Imported,
		Links:            links,
	}
}

// Hypotheses converts LinkHypothesis rows to Hypothesis DTOs. The result is
// never nil.
func Hypotheses(rows []StatementRow) []domain.Hypothesis {
	out := make([]domain.Hypothesis, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.Hypothesis())
	}
	return out
}
