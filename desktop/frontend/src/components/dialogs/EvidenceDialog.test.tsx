import type { ReactNode } from 'react'
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Api } from '../../api/api'
import { ApiError, NOT_SAVED_COPY, REQUIRED_COPY } from '../../api/errors'
import type { Evidence } from '../../api/types'
import { fromLocalInputValue, toLocalInputValue } from '../../lib/captured'
import { CAPTURED_INVALID_COPY } from './EvidenceDialog'
import { AppDataProvider } from '../../state/appData'
import { evidence } from '../../test/fixtures'
import { E003_RAW, evidenceWith } from '../../test/fixtures.records'
import { mockApi } from '../../test/mockApi'
import { OpenAtLaunch } from '../../test/OpenAtLaunch'
import { ToastProvider } from '../ui/Toaster'
import { CHANGED_ELSEWHERE_COPY, SHOW_NEW_VERSION } from '../ui/ChangedElsewhereBanner'
import { EvidenceDialog } from './EvidenceDialog'

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
const KIND = 'Kind of source'
const DATE = 'Date of the source (optional)'

/** The six kinds a person can choose, in the order shown. */
const SIX_KINDS = [
  'Report or spreadsheet',
  'Document or email',
  'Web page',
  'Something a person said',
  'Something you saw',
  'Something your AI found',
]

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

const more = () => screen.getByRole('button', { name: 'More' })
const kindOptions = () => Array.from((screen.getByLabelText(KIND) as HTMLSelectElement).options).map((o) => o.text)

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

  it('opens with Name focused, four fields and a closed More', () => {
    renderAdd()
    expect(screen.getByRole('dialog', { name: 'Add source' })).toBeInTheDocument()
    expect(screen.getByLabelText(NAME)).toHaveFocus()
    expect(screen.getByLabelText(SHOWS)).toBeInTheDocument()
    expect(screen.getByLabelText(WHERE)).toBeInTheDocument()
    expect(screen.getByLabelText(WORDS)).toBeInTheDocument()
    expect(more()).toHaveAttribute('aria-expanded', 'false')
    expect(screen.queryByLabelText(KIND)).toBeNull()
    expect(screen.queryByLabelText(DATE)).toBeNull()
    expect(screen.queryByLabelText(/Where it came from/)).toBeNull()
    expect(screen.queryByLabelText(/Extra details/)).toBeNull()
    expect(screen.queryByRole('button', { name: 'What to write down' })).toBeNull()
    expect(screen.queryByRole('button', { name: 'Add source detail' })).toBeNull()
  })

  it('opening More shows the kind of source with exactly six choices and the date', async () => {
    const user = userEvent.setup()
    renderAdd()
    await user.click(more())
    expect(more()).toHaveAttribute('aria-expanded', 'true')
    expect(screen.getByLabelText(KIND)).toHaveValue('manual_observation')
    expect(kindOptions()).toEqual(SIX_KINDS)
    expect(screen.getByLabelText(DATE)).toHaveValue('')
  })

  it('adds a source with every field, converting the date, and sends no source_label or metadata', async () => {
    const user = userEvent.setup()
    const { onClose, onSaved } = renderAdd()
    await user.type(screen.getByLabelText(NAME), 'October sales report')
    await user.type(screen.getByLabelText(SHOWS), 'Sales were $512k in October.')
    await user.type(screen.getByLabelText(WHERE), 'Shared drive, Sales, october.pdf, page 2')
    await user.type(screen.getByLabelText(WORDS), 'October: $512k')
    await user.click(more())
    await user.selectOptions(screen.getByLabelText(KIND), 'documentation')
    fireEvent.change(screen.getByLabelText(DATE), { target: { value: '2026-09-28T13:05' } })
    await user.click(screen.getByRole('button', { name: 'Add source' }))
    expect(mocks.api.addEvidence).toHaveBeenCalledWith('INV-0001', {
      title: 'October sales report',
      summary: 'Sales were $512k in October.',
      source_type: 'documentation',
      source_ref: 'Shared drive, Sales, october.pdf, page 2',
      captured_at: fromLocalInputValue('2026-09-28T13:05'),
      raw_detail: 'October: $512k',
    })
    const sent = vi.mocked(mocks.api.addEvidence).mock.calls[0][1]
    expect(sent).not.toHaveProperty('source_label')
    expect(sent).not.toHaveProperty('metadata')
    await waitFor(() => expect(onClose).toHaveBeenCalled())
    expect(onSaved).toHaveBeenCalledWith(evidence[0])
  })

  it('does not add a source with an invalid date: More opens and shows the date error', async () => {
    const user = userEvent.setup()
    renderAdd()
    await user.type(screen.getByLabelText(NAME), 'A name')
    await user.type(screen.getByLabelText(SHOWS), 'What it shows.')
    await user.click(more())
    fireEvent.change(screen.getByLabelText(DATE), { target: { value: '20255-09-28T13:05' } })
    await user.click(more())
    expect(screen.queryByLabelText(DATE)).toBeNull()
    await user.click(screen.getByRole('button', { name: 'Add source' }))
    expect(mocks.api.addEvidence).not.toHaveBeenCalled()
    expect(more()).toHaveAttribute('aria-expanded', 'true')
    expect(screen.getByLabelText(DATE)).toHaveAttribute('aria-invalid', 'true')
    expect(screen.getByLabelText(DATE)).toHaveAccessibleDescription(expect.stringContaining(CAPTURED_INVALID_COPY))
  })

  it('omits empty optional fields, so the date defaults to today', async () => {
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

  it('changes the hints with the kind of source without replacing what was typed', async () => {
    const user = userEvent.setup()
    renderAdd()
    await user.type(screen.getByLabelText(WHERE), 'My own words')
    await user.type(screen.getByLabelText(WORDS), 'My own numbers')
    expect(screen.getByLabelText(WHERE)).toHaveAccessibleDescription(/Where and when you saw it/)
    await user.click(more())
    await user.selectOptions(screen.getByLabelText(KIND), 'file')
    expect(screen.getByLabelText(WHERE)).toHaveAccessibleDescription(/the sheet and the cells/)
    expect(screen.getByLabelText(WORDS)).toHaveAccessibleDescription(/with their headings/)
    await user.selectOptions(screen.getByLabelText(KIND), 'documentation')
    expect(screen.getByLabelText(WHERE)).toHaveAccessibleDescription(/page or section/)
    expect(screen.getByLabelText(WHERE)).toHaveValue('My own words')
    expect(screen.getByLabelText(WORDS)).toHaveValue('My own numbers')
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
    await user.click(more())
    expect(screen.getByLabelText(KIND)).toHaveValue('sql_query')
    expect(screen.getByLabelText(DATE)).toHaveValue(toLocalInputValue(evidenceWith('E-003').captured_at))
    expect(screen.queryByLabelText(/Where it came from/)).toBeNull()
    expect(screen.queryByLabelText(/Extra details/)).toBeNull()

    const title = screen.getByLabelText(NAME)
    await user.clear(title)
    await user.type(title, 'Calls about a bill: 400')
    await user.click(screen.getByRole('button', { name: 'Save changes' }))
    expect(mocks.api.updateEvidence).toHaveBeenCalledWith('INV-0001', 'E-003', { title: 'Calls about a bill: 400' })
    await waitFor(() => expect(onClose).toHaveBeenCalled())
  })

  it('keeps the kind of an existing source that is not one of the six as an extra option', async () => {
    const user = userEvent.setup()
    renderEdit(evidenceWith('E-003'))
    await user.click(more())
    expect(screen.getByLabelText(KIND)).toHaveValue('sql_query')
    expect(kindOptions()).toEqual([...SIX_KINDS, 'Numbers your AI looked up'])
  })

  it('never sends source_label or metadata, even when the source has them', async () => {
    const user = userEvent.setup()
    renderEdit({ ...evidenceWith('E-002'), metadata: '{"custom":{"keep":[1,"existing"]},"period":"2026-Q3"}' })
    await user.click(more())
    await user.selectOptions(screen.getByLabelText(KIND), 'file')
    await user.click(screen.getByRole('button', { name: 'Save changes' }))
    expect(mocks.api.updateEvidence).toHaveBeenCalledWith('INV-0001', 'E-002', { source_type: 'file' })
    const patch = vi.mocked(mocks.api.updateEvidence).mock.calls[0][2]
    expect(patch).not.toHaveProperty('source_label')
    expect(patch).not.toHaveProperty('metadata')
  })

  it('clearing the date sends an empty value, which sets it back to the day it was added', async () => {
    const user = userEvent.setup()
    renderEdit()
    await waitFor(() => expect(screen.getByLabelText(WORDS)).toHaveValue(E003_RAW))
    await user.click(more())
    fireEvent.change(screen.getByLabelText(DATE), { target: { value: '' } })
    await user.click(screen.getByRole('button', { name: 'Save changes' }))
    expect(mocks.api.updateEvidence).toHaveBeenCalledWith('INV-0001', 'E-003', { captured_at: '' })
  })

  it('refuses an invalid date instead of resetting it', async () => {
    const user = userEvent.setup()
    const { onClose } = renderEdit()
    await waitFor(() => expect(screen.getByLabelText(WORDS)).toHaveValue(E003_RAW))
    await user.click(more())
    const field = screen.getByLabelText(DATE)
    fireEvent.change(field, { target: { value: '20255-09-28T13:05' } })
    await user.click(screen.getByRole('button', { name: 'Save changes' }))
    expect(mocks.api.updateEvidence).not.toHaveBeenCalled()
    expect(onClose).not.toHaveBeenCalled()
    expect(screen.getByText(CAPTURED_INVALID_COPY)).toBeInTheDocument()
    expect(field).toHaveAttribute('aria-invalid', 'true')
    await waitFor(() => expect(field).toHaveFocus())
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
