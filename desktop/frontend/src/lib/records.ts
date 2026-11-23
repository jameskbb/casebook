// Pure helpers over an InvestigationDetail for the reading column. Records
// arrive ordered by ref (docs/api.md section 8) and keep that order here.
import type {
  Claim,
  Evidence,
  Hypothesis,
  InvestigationDetail,
  Question,
  Relation,
} from '../api/types'
import { statementMarks } from './recordState'

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
 * Open questions show, then deferred ones (each in ref order): a deferred
 * question is still unanswered. Only resolved questions collapse.
 */
export function partitionQuestions(questions: Question[]): Partition<Question> {
  const p = split(questions, (q) => q.status === 'resolved')
  p.visible = [...p.visible.filter((q) => q.status === 'open'), ...p.visible.filter((q) => q.status !== 'open')]
  return p
}

/** A claim or a hypothesis: on screen both are a finding. */
export type Finding = { kind: 'claim'; record: Claim } | { kind: 'hypothesis'; record: Hypothesis }

/** Findings sorted by what happened to them, claims before hypotheses, each in ref order. */
export interface FindingGroups {
  /** Current claims and supported hypotheses: what the answer can stand on. */
  backing: Finding[]
  /** A source contradicts it and nobody has decided yet. */
  disagree: Finding[]
  /** Hypotheses no source has backed or ruled out. */
  notTested: Finding[]
  /** Rejected claims and disproven hypotheses. */
  ruledOut: Finding[]
  /** Superseded by a newer finding. */
  replaced: Finding[]
}

export function groupFindings(claims: Claim[], hypotheses: Hypothesis[]): FindingGroups {
  const g: FindingGroups = { backing: [], disagree: [], notTested: [], ruledOut: [], replaced: [] }
  for (const record of claims) {
    const marks = statementMarks('claim', record)
    const group = marks.superseded ? g.replaced : record.status === 'rejected' ? g.ruledOut : marks.disputed ? g.disagree : g.backing
    group.push({ kind: 'claim', record })
  }
  for (const record of hypotheses) {
    const marks = statementMarks('hypothesis', record)
    const group = marks.superseded
      ? g.replaced
      : record.status === 'disproven'
        ? g.ruledOut
        : marks.disputed
          ? g.disagree
          : record.status === 'open'
            ? g.notTested
            : g.backing
    group.push({ kind: 'hypothesis', record })
  }
  return g
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

export function evidenceByRef(detail: InvestigationDetail): Map<string, Evidence> {
  return new Map(detail.evidence.map((e) => [e.ref, e]))
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
