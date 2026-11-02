package domain

// EvidenceLinkInput asks for one evidence link when adding a claim or
// hypothesis. EvidenceRef is an evidence ref in the same investigation
// (E-001, or the qualified INV-0001/E-001 form); Relation is supports or
// contradicts.
type EvidenceLinkInput struct {
	EvidenceRef string   `json:"evidence_ref"`
	Relation    Relation `json:"relation"`
}

// EvidenceLink is one evidence link of a claim or hypothesis, with the
// evidence title so surfaces can show it without another lookup.
type EvidenceLink struct {
	EvidenceRef   string   `json:"evidence_ref"`
	Relation      Relation `json:"relation"`
	EvidenceTitle string   `json:"evidence_title"`
	CreatedAt     string   `json:"created_at"`
	CreatedBy     Actor    `json:"created_by"`
}

// ClaimInput is the input to AddClaim. Statement is required. Confidence
// defaults to unknown, Status to active (superseded is not allowed at
// creation: use SupersedeClaim), Verification by actor kind (see
// DefaultVerification). Links are created in the same transaction.
type ClaimInput struct {
	Statement           string              `json:"statement"`
	Confidence          Confidence          `json:"confidence"`
	ConfidenceRationale string              `json:"confidence_rationale"`
	Status              ClaimStatus         `json:"status"`
	Verification        Verification        `json:"verification"`
	Links               []EvidenceLinkInput `json:"links"`
}

// ClaimPatch is the input to UpdateClaim. Nil fields are left unchanged.
// SupersededBy names the replacing claim and implies status superseded (any
// other Status alongside it is a validation error). Status superseded without
// SupersededBy is only accepted when the claim already has a replacement.
// Moving a superseded claim to another status clears SupersededBy.
type ClaimPatch struct {
	Statement           *string       `json:"statement,omitempty"`
	ConfidenceRationale *string       `json:"confidence_rationale,omitempty"`
	Confidence          *Confidence   `json:"confidence,omitempty"`
	Status              *ClaimStatus  `json:"status,omitempty"`
	Verification        *Verification `json:"verification,omitempty"`
	SupersededBy        *string       `json:"superseded_by,omitempty"`
}

// Claim is an interpretation of evidence. SupersededBy is the ref of the
// replacing claim ("" when none). Links are ordered by evidence ref and never
// nil.
type Claim struct {
	ID                  int64        `json:"id"`
	Ref                 string       `json:"ref"`
	InvestigationRef    string       `json:"investigation_ref"`
	Statement           string       `json:"statement"`
	Confidence          Confidence   `json:"confidence"`
	ConfidenceRationale string       `json:"confidence_rationale"`
	Status              ClaimStatus  `json:"status"`
	Verification        Verification `json:"verification"`
	SupersededBy        string       `json:"superseded_by"`
	CreatedAt           string       `json:"created_at"`
	UpdatedAt           string       `json:"updated_at"`
	CreatedBy           Actor        `json:"created_by"`
	// Imported: the row came from ImportInvestigation, so its author,
	// timestamps and text are what the file said (AR-09).
	Imported bool           `json:"imported"`
	Links    []EvidenceLink `json:"links"`
}
