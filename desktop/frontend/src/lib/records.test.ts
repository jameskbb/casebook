import { describe, expect, it } from 'vitest'
import { detail, evidence } from '../test/fixtures'
import type { Claim, Evidence } from '../api/types'
import { answerFindings, evidenceByRef, findingLabel, groupFindings, partitionQuestions, sourceChecks, sourcesDisagree, sourcesToCheck, usedBy } from './records'
import { plural } from './plural'

const sources = evidenceByRef(detail)
const claim = (partial: Partial<Claim>): Claim => ({ ...detail.claims[1], ref: 'C-009', ...partial })
const link = (ref: string, relation: 'supports' | 'contradicts') => ({ ...detail.claims[0].links[0], evidence_ref: ref, relation })
const withSource = (ref: string, verification: Evidence['verification']) => new Map(sources).set(ref, { ...sources.get(ref)!, verification })

describe('records', () => {
  it('works out a finding\'s one label from its sources', () => {
    const label = (ref: string) => findingLabel([...detail.claims.map((record) => ({ kind: 'claim' as const, record })), ...detail.hypotheses.map((record) => ({ kind: 'hypothesis' as const, record }))].find((f) => f.record.ref === ref)!, sources)
    // C-001 is stored as checked by a person; that is not what the label says.
    expect(label('C-001')).toBe('1 of 2 sources checked by you')
    expect(label('C-002')).toBe('0 of 2 sources checked by you')
    expect(label('C-003')).toBe('Replaced')
    expect(label('C-004')).toBe('Sources disagree')
    expect(label('H-001')).toBe('Not tested yet')
    expect(label('H-002')).toBe('Ruled out')
    expect(label('H-003')).toBe('0 of 1 source checked by you')
    const of = (c: Claim, s = sources) => findingLabel({ kind: 'claim', record: c }, s)
    expect(of(claim({ links: [] }))).toBe('No source yet')
    // A replaced source is not counted.
    expect(of(claim({ links: [link('E-002', 'supports'), link('E-004', 'supports')] }), withSource('E-002', 'superseded'))).toBe('1 of 1 source checked by you')
    expect(of(claim({ links: [link('E-002', 'supports')] }), withSource('E-002', 'superseded'))).toBe('No source yet')
    // The person read the sources and decided it still holds.
    expect(of(claim({ verification: 'human_verified', links: [link('E-002', 'supports'), link('E-003', 'contradicts')] }))).toBe('You decided this holds')
  })

  it('says sources disagree when a source that was not replaced goes against a finding nobody decided on', () => {
    const against = [link('E-002', 'supports'), link('E-003', 'contradicts')]
    expect(sourcesDisagree(claim({ links: against }), sources)).toBe(true)
    expect(sourcesDisagree(claim({ links: against, verification: 'source_verified' }), sources)).toBe(true)
    // The person decided it still holds, the source was replaced, or nothing goes against it.
    expect(sourcesDisagree(claim({ links: against, verification: 'human_verified' }), sources)).toBe(false)
    expect(sourcesDisagree(claim({ links: against }), withSource('E-003', 'superseded'))).toBe(false)
    expect(sourcesDisagree(claim({ links: [link('E-002', 'supports')] }), sources)).toBe(false)
    // Stored that way by an AI, with no source against it.
    expect(sourcesDisagree(claim({ links: [], verification: 'disputed' }), sources)).toBe(true)
    // Never a finding that was ruled out or replaced.
    expect(sourcesDisagree(claim({ links: against, status: 'rejected' }), sources)).toBe(false)
    expect(sourcesDisagree(claim({ links: against, status: 'superseded', verification: 'disputed' }), sources)).toBe(false)
    expect(sourcesDisagree({ ...detail.hypotheses[0], links: against }, sources)).toBe(true)
    expect(sourcesDisagree({ ...detail.hypotheses[1], links: against }, sources)).toBe(false)
  })

  it('saves an answer with the findings that back it and the ones where sources disagree', () => {
    const c = claim({ links: [link('E-003', 'contradicts')] })
    expect(answerFindings([...detail.claims, c], sources)).toEqual({ supporting_claims: ['C-001', 'C-002'], contradicting_claims: ['C-004', 'C-009'] })
  })

  it('lists the sources a person has not checked, in page order, without replaced ones', () => {
    expect(sourcesToCheck(detail.evidence).map((e) => e.ref)).toEqual(['E-001', 'E-002', 'E-003'])
    expect(sourcesToCheck([...withSource('E-002', 'superseded').values()]).map((e) => e.ref)).toEqual(['E-001', 'E-003'])
  })

  it('groups findings by what happened to them, claims before hypotheses', () => {
    const g = groupFindings(detail.claims, detail.hypotheses, sources)
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
      sources,
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
