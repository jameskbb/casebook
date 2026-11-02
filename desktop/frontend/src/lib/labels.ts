// Display labels for every engine enum (docs/api.md section 9) and ordered
// option lists for pickers. The one UI difference: confidence "unknown" reads
// "Not assessed" (UI-SPEC Copywriting, Confidence labels).
import type {
  ActorKind,
  ClaimStatus,
  Confidence,
  HypothesisStatus,
  InvestigationStatus,
  QuestionStatus,
  RecordType,
  Relation,
  SourceType,
  Verification,
} from '../api/types'

export const investigationStatusLabels: Record<InvestigationStatus, string> = {
  new: 'New',
  investigating: 'Investigating',
  blocked: 'Blocked',
  resolved: 'Resolved',
  archived: 'Archived',
}

/** Statuses offered by the status picker (Archived is not in the picker). */
export const investigationStatusOptions: InvestigationStatus[] = ['new', 'investigating', 'blocked', 'resolved']

/** Short confidence labels (sidebar rows). */
export const confidenceLabels: Record<Confidence, string> = {
  unknown: 'Not assessed',
  low: 'Low',
  medium: 'Medium',
  high: 'High',
}

/** Long confidence labels (record rows, conclusion). */
export const confidenceLongLabels: Record<Confidence, string> = {
  unknown: 'Not assessed',
  low: 'Low confidence',
  medium: 'Medium confidence',
  high: 'High confidence',
}

/** Confidence picker order. */
export const confidenceOptions: Confidence[] = ['unknown', 'low', 'medium', 'high']

export const verificationLabels: Record<Verification, string> = {
  ai_generated: 'AI generated',
  human_verified: 'Human verified',
  source_verified: 'Source verified',
  disputed: 'Disputed',
  superseded: 'Superseded',
}

export const verificationOptions: Verification[] = [
  'ai_generated',
  'human_verified',
  'source_verified',
  'disputed',
  'superseded',
]

/** One sentence per verification value, for tooltips and the labels legend. */
export const verificationHelp: Record<Verification, string> = {
  ai_generated: 'This version has not been marked as checked in Casebook.',
  source_verified: 'Marked as checked against its source, usually by your AI. Only Human verified means a person vouched for it.',
  human_verified: 'A person checked this and vouched for it.',
  disputed: 'Evidence contradicts this; it stays on record until it is settled.',
  superseded: 'Replaced by a newer record; kept for the history.',
}

/** Status words used in the legend that are not verification values. */
export const statusHelp = {
  open: 'An explanation still being tested; the evidence has not decided it yet.',
  supported: 'The evidence so far backs this explanation, though it is not settled.',
  disproven: 'The evidence ruled this explanation out; it stays visible as a dead end.',
  rejected: 'Withdrawn without a replacement; kept for the history.',
  current: 'A claim the case stands on now.',
}

/** Confidence words in the legend: a judgment, kept apart from verification. */
export const confidenceHelp = {
  rated: 'How sure the author was when writing it. A judgment, not a check; it does not change when something is verified.',
  notAssessed: 'No confidence recorded.',
}

export const claimStatusLabels: Record<ClaimStatus, string> = {
  active: 'Current',
  disputed: 'Disputed',
  superseded: 'Superseded',
  rejected: 'Rejected',
}

export const claimStatusOptions: ClaimStatus[] = ['active', 'disputed', 'superseded', 'rejected']

export const hypothesisStatusLabels: Record<HypothesisStatus, string> = {
  open: 'Open',
  supported: 'Supported',
  disproven: 'Disproven',
  superseded: 'Superseded',
}

export const hypothesisStatusOptions: HypothesisStatus[] = ['open', 'supported', 'disproven', 'superseded']

export const questionStatusLabels: Record<QuestionStatus, string> = {
  open: 'Open',
  resolved: 'Resolved',
  deferred: 'Deferred',
}

export const sourceTypeLabels: Record<SourceType, string> = {
  sql_query: 'SQL query',
  code: 'Code',
  file: 'File',
  jira: 'Jira',
  // Kept for existing records, so evidence of this type still shows its true
  // origin. New evidence cannot pick it (see sourceTypeOptions).
  evernote_note: 'Evernote note',
  documentation: 'Documentation',
  api_response: 'API response',
  user_statement: 'User statement',
  ai_tool_output: 'AI tool output',
  manual_observation: 'Manual observation',
  external_url: 'External URL',
  casebook_artifact: 'Casebook artifact',
}

export const sourceTypeOptions: SourceType[] = [
  'sql_query',
  'code',
  'file',
  'jira',
  'documentation',
  'api_response',
  'user_statement',
  'ai_tool_output',
  'manual_observation',
  'external_url',
  'casebook_artifact',
]

export const relationLabels: Record<Relation, string> = {
  supports: 'Supports',
  contradicts: 'Contradicts',
}

export const actorKindLabels: Record<ActorKind, string> = {
  human: 'Human',
  ai: 'AI',
  import: 'Import',
  system: 'System',
}

export const recordTypeLabels: Record<RecordType, string> = {
  investigation: 'Case',
  evidence: 'Evidence',
  claim: 'Claim',
  hypothesis: 'Hypothesis',
  question: 'Question',
}
