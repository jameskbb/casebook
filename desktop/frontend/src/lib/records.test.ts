import { describe, expect, it } from 'vitest'
import { detail, evidence } from '../test/fixtures'
import { evidenceByRef, openHypotheses, partitionClaims, partitionHypotheses, partitionQuestions, usedBy } from './records'
import { plural } from './plural'

describe('records', () => {
  it('keeps active and disputed claims visible and rules out superseded and rejected ones', () => {
    const p = partitionClaims(detail.claims)
    expect(p.visible.map((c) => c.ref)).toEqual(['C-001', 'C-002', 'C-004'])
    expect(p.ruledOut.map((c) => c.ref)).toEqual(['C-003'])
  })

  it('keeps open and supported hypotheses visible and rules out disproven and superseded ones', () => {
    const p = partitionHypotheses(detail.hypotheses)
    expect(p.visible.map((h) => h.ref)).toEqual(['H-001', 'H-003'])
    expect(p.ruledOut.map((h) => h.ref)).toEqual(['H-002'])
  })

  it('shows open then deferred questions and collapses only resolved ones', () => {
    const p = partitionQuestions(detail.questions)
    expect(p.visible.map((q) => q.ref)).toEqual(['Q-001', 'Q-003'])
    expect(p.ruledOut.map((q) => q.ref)).toEqual(['Q-002'])
    const reordered = partitionQuestions([detail.questions[2], detail.questions[1], detail.questions[0]])
    expect(reordered.visible.map((q) => q.ref)).toEqual(['Q-001', 'Q-003'])
  })

  it('lists only Open hypotheses as still open', () => {
    expect(openHypotheses(detail.hypotheses).map((h) => h.ref)).toEqual(['H-001'])
  })

  it('indexes evidence by ref', () => {
    const m = evidenceByRef(detail)
    expect(m.get('E-003')?.title).toBe('Log ingestion is 71% of the September increase')
    expect(m.get('E-999')).toBeUndefined()
  })

  it('joins the claims and hypotheses that use a piece of evidence with their statements', () => {
    const e3 = evidence.find((e) => e.ref === 'E-003')!
    expect(usedBy(e3, detail)).toEqual([
      {
        kind: 'claim',
        ref: 'C-002',
        relation: 'supports',
        statement: 'Log ingestion, billed per GB, is $96,400 of the $135,800 increase; compute and network are flat.',
      },
      {
        kind: 'claim',
        ref: 'C-003',
        relation: 'contradicts',
        statement: 'The increase comes from the August autoscaling change adding compute nodes.',
      },
      {
        kind: 'claim',
        ref: 'C-004',
        relation: 'contradicts',
        statement: "The payments team's own debug logs are most of the $96,400.",
      },
      {
        kind: 'hypothesis',
        ref: 'H-002',
        relation: 'contradicts',
        statement: 'Data transfer from the region move doubled the network line.',
      },
    ])
  })

  it('pluralizes by count', () => {
    expect(plural(1, 'claim', 'claims')).toBe('claim')
    expect(plural(0, 'claim', 'claims')).toBe('claims')
    expect(plural(3, 'claim', 'claims')).toBe('claims')
  })
})
