// Check the next source: the one way a person checks. One source at a time
// in one window: its name, what it shows, where to find it, the exact words,
// and the finding it backs or goes against. "I checked this" saves the check
// and moves to the next source by itself; "Skip" leaves it as it was. At the
// end it says so, and offers the check of the answer when there is one the
// person has not checked.
//
// The window reads the sources from the latest detail on every render, so a
// source an AI adds meanwhile joins the end and one checked elsewhere drops
// out. Focus stays on the main button from source to source, so Enter moves
// through them; Escape closes and focus returns to what opened the window.
import { useEffect, useRef, useState } from 'react'
import { api } from '../../api/api'
import { plainError, toApiError } from '../../api/errors'
import type { Evidence, InvestigationDetail } from '../../api/types'
import { LIMITS, nearLimit } from '../../lib/bytes'
import { splitFieldErrors } from '../../lib/formErrors'
import { relationTags } from '../../lib/labels'
import { countOf } from '../../lib/plural'
import { isReplaced, isRuledOut, sourcesToCheck, usedBy } from '../../lib/records'
import { useWrite } from '../../state/useWrite'
import { VerificationBadge } from '../record/VerificationBadge'
import { BreakablePath } from '../ui/BreakablePath'
import { Button } from '../ui/Button'
import { Dialog } from '../ui/Dialog'
import { TextArea } from '../ui/TextArea'
import { answerChecked, answerCheckInput, CHECKED_ANSWER_ACTION, WHAT_DID_YOU_CHECK } from './ConclusionPanel'
import './CheckSources.css'

export const CHECK_SOURCES_TITLE = 'Check the sources'
export const SOURCE_CHECKED_ACTION = 'I checked this'
export const SKIP_ACTION = 'Skip'
export const ALL_SOURCES_CHECKED = 'You have checked every source.'
export const GO_BACK_TO_SKIPPED = 'Go back to the skipped sources'
export const ANSWER_CHECKED_COPY = 'You checked this answer.'

/** "Source 3 of 7". */
export const progressText = (position: number, total: number) => `Source ${position} of ${total}`

/** "You skipped 2 sources. They are still not checked." */
export const skippedText = (n: number) => `You skipped ${countOf(n, 'source', 'sources')}. ${n === 1 ? 'It is' : 'They are'} still not checked.`

/**
 * Where the walk stands: the source to show, or none at the end. Skipped
 * sources are passed over; "left" counts the ones still not checked.
 */
export function checkStep(evidence: Evidence[], skipped: string[], checked: number) {
  const open = sourcesToCheck(evidence)
  const queue = open.filter((e) => !skipped.includes(e.ref))
  const done = checked + (open.length - queue.length)
  return { current: queue[0] as Evidence | undefined, position: done + 1, total: done + queue.length, left: open.length }
}

export function CheckSources({ detail, onClose }: { detail: InvestigationDetail; onClose: () => void }) {
  const write = useWrite()
  const inv = detail.investigation
  const [skipped, setSkipped] = useState<string[]>([])
  const [checked, setChecked] = useState(0)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [basis, setBasis] = useState('')
  const [basisError, setBasisError] = useState<string | undefined>()
  const mainRef = useRef<HTMLButtonElement>(null)
  const answerRef = useRef<HTMLDivElement>(null)

  const { current, position, total, left } = checkStep(detail.evidence, skipped, checked)
  const askAnswer = !current && left === 0 && inv.conclusion.trim() !== '' && !answerChecked(inv)
  const stage = current ? current.ref : askAnswer ? 'answer' : 'end'

  // The main button changes at the end; keep the keyboard on it (or in the one field).
  useEffect(() => {
    if (stage === 'answer') answerRef.current?.querySelector('textarea')?.focus()
    else mainRef.current?.focus()
  }, [stage])

  const run = async (fn: () => Promise<unknown>, after: () => void) => {
    if (busy) return
    setBusy(true)
    setError('')
    setBasisError(undefined)
    try {
      const saved = await write(fn, { rethrowConflict: true })
      if (saved !== undefined) after()
    } catch (e) {
      const split = splitFieldErrors(e, ['review_basis'])
      setBasisError(split.fieldErrors.review_basis)
      setError(split.summary ?? '')
    } finally {
      setBusy(false)
    }
  }

  const check = (e: Evidence) => run(() => api.setEvidenceVerification(inv.ref, e.ref, 'human_verified'), () => setChecked((n) => n + 1))
  const checkAnswer = () => run(() => api.updateConclusion(inv.ref, answerCheckInput(inv, basis)), () => setBasis(''))

  let body
  let footer
  if (current) {
    body = <Source key={current.ref} evidence={current} detail={detail} progress={progressText(position, total)} />
    footer = (
      <>
        <Button variant="secondary" disabled={busy} onClick={() => setSkipped((s) => [...s, current.ref])}>{SKIP_ACTION}</Button>
        <Button variant="primary" ref={mainRef} aria-busy={busy || undefined} onClick={() => void check(current)}>{SOURCE_CHECKED_ACTION}</Button>
      </>
    )
  } else if (askAnswer) {
    body = (
      <div className="check-end" ref={answerRef}>
        <p className="check-end-heading" role="status">{ALL_SOURCES_CHECKED}</p>
        <p>Now read the answer. Does it say what the sources say?</p>
        <p className="check-answer">{inv.conclusion}</p>
        <TextArea label={WHAT_DID_YOU_CHECK} value={basis} onChange={setBasis} error={basisError} note={nearLimit(basis, LIMITS.conclusionRationale)} />
      </div>
    )
    footer = (
      <>
        <Button variant="secondary" disabled={busy} onClick={onClose}>Not now</Button>
        <Button variant="primary" ref={mainRef} busy={busy} busyLabel="Saving..." disabled={!basis.trim()} onClick={() => void checkAnswer()}>{CHECKED_ANSWER_ACTION}</Button>
      </>
    )
  } else {
    body = (
      <div className="check-end">
        <p className="check-end-heading" role="status">{left === 0 ? ALL_SOURCES_CHECKED : skippedText(left)}</p>
        {left === 0 && inv.conclusion.trim() !== '' && <p>{ANSWER_CHECKED_COPY}</p>}
        {left === 0 && inv.conclusion.trim() === '' && <p>Close this window and write the answer.</p>}
      </div>
    )
    footer = (
      <>
        {left > 0 && <Button variant="secondary" onClick={() => setSkipped([])}>{GO_BACK_TO_SKIPPED}</Button>}
        <Button variant="primary" ref={mainRef} onClick={onClose}>Done</Button>
      </>
    )
  }

  return (
    <Dialog title={CHECK_SOURCES_TITLE} className="check-sources" onClose={onClose} initialFocus={mainRef} dirty={basis.trim() !== ''} footer={footer}>
      {error && <p className="dialog-summary" role="alert">{error}</p>}
      {body}
    </Dialog>
  )
}

/** One source, laid out the same way every time so the buttons never move. */
function Source({ evidence: e, detail, progress }: { evidence: Evidence; detail: InvestigationDetail; progress: string }) {
  const invRef = detail.investigation.ref
  const [raw, setRaw] = useState<string | null>(null)
  const [rawError, setRawError] = useState('')
  // The exact words are what a person checks against, so they load without asking.
  useEffect(() => {
    if (!e.has_raw_detail) return
    let live = true
    api.getEvidence(invRef, e.ref, { include_raw_detail: true }).then(
      (full) => {
        if (live) setRaw(full.raw_detail ?? '')
      },
      (err) => {
        if (live) setRawError(plainError(toApiError(err)))
      },
    )
    return () => {
      live = false
    }
  }, [invRef, e.ref, e.has_raw_detail, e.updated_at])

  const records = new Map([...detail.claims, ...detail.hypotheses].map((r) => [r.ref, r] as const))
  const findings = usedBy(e, detail).filter((u) => {
    const r = records.get(u.ref)
    return r && !isReplaced(r) && !isRuledOut(r)
  })
  return (
    <div className="check-source">
      <p className="check-progress" role="status">{progress}</p>
      <div>
        <h3 className="check-source-title">{e.title}</h3>
        <VerificationBadge verification={e.verification} />
      </div>
      {e.summary && <p className="check-source-summary">{e.summary}</p>}
      <dl className="check-source-facts">
        {e.source_ref && (
          <div>
            <dt>Where to find it</dt>
            <dd><BreakablePath text={e.source_ref} /></dd>
          </div>
        )}
        {e.has_raw_detail && (
          <div>
            <dt>Exact words or numbers</dt>
            <dd>{raw !== null ? <pre className="raw-detail">{raw}</pre> : rawError || 'Loading the exact words...'}</dd>
          </div>
        )}
        <div>
          <dt>The finding it is used for</dt>
          {findings.length === 0 ? (
            <dd>No finding uses this source yet.</dd>
          ) : (
            findings.map((u) => (
              <dd key={u.ref} className="check-source-finding">
                <span className={`check-source-tag ${u.relation === 'contradicts' ? 'is-against' : ''}`}>{relationTags[u.relation]}:</span> {u.statement}
              </dd>
            ))
          )}
        </div>
      </dl>
    </div>
  )
}
