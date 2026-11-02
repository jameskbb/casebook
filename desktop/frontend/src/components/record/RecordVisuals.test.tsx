import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { claims, detail, evidence, hypotheses, questions } from '../../test/fixtures'
import { claimWith } from '../../test/fixtures.detail'
import { evidenceWith, question } from '../../test/fixtures.records'
import { IMPORTED_TITLE } from './ImportedTag'
import { EvidenceRow } from '../detail/EvidenceRow'
import { QuestionRow } from '../detail/QuestionRow'
import { RecordRow } from '../detail/RecordRow'
import { EvidenceChip } from './EvidenceChip'
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
  it('renders BadgeCheck with Human verified', () => {
    const { container } = render(<VerificationBadge verification="human_verified" />)
    expect(screen.getByText('Human verified')).toBeInTheDocument()
    expect(container.querySelector('.lucide-badge-check')).not.toBeNull()
  })

  it('renders BadgeCheck with Source verified', () => {
    const { container } = render(<VerificationBadge verification="source_verified" />)
    expect(screen.getByText('Source verified')).toBeInTheDocument()
    expect(container.querySelector('.lucide-badge-check')).not.toBeNull()
  })

  it('renders Bot with AI generated', () => {
    const { container } = render(<VerificationBadge verification="ai_generated" />)
    expect(screen.getByText('AI generated')).toBeInTheDocument()
    expect(container.querySelector('.lucide-bot')).not.toBeNull()
  })

  it('renders CircleAlert with Disputed', () => {
    const { container } = render(<VerificationBadge verification="disputed" />)
    expect(screen.getByText('Disputed')).toBeInTheDocument()
    expect(container.querySelector('.lucide-circle-alert')).not.toBeNull()
  })
})

describe('RecordStatusLabel', () => {
  it('names every record status', () => {
    const cases: [Parameters<typeof RecordStatusLabel>[0], string | null][] = [
      [{ kind: 'claim', status: 'active' }, null],
      [{ kind: 'claim', status: 'disputed' }, 'Disputed'],
      [{ kind: 'claim', status: 'superseded', supersededBy: 'C-002' }, 'Superseded by C-002'],
      [{ kind: 'claim', status: 'rejected' }, 'Rejected'],
      [{ kind: 'hypothesis', status: 'open' }, 'Open'],
      [{ kind: 'hypothesis', status: 'supported' }, 'Supported'],
      [{ kind: 'hypothesis', status: 'disproven' }, 'Disproven'],
      [{ kind: 'hypothesis', status: 'superseded', supersededBy: 'H-004' }, 'Superseded by H-004'],
      [{ kind: 'question', status: 'open' }, null],
      [{ kind: 'question', status: 'resolved' }, 'Resolved'],
      [{ kind: 'question', status: 'deferred' }, 'Deferred'],
    ]
    for (const [props, text] of cases) {
      const { container, unmount } = render(<RecordStatusLabel {...props} />)
      if (text === null) expect(container.textContent).toBe('')
      else expect(container.textContent).toBe(text)
      unmount()
    }
  })
})

describe('EvidenceChip', () => {
  it('names a supporting evidence chip on a claim row', () => {
    render(<EvidenceChip target="E-001" relation="supports" linkKind="claim" text="Service chart default log level set to debug" />)
    const chip = screen.getByLabelText('E-001 supports this: Service chart default log level set to debug')
    expect(chip).toHaveAttribute('title', 'E-001 supports this: Service chart default log level set to debug')
    expect(chip.tagName).toBe('SPAN')
    expect(chip.className).toContain('chip-supports')
  })

  it('marks a contradicting chip with Minus', () => {
    render(<EvidenceChip target="E-003" relation="contradicts" linkKind="claim" text="214 services" />)
    const chip = screen.getByLabelText('E-003 contradicts this: 214 services')
    expect(chip.className).toContain('chip-contradicts')
    expect(chip.querySelector('.lucide-minus')).not.toBeNull()
  })

  it('uses the dashed hypothesis outline for hypothesis links', () => {
    render(<EvidenceChip target="E-002" relation="supports" linkKind="hypothesis" text="Docs" />)
    expect(screen.getByLabelText('E-002 supports this: Docs').className).toContain('chip-hypothesis')
  })

  it('reads from the evidence side on evidence rows', () => {
    render(
      <EvidenceChip target="H-002" relation="contradicts" linkKind="hypothesis" placement="evidence" text="Data transfer" />,
    )
    expect(screen.getByLabelText('Contradicts H-002: Data transfer')).toBeInTheDocument()
  })

  it('is a button that opens its ref only when onOpen is passed', async () => {
    const onOpen = vi.fn()
    const user = userEvent.setup()
    render(<EvidenceChip target="E-001" relation="supports" linkKind="claim" text="Gate" onOpen={onOpen} />)
    await user.click(screen.getByRole('button', { name: 'E-001 supports this: Gate' }))
    expect(onOpen).toHaveBeenCalledWith('E-001')
  })
})

describe('RecordRow', () => {
  it('shows a claim with its badge, statement, long confidence label and chips', () => {
    const { container } = render(<RecordRow kind="claim" record={byRef(claims, 'C-001')} />)
    const row = container.querySelector('[data-ref="C-001"]')!
    expect(row).not.toBeNull()
    expect(row.className).toContain('kind-rule-claim')
    expect(within(row as HTMLElement).getByText('Human verified')).toBeInTheDocument()
    expect(within(row as HTMLElement).getByText('High confidence')).toBeInTheDocument()
    expect(
      within(row as HTMLElement).getByLabelText('E-001 supports this: Service chart default log level set to debug'),
    ).toBeInTheDocument()
    expect(screen.queryByRole('button')).not.toBeInTheDocument()
  })

  it('shows a disputed claim with the word Disputed and the hatched rule', () => {
    const { container } = render(<RecordRow kind="claim" record={byRef(claims, 'C-004')} />)
    const row = container.querySelector('[data-ref="C-004"]')!
    expect(row.className).toContain('kind-rule-disputed')
    expect(within(row as HTMLElement).getAllByText('Disputed')).toHaveLength(1)
  })

  it('shows Disputed for a claim whose verification alone is disputed', () => {
    const { container } = render(
      <RecordRow kind="claim" record={claimWith({ ref: 'C-009', status: 'active', verification: 'disputed' })} />,
    )
    const row = container.querySelector('[data-ref="C-009"]')!
    expect(row.className).toContain('kind-rule-disputed')
    expect(within(row as HTMLElement).getByText('Disputed')).toBeInTheDocument()
  })

  it('never shows two contradicting marks on a row (ADR-0013)', () => {
    const { container } = render(
      <>
        <RecordRow kind="claim" record={claimWith({ ref: 'C-010', status: 'active', verification: 'superseded' })} />
        <RecordRow kind="claim" record={claimWith({ ref: 'C-011', status: 'disputed', verification: 'human_verified' })} />
      </>,
    )
    const stray = container.querySelector('[data-ref="C-010"]') as HTMLElement
    expect(within(stray).queryByText(/Superseded/)).toBeNull()
    const disputed = container.querySelector('[data-ref="C-011"]') as HTMLElement
    expect(within(disputed).getAllByText('Disputed')).toHaveLength(1)
    expect(within(disputed).queryByText('Human verified')).toBeNull()
  })

  it('strikes through a superseded claim and names its replacement', () => {
    const { container } = render(<RecordRow kind="claim" record={byRef(claims, 'C-003')} />)
    const row = container.querySelector('[data-ref="C-003"]')!
    expect(row.className).toContain('kind-rule-retired')
    expect(within(row as HTMLElement).getByText('Superseded by C-002')).toBeInTheDocument()
    expect(row.querySelector('.record-statement')?.className).toContain('is-retired')
  })

  it('shows an open hypothesis with the dashed rule and the label Open', () => {
    const { container } = render(<RecordRow kind="hypothesis" record={byRef(hypotheses, 'H-001')} />)
    const row = container.querySelector('[data-ref="H-001"]')!
    expect(row.className).toContain('kind-rule-hypothesis-open')
    expect(within(row as HTMLElement).getByText('Open')).toBeInTheDocument()
    expect(within(row as HTMLElement).getByText('Medium confidence')).toBeInTheDocument()
  })

  it('shows a supported hypothesis with CircleCheck and Supported', () => {
    const { container } = render(<RecordRow kind="hypothesis" record={byRef(hypotheses, 'H-003')} />)
    const row = container.querySelector('[data-ref="H-003"]')!
    expect(row.className).toContain('kind-rule-hypothesis-supported')
    expect(within(row as HTMLElement).getByText('Supported')).toBeInTheDocument()
    expect(row.querySelector('.lucide-circle-check')).not.toBeNull()
  })

  it('explains a ruled-out hypothesis with its contradicting evidence instead of a confidence', () => {
    const h = byRef(hypotheses, 'H-002')
    const title = h.links[0].evidence_title
    const { container } = render(<RecordRow kind="hypothesis" record={h} />)
    const row = container.querySelector('[data-ref="H-002"]') as HTMLElement
    expect(row.querySelector('.record-ruled-out-by')).toHaveTextContent(`Ruled out by E-003: ${title}`)
    expect(within(row).queryByText('Low confidence')).toBeNull()
    expect(within(row).getByText('Disproven')).toBeInTheDocument()
    expect(row.querySelector('.record-statement')!.className).toContain('is-retired')
  })

  it('names the first contradicting evidence on a rejected claim, and keeps the meter without one', () => {
    const contra = byRef(hypotheses, 'H-002').links[0]
    const support = byRef(hypotheses, 'H-003').links[0]
    const { container } = render(
      <>
        <RecordRow kind="claim" record={claimWith({ ref: 'C-020', status: 'rejected', confidence: 'high', links: [support, contra] })} />
        <RecordRow kind="hypothesis" record={{ ...byRef(hypotheses, 'H-002'), ref: 'H-020', links: [] }} />
      </>,
    )
    const rejected = container.querySelector('[data-ref="C-020"]') as HTMLElement
    expect(rejected.querySelector('.record-ruled-out-by')).toHaveTextContent(`Ruled out by ${contra.evidence_ref}: ${contra.evidence_title}`)
    expect(within(rejected).queryByText('High confidence')).toBeNull()
    const bare = container.querySelector('[data-ref="H-020"]') as HTMLElement
    expect(bare.querySelector('.record-ruled-out-by')).toBeNull()
    expect(within(bare).getByText('Low confidence')).toBeInTheDocument()
  })

  it('keeps the meter on a superseded claim', () => {
    const { container } = render(<RecordRow kind="claim" record={claimWith({ ref: 'C-021', status: 'superseded', confidence: 'medium', links: [byRef(hypotheses, 'H-002').links[0]] })} />)
    expect(container.querySelector('.record-ruled-out-by')).toBeNull()
    expect(screen.getByText('Medium confidence')).toBeInTheDocument()
  })

  it('stops a compact row after the statement and still opens the record', async () => {
    const onOpen = vi.fn()
    const user = userEvent.setup()
    const { container } = render(<RecordRow kind="hypothesis" record={byRef(hypotheses, 'H-001')} onOpen={onOpen} compact />)
    expect(container.querySelector('.record-foot')).toBeNull()
    expect(screen.getByText('Open')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: /^Open hypothesis H-001:/ }))
    expect(onOpen).toHaveBeenLastCalledWith('H-001')
  })

  it('opens the record and its chips when onOpen is passed', async () => {
    const onOpen = vi.fn()
    const user = userEvent.setup()
    render(<RecordRow kind="claim" record={byRef(claims, 'C-001')} onOpen={onOpen} />)
    await user.click(screen.getByRole('button', { name: /^Open claim C-001/ }))
    expect(onOpen).toHaveBeenLastCalledWith('C-001')
    await user.click(screen.getByRole('button', { name: 'E-004 supports this: Head of platform: debug was requested for payments only' }))
    expect(onOpen).toHaveBeenLastCalledWith('E-004')
  })
})

describe('EvidenceRow', () => {
  it('shows title, clamped summary, source meta and what it supports and contradicts', () => {
    const e3 = byRef(evidence, 'E-003')
    const { container } = render(<EvidenceRow evidence={e3} detail={detail} />)
    const row = container.querySelector('[data-ref="E-003"]') as HTMLElement
    expect(row.className).toContain('kind-rule-evidence')
    expect(within(row).getByText('Log ingestion is 71% of the September increase')).toBeInTheDocument()
    expect(row.querySelector('.evidence-summary')?.className).toContain('is-clamped')
    expect(within(row).getByText('SQL query')).toBeInTheDocument()
    expect(within(row).getByText('Billing export query')).toBeInTheDocument()
    expect(within(row).getByText('Supports')).toBeInTheDocument()
    expect(within(row).getByText('Contradicts')).toBeInTheDocument()
    expect(
      within(row).getByLabelText('Supports C-002: Log ingestion, billed per GB, is $96,400 of the $135,800 increase; compute and network are flat.'),
    ).toBeInTheDocument()
    expect(
      within(row).getByLabelText('Contradicts H-002: Data transfer from the region move doubled the network line.'),
    ).toBeInTheDocument()
  })

  it('omits an empty source label', () => {
    const e = { ...byRef(evidence, 'E-001'), source_label: '', claim_links: [], hypothesis_links: [] }
    const { container } = render(<EvidenceRow evidence={e} detail={detail} />)
    const meta = container.querySelector('.evidence-meta') as HTMLElement
    expect(meta.children).toHaveLength(1)
    expect(meta).toHaveTextContent('Code')
    expect(screen.queryByText('Supports')).not.toBeInTheDocument()
  })
})

describe('QuestionRow', () => {
  it('shows an open question focusable by script', () => {
    const { container } = render(<QuestionRow question={byRef(questions, 'Q-001')} />)
    const row = container.querySelector('[data-ref="Q-001"]') as HTMLElement
    expect(row).toHaveAttribute('tabindex', '-1')
    expect(row.className).toContain('kind-rule-question')
    expect(
      within(row).getByText('Can every service except payments go back to info level today?'),
    ).toBeInTheDocument()
  })

  it('shows a resolved question with Resolved and its resolution', () => {
    const { container } = render(<QuestionRow question={byRef(questions, 'Q-002')} />)
    const row = container.querySelector('[data-ref="Q-002"]') as HTMLElement
    expect(row.className).toContain('kind-rule-retired')
    expect(within(row).getByText('Resolved')).toBeInTheDocument()
    expect(
      within(row).getByText('Platform shared services (CC-4100), so no product team saw the jump in its own showback (see E-003).'),
    ).toBeInTheDocument()
    expect(row.querySelector('.lucide-circle-check')).not.toBeNull()
  })

  it('shows a deferred question with Deferred and its note', () => {
    const { container } = render(<QuestionRow question={byRef(questions, 'Q-003')} />)
    const row = container.querySelector('[data-ref="Q-003"]') as HTMLElement
    expect(within(row).getByText('Deferred')).toBeInTheDocument()
    expect(within(row).getByText('Waiting for the quarter close on October 10.')).toBeInTheDocument()
    expect(row.querySelector('.lucide-circle-pause')).not.toBeNull()
  })

  it('renders an actions slot when given', () => {
    render(<QuestionRow question={byRef(questions, 'Q-001')} actions={<button type="button">Resolve</button>} />)
    expect(screen.getByRole('button', { name: 'Resolve' })).toBeInTheDocument()
  })
})

describe('Imported marker (AR-09)', () => {
  it('marks imported claims, evidence and questions, and only those', () => {
    const { container } = render(
      <>
        <RecordRow kind="claim" record={claimWith({ ref: 'C-020', imported: true })} />
        <RecordRow kind="claim" record={claimWith({ ref: 'C-021' })} />
        <EvidenceRow evidence={{ ...evidenceWith('E-001', { imported: true }), ref: 'E-020' }} detail={detail} />
        <QuestionRow question={question({ ref: 'Q-020', imported: true })} />
      </>,
    )
    for (const ref of ['C-020', 'E-020', 'Q-020']) {
      const row = container.querySelector(`[data-ref="${ref}"]`) as HTMLElement
      expect(within(row).getByText('Imported')).toHaveAttribute('title', IMPORTED_TITLE)
    }
    expect(within(container.querySelector('[data-ref="C-021"]') as HTMLElement).queryByText('Imported')).toBeNull()
  })
})
