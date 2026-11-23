import type { ReactNode } from 'react'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Api } from '../../api/api'
import { ApiError, NOT_SAVED_COPY, REQUIRED_COPY } from '../../api/errors'
import type { Claim, Hypothesis, InvestigationDetail } from '../../api/types'
import { AppDataProvider } from '../../state/appData'
import { claims, detail, hypotheses } from '../../test/fixtures'
import { claim, hypothesis } from '../../test/fixtures.records'
import { mockApi } from '../../test/mockApi'
import { OpenAtLaunch } from '../../test/OpenAtLaunch'
import { CHANGED_ELSEWHERE_COPY, SHOW_NEW_VERSION } from '../ui/ChangedElsewhereBanner'
import { ToastProvider } from '../ui/Toaster'
import {
  BRING_BACK_ACTION,
  CHECKED_ACTION,
  CONTRADICTING_TITLE,
  DISAGREE_ACTION,
  FINDING_CHANGED_COPY,
  REPLACE_ACTION,
  RULE_OUT_ACTION,
  SETTLE_ACTION,
  SETTLE_COPY,
  StatementDrawer,
  SUPPORTING_TITLE,
} from './StatementDrawer'

const mocks = vi.hoisted(() => ({ api: {} as Api }))
vi.mock('../../api/api', () => ({ api: mocks.api }))

const providers = (ui: ReactNode) => (
  <ToastProvider>
    <AppDataProvider><OpenAtLaunch />{ui}</AppDataProvider>
  </ToastProvider>
)

const C001 = claims[0]
const C002 = claims[1]
const C003 = claims[2]
const C004 = claims[3]
const H001 = hypotheses[0]
const H002 = hypotheses[1]
const H003 = hypotheses[2]

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
const check = () => group('Check this finding')
const buttonNames = (g: HTMLElement) => within(g).getAllByRole('button').map((b) => b.textContent)

const WHY = 'Why you think so (optional)'
/** The why field and the group around it (the group has no heading of its own). */
const whyField = () => screen.getByLabelText(WHY)
const whyGroup = () => whyField().closest('.drawer-group') as HTMLElement

describe('StatementDrawer statement', () => {
  beforeEach(() => {
    Object.assign(mocks.api, mockApi())
  })

  it('asks the person to read the new version before checking when the finding changes while the drawer is open', async () => {
    const user = userEvent.setup()
    const renderSnapshot = (basis: string) => providers(<StatementDrawer kind="claim" record={C002}
      detail={{ ...detail, review_bases: [{ ref: C002.ref, basis }] }} onOpenRef={vi.fn()} />)
    const view = render(renderSnapshot('sources-read'))
    expect(screen.getByRole('button', { name: CHECKED_ACTION })).toBeEnabled()
    expect(screen.queryByText(FINDING_CHANGED_COPY)).toBeNull()
    view.rerender(renderSnapshot('sources-changed'))
    expect(screen.getByText(FINDING_CHANGED_COPY)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: CHECKED_ACTION })).toBeDisabled()
    expect(mocks.api.reviewStatement).not.toHaveBeenCalled()
    await user.click(screen.getByRole('button', { name: SHOW_NEW_VERSION }))
    expect(screen.queryByText(FINDING_CHANGED_COPY)).toBeNull()
    await user.click(screen.getByRole('button', { name: CHECKED_ACTION }))
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
    await waitFor(() => expect(screen.queryByLabelText('Finding')).toBeNull())
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
    expect(screen.queryByLabelText('Finding')).toBeNull()
    expect(screen.getByText(C001.statement)).toBeInTheDocument()
    expect(mocks.api.updateClaim).not.toHaveBeenCalled()
  })

  it('shows a validation error under the finding in plain words', async () => {
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
    expect(await screen.findByText(REQUIRED_COPY)).toBeInTheDocument()
    expect(screen.queryByText('is required')).toBeNull()
    expect(screen.getByLabelText('Finding')).toHaveAttribute('aria-invalid', 'true')
  })
})

describe('StatementDrawer why', () => {
  beforeEach(() => {
    Object.assign(mocks.api, mockApi())
  })

  it('shows no confidence control and no heading Why next to the field label', () => {
    renderDrawer(C001)
    expect(whyField()).toBeInTheDocument()
    expect(screen.queryByRole('radio')).toBeNull()
    expect(screen.queryByText(/confidence/i)).toBeNull()
    expect(screen.queryByRole('heading', { name: /^Why/ })).toBeNull()
    expect(screen.queryByRole('region', { name: /^Why/ })).toBeNull()
    expect(within(whyGroup()).queryByRole('heading')).toBeNull()
  })

  it('enables Save only after a change and sends only confidence_rationale for a claim', async () => {
    const user = userEvent.setup()
    renderDrawer(C001)
    const save = within(whyGroup()).getByRole('button', { name: 'Save' })
    expect(save).toBeDisabled()
    await user.clear(whyField())
    await user.type(whyField(), 'Only the code agrees.')
    expect(save).toBeEnabled()
    await user.click(save)
    expect(mocks.api.updateClaim).toHaveBeenCalledWith('INV-0001', 'C-001', { confidence_rationale: 'Only the code agrees.' })
    expect(Object.keys(vi.mocked(mocks.api.updateClaim).mock.calls[0][2])).toEqual(['confidence_rationale'])
  })

  it('sends only rationale for a hypothesis', async () => {
    const user = userEvent.setup()
    renderDrawer(H001)
    await user.clear(whyField())
    await user.type(whyField(), 'Two sources.')
    await user.click(within(whyGroup()).getByRole('button', { name: 'Save' }))
    expect(mocks.api.updateHypothesis).toHaveBeenCalledWith('INV-0001', 'H-001', { rationale: 'Two sources.' })
    expect(Object.keys(vi.mocked(mocks.api.updateHypothesis).mock.calls[0][2])).toEqual(['rationale'])
  })

  it('shows Saving... while the save is pending', async () => {
    const user = userEvent.setup()
    Object.assign(mocks.api, mockApi({ updateClaim: () => new Promise<Claim>(() => {}) }))
    renderDrawer(C001)
    await user.type(whyField(), ' More.')
    await user.click(within(whyGroup()).getByRole('button', { name: 'Save' }))
    expect(within(whyGroup()).getByRole('button', { name: 'Saving...' })).toBeDisabled()
  })

  it('keeps dirty input when the record changes elsewhere and offers Show the new version (backstop)', async () => {
    const user = userEvent.setup()
    const { rerenderWith } = renderDrawer(C001)
    await user.type(whyField(), ' Mine.')

    rerenderWith({
      ...C001,
      confidence_rationale: 'Codex rewrote it.',
      updated_at: '2026-10-02T10:00:00.000Z',
    })
    expect(whyField()).toHaveValue(`${C001.confidence_rationale} Mine.`)
    expect(within(whyGroup()).getByText(CHANGED_ELSEWHERE_COPY)).toBeInTheDocument()

    await user.click(within(whyGroup()).getByRole('button', { name: SHOW_NEW_VERSION }))
    expect(whyField()).toHaveValue('Codex rewrote it.')
    expect(within(whyGroup()).queryByText(CHANGED_ELSEWHERE_COPY)).toBeNull()
  })

  it('follows the latest values while the form is clean', async () => {
    const { rerenderWith } = renderDrawer(C001)
    rerenderWith({ ...C001, confidence_rationale: 'Rewritten.' })
    expect(whyField()).toHaveValue('Rewritten.')
    expect(within(whyGroup()).queryByText(CHANGED_ELSEWHERE_COPY)).toBeNull()
  })
})

describe('StatementDrawer check this finding', () => {
  beforeEach(() => {
    Object.assign(mocks.api, mockApi())
  })

  it('a finding that stands shows exactly four buttons, and no More options, Status or Label', () => {
    renderDrawer(C002)
    expect(buttonNames(check())).toEqual(['I checked this', 'Sources disagree', 'Rule this out', 'Replace with a newer finding'])
    expect([CHECKED_ACTION, DISAGREE_ACTION, RULE_OUT_ACTION, REPLACE_ACTION]).toEqual(buttonNames(check()))
    expect(screen.queryByRole('button', { name: 'More options' })).toBeNull()
    expect(screen.queryByLabelText('Status')).toBeNull()
    expect(screen.queryByLabelText('Label')).toBeNull()
    expect(screen.queryByRole('combobox')).toBeNull()
  })

  it('I checked this on a claim calls reviewStatement only', async () => {
    const user = userEvent.setup()
    renderDrawer(C002)
    await user.click(within(check()).getByRole('button', { name: CHECKED_ACTION }))
    expect(mocks.api.reviewStatement).toHaveBeenCalledWith('INV-0001', { kind: 'claim', ref: 'C-002', basis: 'displayed-review-basis', verification: 'human_verified', settle_dispute: false })
    await waitFor(() => expect(within(check()).getByRole('button', { name: CHECKED_ACTION })).toBeEnabled())
    expect(mocks.api.updateClaim).not.toHaveBeenCalled()
    expect(mocks.api.setClaimVerification).not.toHaveBeenCalled()
  })

  it('I checked this on a finding not tested yet calls reviewStatement, then marks it supported', async () => {
    const user = userEvent.setup()
    renderDrawer(H001)
    await user.click(within(check()).getByRole('button', { name: CHECKED_ACTION }))
    await waitFor(() => expect(mocks.api.updateHypothesis).toHaveBeenCalledWith('INV-0001', 'H-001', { status: 'supported' }))
    expect(mocks.api.reviewStatement).toHaveBeenCalledWith('INV-0001', { kind: 'hypothesis', ref: 'H-001', basis: 'displayed-review-basis', verification: 'human_verified', settle_dispute: false })
    expect(vi.mocked(mocks.api.reviewStatement).mock.invocationCallOrder[0]).toBeLessThan(
      vi.mocked(mocks.api.updateHypothesis).mock.invocationCallOrder[0],
    )
  })

  it('I checked this on a hypothesis already supported does not call updateHypothesis', async () => {
    const user = userEvent.setup()
    renderDrawer(H003)
    await user.click(within(check()).getByRole('button', { name: CHECKED_ACTION }))
    expect(mocks.api.reviewStatement).toHaveBeenCalledWith('INV-0001', expect.objectContaining({ kind: 'hypothesis', ref: 'H-003', verification: 'human_verified' }))
    await waitFor(() => expect(within(check()).getByRole('button', { name: CHECKED_ACTION })).toBeEnabled())
    expect(mocks.api.updateHypothesis).not.toHaveBeenCalled()
  })

  it('a finding the AI checked can still be checked by the person', async () => {
    const user = userEvent.setup()
    renderDrawer({ ...C002, verification: 'source_verified' })
    expect(within(check()).getByRole('button', { name: CHECKED_ACTION })).toBeEnabled()
    await user.click(within(check()).getByRole('button', { name: CHECKED_ACTION }))
    expect(mocks.api.reviewStatement).toHaveBeenCalledWith('INV-0001', expect.objectContaining({ verification: 'human_verified', basis: 'displayed-review-basis' }))
    expect(mocks.api.setClaimVerification).not.toHaveBeenCalled()
  })

  it('a finding the person already checked shows no I checked this button, enabled or disabled', () => {
    renderDrawer(C001)
    expect(buttonNames(check())).toEqual([DISAGREE_ACTION, RULE_OUT_ACTION, REPLACE_ACTION])
    expect(screen.queryByRole('button', { name: CHECKED_ACTION })).toBeNull()
    expect(screen.queryByRole('button', { name: 'You checked this' })).toBeNull()
    // The label at the top says it.
    expect(document.querySelector('.statement-group .drawer-meta-line')).toHaveTextContent('You checked this')
  })

  it('Sources disagree on a claim sets status and verification in one patch', async () => {
    const user = userEvent.setup()
    renderDrawer(C002)
    await user.click(within(check()).getByRole('button', { name: DISAGREE_ACTION }))
    expect(mocks.api.updateClaim).toHaveBeenCalledWith('INV-0001', 'C-002', { status: 'disputed', verification: 'disputed' })
  })

  it('Sources disagree on a hypothesis sets its verification', async () => {
    const user = userEvent.setup()
    renderDrawer(H001)
    await user.click(within(check()).getByRole('button', { name: DISAGREE_ACTION }))
    expect(mocks.api.setHypothesisVerification).toHaveBeenCalledWith('INV-0001', 'H-001', 'disputed')
  })

  it('Rule this out sends status rejected for a claim', async () => {
    const user = userEvent.setup()
    renderDrawer(C002)
    await user.click(within(check()).getByRole('button', { name: RULE_OUT_ACTION }))
    expect(mocks.api.updateClaim).toHaveBeenCalledWith('INV-0001', 'C-002', { status: 'rejected' })
  })

  it('Rule this out sends status disproven for a hypothesis', async () => {
    const user = userEvent.setup()
    renderDrawer(H001)
    await user.click(within(check()).getByRole('button', { name: RULE_OUT_ACTION }))
    expect(mocks.api.updateHypothesis).toHaveBeenCalledWith('INV-0001', 'H-001', { status: 'disproven' })
  })

  it('a replaced claim shows its replacement and only Bring this back, which makes it active and not checked', async () => {
    const user = userEvent.setup()
    renderDrawer(C003)
    expect(buttonNames(check())).toEqual(['Open the finding that replaced it C-002', BRING_BACK_ACTION])
    await user.click(within(check()).getByRole('button', { name: BRING_BACK_ACTION }))
    expect(mocks.api.updateClaim).toHaveBeenCalledWith('INV-0001', 'C-003', { status: 'active', verification: 'ai_generated' })
  })

  it('a ruled-out claim shows only Bring this back', async () => {
    const user = userEvent.setup()
    renderDrawer(claim({ ref: 'C-009', status: 'rejected', verification: 'ai_generated' }))
    expect(buttonNames(check())).toEqual([BRING_BACK_ACTION])
    await user.click(within(check()).getByRole('button', { name: BRING_BACK_ACTION }))
    expect(mocks.api.updateClaim).toHaveBeenCalledWith('INV-0001', 'C-009', { status: 'active', verification: 'ai_generated' })
  })

  it('a ruled-out hypothesis shows only Bring this back, which makes it open and not checked', async () => {
    const user = userEvent.setup()
    renderDrawer(H002)
    expect(buttonNames(check())).toEqual([BRING_BACK_ACTION])
    await user.click(within(check()).getByRole('button', { name: BRING_BACK_ACTION }))
    expect(mocks.api.updateHypothesis).toHaveBeenCalledWith('INV-0001', 'H-002', { status: 'open', verification: 'ai_generated' })
  })

  it('a replaced hypothesis comes back as open and not checked', async () => {
    const user = userEvent.setup()
    renderDrawer(hypothesis({ ref: 'H-009', status: 'superseded', verification: 'superseded', superseded_by: 'H-003' }))
    expect(buttonNames(check())).toEqual(['Open the finding that replaced it H-003', BRING_BACK_ACTION])
    await user.click(within(check()).getByRole('button', { name: BRING_BACK_ACTION }))
    expect(mocks.api.updateHypothesis).toHaveBeenCalledWith('INV-0001', 'H-009', { status: 'open', verification: 'ai_generated' })
  })

  it('where sources disagree, I checked this first asks, and Yes, I checked it settles it', async () => {
    const user = userEvent.setup()
    renderDrawer(C004)
    const g = check()
    expect(within(g).getByRole('button', { name: DISAGREE_ACTION })).toBeDisabled()
    expect(within(g).queryByText(SETTLE_COPY)).toBeNull()
    await user.click(within(g).getByRole('button', { name: CHECKED_ACTION }))
    expect(mocks.api.reviewStatement).not.toHaveBeenCalled()
    expect(within(g).getByText(SETTLE_COPY)).toBeInTheDocument()
    expect(SETTLE_ACTION).toBe('Yes, I checked it')
    await user.click(within(g).getByRole('button', { name: 'Yes, I checked it' }))
    expect(mocks.api.reviewStatement).toHaveBeenCalledWith('INV-0001', { kind: 'claim', ref: 'C-004', basis: 'displayed-review-basis', verification: 'human_verified', settle_dispute: true })
    // The meta line shows one Sources disagree mark, not a badge plus a status.
    expect(screen.getAllByText('Sources disagree', { selector: '.badge' })).toHaveLength(1)
  })

  it('Cancel takes the question about disagreeing sources away without a call', async () => {
    const user = userEvent.setup()
    renderDrawer(C004)
    await user.click(within(check()).getByRole('button', { name: CHECKED_ACTION }))
    await user.click(within(check()).getByRole('button', { name: 'Cancel' }))
    expect(within(check()).queryByText(SETTLE_COPY)).toBeNull()
    expect(mocks.api.reviewStatement).not.toHaveBeenCalled()
  })

  it('an older contradicting pair still shows one state: the status wins', () => {
    renderDrawer(claim({ ref: 'C-009', status: 'rejected', verification: 'superseded' }))
    expect(screen.getByText('Ruled out')).toBeInTheDocument()
    expect(screen.queryByText('Replaced', { selector: '.badge' })).toBeNull()
    expect(buttonNames(check())).toEqual([BRING_BACK_ACTION])
  })

  it('shows a refused change in plain words next to the buttons, never the engine text', async () => {
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
    await user.click(within(check()).getByRole('button', { name: CHECKED_ACTION }))
    expect(await within(check()).findByRole('alert')).toHaveTextContent(NOT_SAVED_COPY)
    expect(screen.queryByText(/superseded; set its status first/)).toBeNull()
  })

  it('Replace with a newer finding opens the replace dialog', async () => {
    const user = userEvent.setup()
    renderDrawer(C002)
    await user.click(within(check()).getByRole('button', { name: REPLACE_ACTION }))
    expect(screen.getByRole('dialog', { name: 'Replace with a newer finding' })).toBeInTheDocument()
  })
})

describe('StatementDrawer links', () => {
  beforeEach(() => {
    Object.assign(mocks.api, mockApi())
  })

  it('flips a source that backs the finding to one that goes against it', async () => {
    const user = userEvent.setup()
    renderDrawer(C001)
    const supporting = group(SUPPORTING_TITLE)
    const block = supporting.querySelector('[data-ref="E-001"]') as HTMLElement
    await user.click(within(block).getByRole('button', { name: 'It goes against this' }))
    expect(mocks.api.linkEvidenceToClaim).toHaveBeenCalledWith('INV-0001', 'C-001', 'E-001', 'contradicts')
  })

  it('flips a hypothesis source that goes against it to one that backs it', async () => {
    const user = userEvent.setup()
    renderDrawer(H002)
    const block = group(CONTRADICTING_TITLE).querySelector('[data-ref="E-003"]') as HTMLElement
    await user.click(within(block).getByRole('button', { name: 'It backs this' }))
    expect(mocks.api.linkEvidenceToHypothesis).toHaveBeenCalledWith('INV-0001', 'H-002', 'E-003', 'supports')
  })

  it('unlinks with an Undo toast that relinks with the previous relation', async () => {
    const user = userEvent.setup()
    renderDrawer(C001)
    const block = group(SUPPORTING_TITLE).querySelector('[data-ref="E-004"]') as HTMLElement
    await user.click(within(block).getByRole('button', { name: 'Remove from this finding' }))
    expect(mocks.api.unlinkEvidenceFromClaim).toHaveBeenCalledWith('INV-0001', 'C-001', 'E-004')
    const toast = await screen.findByText('Removed the source from this finding.')
    await user.click(within(toast.closest('.toast') as HTMLElement).getByRole('button', { name: 'Undo' }))
    expect(mocks.api.linkEvidenceToClaim).toHaveBeenCalledWith('INV-0001', 'C-001', 'E-004', 'supports')
  })

  it('unlinks hypothesis evidence through the hypothesis call', async () => {
    const user = userEvent.setup()
    renderDrawer(hypothesis({ ref: 'H-003', links: hypotheses[2].links }))
    const block = group(SUPPORTING_TITLE).querySelector('[data-ref="E-002"]') as HTMLElement
    await user.click(within(block).getByRole('button', { name: 'Remove from this finding' }))
    expect(mocks.api.unlinkEvidenceFromHypothesis).toHaveBeenCalledWith('INV-0001', 'H-003', 'E-002')
    expect(await screen.findByText('Removed the source from this finding.')).toBeInTheDocument()
  })

  it('Add a source to this finding opens the link dialog for this record', async () => {
    const user = userEvent.setup()
    renderDrawer(claim({ ref: 'C-001' }))
    await user.click(screen.getByRole('button', { name: 'Add a source to this finding' }))
    expect(screen.getByRole('dialog', { name: 'Add a source to this finding' })).toBeInTheDocument()
  })
})
