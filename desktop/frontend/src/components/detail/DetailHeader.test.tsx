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
import { ACTIVE_CLEARED_COPY, ACTIVE_OFF_LABEL, ACTIVE_ON_LABEL, ACTIVE_SET_COPY, ACTIVE_TITLE_OFF, ACTIVE_TITLE_ON } from './ActiveToggle'
import { DetailHeader } from './DetailHeader'
import { ARCHIVED_COPY, BACK_IN_LIST_COPY, CHECK_FINDINGS_LABEL, COPY_FOR_AI_LABEL, PUT_BACK_LABEL, SAVE_COPY_LABEL } from './MoreMenu'

const mocks = vi.hoisted(() => ({ api: {} as Api }))
vi.mock('../../api/api', () => ({ api: mocks.api }))

const inactive: Investigation = {
  ...investigation,
  ref: 'INV-0003',
  title: 'Why was March payroll $18k over budget?',
  is_active: false,
}

function renderHeader(inv: Investigation, onCheckFindings?: () => void) {
  return render(
    <ToastProvider>
      <AppDataProvider><OpenAtLaunch />
        <CopyContextProvider>
          <DetailHeader investigation={inv} onCheckFindings={onCheckFindings} />
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

  it('orders the right cluster: the AI toggle, then More, with no Copy for your AI button', () => {
    const { container } = renderHeader(investigation)
    const right = container.querySelector('.detail-header-right') as HTMLElement
    const names = within(right)
      .getAllByRole('button')
      .map((b) => b.getAttribute('aria-label') ?? b.textContent)
    expect(names).toEqual([ACTIVE_ON_LABEL, 'More'])
    expect(screen.queryByRole('button', { name: /Copy/ })).not.toBeInTheDocument()
  })

  it('sets an inactive investigation active', async () => {
    const user = userEvent.setup()
    renderHeader(inactive)
    const toggle = screen.getByRole('button', { name: ACTIVE_OFF_LABEL })
    expect(toggle).toHaveAttribute('aria-pressed', 'false')
    expect(toggle).toHaveAttribute('title', ACTIVE_TITLE_OFF)
    await user.click(toggle)
    expect(mocks.api.setActiveInvestigation).toHaveBeenCalledWith('INV-0003')
    expect(await screen.findByText(ACTIVE_SET_COPY)).toBeInTheDocument()
  })

  it('clears the active investigation', async () => {
    const user = userEvent.setup()
    renderHeader(investigation)
    const toggle = screen.getByRole('button', { name: ACTIVE_ON_LABEL })
    expect(toggle).toHaveAttribute('aria-pressed', 'true')
    expect(toggle).toHaveAttribute('title', ACTIVE_TITLE_ON)
    await user.click(toggle)
    expect(mocks.api.clearActiveInvestigation).toHaveBeenCalledTimes(1)
    expect(mocks.api.clearActiveInvestigation).toHaveBeenCalledWith(investigation.ref)
    expect(await screen.findByText(ACTIVE_CLEARED_COPY)).toBeInTheDocument()
  })

  it('says the AI is no longer working here when it had already moved to another question', async () => {
    Object.assign(mocks.api, mockApi({ clearActiveInvestigation: async () => false }))
    const user = userEvent.setup()
    renderHeader(investigation)
    await user.click(screen.getByRole('button', { name: ACTIVE_ON_LABEL }))
    expect(mocks.api.clearActiveInvestigation).toHaveBeenCalledWith(investigation.ref)
    expect(await screen.findByText(ACTIVE_CLEARED_COPY)).toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('is busy while the call runs', async () => {
    let resolve: () => void = () => {}
    Object.assign(
      mocks.api,
      mockApi({ setActiveInvestigation: () => new Promise((r) => (resolve = () => r({ ...inactive, is_active: true }))) }),
    )
    const user = userEvent.setup()
    renderHeader(inactive)
    const toggle = screen.getByRole('button', { name: ACTIVE_OFF_LABEL })
    await user.click(toggle)
    expect(toggle).toBeDisabled()
    expect(toggle).toHaveAttribute('aria-busy', 'true')
    resolve()
    await waitFor(() => expect(toggle).not.toBeDisabled())
  })

  it('lists the More menu in order, with no Copy question as text', async () => {
    const user = userEvent.setup()
    renderHeader(investigation)
    const menu = await openMore(user)
    expect(within(menu).getAllByRole('menuitem').map((i) => i.textContent)).toEqual([
      'Edit question...',
      COPY_FOR_AI_LABEL,
      SAVE_COPY_LABEL,
      'Archive question',
    ])
    expect(within(menu).queryByRole('menuitem', { name: 'Copy question as text' })).not.toBeInTheDocument()
    expect(within(menu).queryByRole('menuitem', { name: CHECK_FINDINGS_LABEL })).not.toBeInTheDocument()
  })

  it('offers Check findings one by one only with onCheckFindings, and calls it', async () => {
    const user = userEvent.setup()
    const onCheckFindings = vi.fn()
    renderHeader(investigation, onCheckFindings)
    const menu = await openMore(user)
    expect(within(menu).getAllByRole('menuitem').map((i) => i.textContent)).toEqual([
      'Edit question...',
      CHECK_FINDINGS_LABEL,
      COPY_FOR_AI_LABEL,
      SAVE_COPY_LABEL,
      'Archive question',
    ])
    await user.click(within(menu).getByRole('menuitem', { name: CHECK_FINDINGS_LABEL }))
    expect(onCheckFindings).toHaveBeenCalledTimes(1)
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
