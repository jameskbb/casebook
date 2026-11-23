import { describe, expect, it } from 'vitest'
import type { Activity, Actor } from '../api/types'
import { activitySentence, activitySummary, actorPhrase, actorTooltip, verbPhrase } from './activity'

const a = (kind: Actor['kind'], client: string): Actor => ({ kind, client })

function row(action: string, target_ref: string, actor: Actor = a('human', 'casebook-ui')): Activity {
  return { id: 1, investigation_ref: 'INV-0001', actor, action, target_ref, summary: '', note: '', created_at: '' }
}

describe('actorPhrase', () => {
  const cases: [Actor, string][] = [
    [a('human', 'casebook-ui'), 'You (Casebook)'],
    [a('human', 'casebook-cli'), 'You (Casebook CLI)'],
    [a('human', 'other'), 'You (other)'],
    [a('ai', 'claude-code'), 'Claude Code (AI)'],
    [a('ai', 'codex'), 'Codex (AI)'],
    [a('ai', 'mcp-client'), 'An AI client (AI)'],
    [a('ai', 'other'), 'other (AI)'],
    [a('import', 'casebook-ui'), 'Import (Casebook)'],
    [a('import', 'casebook-cli'), 'Import (Casebook CLI)'],
    [a('import', 'claude-code'), 'Import (Claude Code)'],
    [a('import', 'codex'), 'Import (Codex)'],
    [a('import', 'my-tool'), 'Import (my-tool)'],
    [a('system', 'casebook'), 'Casebook (system)'],
    [a('system', ''), 'Casebook (system)'],
  ]
  for (const [actor, phrase] of cases) {
    it(`${actor.kind} ${actor.client || '(none)'} reads ${phrase}`, () => {
      expect(actorPhrase(actor)).toBe(phrase)
    })
  }
})

describe('actorTooltip', () => {
  it('names the raw kind and client', () => {
    expect(actorTooltip(a('ai', 'claude-code'))).toBe('AI, claude-code')
    expect(actorTooltip(a('human', 'casebook-ui'))).toBe('Human, casebook-ui')
    expect(actorTooltip(a('import', 'codex'))).toBe('Import, codex')
    expect(actorTooltip(a('system', 'casebook'))).toBe('System, casebook')
  })
})

describe('verbPhrase', () => {
  const cases: [string, string, string][] = [
    ['investigation.create', 'INV-0001', 'created this question'],
    ['investigation.update', 'INV-0001', 'edited the question'],
    ['investigation.archive', 'INV-0001', 'archived this question'],
    ['investigation.unarchive', 'INV-0001', 'unarchived this question'],
    ['conclusion.update', 'INV-0001', 'updated the answer'],
    ['evidence.add', 'E-005', 'added source E-005'],
    ['evidence.update', 'E-002', 'updated source E-002'],
    ['claim.add', 'C-001', 'added finding C-001'],
    ['claim.update', 'C-001', 'updated finding C-001'],
    ['claim.supersede', 'C-003', 'replaced finding C-003'],
    ['hypothesis.add', 'H-001', 'added finding H-001'],
    ['hypothesis.update', 'H-001', 'updated finding H-001'],
    ['hypothesis.supersede', 'H-002', 'replaced finding H-002'],
    ['link.add', 'C-001', 'linked a source to C-001'],
    ['link.update', 'C-001', 'changed how a source relates to C-001'],
    ['link.remove', 'H-003', 'unlinked a source from H-003'],
    ['question.add', 'Q-004', 'asked Q-004'],
    ['question.update', 'Q-001', 'reworded Q-001'],
    ['question.resolve', 'Q-001', 'resolved Q-001'],
    ['question.defer', 'Q-003', 'deferred Q-003'],
    ['question.reopen', 'Q-002', 'reopened Q-002'],
    ['active.set', 'INV-0001', "made this your AI's current question"],
    ['active.clear', 'INV-0001', "cleared your AI's current question"],
    ['checkpoint', 'INV-0001', 'saved a checkpoint'],
    ['import', 'INV-0004', 'imported this question'],
    ['restore', '', 'restored all data from a backup'],
    ['x.y', 'T', 'recorded x.y on T'],
    ['x.y', '', 'recorded x.y'],
  ]
  for (const [action, target, phrase] of cases) {
    it(`${action} ${target || '(no target)'} reads ${phrase}`, () => {
      expect(verbPhrase(action, target)).toBe(phrase)
    })
  }
})

describe('activitySentence', () => {
  it('makes E, C and H refs ref segments', () => {
    const s = activitySentence(row('evidence.add', 'E-005', a('ai', 'claude-code')))
    expect(s.actor).toBe('Claude Code (AI)')
    expect(s.tooltip).toBe('AI, claude-code')
    expect(s.segments).toEqual([{ text: 'added source ' }, { ref: 'E-005' }])
    expect(activitySentence(row('link.update', 'C-001')).segments).toEqual([
      { text: 'changed how a source relates to ' },
      { ref: 'C-001' },
    ])
    expect(activitySentence(row('hypothesis.supersede', 'H-002')).segments).toEqual([
      { text: 'replaced finding ' },
      { ref: 'H-002' },
    ])
  })

  it('keeps Q and INV refs as plain text', () => {
    expect(activitySentence(row('question.resolve', 'Q-001')).segments).toEqual([{ text: 'resolved Q-001' }])
    expect(activitySentence(row('x.y', 'INV-0002')).segments).toEqual([{ text: 'recorded x.y on INV-0002' }])
    expect(activitySentence(row('investigation.create', 'INV-0001')).segments).toEqual([
      { text: 'created this question' },
    ])
  })
})

describe('imported and deleted rows', () => {
  it('never calls the person in an imported file "You"', () => {
    const s = activitySentence({ ...row('evidence.add', 'E-099', a('human', 'james')), imported: true })
    expect(s.imported).toBe(true)
    expect(s.actor).toBe('james (person, per the imported file)')
    expect(s.tooltip).toBe('Human, james (from an imported file)')
    const ai = activitySentence({ ...row('claim.add', 'C-001', a('ai', 'claude-code')), imported: true })
    expect(ai.actor).toBe('Claude Code (AI, per the imported file)')
    expect(activitySentence(row('claim.add', 'C-001')).imported).toBe(false)
  })

  it('phrases deletes and never links the deleted ref', () => {
    expect(verbPhrase('evidence.delete', 'E-002')).toBe('deleted source E-002')
    expect(verbPhrase('question.delete', 'Q-001')).toBe('deleted Q-001')
    expect(verbPhrase('investigation.delete', 'INV-0003')).toBe('deleted question INV-0003')
    expect(activitySentence(row('claim.delete', 'C-003')).segments).toEqual([{ text: 'deleted finding C-003' }])
  })
})

describe('activitySummary', () => {
  const ai = a('ai', 'codex')
  const at = (action: string, target: string, summary: string, actor: Actor = ai, extra: Partial<Activity> = {}): Activity =>
    ({ ...row(action, target, actor), summary, ...extra })

  it('says an AI edit reset verification without naming the lost label', () => {
    expect(activitySummary(at('claim.update', 'C-001', 'Updated claim C-001: statement, verification; verification reset to ai_generated after AI edit')))
      .toBe('Edited finding C-001 (statement). It changed after it was checked, so it reads Not checked again and is back in Review findings.')
    expect(activitySummary(at('hypothesis.update', 'H-002', 'Updated hypothesis H-002: rationale, confidence high, verification; verification reset to ai_generated after AI edit')))
      .toBe('Edited finding H-002 (rationale). It changed after it was checked, so it reads Not checked again and is back in Review findings.')
    expect(activitySummary(at('evidence.update', 'E-004', 'Updated evidence E-004: title, verification; verification reset to ai_generated after AI edit')))
      .toBe('Edited source E-004 (title). It changed after it was checked, so it reads Not checked again.')
  })

  it('names verification labels instead of enum values', () => {
    expect(activitySummary(at('claim.update', 'C-003', 'Set verification of C-003 to human_verified', a('human', 'casebook-ui'))))
      .toBe('Marked C-003: You checked this')
    expect(activitySummary(at('claim.update', 'C-001', 'Set verification of C-001 to disputed; status disputed')))
      .toBe('Marked C-001: Sources disagree')
    expect(activitySummary(at('hypothesis.update', 'H-001', 'Set verification of H-001 to source_verified; status supported')))
      .toBe('Marked H-001: AI checked the source; now Supported')
  })

  it('phrases a person\'s own verification and plain edits without enum values', () => {
    const you = a('human', 'casebook-ui')
    expect(activitySummary(at('claim.update', 'C-002', 'Updated claim C-002: verification human_verified', you)))
      .toBe('Marked C-002: You checked this')
    expect(activitySummary(at('hypothesis.update', 'H-004', 'Updated hypothesis H-004: verification human_verified', you)))
      .toBe('Marked H-004: You checked this')
    expect(activitySummary(at('evidence.update', 'E-003', 'Updated evidence E-003: verification human_verified', you)))
      .toBe('Marked E-003: You checked this')
    expect(activitySummary(at('claim.update', 'C-001', 'Updated claim C-001: status disputed, verification disputed', you)))
      .toBe('Marked C-001: Sources disagree')
    expect(activitySummary(at('claim.update', 'C-002', 'Updated claim C-002: statement, confidence high')))
      .toBe('Edited finding C-002 (statement)')
    expect(activitySummary(at('claim.update', 'C-001', 'Updated claim C-001: statement'))).toBe('Edited finding C-001 (statement)')
  })

  it('speaks of the case, never the investigation', () => {
    expect(activitySummary(at('active.set', 'INV-0001', 'Set active investigation to INV-0001'))).toBe("Made INV-0001 your AI's current question")
    expect(activitySummary(at('active.clear', 'INV-0001', 'Cleared active investigation (was INV-0001)'))).toBe("Cleared your AI's current question (was INV-0001)")
    expect(activitySummary(at('investigation.create', 'INV-0001', 'Created investigation INV-0001 "Northeast margin down 6 points in Q3"'))).toBe('Created this question')
    expect(activitySummary(at('investigation.create', 'INV-0003', 'Created investigation INV-0003 "Other"'))).toBe('Created question INV-0003')
  })

  it('tells the conclusion review apart from the edit before it', () => {
    const ctx = { conclusionReviewedAt: '2026-10-27T03:29:47.051Z' }
    const edit = at('conclusion.update', 'INV-0001', 'Updated conclusion of INV-0001 (confidence high)', ai, { created_at: '2026-10-27T03:20:00.000Z' })
    const review = at('conclusion.update', 'INV-0001', 'Updated conclusion of INV-0001 (confidence high)', a('human', 'casebook-ui'), { created_at: ctx.conclusionReviewedAt })
    expect(activitySummary(edit, ctx)).toBe('Updated the answer')
    expect(activitySummary(review, ctx)).toBe('Recorded a review of the answer')
    expect(activitySummary(review)).toBe('Updated the answer')
  })

  it('keeps any other summary as stored', () => {
    expect(activitySummary(at('evidence.add', 'E-005', 'Added evidence E-005 "Trace store query"'))).toBe('Added evidence E-005 "Trace store query"')
  })
})
