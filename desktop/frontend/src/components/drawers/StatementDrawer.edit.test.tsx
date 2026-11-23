import type { ReactNode } from 'react'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Api } from '../../api/api'
import { ApiError, NOT_SAVED_COPY, REQUIRED_COPY } from '../../api/errors'
import type { Claim, Hypothesis, InvestigationDetail } from '../../api/types'
import { DECIDED_HOLDS, REPLACED, SOURCES_DISAGREE } from '../../lib/labels'
import { AppDataProvider } from '../../state/appData'
import { claims, detail, hypotheses } from '../../test/fixtures'
import { claim, hypothesis } from '../../test/fixtures.records'
import { mockApi } from '../../test/mockApi'
import { OpenAtLaunch } from '../../test/OpenAtLaunch'
import { CHANGED_ELSEWHERE_COPY, SHOW_NEW_VERSION } from '../ui/ChangedElsewhereBanner'
import { ToastProvider } from '../ui/Toaster'
import {
  BRING_BACK_ACTION,
  CONTRADICTING_TITLE,
  DECIDE_TITLES,
  FINDING_CHANGED_COPY,
  MOVE_TO_AGAINST,
  MOVE_TO_BACKS,
  REPLACE_ACTION,
  RULE_OUT_ACTION,
  SETTLE_ACTION,
  SETTLE_COPY,
  StatementDrawer,
  STILL_HOLDS_ACTION,
  SUPPORTING_TITLE,
  TURNED_OUT_RIGHT_ACTION,
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
const titles: string[] = Object.values(DECIDE_TITLES)
/** The group of what a person decides; its heading says where the finding stands. */
const decide = () => screen.getByRole('region', { name: (n) => titles.includes(n) })
const buttonNames = (g: HTMLElement) => within(g).getAllByRole('button').map((b) => b.textContent)
const metaLine = () => document.querySelector('.statement-group .drawer-meta-line') as HTMLElement

describe('StatementDrawer statement', () => {
  beforeEach(() => {
    Object.assign(mocks.api, mockApi())
  })

  it('asks the person to read the new version before deciding when the finding changes while the drawer is open', async () => {
    const user = userEvent.setup()
    const renderSnapshot = (basis: string) => providers(<StatementDrawer kind="claim" record={C004}
      detail={{ ...detail, review_bases: [{ ref: C004.ref, basis }] }} onOpenRef={vi.fn()} />)
    const view = render(renderSnapshot('sources-read'))
    expect(screen.getByRole('button', { name: STILL_HOLDS_ACTION })).toBeEnabled()
    expect(screen.queryByText(FINDING_CHANGED_COPY)).toBeNull()
    view.rerender(renderSnapshot('sources-changed'))
    expect(screen.getByText(FINDING_CHANGED_COPY)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: STILL_HOLDS_ACTION })).toBeDisabled()
    await user.click(screen.getByRole('button', { name: SHOW_NEW_VERSION }))
    expect(screen.queryByText(FINDING_CHANGED_COPY)).toBeNull()
    await user.click(screen.getByRole('button', { name: STILL_HOLDS_ACTION }))
    await user.click(screen.getByRole('button', { name: SETTLE_ACTION }))
    expect(mocks.api.reviewStatement).toHaveBeenCalledWith(detail.investigation.ref,
      expect.objectContaining({ basis: 'sources-changed', verification: 'human_verified', settle_dispute: true }))
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
  const WHY = 'Why you think so (optional)'

  beforeEach(() => {
    Object.assign(mocks.api, mockApi())
  })

  it('shows why as text, with no box or Save of its own and no confidence', () => {
    renderDrawer(C001)
    expect(screen.getByText(C001.confidence_rationale, { exact: false })).toBeInTheDocument()
    expect(screen.queryByLabelText(WHY)).toBeNull()
    expect(screen.queryByRole('button', { name: 'Save' })).toBeNull()
    expect(screen.queryByRole('radio')).toBeNull()
    expect(screen.queryByText(/confidence/i)).toBeNull()
  })

  it('Edit finding holds the finding and why; a change to why sends only confidence_rationale for a claim', async () => {
    const user = userEvent.setup()
    renderDrawer(C001)
    await user.click(screen.getByRole('button', { name: 'Edit finding' }))
    await user.clear(screen.getByLabelText(WHY))
    await user.type(screen.getByLabelText(WHY), 'Only the code agrees.')
    await user.click(screen.getByRole('button', { name: 'Save finding' }))
    expect(mocks.api.updateClaim).toHaveBeenCalledWith('INV-0001', 'C-001', { confidence_rationale: 'Only the code agrees.' })
  })

  it('sends only rationale for a hypothesis', async () => {
    const user = userEvent.setup()
    renderDrawer(H001)
    await user.click(screen.getByRole('button', { name: 'Edit finding' }))
    await user.clear(screen.getByLabelText(WHY))
    await user.type(screen.getByLabelText(WHY), 'Two sources.')
    await user.click(screen.getByRole('button', { name: 'Save finding' }))
    expect(mocks.api.updateHypothesis).toHaveBeenCalledWith('INV-0001', 'H-001', { rationale: 'Two sources.' })
  })

  it('shows Saving... while the save is pending', async () => {
    const user = userEvent.setup()
    Object.assign(mocks.api, mockApi({ updateClaim: () => new Promise<Claim>(() => {}) }))
    renderDrawer(C001)
    await user.click(screen.getByRole('button', { name: 'Edit finding' }))
    await user.type(screen.getByLabelText(WHY), ' More.')
    await user.click(screen.getByRole('button', { name: 'Save finding' }))
    expect(screen.getByRole('button', { name: 'Saving...' })).toBeDisabled()
  })

  it('keeps dirty input when the record changes elsewhere and offers Show the new version (backstop)', async () => {
    const user = userEvent.setup()
    const { rerenderWith } = renderDrawer(C001)
    await user.click(screen.getByRole('button', { name: 'Edit finding' }))
    await user.type(screen.getByLabelText(WHY), ' Mine.')
    rerenderWith({ ...C001, confidence_rationale: 'Codex rewrote it.', updated_at: '2026-10-02T10:00:00.000Z' })
    expect(screen.getByLabelText(WHY)).toHaveValue(`${C001.confidence_rationale} Mine.`)
    expect(screen.getByText(CHANGED_ELSEWHERE_COPY)).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: SHOW_NEW_VERSION }))
    expect(screen.getByLabelText(WHY)).toHaveValue('Codex rewrote it.')
    expect(screen.queryByText(CHANGED_ELSEWHERE_COPY)).toBeNull()
  })

  it('follows the latest values while the editor is clean', async () => {
    const user = userEvent.setup()
    const { rerenderWith } = renderDrawer(C001)
    await user.click(screen.getByRole('button', { name: 'Edit finding' }))
    rerenderWith({ ...C001, confidence_rationale: 'Rewritten.' })
    expect(screen.getByLabelText(WHY)).toHaveValue('Rewritten.')
    expect(screen.queryByText(CHANGED_ELSEWHERE_COPY)).toBeNull()
  })
})

describe('StatementDrawer deciding about a finding', () => {
  beforeEach(() => {
    Object.assign(mocks.api, mockApi())
  })

  it('a finding that stands offers Rule this out and Replace, and no check of its own', () => {
    renderDrawer(C002)
    expect(decide()).toHaveAccessibleName(DECIDE_TITLES.standing)
    expect(buttonNames(decide())).toEqual([RULE_OUT_ACTION, REPLACE_ACTION])
    expect(screen.queryByRole('button', { name: 'I checked this' })).toBeNull()
    expect(screen.queryByRole('button', { name: 'Sources disagree' })).toBeNull()
    expect(screen.queryByRole('combobox')).toBeNull()
  })

  it('the label comes from the sources, not from the stored check', () => {
    // C-001 is stored as checked by a person, but only one of its two sources is.
    renderDrawer(C001)
    expect(metaLine()).toHaveTextContent('1 of 2 sources checked by you')
    expect(metaLine()).not.toHaveTextContent('You checked this')
  })

  it('a finding not tested yet also offers This turned out right, which marks it supported only', async () => {
    const user = userEvent.setup()
    renderDrawer(H001)
    expect(decide()).toHaveAccessibleName(DECIDE_TITLES.notTested)
    expect(buttonNames(decide())).toEqual([TURNED_OUT_RIGHT_ACTION, RULE_OUT_ACTION, REPLACE_ACTION])
    await user.click(within(decide()).getByRole('button', { name: TURNED_OUT_RIGHT_ACTION }))
    expect(mocks.api.updateHypothesis).toHaveBeenCalledWith('INV-0001', 'H-001', { status: 'supported' })
    expect(mocks.api.reviewStatement).not.toHaveBeenCalled()
  })

  it('a hypothesis that already turned out right does not offer it again', () => {
    renderDrawer(H003)
    expect(buttonNames(decide())).toEqual([RULE_OUT_ACTION, REPLACE_ACTION])
  })

  it('where sources disagree, It still holds first asks, and Yes, it still holds settles it', async () => {
    const user = userEvent.setup()
    renderDrawer(C004)
    expect(decide()).toHaveAccessibleName(DECIDE_TITLES.disagree)
    expect(buttonNames(decide())).toEqual([STILL_HOLDS_ACTION, RULE_OUT_ACTION, REPLACE_ACTION])
    expect(screen.queryByText(SETTLE_COPY)).toBeNull()
    await user.click(within(decide()).getByRole('button', { name: STILL_HOLDS_ACTION }))
    expect(mocks.api.reviewStatement).not.toHaveBeenCalled()
    expect(within(decide()).getByText(SETTLE_COPY)).toBeInTheDocument()
    await user.click(within(decide()).getByRole('button', { name: SETTLE_ACTION }))
    expect(mocks.api.reviewStatement).toHaveBeenCalledWith('INV-0001', { kind: 'claim', ref: 'C-004', basis: 'displayed-review-basis', verification: 'human_verified', settle_dispute: true })
    expect(screen.getAllByText(SOURCES_DISAGREE)).toHaveLength(1)
  })

  it('a source that goes against a finding puts it where sources disagree, until the person decides it holds', () => {
    const against = claim({ ref: 'C-009', status: 'active', verification: 'ai_generated', links: [C004.links[0]] })
    const view = renderDrawer(against)
    expect(decide()).toHaveAccessibleName(DECIDE_TITLES.disagree)
    expect(metaLine()).toHaveTextContent(SOURCES_DISAGREE)
    view.rerenderWith({ ...against, verification: 'human_verified' })
    expect(decide()).toHaveAccessibleName(DECIDE_TITLES.standing)
    expect(metaLine()).toHaveTextContent(DECIDED_HOLDS)
  })

  it('a hypothesis not tested yet that the person decides holds is also marked supported', async () => {
    const user = userEvent.setup()
    renderDrawer(hypothesis({ ref: 'H-009', status: 'open', verification: 'disputed' }))
    expect(buttonNames(decide())).toEqual([STILL_HOLDS_ACTION, RULE_OUT_ACTION, REPLACE_ACTION])
    await user.click(within(decide()).getByRole('button', { name: STILL_HOLDS_ACTION }))
    await user.click(within(decide()).getByRole('button', { name: SETTLE_ACTION }))
    await waitFor(() => expect(mocks.api.updateHypothesis).toHaveBeenCalledWith('INV-0001', 'H-009', { status: 'supported' }))
    expect(vi.mocked(mocks.api.reviewStatement).mock.invocationCallOrder[0]).toBeLessThan(
      vi.mocked(mocks.api.updateHypothesis).mock.invocationCallOrder[0],
    )
  })

  it('Cancel takes the question about disagreeing sources away without a call', async () => {
    const user = userEvent.setup()
    renderDrawer(C004)
    await user.click(within(decide()).getByRole('button', { name: STILL_HOLDS_ACTION }))
    await user.click(within(decide()).getByRole('button', { name: 'Cancel' }))
    expect(within(decide()).queryByText(SETTLE_COPY)).toBeNull()
    expect(mocks.api.reviewStatement).not.toHaveBeenCalled()
  })

  it('Rule this out sends status rejected for a claim', async () => {
    const user = userEvent.setup()
    renderDrawer(C002)
    await user.click(within(decide()).getByRole('button', { name: RULE_OUT_ACTION }))
    expect(mocks.api.updateClaim).toHaveBeenCalledWith('INV-0001', 'C-002', { status: 'rejected' })
  })

  it('Rule this out sends status disproven for a hypothesis', async () => {
    const user = userEvent.setup()
    renderDrawer(H001)
    await user.click(within(decide()).getByRole('button', { name: RULE_OUT_ACTION }))
    expect(mocks.api.updateHypothesis).toHaveBeenCalledWith('INV-0001', 'H-001', { status: 'disproven' })
  })

  it('a replaced claim says so at the top, shows its replacement and only Bring this back, which makes it active and not checked', async () => {
    const user = userEvent.setup()
    renderDrawer(C003)
    expect(metaLine()).toHaveTextContent(REPLACED)
    expect(decide()).toHaveAccessibleName(DECIDE_TITLES.replaced)
    expect(buttonNames(decide())).toEqual(['Open the finding that replaced it C-002', BRING_BACK_ACTION])
    await user.click(within(decide()).getByRole('button', { name: BRING_BACK_ACTION }))
    expect(mocks.api.updateClaim).toHaveBeenCalledWith('INV-0001', 'C-003', { status: 'active', verification: 'ai_generated' })
  })

  it('a ruled-out claim shows only Bring this back', async () => {
    const user = userEvent.setup()
    renderDrawer(claim({ ref: 'C-009', status: 'rejected', verification: 'ai_generated' }))
    expect(decide()).toHaveAccessibleName(DECIDE_TITLES.ruledOut)
    expect(buttonNames(decide())).toEqual([BRING_BACK_ACTION])
    await user.click(within(decide()).getByRole('button', { name: BRING_BACK_ACTION }))
    expect(mocks.api.updateClaim).toHaveBeenCalledWith('INV-0001', 'C-009', { status: 'active', verification: 'ai_generated' })
  })

  it('a ruled-out hypothesis shows only Bring this back, which makes it open and not checked', async () => {
    const user = userEvent.setup()
    renderDrawer(H002)
    expect(buttonNames(decide())).toEqual([BRING_BACK_ACTION])
    await user.click(within(decide()).getByRole('button', { name: BRING_BACK_ACTION }))
    expect(mocks.api.updateHypothesis).toHaveBeenCalledWith('INV-0001', 'H-002', { status: 'open', verification: 'ai_generated' })
  })

  it('a replaced hypothesis comes back as open and not checked', async () => {
    const user = userEvent.setup()
    renderDrawer(hypothesis({ ref: 'H-009', status: 'superseded', verification: 'superseded', superseded_by: 'H-003' }))
    expect(buttonNames(decide())).toEqual(['Open the finding that replaced it H-003', BRING_BACK_ACTION])
    await user.click(within(decide()).getByRole('button', { name: BRING_BACK_ACTION }))
    expect(mocks.api.updateHypothesis).toHaveBeenCalledWith('INV-0001', 'H-009', { status: 'open', verification: 'ai_generated' })
  })

  it('an older contradicting pair still shows one state: the status wins', () => {
    renderDrawer(claim({ ref: 'C-009', status: 'rejected', verification: 'superseded' }))
    expect(metaLine()).toHaveTextContent('Ruled out')
    expect(metaLine()).not.toHaveTextContent(REPLACED)
    expect(buttonNames(decide())).toEqual([BRING_BACK_ACTION])
  })

  it('shows a refused change in plain words next to the buttons, never the engine text', async () => {
    Object.assign(
      mocks.api,
      mockApi({
        updateClaim: async () => {
          throw new ApiError('conflict', 'conflict: C-002 is superseded; set its status first')
        },
      }),
    )
    const user = userEvent.setup()
    renderDrawer(C002)
    await user.click(within(decide()).getByRole('button', { name: RULE_OUT_ACTION }))
    expect(await within(decide()).findByRole('alert')).toHaveTextContent(NOT_SAVED_COPY)
    expect(screen.queryByText(/superseded; set its status first/)).toBeNull()
  })

  it('Replace with a newer finding opens the replace window', async () => {
    const user = userEvent.setup()
    renderDrawer(C002)
    await user.click(within(decide()).getByRole('button', { name: REPLACE_ACTION }))
    expect(screen.getByRole('dialog', { name: 'Replace with a newer finding' })).toBeInTheDocument()
  })
})

describe('StatementDrawer links', () => {
  beforeEach(() => {
    Object.assign(mocks.api, mockApi())
  })

  it('moves a source that backs the finding to the ones that go against it', async () => {
    const user = userEvent.setup()
    renderDrawer(C001)
    const block = group(SUPPORTING_TITLE).querySelector('[data-ref="E-001"]') as HTMLElement
    await user.click(within(block).getByRole('button', { name: MOVE_TO_AGAINST }))
    expect(mocks.api.linkEvidenceToClaim).toHaveBeenCalledWith('INV-0001', 'C-001', 'E-001', 'contradicts')
  })

  it('moves a hypothesis source that goes against it to the ones that back it', async () => {
    const user = userEvent.setup()
    renderDrawer(H002)
    const block = group(CONTRADICTING_TITLE).querySelector('[data-ref="E-003"]') as HTMLElement
    await user.click(within(block).getByRole('button', { name: MOVE_TO_BACKS }))
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
