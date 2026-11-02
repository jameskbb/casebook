// AR-10: every delete says what it removes and that it cannot be undone,
// explains what to do first when something depends on the record, and shows
// an engine conflict in plain words.
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Api } from '../../api/api'
import type { InvestigationDetail } from '../../api/types'
import { AppDataProvider } from '../../state/appData'
import { CopyContextProvider } from '../../state/copyContext'
import { detail, summaries } from '../../test/fixtures'
import { archivedDetail } from '../../test/fixtures.detail'
import { bareEvidence, detailWithBareEvidence } from '../../test/fixtures.records'
import { mockApi } from '../../test/mockApi'
import { IRREVERSIBLE } from '../dialogs/DeleteDialog'
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
      <AppDataProvider>
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

describe('deleting records (AR-10)', () => {
  beforeEach(() => {
    Object.assign(mocks.api, mockApi())
  })

  it('refuses linked evidence with what to do first, and offers no Delete', async () => {
    const user = renderDetail()
    await openRecord(user, /^Open evidence E-001:/)
    const drawer = screen.getByRole('dialog', { name: 'Evidence E-001' })
    await user.click(within(drawer).getByRole('button', { name: 'More actions' }))
    await user.click(screen.getByRole('menuitem', { name: 'Delete evidence...' }))
    const dialog = screen.getByRole('dialog', { name: 'Delete evidence E-001?' })
    expect(dialog).toHaveTextContent(/E-001 is linked to C-001.* Unlink it there first/)
    expect(within(dialog).queryByRole('button', { name: 'Delete evidence' })).toBeNull()
    expect(mocks.api.deleteEvidence).not.toHaveBeenCalled()
  })

  it('deletes unlinked evidence after a confirmation that names it and says it is permanent', async () => {
    const user = renderDetail(detailWithBareEvidence())
    await openRecord(user, /^Open evidence E-009:/)
    const drawer = screen.getByRole('dialog', { name: 'Evidence E-009' })
    await user.click(within(drawer).getByRole('button', { name: 'More actions' }))
    await user.click(screen.getByRole('menuitem', { name: 'Delete evidence...' }))
    const dialog = screen.getByRole('dialog', { name: 'Delete evidence E-009?' })
    expect(dialog).toHaveTextContent(`E-009 "${bareEvidence.title}" will be removed`)
    expect(dialog).toHaveTextContent(IRREVERSIBLE)
    expect(within(dialog).getByRole('button', { name: 'Cancel' })).toHaveFocus()
    await user.click(within(dialog).getByRole('button', { name: 'Delete evidence' }))
    expect(mocks.api.deleteEvidence).toHaveBeenCalledWith('INV-0001', 'E-009')
    expect(await screen.findByText('Deleted evidence E-009.')).toBeInTheDocument()
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Evidence E-009' })).toBeNull())
  })

  it('shows an engine conflict in plain words and deletes nothing', async () => {
    const user = renderDetail(detailWithBareEvidence(), {
      deleteEvidence: async () =>
        Promise.reject(new Error('{"kind":"conflict","message":"conflict: E-009 is linked by 1 claim link and 0 hypothesis links; unlink it first"}')),
    })
    await openRecord(user, /^Open evidence E-009:/)
    const drawer = screen.getByRole('dialog', { name: 'Evidence E-009' })
    await user.click(within(drawer).getByRole('button', { name: 'More actions' }))
    await user.click(screen.getByRole('menuitem', { name: 'Delete evidence...' }))
    const dialog = screen.getByRole('dialog', { name: 'Delete evidence E-009?' })
    await user.click(within(dialog).getByRole('button', { name: 'Delete evidence' }))
    expect(await within(dialog).findByRole('alert')).toHaveTextContent(
      'E-009 is linked by 1 claim link and 0 hypothesis links; unlink it first. Nothing was deleted.',
    )
  })

  it('deletes a claim, and refuses one that replaces another', async () => {
    const user = renderDetail()
    await openRecord(user, /^Open claim C-001:/)
    let drawer = screen.getByRole('dialog', { name: 'Claim C-001' })
    await user.click(within(drawer).getByRole('button', { name: 'Delete claim...' }))
    let dialog = screen.getByRole('dialog', { name: 'Delete claim C-001?' })
    expect(dialog).toHaveTextContent('The evidence itself stays.')
    await user.click(within(dialog).getByRole('button', { name: 'Delete claim' }))
    expect(mocks.api.deleteClaim).toHaveBeenCalledWith('INV-0001', 'C-001')

    await openRecord(user, /^Open claim C-002:/)
    drawer = screen.getByRole('dialog', { name: 'Claim C-002' })
    await user.click(within(drawer).getByRole('button', { name: 'Delete claim...' }))
    dialog = screen.getByRole('dialog', { name: 'Delete claim C-002?' })
    expect(dialog).toHaveTextContent('C-002 replaces C-003. Bring it back with a status, or delete it, first.')
    expect(within(dialog).queryByRole('button', { name: 'Delete claim' })).toBeNull()
  })

  it('deletes a question from its row', async () => {
    const user = renderDetail()
    await user.click(await screen.findByRole('button', { name: 'Delete question Q-001' }))
    const dialog = screen.getByRole('dialog', { name: 'Delete question Q-001?' })
    expect(dialog).toHaveTextContent(IRREVERSIBLE)
    await user.click(within(dialog).getByRole('button', { name: 'Delete question' }))
    expect(mocks.api.deleteQuestion).toHaveBeenCalledWith('INV-0001', 'Q-001')
  })

  it('offers Delete case only once archived, with the counts it removes', async () => {
    const user = renderDetail(archivedDetail())
    await user.click(await screen.findByRole('button', { name: 'More actions' }))
    await user.click(screen.getByRole('menuitem', { name: 'Delete case...' }))
    const dialog = screen.getByRole('dialog', { name: 'Delete INV-0003?' })
    expect(dialog).toHaveTextContent(/will be removed with everything in it: \d+ pieces? of evidence/)
    expect(dialog).toHaveTextContent('Export it first')
    await user.click(within(dialog).getByRole('button', { name: 'Delete case' }))
    expect(mocks.api.deleteInvestigation).toHaveBeenCalledWith('INV-0003')
    expect(await screen.findByText('Deleted INV-0003.')).toBeInTheDocument()
  })

  it('does not offer Delete case while it is live', async () => {
    const user = renderDetail()
    await user.click(await screen.findByRole('button', { name: 'More actions' }))
    expect(screen.queryByRole('menuitem', { name: 'Delete case...' })).toBeNull()
  })
})
