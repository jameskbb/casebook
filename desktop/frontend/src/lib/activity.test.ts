import { describe, expect, it } from 'vitest'
import type { Activity, Actor } from '../api/types'
import { activitySentence, activitySummary, actorPhrase, actorTooltip, importedActorPhrase } from './activity'

const a = (kind: Actor['kind'], client: string): Actor => ({ kind, client })

function row(action: string, target_ref: string, actor: Actor = a('human', 'casebook-ui')): Activity {
  return { id: 1, investigation_ref: 'INV-0001', actor, action, target_ref, summary: '', note: '', created_at: '' }
}

/** The verb phrase as one string: a code nothing opens follows a space, as on screen. */
function phrase(action: string, target: string): string {
  return activitySentence(row(action, target))
    .segments.map((s) => ('text' in s ? s.text : 'ref' in s ? s.ref : ` ${s.id}`))
    .join('')
}

describe('actorPhrase', () => {
  const cases: [Actor, string][] = [
    [a('human', 'casebook-ui'), 'You'],
    [a('human', ''), 'You'],
    [a('human', 'casebook-cli'), 'You'],
    [a('human', 'other'), 'You'],
    [a('ai', 'claude-code'), 'Your AI'],
    [a('ai', 'codex'), 'Your AI'],
    [a('ai', 'mcp-client'), 'Your AI'],
    [a('ai', ''), 'Your AI'],
    [a('ai', 'other'), 'Your AI'],
    [a('import', 'casebook-ui'), 'Casebook'],
    [a('import', 'my-tool'), 'Casebook'],
    [a('system', 'casebook'), 'Casebook'],
    [a('system', ''), 'Casebook'],
  ]
  for (const [actor, text] of cases) {
    it(`${actor.kind} ${actor.client || '(none)'} reads ${text}`, () => {
      expect(actorPhrase(actor)).toBe(text)
    })
  }
})

describe('importedActorPhrase', () => {
  it('names who the file says did it, and never says You', () => {
    expect(importedActorPhrase(a('human', 'james'))).toBe('james, according to the file')
    expect(importedActorPhrase(a('human', ''))).toBe('A person, according to the file')
    expect(importedActorPhrase(a('ai', 'claude-code'))).toBe('Claude Code, according to the file')
    expect(importedActorPhrase(a('ai', ''))).toBe('An AI, according to the file')
    expect(importedActorPhrase(a('system', 'casebook'))).toBe('Casebook, according to the file')
  })
})

describe('actorTooltip', () => {
  it('names the kind and the tool by its friendly name', () => {
    expect(actorTooltip(a('ai', 'claude-code'))).toBe('AI, Claude Code')
    expect(actorTooltip(a('human', 'casebook-ui'))).toBe('Person, Casebook')
    expect(actorTooltip(a('import', 'codex'))).toBe('File, Codex')
    expect(actorTooltip(a('ai', 'my-tool'))).toBe('AI, my-tool')
    expect(actorTooltip(a('system', ''))).toBe('Casebook')
  })
})

describe('the verb phrase of a row', () => {
  const cases: [string, string, string][] = [
    ['investigation.create', 'INV-0001', 'created this question'],
    ['investigation.update', 'INV-0001', 'edited the question'],
    ['investigation.archive', 'INV-0001', 'archived this question'],
    ['investigation.unarchive', 'INV-0001', 'put this question back in the list'],
    ['conclusion.update', 'INV-0001', 'updated the answer'],
    ['evidence.add', 'E-005', 'added the source E-005'],
    ['evidence.update', 'E-002', 'updated the source E-002'],
    ['claim.add', 'C-001', 'added the finding C-001'],
    ['claim.update', 'C-001', 'updated the finding C-001'],
    ['claim.supersede', 'C-003', 'replaced the finding C-003'],
    ['hypothesis.add', 'H-001', 'added the finding H-001'],
    ['hypothesis.update', 'H-001', 'updated the finding H-001'],
    ['hypothesis.supersede', 'H-002', 'replaced the finding H-002'],
    ['link.add', 'C-001', 'added a source to the finding C-001'],
    ['link.update', 'C-001', 'changed whether a source backs the finding C-001'],
    ['link.remove', 'H-003', 'removed a source from the finding H-003'],
    ['question.add', 'Q-004', 'added something to find out Q-004'],
    ['question.add', '', 'added something to find out'],
    ['question.update', 'Q-001', 'reworded something to find out Q-001'],
    ['question.resolve', 'Q-001', 'answered something to find out Q-001'],
    ['question.defer', 'Q-003', 'left something to find out for later Q-003'],
    ['question.reopen', 'Q-002', 'opened something to find out again Q-002'],
    ['evidence.delete', 'E-002', 'deleted a source E-002'],
    ['question.delete', 'Q-001', 'deleted something to find out Q-001'],
    ['investigation.delete', 'INV-0003', 'deleted a question INV-0003'],
    ['active.set', 'INV-0001', 'chose this question for your AI to work on'],
    ['active.clear', 'INV-0001', 'stopped your AI working on this question'],
    ['checkpoint', 'INV-0001', 'saved its progress'],
    ['import', 'INV-0004', 'added this question from a file'],
    ['restore', '', 'brought back a backup'],
    // An action this version does not know never shows its engine name or its target.
    ['x.y', 'T', 'made a change'],
    ['x.y', '', 'made a change'],
  ]
  for (const [action, target, text] of cases) {
    it(`${action} ${target || '(no target)'} reads ${text}`, () => {
      expect(phrase(action, target)).toBe(text)
    })
  }
})

describe('activitySentence', () => {
  it('makes E, C and H refs ref segments', () => {
    const s = activitySentence(row('evidence.add', 'E-005', a('ai', 'claude-code')))
    expect(s.actor).toBe('Your AI')
    expect(s.tooltip).toBe('AI, Claude Code')
    expect(s.segments).toEqual([{ text: 'added the source ' }, { ref: 'E-005' }])
    expect(activitySentence(row('link.update', 'C-001')).segments).toEqual([
      { text: 'changed whether a source backs the finding ' },
      { ref: 'C-001' },
    ])
    expect(activitySentence(row('hypothesis.supersede', 'H-002')).segments).toEqual([
      { text: 'replaced the finding ' },
      { ref: 'H-002' },
    ])
  })

  it('makes a code nothing opens an id segment, so the sentence reads without it', () => {
    expect(activitySentence(row('question.resolve', 'Q-001')).segments).toEqual([{ text: 'answered something to find out' }, { id: 'Q-001' }])
    expect(activitySentence(row('investigation.create', 'INV-0001')).segments).toEqual([
      { text: 'created this question' },
    ])
  })

  it('says made a change for an action nobody knows', () => {
    expect(activitySentence(row('x.y', 'INV-0002')).segments).toEqual([{ text: 'made a change' }])
    expect(activitySentence(row('claim.frobnicate', 'C-001')).segments).toEqual([{ text: 'made a change' }])
  })
})

describe('imported and deleted rows', () => {
  it('never calls the person in an imported file "You"', () => {
    const s = activitySentence({ ...row('evidence.add', 'E-099', a('human', 'james')), imported: true })
    expect(s.imported).toBe(true)
    expect(s.actor).toBe('james, according to the file')
    expect(s.tooltip).toBe('Person, james (from a file)')
    const ai = activitySentence({ ...row('claim.add', 'C-001', a('ai', 'claude-code')), imported: true })
    expect(ai.actor).toBe('Claude Code, according to the file')
    expect(activitySentence(row('claim.add', 'C-001')).imported).toBe(false)
  })

  it('never links the deleted ref', () => {
    expect(activitySentence(row('claim.delete', 'C-003')).segments).toEqual([{ text: 'deleted a finding' }, { id: 'C-003' }])
  })
})

describe('activitySummary', () => {
  const ai = a('ai', 'codex')
  const you = a('human', 'casebook-ui')
  const at = (action: string, target: string, summary: string, actor: Actor = ai, extra: Partial<Activity> = {}): Activity =>
    ({ ...row(action, target, actor), summary, ...extra })

  const NEEDS_CHECKING_AGAIN = 'It changed after it was checked, so it needs checking again.'
  const NOTE = 'Compared the call log with the billing notice. Next: ask who approves refunds.'

  // [action, stored summary, second line]. The stored text is what the Go core
  // wrote, in its old wording and in the wording it writes now.
  const shapes: [string, string, string][] = [
    // New wording.
    ['active.set', "Made INV-0002 your AI's current question", ''],
    ['active.set', "Made INV-0002 your AI's current question for C:\\work\\x", ''],
    ['active.clear', "Cleared your AI's current question (was INV-0002)", ''],
    ['investigation.create', 'Created question INV-0002 "Why did calls go up?"', ''],
    ['link.add', 'Linked E-009 to C-003 (supports)', ''],
    ['link.update', 'Changed link E-009 to C-003 from supports to contradicts', ''],
    ['link.remove', 'Unlinked E-009 from C-003', ''],
    ['claim.supersede', 'C-001 was replaced by C-002', ''],
    ['import', 'Imported question INV-0001 as INV-0004 (4 sources, 3 findings, 1 follow-up question)', ''],
    ['conclusion.update', 'Updated conclusion of INV-0001 (confidence high); verification reset to ai_generated after AI edit', NEEDS_CHECKING_AGAIN],
    ['conclusion.update', 'Cleared the answer', 'Cleared the answer.'],
    ['claim.update', 'Updated claim C-002: confidence_rationale', 'Changed why.'],
    ['claim.update', 'Updated claim C-002: statement, confidence high', 'Changed the wording.'],
    ['claim.update', 'Updated claim C-002: verification human_verified', 'Decided this still holds.'],
    ['claim.update', 'Set verification of C-002 to human_verified', 'Decided this still holds.'],
    ['claim.update', 'Set verification of C-002 to disputed; status disputed', 'Said sources disagree.'],
    ['claim.update', 'Updated claim C-002: status rejected', 'Ruled this out.'],
    ['investigation.update', 'Updated question INV-0001: status', ''],
    ['investigation.unarchive', 'Unarchived question INV-0002 (status investigating)', ''],
    ['question.defer', 'Deferred question Q-004', ''],
    ['question.resolve', 'Resolved question Q-001: The billing mistake.', 'The billing mistake.'],
    ['evidence.delete', 'Deleted source E-003 "Old call log"', '"Old call log"'],
    // Old wording.
    ['active.set', 'Set active investigation to INV-0001', ''],
    ['active.set', 'Made INV-0001 the active investigation for C:\\work', ''],
    ['active.clear', 'Cleared active investigation (was INV-0001)', ''],
    ['investigation.create', 'Created investigation INV-0001 "Why did calls go up?"', ''],
    ['investigation.create', 'Created investigation INV-0003 "Other"', ''],
    ['conclusion.update', 'Updated conclusion of INV-0001 (confidence high)', ''],
    ['evidence.add', 'Added evidence E-005 "Call log"', ''],
    ['claim.update', 'Updated claim C-001: statement, verification; verification reset to ai_generated after AI edit', `Changed the wording. ${NEEDS_CHECKING_AGAIN}`],
    ['hypothesis.update', 'Updated hypothesis H-002: rationale, confidence high, verification; verification reset to ai_generated after AI edit', `Changed why. ${NEEDS_CHECKING_AGAIN}`],
    ['hypothesis.update', 'Updated hypothesis H-004: verification human_verified', 'Decided this still holds.'],
    ['evidence.update', 'Updated evidence E-003: verification human_verified', 'Checked this.'],
    ['claim.update', 'Updated claim C-001: status disputed, verification disputed', 'Said sources disagree.'],
    ['hypothesis.update', 'Set verification of H-001 to source_verified; status supported', 'Said this turned out right.'],
    ['evidence.update', 'Updated evidence E-001: source_ref, raw_detail, metadata', 'Changed where to find it, the exact words or numbers, extra details.'],
    // An action nobody knows, with any summary.
    ['x.y', 'Frobnicated claim C-001 (status active)', ''],
    ['x.y', 'anything at all', ''],
    // A note an AI wrote for the next reader.
    ['checkpoint', NOTE, NOTE],
  ]

  for (const [action, summary, expected] of shapes) {
    it(`${action} "${summary}" gives "${expected}"`, () => {
      expect(activitySummary(at(action, 'T', summary))).toBe(expected)
    })
  }

  it('never returns an engine word or the stored text for any of these', () => {
    const ENGINE_WORD = /ai_generated|human_verified|confidence|status|investigation|claim|evidence|hypothesis|superseded|deferred|resolved/i
    for (const [action, summary] of shapes) {
      const out = activitySummary(at(action, 'T', summary))
      expect(out, summary).not.toMatch(ENGINE_WORD)
      // A checkpoint is a person's or an AI's own note, shown as written.
      if (action !== 'checkpoint') expect(out, summary).not.toBe(summary)
    }
  })

  it('says an AI edit to a checked answer needs checking again, without the engine words', () => {
    const out = activitySummary(at('conclusion.update', 'INV-0001', 'Updated conclusion of INV-0001 (confidence high); verification reset to ai_generated after AI edit'))
    expect(out).toBe(NEEDS_CHECKING_AGAIN)
    expect(out).not.toContain('confidence')
    expect(out).not.toContain('ai_generated')
  })

  it('returns a checkpoint note as written', () => {
    expect(activitySummary(at('checkpoint', 'INV-0001', 'Added C-003 (E-007).'))).toBe('Added C-003 (E-007).')
  })

  it('tells the check of the answer apart from the edit before it', () => {
    const ctx = { conclusionReviewedAt: '2026-10-27T03:29:47.051Z' }
    const edit = at('conclusion.update', 'INV-0001', 'Updated conclusion of INV-0001 (confidence high)', ai, { created_at: '2026-10-27T03:20:00.000Z' })
    const check = at('conclusion.update', 'INV-0001', 'Updated conclusion of INV-0001 (confidence high)', you, { created_at: ctx.conclusionReviewedAt })
    expect(activitySummary(edit, ctx)).toBe('')
    expect(activitySummary(check, ctx)).toBe('Checked the answer.')
    expect(activitySummary(check)).toBe('')
  })

  // Suspected source defect: FIELD_WORD.title is "the question", so a source whose name changed reads "Changed the question."
  it('does not call a source name the question', () => {
    expect(activitySummary(at('evidence.update', 'E-004', 'Updated evidence E-004: title'))).not.toContain('the question')
  })
})
