// Package contextpack turns a snapshot of one investigation into a concise,
// deterministic context pack for an AI client, rendered as Markdown or
// versioned JSON (D-25).
//
// The package is pure: it never touches the database, the clock or random
// numbers, and never iterates a map. Every input slice is copied and sorted
// by ref number (activity by id), so the same snapshot always yields the
// same bytes regardless of input order. Every timestamp in the output comes
// from the snapshot.
package contextpack

import "github.com/jameskbb/casebook/internal/domain"

// Format and Version identify the JSON rendering.
const (
	Format  = "casebook.context"
	Version = 1
)

// DefaultCompactMaxBytes is the Markdown budget Compact applies when
// MaxBytes is 0 (sized for session hook injection).
const DefaultCompactMaxBytes = 6000

// RationaleBytes caps the claim and hypothesis rationales a pack carries.
const RationaleBytes = 300

// DefaultRecentChanges is the number of material changes kept when
// Options.RecentChanges is 0.
const DefaultRecentChanges = 10

// Snapshot is everything Build reads. Evidence never needs raw detail.
// Activity may hold any number of rows of the investigation; Build sorts it
// newest first by id.
type Snapshot struct {
	Investigation domain.Investigation
	Evidence      []domain.Evidence
	Claims        []domain.Claim
	Hypotheses    []domain.Hypothesis
	Questions     []domain.Question
	Activity      []domain.Activity
}

// Options tunes Build. MaxBytes > 0 caps the Markdown size; Compact applies
// DefaultCompactMaxBytes when MaxBytes is 0. RecentChanges 0 means 10.
type Options struct {
	Compact       bool `json:"compact"`
	MaxBytes      int  `json:"max_bytes"`
	RecentChanges int  `json:"recent_changes"`
}

// Pack is a built context pack. Every slice is non-nil. Truncated says the
// size budget removed or shortened something; OmittedSections lists the
// sections dropped entirely, in drop order, using these JSON field names
// (rationale for the conclusion rationale); Omitted counts items removed
// from the end of the current claims, hypotheses, open questions and
// disproven hypotheses, and the disputed claims dropped.
// MaxBytes is the budget that was applied (0 when none).
type Pack struct {
	Format               string                     `json:"format"`
	Version              int                        `json:"version"`
	Investigation        PackInvestigation          `json:"investigation"`
	Conclusion           PackConclusion             `json:"conclusion"`
	EstablishedClaims    []PackClaim                `json:"established_claims"`
	DisputedClaims       []PackClaim                `json:"disputed_claims"`
	Hypotheses           []PackHypothesis           `json:"hypotheses"`
	OpenQuestions        []PackQuestion             `json:"open_questions"`
	DeferredQuestions    []PackQuestion             `json:"deferred_questions"`
	DisprovenHypotheses  []PackOneLine              `json:"disproven_hypotheses"`
	EvidenceIndex        []PackEvidence             `json:"evidence_index"`
	EvidenceVerification []PackEvidenceVerification `json:"evidence_verification"`
	RecentChanges        []PackChange               `json:"recent_changes"`
	Truncated            bool                       `json:"truncated"`
	OmittedSections      []string                   `json:"omitted_sections"`
	Omitted              PackOmitted                `json:"omitted"`
	MaxBytes             int                        `json:"max_bytes"`
}

// PackInvestigation is the pack header. PrimaryQuestion is the
// investigation's problem statement.
type PackInvestigation struct {
	Ref             string                     `json:"ref"`
	Title           string                     `json:"title"`
	Status          domain.InvestigationStatus `json:"status"`
	PrimaryQuestion string                     `json:"primary_question"`
}

// PackConclusion is the current conclusion ("" when not yet set).
type PackConclusion struct {
	Text                string                `json:"text"`
	Confidence          domain.Confidence     `json:"confidence"`
	Rationale           string                `json:"rationale"`
	Verification        domain.Verification   `json:"verification"`
	SupportingClaims    []PackConclusionClaim `json:"supporting_claims"`
	ContradictingClaims []PackConclusionClaim `json:"contradicting_claims"`
	ReviewedAt          string                `json:"reviewed_at"`
	ReviewBasis         string                `json:"review_basis"`
	NeedsReview         bool                  `json:"needs_review"`
}

// PackConclusionClaim preserves selected basis labels even when a claim's
// status or the size budget excludes its full statement.
type PackConclusionClaim struct {
	Ref          string              `json:"ref"`
	Status       domain.ClaimStatus  `json:"status"`
	Verification domain.Verification `json:"verification"`
	Missing      bool                `json:"missing,omitempty"`
}

// PackClaim is an active or disputed claim with its evidence refs, each list
// ordered by ref number. Rationale is the confidence rationale cut to one
// line of at most RationaleBytes ("" when none, or when the budget dropped
// it). Imported marks a record copied from an imported file, whose text and
// author are what the file claimed (as on every pack item below).
type PackClaim struct {
	Ref                   string              `json:"ref"`
	Statement             string              `json:"statement"`
	Confidence            domain.Confidence   `json:"confidence"`
	Verification          domain.Verification `json:"verification"`
	Rationale             string              `json:"rationale"`
	SupportingEvidence    []string            `json:"supporting_evidence"`
	ContradictingEvidence []string            `json:"contradicting_evidence"`
	Imported              bool                `json:"imported"`
}

// PackHypothesis is an open or supported hypothesis with its evidence refs
// and its rationale (as PackClaim).
type PackHypothesis struct {
	Ref                   string                  `json:"ref"`
	Statement             string                  `json:"statement"`
	Confidence            domain.Confidence       `json:"confidence"`
	Status                domain.HypothesisStatus `json:"status"`
	Verification          domain.Verification     `json:"verification"`
	Rationale             string                  `json:"rationale"`
	SupportingEvidence    []string                `json:"supporting_evidence"`
	ContradictingEvidence []string                `json:"contradicting_evidence"`
	Imported              bool                    `json:"imported"`
}

// PackQuestion is an open or deferred question. Note is the defer note.
type PackQuestion struct {
	Ref      string `json:"ref"`
	Question string `json:"question"`
	Note     string `json:"note"`
	Imported bool   `json:"imported"`
}

// PackOneLine is a record reduced to its ref and one line of text. For a
// disproven hypothesis, ContradictingEvidence lists the refs of the evidence
// linked as contradicting it, ordered by ref number (empty when none), so a
// reader sees what ruled it out, and Verification says who checked the
// record.
type PackOneLine struct {
	Ref                   string              `json:"ref"`
	Text                  string              `json:"text"`
	ContradictingEvidence []string            `json:"contradicting_evidence"`
	Imported              bool                `json:"imported"`
	Verification          domain.Verification `json:"verification"`
}

// PackEvidence is one evidence index entry.
type PackEvidence struct {
	Ref          string              `json:"ref"`
	Verification domain.Verification `json:"verification"`
	Title        string              `json:"title"`
	SourceLabel  string              `json:"source_label"`
	SourceType   domain.SourceType   `json:"source_type"`
	Imported     bool                `json:"imported"`
}

// PackEvidenceVerification keeps the verification of every cited evidence
// item even when the budget drops titles and sources from EvidenceIndex.
type PackEvidenceVerification struct {
	Ref          string              `json:"ref"`
	Verification domain.Verification `json:"verification"`
}

// PackChange is one material activity row. Imported marks a historical
// row copied from an imported file, whose actor and summary are what the
// file claimed.
type PackChange struct {
	At        string       `json:"at"`
	Actor     domain.Actor `json:"actor"`
	Action    string       `json:"action"`
	TargetRef string       `json:"target_ref"`
	Summary   string       `json:"summary"`
	Imported  bool         `json:"imported"`
}

// PackOmitted counts items the budget removed from lists it keeps.
type PackOmitted struct {
	EstablishedClaims int `json:"established_claims"`
	DisputedClaims    int `json:"disputed_claims"`
	Hypotheses        int `json:"hypotheses"`
	OpenQuestions     int `json:"open_questions"`
	// DisprovenHypotheses counts one-liners removed from the end of
	// disproven_hypotheses.
	DisprovenHypotheses int `json:"disproven_hypotheses"`
	// EvidenceOutsideFindings counts evidence not cited by the findings selected
	// before budgeting, including evidence linked only to excluded records.
	EvidenceOutsideFindings int `json:"evidence_outside_findings"`
}
