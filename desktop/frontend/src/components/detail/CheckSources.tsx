// Check the next source: the one pass where a person does the checking, in
// one window and in this order. First each source they have not checked: its
// name, what it shows, where to find it, the exact words, and the finding it
// backs or goes against; "I checked this" saves the check and moves on by
// itself. Then each finding where sources disagree, with its sources both
// ways: It still holds, Rule this out or Skip. Last the answer, when there is
// one the person has not checked, so a decision never undoes a check made a
// minute before. The end says so, and offers Write the answer when there is
// none.
//
// The window reads the latest detail on every render, so a source an AI adds
// meanwhile joins the end and one checked elsewhere drops out. Focus stays on
// the main button from step to step, so Enter moves through them; Escape
// closes and the page puts focus back on the answer's buttons.
import { useEffect, useRef, useState } from 'react'
import { api } from '../../api/api'
import { ApiError, plainError, toApiError } from '../../api/errors'
import type { Evidence, InvestigationDetail } from '../../api/types'
import { LIMITS, nearLimit } from '../../lib/bytes'
import { splitFieldErrors } from '../../lib/formErrors'
import { relationTags } from '../../lib/labels'
import { countOf, plural } from '../../lib/plural'
import { evidenceByRef, groupFindings, isReplaced, isRuledOut, sourcesToCheck, usedBy, type Finding } from '../../lib/records'
import { useWrite } from '../../state/useWrite'
import { RULE_OUT_ACTION, STILL_HOLDS_ACTION } from '../drawers/StatementDrawer'
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
export const ALL_CHECKED = 'You have checked everything.'
export const DISAGREE_HEADING = 'Sources disagree about this finding'
export const WRITE_THE_ANSWER = 'Write the answer'

/** "Source 3 of 7", "Finding 1 of 2". */
export const progressText = (position: number, total: number, noun = 'Source') => `${noun} ${position} of ${total}`

/** What is still waiting after the person skipped some sources and findings. */
export function skippedText(sources: number, findings = 0): string {
  if (findings === 0) return `You skipped ${countOf(sources, 'source', 'sources')}. ${sources === 1 ? 'It is' : 'They are'} still not checked.`
  if (sources === 0) return `You skipped ${countOf(findings, 'finding', 'findings')}. Sources still disagree about ${findings === 1 ? 'it' : 'them'}.`
  return `You skipped ${countOf(sources, 'source', 'sources')} and ${countOf(findings, 'finding', 'findings')}. They still need you.`
}

/** "Go back to the skipped source", "... sources", "... finding(s)", or "Go back to what you skipped". */
export function goBackText(sources: number, findings = 0): string {
  if (findings === 0) return `Go back to the skipped ${plural(sources, 'source', 'sources')}`
  if (sources === 0) return `Go back to the skipped ${plural(findings, 'finding', 'findings')}`
  return 'Go back to what you skipped'
}

/**
 * Where one part of the pass stands: the item to show, or none when every
 * open one was skipped or done. "done" counts the ones handled in this pass.
 */
function walkStep<T extends { ref: string }>(open: T[], skipped: string[], done: number) {
  const queue = open.filter((x) => !skipped.includes(x.ref))
  const passed = done + (open.length - queue.length)
  return { current: queue[0] as T | undefined, position: passed + 1, total: passed + queue.length, left: open.length }
}

/** The sources part: the ones a person has not checked, in page order, replaced ones left out. */
export function checkStep(evidence: Evidence[], skipped: string[], checked: number) {
  return walkStep(sourcesToCheck(evidence), skipped, checked)
}

export interface CheckSourcesProps {
  detail: InvestigationDetail
  onClose: () => void
  /** Closes the window and opens the answer editor. */
  onWriteAnswer?: () => void
}

export function CheckSources({ detail, onClose, onWriteAnswer }: CheckSourcesProps) {
  const write = useWrite()
  const inv = detail.investigation
  const [skipped, setSkipped] = useState<string[]>([])
  const [checked, setChecked] = useState(0)
  const [decided, setDecided] = useState(0)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [basis, setBasis] = useState('')
  const [basisError, setBasisError] = useState<string | undefined>()
  const mainRef = useRef<HTMLButtonElement>(null)
  const answerRef = useRef<HTMLDivElement>(null)

  const sources = checkStep(detail.evidence, skipped, checked)
  const findings = walkStep(groupFindings(detail.claims, detail.hypotheses, evidenceByRef(detail)).disagree.map((f) => ({ ref: f.record.ref, f })), skipped, decided)
  const source = sources.current
  const finding = source ? undefined : findings.current?.f
  const left = sources.left + findings.left
  const hasAnswer = inv.conclusion.trim() !== ''
  const askAnswer = !source && !finding && left === 0 && hasAnswer && !answerChecked(inv)
  const stage = source ? source.ref : finding ? finding.record.ref : askAnswer ? 'answer' : 'end'

  // The main button changes from step to step; keep the keyboard on it (or in the one field).
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
  const decide = (f: Finding, holds: boolean) => {
    const { kind, record: r } = f
    // The finding as it reads on this step: a change meanwhile redraws the step before the person can press again.
    const seen = detail.review_bases?.find((b) => b.ref === r.ref)?.basis
    return run(async () => {
      if (!holds) return kind === 'claim' ? api.updateClaim(inv.ref, r.ref, { status: 'rejected' }) : api.updateHypothesis(inv.ref, r.ref, { status: 'disproven' })
      if (!seen) throw new ApiError('conflict', 'this finding changed; show the new version')
      await api.reviewStatement(inv.ref, { kind, ref: r.ref, basis: seen, verification: 'human_verified', settle_dispute: true })
      // One the person decided holds is no longer waiting to be tested.
      if (kind === 'hypothesis' && r.status === 'open') await api.updateHypothesis(inv.ref, r.ref, { status: 'supported' })
      return true
    }, () => setDecided((n) => n + 1))
  }
  const skip = (ref: string) => setSkipped((s) => [...s, ref])

  let body
  let footer
  if (source) {
    body = <Source key={source.ref} evidence={source} detail={detail} progress={progressText(sources.position, sources.total)} />
    footer = (
      <>
        <Button variant="secondary" disabled={busy} onClick={() => skip(source.ref)}>{SKIP_ACTION}</Button>
        <Button variant="primary" ref={mainRef} aria-busy={busy || undefined} onClick={() => void check(source)}>{SOURCE_CHECKED_ACTION}</Button>
      </>
    )
  } else if (finding) {
    body = <Disagreement key={finding.record.ref} finding={finding} detail={detail} progress={progressText(findings.position, findings.total, 'Finding')} />
    footer = (
      <>
        <Button variant="secondary" disabled={busy} onClick={() => skip(finding.record.ref)}>{SKIP_ACTION}</Button>
        <Button variant="secondary" disabled={busy} onClick={() => void decide(finding, false)}>{RULE_OUT_ACTION}</Button>
        <Button variant="primary" ref={mainRef} aria-busy={busy || undefined} onClick={() => void decide(finding, true)}>{STILL_HOLDS_ACTION}</Button>
      </>
    )
  } else if (askAnswer) {
    body = (
      <div className="check-end" ref={answerRef}>
        <p className="check-end-heading" role="status">Last, check the answer.</p>
        <p>Read the answer. Does it say what the sources say?</p>
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
    const writeNext = left === 0 && !hasAnswer && onWriteAnswer
    body = (
      <div className="check-end">
        <p className="check-end-heading" role="status">{left === 0 ? ALL_CHECKED : skippedText(sources.left, findings.left)}</p>
        {writeNext && <p>The next step is to write the answer.</p>}
      </div>
    )
    footer = (
      <>
        {left > 0 && <Button variant="secondary" onClick={() => setSkipped([])}>{goBackText(sources.left, findings.left)}</Button>}
        <Button variant={writeNext ? 'secondary' : 'primary'} ref={writeNext ? undefined : mainRef} onClick={onClose}>Done</Button>
        {writeNext && <Button variant="primary" ref={mainRef} onClick={onWriteAnswer}>{WRITE_THE_ANSWER}</Button>}
      </>
    )
  }

  return (
    <Dialog title={CHECK_SOURCES_TITLE} className={['check-sources', stage === 'end' ? 'is-end' : ''].filter(Boolean).join(' ')} onClose={onClose} initialFocus={mainRef} dirty={basis.trim() !== ''} footer={footer}>
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

/** One finding where sources disagree: the finding, then its sources both ways with who checked each. */
function Disagreement({ finding: { record: r }, detail, progress }: { finding: Finding; detail: InvestigationDetail; progress: string }) {
  const byRef = evidenceByRef(detail)
  const side = (relation: 'supports' | 'contradicts') => r.links.filter((l) => l.relation === relation)
  return (
    <div className="check-source">
      <p className="check-progress" role="status">{progress}</p>
      <h3 className="check-source-title">{DISAGREE_HEADING}</h3>
      <p className="check-answer">{r.statement}</p>
      <dl className="check-source-facts">
        {(['supports', 'contradicts'] as const).map((relation) =>
          side(relation).length > 0 && (
            <div key={relation}>
              <dt>{relationTags[relation]}</dt>
              {side(relation).map((l) => {
                const e = byRef.get(l.evidence_ref)
                return (
                  <dd key={l.evidence_ref} className="check-disagree-source">
                    <span>{e?.title ?? l.evidence_title}</span> {e && <VerificationBadge verification={e.verification} />}
                    {e?.summary && <span className="check-disagree-summary">{e.summary}</span>}
                  </dd>
                )
              })}
            </div>
          ),
        )}
      </dl>
      <p>Read the sources. Does the finding still hold?</p>
    </div>
  )
}
