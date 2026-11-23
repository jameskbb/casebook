import type { ReactNode } from 'react'
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Api } from '../../api/api'
import { ApiError } from '../../api/errors'
import type { Evidence } from '../../api/types'
import { fromLocalInputValue, toLocalInputValue } from '../../lib/captured'
import { CAPTURED_INVALID_COPY } from './EvidenceDialog'
import { AppDataProvider } from '../../state/appData'
import { evidence } from '../../test/fixtures'
import { E003_RAW, evidenceWith } from '../../test/fixtures.records'
import { mockApi } from '../../test/mockApi'
import { OpenAtLaunch } from '../../test/OpenAtLaunch'
import { ToastProvider } from '../ui/Toaster'
import { CHANGED_ELSEWHERE_COPY } from '../ui/ChangedElsewhereBanner'
import { EvidenceDialog } from './EvidenceDialog'

const mocks = vi.hoisted(() => ({ api: {} as Api }))
vi.mock('../../api/api', () => ({ api: mocks.api }))

const providers = (ui: ReactNode) => (
  <ToastProvider>
    <AppDataProvider><OpenAtLaunch />{ui}</AppDataProvider>
  </ToastProvider>
)

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

  it('opens with the title field focused and Manual observation as the source type', () => {
    renderAdd()
    expect(screen.getByRole('dialog', { name: 'Add source' })).toBeInTheDocument()
    expect(screen.getByLabelText('Title')).toHaveFocus()
    expect(screen.getByLabelText('Kind of source')).toHaveValue('manual_observation')
    const options = Array.from((screen.getByLabelText('Kind of source') as HTMLSelectElement).options).map((o) => o.text)
    expect(options).toEqual([
      'Data query',
      'Code',
      'File',
      'Jira',
      'Document',
      'Data from another system',
      'Something a person said',
      'Result from your AI',
      'Something you saw',
      'Web page',
      'Another Casebook question',
    ])
    expect(screen.queryByLabelText('Source detail (optional)')).toBeNull()
    expect(screen.queryByLabelText('Extra details (optional)')).toBeNull()
  })

  it('adds evidence with every field, converting the captured time', async () => {
    const user = userEvent.setup()
    const { onClose, onSaved } = renderAdd()
    await user.type(screen.getByLabelText('Title'), 'Log volume by service query')
    await user.type(screen.getByLabelText('Summary'), 'Debug log volume for each service in September.')
    await user.selectOptions(screen.getByLabelText('Kind of source'), 'sql_query')
    await user.type(screen.getByLabelText('Where it came from (optional)'), 'Billing export')
    await user.type(screen.getByLabelText('Where to find it (optional)'), 'finops/queries/log_volume_by_service.sql')
    fireEvent.change(screen.getByLabelText('As of (optional)'), { target: { value: '2026-09-28T13:05' } })
    await user.click(screen.getByRole('button', { name: 'Add source detail' }))
    await user.type(screen.getByLabelText('Source detail (optional)'), 'SELECT 1;')
    await user.click(screen.getByRole('button', { name: 'For your AI' }))
    await user.click(screen.getByLabelText('Extra details (optional)'))
    await user.paste('{"affected_records": 22}')
    await user.click(screen.getByRole('button', { name: 'Add source' }))
    expect(mocks.api.addEvidence).toHaveBeenCalledWith('INV-0001', {
      title: 'Log volume by service query',
      summary: 'Debug log volume for each service in September.',
      source_type: 'sql_query',
      source_label: 'Billing export',
      source_ref: 'finops/queries/log_volume_by_service.sql',
      captured_at: fromLocalInputValue('2026-09-28T13:05'),
      raw_detail: 'SELECT 1;',
      metadata: '{"affected_records": 22}',
    })
    await waitFor(() => expect(onClose).toHaveBeenCalled())
    expect(onSaved).toHaveBeenCalledWith(evidence[0])
  })

  it('does not add evidence with an unparseable captured value', async () => {
    const user = userEvent.setup()
    renderAdd()
    await user.type(screen.getByLabelText('Title'), 'A title')
    await user.type(screen.getByLabelText('Summary'), 'A summary.')
    fireEvent.change(screen.getByLabelText('As of (optional)'), { target: { value: '20255-09-28T13:05' } })
    await user.click(screen.getByRole('button', { name: 'Add source' }))
    expect(mocks.api.addEvidence).not.toHaveBeenCalled()
    expect(screen.getByText(CAPTURED_INVALID_COPY)).toBeInTheDocument()
  })

  it('omits empty optional fields, so the captured time defaults to now', async () => {
    const user = userEvent.setup()
    renderAdd()
    await user.type(screen.getByLabelText('Title'), 'Screenshot')
    await user.type(screen.getByLabelText('Summary'), 'The wall board shows 3,120 calls.')
    await user.click(screen.getByRole('button', { name: 'Add source' }))
    expect(mocks.api.addEvidence).toHaveBeenCalledWith('INV-0001', {
      title: 'Screenshot',
      summary: 'The wall board shows 3,120 calls.',
      source_type: 'manual_observation',
    })
  })

  it('shows source-sensitive capture examples without inserting fictional evidence', async () => {
    const user = userEvent.setup()
    renderAdd()
    await user.click(screen.getByRole('button', { name: 'What to write down' }))
    expect(screen.getByText(/enough for someone else to find this source and check it later/)).toBeInTheDocument()
    await user.selectOptions(screen.getByLabelText('Kind of source'), 'file')
    expect(screen.getByLabelText('Where to find it (optional)')).toHaveAccessibleDescription(/the sheet and the cells/)
    expect(screen.getByText('Call log October.xlsx, sheet By reason, cells B4 to C6, saved November 2')).toBeInTheDocument()
    expect(screen.getByText('A made-up example. Nothing is added to your sources.')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Add source detail' }))
    expect(screen.getByLabelText('Source detail (optional)')).toHaveAccessibleDescription(/with their headings/)
    expect(screen.getByLabelText('Source detail (optional)')).toHaveValue('')
    expect(screen.getByLabelText('Where to find it (optional)')).toHaveValue('')
    await user.type(screen.getByLabelText('Title'), 'Workbook review')
    await user.type(screen.getByLabelText('Summary'), 'The workbook is available for review.')
    await user.click(screen.getByRole('button', { name: 'Add source' }))
    expect(mocks.api.addEvidence).toHaveBeenCalledWith('INV-0001', {
      title: 'Workbook review',
      summary: 'The workbook is available for review.',
      source_type: 'file',
    })
  })

  it('updates locator guidance across SQL and document sources without replacing text', async () => {
    const user = userEvent.setup()
    renderAdd()
    await user.click(screen.getByRole('button', { name: 'Add source detail' }))
    await user.type(screen.getByLabelText('Where to find it (optional)'), 'My existing reference')
    await user.type(screen.getByLabelText('Source detail (optional)'), 'My concise source excerpt')
    await user.selectOptions(screen.getByLabelText('Kind of source'), 'sql_query')
    expect(screen.getByLabelText('Where to find it (optional)')).toHaveAccessibleDescription(/the data it reads/)
    expect(screen.getByLabelText('Source detail (optional)')).toHaveAccessibleDescription(/The query and the rows that matter/)
    await user.selectOptions(screen.getByLabelText('Kind of source'), 'documentation')
    expect(screen.getByLabelText('Where to find it (optional)')).toHaveAccessibleDescription(/page or section/)
    expect(screen.getByLabelText('Where to find it (optional)')).toHaveValue('My existing reference')
    expect(screen.getByLabelText('Source detail (optional)')).toHaveValue('My concise source excerpt')
  })

  it('shows Adding... and disables submit while the call runs', async () => {
    const user = userEvent.setup()
    Object.assign(mocks.api, mockApi({ addEvidence: () => new Promise<Evidence>(() => {}) }))
    renderAdd()
    await user.type(screen.getByLabelText('Title'), 'T')
    await user.type(screen.getByLabelText('Summary'), 'S')
    await user.click(screen.getByRole('button', { name: 'Add source' }))
    expect(screen.getByRole('button', { name: 'Adding...' })).toBeDisabled()
  })

  it('shows a summary error under Summary', async () => {
    const user = userEvent.setup()
    Object.assign(
      mocks.api,
      mockApi({
        addEvidence: async () => {
          throw new ApiError('validation', 'is required', 'summary')
        },
      }),
    )
    renderAdd()
    await user.type(screen.getByLabelText('Title'), 'T')
    await user.click(screen.getByRole('button', { name: 'Add source' }))
    expect(await screen.findByText('is required')).toBeInTheDocument()
    expect(screen.getByLabelText('Summary')).toHaveAttribute('aria-invalid', 'true')
    expect(screen.getByLabelText('Summary')).toHaveFocus()
  })

  it('opens Advanced and shows a metadata error under Metadata', async () => {
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
    await user.type(screen.getByLabelText('Title'), 'T')
    await user.type(screen.getByLabelText('Summary'), 'S')
    await user.click(screen.getByRole('button', { name: 'Add source' }))
    expect(await screen.findByText('must be a JSON object')).toBeInTheDocument()
    expect(screen.getByLabelText('Extra details (optional)')).toHaveAttribute('aria-invalid', 'true')
    expect(screen.getByRole('button', { name: 'For your AI' })).toHaveAttribute('aria-expanded', 'true')
  })

  it('puts an error on a field the form does not show into the summary', async () => {
    const user = userEvent.setup()
    Object.assign(
      mocks.api,
      mockApi({
        addEvidence: async () => {
          throw new ApiError('validation', 'is not a verification', 'verification')
        },
      }),
    )
    renderAdd()
    await user.type(screen.getByLabelText('Title'), 'T')
    await user.type(screen.getByLabelText('Summary'), 'S')
    await user.click(screen.getByRole('button', { name: 'Add source' }))
    expect(await screen.findByText('Check this entry: verification: is not a verification')).toBeInTheDocument()
  })

  it('shows the near-limit counter for raw detail', async () => {
    const user = userEvent.setup()
    renderAdd()
    await user.click(screen.getByRole('button', { name: 'Add source detail' }))
    fireEvent.change(screen.getByLabelText('Source detail (optional)'), { target: { value: 'x'.repeat(59000) } })
    expect(screen.getByText('Almost too long (59,000 of 65,536 characters)')).toBeInTheDocument()
  })

  it('asks before discarding a draft', async () => {
    const user = userEvent.setup()
    const { onClose } = renderAdd()
    await user.type(screen.getByLabelText('Title'), 'Half written')
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

  it('loads the record with its raw detail and sends only the changed fields', async () => {
    const user = userEvent.setup()
    const { onClose } = renderEdit()
    expect(screen.getByRole('dialog', { name: 'Edit source' })).toBeInTheDocument()
    expect(mocks.api.getEvidence).toHaveBeenCalledWith('INV-0001', 'E-003', { include_raw_detail: true })
    expect(screen.getByLabelText('Title')).toHaveValue('Calls about a bill: 400 in September, 1,070 in October')
    expect(screen.getByLabelText('Kind of source')).toHaveValue('sql_query')
    expect(screen.getByLabelText('As of (optional)')).toHaveValue(toLocalInputValue(evidenceWith('E-003').captured_at))
    await waitFor(() => expect(screen.getByLabelText('Source detail (optional)')).toHaveValue(E003_RAW))
    expect(screen.getByLabelText('Extra details (optional)')).toHaveValue('{\n  "calls_counted": 5520\n}')

    const title = screen.getByLabelText('Title')
    await user.clear(title)
    await user.type(title, 'Calls about a bill: 400')
    await user.click(screen.getByRole('button', { name: 'Save changes' }))
    expect(mocks.api.updateEvidence).toHaveBeenCalledWith('INV-0001', 'E-003', { title: 'Calls about a bill: 400' })
    await waitFor(() => expect(onClose).toHaveBeenCalled())
  })

  it('keeps a source type kept only for existing records when editing one', async () => {
    renderEdit({ ...evidenceWith('E-002'), source_type: 'evernote_note' })
    const select = screen.getByLabelText('Kind of source') as HTMLSelectElement
    expect(select).toHaveValue('evernote_note')
    expect(Array.from(select.options).map((o) => o.text)).toContain('Evernote note')
  })

  it('clearing Captured resets it to the creation time with an empty value', async () => {
    const user = userEvent.setup()
    renderEdit()
    await waitFor(() => expect(screen.getByLabelText('Source detail (optional)')).toHaveValue(E003_RAW))
    fireEvent.change(screen.getByLabelText('As of (optional)'), { target: { value: '' } })
    await user.click(screen.getByRole('button', { name: 'Save changes' }))
    expect(mocks.api.updateEvidence).toHaveBeenCalledWith('INV-0001', 'E-003', { captured_at: '' })
  })

  it('refuses an unparseable captured value instead of resetting the captured time', async () => {
    const user = userEvent.setup()
    const { onClose } = renderEdit()
    await waitFor(() => expect(screen.getByLabelText('Source detail (optional)')).toHaveValue(E003_RAW))
    const field = screen.getByLabelText('As of (optional)')
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
    await waitFor(() => expect(screen.getByLabelText('Source detail (optional)')).toHaveValue(E003_RAW))
    await user.click(screen.getByRole('button', { name: 'Save changes' }))
    expect(mocks.api.updateEvidence).not.toHaveBeenCalled()
    expect(onClose).toHaveBeenCalled()
  })

  it('does not fetch raw detail for evidence without any', () => {
    renderEdit(evidenceWith('E-002'))
    expect(mocks.api.getEvidence).not.toHaveBeenCalled()
    expect(screen.getByRole('button', { name: 'Add source detail' })).toBeInTheDocument()
  })

  it.each([
    '{"custom":{"keep":[1,"existing"]},"period":"2026-Q3"}',
    'unfinished JSON {',
  ])('keeps existing metadata untouched while the capture guide is used: %s', async (metadata) => {
    const user = userEvent.setup()
    renderEdit({ ...evidenceWith('E-002'), metadata })
    const original = (screen.getByLabelText('Extra details (optional)') as HTMLTextAreaElement).value
    await user.click(screen.getByRole('button', { name: 'What to write down' }))
    await user.selectOptions(screen.getByLabelText('Kind of source'), 'file')
    await user.click(screen.getByRole('button', { name: 'What to write down' }))
    expect(screen.getByLabelText('Extra details (optional)')).toHaveValue(original)
    await user.click(screen.getByRole('button', { name: 'Save changes' }))
    expect(mocks.api.updateEvidence).toHaveBeenCalledWith('INV-0001', 'E-002', { source_type: 'file' })
  })
})

describe('EvidenceDialog edit changed elsewhere', () => {
  beforeEach(() => {
    Object.assign(mocks.api, mockApi())
  })

  it('flags a summary changed in another app while the form holds input', async () => {
    const user = userEvent.setup()
    const e = evidenceWith('E-001')
    const ui = (ev: Evidence) => providers(<EvidenceDialog mode="edit" evidence={ev} invRef="INV-0001" onClose={vi.fn()} />)
    const { rerender } = render(ui(e))
    const dialog = screen.getByRole('dialog', { name: 'Edit source' })
    await user.type(within(dialog).getByLabelText('Title'), ' (mine)')
    rerender(ui({ ...e, summary: 'Rewritten by Claude Code.' }))
    expect(within(dialog).getByText(CHANGED_ELSEWHERE_COPY)).toBeInTheDocument()
    await user.click(within(dialog).getByRole('button', { name: 'Show latest' }))
    expect(within(dialog).getByLabelText('Summary')).toHaveValue('Rewritten by Claude Code.')
    expect(within(dialog).getByLabelText('Title')).toHaveValue(e.title)
  })
})
