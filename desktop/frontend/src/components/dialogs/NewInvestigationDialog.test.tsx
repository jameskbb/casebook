import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Api } from '../../api/api'
import { ApiError, BUSY_COPY, NOT_SAVED_COPY, REQUIRED_COPY, TOO_LONG_COPY } from '../../api/errors'
import type { Investigation } from '../../api/types'
import { AppDataProvider, useAppData } from '../../state/appData'
import { investigation } from '../../test/fixtures'
import { clientConnections } from '../../test/fixtures.clients'
import { mockApi } from '../../test/mockApi'
import { OpenAtLaunch } from '../../test/OpenAtLaunch'
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
      <AppDataProvider><OpenAtLaunch />
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

  it('leaves Have my AI work on this question unchecked when it already works on another', async () => {
    renderDialog()
    const dialog = await screen.findByRole('dialog', { name: 'New question' })
    expect(await within(dialog).findByRole('checkbox', { name: 'Have my AI work on this question' })).not.toBeChecked()
    expect(within(dialog).getByText("What do you want to know? For example: Why did calls go up when sales stayed the same?")).toBeInTheDocument()
    expect(within(dialog).getByText('Which months, the numbers, and where to look.')).toBeInTheDocument()
  })

  it('ticks Have my AI work on this question when an AI tool is connected and no question has it', async () => {
    Object.assign(mocks.api, mockApi({ listInvestigations: async () => [] }))
    const user = userEvent.setup()
    renderDialog()
    const dialog = await screen.findByRole('dialog', { name: 'New question' })
    expect(await within(dialog).findByRole('checkbox', { name: 'Have my AI work on this question' })).toBeChecked()
    await user.type(within(dialog).getByLabelText('Question'), 'Why are tickets slower?')
    await user.click(within(dialog).getByRole('button', { name: 'Add question' }))
    expect(mocks.api.createInvestigation).toHaveBeenCalledWith(expect.objectContaining({ make_active: true }))
  })

  it('offers no AI choice and never turns the AI on when no AI tool is connected', async () => {
    const off = { ...clientConnections, claude: { ...clientConnections.claude, configured: false } }
    Object.assign(mocks.api, mockApi({ listInvestigations: async () => [], clientConnections: async () => off }))
    const user = userEvent.setup()
    renderDialog()
    const dialog = await screen.findByRole('dialog', { name: 'New question' })
    await waitFor(() => expect(mocks.api.clientConnections).toHaveBeenCalled())
    expect(within(dialog).queryByRole('checkbox')).toBeNull()
    await user.type(within(dialog).getByLabelText('Question'), 'Why are tickets slower?')
    await user.click(within(dialog).getByRole('button', { name: 'Add question' }))
    expect(mocks.api.createInvestigation).toHaveBeenCalledWith(expect.objectContaining({ make_active: false }))
  })

  it('shows Adding... and disables the submit while the call runs', async () => {
    let resolve: (v: Investigation) => void = () => {}
    Object.assign(
      mocks.api,
      mockApi({ createInvestigation: () => new Promise<Investigation>((r) => (resolve = r)) }),
    )
    const user = userEvent.setup()
    const { onClose, onCreated } = renderDialog()
    const dialog = await screen.findByRole('dialog', { name: 'New question' })
    await user.type(within(dialog).getByLabelText('Question'), 'Vendor contract renewals')
    await user.click(within(dialog).getByRole('button', { name: 'Add question' }))

    const busy = within(dialog).getByRole('button', { name: 'Adding...' })
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

  it('says Write something here first under the question when it is missing', async () => {
    Object.assign(
      mocks.api,
      mockApi({
        createInvestigation: async () => {
          throw new ApiError('validation', 'is required', 'title')
        },
      }),
    )
    const user = userEvent.setup()
    const { onClose } = renderDialog()
    const dialog = await screen.findByRole('dialog', { name: 'New question' })
    await user.click(within(dialog).getByRole('button', { name: 'Add question' }))

    const title = within(dialog).getByLabelText('Question')
    await waitFor(() => expect(title).toHaveAttribute('aria-invalid', 'true'))
    expect(title).toHaveAccessibleDescription(expect.stringContaining(REQUIRED_COPY))
    expect(dialog).not.toHaveTextContent('is required')
    expect(title).toHaveFocus()
    expect(onClose).not.toHaveBeenCalled()
  })

  it('shows a plain too-long sentence under the question, never the engine text', async () => {
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
    const dialog = await screen.findByRole('dialog', { name: 'New question' })
    await user.type(within(dialog).getByLabelText('Question'), 'x')
    await user.click(within(dialog).getByRole('button', { name: 'Add question' }))

    const title = within(dialog).getByLabelText('Question')
    await waitFor(() => expect(title).toHaveAttribute('aria-invalid', 'true'))
    expect(title).toHaveAccessibleDescription(expect.stringContaining(TOO_LONG_COPY))
    expect(dialog).not.toHaveTextContent('200 bytes')
    expect(title).toHaveFocus()
    expect(within(dialog).getByRole('button', { name: 'Add question' })).toBeEnabled()
    expect(onClose).not.toHaveBeenCalled()
  })

  it('puts a validation error on a field the form does not show into the summary, in plain words', async () => {
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
    const dialog = await screen.findByRole('dialog', { name: 'New question' })
    await user.type(within(dialog).getByLabelText('Question'), 'x')
    await user.click(within(dialog).getByRole('button', { name: 'Add question' }))
    expect(await within(dialog).findByRole('alert')).toHaveTextContent(NOT_SAVED_COPY)
    expect(dialog).not.toHaveTextContent(/status|must not be archived/)
  })

  it('warns when the title is close to its byte limit', async () => {
    const user = userEvent.setup()
    renderDialog()
    const dialog = await screen.findByRole('dialog', { name: 'New question' })
    const title = within(dialog).getByLabelText('Question')
    await user.click(title)
    await user.paste('a'.repeat(185))
    expect(within(dialog).getByText('Almost too long (185 of 200 characters)')).toBeInTheDocument()
  })

  it('shows the busy toast with Try again when Casebook is busy', async () => {
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
    const dialog = await screen.findByRole('dialog', { name: 'New question' })
    await user.type(within(dialog).getByLabelText('Question'), 'Retry me')
    await user.click(within(dialog).getByRole('button', { name: 'Add question' }))
    const toast = await screen.findByRole('alert')
    expect(toast).toHaveTextContent(BUSY_COPY)
    await user.click(within(toast).getByRole('button', { name: 'Try again' }))
    await waitFor(() => expect(onCreated).toHaveBeenCalledWith('INV-0004'))
  })
})
