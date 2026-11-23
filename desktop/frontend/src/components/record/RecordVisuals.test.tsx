import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { claims, evidence, hypotheses, questions } from '../../test/fixtures'
import { claimWith } from '../../test/fixtures.detail'
import { evidenceWith, question } from '../../test/fixtures.records'
import { IMPORTED_TITLE } from './ImportedTag'
import { EvidenceRow } from '../detail/EvidenceRow'
import { QuestionRow } from '../detail/QuestionRow'
import { RecordRow } from '../detail/RecordRow'
import { kindRuleClass } from './KindRule'
import { FindingLabel, LATER } from './StatusLabel'
import { VerificationBadge } from './VerificationBadge'

const byRef = <T extends { ref: string }>(list: T[], ref: string) => list.find((x) => x.ref === ref)!

describe('kindRuleClass', () => {
  it('maps each kind and state to its rule', () => {
    expect(kindRuleClass('evidence')).toContain('kind-rule-evidence')
    expect(kindRuleClass('claim', { status: 'active' })).toContain('kind-rule-claim')
    // Not tested yet is neutral, not the green of a finding that holds.
    expect(kindRuleClass('hypothesis', { status: 'open' })).toContain('kind-rule-untested')
    expect(kindRuleClass('hypothesis', { status: 'supported' })).toContain('kind-rule-claim')
    expect(kindRuleClass('question', { status: 'open' })).toContain('kind-rule-question')
    expect(kindRuleClass('claim', { status: 'disputed' })).toContain('kind-rule-disputed')
    expect(kindRuleClass('claim', { status: 'active', verification: 'disputed' })).toContain('kind-rule-disputed')
    expect(kindRuleClass('hypothesis', { status: 'open', verification: 'disputed' })).toContain('kind-rule-disputed')
    // A source never takes the hatched rule.
    expect(kindRuleClass('evidence', { verification: 'disputed' })).toContain('kind-rule-evidence')
    expect(kindRuleClass('claim', { status: 'superseded' })).toContain('kind-rule-retired')
    expect(kindRuleClass('claim', { status: 'rejected' })).toContain('kind-rule-retired')
    expect(kindRuleClass('hypothesis', { status: 'disproven' })).toContain('kind-rule-retired')
    expect(kindRuleClass('question', { status: 'resolved' })).toContain('kind-rule-retired')
    expect(kindRuleClass('question', { status: 'deferred' })).toContain('kind-rule-retired')
  })
})

describe('VerificationBadge', () => {
  it('shows who checked a source, with an icon beside the words', () => {
    const cases = [
      ['human_verified', 'You checked this', '.lucide-badge-check'],
      ['source_verified', 'AI checked the source', '.lucide-badge-check'],
      ['ai_generated', 'Not checked', '.lucide-circle-dashed'],
      // Stored by an AI as sources disagree: no person has checked it.
      ['disputed', 'Not checked', '.lucide-circle-dashed'],
    ] as const
    for (const [verification, text, icon] of cases) {
      const { container, unmount } = render(<VerificationBadge verification={verification} />)
      expect(container.textContent).toBe(text)
      expect(container.querySelector(icon)).not.toBeNull()
      unmount()
    }
    const { container } = render(<VerificationBadge verification="superseded" />)
    expect(container.textContent).toBe('Replaced')
  })
})

describe('FindingLabel', () => {
  it('shows one label for every state of a finding', () => {
    const sources = new Map(evidence.map((e) => [e.ref, e]))
    const cases: [string, string][] = [
      ['C-001', '1 of 2 sources checked by you'],
      ['C-002', '0 of 2 sources checked by you'],
      ['C-003', 'Replaced by C-002'],
      ['C-004', 'Sources disagree'],
      ['H-001', 'Not tested yet'],
      ['H-002', 'Ruled out'],
      ['H-003', '0 of 1 source checked by you'],
    ]
    for (const [ref, text] of cases) {
      const finding = ref.startsWith('C') ? { kind: 'claim' as const, record: byRef(claims, ref) } : { kind: 'hypothesis' as const, record: byRef(hypotheses, ref) }
      const { container, unmount } = render(<FindingLabel finding={finding} sources={sources} />)
      expect(container.textContent).toBe(text)
      unmount()
    }
  })

  it('marks a finding green only when every one of its sources is checked by the person', () => {
    const all = new Map(evidence.map((e) => [e.ref, { ...e, verification: 'human_verified' as const }]))
    const { container } = render(<FindingLabel finding={{ kind: 'claim', record: byRef(claims, 'C-002') }} sources={all} />)
    expect(container.textContent).toBe('2 of 2 sources checked by you')
    expect(container.querySelector('.badge-verified .lucide-badge-check')).not.toBeNull()
  })
})

describe('RecordRow', () => {
  it('shows a finding with its one label, statement and nested sources, and nothing about how sure anyone is', () => {
    const sources = new Map(evidence.map((e) => [e.ref, e]))
    const { container } = render(<RecordRow kind="claim" record={byRef(claims, 'C-001')} sources={sources} />)
    const row = container.querySelector('[data-ref="C-001"]') as HTMLElement
    expect(row.className).toContain('kind-rule-claim')
    // Stored as checked by a person; the row says what its sources say.
    expect(row.querySelector('.row-line')).toHaveTextContent(/^C-0011 of 2 sources checked by you$/)
    expect(within(row).queryByText(/confidence/i)).toBeNull()
    const list = within(within(row).getByRole('list', { name: 'Sources for this finding' })).getAllByRole('listitem')
    expect(list).toHaveLength(2)
    expect(list[0]).toHaveTextContent('Backs it: E-001 Notice dated October 4: 1,900 customers were charged twice Not checked')
    expect(list[1]).toHaveTextContent('Backs it: E-004 Head of billing: the mistake was on October 3 only You checked this')
    expect(screen.queryByRole('button')).not.toBeInTheDocument()
  })

  it('says in words which way each source points, with no plus or minus sign', () => {
    const sources = new Map(evidence.map((e) => [e.ref, e]))
    const { container } = render(
      <>
        <RecordRow kind="claim" record={byRef(claims, 'C-001')} sources={sources} />
        <RecordRow kind="claim" record={byRef(claims, 'C-004')} sources={sources} />
      </>,
    )
    const against = (container.querySelector('[data-ref="C-004"]') as HTMLElement).querySelector('.finding-source') as HTMLElement
    expect(against).toHaveClass('is-against')
    expect(against).toHaveTextContent(/^Goes against it:/)
    expect(container.querySelector('.lucide-plus, .lucide-minus')).toBeNull()
    expect(container).not.toHaveTextContent(/Supports|Contradicts/)
    // Where a source came from and what kind it is are not shown.
    expect(container).not.toHaveTextContent('Email from the billing team')
  })

  it('shows a finding a source goes against with Sources disagree and the hatched rule, whatever is stored', () => {
    const against = byRef(claims, 'C-004').links
    const { container } = render(
      <>
        <RecordRow kind="claim" record={byRef(claims, 'C-004')} />
        <RecordRow kind="claim" record={claimWith({ ref: 'C-009', status: 'active', verification: 'disputed' })} />
        <RecordRow kind="claim" record={claimWith({ ref: 'C-010', status: 'active', verification: 'source_verified', links: against })} />
        <RecordRow kind="claim" record={claimWith({ ref: 'C-011', status: 'active', verification: 'human_verified', links: against })} />
      </>,
    )
    for (const ref of ['C-004', 'C-009', 'C-010']) {
      const row = container.querySelector(`[data-ref="${ref}"]`) as HTMLElement
      expect(row.className, ref).toContain('kind-rule-disputed')
      expect(row.querySelectorAll('.row-line .badge'), ref).toHaveLength(1)
      expect(row.querySelector('.row-line .badge'), ref).toHaveTextContent('Sources disagree')
    }
    // The person decided it still holds.
    const decided = container.querySelector('[data-ref="C-011"]') as HTMLElement
    expect(decided.className).not.toContain('kind-rule-disputed')
    expect(decided.querySelector('.row-line')).toHaveTextContent('You decided this holds')
  })

  it('strikes through a replaced finding and names its replacement', () => {
    const { container } = render(<RecordRow kind="claim" record={byRef(claims, 'C-003')} />)
    const row = container.querySelector('[data-ref="C-003"]')!
    expect(row.className).toContain('kind-rule-retired')
    expect(row.querySelector('.record-status')).toHaveTextContent('Replaced by C-002')
    expect(within(row as HTMLElement).getByText('by C-002')).toHaveClass('record-id')
    expect(row.querySelector('.record-statement')?.className).toContain('is-retired')
  })

  it('shows a finding nobody has tested with the neutral rule and one label, Not tested yet', () => {
    const { container } = render(<RecordRow kind="hypothesis" record={{ ...byRef(hypotheses, 'H-001'), verification: 'human_verified' }} />)
    const row = container.querySelector('[data-ref="H-001"]') as HTMLElement
    expect(row.className).toContain('kind-rule-untested')
    expect(row.className).not.toContain('kind-rule-claim')
    expect(row.querySelector('.row-line')).toHaveTextContent(/^H-001Not tested yet$/)
  })

  it('shows a ruled out finding with the source that goes against it nested under it', () => {
    const h = byRef(hypotheses, 'H-002')
    const { container } = render(<RecordRow kind="hypothesis" record={h} />)
    const row = container.querySelector('[data-ref="H-002"]') as HTMLElement
    expect(within(row).getByRole('listitem')).toHaveTextContent(`Goes against it: E-003 ${h.links[0].evidence_title}`)
    expect(row.querySelector('.row-line')).toHaveTextContent(/^H-002Ruled out$/)
    expect(row.querySelector('.record-statement')!.className).toContain('is-retired')
    const bare = render(<RecordRow kind="hypothesis" record={{ ...h, ref: 'H-020', links: [] }} />).container
    expect(within(bare).queryByRole('list')).toBeNull()
  })

  it('opens the finding and each nested source when onOpen is passed', async () => {
    const onOpen = vi.fn()
    const user = userEvent.setup()
    render(<RecordRow kind="claim" record={byRef(claims, 'C-001')} onOpen={onOpen} />)
    await user.click(screen.getByRole('button', { name: /^Open finding C-001/ }))
    expect(onOpen).toHaveBeenLastCalledWith('C-001')
    await user.click(screen.getByRole('button', { name: 'Backs it: E-004 Head of billing: the mistake was on October 3 only' }))
    expect(onOpen).toHaveBeenLastCalledWith('E-004')
  })
})

describe('EvidenceRow', () => {
  it('shows its check label, the title, and what it shows under the title', () => {
    const e3 = byRef(evidence, 'E-003')
    const { container } = render(<EvidenceRow evidence={e3} />)
    const row = container.querySelector('[data-ref="E-003"]') as HTMLElement
    expect(row.className).toContain('kind-rule-evidence')
    expect(within(row).getByText('Not checked')).toBeInTheDocument()
    expect(within(row).getByText('Calls about a bill: 400 in September, 1,070 in October')).toBeInTheDocument()
    const shows = row.querySelector('.evidence-summary')
    expect(shows).toHaveTextContent(e3.summary)
    expect(within(row).getByText('Calls about a bill: 400 in September, 1,070 in October').nextElementSibling).toBe(shows)
    // Where it came from and its kind are not shown.
    expect(row).not.toHaveTextContent('Call log')
    expect(row.querySelector('.evidence-meta')).toBeNull()
    expect(within(row).queryByRole('button')).toBeNull()
  })
})

describe('QuestionRow', () => {
  it('shows an open question focusable by script', () => {
    const { container } = render(<QuestionRow question={byRef(questions, 'Q-001')} />)
    const row = container.querySelector('[data-ref="Q-001"]') as HTMLElement
    expect(row).toHaveAttribute('tabindex', '-1')
    expect(row.className).toContain('kind-rule-question')
    expect(
      within(row).getByText('Have the customers who were charged twice been paid back?'),
    ).toBeInTheDocument()
  })

  it('shows an answered one with its answer and no label', () => {
    const { container } = render(<QuestionRow question={byRef(questions, 'Q-002')} />)
    const row = container.querySelector('[data-ref="Q-002"]') as HTMLElement
    expect(row.className).toContain('kind-rule-retired')
    expect(within(row).queryByText('Answered')).toBeNull()
    expect(row.querySelector('.record-status')).toBeNull()
    expect(row.querySelector('.question-row-text')).toHaveClass('is-closed')
    // The answer names a source by its code: the code hides, and "a source" stands in for it.
    const answer = row.querySelector('.question-row-answer') as HTMLElement
    expect(answer).toHaveTextContent(/^Yes\. Both go to the sales line, which is why the total went up \(see E-003 a source\)\.$/)
    const code = within(answer).getByText('E-003')
    expect(code).toHaveClass('record-id')
    expect(code.nextElementSibling).toHaveClass('codes-off')
    expect(code.nextElementSibling).toHaveTextContent('a source')
  })

  it('shows one left for later with Later and its note', () => {
    const { container } = render(<QuestionRow question={byRef(questions, 'Q-003')} />)
    const row = container.querySelector('[data-ref="Q-003"]') as HTMLElement
    expect(within(row).getByText(LATER)).toBeInTheDocument()
    expect(within(row).queryByText('Set aside')).toBeNull()
    expect(within(row).getByText('Waiting for the refund list on November 10.')).toBeInTheDocument()
    expect(row.querySelector('.lucide-circle-pause')).not.toBeNull()
  })

  it('renders an actions slot when given', () => {
    render(<QuestionRow question={byRef(questions, 'Q-001')} actions={<button type="button">Answer</button>} />)
    expect(screen.getByRole('button', { name: 'Answer' })).toBeInTheDocument()
  })
})

describe('Imported marker', () => {
  it('marks imported claims, evidence and questions, and only those', () => {
    const { container } = render(
      <>
        <RecordRow kind="claim" record={claimWith({ ref: 'C-020', imported: true })} />
        <RecordRow kind="claim" record={claimWith({ ref: 'C-021' })} />
        <EvidenceRow evidence={{ ...evidenceWith('E-001', { imported: true }), ref: 'E-020' }} />
        <QuestionRow question={question({ ref: 'Q-020', imported: true })} />
      </>,
    )
    for (const ref of ['C-020', 'E-020', 'Q-020']) {
      const row = container.querySelector(`[data-ref="${ref}"]`) as HTMLElement
      expect(within(row).getByText('From a file')).toHaveAttribute('title', IMPORTED_TITLE)
    }
    expect(within(container.querySelector('[data-ref="C-021"]') as HTMLElement).queryByText('From a file')).toBeNull()
  })
})
