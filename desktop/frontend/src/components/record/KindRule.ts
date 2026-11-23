// Left kind rules: one class per
// kind and state. Retired records (superseded, rejected, disproven, resolved,
// deferred) take the muted rule first; then a finding where sources disagree
// takes the hatched rule (a finding row passes status "disputed" when
// lib/records sourcesDisagree says so; a source never takes it). A finding
// not tested yet (an open hypothesis) takes the neutral rule: nothing backs it
// yet. Every other finding, claim or hypothesis, has the same rule: its label
// says its state.
import type { Verification } from '../../api/types'
import './KindRule.css'

export type RecordKind = 'evidence' | 'claim' | 'hypothesis' | 'question'

export type RuleVariant = 'evidence' | 'claim' | 'question' | 'disputed' | 'retired' | 'untested'

export interface RuleState {
  status?: string
  verification?: Verification
}

const RETIRED = new Set(['superseded', 'rejected', 'disproven', 'resolved', 'deferred'])

export function ruleVariant(kind: RecordKind, state: RuleState = {}): RuleVariant {
  const { status = '', verification } = state
  if (RETIRED.has(status)) return 'retired'
  if (kind === 'evidence' && verification === 'superseded') return 'retired'
  if (status === 'disputed' || (kind !== 'evidence' && verification === 'disputed')) return 'disputed'
  if (kind === 'hypothesis') return status === 'open' ? 'untested' : 'claim'
  return kind
}

/** "kind-rule kind-rule-<variant>" for a row root. */
export function kindRuleClass(kind: RecordKind, state: RuleState = {}): string {
  return `kind-rule kind-rule-${ruleVariant(kind, state)}`
}
