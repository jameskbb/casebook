package app

import (
	"github.com/jameskbb/casebook/internal/contextpack"
	"github.com/jameskbb/casebook/internal/domain"
)

// This file re-exports internal/domain and internal/contextpack so surfaces
// import only internal/app (D-03). Everything here is an alias: values flow
// between packages unchanged.

// DTOs and value types.
type (
	Actor                    = domain.Actor
	ActorKind                = domain.ActorKind
	Activity                 = domain.Activity
	ActivityFilter           = domain.ActivityFilter
	Investigation            = domain.Investigation
	CreateInvestigationInput = domain.CreateInvestigationInput
	InvestigationPatch       = domain.InvestigationPatch
	ConclusionInput          = domain.ConclusionInput
	Evidence                 = domain.Evidence
	EvidenceInput            = domain.EvidenceInput
	EvidencePatch            = domain.EvidencePatch
	GetEvidenceOptions       = domain.GetEvidenceOptions
	RecordLink               = domain.RecordLink
	EvidenceLinkInput        = domain.EvidenceLinkInput
	EvidenceLink             = domain.EvidenceLink
	Claim                    = domain.Claim
	ClaimInput               = domain.ClaimInput
	ClaimPatch               = domain.ClaimPatch
	Hypothesis               = domain.Hypothesis
	HypothesisInput          = domain.HypothesisInput
	HypothesisPatch          = domain.HypothesisPatch
	Question                 = domain.Question
	QuestionInput            = domain.QuestionInput
	Counts                   = domain.Counts
	StatusInfo               = domain.StatusInfo
	DetailOptions            = domain.DetailOptions
	InvestigationDetail      = domain.InvestigationDetail
	ListOptions              = domain.ListOptions
	RecordCounts             = domain.RecordCounts
	InvestigationSummary     = domain.InvestigationSummary
	SearchOptions            = domain.SearchOptions
	SearchHit                = domain.SearchHit
	SnippetSegment           = domain.SnippetSegment
	CheckpointItem           = domain.CheckpointItem
	CheckpointResult         = domain.CheckpointResult
	VerificationReset        = domain.VerificationReset
	CheckpointInput          = domain.CheckpointInput
	CheckpointEvidence       = domain.CheckpointEvidence
	ClaimUpdate              = domain.ClaimUpdate
	HypothesisUpdate         = domain.HypothesisUpdate
	QuestionUpdate           = domain.QuestionUpdate
	InvestigationStatus      = domain.InvestigationStatus
	Confidence               = domain.Confidence
	Verification             = domain.Verification
	ClaimStatus              = domain.ClaimStatus
	HypothesisStatus         = domain.HypothesisStatus
	QuestionStatus           = domain.QuestionStatus
	SourceType               = domain.SourceType
	Relation                 = domain.Relation
	RecordType               = domain.RecordType
	RecordRef                = domain.RecordRef
	ValidationError          = domain.ValidationError
	IntegrityError           = domain.IntegrityError
	ExportDocument           = domain.ExportDocument
	ExportInvestigation      = domain.ExportInvestigation
	ExportEvidence           = domain.ExportEvidence
	ExportClaim              = domain.ExportClaim
	ExportHypothesis         = domain.ExportHypothesis
	ExportQuestion           = domain.ExportQuestion
	ExportClaimLink          = domain.ExportClaimLink
	ExportHypothesisLink     = domain.ExportHypothesisLink
	ExportActivity           = domain.ExportActivity
	ImportResult             = domain.ImportResult
	BackupInfo               = domain.BackupInfo
	RestoreResult            = domain.RestoreResult
)

// Investigation export file format marker and version (D-23).
const (
	ExportFormat  = domain.ExportFormat
	ExportVersion = domain.ExportVersion
)

// Context pack types (internal/contextpack).
type (
	ContextOptions    = contextpack.Options
	Pack              = contextpack.Pack
	PackInvestigation = contextpack.PackInvestigation
	PackConclusion    = contextpack.PackConclusion
	PackClaim         = contextpack.PackClaim
	PackHypothesis    = contextpack.PackHypothesis
	PackQuestion      = contextpack.PackQuestion
	PackOneLine       = contextpack.PackOneLine
	PackEvidence      = contextpack.PackEvidence
	PackChange        = contextpack.PackChange
	PackOmitted       = contextpack.PackOmitted
)

// Context pack constants: the JSON format marker and version, the compact
// budget and the default number of recent changes.
const (
	ContextFormat          = contextpack.Format
	ContextVersion         = contextpack.Version
	DefaultCompactMaxBytes = contextpack.DefaultCompactMaxBytes
	DefaultRecentChanges   = contextpack.DefaultRecentChanges
)

// Actor kinds.
const (
	ActorHuman  = domain.ActorHuman
	ActorAI     = domain.ActorAI
	ActorImport = domain.ActorImport
	ActorSystem = domain.ActorSystem
)

// Investigation statuses.
const (
	InvestigationStatusNew           = domain.InvestigationStatusNew
	InvestigationStatusInvestigating = domain.InvestigationStatusInvestigating
	InvestigationStatusBlocked       = domain.InvestigationStatusBlocked
	InvestigationStatusResolved      = domain.InvestigationStatusResolved
	InvestigationStatusArchived      = domain.InvestigationStatusArchived
)

// Confidence levels.
const (
	ConfidenceUnknown = domain.ConfidenceUnknown
	ConfidenceLow     = domain.ConfidenceLow
	ConfidenceMedium  = domain.ConfidenceMedium
	ConfidenceHigh    = domain.ConfidenceHigh
)

// Verification states.
const (
	VerificationAIGenerated    = domain.VerificationAIGenerated
	VerificationHumanVerified  = domain.VerificationHumanVerified
	VerificationSourceVerified = domain.VerificationSourceVerified
	VerificationDisputed       = domain.VerificationDisputed
	VerificationSuperseded     = domain.VerificationSuperseded
)

// Claim statuses.
const (
	ClaimStatusActive     = domain.ClaimStatusActive
	ClaimStatusDisputed   = domain.ClaimStatusDisputed
	ClaimStatusSuperseded = domain.ClaimStatusSuperseded
	ClaimStatusRejected   = domain.ClaimStatusRejected
)

// Hypothesis statuses.
const (
	HypothesisStatusOpen       = domain.HypothesisStatusOpen
	HypothesisStatusSupported  = domain.HypothesisStatusSupported
	HypothesisStatusDisproven  = domain.HypothesisStatusDisproven
	HypothesisStatusSuperseded = domain.HypothesisStatusSuperseded
)

// Question statuses.
const (
	QuestionStatusOpen     = domain.QuestionStatusOpen
	QuestionStatusResolved = domain.QuestionStatusResolved
	QuestionStatusDeferred = domain.QuestionStatusDeferred
)

// Evidence source types.
const (
	SourceTypeSQLQuery          = domain.SourceTypeSQLQuery
	SourceTypeCode              = domain.SourceTypeCode
	SourceTypeFile              = domain.SourceTypeFile
	SourceTypeJira              = domain.SourceTypeJira
	SourceTypeDocumentation     = domain.SourceTypeDocumentation
	SourceTypeAPIResponse       = domain.SourceTypeAPIResponse
	SourceTypeUserStatement     = domain.SourceTypeUserStatement
	SourceTypeAIToolOutput      = domain.SourceTypeAIToolOutput
	SourceTypeManualObservation = domain.SourceTypeManualObservation
	SourceTypeExternalURL       = domain.SourceTypeExternalURL
	SourceTypeCasebookArtifact  = domain.SourceTypeCasebookArtifact
	// SourceTypeEvernoteNote is a legacy source type kept so existing
	// evidence and older case exports stay readable and keep their label;
	// nothing recommends it for new evidence.
	SourceTypeEvernoteNote = domain.SourceTypeEvernoteNote
)

// Link relations.
const (
	RelationSupports    = domain.RelationSupports
	RelationContradicts = domain.RelationContradicts
)

// Record types.
const (
	RecordInvestigation = domain.RecordInvestigation
	RecordEvidence      = domain.RecordEvidence
	RecordClaim         = domain.RecordClaim
	RecordHypothesis    = domain.RecordHypothesis
	RecordQuestion      = domain.RecordQuestion
)

// TimeLayout is the fixed-width UTC RFC3339 layout of every timestamp.
const TimeLayout = domain.TimeLayout

// DefaultMCPClient is the client name ActorMCP falls back to when the MCP
// client sent no usable name.
const DefaultMCPClient = domain.DefaultMCPClient

// Free-text limits, in bytes.
const (
	MaxTitleBytes               = domain.MaxTitleBytes
	MaxProblemStatementBytes    = domain.MaxProblemStatementBytes
	MaxConclusionBytes          = domain.MaxConclusionBytes
	MaxConclusionRationaleBytes = domain.MaxConclusionRationaleBytes
	MaxEvidenceSummaryBytes     = domain.MaxEvidenceSummaryBytes
	MaxRawDetailBytes           = domain.MaxRawDetailBytes
	MaxSourceRefBytes           = domain.MaxSourceRefBytes
	MaxSourceLabelBytes         = domain.MaxSourceLabelBytes
	MaxMetadataBytes            = domain.MaxMetadataBytes
	MaxStatementBytes           = domain.MaxStatementBytes
	MaxRationaleBytes           = domain.MaxRationaleBytes
	MaxQuestionBytes            = domain.MaxQuestionBytes
	MaxResolutionBytes          = domain.MaxResolutionBytes
	MaxDeferNoteBytes           = domain.MaxDeferNoteBytes
	MaxActivitySummaryBytes     = domain.MaxActivitySummaryBytes
	MaxCheckpointNoteBytes      = domain.MaxCheckpointNoteBytes
	MaxCheckpointItems          = domain.MaxCheckpointItems
	MaxClientBytes              = domain.MaxClientBytes
	MaxSearchQueryBytes         = domain.MaxSearchQueryBytes
	MaxImportBytes              = domain.MaxImportBytes
)

// Ref number bounds: refs are parsed and allocated up to MaxRefNum; import
// accepts investigation ref numbers up to MaxImportInvestigationRefNum and
// record ref numbers up to MaxImportRecordRefNum.
const (
	MaxRefNum                    = domain.MaxRefNum
	MaxImportInvestigationRefNum = domain.MaxImportInvestigationRefNum
	MaxImportRecordRefNum        = domain.MaxImportRecordRefNum
)

// Errors. Test sentinels with errors.Is and validation errors with
// IsValidation.
var (
	ErrNotFound              = domain.ErrNotFound
	ErrConflict              = domain.ErrConflict
	ErrNoActiveInvestigation = domain.ErrNoActiveInvestigation
	ErrBusy                  = domain.ErrBusy
	ErrInternal              = domain.ErrInternal
	ErrSchemaTooNew          = domain.ErrSchemaTooNew
)

// Helpers.
var (
	NewValidationError = domain.NewValidationError
	IsValidation       = domain.IsValidation
	IsIntegrity        = domain.IsIntegrity

	FormatTime     = domain.FormatTime
	ParseTimestamp = domain.ParseTimestamp

	FormatInvestigationRef = domain.FormatInvestigationRef
	FormatRecordRef        = domain.FormatRecordRef
	ParseInvestigationRef  = domain.ParseInvestigationRef
	ParseRecordRef         = domain.ParseRecordRef

	SanitizeText = domain.SanitizeText
	SanitizeLine = domain.SanitizeLine
	TruncateLine = domain.TruncateLine

	ActorDesktop        = domain.ActorDesktop
	ActorCLI            = domain.ActorCLI
	ActorMCP            = domain.ActorMCP
	DefaultVerification = domain.DefaultVerification
	NormalizeMetadata   = domain.NormalizeMetadata

	AllInvestigationStatuses = domain.AllInvestigationStatuses
	AllConfidences           = domain.AllConfidences
	AllVerifications         = domain.AllVerifications
	AllClaimStatuses         = domain.AllClaimStatuses
	AllHypothesisStatuses    = domain.AllHypothesisStatuses
	AllQuestionStatuses      = domain.AllQuestionStatuses
	AllSourceTypes           = domain.AllSourceTypes
	AllRelations             = domain.AllRelations
	AllActorKinds            = domain.AllActorKinds
	AllRecordTypes           = domain.AllRecordTypes

	ParseInvestigationStatus = domain.ParseInvestigationStatus
	ParseConfidence          = domain.ParseConfidence
	ParseVerification        = domain.ParseVerification
	ParseClaimStatus         = domain.ParseClaimStatus
	ParseHypothesisStatus    = domain.ParseHypothesisStatus
	ParseQuestionStatus      = domain.ParseQuestionStatus
	ParseSourceType          = domain.ParseSourceType
	ParseRelation            = domain.ParseRelation
	ParseActorKind           = domain.ParseActorKind
	ParseRecordType          = domain.ParseRecordType
)
