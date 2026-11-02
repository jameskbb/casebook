// Plain-language History (D-23, UI-23; UI-SPEC Activity phrasing). A row
// reads "{actor} {verb phrase}", for example "Claude Code (AI) added evidence
// E-005". Client names are engine data from any surface (an MCP client picks
// its own), so they are only ever rendered as text; the raw kind and client
// stay inspectable in the actor tooltip.
import type { Activity, Actor, ClaimStatus, Confidence, HypothesisStatus, Verification } from '../api/types'
import { actorKindLabels, claimStatusLabels, confidenceLabels, hypothesisStatusLabels, verificationLabels } from './labels'

const ANY_AI_CLIENT = 'An AI client (AI)'

const FRIENDLY_CLIENT: Record<string, string> = {
  'casebook-ui': 'Casebook',
  'casebook-cli': 'Casebook CLI',
  'claude-code': 'Claude Code',
  codex: 'Codex',
}

function friendlyClient(client: string): string {
  return FRIENDLY_CLIENT[client] ?? client
}

/** "You (Casebook)", "Claude Code (AI)", "Codex (AI)", ... */
export function actorPhrase(actor: Actor): string {
  const { kind, client } = actor
  switch (kind) {
    case 'human':
      if (client === 'casebook-ui') return 'You (Casebook)'
      if (client === 'casebook-cli') return 'You (Casebook CLI)'
      return client ? `You (${client})` : 'You'
    case 'ai':
      if (client === 'claude-code') return 'Claude Code (AI)'
      if (client === 'codex') return 'Codex (AI)'
      if (client === 'mcp-client' || !client) return ANY_AI_CLIENT
      return `${client} (AI)`
    case 'import':
      return client ? `Import (${friendlyClient(client)})` : 'Import'
    case 'system':
      return 'Casebook (system)'
    default:
      return client ? `${client} (${kind})` : String(kind)
  }
}

/** "{kind label}, {client}", for example "AI, claude-code". */
export function actorTooltip(actor: Actor): string {
  const kind = actorKindLabels[actor.kind] ?? actor.kind
  return actor.client ? `${kind}, ${actor.client}` : kind
}

// One entry per row of the UI-SPEC verb table. "{T}" is the target ref.
const VERBS: Record<string, string> = {
  'investigation.create': 'created this case',
  'investigation.update': 'edited the case details',
  'investigation.archive': 'archived this case',
  'investigation.unarchive': 'unarchived this case',
  'conclusion.update': 'updated the conclusion',
  'evidence.add': 'added evidence {T}',
  'evidence.update': 'updated evidence {T}',
  'claim.add': 'added claim {T}',
  'claim.update': 'updated claim {T}',
  'claim.supersede': 'superseded claim {T}',
  'hypothesis.add': 'added hypothesis {T}',
  'hypothesis.update': 'updated hypothesis {T}',
  'hypothesis.supersede': 'superseded hypothesis {T}',
  'link.add': 'linked evidence to {T}',
  'link.update': 'changed how evidence relates to {T}',
  'link.remove': 'unlinked evidence from {T}',
  'question.add': 'asked {T}',
  'question.update': 'reworded {T}',
  'question.resolve': 'resolved {T}',
  'question.defer': 'deferred {T}',
  'question.reopen': 'reopened {T}',
  'investigation.delete': 'deleted case {T}',
  'evidence.delete': 'deleted evidence {T}',
  'claim.delete': 'deleted claim {T}',
  'hypothesis.delete': 'deleted hypothesis {T}',
  'question.delete': 'deleted {T}',
  'active.set': "made this your AI's current case",
  'active.clear': "cleared your AI's current case",
  checkpoint: 'saved a checkpoint',
  import: 'imported this case',
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

/** "added evidence E-005", "resolved Q-001", "recorded x.y on T". */
export function verbPhrase(action: string, targetRef: string): string {
  return fill(template(action, targetRef), targetRef)
}

export type SentenceSegment = { text: string } | { ref: string }

export interface ActivitySentence {
  actor: string
  tooltip: string
  /** The row came from an imported file: the actor is the file's, not someone here. */
  imported: boolean
  /** The verb phrase; E, C and H refs are ref segments (they open drawers). */
  segments: SentenceSegment[]
}

const OPENABLE_REF = /^[ECH]-\d+$/

/**
 * The actor of a History row copied from an imported file. Nobody here did
 * it, so a human is never "You": "james (person in the imported file)".
 */
export function importedActorPhrase(actor: Actor): string {
  const client = actor.client ? friendlyClient(actor.client) : ''
  switch (actor.kind) {
    case 'human':
      return client ? `${client} (person, per the imported file)` : 'A person (per the imported file)'
    case 'ai':
      return `${client || 'An AI client'} (AI, per the imported file)`
    default:
      return `${actorPhrase(actor)}, per the imported file`
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
  } else {
    segments = [{ text: fill(tpl, target) }]
  }
  const imported = activity.imported === true
  return {
    actor: imported ? importedActorPhrase(activity.actor) : actorPhrase(activity.actor),
    tooltip: imported ? `${actorTooltip(activity.actor)} (from an imported file)` : actorTooltip(activity.actor),
    imported,
    segments,
  }
}

// The second line of a History row. The engine writes one summary for every
// surface (CLI, MCP, report timeline), so it keeps its own wording; History
// rewrites the shapes a person meets most often into plain sentences with no
// enum values and no "investigation". Any other summary is shown as stored.
// The raw summary stays in the row's title attribute.

const RESET_NOTE = /; verification reset to ai_generated after \w+ edit$/

function verificationWord(v: string): string {
  return verificationLabels[v as Verification] ?? v
}

function statusWord(rec: string, v: string): string {
  if (rec === 'claim') return claimStatusLabels[v as ClaimStatus] ?? v
  if (rec === 'hypothesis') return hypothesisStatusLabels[v as HypothesisStatus] ?? v
  return v
}

/** "statement", "confidence high" -> "statement", "confidence High". */
function fieldWord(rec: string, item: string): string {
  const [field, value] = item.split(' ', 2)
  if (value === undefined) return field.replace(/_/g, ' ')
  if (field === 'confidence') return `confidence ${confidenceLabels[value as Confidence] ?? value}`
  if (field === 'status') return `status ${statusWord(rec, value)}`
  if (field === 'verification') return `verification ${verificationWord(value)}`
  return `${field.replace(/_/g, ' ')} ${value}`
}

/** "Marked C-002 Human verified", with the new status when it reads differently. */
function markedSentence(rec: string, target: string, verification: string, status?: string): string {
  const label = verificationWord(verification)
  const statusText = status && statusWord(rec, status)
  return statusText && statusText !== label ? `Marked ${target} ${label}; status now ${statusText}` : `Marked ${target} ${label}`
}

export interface SummaryContext {
  /** The case's conclusion_reviewed_at: the conclusion row written at that instant is the review. */
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
      const fields = list.split(', ').filter((f) => f !== 'verification').map((f) => fieldWord(rec, f))
      const edited = fields.length ? `Edited ${rec} ${target} (${fields.join(', ')}).` : `Edited ${rec} ${target}.`
      const back = rec === 'evidence' ? '' : '; it is back in Review findings'
      return `${edited} Its verification was reset to AI generated after the edit${back}.`
    }
  }
  let m = /^Set verification of (\S+) to ([a-z_]+)(?:; status ([a-z_]+))?$/.exec(sum)
  if (m) {
    const rec = activity.action.split('.')[0]
    return markedSentence(rec, m[1], m[2], m[3])
  }
  // An edit with no reset, such as Verify myself from Review findings
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
    return `Edited ${rec} ${target} (${items.map((f) => fieldWord(rec, f)).join(', ')})`
  }
  m = /^Set active investigation to (\S+)$/.exec(sum)
  if (m) return `Made ${m[1]} your AI's current case`
  m = /^Made (\S+) the active investigation for (.+)$/.exec(sum)
  if (m) return `Made ${m[1]} your AI's current case for ${m[2]}`
  m = /^Cleared active investigation \(was (\S+)\)$/.exec(sum)
  if (m) return `Cleared your AI's current case (was ${m[1]})`
  m = /^Created investigation (\S+) /.exec(sum)
  if (m) return m[1] === activity.investigation_ref ? 'Created this case' : `Created case ${m[1]}`
  m = /^Updated conclusion of \S+ \(confidence ([a-z]+)\)$/.exec(sum)
  if (m && activity.action === 'conclusion.update') {
    const conf = m[1] === 'unknown' ? 'confidence not assessed' : `${confidenceLabels[m[1] as Confidence] ?? m[1]} confidence`
    const review = Boolean(ctx.conclusionReviewedAt) && activity.created_at === ctx.conclusionReviewedAt && activity.actor.kind === 'human'
    return review ? `Recorded a review of the conclusion (${conf})` : `Updated the conclusion (${conf})`
  }
  return sum
}
