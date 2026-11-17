// Extra detail fixtures for Plan 02 (reading column, history, editing and
// live refresh). Built from fixtures.ts, which stays unchanged.
import type { Activity, Actor, Claim, InvestigationDetail, Question } from '../api/types'
import { claims, detail, emptyDetail, investigation, questions, T0 } from './fixtures'

/** The seeded INV-0001 detail with the investigation archived. */
export function archivedDetail(): InvestigationDetail {
  return {
    ...detail,
    investigation: {
      ...investigation,
      ref: 'INV-0003',
      status: 'archived',
      archived: true,
      archived_at: T0,
      is_active: false,
    },
  }
}

/** A claim with the given fields over C-001's defaults. */
export function claimWith(partial: Partial<Claim>): Claim {
  return { ...claims[0], links: [], ...partial }
}

/** A detail with three superseded or rejected claims and one resolved question. */
export function manyRuledOutDetail(): InvestigationDetail {
  const resolved: Question = { ...questions[1] }
  return {
    ...detail,
    claims: [
      claims[0],
      claimWith({ id: 11, ref: 'C-011', statement: 'Retired one.', status: 'superseded', superseded_by: 'C-001', verification: 'superseded' }),
      claimWith({ id: 12, ref: 'C-012', statement: 'Retired two.', status: 'rejected' }),
      claimWith({ id: 13, ref: 'C-013', statement: 'Retired three.', status: 'rejected' }),
    ],
    questions: [questions[0], resolved],
  }
}

/** A detail with no problem statement and no conclusion but with records. */
export function partialDetail(): InvestigationDetail {
  return {
    ...detail,
    investigation: {
      ...investigation,
      problem_statement: '',
      conclusion: '',
      conclusion_confidence: 'unknown',
      conclusion_rationale: '',
    },
  }
}

/** An investigation with nothing in it (re-exported for convenience). */
export { emptyDetail }

let nextActivityId = 1000

/** One activity row. */
export function act(actor: Actor, action: string, target_ref: string, summary = ''): Activity {
  return {
    id: nextActivityId++,
    investigation_ref: 'INV-0001',
    actor,
    action,
    target_ref,
    summary,
    note: '',
    created_at: T0,
  }
}

/** n activity rows, newest first. */
export function manyActivity(n: number): Activity[] {
  const rows: Activity[] = []
  for (let i = 0; i < n; i++) {
    rows.push({
      ...act({ kind: 'ai', client: 'claude-code' }, 'evidence.add', `E-${String(n - i).padStart(3, '0')}`, `Finding ${n - i}`),
      id: n - i,
    })
  }
  return rows
}
