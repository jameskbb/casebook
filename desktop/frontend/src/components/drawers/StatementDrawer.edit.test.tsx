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
import { OpenAtLaunch } from '../../test/OpenAtLaunch'
import { ToastProvider } from '../ui/Toaster'
import { MORE_OPTIONS_LABEL, StatementDrawer, SUPERSEDED_HINT } from './StatementDrawer'

const mocks = vi.hoisted(() => ({ api: {} as Api }))
vi.mock('../../api/api', () => ({ api: mocks.api }))

const providers = (ui: ReactNode) => (
  <ToastProvider>
    <AppDataProvider><OpenAtLaunch />{ui}</AppDataProvider>
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
  const g = group('Check this finding')
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
    expect(screen.getByRole('button', { name: 'I checked this' })).toBeEnabled()
    view.rerender(renderSnapshot('sources-changed'))
    expect(screen.getByRole('button', { name: 'I checked this' })).toBeDisabled()
    expect(mocks.api.reviewStatement).not.toHaveBeenCalled()
    await user.click(screen.getByRole('button', { name: 'Use latest for review' }))
    await user.click(screen.getByRole('button', { name: 'I checked this' }))
    expect(mocks.api.reviewStatement).toHaveBeenCalledWith(detail.investigation.ref,
      expect.objectContaining({ basis: 'sources-changed', verification: 'human_verified' }))
  })

  it('edits the claim statement inline and saves it without a toast', async () => {
    const user = userEvent.setup()
    renderDrawer(C001)
    await user.click(screen.getByRole('button', { name: 'Edit finding' }))
    const box = screen.getByLabelText('Finding')
    expect(box).toHaveFocus()
    await user.clear(box)
    await user.type(box, 'The bills went out twice because a file was loaded two times.')
    await user.click(screen.getByRole('button', { name: 'Save finding' }))
    expect(mocks.api.updateClaim).toHaveBeenCalledWith('INV-0001', 'C-001', { statement: 'The bills went out twice because a file was loaded two times.' })
    await waitFor(() => expect(screen.queryByLabelText('Statement')).toBeNull())
    expect(screen.queryByRole('status')).toBeNull()
  })

  it('saves a hypothesis statement through updateHypothesis', async () => {
    const user = userEvent.setup()
    renderDrawer(H001)
    await user.click(screen.getByRole('button', { name: 'Edit finding' }))
    await user.type(screen.getByLabelText('Finding'), ' Maybe.')
    await user.click(screen.getByRole('button', { name: 'Save finding' }))
    expect(mocks.api.updateHypothesis).toHaveBeenCalledWith('INV-0001', 'H-001', { statement: `${H001.statement} Maybe.` })
  })

  it('Cancel restores the view without saving', async () => {
    const user = userEvent.setup()
    renderDrawer(C001)
    await user.click(screen.getByRole('button', { name: 'Edit finding' }))
    await user.type(screen.getByLabelText('Finding'), ' Draft.')
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
    await user.click(screen.getByRole('button', { name: 'Edit finding' }))
    await user.clear(screen.getByLabelText('Finding'))
    await user.type(screen.getByLabelText('Finding'), ' ')
    await user.click(screen.getByRole('button', { name: 'Save finding' }))
    expect(await screen.findByText('is required')).toBeInTheDocument()
    expect(screen.getByLabelText('Finding')).toHaveAttribute('aria-invalid', 'true')
  })
})

describe('StatementDrawer why', () => {
  beforeEach(() => {
    Object.assign(mocks.api, mockApi())
  })

  it('shows no confidence control', () => {
    renderDrawer(C001)
    expect(within(group('Why')).queryByRole('radio')).toBeNull()
    expect(screen.queryByText(/confidence/i)).toBeNull()
  })

  it('enables Save only after a change and sends the rationale with the stored confidence for claims', async () => {
    const user = userEvent.setup()
    renderDrawer(C001)
    const g = group('Why')
    const save = within(g).getByRole('button', { name: 'Save' })
    expect(save).toBeDisabled()
    const why = within(g).getByLabelText('Why (optional)')
    await user.clear(why)
    await user.type(why, 'Only the code agrees.')
    expect(save).toBeEnabled()
    await user.click(save)
    expect(mocks.api.updateClaim).toHaveBeenCalledWith('INV-0001', 'C-001', {
      confidence: C001.confidence,
      confidence_rationale: 'Only the code agrees.',
    })
  })

  it('sends rationale for hypotheses', async () => {
    const user = userEvent.setup()
    renderDrawer(H001)
    const g = group('Why')
    const why = within(g).getByLabelText('Why (optional)')
    await user.clear(why)
    await user.type(why, 'Two sources.')
    await user.click(within(g).getByRole('button', { name: 'Save' }))
    expect(mocks.api.updateHypothesis).toHaveBeenCalledWith('INV-0001', 'H-001', {
      confidence: H001.confidence,
      rationale: 'Two sources.',
    })
  })

  it('shows Saving... while the save is pending', async () => {
    const user = userEvent.setup()
    Object.assign(mocks.api, mockApi({ updateClaim: () => new Promise<Claim>(() => {}) }))
    renderDrawer(C001)
    const g = group('Why')
    await user.type(within(g).getByLabelText('Why (optional)'), ' More.')
    await user.click(within(g).getByRole('button', { name: 'Save' }))
    expect(within(g).getByRole('button', { name: 'Saving...' })).toBeDisabled()
  })

  it('keeps dirty input when the record changes elsewhere and offers Show latest (backstop)', async () => {
    const user = userEvent.setup()
    const { rerenderWith } = renderDrawer(C001)
    const g = () => group('Why')
    await user.type(within(g()).getByLabelText('Why (optional)'), ' Mine.')

    rerenderWith({
      ...C001,
      confidence_rationale: 'Codex rewrote it.',
      updated_at: '2026-10-02T10:00:00.000Z',
    })
    expect(within(g()).getByLabelText('Why (optional)')).toHaveValue(
      `${C001.confidence_rationale} Mine.`,
    )
    expect(within(g()).getByText('This changed in another app.')).toBeInTheDocument()

    await user.click(within(g()).getByRole('button', { name: 'Show latest' }))
    expect(within(g()).getByLabelText('Why (optional)')).toHaveValue('Codex rewrote it.')
    expect(within(g()).queryByText('This changed in another app.')).toBeNull()
  })

  it('follows the latest values while the form is clean', async () => {
    const { rerenderWith } = renderDrawer(C001)
    rerenderWith({ ...C001, confidence_rationale: 'Rewritten.' })
    expect(within(group('Why')).getByLabelText('Why (optional)')).toHaveValue('Rewritten.')
    expect(within(group('Why')).queryByText('This changed in another app.')).toBeNull()
  })
})

describe('StatementDrawer status and verification', () => {
  beforeEach(() => {
    Object.assign(mocks.api, mockApi())
  })

  it('Verify sets human_verified; a verified record shows Verified disabled', async () => {
    const user = userEvent.setup()
    const { unmount } = renderDrawer(C002)
    await user.click(within(group('Check this finding')).getByRole('button', { name: 'I checked this' }))
    expect(mocks.api.reviewStatement).toHaveBeenCalledWith('INV-0001', { kind: 'claim', ref: 'C-002', basis: 'displayed-review-basis', verification: 'human_verified', settle_dispute: false })
    unmount()
    renderDrawer(C001)
    expect(within(group('Check this finding')).getByRole('button', { name: 'You checked this' })).toBeDisabled()
  })

  it('keeps source checked findings available for a person to verify', async () => {
    const user = userEvent.setup()
    renderDrawer({ ...C002, verification: 'source_verified' })
    expect(within(group('Check this finding')).getByRole('button', { name: 'I checked this' })).toBeEnabled()
    await user.click(within(group('Check this finding')).getByRole('button', { name: 'I checked this' }))
    expect(mocks.api.reviewStatement).toHaveBeenCalledWith('INV-0001', expect.objectContaining({ verification: 'human_verified', basis: 'displayed-review-basis' }))
    expect(mocks.api.setClaimVerification).not.toHaveBeenCalled()
  })

  it('guards human verification chosen from More options too', async () => {
    const user = userEvent.setup()
    renderDrawer(C002)
    const g = await statusSelects(user)
    await user.selectOptions(within(g).getByLabelText('Label'), 'human_verified')
    expect(mocks.api.reviewStatement).toHaveBeenCalledWith('INV-0001', expect.objectContaining({ verification: 'human_verified', basis: 'displayed-review-basis' }))
    expect(mocks.api.setClaimVerification).not.toHaveBeenCalled()
  })

  it('Dispute on a claim sets status and verification in one patch', async () => {
    const user = userEvent.setup()
    renderDrawer(C002)
    await user.click(within(group('Check this finding')).getByRole('button', { name: 'Sources disagree' }))
    expect(mocks.api.updateClaim).toHaveBeenCalledWith('INV-0001', 'C-002', { status: 'disputed', verification: 'disputed' })
  })

  it('Dispute on a hypothesis sets its verification', async () => {
    const user = userEvent.setup()
    renderDrawer(H001)
    await user.click(within(group('Check this finding')).getByRole('button', { name: 'Sources disagree' }))
    expect(mocks.api.setHypothesisVerification).toHaveBeenCalledWith('INV-0001', 'H-001', 'disputed')
  })

  it('offers the claim and verification values in the selects and saves a change', async () => {
    const user = userEvent.setup()
    renderDrawer(C002)
    const g = await statusSelects(user)
    const status = within(g).getByLabelText('Status')
    expect(Array.from((status as HTMLSelectElement).options).map((o) => o.text)).toEqual(['Current', 'Sources disagree', 'Ruled out'])
    const verification = within(g).getByLabelText('Label')
    expect(Array.from((verification as HTMLSelectElement).options).map((o) => o.text)).toEqual([
      'Not checked',
      'You checked this',
      'AI checked the source',
      'Sources disagree',
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
    expect(Array.from(status.options).map((o) => o.text)).toEqual(['Not tested yet', 'Supported', 'Ruled out'])
    await user.selectOptions(status, 'supported')
    expect(mocks.api.updateHypothesis).toHaveBeenCalledWith('INV-0001', 'H-001', { status: 'supported' })
  })

  it('a superseded claim shows its replacement and can return to Current', async () => {
    const user = userEvent.setup()
    const C003 = claims[2]
    renderDrawer(C003)
    const g = await statusSelects(user)
    expect(within(g).getByRole('button', { name: 'Open the finding that replaced it C-002' })).toBeInTheDocument()
    const status = within(g).getByLabelText('Status') as HTMLSelectElement
    expect(status.value).toBe('superseded')
    await user.selectOptions(status, 'active')
    expect(mocks.api.updateClaim).toHaveBeenCalledWith('INV-0001', 'C-003', { status: 'active' })
  })

  it('a superseded record offers no verification change until it is reinstated', async () => {
    const user = userEvent.setup()
    renderDrawer(claims[2])
    const g = await statusSelects(user)
    expect(within(g).getByRole('button', { name: 'I checked this' })).toBeDisabled()
    expect(within(g).getByRole('button', { name: 'Sources disagree' })).toBeDisabled()
    const verification = within(g).getByLabelText('Label') as HTMLSelectElement
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
    expect((within(g).getByLabelText('Label') as HTMLSelectElement).value).toBe('disputed')
    expect(within(g).getByRole('button', { name: 'Sources disagree' })).toBeDisabled()
    await user.click(within(g).getByRole('button', { name: 'I checked this' }))
    expect(mocks.api.reviewStatement).not.toHaveBeenCalled()
    await user.click(within(g).getByRole('button', { name: 'I checked this, settle it' }))
    expect(mocks.api.reviewStatement).toHaveBeenCalledWith('INV-0001', { kind: 'claim', ref: 'C-004', basis: 'displayed-review-basis', verification: 'human_verified', settle_dispute: true })
    // The meta line shows one Disputed mark, not a badge plus a status.
    expect(screen.getAllByText('Sources disagree', { selector: '.badge' })).toHaveLength(1)
  })

  it('an older contradicting pair still shows one state: the status wins', async () => {
    const user = userEvent.setup()
    renderDrawer(claim({ ref: 'C-009', status: 'rejected', verification: 'superseded' }))
    const g = await statusSelects(user)
    expect((within(g).getByLabelText('Status') as HTMLSelectElement).value).toBe('rejected')
    expect(screen.queryByText('Replaced', { selector: '.badge' })).toBeNull()
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
    await user.click(within(group('Check this finding')).getByRole('button', { name: 'I checked this' }))
    expect(await within(group('Check this finding')).findByRole('alert')).toHaveTextContent(
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
    const g = group('Check this finding')
    expect(within(g).queryByLabelText('Label')).toBeNull()
    await user.click(within(g).getByRole('button', { name: 'I checked this' }))
    expect(await within(g).findByText('verification is not allowed here')).toBeInTheDocument()
    expect(within(g).getByLabelText('Label')).toBeInTheDocument()
    expect(within(g).getByRole('button', { name: 'Fewer options', expanded: true })).toBeInTheDocument()
  })

  it('Replace with... opens the supersede dialog', async () => {
    const user = userEvent.setup()
    renderDrawer(C002)
    await user.click(within(group('Check this finding')).getByRole('button', { name: 'Replace with...' }))
    expect(screen.getByRole('dialog', { name: 'Replace this finding' })).toBeInTheDocument()
  })
})

describe('StatementDrawer links', () => {
  beforeEach(() => {
    Object.assign(mocks.api, mockApi())
  })

  it('flips a supporting link to contradicting', async () => {
    const user = userEvent.setup()
    renderDrawer(C001)
    const supporting = group('Sources that support it')
    const block = supporting.querySelector('[data-ref="E-001"]') as HTMLElement
    await user.click(within(block).getByRole('button', { name: 'It contradicts this' }))
    expect(mocks.api.linkEvidenceToClaim).toHaveBeenCalledWith('INV-0001', 'C-001', 'E-001', 'contradicts')
  })

  it('flips a contradicting hypothesis link to supporting', async () => {
    const user = userEvent.setup()
    renderDrawer(hypotheses[1])
    const block = group('Sources that contradict it').querySelector('[data-ref="E-003"]') as HTMLElement
    await user.click(within(block).getByRole('button', { name: 'It supports this' }))
    expect(mocks.api.linkEvidenceToHypothesis).toHaveBeenCalledWith('INV-0001', 'H-002', 'E-003', 'supports')
  })

  it('unlinks with an Undo toast that relinks with the previous relation', async () => {
    const user = userEvent.setup()
    renderDrawer(C001)
    const block = group('Sources that support it').querySelector('[data-ref="E-004"]') as HTMLElement
    await user.click(within(block).getByRole('button', { name: 'Remove from this finding' }))
    expect(mocks.api.unlinkEvidenceFromClaim).toHaveBeenCalledWith('INV-0001', 'C-001', 'E-004')
    const toast = await screen.findByText('Removed the source from this finding.')
    await user.click(within(toast.closest('.toast') as HTMLElement).getByRole('button', { name: 'Undo' }))
    expect(mocks.api.linkEvidenceToClaim).toHaveBeenCalledWith('INV-0001', 'C-001', 'E-004', 'supports')
  })

  it('unlinks hypothesis evidence through the hypothesis call', async () => {
    const user = userEvent.setup()
    renderDrawer(hypothesis({ ref: 'H-003', links: hypotheses[2].links }))
    const block = group('Sources that support it').querySelector('[data-ref="E-002"]') as HTMLElement
    await user.click(within(block).getByRole('button', { name: 'Remove from this finding' }))
    expect(mocks.api.unlinkEvidenceFromHypothesis).toHaveBeenCalledWith('INV-0001', 'H-003', 'E-002')
    expect(await screen.findByText('Removed the source from this finding.')).toBeInTheDocument()
  })

  it('Link a source opens the link dialog for this record', async () => {
    const user = userEvent.setup()
    renderDrawer(claim({ ref: 'C-001' }))
    await user.click(screen.getByRole('button', { name: 'Add a source to this finding' }))
    expect(screen.getByRole('dialog', { name: 'Add a source to this finding' })).toBeInTheDocument()
  })
})
