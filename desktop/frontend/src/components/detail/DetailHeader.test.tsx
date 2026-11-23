import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Api } from '../../api/api'
import type { Investigation } from '../../api/types'
import { AppDataProvider } from '../../state/appData'
import { CopyContextProvider } from '../../state/copyContext'
import { investigation } from '../../test/fixtures'
import { mockApi } from '../../test/mockApi'
import { OpenAtLaunch } from '../../test/OpenAtLaunch'
import { ToastProvider } from '../ui/Toaster'
import { DetailHeader } from './DetailHeader'

const mocks = vi.hoisted(() => ({ api: {} as Api }))
vi.mock('../../api/api', () => ({ api: mocks.api }))

const inactive: Investigation = {
  ...investigation,
  ref: 'INV-0003',
  title: 'Why was March payroll $18k over budget?',
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
  await user.click(screen.getByRole('button', { name: 'More actions' }))
  return screen.findByRole('menu', { name: 'More actions' })
}

describe('DetailHeader actions', () => {
  beforeEach(() => {
    Object.assign(mocks.api, mockApi())
  })

  it('orders the right cluster: the AI toggle, Copy for your AI, More actions (Share the answer lives in the answer panel)', () => {
    const { container } = renderHeader(investigation)
    const right = container.querySelector('.detail-header-right') as HTMLElement
    const names = within(right)
      .getAllByRole('button')
      .map((b) => b.getAttribute('aria-label') ?? b.textContent)
    expect(names).toEqual(["Your AI's current question", 'Copy for your AI', 'More actions'])
  })

  it('sets an inactive investigation active', async () => {
    const user = userEvent.setup()
    renderHeader(inactive)
    const toggle = screen.getByRole('button', { name: 'Use with my AI' })
    expect(toggle).toHaveAttribute('aria-pressed', 'false')
    expect(toggle).toHaveAttribute('title', 'Click so your AI starts from this question.')
    await user.click(toggle)
    expect(mocks.api.setActiveInvestigation).toHaveBeenCalledWith('INV-0003')
    expect(await screen.findByText("This is now your AI's current question.")).toBeInTheDocument()
  })

  it('clears the active investigation', async () => {
    const user = userEvent.setup()
    renderHeader(investigation)
    const toggle = screen.getByRole('button', { name: "Your AI's current question" })
    expect(toggle).toHaveAttribute('aria-pressed', 'true')
    expect(toggle).toHaveAttribute('title', 'Your AI starts from this question. Click to stop.')
    await user.click(toggle)
    expect(mocks.api.clearActiveInvestigation).toHaveBeenCalledTimes(1)
    expect(mocks.api.clearActiveInvestigation).toHaveBeenCalledWith(investigation.ref)
    expect(await screen.findByText("No question is your AI's current question now.")).toBeInTheDocument()
  })

  it('does not claim to clear when another investigation became active elsewhere', async () => {
    Object.assign(mocks.api, mockApi({ clearActiveInvestigation: async () => false }))
    const user = userEvent.setup()
    renderHeader(investigation)
    await user.click(screen.getByRole('button', { name: "Your AI's current question" }))
    expect(mocks.api.clearActiveInvestigation).toHaveBeenCalledWith(investigation.ref)
    expect(
      await screen.findByText("This was no longer your AI's current question, so nothing changed."),
    ).toBeInTheDocument()
    expect(screen.queryByText("No question is your AI's current question now.")).not.toBeInTheDocument()
  })

  it('is busy while the call runs', async () => {
    let resolve: () => void = () => {}
    Object.assign(
      mocks.api,
      mockApi({ setActiveInvestigation: () => new Promise((r) => (resolve = () => r({ ...inactive, is_active: true }))) }),
    )
    const user = userEvent.setup()
    renderHeader(inactive)
    const toggle = screen.getByRole('button', { name: 'Use with my AI' })
    await user.click(toggle)
    expect(toggle).toBeDisabled()
    expect(toggle).toHaveAttribute('aria-busy', 'true')
    resolve()
    await waitFor(() => expect(toggle).not.toBeDisabled())
  })

  it('lists the More actions', async () => {
    const user = userEvent.setup()
    renderHeader(investigation)
    const menu = await openMore(user)
    expect(within(menu).getAllByRole('menuitem').map((i) => i.textContent)).toEqual([
      'Edit question...',
      'Copy question as text',
      'Save as a file...',
      'Archive question',
    ])
  })

  it('opens the edit dialog from the menu', async () => {
    const user = userEvent.setup()
    renderHeader(investigation)
    const menu = await openMore(user)
    await user.click(within(menu).getByRole('menuitem', { name: 'Edit question...' }))
    expect(await screen.findByRole('dialog', { name: 'Edit question' })).toBeInTheDocument()
  })

  it('copies the case as text from the menu', async () => {
    const user = userEvent.setup()
    renderHeader(investigation)
    const menu = await openMore(user)
    await user.click(within(menu).getByRole('menuitem', { name: 'Copy question as text' }))
    expect(mocks.api.copyContextPlain).toHaveBeenCalledWith('INV-0001')
    expect(mocks.api.copyContext).not.toHaveBeenCalled()
    expect(await screen.findByText('Copied the question as text.')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Copied' })).toBeInTheDocument()
  })

  it('copies the framed context from the button', async () => {
    const user = userEvent.setup()
    renderHeader(investigation)
    await user.click(screen.getByRole('button', { name: 'Copy for your AI' }))
    expect(mocks.api.copyContext).toHaveBeenCalledWith('INV-0001')
    expect(mocks.api.copyContextPlain).not.toHaveBeenCalled()
  })

  it('saves the question as a file and says where', async () => {
    const user = userEvent.setup()
    renderHeader(investigation)
    const menu = await openMore(user)
    await user.click(within(menu).getByRole('menuitem', { name: 'Save as a file...' }))
    expect(mocks.api.exportInvestigation).toHaveBeenCalledWith('INV-0001')
    expect(await screen.findByText('Saved this question as INV-0001-why-did-calls-go-up.json.')).toBeInTheDocument()
  })

  it('shows nothing when the export dialog is cancelled', async () => {
    Object.assign(mocks.api, mockApi({ exportInvestigation: async () => ({ cancelled: true, path: '', file_name: '' }) }))
    const user = userEvent.setup()
    renderHeader(investigation)
    const menu = await openMore(user)
    await user.click(within(menu).getByRole('menuitem', { name: 'Save as a file...' }))
    await waitFor(() => expect(mocks.api.exportInvestigation).toHaveBeenCalled())
    await new Promise((r) => setTimeout(r, 20))
    expect(screen.queryByText(/Exported/)).not.toBeInTheDocument()
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
    await user.click(within(menu).getByRole('menuitem', { name: 'Save as a file...' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('Could not open the window for choosing a file. Try again.')
  })

  it('archives with Undo', async () => {
    const user = userEvent.setup()
    renderHeader(inactive)
    const menu = await openMore(user)
    await user.click(within(menu).getByRole('menuitem', { name: 'Archive question' }))
    expect(mocks.api.archiveInvestigation).toHaveBeenCalledWith('INV-0003')
    const toast = (await screen.findByText(
      'Archived this question. It is hidden from the list and nothing was deleted.',
    )).closest('.toast') as HTMLElement
    await user.click(within(toast).getByRole('button', { name: 'Undo' }))
    expect(mocks.api.unarchiveInvestigation).toHaveBeenCalledWith('INV-0003')
  })

  it('unarchives an archived investigation', async () => {
    const user = userEvent.setup()
    renderHeader({ ...inactive, status: 'archived', archived: true, archived_at: '2026-11-04T11:00:00.000Z' })
    const menu = await openMore(user)
    await user.click(within(menu).getByRole('menuitem', { name: 'Unarchive question' }))
    expect(mocks.api.unarchiveInvestigation).toHaveBeenCalledWith('INV-0003')
    expect(await screen.findByText('Unarchived this question. It is back in the list.')).toBeInTheDocument()
  })
})
