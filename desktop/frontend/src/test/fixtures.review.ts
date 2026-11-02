import type { ReviewFinding } from '../api/types'
import { claims, evidence } from './fixtures'

export const reviewFinding: ReviewFinding = {
  kind: 'claim', ref: claims[1].ref, statement: claims[1].statement,
  confidence: claims[1].confidence, rationale: claims[1].confidence_rationale,
  status: 'active', verification: 'ai_generated', filters: ['unchecked'], warnings: [],
  basis: 'snapshot-before-edit',
  evidence: [
    { relation: 'supports', evidence: { ...evidence[0], raw_detail: 'Source detail inspected by the person' } },
    { relation: 'contradicts', evidence: evidence[1] },
  ],
}
