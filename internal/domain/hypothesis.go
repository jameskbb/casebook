package domain

// HypothesisInput is the input to AddHypothesis. Statement is required.
// Confidence defaults to unknown, Status to open (superseded is not allowed
// at creation: use SupersedeHypothesis), Verification by actor kind (see
// DefaultVerification). Links are created in the same transaction.
type HypothesisInput struct {
	Statement    string              `json:"statement"`
	Confidence   Confidence          `json:"confidence"`
	Rationale    string              `json:"rationale"`
	Status       HypothesisStatus    `json:"status"`
	Verification Verification        `json:"verification"`
	Links        []EvidenceLinkInput `json:"links"`
}

// HypothesisPatch is the input to UpdateHypothesis. Nil fields are left
// unchanged; the superseding rules are the same as ClaimPatch.
type HypothesisPatch struct {
	Statement    *string           `json:"statement,omitempty"`
	Rationale    *string           `json:"rationale,omitempty"`
	Confidence   *Confidence       `json:"confidence,omitempty"`
	Status       *HypothesisStatus `json:"status,omitempty"`
	Verification *Verification     `json:"verification,omitempty"`
	SupersededBy *string           `json:"superseded_by,omitempty"`
}

// Hypothesis is a possible explanation being tested. SupersededBy is the ref
// of the replacing hypothesis ("" when none). Links are ordered by evidence
// ref and never nil.
type Hypothesis struct {
	ID               int64            `json:"id"`
	Ref              string           `json:"ref"`
	InvestigationRef string           `json:"investigation_ref"`
	Statement        string           `json:"statement"`
	Confidence       Confidence       `json:"confidence"`
	Rationale        string           `json:"rationale"`
	Status           HypothesisStatus `json:"status"`
	Verification     Verification     `json:"verification"`
	SupersededBy     string           `json:"superseded_by"`
	CreatedAt        string           `json:"created_at"`
	UpdatedAt        string           `json:"updated_at"`
	CreatedBy        Actor            `json:"created_by"`
	// Imported: the row came from ImportInvestigation, so its author,
	// timestamps and text are what the file said (AR-09).
	Imported bool           `json:"imported"`
	Links    []EvidenceLink `json:"links"`
}
