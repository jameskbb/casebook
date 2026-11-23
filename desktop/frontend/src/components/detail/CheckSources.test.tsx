import { useState, type ReactNode } from 'react'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Api } from '../../api/api'
import type { InvestigationDetail } from '../../api/types'
import { AppDataProvider, useAppData } from '../../state/appData'
import { detail as baseDetail, evidence } from '../../test/fixtures'
import { mockApi } from '../../test/mockApi'
import { OpenAtLaunch } from '../../test/OpenAtLaunch'
import { ToastProvider } from '../ui/Toaster'
import { RULE_OUT_ACTION, STILL_HOLDS_ACTION } from '../drawers/StatementDrawer'
import { ALL_CHECKED, CHECK_SOURCES_TITLE, CheckSources, checkStep, DISAGREE_HEADING, goBackText, SKIP_ACTION, skippedText, SOURCE_CHECKED_ACTION, WRITE_THE_ANSWER } from './CheckSources'
import { CHECKED_ANSWER_ACTION, WHAT_DID_YOU_CHECK } from './ConclusionPanel'

const mocks = vi.hoisted(() => ({ api: {} as Api }))
vi.mock('../../api/api', () => ({ api: mocks.api }))

describe('checkStep', () => {
  it('walks the sources a person has not checked in page order, skipping replaced ones', () => {
    const list = [...evidence, { ...evidence[0], ref: 'E-009', verification: 'superseded' as const }]
    // E-004 is already checked by the person; E-009 is replaced.
    expect(checkStep(list, [], 0)).toMatchObject({ current: { ref: 'E-001' }, position: 1, total: 3, left: 3 })
    expect(checkStep(list, ['E-001'], 0)).toMatchObject({ current: { ref: 'E-002' }, position: 2, total: 3, left: 3 })
    expect(checkStep(list, ['E-001', 'E-002', 'E-003'], 0)).toMatchObject({ current: undefined, left: 3 })
  })

  it('keeps counting the sources already checked in this walk', () => {
    // Two checked here, so the list now holds only E-003.
    const rest = evidence.map((e) => (e.ref === 'E-003' ? e : { ...e, verification: 'human_verified' as const }))
    expect(checkStep(rest, [], 2)).toMatchObject({ current: { ref: 'E-003' }, position: 3, total: 3 })
  })
})

describe('skipped wording', () => {
  it('names one or many sources and findings', () => {
    expect(goBackText(1)).toBe('Go back to the skipped source')
    expect(goBackText(2)).toBe('Go back to the skipped sources')
    expect(goBackText(0, 1)).toBe('Go back to the skipped finding')
    expect(goBackText(1, 1)).toBe('Go back to what you skipped')
    expect(skippedText(1)).toBe('You skipped 1 source. It is still not checked.')
    expect(skippedText(0, 2)).toBe('You skipped 2 findings. Sources still disagree about them.')
  })
})

describe('CheckSources', () => {
  let current: InvestigationDetail
  const onWriteAnswer = vi.fn()

  beforeEach(() => {
    current = structuredClone(baseDetail)
    Object.assign(
      mocks.api,
      mockApi({
        getInvestigationDetail: async () => current,
        setEvidenceVerification: async (_inv, ref, v) => {
          current = { ...current, evidence: current.evidence.map((e) => (e.ref === ref ? { ...e, verification: v } : e)) }
          return current.evidence.find((e) => e.ref === ref)!
        },
        updateClaim: async (_inv, ref, patch) => {
          current = { ...current, claims: current.claims.map((c) => (c.ref === ref ? { ...c, ...patch, verification: 'ai_generated' } : c)) }
          return current.claims.find((c) => c.ref === ref)!
        },
        reviewStatement: async (_inv, input) => {
          current = { ...current, claims: current.claims.map((c) => (c.ref === input.ref ? { ...c, status: 'active', verification: 'human_verified' } : c)) }
        },
      }),
    )
    onWriteAnswer.mockClear()
  })

  function Harness() {
    const { detail } = useAppData()
    const [open, setOpen] = useState(false)
    return (
      <>
        <button onClick={() => setOpen(true)}>Open</button>
        {open && detail && <CheckSources detail={detail} onClose={() => setOpen(false)} onWriteAnswer={onWriteAnswer} />}
      </>
    )
  }

  const renderWalk = async () => {
    const user = userEvent.setup()
    const wrap = (ui: ReactNode) => <ToastProvider><AppDataProvider><OpenAtLaunch />{ui}</AppDataProvider></ToastProvider>
    render(wrap(<Harness />))
    await waitFor(() => expect(mocks.api.getInvestigationDetail).toHaveBeenCalled())
    await user.click(screen.getByRole('button', { name: 'Open' }))
    return { user, dialog: await screen.findByRole('dialog', { name: CHECK_SOURCES_TITLE }) }
  }

  it('goes through the sources, then each finding where sources disagree, then the answer, in one pass', async () => {
    const { user, dialog } = await renderWalk()
    expect(within(dialog).getByText('Source 1 of 3')).toBeInTheDocument()
    expect(within(dialog).getByRole('heading', { name: evidence[0].title })).toBeInTheDocument()
    expect(within(dialog).getByText('Backs it:')).toBeInTheDocument()

    await user.click(within(dialog).getByRole('button', { name: SOURCE_CHECKED_ACTION }))
    expect(mocks.api.setEvidenceVerification).toHaveBeenCalledWith('INV-0001', 'E-001', 'human_verified')
    expect(await within(dialog).findByText('Source 2 of 3')).toBeInTheDocument()
    expect(within(dialog).getByRole('heading', { name: evidence[1].title })).toBeInTheDocument()

    await user.click(within(dialog).getByRole('button', { name: SKIP_ACTION }))
    expect(within(dialog).getByText('Source 3 of 3')).toBeInTheDocument()
    expect(within(dialog).getByText('Goes against it:', { exact: false })).toBeInTheDocument()
    await user.click(within(dialog).getByRole('button', { name: SOURCE_CHECKED_ACTION }))

    // The skipped one is still not checked; going back offers it again.
    // Then the finding where sources disagree, with its sources both ways.
    expect(await within(dialog).findByRole('heading', { name: DISAGREE_HEADING })).toBeInTheDocument()
    expect(within(dialog).getByText('Finding 1 of 1')).toBeInTheDocument()
    expect(within(dialog).getByText('Callers waited too long and hung up before buying.')).toBeInTheDocument()
    expect(within(dialog).getByText('Goes against it')).toBeInTheDocument()
    expect(within(dialog).getByRole('button', { name: STILL_HOLDS_ACTION })).toHaveFocus()
    await user.click(within(dialog).getByRole('button', { name: SKIP_ACTION }))

    // The skipped ones still need the person; going back offers them again, source first.
    expect(await within(dialog).findByText(skippedText(1, 1))).toBeInTheDocument()
    expect(within(dialog).queryByLabelText(WHAT_DID_YOU_CHECK)).toBeNull()
    await user.click(within(dialog).getByRole('button', { name: goBackText(1, 1) }))
    expect(within(dialog).getByText('Source 3 of 3')).toBeInTheDocument()
    await user.click(within(dialog).getByRole('button', { name: SOURCE_CHECKED_ACTION }))
    await user.click(await within(dialog).findByRole('button', { name: RULE_OUT_ACTION }))
    expect(mocks.api.updateClaim).toHaveBeenCalledWith('INV-0001', 'C-004', { status: 'rejected' })

    // Every source is checked and nothing is left to decide: the answer is the last step.
    expect(await within(dialog).findByText('Last, check the answer.')).toBeInTheDocument()
    const field = within(dialog).getByLabelText(WHAT_DID_YOU_CHECK)
    expect(field).toHaveFocus()
    const confirm = within(dialog).getByRole('button', { name: CHECKED_ANSWER_ACTION })
    expect(confirm).toBeDisabled()
    await user.type(field, 'The call log matches the answer.')
    await user.click(confirm)
    expect(mocks.api.updateConclusion).toHaveBeenCalledWith('INV-0001', expect.objectContaining({ review: true, review_basis: 'The call log matches the answer.', verification: 'human_verified' }))
  })

  it('keeps focus on I checked this so Enter moves through the sources, and Escape closes with focus back on the opener', async () => {
    const { user, dialog } = await renderWalk()
    expect(within(dialog).getByRole('button', { name: SOURCE_CHECKED_ACTION })).toHaveFocus()
    await user.keyboard('{Enter}')
    expect(await within(dialog).findByText('Source 2 of 3')).toBeInTheDocument()
    expect(within(dialog).getByRole('button', { name: SOURCE_CHECKED_ACTION })).toHaveFocus()
    await user.keyboard('{Escape}')
    expect(screen.queryByRole('dialog', { name: CHECK_SOURCES_TITLE })).toBeNull()
    expect(screen.getByRole('button', { name: 'Open' })).toHaveFocus()
  })

  it('records It still holds as the person read the finding', async () => {
    current = { ...current, evidence: current.evidence.map((e) => ({ ...e, verification: 'human_verified' as const })) }
    const { user, dialog } = await renderWalk()
    expect(within(dialog).getByRole('heading', { name: DISAGREE_HEADING })).toBeInTheDocument()
    await user.click(within(dialog).getByRole('button', { name: STILL_HOLDS_ACTION }))
    expect(mocks.api.reviewStatement).toHaveBeenCalledWith('INV-0001', { kind: 'claim', ref: 'C-004', basis: 'fixture-C-004', verification: 'human_verified', settle_dispute: true })
    expect(await within(dialog).findByText('Last, check the answer.')).toBeInTheDocument()
  })

  it('ends on You have checked everything with Write the answer when there is no answer', async () => {
    current = {
      ...current,
      investigation: { ...current.investigation, conclusion: '' },
      evidence: current.evidence.filter((e) => e.ref === 'E-001' || e.ref === 'E-004'),
      claims: current.claims.filter((c) => c.ref !== 'C-004'),
    }
    const { user, dialog } = await renderWalk()
    expect(within(dialog).getByText('Source 1 of 1')).toBeInTheDocument()
    await user.click(within(dialog).getByRole('button', { name: SOURCE_CHECKED_ACTION }))
    expect(await within(dialog).findByText(ALL_CHECKED)).toBeInTheDocument()
    expect(within(dialog).queryByLabelText(WHAT_DID_YOU_CHECK)).toBeNull()
    const writeIt = within(dialog).getByRole('button', { name: WRITE_THE_ANSWER })
    expect(writeIt).toHaveFocus()
    await user.click(writeIt)
    expect(onWriteAnswer).toHaveBeenCalled()
  })
})
