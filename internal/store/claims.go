package store

import "github.com/jameskbb/casebook/internal/domain"

// Claim converts a LinkClaim row to the Claim DTO.
func (r StatementRow) Claim() domain.Claim {
	links := r.Links
	if links == nil {
		links = []domain.EvidenceLink{}
	}
	return domain.Claim{
		ID:                  r.ID,
		Ref:                 r.Ref,
		InvestigationRef:    r.InvestigationRef,
		Statement:           r.Statement,
		Confidence:          r.Confidence,
		ConfidenceRationale: r.Rationale,
		Status:              domain.ClaimStatus(r.Status),
		Verification:        r.Verification,
		SupersededBy:        r.SupersededByRef,
		CreatedAt:           r.CreatedAt,
		UpdatedAt:           r.UpdatedAt,
		CreatedBy:           r.CreatedBy,
		Imported:            r.Imported,
		Links:               links,
	}
}

// Claims converts LinkClaim rows to Claim DTOs. The result is never nil.
func Claims(rows []StatementRow) []domain.Claim {
	out := make([]domain.Claim, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.Claim())
	}
	return out
}
