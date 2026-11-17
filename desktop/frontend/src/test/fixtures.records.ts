// Fixtures for Plan 03 (drawers, record editing and record dialogs). Built
// from fixtures.ts, which stays unchanged.
import type { Claim, Evidence, Hypothesis, InvestigationDetail, Question } from '../api/types'
import { claims, detail, evidence, hypotheses, questions, T0 } from './fixtures'

/** The billing export query stored as E-003's raw detail in the demo seed. */
export const E003_RAW = `SELECT line_item,
       SUM(CASE WHEN usage_month = '2026-09' THEN cost END)
     - SUM(CASE WHEN usage_month = '2026-08' THEN cost END) AS delta_usd
FROM billing.monthly_costs
WHERE usage_month IN ('2026-08', '2026-09')
GROUP BY line_item
ORDER BY delta_usd DESC;
-- log_ingestion +96,400 | traces +29,800 | other +8,300 | compute +1,900 | network -600`

/** A raw detail of exactly 64 KiB (the engine maximum), many short lines. */
export function bigRawDetail(): string {
  const line = 'row 0000: value=abcdefghijklmnopqrstuvwxyz0123456789\n'
  let out = ''
  let i = 0
  while (out.length + line.length <= 65536) {
    out += line.replace('0000', String(i++).padStart(4, '0'))
  }
  return out.padEnd(65536, 'x')
}

/** Evidence by ref from the seeded detail, with fields replaced. */
export function evidenceWith(ref: string, partial: Partial<Evidence> = {}): Evidence {
  const e = evidence.find((x) => x.ref === ref)
  if (!e) throw new Error(`no evidence ${ref} in fixtures`)
  return { ...e, ...partial }
}

/** A bare piece of evidence: no source label, reference, raw detail, metadata or links. */
export const bareEvidence: Evidence = {
  ...evidence[0],
  id: 9,
  ref: 'E-009',
  title: 'Screenshot of the billing console',
  summary: 'The billing console lists log ingestion as the largest line item.',
  source_type: 'manual_observation',
  source_ref: '',
  source_label: '',
  captured_at: '',
  has_raw_detail: false,
  metadata: '',
  claim_links: [],
  hypothesis_links: [],
}

/** The seeded detail with fields replaced. */
export function detailWith(partial: Partial<InvestigationDetail>): InvestigationDetail {
  return { ...detail, ...partial }
}

/** The seeded detail plus the bare evidence E-009. */
export function detailWithBareEvidence(): InvestigationDetail {
  return detailWith({ evidence: [...evidence, bareEvidence] })
}

/** A claim over C-001's defaults. */
export function claim(partial: Partial<Claim>): Claim {
  return { ...claims[0], ...partial }
}

/** A hypothesis over H-001's defaults. */
export function hypothesis(partial: Partial<Hypothesis>): Hypothesis {
  return { ...hypotheses[0], ...partial }
}

/** A question over Q-001's defaults. */
export function question(partial: Partial<Question>): Question {
  return { ...questions[0], ...partial }
}

/** The claim C-005 the add-claim tests create. */
export const newClaim: Claim = claim({
  id: 5,
  ref: 'C-005',
  statement: 'The log level comes from the chart default.',
  confidence: 'medium',
  confidence_rationale: '',
  verification: 'human_verified',
  links: [],
  created_at: T0,
  updated_at: T0,
})

/** The seeded detail after C-005 was added. */
export function detailWithNewClaim(): InvestigationDetail {
  return detailWith({ claims: [...claims, newClaim] })
}

/** The seeded detail with every link of one claim removed and no evidence. */
export function detailWithoutEvidence(): InvestigationDetail {
  return detailWith({
    evidence: [],
    claims: claims.map((c) => ({ ...c, links: [] })),
    hypotheses: hypotheses.map((h) => ({ ...h, links: [] })),
  })
}
