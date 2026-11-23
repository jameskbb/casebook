import type { ReactNode } from 'react'
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Api } from '../../api/api'
import { ApiError, NOT_SAVED_COPY, REQUIRED_COPY } from '../../api/errors'
import type { Evidence } from '../../api/types'
import { AppDataProvider } from '../../state/appData'
import { evidence } from '../../test/fixtures'
import { E003_RAW, evidenceWith } from '../../test/fixtures.records'
import { mockApi } from '../../test/mockApi'
import { OpenAtLaunch } from '../../test/OpenAtLaunch'
import { ToastProvider } from '../ui/Toaster'
import { CHANGED_ELSEWHERE_COPY, SHOW_NEW_VERSION } from '../ui/ChangedElsewhereBanner'
import { ADDED_COUNTS_AS_CHECKED, EvidenceDialog } from './EvidenceDialog'

const mocks = vi.hoisted(() => ({ api: {} as Api }))
vi.mock('../../api/api', () => ({ api: mocks.api }))

const providers = (ui: ReactNode) => (
  <ToastProvider>
    <AppDataProvider><OpenAtLaunch />{ui}</AppDataProvider>
  </ToastProvider>
)

const NAME = 'Name'
const SHOWS = 'What it shows'
const WHERE = 'Where to find it'
const WORDS = 'Exact words or numbers (optional)'

function renderAdd() {
  const onClose = vi.fn()
  const onSaved = vi.fn()
  render(providers(<EvidenceDialog mode="add" invRef="INV-0001" onClose={onClose} onSaved={onSaved} />))
  return { onClose, onSaved }
}

function renderEdit(e: Evidence = evidenceWith('E-003')) {
  const onClose = vi.fn()
  render(providers(<EvidenceDialog mode="edit" evidence={e} invRef="INV-0001" onClose={onClose} />))
  return { onClose }
}


describe('EvidenceDialog add', () => {
  beforeEach(() => {
    Object.assign(
      mocks.api,
      mockApi({
        getEvidence: async (_inv, ref, o) => ({
          ...evidenceWith(ref),
          ...(o?.include_raw_detail ? { raw_detail: E003_RAW } : {}),
        }),
      }),
    )
  })

  it('opens with Name focused and four fields, and says a source added by hand counts as checked', () => {
    renderAdd()
    expect(screen.getByRole('dialog', { name: 'Add source' })).toBeInTheDocument()
    expect(screen.getByLabelText(NAME)).toHaveFocus()
    expect(screen.getByLabelText(SHOWS)).toBeInTheDocument()
    expect(screen.getByLabelText(WHERE)).toBeInTheDocument()
    expect(screen.getByLabelText(WORDS)).toBeInTheDocument()
    expect(screen.getByText(ADDED_COUNTS_AS_CHECKED)).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'More' })).toBeNull()
    expect(screen.queryByLabelText(/Kind of source/)).toBeNull()
    expect(screen.queryByLabelText(/Date of the source/)).toBeNull()
    expect(screen.queryByLabelText(/^From/)).toBeNull()
    expect(screen.queryByRole('combobox')).toBeNull()
    expect(screen.queryByLabelText(/Where it came from/)).toBeNull()
    expect(screen.queryByLabelText(/Extra details/)).toBeNull()
    expect(screen.queryByRole('button', { name: 'What to write down' })).toBeNull()
    expect(screen.queryByRole('button', { name: 'Add source detail' })).toBeNull()
  })

  it('adds a source with every field as one the person saw, and sends no source_label, date or metadata', async () => {
    const user = userEvent.setup()
    const { onClose, onSaved } = renderAdd()
    await user.type(screen.getByLabelText(NAME), 'October sales report')
    await user.type(screen.getByLabelText(SHOWS), 'Sales were $512,000 in October.')
    await user.type(screen.getByLabelText(WHERE), 'Shared drive, Sales, october.pdf, page 2')
    await user.type(screen.getByLabelText(WORDS), 'October: $512,000')
    await user.click(screen.getByRole('button', { name: 'Add source' }))
    expect(mocks.api.addEvidence).toHaveBeenCalledWith('INV-0001', {
      title: 'October sales report',
      summary: 'Sales were $512,000 in October.',
      source_type: 'manual_observation',
      source_ref: 'Shared drive, Sales, october.pdf, page 2',
      raw_detail: 'October: $512,000',
    })
    const sent = vi.mocked(mocks.api.addEvidence).mock.calls[0][1]
    expect(sent).not.toHaveProperty('source_label')
    expect(sent).not.toHaveProperty('metadata')
    expect(sent).not.toHaveProperty('captured_at')
    await waitFor(() => expect(onClose).toHaveBeenCalled())
    expect(onSaved).toHaveBeenCalledWith(evidence[0])
  })

  it('omits empty optional fields', async () => {
    const user = userEvent.setup()
    renderAdd()
    await user.type(screen.getByLabelText(NAME), 'Screenshot')
    await user.type(screen.getByLabelText(SHOWS), 'The wall board shows 3,120 calls.')
    await user.click(screen.getByRole('button', { name: 'Add source' }))
    expect(mocks.api.addEvidence).toHaveBeenCalledWith('INV-0001', {
      title: 'Screenshot',
      summary: 'The wall board shows 3,120 calls.',
      source_type: 'manual_observation',
    })
  })

  it('shows Adding... and disables submit while the call runs', async () => {
    const user = userEvent.setup()
    Object.assign(mocks.api, mockApi({ addEvidence: () => new Promise<Evidence>(() => {}) }))
    renderAdd()
    await user.type(screen.getByLabelText(NAME), 'T')
    await user.type(screen.getByLabelText(SHOWS), 'S')
    await user.click(screen.getByRole('button', { name: 'Add source' }))
    expect(screen.getByRole('button', { name: 'Adding...' })).toBeDisabled()
  })

  it('shows a title error under Name in plain words, never the engine text', async () => {
    const user = userEvent.setup()
    Object.assign(
      mocks.api,
      mockApi({
        addEvidence: async () => {
          throw new ApiError('validation', 'is required', 'title')
        },
      }),
    )
    renderAdd()
    await user.type(screen.getByLabelText(SHOWS), 'S')
    await user.click(screen.getByRole('button', { name: 'Add source' }))
    expect(await screen.findByText('Write something here first.')).toBeInTheDocument()
    expect(screen.getByLabelText(NAME)).toHaveAttribute('aria-invalid', 'true')
    expect(screen.getByLabelText(NAME)).toHaveAccessibleDescription(expect.stringContaining(REQUIRED_COPY))
    expect(screen.getByLabelText(NAME)).toHaveFocus()
    expect(screen.queryByText(/is required/)).toBeNull()
    expect(screen.queryByText(/title/)).toBeNull()
  })

  it('puts an error on a field the form does not show at the top, in plain words', async () => {
    const user = userEvent.setup()
    Object.assign(
      mocks.api,
      mockApi({
        addEvidence: async () => {
          throw new ApiError('validation', 'must be a JSON object', 'metadata')
        },
      }),
    )
    renderAdd()
    await user.type(screen.getByLabelText(NAME), 'T')
    await user.type(screen.getByLabelText(SHOWS), 'S')
    await user.click(screen.getByRole('button', { name: 'Add source' }))
    expect(await screen.findByRole('alert')).toHaveTextContent(NOT_SAVED_COPY)
    expect(screen.queryByText(/JSON|metadata/)).toBeNull()
  })

  it('shows the near-limit counter for the exact words', () => {
    renderAdd()
    fireEvent.change(screen.getByLabelText(WORDS), { target: { value: 'x'.repeat(59000) } })
    expect(screen.getByText('Almost too long (59,000 of 65,536 characters)')).toBeInTheDocument()
  })

  it('asks before discarding a draft', async () => {
    const user = userEvent.setup()
    const { onClose } = renderAdd()
    await user.type(screen.getByLabelText(NAME), 'Half written')
    await user.click(screen.getByRole('button', { name: 'Cancel' }))
    expect(screen.getByRole('dialog', { name: 'Discard this draft?' })).toBeInTheDocument()
    expect(onClose).not.toHaveBeenCalled()
  })
})

describe('EvidenceDialog edit', () => {
  beforeEach(() => {
    Object.assign(
      mocks.api,
      mockApi({
        getEvidence: async (_inv, ref, o) => ({
          ...evidenceWith(ref),
          ...(o?.include_raw_detail ? { raw_detail: E003_RAW } : {}),
        }),
      }),
    )
  })

  it('loads the source with its exact words and sends only the changed fields', async () => {
    const user = userEvent.setup()
    const { onClose } = renderEdit()
    expect(screen.getByRole('dialog', { name: 'Edit source' })).toBeInTheDocument()
    expect(mocks.api.getEvidence).toHaveBeenCalledWith('INV-0001', 'E-003', { include_raw_detail: true })
    expect(screen.getByLabelText(NAME)).toHaveValue('Calls about a bill: 400 in September, 1,070 in October')
    await waitFor(() => expect(screen.getByLabelText(WORDS)).toHaveValue(E003_RAW))
    expect(screen.queryByText(ADDED_COUNTS_AS_CHECKED)).toBeNull()
    expect(screen.queryByLabelText(/Kind of source/)).toBeNull()
    expect(screen.queryByLabelText(/Where it came from/)).toBeNull()
    expect(screen.queryByLabelText(/Extra details/)).toBeNull()

    const title = screen.getByLabelText(NAME)
    await user.clear(title)
    await user.type(title, 'Calls about a bill: 400')
    await user.click(screen.getByRole('button', { name: 'Save changes' }))
    expect(mocks.api.updateEvidence).toHaveBeenCalledWith('INV-0001', 'E-003', { title: 'Calls about a bill: 400' })
    await waitFor(() => expect(onClose).toHaveBeenCalled())
  })

  it('leaves the stored kind, date, source_label and metadata as they are', async () => {
    const user = userEvent.setup()
    renderEdit({ ...evidenceWith('E-002'), metadata: '{"custom":{"keep":[1,"existing"]},"period":"2026-Q3"}' })
    await user.type(screen.getByLabelText(SHOWS), ' More.')
    await user.click(screen.getByRole('button', { name: 'Save changes' }))
    expect(Object.keys(vi.mocked(mocks.api.updateEvidence).mock.calls[0][2])).toEqual(['summary'])
  })

  it('closes without a call when nothing changed', async () => {
    const user = userEvent.setup()
    const { onClose } = renderEdit()
    await waitFor(() => expect(screen.getByLabelText(WORDS)).toHaveValue(E003_RAW))
    await user.click(screen.getByRole('button', { name: 'Save changes' }))
    expect(mocks.api.updateEvidence).not.toHaveBeenCalled()
    expect(onClose).toHaveBeenCalled()
  })

  it('does not fetch the exact words for a source without any', () => {
    renderEdit(evidenceWith('E-002'))
    expect(mocks.api.getEvidence).not.toHaveBeenCalled()
    expect(screen.getByLabelText(WORDS)).toBeEnabled()
    expect(screen.getByLabelText(WORDS)).toHaveValue('')
  })
})

describe('EvidenceDialog edit changed elsewhere', () => {
  beforeEach(() => {
    Object.assign(mocks.api, mockApi())
  })

  it('flags a source changed in another app while the form holds input', async () => {
    const user = userEvent.setup()
    const e = evidenceWith('E-002')
    const ui = (ev: Evidence) => providers(<EvidenceDialog mode="edit" evidence={ev} invRef="INV-0001" onClose={vi.fn()} />)
    const { rerender } = render(ui(e))
    const dialog = screen.getByRole('dialog', { name: 'Edit source' })
    await user.type(within(dialog).getByLabelText(NAME), ' (mine)')
    rerender(ui({ ...e, summary: 'Rewritten by Claude Code.' }))
    expect(within(dialog).getByText(CHANGED_ELSEWHERE_COPY)).toBeInTheDocument()
    await user.click(within(dialog).getByRole('button', { name: SHOW_NEW_VERSION }))
    expect(within(dialog).getByLabelText(SHOWS)).toHaveValue('Rewritten by Claude Code.')
    expect(within(dialog).getByLabelText(NAME)).toHaveValue(e.title)
  })
})
