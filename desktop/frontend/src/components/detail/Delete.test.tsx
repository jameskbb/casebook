// Every delete says what it removes and that it cannot be undone,
// explains what to do first when something depends on the record, and shows
// an engine conflict in plain words.
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Api } from '../../api/api'
import type { InvestigationDetail } from '../../api/types'
import { AppDataProvider } from '../../state/appData'
import { CopyContextProvider } from '../../state/copyContext'
import { claims, detail, investigation, summaries } from '../../test/fixtures'
import { archivedDetail } from '../../test/fixtures.detail'
import { bareEvidence, detailWithBareEvidence } from '../../test/fixtures.records'
import { mockApi } from '../../test/mockApi'
import { OpenAtLaunch } from '../../test/OpenAtLaunch'
import { findSourceButton } from '../../test/openQuestion'
import { IRREVERSIBLE } from '../dialogs/DeleteDialog'
import { deleteEvidenceBody, evidenceLinkedBlocker } from '../drawers/EvidenceDrawer'
import { deleteStatementBody, replacesBlocker } from '../drawers/StatementDrawer'
import { ToastProvider } from '../ui/Toaster'
import { DetailView } from './DetailView'

const mocks = vi.hoisted(() => ({ api: {} as Api }))
vi.mock('../../api/api', () => ({ api: mocks.api }))

function renderDetail(d: InvestigationDetail = detail, overrides: Partial<Api> = {}) {
  Object.assign(
    mocks.api,
    mockApi({
      listInvestigations: async () => [{ ...summaries[0], ref: d.investigation.ref, is_active: true }],
      getInvestigationDetail: async () => d,
      ...overrides,
    }),
  )
  render(
    <ToastProvider>
      <AppDataProvider><OpenAtLaunch />
        <CopyContextProvider>
          <DetailView />
        </CopyContextProvider>
      </AppDataProvider>
    </ToastProvider>,
  )
  return userEvent.setup()
}

const openRecord = async (user: ReturnType<typeof userEvent.setup>, name: RegExp) =>
  user.click(await screen.findByRole('button', { name }))

describe('deleting records', () => {
  beforeEach(() => {
    Object.assign(mocks.api, mockApi())
  })

  it('says a source is used by a finding, with what to do first and only OK', async () => {
    const user = renderDetail()
    await user.click(await findSourceButton('E-001'))
    const drawer = screen.getByRole('dialog', { name: 'Source E-001' })
    await user.click(within(drawer).getByRole('button', { name: 'More' }))
    await user.click(screen.getByRole('menuitem', { name: 'Delete source...' }))
    // The title says what stands in the way; it does not ask a question there is no way to say yes to.
    expect(screen.queryByRole('dialog', { name: 'Delete this source?' })).toBeNull()
    const dialog = screen.getByRole('dialog', { name: 'This source is used by a finding' })
    expect(dialog).toHaveTextContent(evidenceLinkedBlocker(['C-001']))
    expect(dialog).not.toHaveTextContent('C-001')
    expect(dialog).not.toHaveTextContent(IRREVERSIBLE)
    // Besides the window's own X, OK is the only button.
    const buttons = within(dialog).getAllByRole('button').map((b) => b.textContent || b.getAttribute('aria-label'))
    expect(buttons.filter((b) => b !== 'Close')).toEqual(['OK'])
    await user.click(within(dialog).getByRole('button', { name: 'OK' }))
    expect(screen.queryByRole('dialog', { name: 'This source is used by a finding' })).toBeNull()
    expect(mocks.api.deleteEvidence).not.toHaveBeenCalled()
  })

  it('deletes unlinked evidence after a confirmation that names it and says it is permanent', async () => {
    const user = renderDetail(detailWithBareEvidence())
    await openRecord(user, /^Open source E-009:/)
    const drawer = screen.getByRole('dialog', { name: 'Source E-009' })
    await user.click(within(drawer).getByRole('button', { name: 'More' }))
    await user.click(screen.getByRole('menuitem', { name: 'Delete source...' }))
    const dialog = screen.getByRole('dialog', { name: 'Delete this source?' })
    expect(dialog).toHaveTextContent(deleteEvidenceBody(bareEvidence))
    expect(dialog).toHaveTextContent(`"${bareEvidence.title}"`)
    expect(dialog).not.toHaveTextContent('E-009')
    expect(dialog).toHaveTextContent(IRREVERSIBLE)
    expect(within(dialog).getByRole('button', { name: 'Cancel' })).toHaveFocus()
    await user.click(within(dialog).getByRole('button', { name: 'Delete source' }))
    expect(mocks.api.deleteEvidence).toHaveBeenCalledWith('INV-0001', 'E-009')
    expect(await screen.findByText('Deleted the source.')).toBeInTheDocument()
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Source E-009' })).toBeNull())
  })

  it('shows an engine conflict in plain words and deletes nothing', async () => {
    const user = renderDetail(detailWithBareEvidence(), {
      deleteEvidence: async () =>
        Promise.reject(new Error('{"kind":"conflict","message":"conflict: E-009 is linked by 1 claim link and 0 hypothesis links; unlink it first"}')),
    })
    await openRecord(user, /^Open source E-009:/)
    const drawer = screen.getByRole('dialog', { name: 'Source E-009' })
    await user.click(within(drawer).getByRole('button', { name: 'More' }))
    await user.click(screen.getByRole('menuitem', { name: 'Delete source...' }))
    const dialog = screen.getByRole('dialog', { name: 'Delete this source?' })
    await user.click(within(dialog).getByRole('button', { name: 'Delete source' }))
    const alert = await within(dialog).findByRole('alert')
    expect(alert).toHaveTextContent(/^A finding uses this source\. Remove it from that finding first\. Nothing was deleted\.$/)
    // None of the engine's own words reach the page.
    expect(dialog).not.toHaveTextContent(/E-009|claim|hypothesis|unlink|conflict/)
  })

  it('takes a finding out of the answer before deleting it', async () => {
    // C-001 is named on both sides of the answer here, so both lists must let go of it.
    const user = renderDetail({
      ...detail,
      investigation: { ...investigation, conclusion_supporting_claims: ['C-001', 'C-002'], conclusion_contradicting_claims: ['C-004', 'C-001'] },
    })
    await openRecord(user, /^Open finding C-001:/)
    const drawer = screen.getByRole('dialog', { name: 'Finding C-001' })
    await user.click(within(drawer).getByRole('button', { name: 'Delete finding...' }))
    const dialog = screen.getByRole('dialog', { name: 'Delete this finding?' })
    expect(dialog).toHaveTextContent(deleteStatementBody(claims[0]))
    expect(dialog).toHaveTextContent('Its sources stay.')
    expect(dialog).toHaveTextContent(IRREVERSIBLE)
    await user.click(within(dialog).getByRole('button', { name: 'Delete finding' }))
    await waitFor(() => expect(mocks.api.deleteClaim).toHaveBeenCalledWith('INV-0001', 'C-001'))
    expect(mocks.api.updateConclusion).toHaveBeenCalledTimes(1)
    expect(mocks.api.updateConclusion).toHaveBeenCalledWith('INV-0001', {
      conclusion: investigation.conclusion,
      confidence: investigation.conclusion_confidence,
      rationale: investigation.conclusion_rationale,
      supporting_claims: ['C-002'],
      contradicting_claims: ['C-004'],
    })
    // The engine keeps a finding the answer is based on, so the answer is saved first.
    expect(vi.mocked(mocks.api.updateConclusion).mock.invocationCallOrder[0]).toBeLessThan(
      vi.mocked(mocks.api.deleteClaim).mock.invocationCallOrder[0],
    )
    expect(await screen.findByText('Deleted the finding.')).toBeInTheDocument()
  })

  it('deletes a finding the answer does not name without touching the answer', async () => {
    const user = renderDetail()
    expect(investigation.conclusion_supporting_claims).not.toContain('C-004')
    await openRecord(user, /^Open finding C-004:/)
    const drawer = screen.getByRole('dialog', { name: 'Finding C-004' })
    await user.click(within(drawer).getByRole('button', { name: 'Delete finding...' }))
    const dialog = screen.getByRole('dialog', { name: 'Delete this finding?' })
    await user.click(within(dialog).getByRole('button', { name: 'Delete finding' }))
    await waitFor(() => expect(mocks.api.deleteClaim).toHaveBeenCalledWith('INV-0001', 'C-004'))
    expect(mocks.api.updateConclusion).not.toHaveBeenCalled()
  })

  it('refuses a finding that replaced another one, with what to do first', async () => {
    const user = renderDetail()
    await openRecord(user, /^Open finding C-002:/)
    const drawer = screen.getByRole('dialog', { name: 'Finding C-002' })
    await user.click(within(drawer).getByRole('button', { name: 'Delete finding...' }))
    const dialog = screen.getByRole('dialog', { name: 'This finding replaced another one' })
    expect(dialog).toHaveTextContent(replacesBlocker(['C-003']))
    expect(within(dialog).queryByRole('button', { name: 'Delete finding' })).toBeNull()
    expect(within(dialog).getByRole('button', { name: 'OK' })).toBeInTheDocument()
    expect(mocks.api.deleteClaim).not.toHaveBeenCalled()
  })

  it('deletes a question from its row', async () => {
    const user = renderDetail()
    await user.click(await screen.findByRole('button', { name: 'Delete follow-up question: Have the customers who were charged twice been paid back?' }))
    const dialog = screen.getByRole('dialog', { name: 'Delete this follow-up question?' })
    expect(dialog).toHaveTextContent(IRREVERSIBLE)
    await user.click(within(dialog).getByRole('button', { name: 'Delete follow-up question' }))
    expect(mocks.api.deleteQuestion).toHaveBeenCalledWith('INV-0001', 'Q-001')
  })

  it('offers Delete question only once archived, with the counts it removes', async () => {
    const user = renderDetail(archivedDetail())
    await user.click(await screen.findByRole('button', { name: 'More' }))
    await user.click(screen.getByRole('menuitem', { name: 'Delete question...' }))
    const dialog = screen.getByRole('dialog', { name: 'Delete this question?' })
    expect(dialog).toHaveTextContent(/will be removed with everything in it: \d+ sources?/)
    expect(dialog).toHaveTextContent('Save a copy first if you might need it again.')
    expect(dialog).not.toHaveTextContent('INV-0003')
    await user.click(within(dialog).getByRole('button', { name: 'Delete question' }))
    expect(mocks.api.deleteInvestigation).toHaveBeenCalledWith('INV-0003')
    expect(await screen.findByText('Deleted the question.')).toBeInTheDocument()
  })

  it('does not offer Delete question while it is live', async () => {
    const user = renderDetail()
    await user.click(await screen.findByRole('button', { name: 'More' }))
    expect(screen.getByRole('menuitem', { name: 'Archive question' })).toBeInTheDocument()
    expect(screen.queryByRole('menuitem', { name: 'Delete question...' })).toBeNull()
  })
})
