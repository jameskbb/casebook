import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { api } from '../../api/api'
import { WENT_WRONG_COPY } from '../../api/errors'
import type { SharePreview } from '../../api/types'
import { ToastProvider } from '../ui/Toaster'
import { ANSWER_COPIED, ANSWER_SAVED, SHARE_CHANGED, SHARE_LOADING, SHARE_READY, ShareSummaryDialog } from './ShareSummaryDialog'

vi.mock('../../api/api', () => ({ api: { sharePreview: vi.fn(), copySharePreview: vi.fn(), saveSharePreview: vi.fn(), copyText: vi.fn() } }))
const mocked = vi.mocked(api)
const preview = (patch: Partial<SharePreview> = {}): SharePreview => ({
  markdown: '# INV-0001: Margin\n\n## Question\n\nWhy?\n',
  html: '<h1>INV-0001: Margin</h1>\n<h2>Question</h2>\n<p>Why?</p>\n',
  snapshot_id: 'reviewed-original', investigation_ref: 'INV-0001', mode: 'brief',
  candidates: [
    { ref: 'C-001', text: 'Selected cost change', label: 'Not checked', group: 'claims', required: false, selected: true },
    { ref: 'C-002', text: 'Material cost went down', label: 'Sources disagree', group: 'claims', required: true, selected: true },
    { ref: 'H-003', text: 'Policy change explains it', label: 'Looks right so far; Not checked', group: 'hypotheses', required: false, selected: true },
    { ref: 'H-004', text: 'Travel is growing', label: 'Not tested yet; Not checked', group: 'hypotheses', required: false, selected: false },
  ], omitted: { claims: 0, hypotheses: 1, questions: 1, evidence: 2, claim_refs: [], hypothesis_refs: ['H-004'], question_refs: ['Q-001'], evidence_refs: ['E-002', 'E-003'] }, ...patch,
})
function renderDialog(invRef = 'INV-0001') {
  return render(<ToastProvider><ShareSummaryDialog invRef={invRef} onClose={() => {}} /></ToastProvider>)
}
const choices = () => screen.getByText('Choose what to share', { selector: 'summary' })

describe('ShareSummaryDialog', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocked.sharePreview.mockResolvedValue(preview())
    mocked.copySharePreview.mockResolvedValue(undefined)
    mocked.saveSharePreview.mockResolvedValue({ cancelled: false, path: '/x/Why did calls go up.html', file_name: 'Why did calls go up.html' })
    mocked.copyText.mockResolvedValue(undefined)
  })
  it('shows the short version and copies and saves only what is on the sheet', async () => {
    renderDialog()
    expect(await screen.findByRole('heading', { level: 1, name: 'INV-0001: Margin' })).toBeInTheDocument()
    expect(mocked.sharePreview).toHaveBeenCalledWith('INV-0001', { mode: 'brief' })
    expect(screen.getByText(SHARE_READY)).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Copy' }))
    expect(mocked.copySharePreview).toHaveBeenCalledWith('reviewed-original')
    expect(await screen.findByText(ANSWER_COPIED)).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Save as a file...' }))
    await waitFor(() => expect(mocked.saveSharePreview).toHaveBeenCalledWith('reviewed-original'))
    expect(await screen.findByText(ANSWER_SAVED('Why did calls go up.html'))).toBeInTheDocument()
  })
  it('loads the answer again when it changed before Copy, and sends nothing old', async () => {
    mocked.copySharePreview.mockRejectedValueOnce(JSON.stringify({ kind: 'conflict', message: 'conflict: This is no longer the latest answer.' }))
    renderDialog()
    await screen.findByText(SHARE_READY)
    mocked.sharePreview.mockResolvedValue(preview({ snapshot_id: 'newer' }))
    await userEvent.click(screen.getByRole('button', { name: 'Copy' }))
    expect(await screen.findByText(SHARE_CHANGED)).toBeInTheDocument()
    await waitFor(() => expect(mocked.sharePreview).toHaveBeenCalledTimes(2))
    expect(screen.queryByText(/no longer the latest/)).toBeNull()
    await userEvent.click(await screen.findByRole('button', { name: 'Copy' }))
    await waitFor(() => expect(mocked.copySharePreview).toHaveBeenLastCalledWith('newer'))
  })
  it('names the file the save returned in the toast', async () => {
    mocked.saveSharePreview.mockResolvedValue({ cancelled: false, path: 'C:\\Users\\Ada\\Documents\\Why did calls go up (everything).html', file_name: 'Why did calls go up (everything).html' })
    renderDialog()
    await screen.findByRole('heading', { level: 1 })
    await userEvent.click(screen.getByRole('button', { name: 'Save as a file...' }))
    expect(await screen.findByText('Saved the answer as Why did calls go up (everything).html.')).toBeInTheDocument()
    expect(screen.queryByText(/Documents/)).not.toBeInTheDocument()
  })
  it('disables Copy and Save while a load is in flight; a failed load shows a plain message and Try again loads again', async () => {
    let reject!: (error: unknown) => void
    mocked.sharePreview.mockReturnValueOnce(new Promise((_resolve, no) => { reject = no }))
    renderDialog()
    expect(screen.getByText(SHARE_LOADING)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Copy' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Save as a file...' })).toBeDisabled()
    reject(new Error('Cannot read case'))
    const alert = await screen.findByRole('alert')
    expect(alert).toHaveTextContent(WENT_WRONG_COPY)
    expect(alert).not.toHaveTextContent('Cannot read case')
    expect(screen.getByRole('button', { name: 'Copy' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Save as a file...' })).toBeDisabled()
    expect(mocked.copySharePreview).not.toHaveBeenCalled()
    await userEvent.click(within(alert).getByRole('button', { name: 'Try again' }))
    expect(mocked.sharePreview).toHaveBeenCalledTimes(2)
    expect(mocked.sharePreview).toHaveBeenLastCalledWith('INV-0001', { mode: 'brief' })
    expect(await screen.findByRole('heading', { level: 1, name: 'INV-0001: Margin' })).toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Copy' })).toBeEnabled()
  })
  it('offers Short version and Everything under How much to share, and Everything loads the whole report', async () => {
    renderDialog()
    await screen.findByRole('heading', { level: 1 })
    const group = screen.getByRole('radiogroup', { name: 'How much to share' })
    expect(within(group).getAllByRole('radio').map((r) => r.textContent)).toEqual(['Short version', 'Everything'])
    expect(within(group).getByRole('radio', { name: 'Short version' })).toBeChecked()
    mocked.sharePreview.mockResolvedValue(preview({ mode: 'report', snapshot_id: 'reviewed-report' }))
    await userEvent.click(within(group).getByRole('radio', { name: 'Everything' }))
    expect(mocked.sharePreview).toHaveBeenLastCalledWith('INV-0001', { mode: 'report' })
    await waitFor(() => expect(screen.getByRole('button', { name: 'Copy' })).toBeEnabled())
    expect(within(group).getByRole('radio', { name: 'Everything' })).toBeChecked()
    expect(screen.queryByText('Choose what to share')).not.toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Copy' }))
    expect(mocked.copySharePreview).toHaveBeenLastCalledWith('reviewed-report')
  })
  it('keeps Choose what to share closed until asked, and has no refresh button and no block about another app', async () => {
    renderDialog()
    await screen.findByRole('heading', { level: 1 })
    expect(choices().closest('details')).toHaveProperty('open', false)
    expect(screen.queryByRole('button', { name: /refresh|update|reload/i })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /example/i })).not.toBeInTheDocument()
    expect(screen.queryByText(/another app/i)).not.toBeInTheDocument()
    expect(screen.queryByRole('combobox')).not.toBeInTheDocument()
    expect(mocked.copyText).not.toHaveBeenCalled()
  })
  it('keeps findings where sources disagree ticked, and a changed choice loads its own version before Copy works', async () => {
    renderDialog()
    await screen.findByRole('heading', { level: 1 })
    await userEvent.click(choices())
    expect(screen.getByRole('checkbox', { name: /Material cost went down/ })).toBeDisabled()
    let resolve!: (p: SharePreview) => void
    mocked.sharePreview.mockReturnValueOnce(new Promise((yes) => { resolve = yes }))
    await userEvent.click(screen.getByRole('checkbox', { name: /Selected cost change/ }))
    expect(mocked.sharePreview).toHaveBeenCalledTimes(2)
    expect(mocked.sharePreview).toHaveBeenLastCalledWith('INV-0001', { mode: 'brief', claim_refs: ['C-002'], hypothesis_refs: ['H-003'], question_refs: [], ruled_out_refs: [] })
    expect(screen.getByText(SHARE_LOADING)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Copy' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Save as a file...' })).toBeDisabled()
    resolve(preview({ snapshot_id: 'reviewed-selected', candidates: preview().candidates.map(c => c.ref === 'C-001' ? { ...c, selected: false } : c) }))
    await waitFor(() => expect(screen.getByRole('button', { name: 'Copy' })).toBeEnabled())
    expect(screen.getByRole('checkbox', { name: /Selected cost change/ })).not.toBeChecked()
    await userEvent.click(screen.getByRole('button', { name: 'Copy' }))
    expect(mocked.copySharePreview).toHaveBeenLastCalledWith('reviewed-selected')
  })
  it('offers findings not tested yet as their own group, and ticking one loads again with it', async () => {
    renderDialog()
    await screen.findByRole('heading', { level: 1 })
    await userEvent.click(choices())
    const group = screen.getByRole('group', { name: 'Findings not tested yet' })
    const backed = within(group).getByRole('checkbox', { name: /Policy change explains it/ })
    const open = within(group).getByRole('checkbox', { name: /Travel is growing/ })
    expect(backed).toBeChecked()
    expect(open).not.toBeChecked()
    expect(within(group).getByText('Not tested yet; Not checked')).toBeInTheDocument()
    mocked.sharePreview.mockResolvedValue(preview({ snapshot_id: 'reviewed-travel', candidates: preview().candidates.map(c => c.ref === 'H-004' ? { ...c, selected: true } : c) }))
    await userEvent.click(open)
    expect(mocked.sharePreview).toHaveBeenLastCalledWith('INV-0001', { mode: 'brief', claim_refs: ['C-001', 'C-002'], hypothesis_refs: ['H-003', 'H-004'], question_refs: [], ruled_out_refs: [] })
    await waitFor(() => expect(screen.getByRole('button', { name: 'Copy' })).toBeEnabled())
    expect(open).toBeChecked()
    await userEvent.click(screen.getByRole('button', { name: 'Copy' }))
    expect(mocked.copySharePreview).toHaveBeenLastCalledWith('reviewed-travel')
  })
  it('lists the groups in the order the shared answer prints them', async () => {
    mocked.sharePreview.mockResolvedValue(preview({ candidates: [
      { ref: 'C-009', text: 'Ruled out early', label: 'Ruled out', group: 'ruled_out', required: false, selected: false },
      { ref: 'Q-001', text: 'Did refunds go out?', label: 'Not answered', group: 'questions', required: false, selected: false },
      ...preview().candidates,
    ] }))
    renderDialog()
    await screen.findByRole('heading', { level: 1 })
    expect(Array.from(document.querySelectorAll('.share-summary-choices legend'), l => l.textContent)).toEqual(['Findings', 'Findings not tested yet', 'Follow-up questions', 'Ruled out'])
  })
  it('ignores a late response for the previous question', async () => {
    let resolveOld!: (p: SharePreview) => void
    mocked.sharePreview.mockImplementationOnce(() => new Promise(resolve => { resolveOld = resolve }))
    const view = renderDialog()
    mocked.sharePreview.mockResolvedValue(preview({ investigation_ref: 'INV-0002', snapshot_id: 'case-two', html: '<h1>Case two</h1>' }))
    view.rerender(<ToastProvider><ShareSummaryDialog invRef="INV-0002" onClose={() => {}} /></ToastProvider>)
    await screen.findByRole('heading', { level: 1, name: 'Case two' })
    resolveOld(preview())
    await waitFor(() => expect(screen.queryByRole('heading', { name: 'INV-0001: Margin' })).not.toBeInTheDocument())
    await userEvent.click(screen.getByRole('button', { name: 'Copy' }))
    expect(mocked.copySharePreview).toHaveBeenCalledWith('case-two')
  })
  it('does not announce a cancelled save', async () => {
    mocked.saveSharePreview.mockResolvedValue({ cancelled: true, path: '', file_name: '' })
    renderDialog()
    await screen.findByRole('heading', { level: 1 })
    await userEvent.click(screen.getByRole('button', { name: 'Save as a file...' }))
    await waitFor(() => expect(mocked.saveSharePreview).toHaveBeenCalled())
    await waitFor(() => expect(screen.getByRole('button', { name: 'Copy' })).toBeEnabled())
    expect(screen.queryByText(/Saved the answer/)).not.toBeInTheDocument()
  })
})
