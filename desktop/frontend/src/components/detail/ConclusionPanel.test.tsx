import type { ReactNode } from 'react'
import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { Api } from '../../api/api'
import { ApiError } from '../../api/errors'
import type { Investigation, InvestigationDetail } from '../../api/types'
import { AppDataProvider, useAppData } from '../../state/appData'
import { detail, investigation } from '../../test/fixtures'
import { mockApi } from '../../test/mockApi'
import { ToastProvider } from '../ui/Toaster'
import { ConclusionPanel } from './ConclusionPanel'

const mocks = vi.hoisted(() => ({ api: {} as Api }))
vi.mock('../../api/api', () => ({ api: mocks.api }))

const providers = (ui: ReactNode) => (
  <ToastProvider>
    <AppDataProvider>{ui}</AppDataProvider>
  </ToastProvider>
)

function renderPanel(inv: Investigation = investigation) {
  return render(providers(<ConclusionPanel investigation={inv} />))
}

const panel = () => screen.getByRole('region', { name: 'The answer' })

describe('ConclusionPanel editing', () => {
  beforeEach(() => {
    Object.assign(mocks.api, mockApi())
  })

  it('opens inline edit with the current text and rationale, and no confidence control', async () => {
    const user = userEvent.setup()
    renderPanel()
    await user.click(within(panel()).getByRole('button', { name: 'Edit answer' }))
    expect(within(panel()).getByLabelText('Answer')).toHaveValue(
      'Debug logging requested for payments only reached every service and drove most of the increase.',
    )
    expect(within(panel()).queryByRole('radiogroup', { name: 'Confidence' })).toBeNull()
    expect(within(panel()).getByLabelText('Why (optional)')).toHaveValue(
      investigation.conclusion_rationale,
    )
    expect(within(panel()).getByLabelText('Answer')).toHaveFocus()
  })

  it('saves the answer and rationale with the stored confidence and returns to view mode without a toast', async () => {
    const user = userEvent.setup()
    renderPanel()
    await user.click(within(panel()).getByRole('button', { name: 'Edit answer' }))
    const text = within(panel()).getByLabelText('Answer')
    await user.clear(text)
    await user.type(text, 'Different definitions.')
    const why = within(panel()).getByLabelText('Why (optional)')
    await user.clear(why)
    await user.type(why, 'Two sources.')
    await user.click(within(panel()).getByRole('button', { name: 'Save answer' }))
    expect(mocks.api.updateConclusion).toHaveBeenCalledWith('INV-0001', {
      conclusion: 'Different definitions.',
      confidence: investigation.conclusion_confidence,
      rationale: 'Two sources.',
      verification: investigation.conclusion_verification,
      supporting_claims: investigation.conclusion_supporting_claims,
      contradicting_claims: investigation.conclusion_contradicting_claims,
    })
    await waitFor(() => expect(within(panel()).queryByLabelText('Answer')).toBeNull())
    expect(document.querySelector('.toast')).toBeNull()
  })

  it('shows Saving... while the save is pending', async () => {
    const user = userEvent.setup()
    let resolve: (v: Investigation) => void = () => {}
    Object.assign(
      mocks.api,
      mockApi({ updateConclusion: () => new Promise<Investigation>((r) => (resolve = r)) }),
    )
    renderPanel()
    await user.click(within(panel()).getByRole('button', { name: 'Edit answer' }))
    await user.type(within(panel()).getByLabelText('Answer'), ' Confirmed.')
    await user.click(within(panel()).getByRole('button', { name: 'Save answer' }))
    expect(within(panel()).getByRole('button', { name: 'Saving...' })).toBeDisabled()
    resolve(investigation)
    await waitFor(() => expect(within(panel()).queryByRole('button', { name: 'Saving...' })).toBeNull())
  })

  it('cancels back to view mode without saving', async () => {
    const user = userEvent.setup()
    renderPanel()
    await user.click(within(panel()).getByRole('button', { name: 'Edit answer' }))
    await user.type(within(panel()).getByLabelText('Answer'), ' More.')
    await user.click(within(panel()).getByRole('button', { name: 'Cancel' }))
    expect(within(panel()).queryByLabelText('Answer')).toBeNull()
    expect(within(panel()).getByText('Debug logging requested for payments only reached every service and drove most of the increase.')).toBeInTheDocument()
    expect(mocks.api.updateConclusion).not.toHaveBeenCalled()
  })

  it('returns to view mode without a call when nothing changed', async () => {
    const user = userEvent.setup()
    renderPanel()
    await user.click(within(panel()).getByRole('button', { name: 'Edit answer' }))
    await user.click(within(panel()).getByRole('button', { name: 'Save answer' }))
    expect(within(panel()).queryByLabelText('Answer')).toBeNull()
    expect(mocks.api.updateConclusion).not.toHaveBeenCalled()
    expect(within(panel()).getByRole('button', { name: 'Edit answer' })).toHaveFocus()
  })

  it('asks before clearing and then sends an empty conclusion, confidence and rationale', async () => {
    const user = userEvent.setup()
    renderPanel()
    await user.click(within(panel()).getByRole('button', { name: 'Edit answer' }))
    await user.click(within(panel()).getByRole('button', { name: 'Clear answer' }))
    const confirm = screen.getByRole('dialog', { name: 'Clear the answer?' })
    expect(
      within(confirm).getByText(
        'This removes the answer and its notes. The history keeps a record that it changed.',
      ),
    ).toBeInTheDocument()
    expect(within(confirm).getByRole('button', { name: 'Cancel' })).toHaveFocus()
    expect(mocks.api.updateConclusion).not.toHaveBeenCalled()
    await user.click(within(confirm).getByRole('button', { name: 'Clear answer' }))
    expect(mocks.api.updateConclusion).toHaveBeenCalledWith('INV-0001', {
      conclusion: '',
      confidence: '',
      rationale: '',
    })
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
  })

  it('saves an emptied answer with no confidence, and shows an engine error on a hidden field as a summary', async () => {
    const user = userEvent.setup()
    Object.assign(
      mocks.api,
      mockApi({
        updateConclusion: async () => {
          throw new ApiError('validation', 'must be empty or unknown when the conclusion is empty', 'confidence')
        },
      }),
    )
    renderPanel()
    await user.click(within(panel()).getByRole('button', { name: 'Edit answer' }))
    await user.clear(within(panel()).getByLabelText('Answer'))
    await user.click(within(panel()).getByRole('button', { name: 'Save answer' }))
    expect(mocks.api.updateConclusion).toHaveBeenCalledWith('INV-0001', { conclusion: '', confidence: '', rationale: investigation.conclusion_rationale })
    expect(await within(panel()).findByRole('alert')).toHaveTextContent('must be empty or unknown when the conclusion is empty')
    expect(within(panel()).getByLabelText('Answer')).toBeInTheDocument()
  })

  it('writes a first conclusion from the empty state', async () => {
    const user = userEvent.setup()
    renderPanel({ ...investigation, conclusion: '', conclusion_confidence: 'unknown', conclusion_rationale: '' })
    expect(
      within(panel()).getByText('No answer yet. Write it here when the findings back one.'),
    ).toBeInTheDocument()
    await user.click(within(panel()).getByRole('button', { name: 'Write the answer' }))
    expect(within(panel()).getByLabelText('Answer')).toHaveValue('')

    expect(within(panel()).queryByRole('button', { name: 'Clear answer' })).toBeNull()
  })
})

describe('ConclusionPanel edit-in-place safety', () => {
  beforeEach(() => {
    Object.assign(mocks.api, mockApi())
  })

  it('keeps unsaved text when the stored conclusion changes elsewhere and offers Show latest', async () => {
    const user = userEvent.setup()
    const { rerender } = renderPanel()
    await user.click(within(panel()).getByRole('button', { name: 'Edit answer' }))
    const text = within(panel()).getByLabelText('Answer')
    await user.type(text, ' Mine.')
    expect(within(panel()).queryByText('This changed in another app.')).toBeNull()

    const elsewhere: Investigation = {
      ...investigation,
      conclusion: 'Claude Code found a third definition.',
      conclusion_confidence: 'medium',
      conclusion_rationale: 'E-005.',
    }
    rerender(providers(<ConclusionPanel investigation={elsewhere} />))
    expect(within(panel()).getByLabelText('Answer')).toHaveValue(
      'Debug logging requested for payments only reached every service and drove most of the increase. Mine.',
    )
    expect(within(panel()).getByText('This changed in another app.')).toBeInTheDocument()

    await user.click(within(panel()).getByRole('button', { name: 'Show latest' }))
    expect(within(panel()).getByLabelText('Answer')).toHaveValue('Claude Code found a third definition.')
    expect(within(panel()).getByLabelText('Why (optional)')).toHaveValue('E-005.')
    expect(within(panel()).queryByText('This changed in another app.')).toBeNull()
  })

  it('shows no notice when an unrelated field of the investigation changes', async () => {
    const user = userEvent.setup()
    const { rerender } = renderPanel()
    await user.click(within(panel()).getByRole('button', { name: 'Edit answer' }))
    await user.type(within(panel()).getByLabelText('Answer'), ' Mine.')
    rerender(providers(<ConclusionPanel investigation={{ ...investigation, title: 'Renamed', status: 'blocked' }} />))
    expect(within(panel()).queryByText('This changed in another app.')).toBeNull()
    expect(within(panel()).getByLabelText('Answer')).toHaveValue(
      'Debug logging requested for payments only reached every service and drove most of the increase. Mine.',
    )
  })

  it('follows the latest stored values while the editor has no unsaved input', async () => {
    const user = userEvent.setup()
    const { rerender } = renderPanel()
    await user.click(within(panel()).getByRole('button', { name: 'Edit answer' }))
    rerender(providers(<ConclusionPanel investigation={{ ...investigation, conclusion: 'Updated by Codex.' }} />))
    expect(within(panel()).getByLabelText('Answer')).toHaveValue('Updated by Codex.')
    expect(within(panel()).queryByText('This changed in another app.')).toBeNull()
  })
})

describe('ConclusionPanel with live polling (backstop)', () => {
  afterEach(() => {
    vi.useRealTimers()
    vi.restoreAllMocks()
  })

  function Live() {
    const { detail: d } = useAppData()
    return d ? <ConclusionPanel investigation={d.investigation} /> : null
  }

  it('a ChangeToken change while the editor holds unsaved text keeps the text and shows the notice', async () => {
    vi.useFakeTimers()
    Object.defineProperty(document, 'visibilityState', { configurable: true, get: () => 'visible' })
    vi.spyOn(document, 'hasFocus').mockImplementation(() => true)
    let token = '1.1'
    let current: InvestigationDetail = detail
    Object.assign(
      mocks.api,
      mockApi({ changeToken: async () => token, getInvestigationDetail: async () => current }),
    )
    render(providers(<Live />))
    await act(async () => {
      await vi.advanceTimersByTimeAsync(0)
    })
    fireEvent.click(within(panel()).getByRole('button', { name: 'Edit answer' }))
    fireEvent.change(within(panel()).getByLabelText('Answer'), { target: { value: 'My draft.' } })

    current = { ...detail, investigation: { ...investigation, conclusion: 'Written over MCP.' } }
    token = '2.2'
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1500)
    })
    expect(mocks.api.getInvestigationDetail).toHaveBeenCalledTimes(2)
    expect(within(panel()).getByLabelText('Answer')).toHaveValue('My draft.')
    expect(within(panel()).getByText('This changed in another app.')).toBeInTheDocument()
  })
})


describe('Conclusion accountability', () => {
  beforeEach(() => { Object.assign(mocks.api, mockApi()) })

  it('keeps the historical statement verification visible when the basis changed', () => {
    renderPanel({ ...investigation, conclusion_verification: 'human_verified', conclusion_reviewed_at: '2026-10-20T12:00:00.000Z', conclusion_review_basis: 'Compared the billing export.', conclusion_needs_review: true })
    expect(within(panel()).getByText('You checked this')).toBeInTheDocument()
    expect(within(panel()).getByText(/The findings behind this answer, or their sources, changed since your review/)).toBeInTheDocument()
    expect(within(panel()).getByText(/Compared the billing export/)).toBeInTheDocument()
  })

  it('saves selected contradicting claim refs with the conclusion', async () => {
    const user = userEvent.setup()
    render(providers(<ConclusionPanel investigation={investigation} claims={detail.claims} />))
    await user.click(within(panel()).getByRole('button', { name: 'Edit answer' }))
    await user.selectOptions(within(panel()).getByRole('combobox', { name: /C-001:/ }), 'contradicts')
    await user.click(within(panel()).getByRole('button', { name: 'Save answer' }))
    expect(mocks.api.updateConclusion).toHaveBeenCalledWith('INV-0001', expect.objectContaining({ supporting_claims: [], contradicting_claims: ['C-001'] }))
  })

  it('records an explicit review with the token of the displayed records', async () => {
    const user = userEvent.setup()
    renderPanel()
    await user.click(within(panel()).getByRole('button', { name: 'Review the answer' }))
    const record = within(panel()).getByRole('button', { name: 'I checked this answer' })
    expect(record).toBeDisabled()
    await user.type(within(panel()).getByLabelText('What did you check?'), 'Matched the selected claims to the original export.')
    await user.click(record)
    expect(mocks.api.updateConclusion).toHaveBeenCalledWith('INV-0001', expect.objectContaining({ verification: 'human_verified', review: true, review_basis: 'Matched the selected claims to the original export.', expected_review_token: investigation.conclusion_review_token }))
  })

  it('keeps a stale review draft and prevents endorsing changed material', async () => {
    const user = userEvent.setup()
    const { rerender } = renderPanel()
    await user.click(within(panel()).getByRole('button', { name: 'Review the answer' }))
    await user.type(within(panel()).getByLabelText('What did you check?'), 'Original export.')
    rerender(providers(<ConclusionPanel investigation={{ ...investigation, conclusion_review_token: 'changed-basis' }} />))
    expect(within(panel()).getByLabelText('What did you check?')).toHaveValue('Original export.')
    expect(within(panel()).getByText(/This answer or the findings behind it changed/)).toBeInTheDocument()
    expect(within(panel()).getByRole('button', { name: 'I checked this answer' })).toBeDisabled()
    expect(mocks.api.updateConclusion).not.toHaveBeenCalled()
  })
})
