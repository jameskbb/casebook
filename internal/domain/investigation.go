package domain

// Investigation is the top-level durable object.
type Investigation struct {
	ID                            int64               `json:"id"`
	Ref                           string              `json:"ref"`
	Title                         string              `json:"title"`
	ProblemStatement              string              `json:"problem_statement"`
	Status                        InvestigationStatus `json:"status"`
	Conclusion                    string              `json:"conclusion"`
	ConclusionConfidence          Confidence          `json:"conclusion_confidence"`
	ConclusionRationale           string              `json:"conclusion_rationale"`
	ConclusionVerification        Verification        `json:"conclusion_verification"`
	ConclusionSupportingClaims    []string            `json:"conclusion_supporting_claims"`
	ConclusionContradictingClaims []string            `json:"conclusion_contradicting_claims"`
	ConclusionReviewedAt          string              `json:"conclusion_reviewed_at"`
	ConclusionReviewBasis         string              `json:"conclusion_review_basis"`
	ConclusionReviewToken         string              `json:"conclusion_review_token"`
	ConclusionNeedsReview         bool                `json:"conclusion_needs_review"`
	CreatedAt                     string              `json:"created_at"`
	UpdatedAt                     string              `json:"updated_at"`
	ArchivedAt                    string              `json:"archived_at"` // "" when not archived
	Archived                      bool                `json:"archived"`
	IsActive                      bool                `json:"is_active"`
	CreatedBy                     Actor               `json:"created_by"`
}

// CreateInvestigationInput is the input to CreateInvestigation. Status is
// optional (default new) and must not be archived.
type CreateInvestigationInput struct {
	Title            string              `json:"title"`
	ProblemStatement string              `json:"problem_statement"`
	Status           InvestigationStatus `json:"status"`
	MakeActive       bool                `json:"make_active"`
}

// InvestigationPatch is the input to UpdateInvestigation. Nil fields are left
// unchanged. Status archived is equivalent to ArchiveInvestigation; a
// non-archived status on an archived investigation unarchives it to that
// status.
type InvestigationPatch struct {
	Title            *string              `json:"title,omitempty"`
	ProblemStatement *string              `json:"problem_statement,omitempty"`
	Status           *InvestigationStatus `json:"status,omitempty"`
}

// ConclusionInput is the input to UpdateConclusion. It replaces the whole
// conclusion: an empty Conclusion clears it and an empty Confidence means
// unknown. Verification is optional and defaults by actor kind on creation.
// Nil claim selections preserve the previous refs; an empty slice clears one.
// Review is an explicit human action with a concise basis and the token of
// the displayed conclusion. Save changed selections before reviewing them.
type ConclusionInput struct {
	Conclusion          string       `json:"conclusion"`
	Confidence          Confidence   `json:"confidence"`
	Rationale           string       `json:"rationale"`
	Verification        Verification `json:"verification,omitempty"`
	SupportingClaims    []string     `json:"supporting_claims,omitempty"`
	ContradictingClaims []string     `json:"contradicting_claims,omitempty"`
	Review              bool         `json:"review,omitempty"`
	ExpectedReviewToken string       `json:"expected_review_token,omitempty"`
	ReviewBasis         string       `json:"review_basis,omitempty"`
}
