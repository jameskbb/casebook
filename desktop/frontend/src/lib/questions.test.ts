import { describe, expect, it } from 'vitest'
import type { InvestigationSummary } from '../api/types'
import { summaries } from '../test/fixtures'
import { checkedLine, groupQuestions, homeLists, needOf, needsYou, questionChecks } from './questions'

const [calls, helpDesk, cancels] = summaries
const q = (partial: Partial<InvestigationSummary>): InvestigationSummary => ({ ...calls, archived: false, ...partial })

describe('questions', () => {
  it('groups by whether the question has an answer, keeping the list order', () => {
    const g = groupQuestions(summaries)
    expect(g.noAnswer.map((s) => s.ref)).toEqual(['INV-0002'])
    expect(g.answered.map((s) => s.ref)).toEqual(['INV-0001', 'INV-0003'])
    expect(groupQuestions([q({ conclusion: '   ' })]).answered).toEqual([])
  })

  it('names the one thing a question needs next, first match first', () => {
    const all = { sources_total: 3, sources_checked_by_you: 3, findings_disagree: 0, answer_checked_by_you: true }
    expect(needOf(q({ ...all, sources_checked_by_you: 1, findings_disagree: 2, answer_checked_by_you: false }))).toEqual({ kind: 'check-sources', line: '1 of 3 sources checked by you', button: 'Check' })
    expect(needOf(q({ ...all, findings_disagree: 2, answer_checked_by_you: false }))).toEqual({ kind: 'decide', line: '2 findings where sources disagree', button: 'Decide' })
    expect(needOf(q({ ...all, findings_disagree: 1 }))?.line).toBe('1 finding where sources disagree')
    expect(needOf(q({ ...all, conclusion: '', answer_checked_by_you: false }))).toEqual({ kind: 'write-answer', line: 'Every source is checked. Write the answer.', button: 'Write the answer' })
    expect(needOf(q({ ...all, answer_checked_by_you: false }))).toEqual({ kind: 'check-answer', line: 'The answer is not checked yet.', button: 'Check the answer' })
    expect(needOf(q(all))).toBeNull()
    // No sources and no answer: nothing for a person to do yet.
    expect(needOf(q({ conclusion: '', sources_total: 0, sources_checked_by_you: 0, findings_disagree: 0, answer_checked_by_you: false }))).toBeNull()
    // An answer with no sources still wants its own check.
    expect(needOf(q({ sources_total: 0, sources_checked_by_you: 0, findings_disagree: 0, answer_checked_by_you: false }))?.kind).toBe('check-answer')
    // A backend that does not say whether the answer is checked never asks for it.
    const older = q(all)
    delete older.answer_checked_by_you
    expect(needOf(older)).toBeNull()
    expect(needOf(q({ ...all, sources_checked_by_you: 0, archived: true }))).toBeNull()
  })

  it('needs you when a source is not checked by you or sources disagree', () => {
    expect(needsYou(calls)).toBe(true)
    expect(needsYou(helpDesk)).toBe(true)
    expect(needsYou(q({ sources_total: 3, sources_checked_by_you: 3, findings_disagree: 0, answer_checked_by_you: true }))).toBe(false)
    expect(needsYou(q({ sources_total: 3, sources_checked_by_you: 3, findings_disagree: 1, answer_checked_by_you: true }))).toBe(true)
    expect(needsYou(q({ sources_total: 0, sources_checked_by_you: 0, findings_disagree: 0, answer_checked_by_you: true }))).toBe(false)
    // An archived question is put away, whatever is left in it.
    expect(needsYou(q({ archived: true }))).toBe(false)
  })

  it('reads a row from an older backend as no answer, no sources and nothing to check', () => {
    const old: InvestigationSummary = { ...calls }
    delete old.conclusion
    delete old.sources_total
    delete old.sources_checked_by_you
    delete old.findings_disagree
    delete old.answer_checked_by_you
    expect(questionChecks(old)).toEqual({ answer: '', answered: false, sourcesTotal: 0, checkedByYou: 0, disagree: 0 })
    expect(needsYou(old)).toBe(false)
    expect(checkedLine(old)).toBe('')
    expect(homeLists([old]).waiting).toEqual([old])
  })

  it('words the checked count with the right plural', () => {
    expect(checkedLine(calls)).toBe('1 of 4 sources checked by you')
    expect(checkedLine(cancels)).toBe('1 of 1 source checked by you')
    // A count above the total cannot be shown as more than all of them.
    expect(checkedLine(q({ sources_total: 2, sources_checked_by_you: 5 }))).toBe('2 of 2 sources checked by you')
  })

  it('sorts Home into need you, answered and waiting, and leaves archived questions out', () => {
    const done = q({ ref: 'INV-0005', sources_total: 2, sources_checked_by_you: 2, findings_disagree: 0, answer_checked_by_you: true })
    const fresh = q({ ref: 'INV-0006', conclusion: '', sources_total: 0, sources_checked_by_you: 0, findings_disagree: 0 })
    const h = homeLists([...summaries, done, fresh])
    expect(h.needYou.map((s) => s.ref)).toEqual(['INV-0001', 'INV-0002'])
    expect(h.answered.map((s) => s.ref)).toEqual(['INV-0005'])
    expect(h.waiting.map((s) => s.ref)).toEqual(['INV-0006'])
  })
})
