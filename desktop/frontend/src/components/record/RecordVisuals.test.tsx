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
import { RecordStatusLabel } from './StatusLabel'
import { VerificationBadge } from './VerificationBadge'

const byRef = <T extends { ref: string }>(list: T[], ref: string) => list.find((x) => x.ref === ref)!

describe('kindRuleClass', () => {
  it('maps each kind and state to its rule', () => {
    expect(kindRuleClass('evidence')).toContain('kind-rule-evidence')
    expect(kindRuleClass('claim', { status: 'active' })).toContain('kind-rule-claim')
    expect(kindRuleClass('hypothesis', { status: 'open' })).toContain('kind-rule-hypothesis-open')
    expect(kindRuleClass('hypothesis', { status: 'supported' })).toContain('kind-rule-hypothesis-supported')
    expect(kindRuleClass('question', { status: 'open' })).toContain('kind-rule-question')
    expect(kindRuleClass('claim', { status: 'disputed' })).toContain('kind-rule-disputed')
    expect(kindRuleClass('claim', { status: 'active', verification: 'disputed' })).toContain('kind-rule-disputed')
    expect(kindRuleClass('hypothesis', { status: 'open', verification: 'disputed' })).toContain('kind-rule-disputed')
    expect(kindRuleClass('claim', { status: 'superseded' })).toContain('kind-rule-retired')
    expect(kindRuleClass('claim', { status: 'rejected' })).toContain('kind-rule-retired')
    expect(kindRuleClass('hypothesis', { status: 'disproven' })).toContain('kind-rule-retired')
    expect(kindRuleClass('question', { status: 'resolved' })).toContain('kind-rule-retired')
    expect(kindRuleClass('question', { status: 'deferred' })).toContain('kind-rule-retired')
  })
})

describe('VerificationBadge', () => {
  it('renders BadgeCheck with You checked this', () => {
    const { container } = render(<VerificationBadge verification="human_verified" />)
    expect(screen.getByText('You checked this')).toBeInTheDocument()
    expect(container.querySelector('.lucide-badge-check')).not.toBeNull()
  })

  it('renders BadgeCheck with AI checked the source', () => {
    const { container } = render(<VerificationBadge verification="source_verified" />)
    expect(screen.getByText('AI checked the source')).toBeInTheDocument()
    expect(container.querySelector('.lucide-badge-check')).not.toBeNull()
  })

  it('renders Bot with Not checked', () => {
    const { container } = render(<VerificationBadge verification="ai_generated" />)
    expect(screen.getByText('Not checked')).toBeInTheDocument()
    expect(container.querySelector('.lucide-bot')).not.toBeNull()
  })

  it('renders CircleAlert with Disputed', () => {
    const { container } = render(<VerificationBadge verification="disputed" />)
    expect(screen.getByText('Sources disagree')).toBeInTheDocument()
    expect(container.querySelector('.lucide-circle-alert')).not.toBeNull()
  })
})

describe('RecordStatusLabel', () => {
  it('names every record status', () => {
    const cases: [Parameters<typeof RecordStatusLabel>[0], string | null][] = [
      [{ kind: 'claim', status: 'active' }, null],
      [{ kind: 'claim', status: 'disputed' }, 'Sources disagree'],
      [{ kind: 'claim', status: 'superseded', supersededBy: 'C-002' }, 'Replaced by C-002'],
      [{ kind: 'claim', status: 'rejected' }, 'Ruled out'],
      [{ kind: 'hypothesis', status: 'open' }, 'Not tested yet'],
      [{ kind: 'hypothesis', status: 'supported' }, 'Supported'],
      [{ kind: 'hypothesis', status: 'disproven' }, 'Ruled out'],
      [{ kind: 'hypothesis', status: 'superseded', supersededBy: 'H-004' }, 'Replaced by H-004'],
      [{ kind: 'question', status: 'open' }, null],
      [{ kind: 'question', status: 'resolved' }, 'Answered'],
      [{ kind: 'question', status: 'deferred' }, 'Set aside'],
    ]
    for (const [props, text] of cases) {
      const { container, unmount } = render(<RecordStatusLabel {...props} />)
      if (text === null) expect(container.textContent).toBe('')
      else expect(container.textContent).toBe(text)
      unmount()
    }
  })
})

describe('RecordRow', () => {
  it('shows a finding with its badge, statement and nested sources, and no confidence', () => {
    const { container } = render(<RecordRow kind="claim" record={byRef(claims, 'C-001')} />)
    const row = container.querySelector('[data-ref="C-001"]')!
    expect(row).not.toBeNull()
    expect(row.className).toContain('kind-rule-claim')
    expect(within(row as HTMLElement).getByText('You checked this')).toBeInTheDocument()
    expect(within(row as HTMLElement).queryByText(/confidence/i)).toBeNull()
    const sources = within(within(row as HTMLElement).getByRole('list', { name: 'Sources for this finding' })).getAllByRole('listitem')
    expect(sources).toHaveLength(2)
    expect(sources[0]).toHaveTextContent('Supports this: E-001 Notice dated October 4: 1,900 customers were charged twice')
    // Without the question's sources to look up, a nested source shows its title only.
    expect(sources[0].querySelector('.badge')).toBeNull()
    expect(screen.queryByRole('button')).not.toBeInTheDocument()
  })

  it('shows each nested source with where it came from and its own check label, and a minus when it contradicts', () => {
    const sources = new Map(evidence.map((e) => [e.ref, e]))
    const { container } = render(
      <>
        <RecordRow kind="claim" record={byRef(claims, 'C-001')} sources={sources} />
        <RecordRow kind="claim" record={byRef(claims, 'C-004')} sources={sources} />
        <RecordRow kind="hypothesis" record={{ ...byRef(hypotheses, 'H-003'), ref: 'H-030' }} sources={new Map([['E-002', { ...byRef(evidence, 'E-002'), source_label: '' }]])} />
      </>,
    )
    const c1 = container.querySelector('[data-ref="C-001"]') as HTMLElement
    const [e1, e4] = within(c1).getAllByRole('listitem')
    expect(e1).toHaveTextContent('Email from the billing team')
    expect(within(e1).getByText('Not checked')).toBeInTheDocument()
    expect(within(e4).getByText('You checked this')).toBeInTheDocument()
    expect(e1.querySelector('.lucide-plus')).not.toBeNull()
    const against = (container.querySelector('[data-ref="C-004"]') as HTMLElement).querySelector('.finding-source') as HTMLElement
    expect(against).toHaveClass('is-against')
    expect(against.querySelector('.lucide-minus')).not.toBeNull()
    expect(against).toHaveTextContent('Contradicts this:')
    // With no "where it came from", the kind of source stands in.
    expect(container.querySelector('[data-ref="H-030"] .finding-source-from')).toHaveTextContent('Document')
  })

  it('shows a disputed claim with the word Disputed and the hatched rule', () => {
    const { container } = render(<RecordRow kind="claim" record={byRef(claims, 'C-004')} />)
    const row = container.querySelector('[data-ref="C-004"]')!
    expect(row.className).toContain('kind-rule-disputed')
    expect(within(row as HTMLElement).getAllByText('Sources disagree')).toHaveLength(1)
  })

  it('shows Disputed for a claim whose verification alone is disputed', () => {
    const { container } = render(
      <RecordRow kind="claim" record={claimWith({ ref: 'C-009', status: 'active', verification: 'disputed' })} />,
    )
    const row = container.querySelector('[data-ref="C-009"]')!
    expect(row.className).toContain('kind-rule-disputed')
    expect(within(row as HTMLElement).getByText('Sources disagree')).toBeInTheDocument()
  })

  it('never shows two contradicting marks on a row', () => {
    const { container } = render(
      <>
        <RecordRow kind="claim" record={claimWith({ ref: 'C-010', status: 'active', verification: 'superseded' })} />
        <RecordRow kind="claim" record={claimWith({ ref: 'C-011', status: 'disputed', verification: 'human_verified' })} />
      </>,
    )
    const stray = container.querySelector('[data-ref="C-010"]') as HTMLElement
    expect(within(stray).queryByText(/Superseded/)).toBeNull()
    const disputed = container.querySelector('[data-ref="C-011"]') as HTMLElement
    expect(within(disputed).getAllByText('Sources disagree')).toHaveLength(1)
    expect(within(disputed).queryByText('You checked this')).toBeNull()
  })

  it('strikes through a superseded claim and names its replacement', () => {
    const { container } = render(<RecordRow kind="claim" record={byRef(claims, 'C-003')} />)
    const row = container.querySelector('[data-ref="C-003"]')!
    expect(row.className).toContain('kind-rule-retired')
    expect(row.querySelector('.record-status')).toHaveTextContent('Replaced by C-002')
    expect(within(row as HTMLElement).getByText('by C-002')).toHaveClass('record-id')
    expect(row.querySelector('.record-statement')?.className).toContain('is-retired')
  })

  it('shows an open hypothesis with the dashed rule and the label Not tested yet', () => {
    const { container } = render(<RecordRow kind="hypothesis" record={byRef(hypotheses, 'H-001')} />)
    const row = container.querySelector('[data-ref="H-001"]')!
    expect(row.className).toContain('kind-rule-hypothesis-open')
    expect(within(row as HTMLElement).getByText('Not tested yet')).toBeInTheDocument()
    expect(within(row as HTMLElement).queryByText(/confidence/i)).toBeNull()
  })

  it('shows a supported hypothesis with CircleCheck and Supported', () => {
    const { container } = render(<RecordRow kind="hypothesis" record={byRef(hypotheses, 'H-003')} />)
    const row = container.querySelector('[data-ref="H-003"]')!
    expect(row.className).toContain('kind-rule-hypothesis-supported')
    expect(within(row as HTMLElement).getByText('Supported')).toBeInTheDocument()
    expect(row.querySelector('.lucide-circle-check')).not.toBeNull()
  })

  it('shows a ruled-out finding with the source that contradicts it nested under it, and no confidence', () => {
    const h = byRef(hypotheses, 'H-002')
    const title = h.links[0].evidence_title
    const { container } = render(<RecordRow kind="hypothesis" record={h} />)
    const row = container.querySelector('[data-ref="H-002"]') as HTMLElement
    expect(within(row).getByRole('listitem')).toHaveTextContent(`Contradicts this: E-003 ${title}`)
    expect(within(row).queryByText('Low confidence')).toBeNull()
    expect(within(row).getByText('Ruled out')).toBeInTheDocument()
    expect(row.querySelector('.record-statement')!.className).toContain('is-retired')
  })

  it('lists both kinds of source on a ruled-out finding, and no list without sources', () => {
    const contra = byRef(hypotheses, 'H-002').links[0]
    const support = byRef(hypotheses, 'H-003').links[0]
    const { container } = render(
      <>
        <RecordRow kind="claim" record={claimWith({ ref: 'C-020', status: 'rejected', confidence: 'high', links: [support, contra] })} />
        <RecordRow kind="hypothesis" record={{ ...byRef(hypotheses, 'H-002'), ref: 'H-020', links: [] }} />
      </>,
    )
    const rejected = container.querySelector('[data-ref="C-020"]') as HTMLElement
    expect(within(rejected).getAllByRole('listitem').map((li) => li.querySelector('.finding-source')?.classList.contains('is-against'))).toEqual([false, true])
    expect(within(rejected).queryByText('High confidence')).toBeNull()
    const bare = container.querySelector('[data-ref="H-020"]') as HTMLElement
    expect(within(bare).queryByRole('list')).toBeNull()
  })

  it('opens the finding and each nested source when onOpen is passed', async () => {
    const onOpen = vi.fn()
    const user = userEvent.setup()
    render(<RecordRow kind="claim" record={byRef(claims, 'C-001')} onOpen={onOpen} />)
    await user.click(screen.getByRole('button', { name: /^Open finding C-001/ }))
    expect(onOpen).toHaveBeenLastCalledWith('C-001')
    await user.click(screen.getByRole('button', { name: 'Supports this: E-004 Head of billing: the mistake was on October 3 only' }))
    expect(onOpen).toHaveBeenLastCalledWith('E-004')
  })
})

describe('EvidenceRow', () => {
  it('shows the title, clamped summary, kind of source and where it came from', () => {
    const e3 = byRef(evidence, 'E-003')
    const { container } = render(<EvidenceRow evidence={e3} />)
    const row = container.querySelector('[data-ref="E-003"]') as HTMLElement
    expect(row.className).toContain('kind-rule-evidence')
    expect(within(row).getByText('Calls about a bill: 400 in September, 1,070 in October')).toBeInTheDocument()
    expect(row.querySelector('.evidence-summary')?.className).toContain('is-clamped')
    expect(within(row).getByText('Data query')).toBeInTheDocument()
    expect(within(row).getByText('Call log')).toBeInTheDocument()
    // The row has one control at most (none here): it never lists findings.
    expect(within(row).queryByRole('button')).toBeNull()
  })

  it('omits an empty source label', () => {
    const e = { ...byRef(evidence, 'E-001'), source_label: '', claim_links: [], hypothesis_links: [] }
    const { container } = render(<EvidenceRow evidence={e} />)
    const meta = container.querySelector('.evidence-meta') as HTMLElement
    expect(meta.children).toHaveLength(1)
    expect(meta).toHaveTextContent('File')
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

  it('shows an answered follow-up question with Answered and its answer', () => {
    const { container } = render(<QuestionRow question={byRef(questions, 'Q-002')} />)
    const row = container.querySelector('[data-ref="Q-002"]') as HTMLElement
    expect(row.className).toContain('kind-rule-retired')
    expect(within(row).getByText('Answered')).toBeInTheDocument()
    expect(
      within(row).getByText('Yes. Both go to the sales line, which is why the total went up (see E-003).'),
    ).toBeInTheDocument()
    expect(row.querySelector('.lucide-circle-check')).not.toBeNull()
  })

  it('shows a follow-up question set aside with Set aside and its note', () => {
    const { container } = render(<QuestionRow question={byRef(questions, 'Q-003')} />)
    const row = container.querySelector('[data-ref="Q-003"]') as HTMLElement
    expect(within(row).getByText('Set aside')).toBeInTheDocument()
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
