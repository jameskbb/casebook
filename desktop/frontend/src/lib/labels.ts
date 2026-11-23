// Display labels for the engine enums (docs/api.md section 9) and ordered
// option lists for pickers. The screen says question, answer, finding and
// source, and one verb, check. Confidence is stored but never shown.
import type {
  ActorKind,
  ClaimStatus,
  Confidence,
  HypothesisStatus,
  QuestionStatus,
  RecordType,
  Relation,
  SourceType,
  Verification,
} from '../api/types'

/** The stored confidence values. Edits pass the stored value through unchanged. */
export const confidenceOptions: Confidence[] = ['unknown', 'low', 'medium', 'high']

export const verificationLabels: Record<Verification, string> = {
  ai_generated: 'Not checked',
  human_verified: 'You checked this',
  source_verified: 'AI checked the source',
  disputed: 'Sources disagree',
  superseded: 'Replaced',
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
  ai_generated: 'Nobody has checked this version against its source.',
  source_verified: 'Your AI compared this with its source. You have not checked it yourself.',
  human_verified: 'A person checked this against its source.',
  disputed: 'A source contradicts this. It stays here until you decide.',
  superseded: 'A newer record replaced this one. It is kept for the history.',
}

/** Status words used in the legend that are not verification values. */
export const statusHelp = {
  open: 'A finding that has not been tested against a source.',
  supported: 'The sources so far back this finding. It is not settled.',
  ruledOut: 'A source ruled this out, or it was withdrawn. It stays visible.',
  current: 'A finding the answer stands on now.',
}

export const claimStatusLabels: Record<ClaimStatus, string> = {
  active: 'Current',
  disputed: 'Sources disagree',
  superseded: 'Replaced',
  rejected: 'Ruled out',
}

export const claimStatusOptions: ClaimStatus[] = ['active', 'disputed', 'superseded', 'rejected']

export const hypothesisStatusLabels: Record<HypothesisStatus, string> = {
  open: 'Not tested yet',
  supported: 'Supported',
  disproven: 'Ruled out',
  superseded: 'Replaced',
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
  // Kept for existing records, so a source of this type still shows its true
  // origin. A new source cannot pick it (see sourceTypeOptions).
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
  investigation: 'Question',
  evidence: 'Source',
  claim: 'Finding',
  hypothesis: 'Finding',
  question: 'Follow-up question',
}
