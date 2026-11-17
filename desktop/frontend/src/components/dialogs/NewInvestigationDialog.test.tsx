import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Api } from '../../api/api'
import { ApiError } from '../../api/errors'
import type { Investigation } from '../../api/types'
import { AppDataProvider, useAppData } from '../../state/appData'
import { investigation } from '../../test/fixtures'
import { mockApi } from '../../test/mockApi'
import { ToastProvider } from '../ui/Toaster'
import { NewInvestigationDialog } from './NewInvestigationDialog'

const mocks = vi.hoisted(() => ({ api: {} as Api }))
vi.mock('../../api/api', () => ({ api: mocks.api }))

function WhenReady({ onClose, onCreated }: { onClose: () => void; onCreated: (ref: string) => void }) {
  const { listStatus } = useAppData()
  return listStatus === 'ready' ? <NewInvestigationDialog onClose={onClose} onCreated={onCreated} /> : null
}

function renderDialog() {
  const onClose = vi.fn()
  const onCreated = vi.fn()
  render(
    <ToastProvider>
      <AppDataProvider>
        <WhenReady onClose={onClose} onCreated={onCreated} />
      </AppDataProvider>
    </ToastProvider>,
  )
  return { onClose, onCreated }
}

describe('NewInvestigationDialog', () => {
  beforeEach(() => {
    Object.assign(mocks.api, mockApi())
  })

  it('leaves Make this the active investigation unchecked when one is already active', async () => {
    renderDialog()
    const dialog = await screen.findByRole('dialog', { name: 'New case' })
    expect(within(dialog).getByLabelText("Make this your AI's current case")).not.toBeChecked()
    expect(within(dialog).getByText("A short name you'll recognize, like Northeast margin down 6 points in Q3")).toBeInTheDocument()
    expect(within(dialog).getByText('What are we trying to figure out?')).toBeInTheDocument()
  })

  it('shows Creating... and disables the submit while the call runs', async () => {
    let resolve: (v: Investigation) => void = () => {}
    Object.assign(
      mocks.api,
      mockApi({ createInvestigation: () => new Promise<Investigation>((r) => (resolve = r)) }),
    )
    const user = userEvent.setup()
    const { onClose, onCreated } = renderDialog()
    const dialog = await screen.findByRole('dialog', { name: 'New case' })
    await user.type(within(dialog).getByLabelText('Title'), 'Vendor contract renewals')
    await user.click(within(dialog).getByRole('button', { name: 'Create case' }))

    const busy = within(dialog).getByRole('button', { name: 'Creating...' })
    expect(busy).toBeDisabled()
    expect(mocks.api.createInvestigation).toHaveBeenCalledWith({
      title: 'Vendor contract renewals',
      problem_statement: '',
      make_active: false,
    })

    resolve({ ...investigation, ref: 'INV-0004', title: 'Vendor contract renewals' })
    await waitFor(() => expect(onCreated).toHaveBeenCalledWith('INV-0004'))
    expect(onClose).toHaveBeenCalled()
  })

  it('shows a title validation error under the Title field', async () => {
    Object.assign(
      mocks.api,
      mockApi({
        createInvestigation: async () => {
          throw new ApiError('validation', 'must be at most 200 bytes', 'title')
        },
      }),
    )
    const user = userEvent.setup()
    const { onClose } = renderDialog()
    const dialog = await screen.findByRole('dialog', { name: 'New case' })
    await user.type(within(dialog).getByLabelText('Title'), 'x')
    await user.click(within(dialog).getByRole('button', { name: 'Create case' }))

    const title = within(dialog).getByLabelText('Title')
    await waitFor(() => expect(title).toHaveAttribute('aria-invalid', 'true'))
    expect(title).toHaveAccessibleDescription(expect.stringContaining('must be at most 200 bytes'))
    expect(title).toHaveFocus()
    expect(within(dialog).getByRole('button', { name: 'Create case' })).toBeEnabled()
    expect(onClose).not.toHaveBeenCalled()
  })

  it('summarizes a validation error on a field the form does not show', async () => {
    Object.assign(
      mocks.api,
      mockApi({
        createInvestigation: async () => {
          throw new ApiError('validation', 'must not be archived', 'status')
        },
      }),
    )
    const user = userEvent.setup()
    renderDialog()
    const dialog = await screen.findByRole('dialog', { name: 'New case' })
    await user.type(within(dialog).getByLabelText('Title'), 'x')
    await user.click(within(dialog).getByRole('button', { name: 'Create case' }))
    expect(await within(dialog).findByText('Check this entry: status: must not be archived')).toBeInTheDocument()
  })

  it('warns when the title is close to its byte limit', async () => {
    const user = userEvent.setup()
    renderDialog()
    const dialog = await screen.findByRole('dialog', { name: 'New case' })
    const title = within(dialog).getByLabelText('Title')
    await user.click(title)
    await user.paste('a'.repeat(185))
    expect(within(dialog).getByText('Close to the limit (185 of 200 bytes)')).toBeInTheDocument()
  })

  it('shows the busy toast with Retry when the database is busy', async () => {
    let calls = 0
    Object.assign(
      mocks.api,
      mockApi({
        createInvestigation: async (input) => {
          calls++
          if (calls === 1) throw new ApiError('busy', 'Casebook database is busy; try again')
          return { ...investigation, ref: 'INV-0004', title: input.title }
        },
      }),
    )
    const user = userEvent.setup()
    const { onCreated } = renderDialog()
    const dialog = await screen.findByRole('dialog', { name: 'New case' })
    await user.type(within(dialog).getByLabelText('Title'), 'Retry me')
    await user.click(within(dialog).getByRole('button', { name: 'Create case' }))
    const toast = await screen.findByRole('alert')
    expect(toast).toHaveTextContent('Casebook is busy with another change. Nothing was saved.')
    await user.click(within(toast).getByRole('button', { name: 'Retry' }))
    await waitFor(() => expect(onCreated).toHaveBeenCalledWith('INV-0004'))
  })
})
