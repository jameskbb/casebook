import { describe, expect, it } from 'vitest'
import { detail, evidence } from '../test/fixtures'
import { evidenceByRef, groupFindings, partitionQuestions, sourceChecks, usedBy } from './records'
import { plural } from './plural'

describe('records', () => {
  it('groups findings by what happened to them, claims before hypotheses', () => {
    const g = groupFindings(detail.claims, detail.hypotheses)
    const refs = (list: { record: { ref: string } }[]) => list.map((f) => f.record.ref)
    expect(refs(g.backing)).toEqual(['C-001', 'C-002', 'H-003'])
    expect(refs(g.disagree)).toEqual(['C-004'])
    expect(refs(g.notTested)).toEqual(['H-001'])
    expect(refs(g.ruledOut)).toEqual(['H-002'])
    expect(refs(g.replaced)).toEqual(['C-003'])
  })

  it('puts a rejected claim under ruled out and a hypothesis whose sources disagree under disagree', () => {
    const g = groupFindings(
      [{ ...detail.claims[0], ref: 'C-009', status: 'rejected' }],
      [{ ...detail.hypotheses[0], ref: 'H-009', status: 'open', verification: 'disputed' }],
    )
    expect(g.ruledOut.map((f) => f.record.ref)).toEqual(['C-009'])
    expect(g.disagree.map((f) => f.record.ref)).toEqual(['H-009'])
    expect(g.backing).toEqual([])
  })

  it('counts who checked each source and leaves replaced sources out', () => {
    const e = detail.evidence[0]
    const with_ = (verification: typeof e.verification) => ({ ...e, verification })
    expect(sourceChecks([with_('human_verified'), with_('source_verified'), with_('ai_generated'), with_('disputed'), with_('superseded')])).toEqual({
      total: 4,
      byYou: 1,
      byAi: 1,
      notChecked: 2,
    })
    expect(sourceChecks([])).toEqual({ total: 0, byYou: 0, byAi: 0, notChecked: 0 })
  })

  it('shows open then deferred questions and collapses only resolved ones', () => {
    const p = partitionQuestions(detail.questions)
    expect(p.visible.map((q) => q.ref)).toEqual(['Q-001', 'Q-003'])
    expect(p.ruledOut.map((q) => q.ref)).toEqual(['Q-002'])
    const reordered = partitionQuestions([detail.questions[2], detail.questions[1], detail.questions[0]])
    expect(reordered.visible.map((q) => q.ref)).toEqual(['Q-001', 'Q-003'])
  })

  it('indexes evidence by ref', () => {
    const m = evidenceByRef(detail)
    expect(m.get('E-003')?.title).toBe('Calls about a bill: 400 in September, 1,070 in October')
    expect(m.get('E-999')).toBeUndefined()
  })

  it('joins the claims and hypotheses that use a piece of evidence with their statements', () => {
    const e3 = evidence.find((e) => e.ref === 'E-003')!
    expect(usedBy(e3, detail)).toEqual([
      {
        kind: 'claim',
        ref: 'C-002',
        relation: 'supports',
        statement: 'The extra calls were about a bill. About the same number of people called to buy.',
      },
      {
        kind: 'claim',
        ref: 'C-003',
        relation: 'contradicts',
        statement: 'The extra calls were people calling to buy.',
      },
      {
        kind: 'claim',
        ref: 'C-004',
        relation: 'contradicts',
        statement: "Callers waited too long and hung up before buying.",
      },
      {
        kind: 'hypothesis',
        ref: 'H-002',
        relation: 'contradicts',
        statement: 'A new ad brought in more callers.',
      },
    ])
  })

  it('pluralizes by count', () => {
    expect(plural(1, 'claim', 'claims')).toBe('claim')
    expect(plural(0, 'claim', 'claims')).toBe('claims')
    expect(plural(3, 'claim', 'claims')).toBe('claims')
  })
})
