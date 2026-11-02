// Left kind rules (UI-SPEC Record kind visual language, D-14): one class per
// kind and state. Retired records (superseded, rejected, disproven, resolved,
// deferred) take the muted rule first; then disputed (claim status, or
// verification on any record) takes the hatched rule; hypotheses are dashed
// while open and solid once supported.
import type { Verification } from '../../api/types'
import './KindRule.css'

export type RecordKind = 'evidence' | 'claim' | 'hypothesis' | 'question'

export type RuleVariant =
  | 'evidence'
  | 'claim'
  | 'hypothesis-open'
  | 'hypothesis-supported'
  | 'question'
  | 'disputed'
  | 'retired'

export interface RuleState {
  status?: string
  verification?: Verification
}

const RETIRED = new Set(['superseded', 'rejected', 'disproven', 'resolved', 'deferred'])

export function ruleVariant(kind: RecordKind, state: RuleState = {}): RuleVariant {
  const { status = '', verification } = state
  if (RETIRED.has(status)) return 'retired'
  if (kind === 'evidence' && verification === 'superseded') return 'retired'
  if (status === 'disputed' || verification === 'disputed') return 'disputed'
  if (kind === 'hypothesis') return status === 'supported' ? 'hypothesis-supported' : 'hypothesis-open'
  return kind
}

/** "kind-rule kind-rule-<variant>" for a row root. */
export function kindRuleClass(kind: RecordKind, state: RuleState = {}): string {
  return `kind-rule kind-rule-${ruleVariant(kind, state)}`
}
