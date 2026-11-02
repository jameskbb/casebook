// TypeScript mirrors of the internal/app JSON contract (docs/api.md sections
// 8 and 9) and the desktop glue DTOs. Field names are the JSON tags
// (snake_case). Timestamps are UTC strings in the fixed layout
// 2006-01-02T15:04:05.000Z; an unset timestamp is "".

// Enums (docs/api.md section 9).
export type InvestigationStatus = 'new' | 'investigating' | 'blocked' | 'resolved' | 'archived'
export type Confidence = 'unknown' | 'low' | 'medium' | 'high'
export type Verification = 'ai_generated' | 'human_verified' | 'source_verified' | 'disputed' | 'superseded'
export type ClaimStatus = 'active' | 'disputed' | 'superseded' | 'rejected'
export type HypothesisStatus = 'open' | 'supported' | 'disproven' | 'superseded'
export type QuestionStatus = 'open' | 'resolved' | 'deferred'
export type SourceType =
  | 'sql_query'
  | 'code'
  | 'file'
  | 'jira'
  | 'evernote_note'
  | 'documentation'
  | 'api_response'
  | 'user_statement'
  | 'ai_tool_output'
  | 'manual_observation'
  | 'external_url'
  | 'casebook_artifact'
export type Relation = 'supports' | 'contradicts'
export type ActorKind = 'human' | 'ai' | 'import' | 'system'
export type RecordType = 'investigation' | 'evidence' | 'claim' | 'hypothesis' | 'question'

export interface Actor {
  kind: ActorKind
  client: string
}

export interface Investigation {
  id: number
  ref: string
  title: string
  problem_statement: string
  status: InvestigationStatus
  conclusion: string
  conclusion_confidence: Confidence
  conclusion_rationale: string
  conclusion_verification: Verification
  conclusion_supporting_claims: string[]
  conclusion_contradicting_claims: string[]
  conclusion_reviewed_at: string
  conclusion_review_basis: string
  conclusion_needs_review: boolean
  conclusion_review_token: string
  created_at: string
  updated_at: string
  archived_at: string
  archived: boolean
  is_active: boolean
  created_by: Actor
}

export interface RecordCounts {
  evidence: number
  claims: number
  hypotheses: number
  questions: number
  open_questions: number
}

export interface InvestigationSummary {
  id: number
  ref: string
  title: string
  status: InvestigationStatus
  conclusion_confidence: Confidence
  created_at: string
  updated_at: string
  archived_at: string
  archived: boolean
  is_active: boolean
  counts: RecordCounts
}

export interface RecordLink {
  ref: string
  relation: Relation
}

export interface Evidence {
  id: number
  ref: string
  investigation_ref: string
  title: string
  summary: string
  /** Present only when requested with include_raw_detail and stored. */
  raw_detail?: string
  has_raw_detail: boolean
  source_type: SourceType
  source_ref: string
  source_label: string
  captured_at: string
  created_at: string
  updated_at: string
  created_by: Actor
  verification: Verification
  /** Compact JSON object text, or "". */
  metadata: string
  claim_links: RecordLink[]
  hypothesis_links: RecordLink[]
  /** True when the row came from an imported file (author and verification are the file's). */
  imported?: boolean
}

export interface EvidenceLink {
  evidence_ref: string
  relation: Relation
  evidence_title: string
  created_at: string
  created_by: Actor
}

export interface Claim {
  id: number
  ref: string
  investigation_ref: string
  statement: string
  confidence: Confidence
  confidence_rationale: string
  status: ClaimStatus
  verification: Verification
  superseded_by: string
  created_at: string
  updated_at: string
  created_by: Actor
  links: EvidenceLink[]
  /** True when the row came from an imported file (author and verification are the file's). */
  imported?: boolean
}

export interface Hypothesis {
  id: number
  ref: string
  investigation_ref: string
  statement: string
  confidence: Confidence
  rationale: string
  status: HypothesisStatus
  verification: Verification
  superseded_by: string
  created_at: string
  updated_at: string
  created_by: Actor
  links: EvidenceLink[]
  /** True when the row came from an imported file (author and verification are the file's). */
  imported?: boolean
}

export interface Question {
  id: number
  ref: string
  investigation_ref: string
  question: string
  status: QuestionStatus
  resolution: string
  defer_note: string
  resolved_at: string
  created_at: string
  updated_at: string
  created_by: Actor
  /** True when the row came from an imported file (author and verification are the file's). */
  imported?: boolean
}

export interface Activity {
  id: number
  investigation_ref: string
  actor: Actor
  action: string
  target_ref: string
  summary: string
  note: string
  created_at: string
  /** True when the row came from an imported file (author and verification are the file's). */
  imported?: boolean
}

export interface InvestigationDetail {
  investigation: Investigation
  evidence: Evidence[]
  claims: Claim[]
  hypotheses: Hypothesis[]
  questions: Question[]
  recent_activity: Activity[]
  review_bases?: { ref: string; basis: string }[]
}

export interface ReviewFinding {
  kind: 'claim' | 'hypothesis'
  ref: string
  statement: string
  confidence: Confidence
  rationale: string
  status: string
  verification: Verification
  evidence: { relation: Relation; evidence: Evidence }[]
  filters: string[]
  warnings: string[]
  basis: string
}

export interface ReviewStatementInput {
  kind: 'claim' | 'hypothesis'
  ref: string
  basis: string
  verification: 'human_verified' | 'disputed'
  settle_dispute: boolean
}

export interface SearchHit {
  investigation_ref: string
  investigation_title: string
  record_type: RecordType
  record_ref: string
  /** Matches wrapped in **, elisions as ... (text output only). */
  snippet: string
  /** The snippet as plain pieces with the matches flagged (render these). */
  segments?: { text: string; match: boolean }[]
  rank: number
}

export interface Counts {
  investigations: number
  evidence: number
  claims: number
  hypotheses: number
  questions: number
  claim_links: number
  hypothesis_links: number
  activity: number
}

export interface BackupInfo {
  path: string
  size_bytes: number
  schema_version: number
  created_at: string
  counts: Counts
}

export interface RestoreResult {
  source_path: string
  safety_copy_path: string
  source_schema_version: number
  counts: Counts
}

export interface ImportResult {
  investigation_ref: string
  title: string
  original_ref: string
  ref_changed: boolean
  counts: Counts
}

// Inputs and options (docs/api.md section 8, "Inputs and options").

export interface CreateInvestigationInput {
  title: string
  problem_statement?: string
  status?: InvestigationStatus | ''
  make_active?: boolean
}

/** Omitted fields are unchanged. */
export interface InvestigationPatch {
  title?: string
  problem_statement?: string
  status?: InvestigationStatus
}

export interface ConclusionInput {
  conclusion: string
  confidence?: Confidence | ''
  rationale?: string
  verification?: Verification
  supporting_claims?: string[]
  contradicting_claims?: string[]
  review?: boolean
  review_basis?: string
  expected_review_token?: string
}

export interface EvidenceInput {
  title: string
  summary: string
  source_type: SourceType
  raw_detail?: string
  source_ref?: string
  source_label?: string
  captured_at?: string
  verification?: Verification | ''
  metadata?: string
}

/** Omitted fields are unchanged; an empty string clears where allowed. */
export interface EvidencePatch {
  title?: string
  summary?: string
  raw_detail?: string
  source_type?: SourceType
  source_ref?: string
  source_label?: string
  captured_at?: string
  verification?: Verification
  metadata?: string
}

export interface GetEvidenceOptions {
  include_raw_detail?: boolean
}

export interface EvidenceLinkInput {
  evidence_ref: string
  relation: Relation
}

export interface ClaimInput {
  statement: string
  confidence?: Confidence | ''
  confidence_rationale?: string
  status?: ClaimStatus | ''
  verification?: Verification | ''
  links?: EvidenceLinkInput[]
}

export interface ClaimPatch {
  statement?: string
  confidence_rationale?: string
  confidence?: Confidence
  status?: ClaimStatus
  verification?: Verification
  superseded_by?: string
}

export interface HypothesisInput {
  statement: string
  confidence?: Confidence | ''
  rationale?: string
  status?: HypothesisStatus | ''
  verification?: Verification | ''
  links?: EvidenceLinkInput[]
}

export interface HypothesisPatch {
  statement?: string
  rationale?: string
  confidence?: Confidence
  status?: HypothesisStatus
  verification?: Verification
  superseded_by?: string
}

export interface QuestionInput {
  question: string
}

export interface DetailOptions {
  /** 0 means 20, maximum 200. */
  activity_limit?: number
  include_raw_detail?: boolean
}

export interface ListOptions {
  include_archived?: boolean
  status?: string
  /** 0 means 100, maximum 1000. */
  limit?: number
}

export interface SearchOptions {
  investigation_ref?: string
  include_archived?: boolean
  /** 0 means 20, maximum 100. */
  limit?: number
}

// Desktop glue DTOs (desktop/errors.go and desktop/files.go).

export interface StartupInfo {
  ok: boolean
  /** "schema_too_new" or "other" when ok is false. */
  kind: string
  message: string
  db_path: string
  /** True when restoring a backup can replace the data file (startup recovery). */
  recoverable?: boolean
}

export interface ReportPreview {
  markdown: string
  html: string
}

export interface ShareOptions {
  mode: 'brief' | 'report'
  claim_refs?: string[]
  /** Open or supported hypotheses (the "hypotheses" group). */
  hypothesis_refs?: string[]
  question_refs?: string[]
  ruled_out_refs?: string[]
}

export interface ShareCandidate {
  ref: string
  text: string
  group: 'claims' | 'hypotheses' | 'questions' | 'ruled_out'
  label: string
  required: boolean
  selected: boolean
}

export interface SharePreview extends ReportPreview {
  snapshot_id: string
  investigation_ref: string
  mode: 'brief' | 'report'
  candidates: ShareCandidate[]
  /** What the brief leaves out: counts, and the refs in ref order. */
  omitted: {
    claims: number
    hypotheses: number
    questions: number
    evidence: number
    claim_refs: string[]
    hypothesis_refs: string[]
    question_refs: string[]
    evidence_refs: string[]
  }
}

export interface FileResult {
  cancelled: boolean
  path: string
  file_name: string
}

export interface ImportOutcome {
  cancelled: boolean
  file_name: string
  title: string
  result: ImportResult
}

export interface BackupOutcome {
  cancelled: boolean
  info: BackupInfo
}

export interface RestorePick {
  cancelled: boolean
  path: string
  file_name: string
  info: BackupInfo
}

export type WindowTheme = 'system' | 'light' | 'dark'

// Connect AI clients panel (desktop/clients.go). Read-only: the desktop app
// never writes client configs; `casebook setup <client>` does.

export interface ClientConnection {
  configured: boolean
  matches_cli: boolean
  command: string
  command_exists: boolean
  config_path: string
  config_problem: boolean
  hooks: string[]
  setup_command: string
  /** The same command for Command Prompt on Windows; "" when not needed. */
  setup_command_cmd?: string
  /**
   * Registers the Windows CLI with the client installed inside WSL, for a
   * WSL shell (bash). Windows only; "" when not available.
   */
  setup_command_wsl?: string
  snippet: string
}

export interface ClientConnections {
  cli_path: string
  /** "bundled", "nearby", "path" or "missing". */
  cli_source: 'bundled' | 'nearby' | 'path' | 'missing'
  cli_name: string
  /** Set when the app uses a non-default database; the snippets pass --db. */
  db_path: string
  /** The shell setup_command is written for: "PowerShell" on Windows, else "terminal". */
  shell?: string
  claude: ClientConnection
  codex: ClientConnection
}

// Check for updates (internal/update, desktop/update.go; ADR 0022). The
// check runs only when the person asks; Go makes the one request to GitHub.

/** How this build compares with the latest release. */
export type UpdateState = 'update_available' | 'up_to_date' | 'ahead' | 'unknown'

/** The download a release offers for this system. */
export interface UpdateAsset {
  name: string
  /** The browser download address on github.com. */
  url: string
  size_bytes: number
}

/** The result of one check (update.Result). */
export interface UpdateCheck {
  /** This build's version, for example v0.4.0, or dev. */
  current: string
  /** The latest release's tag, for example v0.5.0. */
  latest: string
  state: UpdateState
  /** The latest release's page on github.com. */
  release_url: string
  /** UTC timestamp. */
  published_at: string
  /** Absent (Go omits it) or null when the release has no download for this system. */
  asset?: UpdateAsset | null
  /** UTC timestamp. */
  checked_at: string
}
