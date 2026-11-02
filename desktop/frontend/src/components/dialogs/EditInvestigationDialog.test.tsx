import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Api } from '../../api/api'
import { ApiError } from '../../api/errors'
import type { Investigation } from '../../api/types'
import { AppDataProvider } from '../../state/appData'
import { investigation } from '../../test/fixtures'
import { mockApi } from '../../test/mockApi'
import { ToastProvider } from '../ui/Toaster'
import { CHANGED_ELSEWHERE_COPY } from '../ui/ChangedElsewhereBanner'
import { EditInvestigationDialog } from './EditInvestigationDialog'

const mocks = vi.hoisted(() => ({ api: {} as Api }))
vi.mock('../../api/api', () => ({ api: mocks.api }))

function renderDialog(onClose = vi.fn()) {
  render(
    <ToastProvider>
      <AppDataProvider>
        <EditInvestigationDialog investigation={investigation} onClose={onClose} />
      </AppDataProvider>
    </ToastProvider>,
  )
  return { onClose, dialog: screen.getByRole('dialog', { name: 'Edit title and question' }) }
}

describe('EditInvestigationDialog', () => {
  beforeEach(() => {
    Object.assign(mocks.api, mockApi())
  })

  it('prefills the title and primary question', () => {
    const { dialog } = renderDialog()
    expect(within(dialog).getByLabelText('Title')).toHaveValue('September cloud bill up 38%')
    expect(within(dialog).getByLabelText('Primary question (optional)')).toHaveValue(
      investigation.problem_statement,
    )
    expect(within(dialog).getByLabelText('Title')).toHaveFocus()
  })

  it('sends only the changed fields and closes', async () => {
    const user = userEvent.setup()
    const { dialog, onClose } = renderDialog()
    const title = within(dialog).getByLabelText('Title')
    await user.clear(title)
    await user.type(title, 'September cloud bill jump')
    await user.click(within(dialog).getByRole('button', { name: 'Save changes' }))
    expect(mocks.api.updateInvestigation).toHaveBeenCalledWith('INV-0001', { title: 'September cloud bill jump' })
    await waitFor(() => expect(onClose).toHaveBeenCalled())
  })

  it('closes without a call when nothing changed', async () => {
    const user = userEvent.setup()
    const { dialog, onClose } = renderDialog()
    await user.click(within(dialog).getByRole('button', { name: 'Save changes' }))
    expect(mocks.api.updateInvestigation).not.toHaveBeenCalled()
    expect(onClose).toHaveBeenCalled()
  })

  it('shows Saving... while pending', async () => {
    const user = userEvent.setup()
    let resolve: (v: Investigation) => void = () => {}
    Object.assign(
      mocks.api,
      mockApi({ updateInvestigation: () => new Promise<Investigation>((r) => (resolve = r)) }),
    )
    const { dialog } = renderDialog()
    await user.type(within(dialog).getByLabelText('Primary question (optional)'), ' Really?')
    await user.click(within(dialog).getByRole('button', { name: 'Save changes' }))
    expect(within(dialog).getByRole('button', { name: 'Saving...' })).toBeDisabled()
    expect(mocks.api.updateInvestigation).toHaveBeenCalledWith('INV-0001', {
      problem_statement: `${investigation.problem_statement} Really?`,
    })
    resolve(investigation)
  })

  it('renders a title validation error under Title', async () => {
    const user = userEvent.setup()
    Object.assign(
      mocks.api,
      mockApi({
        updateInvestigation: async () => {
          throw new ApiError('validation', 'is required', 'title')
        },
      }),
    )
    const { dialog } = renderDialog()
    await user.clear(within(dialog).getByLabelText('Title'))
    await user.click(within(dialog).getByRole('button', { name: 'Save changes' }))
    const title = within(dialog).getByLabelText('Title')
    expect(await within(dialog).findByText('is required')).toBeInTheDocument()
    expect(title).toHaveAttribute('aria-invalid', 'true')
    await waitFor(() => expect(title).toHaveFocus())
  })

  it('asks before discarding edits', async () => {
    const user = userEvent.setup()
    const { dialog } = renderDialog()
    await user.type(within(dialog).getByLabelText('Title'), ' x')
    await user.click(within(dialog).getByRole('button', { name: 'Cancel' }))
    expect(screen.getByRole('dialog', { name: 'Discard this draft?' })).toBeInTheDocument()
  })
})

describe('EditInvestigationDialog changed elsewhere (AR-23)', () => {
  beforeEach(() => {
    Object.assign(mocks.api, mockApi())
  })

  it('keeps the input, says the title changed in another app, and Show latest loads it', async () => {
    const user = userEvent.setup()
    const ui = (inv: Investigation) => (
      <ToastProvider>
        <AppDataProvider>
          <EditInvestigationDialog investigation={inv} onClose={vi.fn()} />
        </AppDataProvider>
      </ToastProvider>
    )
    const { rerender } = render(ui(investigation))
    const dialog = screen.getByRole('dialog', { name: 'Edit title and question' })
    await user.type(within(dialog).getByLabelText('Title'), ' (mine)')
    expect(within(dialog).queryByText(CHANGED_ELSEWHERE_COPY)).toBeNull()
    rerender(ui({ ...investigation, title: 'Renamed by Codex' }))
    expect(within(dialog).getByText(CHANGED_ELSEWHERE_COPY)).toBeInTheDocument()
    expect(within(dialog).getByLabelText('Title')).toHaveValue('September cloud bill up 38% (mine)')
    await user.click(within(dialog).getByRole('button', { name: 'Show latest' }))
    expect(within(dialog).getByLabelText('Title')).toHaveValue('Renamed by Codex')
    expect(within(dialog).queryByText(CHANGED_ELSEWHERE_COPY)).toBeNull()
  })
})
