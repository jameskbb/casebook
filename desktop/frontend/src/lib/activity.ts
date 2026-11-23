// How we got here, in plain words. A row reads "{actor} {verb phrase}", for
// example "Claude Code added the source E-005" (the screen shows the source's
// title in place of its code). The names AI tools give themselves are engine
// data, so they are only ever rendered as text; the stored kind and name stay
// in the actor tooltip.
import type { Activity, Actor, ClaimStatus, HypothesisStatus, Verification } from '../api/types'
import { actorKindLabels, claimStatusLabels, hypothesisStatusLabels, verificationLabels } from './labels'

const ANY_AI = 'Your AI'

const FRIENDLY_CLIENT: Record<string, string> = {
  'casebook-ui': 'Casebook',
  'casebook-cli': 'Casebook command line',
  'claude-code': 'Claude Code',
  codex: 'Codex',
}

function friendlyClient(client: string): string {
  return FRIENDLY_CLIENT[client] ?? client
}

/** "You", "Claude Code", "Codex", "Your AI", ... */
export function actorPhrase(actor: Actor): string {
  const { kind, client } = actor
  switch (kind) {
    case 'human':
      if (client === 'casebook-ui' || !client) return 'You'
      if (client === 'casebook-cli') return 'You (command line)'
      return `You (${client})`
    case 'ai':
      if (client === 'claude-code') return 'Claude Code'
      if (client === 'codex') return 'Codex'
      if (client === 'mcp-client' || !client) return ANY_AI
      return `${client} (AI)`
    case 'import':
    case 'system':
      return 'Casebook'
    default:
      return client ? `${client} (${kind})` : String(kind)
  }
}

/** "{kind label}, {client}", for example "AI, claude-code". */
export function actorTooltip(actor: Actor): string {
  const kind = actorKindLabels[actor.kind] ?? actor.kind
  return actor.client ? `${kind}, ${actor.client}` : kind
}

// One entry per activity verb. "{T}" is the target's code. Each phrase still
// reads as a sentence with the code hidden.
const VERBS: Record<string, string> = {
  'investigation.create': 'created this question',
  'investigation.update': 'edited the question',
  'investigation.archive': 'archived this question',
  'investigation.unarchive': 'unarchived this question',
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
  'link.update': 'changed whether a source supports the finding {T}',
  'link.remove': 'removed a source from the finding {T}',
  'question.add': 'asked a follow-up question {T}',
  'question.update': 'reworded a follow-up question {T}',
  'question.resolve': 'answered a follow-up question {T}',
  'question.defer': 'set aside a follow-up question {T}',
  'question.reopen': 'reopened a follow-up question {T}',
  'investigation.delete': 'deleted a question {T}',
  'evidence.delete': 'deleted a source {T}',
  'claim.delete': 'deleted a finding {T}',
  'hypothesis.delete': 'deleted a finding {T}',
  'question.delete': 'deleted a follow-up question {T}',
  'active.set': "made this your AI's current question",
  'active.clear': "cleared your AI's current question",
  checkpoint: 'saved its progress',
  import: 'added this question from a file',
  restore: 'restored all data from a backup',
}

function template(action: string, targetRef: string): string {
  const known = VERBS[action]
  if (known !== undefined) return known
  return targetRef ? `recorded ${action} on {T}` : `recorded ${action}`
}

function fill(tpl: string, targetRef: string): string {
  return targetRef ? tpl.replace('{T}', targetRef) : tpl.replace(/ ?\{T\}/, '')
}

/** "added the source E-005", "answered a follow-up question Q-001", "recorded x.y on T". */
export function verbPhrase(action: string, targetRef: string): string {
  return fill(template(action, targetRef), targetRef)
}

/** Text, a record a click opens (ref), or a code that is only shown when codes are on (id). */
export type SentenceSegment = { text: string } | { ref: string } | { id: string }

export interface ActivitySentence {
  actor: string
  tooltip: string
  /** The row came from an imported file: the actor is the file's, not someone here. */
  imported: boolean
  /** The verb phrase; a source or finding that can be opened is a ref segment, any other code an id segment. */
  segments: SentenceSegment[]
}

const OPENABLE_REF = /^[ECH]-\d+$/

/**
 * The actor of a row that came from a file. Nobody here did it, so a person
 * is never "You": "james (a person, the file says)".
 */
export function importedActorPhrase(actor: Actor): string {
  const client = actor.client ? friendlyClient(actor.client) : ''
  switch (actor.kind) {
    case 'human':
      return client ? `${client} (a person, the file says)` : 'A person (the file says)'
    case 'ai':
      return `${client || 'An AI'} (AI, the file says)`
    default:
      return `${actorPhrase(actor)} (the file says)`
  }
}

export function activitySentence(activity: Activity): ActivitySentence {
  const target = activity.target_ref
  const tpl = template(activity.action, target)
  let segments: SentenceSegment[]
  // A deleted record has no drawer to open.
  const deleted = activity.action.endsWith('.delete')
  if (target && !deleted && OPENABLE_REF.test(target) && tpl.includes('{T}')) {
    const [before, after] = tpl.split('{T}')
    segments = [...(before ? [{ text: before }] : []), { ref: target }, ...(after ? [{ text: after }] : [])]
  } else if (target && tpl.endsWith(' {T}')) {
    segments = [{ text: tpl.slice(0, -' {T}'.length) }, { id: target }]
  } else {
    segments = [{ text: fill(tpl, target) }]
  }
  const imported = activity.imported === true
  return {
    actor: imported ? importedActorPhrase(activity.actor) : actorPhrase(activity.actor),
    tooltip: imported ? `${actorTooltip(activity.actor)} (from a file)` : actorTooltip(activity.actor),
    imported,
    segments,
  }
}

// The second line of a row. The engine writes one summary for every surface,
// so it keeps its own wording; this rewrites the shapes a person meets most
// often into plain sentences with no stored values and no engine nouns. Any
// other summary is shown as stored. A line that only repeats the first one
// comes back empty. The stored summary stays in the row's title attribute.

const RESET_NOTE = /; verification reset to ai_generated after \w+ edit$/

function verificationWord(v: string): string {
  return verificationLabels[v as Verification] ?? v
}

function statusWord(rec: string, v: string): string {
  if (rec === 'claim') return claimStatusLabels[v as ClaimStatus] ?? v
  if (rec === 'hypothesis') return hypothesisStatusLabels[v as HypothesisStatus] ?? v
  return v
}

/** The engine's record noun, as a person reads it. */
const RECORD_WORD: Record<string, string> = { claim: 'finding', hypothesis: 'finding', evidence: 'source' }

/** The engine's field names, as a person reads them. */
const FIELD_WORD: Record<string, string> = {
  statement: 'wording',
  rationale: 'why',
  confidence_rationale: 'why',
  source_type: 'kind of source',
  source_label: 'where it came from',
  source_ref: 'where to find it',
  captured_at: 'as of date',
  raw_detail: 'source detail',
  metadata: 'extra details',
  superseded_by: 'replaced by',
  problem_statement: 'background',
}

/** "statement", "status rejected" -> "wording", "now Ruled out". */
function fieldWord(rec: string, item: string): string {
  const [field, value] = item.split(' ', 2)
  if (value === undefined) return FIELD_WORD[field] ?? field.replace(/_/g, ' ')
  if (field === 'status') return `now ${statusWord(rec, value)}`
  if (field === 'verification') return `marked ${verificationWord(value)}`
  return `${FIELD_WORD[field] ?? field.replace(/_/g, ' ')} ${value}`
}

/** The edited fields in plain words. Confidence is stored but never shown. */
function fieldWords(rec: string, items: string[]): string[] {
  return items.filter((f) => !f.startsWith('confidence')).map((f) => fieldWord(rec, f))
}

/** "Edited finding C-002 (wording)", or without the list when nothing shown changed. */
function editedSentence(rec: string, target: string, items: string[]): string {
  const fields = fieldWords(rec, items)
  const noun = RECORD_WORD[rec] ?? rec
  return fields.length ? `Edited ${noun} ${target} (${fields.join(', ')})` : `Edited ${noun} ${target}`
}

/** "Marked C-002: You checked this", with the new status when it reads differently. */
function markedSentence(rec: string, target: string, verification: string, status?: string): string {
  const label = verificationWord(verification)
  const statusText = status && statusWord(rec, status)
  return statusText && statusText !== label && statusText !== 'Current' ? `Marked ${target}: ${label}; now ${statusText}` : `Marked ${target}: ${label}`
}

export interface SummaryContext {
  /** The question's conclusion_reviewed_at: the answer row written at that instant is the review. */
  conclusionReviewedAt?: string
}

/** The people-facing second line for an activity row; the stored summary when no rule applies. */
export function activitySummary(activity: Activity, ctx: SummaryContext = {}): string {
  const sum = activity.summary.trim()
  // An AI edit to a checked record: the summary names only the new value, so
  // the sentence does not claim which label was lost.
  const reset = RESET_NOTE.exec(sum)
  if (reset) {
    const m = /^Updated (claim|hypothesis|evidence) (\S+): (.*)$/.exec(sum.slice(0, reset.index))
    if (m) {
      const [, rec, target, list] = m
      const edited = editedSentence(rec, target, list.split(', ').filter((f) => f !== 'verification'))
      const back = rec === 'evidence' ? '' : ' and is back in Review findings'
      return `${edited}. It changed after it was checked, so it reads Not checked again${back}.`
    }
  }
  let m = /^Set verification of (\S+) to ([a-z_]+)(?:; status ([a-z_]+))?$/.exec(sum)
  if (m) {
    const rec = activity.action.split('.')[0]
    return markedSentence(rec, m[1], m[2], m[3])
  }
  // An edit with no reset, such as I checked this from Review findings
  // ("Updated claim C-002: verification human_verified").
  m = /^Updated (claim|hypothesis|evidence) (\S+): (.+)$/.exec(sum)
  if (m) {
    const [, rec, target, list] = m
    const items = list.split(', ')
    const verification = items.find((f) => f.startsWith('verification '))
    const status = items.find((f) => f.startsWith('status '))
    const others = items.filter((f) => f !== verification && f !== status)
    if (verification && others.length === 0) {
      return markedSentence(rec, target, verification.slice('verification '.length), status?.slice('status '.length))
    }
    return editedSentence(rec, target, items)
  }
  // The first line already says it.
  if (/^Set active investigation to \S+$/.test(sum) || /^Cleared active investigation \(was \S+\)$/.test(sum)) return ''
  m = /^Made \S+ the active investigation for (.+)$/.exec(sum)
  if (m) return `For work in ${m[1]}`
  m = /^Created investigation (\S+) /.exec(sum)
  if (m) return m[1] === activity.investigation_ref ? '' : `Created question ${m[1]}`
  m = /^Updated conclusion of \S+ \(confidence [a-z]+\)$/.exec(sum)
  if (m && activity.action === 'conclusion.update') {
    const review = Boolean(ctx.conclusionReviewedAt) && activity.created_at === ctx.conclusionReviewedAt && activity.actor.kind === 'human'
    return review ? 'Reviewed the answer' : ''
  }
  return sum
}
