package domain

import "strings"

// This file defines every D-09 enum as a named string type. Each has exported
// constants, All<Plural>() (a fresh slice in declaration order), Valid(),
// Validate(field), Label() for display, and Parse<Enum>(s) which trims and
// lowercases. There are no numeric scores anywhere.

// enumMessage lists the allowed values for a validation message.
func enumMessage[T ~string](all []T) string {
	parts := make([]string, len(all))
	for i, v := range all {
		parts[i] = string(v)
	}
	return "must be one of " + strings.Join(parts, ", ")
}

func enumValid[T ~string](v T, all []T) bool {
	for _, a := range all {
		if v == a {
			return true
		}
	}
	return false
}

func enumValidate[T ~string](field string, v T, all []T) error {
	if enumValid(v, all) {
		return nil
	}
	return NewValidationError(field, "%s, got %q", enumMessage(all), string(v))
}

func enumParse[T ~string](field, s string, all []T) (T, error) {
	v := T(strings.ToLower(strings.TrimSpace(s)))
	if err := enumValidate(field, v, all); err != nil {
		return "", err
	}
	return v, nil
}

// InvestigationStatus is the lifecycle state of an investigation.
type InvestigationStatus string

// Investigation statuses.
const (
	InvestigationStatusNew           InvestigationStatus = "new"
	InvestigationStatusInvestigating InvestigationStatus = "investigating"
	InvestigationStatusBlocked       InvestigationStatus = "blocked"
	InvestigationStatusResolved      InvestigationStatus = "resolved"
	InvestigationStatusArchived      InvestigationStatus = "archived"
)

// AllInvestigationStatuses returns every investigation status.
func AllInvestigationStatuses() []InvestigationStatus {
	return []InvestigationStatus{InvestigationStatusNew, InvestigationStatusInvestigating,
		InvestigationStatusBlocked, InvestigationStatusResolved, InvestigationStatusArchived}
}

// Valid reports whether s is a known investigation status.
func (s InvestigationStatus) Valid() bool { return enumValid(s, AllInvestigationStatuses()) }

// Validate returns a *ValidationError on field when s is not valid.
func (s InvestigationStatus) Validate(field string) error {
	return enumValidate(field, s, AllInvestigationStatuses())
}

// Label returns a display label.
func (s InvestigationStatus) Label() string {
	switch s {
	case InvestigationStatusNew:
		return "New"
	case InvestigationStatusInvestigating:
		return "Investigating"
	case InvestigationStatusBlocked:
		return "Blocked"
	case InvestigationStatusResolved:
		return "Resolved"
	case InvestigationStatusArchived:
		return "Archived"
	}
	return string(s)
}

// ParseInvestigationStatus parses s case-insensitively.
func ParseInvestigationStatus(s string) (InvestigationStatus, error) {
	return enumParse("status", s, AllInvestigationStatuses())
}

// Confidence is a qualitative confidence level.
type Confidence string

// Confidence levels.
const (
	ConfidenceUnknown Confidence = "unknown"
	ConfidenceLow     Confidence = "low"
	ConfidenceMedium  Confidence = "medium"
	ConfidenceHigh    Confidence = "high"
)

// AllConfidences returns every confidence level.
func AllConfidences() []Confidence {
	return []Confidence{ConfidenceUnknown, ConfidenceLow, ConfidenceMedium, ConfidenceHigh}
}

// Valid reports whether c is a known confidence level.
func (c Confidence) Valid() bool { return enumValid(c, AllConfidences()) }

// Validate returns a *ValidationError on field when c is not valid.
func (c Confidence) Validate(field string) error { return enumValidate(field, c, AllConfidences()) }

// Label returns a display label.
func (c Confidence) Label() string {
	switch c {
	case ConfidenceUnknown:
		return "Unknown"
	case ConfidenceLow:
		return "Low"
	case ConfidenceMedium:
		return "Medium"
	case ConfidenceHigh:
		return "High"
	}
	return string(c)
}

// ParseConfidence parses s case-insensitively.
func ParseConfidence(s string) (Confidence, error) {
	return enumParse("confidence", s, AllConfidences())
}

// Verification says how far a record has been checked.
type Verification string

// Verification states.
const (
	VerificationAIGenerated    Verification = "ai_generated"
	VerificationHumanVerified  Verification = "human_verified"
	VerificationSourceVerified Verification = "source_verified"
	VerificationDisputed       Verification = "disputed"
	VerificationSuperseded     Verification = "superseded"
)

// AllVerifications returns every verification state.
func AllVerifications() []Verification {
	return []Verification{VerificationAIGenerated, VerificationHumanVerified, VerificationSourceVerified,
		VerificationDisputed, VerificationSuperseded}
}

// Valid reports whether v is a known verification state.
func (v Verification) Valid() bool { return enumValid(v, AllVerifications()) }

// Validate returns a *ValidationError on field when v is not valid.
func (v Verification) Validate(field string) error { return enumValidate(field, v, AllVerifications()) }

// Label returns a display label.
func (v Verification) Label() string {
	switch v {
	case VerificationAIGenerated:
		return "AI generated"
	case VerificationHumanVerified:
		return "Human verified"
	case VerificationSourceVerified:
		return "Source verified"
	case VerificationDisputed:
		return "Disputed"
	case VerificationSuperseded:
		return "Superseded"
	}
	return string(v)
}

// ParseVerification parses s case-insensitively.
func ParseVerification(s string) (Verification, error) {
	return enumParse("verification", s, AllVerifications())
}

// DefaultVerification is the verification a new record gets when the caller
// does not provide one: human_verified for humans, ai_generated otherwise.
func DefaultVerification(k ActorKind) Verification {
	if k == ActorHuman {
		return VerificationHumanVerified
	}
	return VerificationAIGenerated
}

// ClaimStatus is the state of a claim.
type ClaimStatus string

// Claim statuses.
const (
	ClaimStatusActive     ClaimStatus = "active"
	ClaimStatusDisputed   ClaimStatus = "disputed"
	ClaimStatusSuperseded ClaimStatus = "superseded"
	ClaimStatusRejected   ClaimStatus = "rejected"
)

// AllClaimStatuses returns every claim status.
func AllClaimStatuses() []ClaimStatus {
	return []ClaimStatus{ClaimStatusActive, ClaimStatusDisputed, ClaimStatusSuperseded, ClaimStatusRejected}
}

// Valid reports whether s is a known claim status.
func (s ClaimStatus) Valid() bool { return enumValid(s, AllClaimStatuses()) }

// Validate returns a *ValidationError on field when s is not valid.
func (s ClaimStatus) Validate(field string) error { return enumValidate(field, s, AllClaimStatuses()) }

// Label returns a display label.
func (s ClaimStatus) Label() string {
	switch s {
	case ClaimStatusActive:
		return "Current"
	case ClaimStatusDisputed:
		return "Disputed"
	case ClaimStatusSuperseded:
		return "Superseded"
	case ClaimStatusRejected:
		return "Rejected"
	}
	return string(s)
}

// ParseClaimStatus parses s case-insensitively.
func ParseClaimStatus(s string) (ClaimStatus, error) {
	return enumParse("status", s, AllClaimStatuses())
}

// HypothesisStatus is the state of a hypothesis.
type HypothesisStatus string

// Hypothesis statuses.
const (
	HypothesisStatusOpen       HypothesisStatus = "open"
	HypothesisStatusSupported  HypothesisStatus = "supported"
	HypothesisStatusDisproven  HypothesisStatus = "disproven"
	HypothesisStatusSuperseded HypothesisStatus = "superseded"
)

// AllHypothesisStatuses returns every hypothesis status.
func AllHypothesisStatuses() []HypothesisStatus {
	return []HypothesisStatus{HypothesisStatusOpen, HypothesisStatusSupported, HypothesisStatusDisproven,
		HypothesisStatusSuperseded}
}

// Valid reports whether s is a known hypothesis status.
func (s HypothesisStatus) Valid() bool { return enumValid(s, AllHypothesisStatuses()) }

// Validate returns a *ValidationError on field when s is not valid.
func (s HypothesisStatus) Validate(field string) error {
	return enumValidate(field, s, AllHypothesisStatuses())
}

// Label returns a display label.
func (s HypothesisStatus) Label() string {
	switch s {
	case HypothesisStatusOpen:
		return "Open"
	case HypothesisStatusSupported:
		return "Supported"
	case HypothesisStatusDisproven:
		return "Disproven"
	case HypothesisStatusSuperseded:
		return "Superseded"
	}
	return string(s)
}

// ParseHypothesisStatus parses s case-insensitively.
func ParseHypothesisStatus(s string) (HypothesisStatus, error) {
	return enumParse("status", s, AllHypothesisStatuses())
}

// QuestionStatus is the state of an open question.
type QuestionStatus string

// Question statuses.
const (
	QuestionStatusOpen     QuestionStatus = "open"
	QuestionStatusResolved QuestionStatus = "resolved"
	QuestionStatusDeferred QuestionStatus = "deferred"
)

// AllQuestionStatuses returns every question status.
func AllQuestionStatuses() []QuestionStatus {
	return []QuestionStatus{QuestionStatusOpen, QuestionStatusResolved, QuestionStatusDeferred}
}

// Valid reports whether s is a known question status.
func (s QuestionStatus) Valid() bool { return enumValid(s, AllQuestionStatuses()) }

// Validate returns a *ValidationError on field when s is not valid.
func (s QuestionStatus) Validate(field string) error {
	return enumValidate(field, s, AllQuestionStatuses())
}

// Label returns a display label.
func (s QuestionStatus) Label() string {
	switch s {
	case QuestionStatusOpen:
		return "Open"
	case QuestionStatusResolved:
		return "Resolved"
	case QuestionStatusDeferred:
		return "Deferred"
	}
	return string(s)
}

// ParseQuestionStatus parses s case-insensitively.
func ParseQuestionStatus(s string) (QuestionStatus, error) {
	return enumParse("status", s, AllQuestionStatuses())
}

// SourceType says where a piece of evidence came from.
type SourceType string

// Evidence source types.
const (
	SourceTypeSQLQuery          SourceType = "sql_query"
	SourceTypeCode              SourceType = "code"
	SourceTypeFile              SourceType = "file"
	SourceTypeJira              SourceType = "jira"
	SourceTypeDocumentation     SourceType = "documentation"
	SourceTypeAPIResponse       SourceType = "api_response"
	SourceTypeUserStatement     SourceType = "user_statement"
	SourceTypeAIToolOutput      SourceType = "ai_tool_output"
	SourceTypeManualObservation SourceType = "manual_observation"
	SourceTypeExternalURL       SourceType = "external_url"
	SourceTypeCasebookArtifact  SourceType = "casebook_artifact"
	// SourceTypeEvernoteNote is a legacy source type. Casebook no longer
	// has an Evernote integration, but the value stays valid (and keeps its
	// label) so existing evidence and older case exports stay readable and
	// importable and are never relabelled. No description or hint
	// recommends it for new evidence.
	SourceTypeEvernoteNote SourceType = "evernote_note"
)

// AllSourceTypes returns every evidence source type, the legacy
// SourceTypeEvernoteNote included (it stays valid for compatibility).
func AllSourceTypes() []SourceType {
	return []SourceType{SourceTypeSQLQuery, SourceTypeCode, SourceTypeFile, SourceTypeJira,
		SourceTypeDocumentation, SourceTypeAPIResponse, SourceTypeUserStatement, SourceTypeAIToolOutput,
		SourceTypeManualObservation, SourceTypeExternalURL, SourceTypeCasebookArtifact, SourceTypeEvernoteNote}
}

// Valid reports whether t is a known source type.
func (t SourceType) Valid() bool { return enumValid(t, AllSourceTypes()) }

// Validate returns a *ValidationError on field when t is not valid.
func (t SourceType) Validate(field string) error { return enumValidate(field, t, AllSourceTypes()) }

// Label returns a display label.
func (t SourceType) Label() string {
	switch t {
	case SourceTypeSQLQuery:
		return "SQL query"
	case SourceTypeCode:
		return "Code"
	case SourceTypeFile:
		return "File"
	case SourceTypeJira:
		return "Jira"
	case SourceTypeDocumentation:
		return "Documentation"
	case SourceTypeAPIResponse:
		return "API response"
	case SourceTypeUserStatement:
		return "User statement"
	case SourceTypeAIToolOutput:
		return "AI tool output"
	case SourceTypeManualObservation:
		return "Manual observation"
	case SourceTypeExternalURL:
		return "External URL"
	case SourceTypeCasebookArtifact:
		return "Casebook artifact"
	case SourceTypeEvernoteNote: // legacy; existing evidence keeps its label
		return "Evernote note"
	}
	return string(t)
}

// ParseSourceType parses s case-insensitively.
func ParseSourceType(s string) (SourceType, error) {
	return enumParse("source_type", s, AllSourceTypes())
}

// Relation says how evidence bears on a claim or hypothesis.
type Relation string

// Link relations.
const (
	RelationSupports    Relation = "supports"
	RelationContradicts Relation = "contradicts"
)

// AllRelations returns every link relation.
func AllRelations() []Relation { return []Relation{RelationSupports, RelationContradicts} }

// Valid reports whether r is a known relation.
func (r Relation) Valid() bool { return enumValid(r, AllRelations()) }

// Validate returns a *ValidationError on field when r is not valid.
func (r Relation) Validate(field string) error { return enumValidate(field, r, AllRelations()) }

// Label returns a display label.
func (r Relation) Label() string {
	switch r {
	case RelationSupports:
		return "Supports"
	case RelationContradicts:
		return "Contradicts"
	}
	return string(r)
}

// ParseRelation parses s case-insensitively.
func ParseRelation(s string) (Relation, error) {
	return enumParse("relation", s, AllRelations())
}

// ActorKind says what kind of party performed a mutation.
type ActorKind string

// Actor kinds.
const (
	ActorHuman  ActorKind = "human"
	ActorAI     ActorKind = "ai"
	ActorImport ActorKind = "import"
	ActorSystem ActorKind = "system"
)

// AllActorKinds returns every actor kind.
func AllActorKinds() []ActorKind { return []ActorKind{ActorHuman, ActorAI, ActorImport, ActorSystem} }

// Valid reports whether k is a known actor kind.
func (k ActorKind) Valid() bool { return enumValid(k, AllActorKinds()) }

// Validate returns a *ValidationError on field when k is not valid.
func (k ActorKind) Validate(field string) error { return enumValidate(field, k, AllActorKinds()) }

// Label returns a display label.
func (k ActorKind) Label() string {
	switch k {
	case ActorHuman:
		return "Human"
	case ActorAI:
		return "AI"
	case ActorImport:
		return "Import"
	case ActorSystem:
		return "System"
	}
	return string(k)
}

// ParseActorKind parses s case-insensitively.
func ParseActorKind(s string) (ActorKind, error) {
	return enumParse("actor.kind", s, AllActorKinds())
}

// RecordType names a kind of record that carries a ref.
type RecordType string

// Record types.
const (
	RecordInvestigation RecordType = "investigation"
	RecordEvidence      RecordType = "evidence"
	RecordClaim         RecordType = "claim"
	RecordHypothesis    RecordType = "hypothesis"
	RecordQuestion      RecordType = "question"
)

// AllRecordTypes returns every record type.
func AllRecordTypes() []RecordType {
	return []RecordType{RecordInvestigation, RecordEvidence, RecordClaim, RecordHypothesis, RecordQuestion}
}

// Valid reports whether t is a known record type.
func (t RecordType) Valid() bool { return enumValid(t, AllRecordTypes()) }

// Validate returns a *ValidationError on field when t is not valid.
func (t RecordType) Validate(field string) error { return enumValidate(field, t, AllRecordTypes()) }

// Label returns a display label.
func (t RecordType) Label() string {
	switch t {
	case RecordInvestigation:
		return "Investigation"
	case RecordEvidence:
		return "Evidence"
	case RecordClaim:
		return "Claim"
	case RecordHypothesis:
		return "Hypothesis"
	case RecordQuestion:
		return "Question"
	}
	return string(t)
}

// Prefix returns the ref prefix: INV, E, C, H or Q ("" when unknown).
func (t RecordType) Prefix() string {
	switch t {
	case RecordInvestigation:
		return "INV"
	case RecordEvidence:
		return "E"
	case RecordClaim:
		return "C"
	case RecordHypothesis:
		return "H"
	case RecordQuestion:
		return "Q"
	}
	return ""
}

// ParseRecordType parses s case-insensitively.
func ParseRecordType(s string) (RecordType, error) {
	return enumParse("record_type", s, AllRecordTypes())
}
