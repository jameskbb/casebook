import type { ReactNode } from 'react'
import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { Api } from '../../api/api'
import { ApiError, NOT_SAVED_COPY } from '../../api/errors'
import type { Claim, Investigation, InvestigationDetail } from '../../api/types'
import { formatDate } from '../../lib/relativeTime'
import { AppDataProvider, useAppData } from '../../state/appData'
import { detail, investigation } from '../../test/fixtures'
import { mockApi } from '../../test/mockApi'
import { OpenAtLaunch } from '../../test/OpenAtLaunch'
import { CHANGED_ELSEWHERE_COPY, SHOW_NEW_VERSION } from '../ui/ChangedElsewhereBanner'
import { ToastProvider } from '../ui/Toaster'
import { CHECK_ANSWER_AGAIN, CHECK_NEXT_SOURCE, CHECKED_ANSWER_ACTION, ConclusionPanel, NOT_CHECKED_ANSWER, nextSourceToCheck, SHARE_THE_ANSWER } from './ConclusionPanel'
import { ANSWER_COPIED } from './ShareSummaryDialog'

const mocks = vi.hoisted(() => ({ api: {} as Api }))
vi.mock('../../api/api', () => ({ api: mocks.api }))

const providers = (ui: ReactNode) => (
  <ToastProvider>
    <AppDataProvider><OpenAtLaunch />{ui}</AppDataProvider>
  </ToastProvider>
)

function renderPanel(inv: Investigation = investigation, claims?: Claim[]) {
  return render(providers(<ConclusionPanel investigation={inv} claims={claims} />))
}

const panel = () => screen.getByRole('region', { name: 'The answer' })

describe('ConclusionPanel editing', () => {
  beforeEach(() => {
    Object.assign(mocks.api, mockApi())
  })

  it('opens inline edit with only Answer and How you know: no Label select and no per-finding pickers', async () => {
    const user = userEvent.setup()
    renderPanel()
    await user.click(within(panel()).getByRole('button', { name: 'Edit answer' }))
    expect(within(panel()).getByLabelText('Answer')).toHaveValue(
      'The extra calls were customers asking about a bill. A billing mistake on October 3 charged some customers twice. About the same number of people called to buy, so sales stayed the same.',
    )
    expect(within(panel()).queryByRole('radiogroup')).toBeNull()
    expect(within(panel()).queryByRole('combobox')).toBeNull()
    expect(within(panel()).queryByLabelText('Label')).toBeNull()
    expect(within(panel()).getAllByRole('textbox')).toHaveLength(2)
    expect(within(panel()).getByLabelText('How you know (optional)')).toHaveValue(
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
    const why = within(panel()).getByLabelText('How you know (optional)')
    await user.clear(why)
    await user.type(why, 'Two sources.')
    await user.click(within(panel()).getByRole('button', { name: 'Save answer' }))
    expect(mocks.api.updateConclusion).toHaveBeenCalledWith('INV-0001', {
      conclusion: 'Different definitions.',
      confidence: investigation.conclusion_confidence,
      rationale: 'Two sources.',
      supporting_claims: [],
      contradicting_claims: [],
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
    expect(within(panel()).getByText('The extra calls were customers asking about a bill. A billing mistake on October 3 charged some customers twice. About the same number of people called to buy, so sales stayed the same.')).toBeInTheDocument()
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
        'This removes the answer and how you know. The history still shows that it changed.',
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

  it('saves an emptied answer with no confidence, and shows a plain sentence, never the engine text, for an error on a hidden field', async () => {
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
    const alert = await within(panel()).findByRole('alert')
    expect(alert).toHaveTextContent(NOT_SAVED_COPY)
    expect(alert).not.toHaveTextContent('must be empty')
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

  it('keeps unsaved text when the stored conclusion changes elsewhere and offers Show the new version', async () => {
    const user = userEvent.setup()
    const { rerender } = renderPanel()
    await user.click(within(panel()).getByRole('button', { name: 'Edit answer' }))
    const text = within(panel()).getByLabelText('Answer')
    await user.type(text, ' Mine.')
    expect(within(panel()).queryByText(CHANGED_ELSEWHERE_COPY)).toBeNull()

    const elsewhere: Investigation = {
      ...investigation,
      conclusion: 'Claude Code found a third definition.',
      conclusion_confidence: 'medium',
      conclusion_rationale: 'E-005.',
    }
    rerender(providers(<ConclusionPanel investigation={elsewhere} />))
    expect(within(panel()).getByLabelText('Answer')).toHaveValue(
      'The extra calls were customers asking about a bill. A billing mistake on October 3 charged some customers twice. About the same number of people called to buy, so sales stayed the same. Mine.',
    )
    expect(within(panel()).getByText(CHANGED_ELSEWHERE_COPY)).toBeInTheDocument()

    await user.click(within(panel()).getByRole('button', { name: SHOW_NEW_VERSION }))
    expect(within(panel()).getByLabelText('Answer')).toHaveValue('Claude Code found a third definition.')
    expect(within(panel()).getByLabelText('How you know (optional)')).toHaveValue('E-005.')
    expect(within(panel()).queryByText(CHANGED_ELSEWHERE_COPY)).toBeNull()
  })

  it('shows no notice when an unrelated field of the investigation changes', async () => {
    const user = userEvent.setup()
    const { rerender } = renderPanel()
    await user.click(within(panel()).getByRole('button', { name: 'Edit answer' }))
    await user.type(within(panel()).getByLabelText('Answer'), ' Mine.')
    rerender(providers(<ConclusionPanel investigation={{ ...investigation, title: 'Renamed', status: 'blocked' }} />))
    expect(within(panel()).queryByText(CHANGED_ELSEWHERE_COPY)).toBeNull()
    expect(within(panel()).getByLabelText('Answer')).toHaveValue(
      'The extra calls were customers asking about a bill. A billing mistake on October 3 charged some customers twice. About the same number of people called to buy, so sales stayed the same. Mine.',
    )
  })

  it('follows the latest stored values while the editor has no unsaved input', async () => {
    const user = userEvent.setup()
    const { rerender } = renderPanel()
    await user.click(within(panel()).getByRole('button', { name: 'Edit answer' }))
    rerender(providers(<ConclusionPanel investigation={{ ...investigation, conclusion: 'Updated by Codex.' }} />))
    expect(within(panel()).getByLabelText('Answer')).toHaveValue('Updated by Codex.')
    expect(within(panel()).queryByText(CHANGED_ELSEWHERE_COPY)).toBeNull()
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
    expect(within(panel()).getByText(CHANGED_ELSEWHERE_COPY)).toBeInTheDocument()
  })
})


describe('Check the next source and Share the answer', () => {
  beforeEach(() => { Object.assign(mocks.api, mockApi()) })

  it('picks the first source a person has not checked, skipping replaced ones', () => {
    const [e1, e2, , e4] = detail.evidence
    expect(nextSourceToCheck(detail.evidence)?.ref).toBe('E-001')
    expect(nextSourceToCheck([{ ...e1, verification: 'superseded' }, { ...e2, verification: 'source_verified' }, e4])?.ref).toBe('E-002')
    expect(nextSourceToCheck([e4, { ...e1, verification: 'human_verified' }])).toBeUndefined()
    expect(nextSourceToCheck([])).toBeUndefined()
  })

  it('opens the next source to check, and Share the answer steps back to secondary beside it', async () => {
    const user = userEvent.setup()
    const onOpenSource = vi.fn()
    render(providers(<ConclusionPanel investigation={investigation} evidence={detail.evidence} onOpenSource={onOpenSource} share />))
    const next = within(panel()).getByRole('button', { name: CHECK_NEXT_SOURCE })
    expect(next).toHaveClass('btn-primary')
    expect(within(panel()).getByRole('button', { name: SHARE_THE_ANSWER })).toHaveClass('btn-secondary')
    await user.click(next)
    expect(onOpenSource).toHaveBeenCalledWith('E-001')
  })

  it('offers only Share the answer, as the primary action, once every source is checked', async () => {
    const user = userEvent.setup()
    const checked = detail.evidence.map((e) => ({ ...e, verification: 'human_verified' as const }))
    render(providers(<ConclusionPanel investigation={investigation} evidence={checked} onOpenSource={() => {}} share />))
    expect(within(panel()).queryByRole('button', { name: CHECK_NEXT_SOURCE })).toBeNull()
    const share = within(panel()).getByRole('button', { name: 'Share the answer' })
    expect(share).toHaveClass('btn-primary')
    await user.click(share)
    const dialog = await screen.findByRole('dialog', { name: 'Share the answer' })
    expect(mocks.api.sharePreview).toHaveBeenCalledWith('INV-0001', { mode: 'brief' })
    await user.click(within(dialog).getByRole('button', { name: 'Copy' }))
    expect(mocks.api.copySharePreview).toHaveBeenCalledWith('reviewed-1')
    expect(await screen.findByText(ANSWER_COPIED)).toBeInTheDocument()
  })

  it('offers both while there is no answer yet, and neither unless asked for', () => {
    const { unmount } = render(providers(<ConclusionPanel investigation={{ ...investigation, conclusion: '' }} evidence={detail.evidence} onOpenSource={() => {}} share />))
    expect(within(panel()).getByRole('button', { name: CHECK_NEXT_SOURCE })).toBeInTheDocument()
    expect(within(panel()).getByRole('button', { name: 'Share the answer' })).toBeInTheDocument()
    unmount()
    renderPanel()
    expect(within(panel()).queryByRole('button', { name: CHECK_NEXT_SOURCE })).toBeNull()
    expect(within(panel()).queryByRole('button', { name: 'Share the answer' })).toBeNull()
  })
})

describe('The findings an answer is saved with', () => {
  beforeEach(() => { Object.assign(mocks.api, mockApi()) })

  const edit = async (inv: Investigation, claims?: Claim[]) => {
    const user = userEvent.setup()
    renderPanel(inv, claims)
    await user.click(within(panel()).getByRole('button', { name: 'Edit answer' }))
    await user.type(within(panel()).getByLabelText('Answer'), ' More.')
    await user.click(within(panel()).getByRole('button', { name: 'Save answer' }))
    return vi.mocked(mocks.api.updateConclusion).mock.calls[0][1]
  }

  it('sends the findings that stand as supporting and the ones where sources disagree as contradicting', async () => {
    // C-001 and C-002 stand, C-003 was replaced, C-004 has sources that disagree, C-005 is ruled out.
    const ruledOut: Claim = { ...detail.claims[1], id: 5, ref: 'C-005', status: 'rejected' }
    const sent = await edit(investigation, [...detail.claims, ruledOut])
    expect(sent.supporting_claims).toEqual(['C-001', 'C-002'])
    expect(sent.contradicting_claims).toEqual(['C-004'])
    expect(sent.confidence).toBe(investigation.conclusion_confidence)
  })

  it('marks an answer the person had checked as not checked any more when it changes', async () => {
    const sent = await edit({ ...investigation, conclusion_verification: 'human_verified' })
    expect(sent.verification).toBe('ai_generated')
  })

  it('sends no verification when the person had not checked the answer', async () => {
    const sent = await edit(investigation)
    expect(sent).not.toHaveProperty('verification')
  })
})

describe('Checking the answer', () => {
  beforeEach(() => { Object.assign(mocks.api, mockApi()) })

  const checkButton = () => within(panel()).queryByRole('button', { name: CHECKED_ANSWER_ACTION })
  const checkedAt = '2026-10-20T12:00:00.000Z'

  it('says the answer is not checked yet and offers I checked this answer', () => {
    renderPanel()
    expect(within(panel()).getByRole('status')).toHaveTextContent(NOT_CHECKED_ANSWER)
    expect(checkButton()).toBeInTheDocument()
  })

  it('asks for another check when something changed after the check', () => {
    renderPanel({ ...investigation, conclusion_verification: 'human_verified', conclusion_reviewed_at: checkedAt, conclusion_review_basis: 'Compared the call log.', conclusion_needs_review: true })
    expect(within(panel()).getByRole('status')).toHaveTextContent(CHECK_ANSWER_AGAIN)
    expect(checkButton()).toBeInTheDocument()
  })

  it('says when the person checked the answer, shows what they checked and offers no check button', () => {
    renderPanel({ ...investigation, conclusion_verification: 'human_verified', conclusion_reviewed_at: checkedAt, conclusion_review_basis: 'Compared the call log.', conclusion_needs_review: false })
    expect(within(panel()).getByRole('status')).toHaveTextContent(`You checked this answer on ${formatDate(checkedAt)}.`)
    expect(within(panel()).getByText(/Compared the call log/)).toBeInTheDocument()
    expect(checkButton()).toBeNull()
  })

  it('opens What did you check?, needs words, and sends the check with the token of what is shown', async () => {
    const user = userEvent.setup()
    renderPanel()
    await user.click(checkButton()!)
    const basis = within(panel()).getByLabelText('What did you check?')
    expect(basis).toHaveFocus()
    const record = within(panel()).getByRole('button', { name: CHECKED_ANSWER_ACTION })
    expect(record).toBeDisabled()
    await user.type(basis, 'Matched the findings to the original export.')
    await user.click(record)
    expect(mocks.api.updateConclusion).toHaveBeenCalledWith('INV-0001', {
      conclusion: investigation.conclusion,
      confidence: investigation.conclusion_confidence,
      rationale: investigation.conclusion_rationale,
      verification: 'human_verified',
      review: true,
      review_basis: 'Matched the findings to the original export.',
      expected_review_token: investigation.conclusion_review_token,
    })
  })

  it('Cancel closes the form without saving and returns focus to the button', async () => {
    const user = userEvent.setup()
    renderPanel()
    await user.click(checkButton()!)
    await user.type(within(panel()).getByLabelText('What did you check?'), 'Half a thought')
    await user.click(within(panel()).getByRole('button', { name: 'Cancel' }))
    expect(within(panel()).queryByLabelText('What did you check?')).toBeNull()
    expect(checkButton()).toHaveFocus()
    expect(mocks.api.updateConclusion).not.toHaveBeenCalled()
  })

  it('keeps the words but stops the check when the answer changes while the form is open', async () => {
    const user = userEvent.setup()
    const { rerender } = renderPanel()
    await user.click(checkButton()!)
    await user.type(within(panel()).getByLabelText('What did you check?'), 'Original export.')
    expect(within(panel()).queryByText(CHANGED_ELSEWHERE_COPY)).toBeNull()
    rerender(providers(<ConclusionPanel investigation={{ ...investigation, conclusion_review_token: 'changed-basis' }} />))
    expect(within(panel()).getByLabelText('What did you check?')).toHaveValue('Original export.')
    expect(within(panel()).getByText(CHANGED_ELSEWHERE_COPY)).toBeInTheDocument()
    expect(within(panel()).getByRole('button', { name: CHECKED_ANSWER_ACTION })).toBeDisabled()
    expect(mocks.api.updateConclusion).not.toHaveBeenCalled()

    await user.click(within(panel()).getByRole('button', { name: SHOW_NEW_VERSION }))
    expect(within(panel()).queryByText(CHANGED_ELSEWHERE_COPY)).toBeNull()
    await user.click(within(panel()).getByRole('button', { name: CHECKED_ANSWER_ACTION }))
    expect(mocks.api.updateConclusion).toHaveBeenCalledWith('INV-0001', expect.objectContaining({ review_basis: 'Original export.', expected_review_token: 'changed-basis' }))
  })
})

describe('Share the answer and codes in the text', () => {
  beforeEach(() => { Object.assign(mocks.api, mockApi()) })

  it('keeps the Share the answer dialog open when the answer goes away while it is open', async () => {
    const user = userEvent.setup()
    const { rerender } = render(providers(<ConclusionPanel investigation={investigation} share />))
    await user.click(within(panel()).getByRole('button', { name: SHARE_THE_ANSWER }))
    await screen.findByRole('dialog', { name: 'Share the answer' })
    rerender(providers(<ConclusionPanel investigation={{ ...investigation, conclusion: '' }} share />))
    expect(screen.getByRole('dialog', { name: 'Share the answer' })).toBeInTheDocument()
    expect(mocks.api.sharePreview).toHaveBeenCalledTimes(1)
  })

  it('wraps a code in brackets in the answer and in How you know in a record-id element', () => {
    renderPanel({ ...investigation, conclusion: 'A billing mistake charged customers twice (E-007).', conclusion_rationale: 'The notice says so (E-001, C-002).' })
    const codes = (selector: string) => Array.from(panel().querySelectorAll(`${selector} .record-id`), (el) => el.textContent)
    expect(codes('.conclusion-text')).toEqual([' (E-007)'])
    expect(codes('.conclusion-rationale')).toEqual([' (E-001, C-002)'])
    expect(panel().querySelector('.conclusion-text')).toHaveTextContent('A billing mistake charged customers twice (E-007).')
  })
})
