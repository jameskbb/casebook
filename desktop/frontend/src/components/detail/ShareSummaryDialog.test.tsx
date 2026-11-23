import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
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
  // The dialog sends only the mode; whatever candidates come back are ignored.
  candidates: [], omitted: { claims: 0, hypotheses: 0, questions: 0, evidence: 0, claim_refs: [], hypothesis_refs: [], question_refs: [], evidence_refs: [] }, ...patch,
})
function renderDialog(invRef = 'INV-0001') {
  return render(<ToastProvider><ShareSummaryDialog invRef={invRef} onClose={() => {}} /></ToastProvider>)
}

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
  it('has no Choose what to share, no ticks, no refresh button and no block about another app', async () => {
    renderDialog()
    await screen.findByRole('heading', { level: 1 })
    expect(screen.queryByText('Choose what to share')).not.toBeInTheDocument()
    expect(screen.queryByRole('checkbox')).not.toBeInTheDocument()
    expect(document.querySelector('details')).toBeNull()
    expect(screen.queryByRole('button', { name: /refresh|update|reload/i })).not.toBeInTheDocument()
    expect(screen.queryByText(/another app/i)).not.toBeInTheDocument()
    expect(mocked.copyText).not.toHaveBeenCalled()
  })
  describe('Copy as rich text', () => {
    const write = vi.fn()
    class FakeClipboardItem { constructor(public items: Record<string, Blob>) {} }
    beforeEach(() => {
      write.mockReset().mockResolvedValue(undefined)
      vi.stubGlobal('ClipboardItem', FakeClipboardItem)
      Object.defineProperty(navigator, 'clipboard', { value: { write }, configurable: true })
      mocked.sharePreview.mockResolvedValue(preview({ text: 'INV-0001: Margin\n\nQuestion\n\nWhy?\n' }))
    })
    afterEach(() => { vi.unstubAllGlobals() })
    it('puts the html and the plain text on the clipboard in one item, without calling Casebook', async () => {
      renderDialog()
      await screen.findByText(SHARE_READY)
      await userEvent.click(screen.getByRole('button', { name: 'Copy' }))
      expect(await screen.findByText(ANSWER_COPIED)).toBeInTheDocument()
      expect(write).toHaveBeenCalledTimes(1)
      const [[item]] = write.mock.calls[0] as [[FakeClipboardItem]]
      expect(Object.keys(item.items).sort()).toEqual(['text/html', 'text/plain'])
      expect(await item.items['text/html'].text()).toBe(preview().html)
      expect(await item.items['text/plain'].text()).toBe('INV-0001: Margin\n\nQuestion\n\nWhy?\n')
      expect(mocked.copySharePreview).not.toHaveBeenCalled()
    })
    it('falls back to the plain copy when the window refuses rich text', async () => {
      write.mockRejectedValue(new Error('denied'))
      renderDialog()
      await screen.findByText(SHARE_READY)
      await userEvent.click(screen.getByRole('button', { name: 'Copy' }))
      expect(await screen.findByText(ANSWER_COPIED)).toBeInTheDocument()
      expect(mocked.copySharePreview).toHaveBeenCalledWith('reviewed-original')
    })
    it('copies plain text through Casebook when the preview has no plain version', async () => {
      mocked.sharePreview.mockResolvedValue(preview())
      renderDialog()
      await screen.findByText(SHARE_READY)
      await userEvent.click(screen.getByRole('button', { name: 'Copy' }))
      expect(await screen.findByText(ANSWER_COPIED)).toBeInTheDocument()
      expect(write).not.toHaveBeenCalled()
      expect(mocked.copySharePreview).toHaveBeenCalledWith('reviewed-original')
    })
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
