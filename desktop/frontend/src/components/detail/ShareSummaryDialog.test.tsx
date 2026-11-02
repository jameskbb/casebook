import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { api } from '../../api/api'
import type { SharePreview } from '../../api/types'
import { ToastProvider } from '../ui/Toaster'
import { ELSEWHERE_COPY, ELSEWHERE_NOTE, EXAMPLE_COPIED, exampleRequest, ShareSummaryDialog } from './ShareSummaryDialog'

vi.mock('../../api/api', () => ({ api: { sharePreview: vi.fn(), copySharePreview: vi.fn(), saveSharePreview: vi.fn(), copyText: vi.fn() } }))
const mocked = vi.mocked(api)
const preview = (patch: Partial<SharePreview> = {}): SharePreview => ({
  markdown: '# INV-0001: Margin\n\n## Question\n\nWhy?\n',
  html: '<h1>INV-0001: Margin</h1>\n<h2>Question</h2>\n<p>Why?</p>\n',
  snapshot_id: 'reviewed-original', investigation_ref: 'INV-0001', mode: 'brief',
  candidates: [
    { ref: 'C-001', text: 'Selected cost change', label: 'Current; High confidence; AI generated', group: 'claims', required: false, selected: true },
    { ref: 'C-002', text: 'Material dispute', label: 'Disputed; Low confidence', group: 'claims', required: true, selected: true },
    { ref: 'H-003', text: 'Policy change explains it', label: 'Supported; High confidence; AI generated', group: 'hypotheses', required: false, selected: true },
    { ref: 'H-004', text: 'Travel is growing', label: 'Open; Medium confidence; AI generated', group: 'hypotheses', required: false, selected: false },
  ], omitted: { claims: 0, hypotheses: 1, questions: 1, evidence: 2, claim_refs: [], hypothesis_refs: ['H-004'], question_refs: ['Q-001'], evidence_refs: ['E-002', 'E-003'] }, ...patch,
})
function renderDialog(invRef = 'INV-0001') {
  return render(<ToastProvider><ShareSummaryDialog invRef={invRef} title="Northeast margin review" onClose={() => {}} /></ToastProvider>)
}

describe('ShareSummaryDialog', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocked.sharePreview.mockResolvedValue(preview())
    mocked.copySharePreview.mockResolvedValue(undefined)
    mocked.saveSharePreview.mockResolvedValue({ cancelled: false, path: '/x/summary.md', file_name: 'summary.md' })
    mocked.copyText.mockResolvedValue(undefined)
  })
  it('previews a leadership brief and shares only its reviewed snapshot', async () => {
    renderDialog()
    expect(await screen.findByRole('heading', { level: 1, name: 'INV-0001: Margin' })).toBeInTheDocument()
    expect(mocked.sharePreview).toHaveBeenCalledWith('INV-0001', { mode: 'brief' })
    expect(screen.getByRole('status')).toHaveTextContent('This preview stays unchanged until you refresh')
    await userEvent.click(screen.getByRole('button', { name: 'Copy' }))
    expect(mocked.copySharePreview).toHaveBeenCalledWith('reviewed-original')
    await userEvent.click(screen.getByRole('button', { name: 'Save as file...' }))
    await waitFor(() => expect(mocked.saveSharePreview).toHaveBeenCalledWith('reviewed-original'))
    expect(await screen.findByText('Saved the summary as summary.md.')).toBeInTheDocument()
  })
  it('disables sharing until a pending preview succeeds, and on a preview error', async () => {
    let reject!: (error: unknown) => void
    mocked.sharePreview.mockReturnValue(new Promise((_resolve, no) => { reject = no }))
    renderDialog()
    expect(screen.getByRole('button', { name: 'Copy' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Save as file...' })).toBeDisabled()
    reject(new Error('Cannot read case'))
    await screen.findByRole('alert')
    expect(screen.getByRole('button', { name: 'Copy' })).toBeDisabled()
    expect(mocked.copySharePreview).not.toHaveBeenCalled()
  })
  it('keeps mandatory caveats selected and requires reviewing selection changes', async () => {
    renderDialog()
    await screen.findByRole('heading', { level: 1 })
    await userEvent.click(screen.getByText('Choose findings and questions'))
    expect(screen.getByRole('checkbox', { name: /Material dispute/ })).toBeDisabled()
    await userEvent.click(screen.getByRole('checkbox', { name: /Selected cost change/ }))
    expect(screen.getByRole('button', { name: 'Copy' })).toBeDisabled()
    expect(screen.getByRole('status')).toHaveTextContent('Update the preview before sharing')
    mocked.sharePreview.mockResolvedValue(preview({ snapshot_id: 'reviewed-selected', candidates: preview().candidates.map(c => c.ref === 'C-001' ? { ...c, selected: false } : c) }))
    await userEvent.click(screen.getByRole('button', { name: 'Update preview' }))
    await waitFor(() => expect(mocked.sharePreview).toHaveBeenLastCalledWith('INV-0001', { mode: 'brief', claim_refs: ['C-002'], hypothesis_refs: ['H-003'], question_refs: [], ruled_out_refs: [] }))
    await waitFor(() => expect(screen.getByRole('button', { name: 'Copy' })).toBeEnabled())
    await userEvent.click(screen.getByRole('button', { name: 'Copy' }))
    expect(mocked.copySharePreview).toHaveBeenLastCalledWith('reviewed-selected')
  })
  it('offers open or supported explanations as their own group and requires reviewing changes to it', async () => {
    renderDialog()
    await screen.findByRole('heading', { level: 1 })
    await userEvent.click(screen.getByText('Choose findings and questions'))
    const group = screen.getByRole('group', { name: 'Explanations still open or supported' })
    const supported = within(group).getByRole('checkbox', { name: /Policy change explains it/ })
    const open = within(group).getByRole('checkbox', { name: /Travel is growing/ })
    expect(supported).toBeChecked()
    expect(open).not.toBeChecked()
    expect(within(group).getByText('Open; Medium confidence; AI generated')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Copy' })).toBeEnabled()
    await userEvent.click(open)
    expect(open).toBeChecked()
    expect(screen.getByRole('button', { name: 'Copy' })).toBeDisabled()
    expect(screen.getByRole('status')).toHaveTextContent('Selection changed. Update the preview before sharing.')
    mocked.sharePreview.mockResolvedValue(preview({ snapshot_id: 'reviewed-travel', candidates: preview().candidates.map(c => c.ref === 'H-004' ? { ...c, selected: true } : c) }))
    await userEvent.click(screen.getByRole('button', { name: 'Update preview' }))
    await waitFor(() => expect(mocked.sharePreview).toHaveBeenLastCalledWith('INV-0001', { mode: 'brief', claim_refs: ['C-001', 'C-002'], hypothesis_refs: ['H-003', 'H-004'], question_refs: [], ruled_out_refs: [] }))
    await waitFor(() => expect(screen.getByRole('button', { name: 'Copy' })).toBeEnabled())
    await userEvent.click(screen.getByRole('button', { name: 'Copy' }))
    expect(mocked.copySharePreview).toHaveBeenLastCalledWith('reviewed-travel')
  })
  it('lists the groups in the order the brief prints them', async () => {
    renderDialog()
    await screen.findByRole('heading', { level: 1 })
    await userEvent.click(screen.getByText('Choose findings and questions'))
    expect(Array.from(document.querySelectorAll('.share-summary-choices legend'), l => l.textContent)).toEqual(['Claims', 'Explanations still open or supported', 'Questions', 'Ruled out'])
  })
  it('keeps the complete report available and refreshes explicitly', async () => {
    renderDialog()
    await screen.findByRole('heading', { level: 1 })
    mocked.sharePreview.mockResolvedValue(preview({ mode: 'report', snapshot_id: 'reviewed-report' }))
    await userEvent.selectOptions(screen.getByRole('combobox', { name: 'Format' }), 'report')
    await waitFor(() => expect(mocked.sharePreview).toHaveBeenLastCalledWith('INV-0001', { mode: 'report' }))
    await waitFor(() => expect(screen.getByRole('button', { name: 'Refresh preview' })).toBeEnabled())
    expect(screen.queryByText('Choose findings and questions')).not.toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Refresh preview' }))
    await waitFor(() => expect(mocked.sharePreview).toHaveBeenCalledTimes(3))
  })
  it('ignores a late response for the previous case', async () => {
    let resolveOld!: (p: SharePreview) => void
    mocked.sharePreview.mockImplementationOnce(() => new Promise(resolve => { resolveOld = resolve }))
    const view = renderDialog()
    mocked.sharePreview.mockResolvedValue(preview({ investigation_ref: 'INV-0002', snapshot_id: 'case-two', html: '<h1>Case two</h1>' }))
    view.rerender(<ToastProvider><ShareSummaryDialog invRef="INV-0002" title="Northeast margin review" onClose={() => {}} /></ToastProvider>)
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
    await userEvent.click(screen.getByRole('button', { name: 'Save as file...' }))
    await waitFor(() => expect(mocked.saveSharePreview).toHaveBeenCalled())
    expect(screen.queryByText(/Saved the summary/)).not.toBeInTheDocument()
  })
  it('builds the example request from the case and the chosen format', async () => {
    renderDialog('INV-0002')
    await screen.findByRole('heading', { level: 1 })
    const brief = 'Save the leadership brief for case INV-0002 in my connected notes app as a new note titled "Northeast margin review". Keep its verification labels, evidence references and caveats.'
    expect(exampleRequest('brief', 'INV-0002', 'Northeast margin review')).toBe(brief)
    expect(screen.getByText(brief)).toBeInTheDocument()
    expect(screen.getByText(ELSEWHERE_COPY)).toBeInTheDocument()
    expect(screen.getByText(ELSEWHERE_NOTE)).toBeInTheDocument()
    mocked.sharePreview.mockResolvedValue(preview({ mode: 'report', snapshot_id: 'reviewed-report' }))
    await userEvent.selectOptions(screen.getByRole('combobox', { name: 'Format' }), 'report')
    expect(await screen.findByText('Save the complete report for case INV-0002 in my connected notes app as a new note titled "Northeast margin review". Keep its verification labels, evidence references and caveats.')).toBeInTheDocument()
  })
  it('copies the exact example request as text and sends nothing else', async () => {
    renderDialog('INV-0002')
    await screen.findByRole('heading', { level: 1 })
    await userEvent.click(screen.getByRole('button', { name: 'Copy example request' }))
    expect(mocked.copyText).toHaveBeenCalledWith(exampleRequest('brief', 'INV-0002', 'Northeast margin review'))
    expect(await screen.findByText(EXAMPLE_COPIED)).toBeInTheDocument()
    expect(mocked.copySharePreview).not.toHaveBeenCalled()
  })
  it('shows an error when copying the example request fails', async () => {
    mocked.copyText.mockRejectedValue(new Error('clipboard unavailable'))
    renderDialog()
    await screen.findByRole('heading', { level: 1 })
    await userEvent.click(screen.getByRole('button', { name: 'Copy example request' }))
    await waitFor(() => expect(screen.queryByText(EXAMPLE_COPIED)).not.toBeInTheDocument())
    expect(mocked.copyText).toHaveBeenCalled()
  })
})
