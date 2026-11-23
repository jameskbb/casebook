import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Api } from '../../api/api'
import { DIALOG_COPY } from '../../api/errors'
import type { Investigation } from '../../api/types'
import { AppDataProvider } from '../../state/appData'
import { COPIED_FOR_AI, CopyContextProvider } from '../../state/copyContext'
import { investigation } from '../../test/fixtures'
import { mockApi } from '../../test/mockApi'
import { OpenAtLaunch } from '../../test/OpenAtLaunch'
import { ToastProvider } from '../ui/Toaster'
import { DetailHeader } from './DetailHeader'
import { AI_OFF_COPY, AI_OFF_LABEL, AI_ON_COPY, AI_ON_LABEL, ARCHIVED_COPY, BACK_IN_LIST_COPY, COPY_FOR_AI_LABEL, PUT_BACK_LABEL, SAVE_COPY_LABEL } from './MoreMenu'

const mocks = vi.hoisted(() => ({ api: {} as Api }))
vi.mock('../../api/api', () => ({ api: mocks.api }))

const inactive: Investigation = {
  ...investigation,
  ref: 'INV-0003',
  title: 'Why was March payroll $18,000 over budget?',
  is_active: false,
}

function renderHeader(inv: Investigation) {
  return render(
    <ToastProvider>
      <AppDataProvider><OpenAtLaunch />
        <CopyContextProvider>
          <DetailHeader investigation={inv} />
        </CopyContextProvider>
      </AppDataProvider>
    </ToastProvider>,
  )
}

async function openMore(user: ReturnType<typeof userEvent.setup>) {
  await user.click(screen.getByRole('button', { name: 'More' }))
  return screen.findByRole('menu', { name: 'More' })
}

describe('DetailHeader actions', () => {
  beforeEach(() => {
    Object.assign(mocks.api, mockApi())
  })

  it('shows a plain marker while the AI is working on the question, and More is the only button', () => {
    const { container } = renderHeader(investigation)
    const right = container.querySelector('.detail-header-right') as HTMLElement
    expect(within(right).getByText('Your AI is working on this')).toBeInTheDocument()
    // A marker, not a switch: nothing to click by accident.
    expect(within(right).getAllByRole('button').map((b) => b.getAttribute('aria-label') ?? b.textContent)).toEqual(['More'])
  })

  it('shows no marker when the AI is not working on it, and never on an archived question', () => {
    const { unmount } = renderHeader(inactive)
    expect(screen.queryByText('Your AI is working on this')).toBeNull()
    unmount()
    renderHeader({ ...investigation, status: 'archived', archived: true, archived_at: '2026-11-04T11:00:00.000Z' })
    expect(screen.queryByText('Your AI is working on this')).toBeNull()
  })

  it('has the AI work on a question from the More menu', async () => {
    const user = userEvent.setup()
    renderHeader(inactive)
    const menu = await openMore(user)
    expect(within(menu).queryByRole('menuitem', { name: AI_OFF_LABEL })).toBeNull()
    await user.click(within(menu).getByRole('menuitem', { name: AI_ON_LABEL }))
    expect(mocks.api.setActiveInvestigation).toHaveBeenCalledWith('INV-0003')
    expect(await screen.findByText(AI_ON_COPY)).toBeInTheDocument()
  })

  it('stops the AI working on a question from the More menu', async () => {
    const user = userEvent.setup()
    renderHeader(investigation)
    const menu = await openMore(user)
    expect(within(menu).queryByRole('menuitem', { name: AI_ON_LABEL })).toBeNull()
    await user.click(within(menu).getByRole('menuitem', { name: AI_OFF_LABEL }))
    expect(mocks.api.clearActiveInvestigation).toHaveBeenCalledTimes(1)
    expect(mocks.api.clearActiveInvestigation).toHaveBeenCalledWith(investigation.ref)
    expect(await screen.findByText(AI_OFF_COPY)).toBeInTheDocument()
  })

  it('says the AI is no longer working here when it had already moved to another question', async () => {
    Object.assign(mocks.api, mockApi({ clearActiveInvestigation: async () => false }))
    const user = userEvent.setup()
    renderHeader(investigation)
    await user.click(within(await openMore(user)).getByRole('menuitem', { name: AI_OFF_LABEL }))
    expect(await screen.findByText(AI_OFF_COPY)).toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('lists the More menu in order, with one way to check and so no entry for it', async () => {
    const user = userEvent.setup()
    renderHeader(inactive)
    const menu = await openMore(user)
    expect(within(menu).getAllByRole('menuitem').map((i) => i.textContent)).toEqual([
      'Edit question...',
      AI_ON_LABEL,
      COPY_FOR_AI_LABEL,
      SAVE_COPY_LABEL,
      'Archive question',
    ])
    expect(within(menu).queryByRole('menuitem', { name: /check/i })).not.toBeInTheDocument()
  })

  it('lists Put back in the list and Delete question... for an archived question', async () => {
    const user = userEvent.setup()
    renderHeader({ ...inactive, status: 'archived', archived: true, archived_at: '2026-11-04T11:00:00.000Z' })
    const menu = await openMore(user)
    expect(within(menu).getAllByRole('menuitem').map((i) => i.textContent)).toEqual([
      'Edit question...',
      COPY_FOR_AI_LABEL,
      SAVE_COPY_LABEL,
      PUT_BACK_LABEL,
      'Delete question...',
    ])
  })

  it('opens the edit dialog from the menu', async () => {
    const user = userEvent.setup()
    renderHeader(investigation)
    const menu = await openMore(user)
    await user.click(within(menu).getByRole('menuitem', { name: 'Edit question...' }))
    expect(await screen.findByRole('dialog', { name: 'Edit question' })).toBeInTheDocument()
  })

  it('copies the question for an AI from the menu and says so', async () => {
    const user = userEvent.setup()
    renderHeader(investigation)
    const menu = await openMore(user)
    await user.click(within(menu).getByRole('menuitem', { name: COPY_FOR_AI_LABEL }))
    expect(mocks.api.copyContext).toHaveBeenCalledWith('INV-0001')
    expect(mocks.api.copyContextPlain).not.toHaveBeenCalled()
    expect(await screen.findByText(COPIED_FOR_AI)).toBeInTheDocument()
  })

  it('saves a copy of the question and names the file', async () => {
    const user = userEvent.setup()
    renderHeader(investigation)
    const menu = await openMore(user)
    await user.click(within(menu).getByRole('menuitem', { name: SAVE_COPY_LABEL }))
    expect(mocks.api.exportInvestigation).toHaveBeenCalledWith('INV-0001')
    expect(await screen.findByText('Saved a copy as INV-0001-why-did-calls-go-up.json.')).toBeInTheDocument()
  })

  it('shows nothing when the export dialog is cancelled', async () => {
    Object.assign(mocks.api, mockApi({ exportInvestigation: async () => ({ cancelled: true, path: '', file_name: '' }) }))
    const user = userEvent.setup()
    renderHeader(investigation)
    const menu = await openMore(user)
    await user.click(within(menu).getByRole('menuitem', { name: SAVE_COPY_LABEL }))
    await waitFor(() => expect(mocks.api.exportInvestigation).toHaveBeenCalled())
    await new Promise((r) => setTimeout(r, 20))
    expect(screen.queryByText(/Saved a copy/)).not.toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('reports a native dialog failure', async () => {
    Object.assign(
      mocks.api,
      mockApi({
        exportInvestigation: async () => {
          throw JSON.stringify({ kind: 'other', field: 'dialog', message: 'dialog failed' })
        },
      }),
    )
    const user = userEvent.setup()
    renderHeader(investigation)
    const menu = await openMore(user)
    await user.click(within(menu).getByRole('menuitem', { name: SAVE_COPY_LABEL }))
    expect(await screen.findByRole('alert')).toHaveTextContent(DIALOG_COPY)
  })

  it('archives with Undo', async () => {
    const user = userEvent.setup()
    renderHeader(inactive)
    const menu = await openMore(user)
    await user.click(within(menu).getByRole('menuitem', { name: 'Archive question' }))
    expect(mocks.api.archiveInvestigation).toHaveBeenCalledWith('INV-0003')
    const toast = (await screen.findByText(ARCHIVED_COPY)).closest('.toast') as HTMLElement
    await user.click(within(toast).getByRole('button', { name: 'Undo' }))
    expect(mocks.api.unarchiveInvestigation).toHaveBeenCalledWith('INV-0003')
  })

  it('puts an archived question back in the list', async () => {
    const user = userEvent.setup()
    renderHeader({ ...inactive, status: 'archived', archived: true, archived_at: '2026-11-04T11:00:00.000Z' })
    const menu = await openMore(user)
    await user.click(within(menu).getByRole('menuitem', { name: PUT_BACK_LABEL }))
    expect(mocks.api.unarchiveInvestigation).toHaveBeenCalledWith('INV-0003')
    expect(await screen.findByText(BACK_IN_LIST_COPY)).toBeInTheDocument()
  })
})
