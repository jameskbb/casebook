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

/** Active and disputed claims show; superseded and rejected collapse. */
export function partitionClaims(claims: Claim[]): Partition<Claim> {
  return split(claims, (c) => c.status === 'superseded' || c.status === 'rejected')
}

/** Open and supported hypotheses show; disproven and superseded collapse. */
export function partitionHypotheses(hypotheses: Hypothesis[]): Partition<Hypothesis> {
  return split(hypotheses, (h) => h.status === 'disproven' || h.status === 'superseded')
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

/** Hypotheses still being tested (status Open), listed again under Still open. */
export function openHypotheses(hypotheses: Hypothesis[]): Hypothesis[] {
  return hypotheses.filter((h) => h.status === 'open')
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
