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

const panel = () => screen.getByRole('region', { name: 'Current conclusion' })

describe('ConclusionPanel editing', () => {
  beforeEach(() => {
    Object.assign(mocks.api, mockApi())
  })

  it('opens inline edit with the current text, confidence and rationale', async () => {
    const user = userEvent.setup()
    renderPanel()
    await user.click(within(panel()).getByRole('button', { name: 'Update conclusion' }))
    expect(within(panel()).getByLabelText('Conclusion')).toHaveValue(
      'Debug logging requested for payments only reached every service and drove most of the increase.',
    )
    expect(within(panel()).getByRole('radio', { name: 'High' })).toHaveAttribute('aria-checked', 'true')
    expect(within(panel()).getByLabelText('Why this confidence (optional)')).toHaveValue(
      investigation.conclusion_rationale,
    )
    expect(within(panel()).getByLabelText('Conclusion')).toHaveFocus()
  })

  it('saves the conclusion, confidence and rationale and returns to view mode without a toast', async () => {
    const user = userEvent.setup()
    renderPanel()
    await user.click(within(panel()).getByRole('button', { name: 'Update conclusion' }))
    const text = within(panel()).getByLabelText('Conclusion')
    await user.clear(text)
    await user.type(text, 'Different definitions.')
    await user.click(within(panel()).getByRole('radio', { name: 'Medium' }))
    const why = within(panel()).getByLabelText('Why this confidence (optional)')
    await user.clear(why)
    await user.type(why, 'Two sources.')
    await user.click(within(panel()).getByRole('button', { name: 'Save conclusion' }))
    expect(mocks.api.updateConclusion).toHaveBeenCalledWith('INV-0001', {
      conclusion: 'Different definitions.',
      confidence: 'medium',
      rationale: 'Two sources.',
      verification: investigation.conclusion_verification,
      supporting_claims: investigation.conclusion_supporting_claims,
      contradicting_claims: investigation.conclusion_contradicting_claims,
    })
    await waitFor(() => expect(within(panel()).queryByLabelText('Conclusion')).toBeNull())
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
    await user.click(within(panel()).getByRole('button', { name: 'Update conclusion' }))
    await user.type(within(panel()).getByLabelText('Conclusion'), ' Confirmed.')
    await user.click(within(panel()).getByRole('button', { name: 'Save conclusion' }))
    expect(within(panel()).getByRole('button', { name: 'Saving...' })).toBeDisabled()
    resolve(investigation)
    await waitFor(() => expect(within(panel()).queryByRole('button', { name: 'Saving...' })).toBeNull())
  })

  it('cancels back to view mode without saving', async () => {
    const user = userEvent.setup()
    renderPanel()
    await user.click(within(panel()).getByRole('button', { name: 'Update conclusion' }))
    await user.type(within(panel()).getByLabelText('Conclusion'), ' More.')
    await user.click(within(panel()).getByRole('button', { name: 'Cancel' }))
    expect(within(panel()).queryByLabelText('Conclusion')).toBeNull()
    expect(within(panel()).getByText('Debug logging requested for payments only reached every service and drove most of the increase.')).toBeInTheDocument()
    expect(mocks.api.updateConclusion).not.toHaveBeenCalled()
  })

  it('returns to view mode without a call when nothing changed', async () => {
    const user = userEvent.setup()
    renderPanel()
    await user.click(within(panel()).getByRole('button', { name: 'Update conclusion' }))
    await user.click(within(panel()).getByRole('button', { name: 'Save conclusion' }))
    expect(within(panel()).queryByLabelText('Conclusion')).toBeNull()
    expect(mocks.api.updateConclusion).not.toHaveBeenCalled()
    expect(within(panel()).getByRole('button', { name: 'Update conclusion' })).toHaveFocus()
  })

  it('asks before clearing and then sends an empty conclusion, confidence and rationale', async () => {
    const user = userEvent.setup()
    renderPanel()
    await user.click(within(panel()).getByRole('button', { name: 'Update conclusion' }))
    await user.click(within(panel()).getByRole('button', { name: 'Clear conclusion' }))
    const confirm = screen.getByRole('dialog', { name: 'Clear the current conclusion?' })
    expect(
      within(confirm).getByText(
        'This removes the conclusion, its confidence and its rationale. The history keeps a record that it changed.',
      ),
    ).toBeInTheDocument()
    expect(within(confirm).getByRole('button', { name: 'Cancel' })).toHaveFocus()
    expect(mocks.api.updateConclusion).not.toHaveBeenCalled()
    await user.click(within(confirm).getByRole('button', { name: 'Clear conclusion' }))
    expect(mocks.api.updateConclusion).toHaveBeenCalledWith('INV-0001', {
      conclusion: '',
      confidence: '',
      rationale: '',
    })
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
  })

  it('renders an engine validation error on confidence under the confidence picker', async () => {
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
    await user.click(within(panel()).getByRole('button', { name: 'Update conclusion' }))
    await user.clear(within(panel()).getByLabelText('Conclusion'))
    await user.click(within(panel()).getByRole('button', { name: 'Save conclusion' }))
    const group = within(panel()).getByRole('radiogroup', { name: 'Confidence' })
    const error = await within(panel()).findByText('must be empty or unknown when the conclusion is empty')
    expect(group).toHaveAttribute('aria-describedby', expect.stringContaining(error.parentElement!.id))
    expect(group.compareDocumentPosition(error) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    expect(within(panel()).getByLabelText('Conclusion')).toBeInTheDocument()
  })

  it('writes a first conclusion from the empty state', async () => {
    const user = userEvent.setup()
    renderPanel({ ...investigation, conclusion: '', conclusion_confidence: 'unknown', conclusion_rationale: '' })
    expect(
      within(panel()).getByText('No conclusion yet. When the evidence points somewhere, write it here.'),
    ).toBeInTheDocument()
    await user.click(within(panel()).getByRole('button', { name: 'Write a conclusion' }))
    expect(within(panel()).getByLabelText('Conclusion')).toHaveValue('')
    expect(within(panel()).getByRole('radio', { name: 'Not assessed' })).toHaveAttribute('aria-checked', 'true')
    expect(within(panel()).queryByRole('button', { name: 'Clear conclusion' })).toBeNull()
  })
})

describe('ConclusionPanel edit-in-place safety', () => {
  beforeEach(() => {
    Object.assign(mocks.api, mockApi())
  })

  it('keeps unsaved text when the stored conclusion changes elsewhere and offers Show latest', async () => {
    const user = userEvent.setup()
    const { rerender } = renderPanel()
    await user.click(within(panel()).getByRole('button', { name: 'Update conclusion' }))
    const text = within(panel()).getByLabelText('Conclusion')
    await user.type(text, ' Mine.')
    expect(within(panel()).queryByText('This changed in another app.')).toBeNull()

    const elsewhere: Investigation = {
      ...investigation,
      conclusion: 'Claude Code found a third definition.',
      conclusion_confidence: 'medium',
      conclusion_rationale: 'E-005.',
    }
    rerender(providers(<ConclusionPanel investigation={elsewhere} />))
    expect(within(panel()).getByLabelText('Conclusion')).toHaveValue(
      'Debug logging requested for payments only reached every service and drove most of the increase. Mine.',
    )
    expect(within(panel()).getByText('This changed in another app.')).toBeInTheDocument()

    await user.click(within(panel()).getByRole('button', { name: 'Show latest' }))
    expect(within(panel()).getByLabelText('Conclusion')).toHaveValue('Claude Code found a third definition.')
    expect(within(panel()).getByRole('radio', { name: 'Medium' })).toHaveAttribute('aria-checked', 'true')
    expect(within(panel()).getByLabelText('Why this confidence (optional)')).toHaveValue('E-005.')
    expect(within(panel()).queryByText('This changed in another app.')).toBeNull()
  })

  it('shows no notice when an unrelated field of the investigation changes', async () => {
    const user = userEvent.setup()
    const { rerender } = renderPanel()
    await user.click(within(panel()).getByRole('button', { name: 'Update conclusion' }))
    await user.type(within(panel()).getByLabelText('Conclusion'), ' Mine.')
    rerender(providers(<ConclusionPanel investigation={{ ...investigation, title: 'Renamed', status: 'blocked' }} />))
    expect(within(panel()).queryByText('This changed in another app.')).toBeNull()
    expect(within(panel()).getByLabelText('Conclusion')).toHaveValue(
      'Debug logging requested for payments only reached every service and drove most of the increase. Mine.',
    )
  })

  it('follows the latest stored values while the editor has no unsaved input', async () => {
    const user = userEvent.setup()
    const { rerender } = renderPanel()
    await user.click(within(panel()).getByRole('button', { name: 'Update conclusion' }))
    rerender(providers(<ConclusionPanel investigation={{ ...investigation, conclusion: 'Updated by Codex.' }} />))
    expect(within(panel()).getByLabelText('Conclusion')).toHaveValue('Updated by Codex.')
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
    fireEvent.click(within(panel()).getByRole('button', { name: 'Update conclusion' }))
    fireEvent.change(within(panel()).getByLabelText('Conclusion'), { target: { value: 'My draft.' } })

    current = { ...detail, investigation: { ...investigation, conclusion: 'Written over MCP.' } }
    token = '2.2'
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1500)
    })
    expect(mocks.api.getInvestigationDetail).toHaveBeenCalledTimes(2)
    expect(within(panel()).getByLabelText('Conclusion')).toHaveValue('My draft.')
    expect(within(panel()).getByText('This changed in another app.')).toBeInTheDocument()
  })
})


describe('Conclusion accountability', () => {
  beforeEach(() => { Object.assign(mocks.api, mockApi()) })

  it('keeps the historical statement verification visible when the basis changed', () => {
    renderPanel({ ...investigation, conclusion_verification: 'human_verified', conclusion_reviewed_at: '2026-10-20T12:00:00.000Z', conclusion_review_basis: 'Compared the billing export.', conclusion_needs_review: true })
    expect(within(panel()).getByText('Human verified')).toBeInTheDocument()
    expect(within(panel()).getByText(/Selected claims or their evidence changed/)).toBeInTheDocument()
    expect(within(panel()).getByText(/Compared the billing export/)).toBeInTheDocument()
  })

  it('saves selected contradicting claim refs with the conclusion', async () => {
    const user = userEvent.setup()
    render(providers(<ConclusionPanel investigation={investigation} claims={detail.claims} />))
    await user.click(within(panel()).getByRole('button', { name: 'Update conclusion' }))
    await user.selectOptions(within(panel()).getByRole('combobox', { name: /C-001:/ }), 'contradicts')
    await user.click(within(panel()).getByRole('button', { name: 'Save conclusion' }))
    expect(mocks.api.updateConclusion).toHaveBeenCalledWith('INV-0001', expect.objectContaining({ supporting_claims: [], contradicting_claims: ['C-001'] }))
  })

  it('records an explicit review with the token of the displayed records', async () => {
    const user = userEvent.setup()
    renderPanel()
    await user.click(within(panel()).getByRole('button', { name: 'Review conclusion' }))
    const record = within(panel()).getByRole('button', { name: 'Verify and record review' })
    expect(record).toBeDisabled()
    await user.type(within(panel()).getByLabelText('What did you check?'), 'Matched the selected claims to the original export.')
    await user.click(record)
    expect(mocks.api.updateConclusion).toHaveBeenCalledWith('INV-0001', expect.objectContaining({ verification: 'human_verified', review: true, review_basis: 'Matched the selected claims to the original export.', expected_review_token: investigation.conclusion_review_token }))
  })

  it('keeps a stale review draft and prevents endorsing changed material', async () => {
    const user = userEvent.setup()
    const { rerender } = renderPanel()
    await user.click(within(panel()).getByRole('button', { name: 'Review conclusion' }))
    await user.type(within(panel()).getByLabelText('What did you check?'), 'Original export.')
    rerender(providers(<ConclusionPanel investigation={{ ...investigation, conclusion_review_token: 'changed-basis' }} />))
    expect(within(panel()).getByLabelText('What did you check?')).toHaveValue('Original export.')
    expect(within(panel()).getByText(/This conclusion or its basis changed/)).toBeInTheDocument()
    expect(within(panel()).getByRole('button', { name: 'Verify and record review' })).toBeDisabled()
    expect(mocks.api.updateConclusion).not.toHaveBeenCalled()
  })
})
