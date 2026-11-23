// The words a person reads for what the engine stores (docs/api.md section
// 9). The screen says question, answer, finding and source, and one verb,
// check. A row shows one label; LEGEND lists every one with its meaning.
// Confidence is stored but never shown.
import type { ActorKind, Confidence, Relation, SourceType, Verification } from '../api/types'

/** The stored confidence values. Edits pass the stored value through unchanged. */
export const confidenceOptions: Confidence[] = ['unknown', 'low', 'medium', 'high']

/** The one label a source shows: who has checked it. */
export const SOURCE_NOT_CHECKED = 'Not checked'
export const SOURCE_AI_CHECKED = 'AI checked the source'
export const SOURCE_YOU_CHECKED = 'You checked this'
export const REPLACED = 'Replaced'

/** A source an AI stored as "disputed" reads Not checked: no person has checked it. */
export function sourceLabel(v: Verification): string {
  if (v === 'human_verified') return SOURCE_YOU_CHECKED
  if (v === 'source_verified') return SOURCE_AI_CHECKED
  if (v === 'superseded') return REPLACED
  return SOURCE_NOT_CHECKED
}

// The one label a finding shows (lib/records findingLabel picks it).
export const SOURCES_DISAGREE = 'Sources disagree'
export const DECIDED_HOLDS = 'You decided this holds'
export const NO_SOURCE_YET = 'No source yet'
export const NOT_TESTED = 'Not tested yet'
export const RULED_OUT = 'Ruled out'

// The other words that sit on a row or in the header.
export const LATER = 'Later'
export const FROM_A_FILE = 'From a file'
export const ANSWERED = 'Answered'
export const NO_ANSWER_YET = 'No answer yet'
export const ARCHIVED = 'Archived'
export const AI_WORKING = 'Your AI is working on this'

/** "2 of 3 sources checked by you". */
export function sourcesCheckedLabel(checked: number, total: number): string {
  return `${checked} of ${total} ${total === 1 ? 'source' : 'sources'} checked by you`
}

/** The choice in Add finding and Add a source to this finding. */
export const relationLabels: Record<Relation, string> = {
  supports: 'Backs it up',
  contradicts: 'Goes against it',
}

/** The tag on a source under a finding. */
export const relationTags: Record<Relation, string> = {
  supports: 'Backs it',
  contradicts: 'Goes against it',
}

/** Every label the page can show, in the words used, one line each. */
export const LEGEND: { heading: string; rows: { label: string; meaning: string }[] }[] = [
  {
    heading: 'On a finding',
    rows: [
      { label: sourcesCheckedLabel(2, 3), meaning: 'How many of its sources you have checked yourself.' },
      { label: NO_SOURCE_YET, meaning: 'No source is saved for this finding.' },
      { label: SOURCES_DISAGREE, meaning: 'A source goes against this finding. It stays that way until you decide.' },
      { label: DECIDED_HOLDS, meaning: 'A source goes against this finding. You read the sources and decided it is still right.' },
      { label: NOT_TESTED, meaning: 'Nobody has shown yet whether it is right or wrong.' },
      { label: RULED_OUT, meaning: 'It turned out to be wrong. It is kept so you can see what was tried.' },
      { label: REPLACED, meaning: 'A newer one took its place.' },
    ],
  },
  {
    heading: 'On a source',
    rows: [
      { label: SOURCE_NOT_CHECKED, meaning: 'Nobody has checked this source yet.' },
      { label: SOURCE_AI_CHECKED, meaning: 'Your AI went back to this source. You have not checked it yourself.' },
      { label: SOURCE_YOU_CHECKED, meaning: 'You looked at this source yourself.' },
      { label: relationTags.supports, meaning: 'This source backs the finding it sits under.' },
      { label: relationTags.contradicts, meaning: 'This source goes against the finding it sits under.' },
    ],
  },
  {
    heading: 'Elsewhere',
    rows: [
      { label: ANSWERED, meaning: 'The question has an answer.' },
      { label: NO_ANSWER_YET, meaning: 'Nobody has written the answer.' },
      { label: LATER, meaning: 'Something to find out that was left for later.' },
      { label: AI_WORKING, meaning: 'Your AI saves what it finds under this question.' },
      { label: ARCHIVED, meaning: 'The question is hidden from the list. Nothing was deleted.' },
      { label: FROM_A_FILE, meaning: 'It came in a file someone sent. Casebook shows what the file says.' },
    ],
  },
]

/** A stored kind of source as one word. Shown in the source panel only when codes are on. */
export const sourceKindWord: Record<SourceType, string> = {
  sql_query: 'Numbers',
  code: 'File',
  file: 'File',
  jira: 'Document',
  evernote_note: 'Note',
  documentation: 'Document',
  api_response: 'Numbers',
  user_statement: 'Person',
  ai_tool_output: 'AI',
  manual_observation: 'Seen',
  external_url: 'Web',
  casebook_artifact: 'Question',
}

export const actorKindLabels: Record<ActorKind, string> = {
  human: 'Person',
  ai: 'AI',
  import: 'File',
  system: 'Casebook',
}
