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
import { RULE_OUT_ACTION, STILL_HOLDS_ACTION, STILL_HOLDS_QUESTION } from '../drawers/StatementDrawer'
import { ALL_DECIDED, ALL_SOURCES_CHECKED, CHECK_SOURCES_TITLE, CheckSources, checkStep, DECIDE_TITLE, endText, goBackText, SKIP_ACTION, skippedText, SOURCE_CHECKED_ACTION, SOURCE_QUESTION, stillHoldsWhy, WHY_STILL_HOLDS, WRITE_THE_ANSWER } from './CheckSources'
import { CHECKED_ANSWER_ACTION, WHAT_DID_YOU_CHECK } from './ConclusionPanel'
import { ADD_FINDING } from './FirstFindingGuide'
import { ADD_FINDING_NEED } from '../../lib/questions'

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

  it('offers a source the person added without ticking that they looked at it, and not one they ticked', () => {
    const you = { kind: 'human' as const, client: 'casebook-ui' }
    const added = { ...evidence[0], ref: 'E-010', source_type: 'manual_observation' as const, created_by: you, verification: 'ai_generated' as const }
    const ticked = { ...added, ref: 'E-011', verification: 'human_verified' as const }
    expect(checkStep([ticked, added], [], 0)).toMatchObject({ current: { ref: 'E-010' }, total: 1 })
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
  const onAddFinding = vi.fn()

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
    onAddFinding.mockClear()
  })

  function Harness() {
    const { detail } = useAppData()
    const [open, setOpen] = useState(false)
    return (
      <>
        <button onClick={() => setOpen(true)}>Open</button>
        {open && detail && <CheckSources detail={detail} onClose={() => setOpen(false)} onWriteAnswer={onWriteAnswer} onAddFinding={onAddFinding} />}
      </>
    )
  }

  const renderWalk = async () => {
    const user = userEvent.setup()
    const wrap = (ui: ReactNode) => <ToastProvider><AppDataProvider><OpenAtLaunch />{ui}</AppDataProvider></ToastProvider>
    render(wrap(<Harness />))
    await waitFor(() => expect(mocks.api.getInvestigationDetail).toHaveBeenCalled())
    await user.click(screen.getByRole('button', { name: 'Open' }))
    return { user, dialog: await screen.findByRole('dialog') }
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
    // Then the finding where sources disagree, with its sources both ways,
    // under its own title, and focus on the question rather than a decision.
    const question = await within(dialog).findByRole('heading', { name: STILL_HOLDS_QUESTION })
    expect(dialog).toHaveAccessibleName(DECIDE_TITLE)
    expect(question).toHaveFocus()
    expect(within(dialog).getByText('Finding 1 of 1')).toBeInTheDocument()
    expect(within(dialog).getByText('Callers waited too long and hung up before buying.')).toBeInTheDocument()
    expect(within(dialog).getByText('Goes against it')).toBeInTheDocument()
    expect(within(dialog).getByRole('button', { name: STILL_HOLDS_ACTION })).not.toHaveFocus()
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

  it('starts each source on its question so Enter checks nothing unread, and Escape closes with focus back on the opener', async () => {
    const { user, dialog } = await renderWalk()
    expect(within(dialog).getByRole('heading', { name: SOURCE_QUESTION })).toHaveFocus()
    await user.keyboard('{Enter}')
    expect(mocks.api.setEvidenceVerification).not.toHaveBeenCalled()
    await user.click(within(dialog).getByRole('button', { name: SOURCE_CHECKED_ACTION }))
    expect(await within(dialog).findByText('Source 2 of 3')).toBeInTheDocument()
    expect(within(dialog).getByRole('heading', { name: SOURCE_QUESTION })).toHaveFocus()
    await user.keyboard('{Escape}')
    expect(screen.queryByRole('dialog', { name: CHECK_SOURCES_TITLE })).toBeNull()
    expect(screen.getByRole('button', { name: 'Open' })).toHaveFocus()
  })

  it('records It still holds as the person read the finding', async () => {
    current = { ...current, evidence: current.evidence.map((e) => ({ ...e, verification: 'human_verified' as const })) }
    const { user, dialog } = await renderWalk()
    expect(dialog).toHaveAccessibleName(DECIDE_TITLE)
    expect(within(dialog).getByRole('heading', { name: STILL_HOLDS_QUESTION })).toHaveFocus()
    // Enter on the question decides nothing.
    await user.keyboard('{Enter}')
    expect(mocks.api.reviewStatement).not.toHaveBeenCalled()
    expect(within(dialog).getByLabelText(WHY_STILL_HOLDS)).toHaveValue('')
    await user.click(within(dialog).getByRole('button', { name: STILL_HOLDS_ACTION }))
    expect(mocks.api.reviewStatement).toHaveBeenCalledWith('INV-0001', { kind: 'claim', ref: 'C-004', basis: 'fixture-C-004', verification: 'human_verified', settle_dispute: true })
    expect(await within(dialog).findByText('Last, check the answer.')).toBeInTheDocument()
  })

  it('shows the exact words of each source in the Decide step, and sends the reason with It still holds', async () => {
    current = { ...current, evidence: current.evidence.map((e) => ({ ...e, verification: 'human_verified' as const })) }
    Object.assign(mocks.api, { getEvidence: vi.fn(async (_inv: string, ref: string) => ({ ...current.evidence.find((e) => e.ref === ref)!, raw_detail: `Words of ${ref}` })) })
    const { user, dialog } = await renderWalk()
    expect(await within(dialog).findByText('Words of E-003')).toBeInTheDocument()
    await user.type(within(dialog).getByLabelText(WHY_STILL_HOLDS), 'The wait stayed the same, but the sales manager saw it.')
    await user.click(within(dialog).getByRole('button', { name: STILL_HOLDS_ACTION }))
    expect(mocks.api.reviewStatement).toHaveBeenCalledWith('INV-0001', expect.objectContaining({ ref: 'C-004', rationale: 'Still holds: The wait stayed the same, but the sales manager saw it.' }))
  })

  it("starts the reason field empty, sends nothing when it stays empty, and keeps the finding's why in front of a new reason", async () => {
    expect(stillHoldsWhy('', 'New reason.')).toBe('Still holds: New reason.')
    expect(stillHoldsWhy('The sales manager said so.', 'New reason.')).toBe('The sales manager said so. Still holds: New reason.')
    current = { ...current, evidence: current.evidence.map((e) => ({ ...e, verification: 'human_verified' as const })), claims: current.claims.map((c) => (c.ref === 'C-004' ? { ...c, confidence_rationale: 'The sales manager said so.' } : c)) }
    const { user, dialog } = await renderWalk()
    expect(within(dialog).getByLabelText(WHY_STILL_HOLDS)).toHaveValue('')
    await user.type(within(dialog).getByLabelText(WHY_STILL_HOLDS), 'The wait stayed the same.')
    await user.click(within(dialog).getByRole('button', { name: STILL_HOLDS_ACTION }))
    expect(vi.mocked(mocks.api.reviewStatement).mock.calls[0][1]).toHaveProperty('rationale', 'The sales manager said so. Still holds: The wait stayed the same.')
  })

  it('ends by saying what the person did: every source checked, and every finding decided only after deciding one', async () => {
    expect(endText(false)).toBe('You have checked every source.')
    expect(endText(true)).toBe('You have checked every source. You decided every finding where sources disagree.')
    current = { ...current, investigation: { ...current.investigation, conclusion: '' }, evidence: current.evidence.map((e) => ({ ...e, verification: 'human_verified' as const })) }
    const { user, dialog } = await renderWalk()
    await user.click(within(dialog).getByRole('button', { name: STILL_HOLDS_ACTION }))
    expect(await within(dialog).findByText(`${ALL_SOURCES_CHECKED} ${ALL_DECIDED}`)).toBeInTheDocument()
    expect(dialog).toHaveAccessibleName(CHECK_SOURCES_TITLE)
    expect(within(dialog).queryByText(/checked everything/)).toBeNull()
  })

  it('ends on You have checked every source with Write the answer when there is no answer', async () => {
    current = {
      ...current,
      investigation: { ...current.investigation, conclusion: '' },
      evidence: current.evidence.filter((e) => e.ref === 'E-001' || e.ref === 'E-004'),
      claims: current.claims.filter((c) => c.ref !== 'C-004'),
    }
    const { user, dialog } = await renderWalk()
    expect(within(dialog).getByText('Source 1 of 1')).toBeInTheDocument()
    await user.click(within(dialog).getByRole('button', { name: SOURCE_CHECKED_ACTION }))
    expect(await within(dialog).findByText(ALL_SOURCES_CHECKED)).toBeInTheDocument()
    expect(within(dialog).queryByText(ALL_DECIDED, { exact: false })).toBeNull()
    expect(within(dialog).queryByLabelText(WHAT_DID_YOU_CHECK)).toBeNull()
    const writeIt = within(dialog).getByRole('button', { name: WRITE_THE_ANSWER })
    expect(writeIt).toHaveFocus()
    await user.click(writeIt)
    expect(onWriteAnswer).toHaveBeenCalled()
  })

  it('ends on Add a finding, not Write the answer, when every finding was ruled out', async () => {
    current = {
      ...current,
      investigation: { ...current.investigation, conclusion: '' },
      evidence: current.evidence.map((e) => ({ ...e, verification: 'human_verified' as const })),
      claims: current.claims.filter((c) => c.ref === 'C-004'),
      hypotheses: [],
    }
    const { user, dialog } = await renderWalk()
    await user.click(within(dialog).getByRole('button', { name: RULE_OUT_ACTION }))
    expect(await within(dialog).findByText(ADD_FINDING_NEED)).toBeInTheDocument()
    expect(within(dialog).queryByRole('button', { name: WRITE_THE_ANSWER })).toBeNull()
    const addIt = within(dialog).getByRole('button', { name: ADD_FINDING })
    expect(addIt).toHaveFocus()
    await user.click(addIt)
    expect(onAddFinding).toHaveBeenCalled()
  })
})
