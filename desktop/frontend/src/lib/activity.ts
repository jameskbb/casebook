// How we got here, in plain words. A row reads "{actor} {verb phrase}", for
// example "Claude Code added the source E-005" (the screen shows the source's
// title in place of its code). The names AI tools give themselves are engine
// data, so they are only ever rendered as text; the stored kind and name stay
// in the actor tooltip.
import type { Activity, Actor, ClaimStatus, HypothesisStatus, Verification } from '../api/types'
import { actorKindLabels, claimStatusLabels, hypothesisStatusLabels } from './labels'

const ANY_AI = 'Your AI'

const FRIENDLY_CLIENT: Record<string, string> = {
  'casebook-ui': 'Casebook',
  'casebook-cli': 'Casebook',
  'claude-code': 'Claude Code',
  codex: 'Codex',
}

function friendlyClient(client: string): string {
  return FRIENDLY_CLIENT[client] ?? client
}

/** "You", "Claude Code", "Codex", "Your AI", "Your AI (name)". */
export function actorPhrase(actor: Actor): string {
  const { kind, client } = actor
  switch (kind) {
    case 'human':
      return 'You'
    case 'ai':
      if (client === 'claude-code') return 'Claude Code'
      if (client === 'codex') return 'Codex'
      if (client === 'mcp-client' || !client) return ANY_AI
      return `${ANY_AI} (${client})`
    default:
      return 'Casebook'
  }
}

/** "{kind}, {name}", for example "AI, Claude Code". */
export function actorTooltip(actor: Actor): string {
  const kind = actorKindLabels[actor.kind] ?? 'Casebook'
  return actor.client ? `${kind}, ${friendlyClient(actor.client)}` : kind
}

// One entry per activity verb. "{T}" is the target's code. Each phrase still
// reads as a sentence with the code hidden.
const VERBS: Record<string, string> = {
  'investigation.create': 'created this question',
  'investigation.update': 'edited the question',
  'investigation.archive': 'archived this question',
  'investigation.unarchive': 'put this question back in the list',
  'conclusion.update': 'updated the answer',
  'evidence.add': 'added the source {T}',
  'evidence.update': 'updated the source {T}',
  'claim.add': 'added the finding {T}',
  'claim.update': 'updated the finding {T}',
  'claim.supersede': 'replaced the finding {T}',
  'hypothesis.add': 'added the finding {T}',
  'hypothesis.update': 'updated the finding {T}',
  'hypothesis.supersede': 'replaced the finding {T}',
  'link.add': 'added a source to the finding {T}',
  'link.update': 'changed whether a source backs the finding {T}',
  'link.remove': 'removed a source from the finding {T}',
  'question.add': 'asked a follow-up question {T}',
  'question.update': 'reworded a follow-up question {T}',
  'question.resolve': 'answered a follow-up question {T}',
  'question.defer': 'left a follow-up question for later {T}',
  'question.reopen': 'asked a follow-up question again {T}',
  'investigation.delete': 'deleted a question {T}',
  'evidence.delete': 'deleted a source {T}',
  'claim.delete': 'deleted a finding {T}',
  'hypothesis.delete': 'deleted a finding {T}',
  'question.delete': 'deleted a follow-up question {T}',
  'active.set': 'chose this question for your AI to work on',
  'active.clear': 'stopped your AI working on this question',
  checkpoint: 'saved its progress',
  import: 'added this question from a file',
  restore: 'brought back a backup',
}

/** An action this version does not know reads "made a change", never its engine name. */
const UNKNOWN_VERB = 'made a change'

/** Text, a source or finding a click opens (ref), or a code that is only shown when codes are on (id). */
export type SentenceSegment = { text: string } | { ref: string } | { id: string }

export interface ActivitySentence {
  actor: string
  tooltip: string
  /** The row came from a file: the actor is the file's, not someone here. */
  imported: boolean
  /** The verb phrase; a source or finding that can be opened is a ref segment, any other code an id segment. */
  segments: SentenceSegment[]
}

const OPENABLE_REF = /^[ECH]-\d+$/

/**
 * The actor of a row that came from a file. Nobody here did it, so a person
 * is never "You": "james, according to the file".
 */
export function importedActorPhrase(actor: Actor): string {
  const client = actor.client ? friendlyClient(actor.client) : ''
  const who = actor.kind === 'human' ? client || 'A person' : actor.kind === 'ai' ? client || 'An AI' : actorPhrase(actor)
  return `${who}, according to the file`
}

export function activitySentence(activity: Activity): ActivitySentence {
  const target = activity.target_ref
  const tpl = VERBS[activity.action] ?? UNKNOWN_VERB
  let segments: SentenceSegment[]
  // A deleted finding or source has nothing to open.
  const deleted = activity.action.endsWith('.delete')
  if (target && !deleted && OPENABLE_REF.test(target) && tpl.includes('{T}')) {
    const [before, after] = tpl.split('{T}')
    segments = [...(before ? [{ text: before }] : []), { ref: target }, ...(after ? [{ text: after }] : [])]
  } else if (target && tpl.endsWith(' {T}')) {
    segments = [{ text: tpl.slice(0, -' {T}'.length) }, { id: target }]
  } else {
    segments = [{ text: tpl.replace(/ ?\{T\}/, '') }]
  }
  const imported = activity.imported === true
  return {
    actor: imported ? importedActorPhrase(activity.actor) : actorPhrase(activity.actor),
    tooltip: imported ? `${actorTooltip(activity.actor)} (from a file)` : actorTooltip(activity.actor),
    imported,
    segments,
  }
}

// The second line of a row. The engine writes one summary for every surface
// and its wording has changed between versions, so this reads both the old
// and the new stored shapes and writes its own plain sentence. A shape it
// does not know gives no second line: the stored text is never shown as it
// is, and no sentence here carries a code that could be hidden.

/** A stored check value as a sentence about what was done. */
const CHECK_SENTENCE: Record<Verification, string> = {
  human_verified: 'Checked this.',
  source_verified: 'Compared this with its source.',
  ai_generated: 'Set this back to Not checked.',
  disputed: 'Said sources disagree.',
  superseded: 'Said a newer one replaces this.',
}

/** A stored state as a sentence about what was done. */
function stateSentence(kind: string, value: string): string {
  if (value === 'rejected' || value === 'disproven') return 'Ruled this out.'
  if (value === 'disputed') return CHECK_SENTENCE.disputed
  if (value === 'active' || value === 'open') return 'Brought this back.'
  const word = kind === 'hypothesis' ? hypothesisStatusLabels[value as HypothesisStatus] : claimStatusLabels[value as Exclude<ClaimStatus, 'active'>]
  return word ? `Now: ${word}.` : ''
}

/** The engine's field names, as a person reads them. A field not listed (how sure the AI is) is never shown. */
const FIELD_WORD: Record<string, string> = {
  title: 'the question',
  problem_statement: 'the background',
  statement: 'the wording',
  rationale: 'why',
  confidence_rationale: 'why',
  summary: 'what it shows',
  source_type: 'the kind of source',
  source_label: 'where it came from',
  source_ref: 'where to find it',
  captured_at: 'its date',
  raw_detail: 'the exact words or numbers',
  metadata: 'extra details',
}

const NEEDS_CHECKING_AGAIN = 'It changed after it was checked, so it needs checking again.'
const RESET_NOTE = /; \w+ reset to \w+ after \w+ edit$/

/** "statement, status rejected" as sentences: what was decided, then which parts changed. */
function changeSentences(kind: string, items: string[]): string {
  const out: string[] = []
  const fields: string[] = []
  let state = ''
  let check = ''
  for (const item of items) {
    const [field, value] = item.split(' ', 2)
    if (field === 'status' && value) state = stateSentence(kind, value)
    else if (field === 'verification' && value) check = CHECK_SENTENCE[value as Verification] ?? ''
    else {
      // A source's title is its name; a question's title is the question.
      const word = field === 'title' && kind === 'evidence' ? 'its name' : FIELD_WORD[field]
      if (word && !fields.includes(word)) fields.push(word)
    }
  }
  // A person's own check says the most; otherwise ruling out or bringing back
  // says more than the check value that rides along with it.
  if (check === CHECK_SENTENCE.human_verified || !state) out.push(check)
  else out.push(state)
  if (fields.length) out.push(`Changed ${fields.join(', ')}.`)
  return out.filter(Boolean).join(' ')
}

export interface SummaryContext {
  /** The question's conclusion_reviewed_at: the answer row written at that instant is the person's check. */
  conclusionReviewedAt?: string
}

/** The second line for an activity row; "" when the first line already says it or the shape is not known. */
export function activitySummary(activity: Activity, ctx: SummaryContext = {}): string {
  let sum = activity.summary.trim()
  const action = activity.action
  const kind = action.split('.')[0]
  // A note an AI wrote for the next reader, not engine text.
  if (action === 'checkpoint') return sum
  // An AI edit to something checked: the stored text names only the new value.
  const reset = RESET_NOTE.test(sum)
  if (reset) sum = sum.replace(RESET_NOTE, '')
  const again = (s: string) => (reset ? [s, NEEDS_CHECKING_AGAIN].filter(Boolean).join(' ') : s)

  if (action === 'conclusion.update') {
    if (reset) return NEEDS_CHECKING_AGAIN
    if (/^Cleared /.test(sum)) return 'Cleared the answer.'
    const checked = Boolean(ctx.conclusionReviewedAt) && activity.created_at === ctx.conclusionReviewedAt && activity.actor.kind === 'human'
    return checked ? 'Checked the answer.' : ''
  }
  let m = /^Set \w+ of \S+ to ([a-z_]+)(?:; status ([a-z_]+))?$/.exec(sum)
  if (m) return changeSentences(kind, [`verification ${m[1]}`, ...(m[2] ? [`status ${m[2]}`] : [])])
  if (kind === 'claim' || kind === 'hypothesis' || kind === 'evidence' || action === 'investigation.update') {
    m = /^Updated \w+ \S+: (.+)$/.exec(sum)
    if (m) return again(changeSentences(kind, m[1].split(', ')))
  }
  // The words of a follow-up question, or of its answer.
  if (action === 'question.add' || action === 'question.update' || action === 'question.resolve') {
    m = /^\w+ question \S+: (.+)$/.exec(sum)
    if (m) return m[1]
  }
  // What was deleted has no other name left on the page.
  if (action.endsWith('.delete')) {
    m = /^Deleted \w+ \S+ (".*?")/.exec(sum)
    if (m) return m[1]
  }
  if (action === 'restore') {
    m = /^Restored backup (.+?) \(/.exec(sum)
    if (m) return m[1]
  }
  return ''
}
