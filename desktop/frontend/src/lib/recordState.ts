// One coherent state per claim or hypothesis (ADR-0013, AR-02). The engine
// keeps status and verification paired: verification "superseded" exactly
// when the status is superseded, and for claims verification "disputed"
// exactly when the status is disputed. The UI shows each state once and lets
// the status win, so a record written by an older Casebook that still
// disagrees never shows two contradicting marks (the engine repairs those
// rows the same way: the status wins).
import type { ClaimStatus, HypothesisStatus, Verification } from '../api/types'

export type StatementKind = 'claim' | 'hypothesis'
export type StatementStatus = ClaimStatus | HypothesisStatus

export interface StatementMarks {
  /** The verification badge to show, or null when the status says it. */
  badge: Verification | null
  /** The status label to show (RecordStatusLabel renders nothing for "active"). */
  status: StatementStatus
  /** True when the record reads as disputed. */
  disputed: boolean
  /** True when the record is superseded (only Supersede by... writes that). */
  superseded: boolean
}

export function statementMarks(
  kind: StatementKind,
  r: { status: StatementStatus; verification: Verification },
): StatementMarks {
  const superseded = r.status === 'superseded'
  if (superseded) return { badge: null, status: 'superseded', disputed: false, superseded }
  if (kind === 'claim') {
    // An active claim whose verification alone says disputed reads disputed,
    // the way migration 0002 repairs it.
    const disputed = r.status === 'disputed' || (r.status === 'active' && r.verification === 'disputed')
    if (disputed) return { badge: null, status: 'disputed', disputed, superseded }
    const contradicts = r.verification === 'disputed' || r.verification === 'superseded'
    return { badge: contradicts ? null : r.verification, status: r.status, disputed, superseded }
  }
  return {
    badge: r.verification === 'superseded' ? null : r.verification,
    status: r.status,
    disputed: r.verification === 'disputed',
    superseded,
  }
}

/** Verification values a person can choose for a claim or hypothesis. */
export const STATEMENT_VERIFICATIONS: Verification[] = ['ai_generated', 'human_verified', 'source_verified', 'disputed']
