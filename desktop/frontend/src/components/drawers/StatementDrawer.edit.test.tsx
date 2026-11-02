import type { ReactNode } from 'react'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Api } from '../../api/api'
import { ApiError } from '../../api/errors'
import type { Claim, Hypothesis, InvestigationDetail } from '../../api/types'
import { AppDataProvider } from '../../state/appData'
import { claims, detail, hypotheses } from '../../test/fixtures'
import { claim, hypothesis } from '../../test/fixtures.records'
import { mockApi } from '../../test/mockApi'
import { ToastProvider } from '../ui/Toaster'
import { MORE_OPTIONS_LABEL, StatementDrawer, SUPERSEDED_HINT } from './StatementDrawer'

const mocks = vi.hoisted(() => ({ api: {} as Api }))
vi.mock('../../api/api', () => ({ api: mocks.api }))

const providers = (ui: ReactNode) => (
  <ToastProvider>
    <AppDataProvider>{ui}</AppDataProvider>
  </ToastProvider>
)

const C001 = claims[0]
const C002 = claims[1]
const H001 = hypotheses[0]

function drawerFor(r: Claim | Hypothesis, d: InvestigationDetail = detail, onOpenRef = vi.fn()) {
  d = { ...d, review_bases: [{ ref: r.ref, basis: 'displayed-review-basis' }] }
  return r.ref.startsWith('C') ? (
    <StatementDrawer kind="claim" record={r as Claim} detail={d} onOpenRef={onOpenRef} />
  ) : (
    <StatementDrawer kind="hypothesis" record={r as Hypothesis} detail={d} onOpenRef={onOpenRef} />
  )
}

function renderDrawer(r: Claim | Hypothesis, d: InvestigationDetail = detail) {
  const utils = render(providers(drawerFor(r, d)))
  return { ...utils, rerenderWith: (next: Claim | Hypothesis) => utils.rerender(providers(drawerFor(next, d))) }
}

const group = (name: string) => screen.getByRole('region', { name })

/** Opens More options in the status group and returns the group. */
async function statusSelects(user: ReturnType<typeof userEvent.setup>) {
  const g = group('Status and verification')
  await user.click(within(g).getByRole('button', { name: MORE_OPTIONS_LABEL }))
  return g
}

describe('StatementDrawer statement', () => {
  beforeEach(() => {
    Object.assign(mocks.api, mockApi())
  })

  it('requires acknowledgement of changed sources before personally verifying a live drawer', async () => {
    const user = userEvent.setup()
    const renderSnapshot = (basis: string) => providers(<StatementDrawer kind="claim" record={C002}
      detail={{ ...detail, review_bases: [{ ref: C002.ref, basis }] }} onOpenRef={vi.fn()} />)
    const view = render(renderSnapshot('sources-read'))
    expect(screen.getByRole('button', { name: 'Verify myself' })).toBeEnabled()
    view.rerender(renderSnapshot('sources-changed'))
    expect(screen.getByRole('button', { name: 'Verify myself' })).toBeDisabled()
    expect(mocks.api.reviewStatement).not.toHaveBeenCalled()
    await user.click(screen.getByRole('button', { name: 'Use latest for review' }))
    await user.click(screen.getByRole('button', { name: 'Verify myself' }))
    expect(mocks.api.reviewStatement).toHaveBeenCalledWith(detail.investigation.ref,
      expect.objectContaining({ basis: 'sources-changed', verification: 'human_verified' }))
  })

  it('edits the claim statement inline and saves it without a toast', async () => {
    const user = userEvent.setup()
    renderDrawer(C001)
    await user.click(screen.getByRole('button', { name: 'Edit statement' }))
    const box = screen.getByLabelText('Statement')
    expect(box).toHaveFocus()
    await user.clear(box)
    await user.type(box, 'The log level comes from the chart default.')
    await user.click(screen.getByRole('button', { name: 'Save statement' }))
    expect(mocks.api.updateClaim).toHaveBeenCalledWith('INV-0001', 'C-001', { statement: 'The log level comes from the chart default.' })
    await waitFor(() => expect(screen.queryByLabelText('Statement')).toBeNull())
    expect(screen.queryByRole('status')).toBeNull()
  })

  it('saves a hypothesis statement through updateHypothesis', async () => {
    const user = userEvent.setup()
    renderDrawer(H001)
    await user.click(screen.getByRole('button', { name: 'Edit statement' }))
    await user.type(screen.getByLabelText('Statement'), ' Maybe.')
    await user.click(screen.getByRole('button', { name: 'Save statement' }))
    expect(mocks.api.updateHypothesis).toHaveBeenCalledWith('INV-0001', 'H-001', { statement: `${H001.statement} Maybe.` })
  })

  it('Cancel restores the view without saving', async () => {
    const user = userEvent.setup()
    renderDrawer(C001)
    await user.click(screen.getByRole('button', { name: 'Edit statement' }))
    await user.type(screen.getByLabelText('Statement'), ' Draft.')
    await user.click(screen.getByRole('button', { name: 'Cancel' }))
    expect(screen.queryByLabelText('Statement')).toBeNull()
    expect(screen.getByText(C001.statement)).toBeInTheDocument()
    expect(mocks.api.updateClaim).not.toHaveBeenCalled()
  })

  it('shows a validation error under the statement', async () => {
    const user = userEvent.setup()
    Object.assign(
      mocks.api,
      mockApi({
        updateClaim: async () => {
          throw new ApiError('validation', 'is required', 'statement')
        },
      }),
    )
    renderDrawer(C001)
    await user.click(screen.getByRole('button', { name: 'Edit statement' }))
    await user.clear(screen.getByLabelText('Statement'))
    await user.type(screen.getByLabelText('Statement'), ' ')
    await user.click(screen.getByRole('button', { name: 'Save statement' }))
    expect(await screen.findByText('is required')).toBeInTheDocument()
    expect(screen.getByLabelText('Statement')).toHaveAttribute('aria-invalid', 'true')
  })
})

describe('StatementDrawer confidence', () => {
  beforeEach(() => {
    Object.assign(mocks.api, mockApi())
  })

  it('enables Save confidence only after a change and sends confidence and confidence_rationale for claims', async () => {
    const user = userEvent.setup()
    renderDrawer(C001)
    const g = group('How sure are we')
    const save = within(g).getByRole('button', { name: 'Save confidence' })
    expect(save).toBeDisabled()
    await user.click(within(g).getByRole('radio', { name: 'Medium' }))
    expect(save).toBeEnabled()
    const why = within(g).getByLabelText('Why this confidence (optional)')
    await user.clear(why)
    await user.type(why, 'Only the code agrees.')
    await user.click(save)
    expect(mocks.api.updateClaim).toHaveBeenCalledWith('INV-0001', 'C-001', {
      confidence: 'medium',
      confidence_rationale: 'Only the code agrees.',
    })
  })

  it('sends confidence and rationale for hypotheses', async () => {
    const user = userEvent.setup()
    renderDrawer(H001)
    const g = group('How sure are we')
    await user.click(within(g).getByRole('radio', { name: 'High' }))
    await user.type(within(g).getByLabelText('Why this confidence (optional)'), 'Two sources.')
    await user.click(within(g).getByRole('button', { name: 'Save confidence' }))
    expect(mocks.api.updateHypothesis).toHaveBeenCalledWith('INV-0001', 'H-001', {
      confidence: 'high',
      rationale: 'Two sources.',
    })
  })

  it('shows Saving... while the confidence save is pending', async () => {
    const user = userEvent.setup()
    Object.assign(mocks.api, mockApi({ updateClaim: () => new Promise<Claim>(() => {}) }))
    renderDrawer(C001)
    const g = group('How sure are we')
    await user.click(within(g).getByRole('radio', { name: 'Low' }))
    await user.click(within(g).getByRole('button', { name: 'Save confidence' }))
    expect(within(g).getByRole('button', { name: 'Saving...' })).toBeDisabled()
  })

  it('keeps dirty input when the record changes elsewhere and offers Show latest (backstop)', async () => {
    const user = userEvent.setup()
    const { rerenderWith } = renderDrawer(C001)
    const g = () => group('How sure are we')
    await user.click(within(g()).getByRole('radio', { name: 'Low' }))
    await user.type(within(g()).getByLabelText('Why this confidence (optional)'), ' Mine.')

    rerenderWith({
      ...C001,
      confidence: 'medium',
      confidence_rationale: 'Codex re-rated it.',
      updated_at: '2026-10-02T10:00:00.000Z',
    })
    expect(within(g()).getByRole('radio', { name: 'Low' })).toHaveAttribute('aria-checked', 'true')
    expect(within(g()).getByLabelText('Why this confidence (optional)')).toHaveValue(
      `${C001.confidence_rationale} Mine.`,
    )
    expect(within(g()).getByText('This changed in another app.')).toBeInTheDocument()

    await user.click(within(g()).getByRole('button', { name: 'Show latest' }))
    expect(within(g()).getByRole('radio', { name: 'Medium' })).toHaveAttribute('aria-checked', 'true')
    expect(within(g()).getByLabelText('Why this confidence (optional)')).toHaveValue('Codex re-rated it.')
    expect(within(g()).queryByText('This changed in another app.')).toBeNull()
  })

  it('follows the latest values while the form is clean', async () => {
    const { rerenderWith } = renderDrawer(C001)
    rerenderWith({ ...C001, confidence: 'low', confidence_rationale: 'Re-rated.' })
    expect(within(group('How sure are we')).getByRole('radio', { name: 'Low' })).toHaveAttribute('aria-checked', 'true')
    expect(within(group('How sure are we')).queryByText('This changed in another app.')).toBeNull()
  })
})

describe('StatementDrawer status and verification', () => {
  beforeEach(() => {
    Object.assign(mocks.api, mockApi())
  })

  it('Verify sets human_verified; a verified record shows Verified disabled', async () => {
    const user = userEvent.setup()
    const { unmount } = renderDrawer(C002)
    await user.click(within(group('Status and verification')).getByRole('button', { name: 'Verify myself' }))
    expect(mocks.api.reviewStatement).toHaveBeenCalledWith('INV-0001', { kind: 'claim', ref: 'C-002', basis: 'displayed-review-basis', verification: 'human_verified', settle_dispute: false })
    unmount()
    renderDrawer(C001)
    expect(within(group('Status and verification')).getByRole('button', { name: 'Verified' })).toBeDisabled()
  })

  it('keeps source checked findings available for a person to verify', async () => {
    const user = userEvent.setup()
    renderDrawer({ ...C002, verification: 'source_verified' })
    expect(within(group('Status and verification')).getByRole('button', { name: 'Verify myself' })).toBeEnabled()
    await user.click(within(group('Status and verification')).getByRole('button', { name: 'Verify myself' }))
    expect(mocks.api.reviewStatement).toHaveBeenCalledWith('INV-0001', expect.objectContaining({ verification: 'human_verified', basis: 'displayed-review-basis' }))
    expect(mocks.api.setClaimVerification).not.toHaveBeenCalled()
  })

  it('guards human verification chosen from More options too', async () => {
    const user = userEvent.setup()
    renderDrawer(C002)
    const g = await statusSelects(user)
    await user.selectOptions(within(g).getByLabelText('Verification'), 'human_verified')
    expect(mocks.api.reviewStatement).toHaveBeenCalledWith('INV-0001', expect.objectContaining({ verification: 'human_verified', basis: 'displayed-review-basis' }))
    expect(mocks.api.setClaimVerification).not.toHaveBeenCalled()
  })

  it('Dispute on a claim sets status and verification in one patch', async () => {
    const user = userEvent.setup()
    renderDrawer(C002)
    await user.click(within(group('Status and verification')).getByRole('button', { name: 'Dispute' }))
    expect(mocks.api.updateClaim).toHaveBeenCalledWith('INV-0001', 'C-002', { status: 'disputed', verification: 'disputed' })
  })

  it('Dispute on a hypothesis sets its verification', async () => {
    const user = userEvent.setup()
    renderDrawer(H001)
    await user.click(within(group('Status and verification')).getByRole('button', { name: 'Dispute' }))
    expect(mocks.api.setHypothesisVerification).toHaveBeenCalledWith('INV-0001', 'H-001', 'disputed')
  })

  it('offers the claim and verification values in the selects and saves a change', async () => {
    const user = userEvent.setup()
    renderDrawer(C002)
    const g = await statusSelects(user)
    const status = within(g).getByLabelText('Status')
    expect(Array.from((status as HTMLSelectElement).options).map((o) => o.text)).toEqual(['Current', 'Disputed', 'Rejected'])
    const verification = within(g).getByLabelText('Verification')
    expect(Array.from((verification as HTMLSelectElement).options).map((o) => o.text)).toEqual([
      'AI generated',
      'Human verified',
      'Source verified',
      'Disputed',
    ])
    await user.selectOptions(status, 'rejected')
    expect(mocks.api.updateClaim).toHaveBeenCalledWith('INV-0001', 'C-002', { status: 'rejected' })
    await user.selectOptions(verification, 'source_verified')
    expect(mocks.api.setClaimVerification).toHaveBeenCalledWith('INV-0001', 'C-002', 'source_verified')
  })

  it('offers the hypothesis statuses and marks one supported', async () => {
    const user = userEvent.setup()
    renderDrawer(H001)
    const status = within(await statusSelects(user)).getByLabelText('Status') as HTMLSelectElement
    expect(Array.from(status.options).map((o) => o.text)).toEqual(['Open', 'Supported', 'Disproven'])
    await user.selectOptions(status, 'supported')
    expect(mocks.api.updateHypothesis).toHaveBeenCalledWith('INV-0001', 'H-001', { status: 'supported' })
  })

  it('a superseded claim shows its replacement and can return to Current', async () => {
    const user = userEvent.setup()
    const C003 = claims[2]
    renderDrawer(C003)
    const g = await statusSelects(user)
    expect(g).toHaveTextContent('Superseded by C-002')
    const status = within(g).getByLabelText('Status') as HTMLSelectElement
    expect(status.value).toBe('superseded')
    await user.selectOptions(status, 'active')
    expect(mocks.api.updateClaim).toHaveBeenCalledWith('INV-0001', 'C-003', { status: 'active' })
  })

  it('a superseded record offers no verification change until it is reinstated (ADR-0013)', async () => {
    const user = userEvent.setup()
    renderDrawer(claims[2])
    const g = await statusSelects(user)
    expect(within(g).getByRole('button', { name: 'Verify myself' })).toBeDisabled()
    expect(within(g).getByRole('button', { name: 'Dispute' })).toBeDisabled()
    const verification = within(g).getByLabelText('Verification') as HTMLSelectElement
    expect(verification).toBeDisabled()
    expect(verification.value).toBe('superseded')
    expect(g).toHaveTextContent(SUPERSEDED_HINT)
    // Superseded is never a free choice.
    const free = Array.from(verification.options).filter((o) => !o.disabled).map((o) => o.value)
    expect(free).not.toContain('superseded')
  })

  it('a disputed claim reads Disputed in both selects, and Verify settles the dispute', async () => {
    const user = userEvent.setup()
    renderDrawer(claims[3])
    const g = await statusSelects(user)
    expect((within(g).getByLabelText('Status') as HTMLSelectElement).value).toBe('disputed')
    expect((within(g).getByLabelText('Verification') as HTMLSelectElement).value).toBe('disputed')
    expect(within(g).getByRole('button', { name: 'Dispute' })).toBeDisabled()
    await user.click(within(g).getByRole('button', { name: 'Verify myself' }))
    expect(mocks.api.reviewStatement).not.toHaveBeenCalled()
    await user.click(within(g).getByRole('button', { name: 'Verify and settle dispute' }))
    expect(mocks.api.reviewStatement).toHaveBeenCalledWith('INV-0001', { kind: 'claim', ref: 'C-004', basis: 'displayed-review-basis', verification: 'human_verified', settle_dispute: true })
    // The meta line shows one Disputed mark, not a badge plus a status.
    expect(screen.getAllByText('Disputed', { selector: '.badge' })).toHaveLength(1)
  })

  it('an older contradicting pair still shows one state: the status wins', async () => {
    const user = userEvent.setup()
    renderDrawer(claim({ ref: 'C-009', status: 'rejected', verification: 'superseded' }))
    const g = await statusSelects(user)
    expect((within(g).getByLabelText('Status') as HTMLSelectElement).value).toBe('rejected')
    expect(screen.queryByText('Superseded', { selector: '.badge' })).toBeNull()
  })

  it('shows a refused change in plain words next to the controls', async () => {
    Object.assign(
      mocks.api,
      mockApi({
        reviewStatement: async () => {
          throw new ApiError('conflict', 'conflict: C-002 is superseded; set its status first')
        },
      }),
    )
    const user = userEvent.setup()
    renderDrawer(C002)
    await user.click(within(group('Status and verification')).getByRole('button', { name: 'Verify myself' }))
    expect(await within(group('Status and verification')).findByRole('alert')).toHaveTextContent(
      'C-002 is superseded; set its status first',
    )
  })

  it('opens More options when a refused change names one of the selects', async () => {
    Object.assign(
      mocks.api,
      mockApi({
        reviewStatement: async () => {
          throw new ApiError('validation', 'verification is not allowed here', 'verification')
        },
      }),
    )
    const user = userEvent.setup()
    renderDrawer(C002)
    const g = group('Status and verification')
    expect(within(g).queryByLabelText('Verification')).toBeNull()
    await user.click(within(g).getByRole('button', { name: 'Verify myself' }))
    expect(await within(g).findByText('verification is not allowed here')).toBeInTheDocument()
    expect(within(g).getByLabelText('Verification')).toBeInTheDocument()
    expect(within(g).getByRole('button', { name: 'Fewer options', expanded: true })).toBeInTheDocument()
  })

  it('Supersede by... opens the supersede dialog', async () => {
    const user = userEvent.setup()
    renderDrawer(C002)
    await user.click(within(group('Status and verification')).getByRole('button', { name: 'Supersede by...' }))
    expect(screen.getByRole('dialog', { name: 'Supersede C-002' })).toBeInTheDocument()
  })
})

describe('StatementDrawer links', () => {
  beforeEach(() => {
    Object.assign(mocks.api, mockApi())
  })

  it('flips a supporting link to contradicting', async () => {
    const user = userEvent.setup()
    renderDrawer(C001)
    const supporting = group('Supporting evidence')
    const block = supporting.querySelector('[data-ref="E-001"]') as HTMLElement
    await user.click(within(block).getByRole('button', { name: 'Mark as contradicting' }))
    expect(mocks.api.linkEvidenceToClaim).toHaveBeenCalledWith('INV-0001', 'C-001', 'E-001', 'contradicts')
  })

  it('flips a contradicting hypothesis link to supporting', async () => {
    const user = userEvent.setup()
    renderDrawer(hypotheses[1])
    const block = group('Contradicting evidence').querySelector('[data-ref="E-003"]') as HTMLElement
    await user.click(within(block).getByRole('button', { name: 'Mark as supporting' }))
    expect(mocks.api.linkEvidenceToHypothesis).toHaveBeenCalledWith('INV-0001', 'H-002', 'E-003', 'supports')
  })

  it('unlinks with an Undo toast that relinks with the previous relation', async () => {
    const user = userEvent.setup()
    renderDrawer(C001)
    const block = group('Supporting evidence').querySelector('[data-ref="E-004"]') as HTMLElement
    await user.click(within(block).getByRole('button', { name: 'Unlink' }))
    expect(mocks.api.unlinkEvidenceFromClaim).toHaveBeenCalledWith('INV-0001', 'C-001', 'E-004')
    const toast = await screen.findByText('Unlinked E-004 from C-001.')
    await user.click(within(toast.closest('.toast') as HTMLElement).getByRole('button', { name: 'Undo' }))
    expect(mocks.api.linkEvidenceToClaim).toHaveBeenCalledWith('INV-0001', 'C-001', 'E-004', 'supports')
  })

  it('unlinks hypothesis evidence through the hypothesis call', async () => {
    const user = userEvent.setup()
    renderDrawer(hypothesis({ ref: 'H-003', links: hypotheses[2].links }))
    const block = group('Supporting evidence').querySelector('[data-ref="E-002"]') as HTMLElement
    await user.click(within(block).getByRole('button', { name: 'Unlink' }))
    expect(mocks.api.unlinkEvidenceFromHypothesis).toHaveBeenCalledWith('INV-0001', 'H-003', 'E-002')
    expect(await screen.findByText('Unlinked E-002 from H-003.')).toBeInTheDocument()
  })

  it('Link evidence opens the link dialog for this record', async () => {
    const user = userEvent.setup()
    renderDrawer(claim({ ref: 'C-001' }))
    await user.click(screen.getByRole('button', { name: 'Link evidence' }))
    expect(screen.getByRole('dialog', { name: 'Link evidence to C-001' })).toBeInTheDocument()
  })
})
