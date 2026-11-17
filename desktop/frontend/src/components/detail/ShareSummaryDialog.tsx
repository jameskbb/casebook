// Review one deterministic snapshot before copying or saving its pinned bytes.
import { useEffect, useRef, useState } from 'react'
import { ClipboardCopy, Download, RefreshCw } from 'lucide-react'
import { api } from '../../api/api'
import { errorCopy, toApiError } from '../../api/errors'
import type { ShareCandidate, ShareOptions, SharePreview } from '../../api/types'
import type { NoteNode } from '../../lib/noteTree'
import { parseReportHtml } from '../../lib/reportView'
import { Button } from '../ui/Button'
import { Dialog } from '../ui/Dialog'
import { useToast } from '../ui/Toaster'
import { NotePaper } from './NotePaper'
import './ShareSummaryDialog.css'

export const SUMMARY_COPIED = 'Copied the summary. Paste it into an email or a document.'
export const SUMMARY_SAVED = (name: string) => `Saved the summary as ${name}.`
export const EXAMPLE_COPIED = 'Copied the example request. Paste it into your AI client.'
export const ELSEWHERE_SUMMARY = 'Use this summary in another app'
export const ELSEWHERE_COPY = 'Use this summary in the tools you already use. Copy it into a note or document, save it as a file, or ask your AI to save it to an app it is already connected to. The connected app has to support creating or updating notes or documents through your AI client.'
export const ELSEWHERE_NOTE = 'Your AI asks Casebook for the summary, then uses its own connection to that app; Casebook does not send anything itself. The copy your AI fetches is a separate snapshot with the default selection of findings, so it matches this preview only if the case has not changed and you kept that selection.'

/** The request to paste into an AI client, for the chosen format and the open case. */
export function exampleRequest(mode: ShareOptions['mode'], invRef: string, title: string): string {
  const what = mode === 'report' ? 'the complete report' : 'the leadership brief'
  return `Save ${what} for case ${invRef} in my connected notes app as a new note titled "${title}". Keep its verification labels, evidence references and caveats.`
}

type SelectionKey = 'claim_refs' | 'hypothesis_refs' | 'question_refs' | 'ruled_out_refs'

// The selection groups in the order the brief prints them, with the option
// each one fills and the legend the sender reads.
const GROUPS: { group: ShareCandidate['group']; key: SelectionKey; legend: string }[] = [
  { group: 'claims', key: 'claim_refs', legend: 'Claims' },
  { group: 'hypotheses', key: 'hypothesis_refs', legend: 'Explanations still open or supported' },
  { group: 'questions', key: 'question_refs', legend: 'Questions' },
  { group: 'ruled_out', key: 'ruled_out_refs', legend: 'Ruled out' },
]
const keyOf = (group: ShareCandidate['group']): SelectionKey => GROUPS.find(g => g.group === group)?.key ?? 'claim_refs'

function selection(preview: SharePreview): ShareOptions {
  const refs = (group: ShareCandidate['group']) => preview.candidates.filter(c => c.group === group && c.selected).map(c => c.ref)
  return { mode: preview.mode, claim_refs: refs('claims'), hypothesis_refs: refs('hypotheses'), question_refs: refs('questions'), ruled_out_refs: refs('ruled_out') }
}

export function ShareSummaryDialog({ invRef, title, onClose }: { invRef: string; title: string; onClose: () => void }) {
  const toast = useToast()
  const [preview, setPreview] = useState<SharePreview | null>(null)
  const [nodes, setNodes] = useState<NoteNode[] | null>(null)
  const [draft, setDraft] = useState<ShareOptions>({ mode: 'brief' })
  const [dirty, setDirty] = useState(false)
  const [busy, setBusy] = useState(true)
  const [sharing, setSharing] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [copied, setCopied] = useState(false)
  const request = useRef(0)

  const load = async (options: ShareOptions) => {
    const current = ++request.current
    setBusy(true)
    setError(null)
    setCopied(false)
    try {
      const p = await api.sharePreview(invRef, options)
      if (current !== request.current) return
      setPreview(p)
      setNodes(parseReportHtml(p.html))
      setDraft(selection(p))
      setDirty(false)
    } catch (e) {
      if (current === request.current) setError(errorCopy(toApiError(e)).message)
    } finally {
      if (current === request.current) setBusy(false)
    }
  }

  useEffect(() => {
    setPreview(null)
    setNodes(null)
    setDraft({ mode: 'brief' })
    setDirty(false)
    void load({ mode: 'brief' })
    return () => { request.current += 1 }
    // Changing cases always starts with that case's default selection.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [invRef])

  const toggle = (c: ShareCandidate) => {
    const key = keyOf(c.group)
    setDraft(prev => {
      const selected = prev[key] ?? []
      return { ...prev, [key]: selected.includes(c.ref) ? selected.filter(ref => ref !== c.ref) : [...selected, c.ref] }
    })
    setDirty(true)
    setCopied(false)
  }
  const copy = async () => {
    if (!preview || busy || dirty || error || sharing) return
    setSharing(true)
    try {
      await api.copySharePreview(preview.snapshot_id)
      setCopied(true)
      toast.success(SUMMARY_COPIED)
    } catch (e) { toast.error(errorCopy(toApiError(e)).message) }
    finally { setSharing(false) }
  }
  const save = async () => {
    if (!preview || busy || dirty || error || sharing) return
    setSharing(true)
    try {
      const res = await api.saveSharePreview(preview.snapshot_id)
      if (!res.cancelled) toast.success(SUMMARY_SAVED(res.file_name))
    } catch (e) { toast.error(errorCopy(toApiError(e)).message) }
    finally { setSharing(false) }
  }
  const disabled = !preview || busy || dirty || !!error || sharing
  const example = exampleRequest(draft.mode, invRef, title)
  // Copies text only; the person pastes it into their AI client.
  const copyExample = async () => {
    try {
      await api.copyText(example)
      toast.success(EXAMPLE_COPIED)
    } catch (e) { toast.error(errorCopy(toApiError(e)).message) }
  }

  return (
    <Dialog title="Share summary" size="form" className="share-summary-dialog" onClose={onClose} busy={sharing}
      footer={<>
        <Button onClick={onClose} disabled={sharing}>Close</Button>
        <Button icon={Download} disabled={disabled} onClick={() => void save()}>Save as file...</Button>
        <Button variant="primary" icon={ClipboardCopy} disabled={disabled} onClick={() => void copy()}>{copied ? 'Copied' : 'Copy'}</Button>
      </>}
    >
      <p className="share-summary-lede">Share the answer with the evidence behind it, who checked what, and what is still in doubt. A leadership brief uses the findings you choose; the complete report includes everything and the history.</p>
      <div className="share-summary-toolbar">
        <label>Format <select value={draft.mode} disabled={busy || sharing} onChange={e => { const mode = e.target.value as ShareOptions['mode']; setDraft({ mode }); void load({ mode }) }}>
          <option value="brief">Leadership brief</option>
          <option value="report">Complete report</option>
        </select></label>
        <Button icon={RefreshCw} disabled={busy || sharing} onClick={() => void load(draft)}>{dirty ? 'Update preview' : 'Refresh preview'}</Button>
      </div>
      {preview && draft.mode === 'brief' && preview.candidates.length > 0 && <details className="share-summary-selection">
        <summary>Choose findings and questions</summary>
        <p>Disputed findings, contradicting evidence and the claims behind the answer are always included. Choose claims, explanations, questions and ruled-out items here.</p>
        <div className="share-summary-choices">
          {GROUPS.map(({ group, key, legend }) => <fieldset key={group}>
            <legend>{legend}</legend>
            {preview.candidates.filter(c => c.group === group).map(c =>
              <label key={c.ref} className="share-summary-choice"><input type="checkbox" checked={draft[key]?.includes(c.ref) ?? false} disabled={c.required || busy || sharing} onChange={() => toggle(c)} /><span><strong>{c.ref}</strong> {c.text}<small>{c.label}{c.required ? '; always included' : ''}</small></span></label>
            )}
          </fieldset>)}
        </div>
      </details>}
      <p className="share-summary-snapshot" role="status">{busy ? 'Preparing preview...' : dirty ? 'Selection changed. Update the preview before sharing.' : preview ? 'This preview stays unchanged until you refresh. Copy and Save use exactly what is shown.' : 'Preview unavailable. Refresh to try again.'}</p>
      {error ? <p role="alert" className="share-summary-error">{error}</p> : <NotePaper nodes={nodes} text={preview?.markdown ?? ''} label="Summary" />}
      <details className="share-summary-elsewhere">
        <summary>{ELSEWHERE_SUMMARY}</summary>
        <p>{ELSEWHERE_COPY}</p>
        <div className="share-summary-example">
          <blockquote className="share-summary-example-text">{example}</blockquote>
          <Button icon={ClipboardCopy} onClick={() => void copyExample()}>Copy example request</Button>
        </div>
        <p className="share-summary-elsewhere-note">{ELSEWHERE_NOTE}</p>
      </details>
    </Dialog>
  )
}
