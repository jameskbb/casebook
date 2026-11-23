// Share the answer: the person reads exactly what Copy and Save will send.
// Short version or Everything; each loads its own version, so what is on the
// sheet is always what goes out. Copy puts the sheet on the clipboard as
// rich text and as plain text (lib/richCopy); where the window cannot do
// that, Casebook copies the plain text itself.
import { useEffect, useRef, useState } from 'react'
import { ClipboardCopy, Download } from 'lucide-react'
import { api } from '../../api/api'
import { errorCopy, toApiError } from '../../api/errors'
import type { ShareOptions, SharePreview } from '../../api/types'
import type { NoteNode } from '../../lib/noteTree'
import { parseReportHtml } from '../../lib/reportView'
import { copyRich } from '../../lib/richCopy'
import { Button } from '../ui/Button'
import { Dialog } from '../ui/Dialog'
import { SegmentedControl } from '../ui/SegmentedControl'
import { useToast } from '../ui/Toaster'
import { NotePaper } from './NotePaper'
import './ShareSummaryDialog.css'

export const ANSWER_COPIED = 'Copied the answer. Paste it into an email or a document.'
export const ANSWER_SAVED = (name: string) => `Saved the answer as ${name}.`
export const SHARE_LEDE = 'Share the answer with the findings and sources behind it. Read it before you send it.'
export const SHARE_READY = 'Copy and Save use what you see below.'
export const SHARE_CHANGED = 'The answer changed. This is how it reads now.'
export const SHARE_LOADING = 'Getting the answer ready...'

type Mode = ShareOptions['mode']

const MODES: { value: Mode; label: string }[] = [
  { value: 'brief', label: 'Short version' },
  { value: 'report', label: 'Everything' },
]

export function ShareSummaryDialog({ invRef, onClose }: { invRef: string; onClose: () => void }) {
  const toast = useToast()
  const [preview, setPreview] = useState<SharePreview | null>(null)
  const [nodes, setNodes] = useState<NoteNode[] | null>(null)
  const [mode, setMode] = useState<Mode>('brief')
  const [busy, setBusy] = useState(true)
  const [sharing, setSharing] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [copied, setCopied] = useState(false)
  const request = useRef(0)

  // Each version loads on its own; only the newest request lands.
  const load = async (next: Mode) => {
    const current = ++request.current
    setMode(next)
    setBusy(true)
    setError(null)
    setCopied(false)
    try {
      const p = await api.sharePreview(invRef, { mode: next })
      if (current !== request.current) return
      setPreview(p)
      setNodes(parseReportHtml(p.html))
    } catch (e) {
      if (current === request.current) setError(errorCopy(toApiError(e)).message)
    } finally {
      if (current === request.current) setBusy(false)
    }
  }

  useEffect(() => {
    setPreview(null)
    setNodes(null)
    void load('brief')
    return () => { request.current += 1 }
    // Another question always starts with the short version.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [invRef])

  // The answer changed since the sheet was loaded: show how it reads now
  // rather than send the older one.
  const failed = (e: unknown) => {
    const err = toApiError(e)
    if (err.kind !== 'conflict') return toast.error(errorCopy(err).message)
    toast.error(SHARE_CHANGED)
    void load(mode)
  }
  // Copy and Save send the version on the sheet, never one still loading.
  const disabled = !preview || busy || !!error || sharing
  const copy = async () => {
    if (!preview || disabled) return
    setSharing(true)
    try {
      // The sheet's own words, as rich and plain text; else Casebook copies the plain text.
      const rich = preview.text !== undefined && (await copyRich(preview.html, preview.text))
      if (!rich) await api.copySharePreview(preview.snapshot_id)
      setCopied(true)
      toast.success(ANSWER_COPIED)
    } catch (e) { failed(e) }
    finally { setSharing(false) }
  }
  const save = async () => {
    if (!preview || disabled) return
    setSharing(true)
    try {
      const res = await api.saveSharePreview(preview.snapshot_id)
      if (!res.cancelled) toast.success(ANSWER_SAVED(res.file_name))
    } catch (e) { failed(e) }
    finally { setSharing(false) }
  }

  return (
    <Dialog title="Share the answer" size="form" className="share-summary-dialog" onClose={onClose} busy={sharing}
      footer={<>
        <Button onClick={onClose} disabled={sharing}>Close</Button>
        <Button icon={Download} disabled={disabled} onClick={() => void save()}>Save as a file...</Button>
        <Button variant="primary" icon={ClipboardCopy} className="share-copy" disabled={disabled} onClick={() => void copy()}>{copied ? 'Copied' : 'Copy'}</Button>
      </>}
    >
      <p className="share-summary-lede">{SHARE_LEDE}</p>
      <SegmentedControl<Mode> label="How much to share" options={MODES} value={mode} disabled={sharing} onChange={(m) => void load(m)} />
      {error
        ? <p role="alert" className="share-summary-error">{error} <Button variant="ghost" onClick={() => void load(mode)}>Try again</Button></p>
        : <>
          <p className="share-summary-snapshot" role="status">{busy ? SHARE_LOADING : SHARE_READY}</p>
          <NotePaper nodes={nodes} text={preview?.text ?? preview?.markdown ?? ''} label="The answer" />
        </>}
    </Dialog>
  )
}
