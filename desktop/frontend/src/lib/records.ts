// Pure helpers over an InvestigationDetail for the reading column. Records
// arrive ordered by ref (docs/api.md section 8) and keep that order here.
//
// A person checks sources, so a finding's label is worked out from its
// sources (findingLabel), never printed from its stored check value.
import type {
  Claim,
  Evidence,
  Hypothesis,
  InvestigationDetail,
  Question,
  Relation,
} from '../api/types'
import { DECIDED_HOLDS, NO_SOURCE_YET, NOT_TESTED, REPLACED, RULED_OUT, SOURCES_DISAGREE, sourcesCheckedLabel } from './labels'

/** Records that always show, and records collapsed under a disclosure. */
export interface Partition<T> {
  visible: T[]
  ruledOut: T[]
}

function split<T>(list: T[], ruledOut: (x: T) => boolean): Partition<T> {
  const p: Partition<T> = { visible: [], ruledOut: [] }
  for (const x of list) (ruledOut(x) ? p.ruledOut : p.visible).push(x)
  return p
}

/**
 * Open ones show, then the ones left for later (each in the order they
 * arrived): one left for later is still unanswered. Only answered ones collapse.
 */
export function partitionQuestions(questions: Question[]): Partition<Question> {
  const p = split(questions, (q) => q.status === 'resolved')
  p.visible = [...p.visible.filter((q) => q.status === 'open'), ...p.visible.filter((q) => q.status !== 'open')]
  return p
}

/** A claim or a hypothesis: on screen both are a finding. */
export type Finding = { kind: 'claim'; record: Claim } | { kind: 'hypothesis'; record: Hypothesis }

/** The question's sources by code. */
export type Sources = Map<string, Evidence>

export function evidenceByRef(detail: Pick<InvestigationDetail, 'evidence'>): Sources {
  return new Map(detail.evidence.map((e) => [e.ref, e]))
}

type FindingRecord = Finding['record']

export const isReplaced = (r: FindingRecord) => r.status === 'superseded'
export const isRuledOut = (r: FindingRecord) => r.status === 'rejected' || r.status === 'disproven'

/** The finding's sources that have not been replaced. A source the page does not know still counts. */
function liveLinks(r: FindingRecord, sources: Sources) {
  return r.links.filter((l) => sources.get(l.evidence_ref)?.verification !== 'superseded')
}

const hasLiveAgainst = (r: FindingRecord, sources: Sources) => liveLinks(r, sources).some((l) => l.relation === 'contradicts')

/**
 * Sources disagree about a finding that is not replaced and not ruled out
 * when it is stored that way, or when a source that was not replaced goes
 * against it and the person has not decided it still holds. The list count
 * and the shared answer use the same rule.
 */
export function sourcesDisagree(r: FindingRecord, sources: Sources): boolean {
  if (isReplaced(r) || isRuledOut(r)) return false
  if (r.verification === 'disputed' || r.status === 'disputed') return true
  return r.verification !== 'human_verified' && hasLiveAgainst(r, sources)
}

/** How a finding's label is drawn: color and icon never carry the meaning alone. */
export type FindingTone = 'checked' | 'disagree' | 'plain' | 'retired'

/** The one label a finding shows, and how it is drawn. */
export function findingMark(f: Finding, sources: Sources): { label: string; tone: FindingTone } {
  const r = f.record
  if (isReplaced(r)) return { label: REPLACED, tone: 'retired' }
  if (isRuledOut(r)) return { label: RULED_OUT, tone: 'retired' }
  if (sourcesDisagree(r, sources)) return { label: SOURCES_DISAGREE, tone: 'disagree' }
  if (r.verification === 'human_verified' && hasLiveAgainst(r, sources)) return { label: DECIDED_HOLDS, tone: 'checked' }
  if (f.kind === 'hypothesis' && r.status === 'open') return { label: NOT_TESTED, tone: 'plain' }
  const live = liveLinks(r, sources)
  if (live.length === 0) return { label: NO_SOURCE_YET, tone: 'plain' }
  const checked = live.filter((l) => sources.get(l.evidence_ref)?.verification === 'human_verified').length
  return { label: sourcesCheckedLabel(checked, live.length), tone: checked === live.length ? 'checked' : 'plain' }
}

export const findingLabel = (f: Finding, sources: Sources): string => findingMark(f, sources).label

/** Findings sorted by what happened to them, claims before hypotheses, each in ref order. */
export interface FindingGroups {
  /** What the answer can stand on. */
  backing: Finding[]
  /** Sources disagree and nobody has decided yet. */
  disagree: Finding[]
  /** Findings nobody has shown right or wrong. */
  notTested: Finding[]
  ruledOut: Finding[]
  replaced: Finding[]
}

export function groupFindings(claims: Claim[], hypotheses: Hypothesis[], sources: Sources): FindingGroups {
  const g: FindingGroups = { backing: [], disagree: [], notTested: [], ruledOut: [], replaced: [] }
  const all: Finding[] = [...claims.map((record) => ({ kind: 'claim' as const, record })), ...hypotheses.map((record) => ({ kind: 'hypothesis' as const, record }))]
  for (const f of all) {
    const r = f.record
    const group = isReplaced(r)
      ? g.replaced
      : isRuledOut(r)
        ? g.ruledOut
        : sourcesDisagree(r, sources)
          ? g.disagree
          : f.kind === 'hypothesis' && r.status === 'open'
            ? g.notTested
            : g.backing
    group.push(f)
  }
  return g
}

/**
 * The findings an answer is saved with, taken from the page: the ones under
 * What backs this up and the ones under Where sources disagree. The engine
 * accepts claims only, so findings not tested yet are left out.
 */
export function answerFindings(claims: Claim[], sources: Sources): { supporting_claims: string[]; contradicting_claims: string[] } {
  const g = groupFindings(claims, [], sources)
  return { supporting_claims: g.backing.map((f) => f.record.ref), contradicting_claims: g.disagree.map((f) => f.record.ref) }
}

/** How many of a question's sources each party has checked. Replaced sources are not counted. */
export interface SourceChecks {
  total: number
  byYou: number
  byAi: number
  notChecked: number
}

export function sourceChecks(evidence: Evidence[]): SourceChecks {
  const live = evidence.filter((e) => e.verification !== 'superseded')
  const byYou = live.filter((e) => e.verification === 'human_verified').length
  const byAi = live.filter((e) => e.verification === 'source_verified').length
  return { total: live.length, byYou, byAi, notChecked: live.length - byYou - byAi }
}

/** The sources a person has not checked, in page order. Replaced sources are skipped. */
export function sourcesToCheck(evidence: Evidence[]): Evidence[] {
  return evidence.filter((e) => e.verification !== 'human_verified' && e.verification !== 'superseded')
}

/** A claim or hypothesis that links a piece of evidence. */
export interface UsedBy {
  kind: 'claim' | 'hypothesis'
  ref: string
  relation: Relation
  statement: string
}

/** The claims, then hypotheses, linking this evidence, with their statements. */
export function usedBy(evidence: Evidence, detail: InvestigationDetail): UsedBy[] {
  const claims = new Map(detail.claims.map((c) => [c.ref, c.statement]))
  const hyps = new Map(detail.hypotheses.map((h) => [h.ref, h.statement]))
  return [
    ...evidence.claim_links.map((l) => ({
      kind: 'claim' as const,
      ref: l.ref,
      relation: l.relation,
      statement: claims.get(l.ref) ?? '',
    })),
    ...evidence.hypothesis_links.map((l) => ({
      kind: 'hypothesis' as const,
      ref: l.ref,
      relation: l.relation,
      statement: hyps.get(l.ref) ?? '',
    })),
  ]
}
