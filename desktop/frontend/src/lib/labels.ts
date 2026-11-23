// Display labels for the engine enums (docs/api.md section 9) and ordered
// option lists for pickers. The screen says question, answer, finding and
// source, and one verb, check. Confidence is stored but never shown.
import type { ActorKind, ClaimStatus, Confidence, HypothesisStatus, Relation, SourceType, Verification } from '../api/types'

/** The stored confidence values. Edits pass the stored value through unchanged. */
export const confidenceOptions: Confidence[] = ['unknown', 'low', 'medium', 'high']

export const verificationLabels: Record<Verification, string> = {
  ai_generated: 'Not checked',
  human_verified: 'You checked this',
  source_verified: 'AI checked the source',
  disputed: 'Sources disagree',
  superseded: 'Replaced',
}

/** One sentence per check label, for tooltips and the legend. */
export const verificationHelp: Record<Verification, string> = {
  ai_generated: 'Nobody has checked this yet.',
  source_verified: 'Your AI compared this with its source. You have not checked it yourself.',
  human_verified: 'You checked this against its source.',
  disputed: 'A source says something different. It stays here until you decide.',
  superseded: 'A newer one took its place.',
}

// A claim whose stored state is "active" shows no state word at all.
export const claimStatusLabels: Record<Exclude<ClaimStatus, 'active'>, string> = {
  disputed: 'Sources disagree',
  superseded: 'Replaced',
  rejected: 'Ruled out',
}

export const hypothesisStatusLabels: Record<HypothesisStatus, string> = {
  open: 'Not tested yet',
  supported: 'Looks right so far',
  disproven: 'Ruled out',
  superseded: 'Replaced',
}

// Every stored kind has a plain label, so a source an AI saved still reads
// well. A person picks from the six in sourceTypeOptions.
export const sourceTypeLabels: Record<SourceType, string> = {
  sql_query: 'Numbers your AI looked up',
  code: 'Something your AI read',
  file: 'Report or spreadsheet',
  jira: 'Document or email',
  evernote_note: 'Document or email',
  documentation: 'Document or email',
  api_response: 'Numbers your AI looked up',
  user_statement: 'Something a person said',
  ai_tool_output: 'Something your AI found',
  manual_observation: 'Something you saw',
  external_url: 'Web page',
  casebook_artifact: 'Another question',
}

/** The kinds a person can choose, in the order shown. The engine has no kind for "something else". */
export const sourceTypeOptions: SourceType[] = ['file', 'documentation', 'external_url', 'user_statement', 'manual_observation', 'ai_tool_output']

export const relationLabels: Record<Relation, string> = {
  supports: 'Backs it up',
  contradicts: 'Goes against it',
}

export const actorKindLabels: Record<ActorKind, string> = {
  human: 'Person',
  ai: 'AI',
  import: 'File',
  system: 'Casebook',
}
