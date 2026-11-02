package domain

import "encoding/json"

// ExportFormat and ExportVersion identify the investigation export file
// format (D-23). Version 2 adds conclusion accountability. The reader also
// accepts version 1, whose conclusions start without a trusted review.
const (
	ExportFormat  = "casebook.investigation"
	ExportVersion = 2
)

// ExportDocument is one investigation exported as versioned JSON. Records are
// addressed by refs (E-001, C-002, ...), never by internal ids. Every array
// is present (possibly empty); records are ordered by ref number, links by
// record ref then evidence ref, and activity in the order it was written.
type ExportDocument struct {
	Format          string                 `json:"format"`
	Version         int                    `json:"version"`
	ExportedAt      string                 `json:"exported_at"`
	Investigation   ExportInvestigation    `json:"investigation"`
	Evidence        []ExportEvidence       `json:"evidence"`
	Claims          []ExportClaim          `json:"claims"`
	Hypotheses      []ExportHypothesis     `json:"hypotheses"`
	Questions       []ExportQuestion       `json:"questions"`
	ClaimLinks      []ExportClaimLink      `json:"claim_links"`
	HypothesisLinks []ExportHypothesisLink `json:"hypothesis_links"`
	Activity        []ExportActivity       `json:"activity"`
}

// ExportInvestigation is the investigation row of an export.
// PreArchiveStatus and ArchivedAt are "" unless the investigation is
// archived.
type ExportInvestigation struct {
	Ref                           string              `json:"ref"`
	Title                         string              `json:"title"`
	ProblemStatement              string              `json:"problem_statement"`
	Status                        InvestigationStatus `json:"status"`
	PreArchiveStatus              InvestigationStatus `json:"pre_archive_status"`
	Conclusion                    string              `json:"conclusion"`
	ConclusionConfidence          Confidence          `json:"conclusion_confidence"`
	ConclusionRationale           string              `json:"conclusion_rationale"`
	ConclusionVerification        Verification        `json:"conclusion_verification,omitempty"`
	ConclusionSupportingClaims    []string            `json:"conclusion_supporting_claims,omitempty"`
	ConclusionContradictingClaims []string            `json:"conclusion_contradicting_claims,omitempty"`
	ConclusionReviewedAt          string              `json:"conclusion_reviewed_at,omitempty"`
	ConclusionReviewBasis         string              `json:"conclusion_review_basis,omitempty"`
	ConclusionNeedsReview         bool                `json:"conclusion_needs_review,omitempty"`
	CreatedAt                     string              `json:"created_at"`
	UpdatedAt                     string              `json:"updated_at"`
	ArchivedAt                    string              `json:"archived_at"`
	CreatedBy                     Actor               `json:"created_by"`
}

// ExportEvidence is one evidence row of an export, including raw detail.
// Metadata is the evidence metadata embedded as a JSON object; it is omitted
// when the evidence has none.
type ExportEvidence struct {
	Ref          string          `json:"ref"`
	Title        string          `json:"title"`
	Summary      string          `json:"summary"`
	RawDetail    string          `json:"raw_detail"`
	SourceType   SourceType      `json:"source_type"`
	SourceRef    string          `json:"source_ref"`
	SourceLabel  string          `json:"source_label"`
	CapturedAt   string          `json:"captured_at"`
	Verification Verification    `json:"verification"`
	Metadata     json.RawMessage `json:"metadata,omitempty"`
	CreatedAt    string          `json:"created_at"`
	UpdatedAt    string          `json:"updated_at"`
	CreatedBy    Actor           `json:"created_by"`
}

// ExportClaim is one claim of an export. SupersededBy is the ref of the
// replacing claim, or "".
type ExportClaim struct {
	Ref                 string       `json:"ref"`
	Statement           string       `json:"statement"`
	Confidence          Confidence   `json:"confidence"`
	ConfidenceRationale string       `json:"confidence_rationale"`
	Status              ClaimStatus  `json:"status"`
	Verification        Verification `json:"verification"`
	SupersededBy        string       `json:"superseded_by"`
	CreatedAt           string       `json:"created_at"`
	UpdatedAt           string       `json:"updated_at"`
	CreatedBy           Actor        `json:"created_by"`
}

// ExportHypothesis is one hypothesis of an export. SupersededBy is the ref
// of the replacing hypothesis, or "".
type ExportHypothesis struct {
	Ref          string           `json:"ref"`
	Statement    string           `json:"statement"`
	Confidence   Confidence       `json:"confidence"`
	Rationale    string           `json:"rationale"`
	Status       HypothesisStatus `json:"status"`
	Verification Verification     `json:"verification"`
	SupersededBy string           `json:"superseded_by"`
	CreatedAt    string           `json:"created_at"`
	UpdatedAt    string           `json:"updated_at"`
	CreatedBy    Actor            `json:"created_by"`
}

// ExportQuestion is one question of an export. ResolvedAt is "" unless the
// question is resolved.
type ExportQuestion struct {
	Ref        string         `json:"ref"`
	Question   string         `json:"question"`
	Status     QuestionStatus `json:"status"`
	Resolution string         `json:"resolution"`
	DeferNote  string         `json:"defer_note"`
	ResolvedAt string         `json:"resolved_at"`
	CreatedAt  string         `json:"created_at"`
	UpdatedAt  string         `json:"updated_at"`
	CreatedBy  Actor          `json:"created_by"`
}

// ExportClaimLink links a claim to evidence in an export.
type ExportClaimLink struct {
	ClaimRef    string   `json:"claim_ref"`
	EvidenceRef string   `json:"evidence_ref"`
	Relation    Relation `json:"relation"`
	CreatedAt   string   `json:"created_at"`
	CreatedBy   Actor    `json:"created_by"`
}

// ExportHypothesisLink links a hypothesis to evidence in an export.
type ExportHypothesisLink struct {
	HypothesisRef string   `json:"hypothesis_ref"`
	EvidenceRef   string   `json:"evidence_ref"`
	Relation      Relation `json:"relation"`
	CreatedAt     string   `json:"created_at"`
	CreatedBy     Actor    `json:"created_by"`
}

// ExportActivity is one activity row of the exported investigation.
type ExportActivity struct {
	Actor     Actor  `json:"actor"`
	Action    string `json:"action"`
	TargetRef string `json:"target_ref"`
	Summary   string `json:"summary"`
	Note      string `json:"note"`
	CreatedAt string `json:"created_at"`
}

// ImportResult reports an ImportInvestigation. InvestigationRef is the ref
// the investigation received; OriginalRef is the ref in the file; RefChanged
// is true when the original ref was already taken. Counts holds what was
// inserted (Investigations is 1; Activity includes the import row).
// SameCaseRefs lists the investigations already in the database that are
// the same case (the original or an earlier import of it: same creation
// time and creator), oldest ref first; the import is a separate copy of
// them, never a merge (ADR-0016). It is empty (omitted from JSON) when
// there are none.
type ImportResult struct {
	InvestigationRef string `json:"investigation_ref"`
	// Title is the imported investigation's title.
	Title       string `json:"title"`
	OriginalRef string `json:"original_ref"`
	RefChanged  bool   `json:"ref_changed"`
	Counts      Counts `json:"counts"`
	// SameCaseRefs: see the type comment.
	SameCaseRefs []string `json:"same_case_refs,omitempty"`
}
