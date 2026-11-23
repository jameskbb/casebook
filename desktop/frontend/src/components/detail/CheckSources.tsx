// Check the next source: the one pass where a person does the checking, in
// one window and in this order. First each source they have not checked: the
// question as the step's heading (focus starts on it), its
// name, what it shows, where to find it, the exact words, and the finding it
// backs or goes against; "I checked this" saves the check and moves on by
// itself. Then each finding where sources disagree, under the title Decide
// about this finding: the question as the step's heading (focus starts on it,
// so Enter cannot decide unread), the finding, its sources both ways with
// their exact words, and an optional Why it still holds (starts empty; the
// reason is added after the finding's why, so none is lost): It still holds, Rule this out
// or Skip. Last the answer, when there is one the person has not checked, so
// a decision never undoes a check made a minute before. The end says what
// happened, and offers Write the answer when there is none and a finding
// holds, or Add a finding when none holds. Opened from a
// finding's Decide button, the pass starts on that finding and then goes on
// in the same order.
//
// The window reads the latest detail on every render, so a source an AI adds
// meanwhile joins the end and one checked elsewhere drops out. Each source
// step starts on its question, so Enter cannot check a source unread; Escape
// closes and the page puts focus back on the answer's buttons.
import { useEffect, useRef, useState, type ReactNode, type RefObject } from 'react'
import { api } from '../../api/api'
import { ApiError, plainError, toApiError } from '../../api/errors'
import type { Evidence, InvestigationDetail } from '../../api/types'
import { LIMITS, nearLimit } from '../../lib/bytes'
import { splitFieldErrors } from '../../lib/formErrors'
import { relationTags } from '../../lib/labels'
import { countOf, plural } from '../../lib/plural'
import { ADD_FINDING_NEED } from '../../lib/questions'
import { evidenceByRef, groupFindings, isReplaced, isRuledOut, sourcesToCheck, usedBy, type Finding } from '../../lib/records'
import { useWrite } from '../../state/useWrite'
import { RULE_OUT_ACTION, STILL_HOLDS_ACTION, STILL_HOLDS_QUESTION } from '../drawers/StatementDrawer'
import { VerificationBadge } from '../record/VerificationBadge'
import { BreakablePath } from '../ui/BreakablePath'
import { Button } from '../ui/Button'
import { Dialog } from '../ui/Dialog'
import { TextArea } from '../ui/TextArea'
import { ADD_FINDING } from './FirstFindingGuide'
import { answerChecked, answerCheckInput, CHECKED_ANSWER_ACTION, WHAT_DID_YOU_CHECK } from './ConclusionPanel'
import './CheckSources.css'

export const CHECK_SOURCES_TITLE = 'Check the sources'
export const DECIDE_TITLE = 'Decide about this finding'
export const WHY_STILL_HOLDS = 'Why it still holds (optional)'
export const SOURCE_CHECKED_ACTION = 'I checked this'
export const SOURCE_QUESTION = 'Open the source. Does it say what is written here?'
export const SKIP_ACTION = 'Skip'
export const ALL_SOURCES_CHECKED = 'You have checked every source.'
export const ALL_DECIDED = 'You decided every finding where sources disagree.'
export const WRITE_THE_ANSWER = 'Write the answer'

/** The end of the pass when nothing is left: what the person did, never more. */
export const endText = (decided: boolean) => (decided ? `${ALL_SOURCES_CHECKED} ${ALL_DECIDED}` : ALL_SOURCES_CHECKED)

/** A finding's why: on a claim it is stored as confidence_rationale. */
const findingWhy = (f: Finding) => (f.kind === 'claim' ? f.record.confidence_rationale : f.record.rationale) ?? ''

/** The why saved with It still holds: the old why, if any, then the new reason. The shared answer splits them at "Still holds: ". */
export const stillHoldsWhy = (old: string, reason: string) => [old.trim(), `Still holds: ${reason.trim()}`].filter(Boolean).join(' ')

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
  /** A finding where sources disagree to show first. */
  startAt?: string
  onClose: () => void
  /** Closes the window and opens the answer editor. */
  onWriteAnswer?: () => void
  /** Closes the window and opens Add finding. */
  onAddFinding?: () => void
}

export function CheckSources({ detail, startAt, onClose, onWriteAnswer, onAddFinding }: CheckSourcesProps) {
  const write = useWrite()
  const inv = detail.investigation
  const [skipped, setSkipped] = useState<string[]>([])
  const [checked, setChecked] = useState(0)
  const [decided, setDecided] = useState(0)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [basis, setBasis] = useState('')
  const [basisError, setBasisError] = useState<string | undefined>()
  // Why it still holds, for the finding on screen (ref). Starts empty.
  const [why, setWhy] = useState<{ ref: string; text: string } | null>(null)
  const [whyError, setWhyError] = useState<string | undefined>()
  const mainRef = useRef<HTMLButtonElement>(null)
  const headingRef = useRef<HTMLHeadingElement>(null)
  const answerRef = useRef<HTMLDivElement>(null)

  const sources = checkStep(detail.evidence, skipped, checked)
  const groups = groupFindings(detail.claims, detail.hypotheses, evidenceByRef(detail))
  const holds = groups.backing.length > 0
  const disagree = groups.disagree.map((f) => ({ ref: f.record.ref, f }))
  // The finding the person chose goes first until they decide or skip it.
  const first = startAt && !skipped.includes(startAt) ? disagree.find((x) => x.ref === startAt) : undefined
  const findings = walkStep(first ? [first, ...disagree.filter((x) => x !== first)] : disagree, skipped, decided)
  const source = first ? undefined : sources.current
  const finding = source ? undefined : findings.current?.f
  const left = sources.left + findings.left
  const hasAnswer = inv.conclusion.trim() !== ''
  const askAnswer = !source && !finding && left === 0 && hasAnswer && !answerChecked(inv)
  const stage = source ? source.ref : finding ? finding.record.ref : askAnswer ? 'answer' : 'end'

  // The main button changes from step to step; keep the keyboard on it (or in
  // the one field). A source or finding step starts on its question, not on a decision.
  useEffect(() => {
    if (stage === 'answer') answerRef.current?.querySelector('textarea')?.focus()
    else if (source || finding) headingRef.current?.focus()
    else mainRef.current?.focus()
    // Runs once per step.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [stage])

  const whyText = finding && why?.ref === finding.record.ref ? why.text : ''

  const run = async (fn: () => Promise<unknown>, after: () => void) => {
    if (busy) return
    setBusy(true)
    setError('')
    setBasisError(undefined)
    setWhyError(undefined)
    try {
      const saved = await write(fn, { rethrowConflict: true })
      if (saved !== undefined) after()
    } catch (e) {
      const split = splitFieldErrors(e, ['review_basis', 'rationale', 'confidence_rationale'])
      setBasisError(split.fieldErrors.review_basis)
      setWhyError(split.fieldErrors.rationale ?? split.fieldErrors.confidence_rationale)
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
      // The old why stays in front, so nothing the person or the AI wrote is lost.
      const reason = whyText.trim() !== '' ? { rationale: stillHoldsWhy(findingWhy(f), whyText) } : {}
      await api.reviewStatement(inv.ref, { kind, ref: r.ref, basis: seen, verification: 'human_verified', settle_dispute: true, ...reason })
      // One the person decided holds is no longer waiting to be tested.
      if (kind === 'hypothesis' && r.status === 'open') await api.updateHypothesis(inv.ref, r.ref, { status: 'supported' })
      return true
    }, () => {
      setWhy(null)
      setDecided((n) => n + 1)
    })
  }
  const skip = (ref: string) => setSkipped((s) => [...s, ref])

  let body
  let footer
  if (source) {
    body = <Source key={source.ref} evidence={source} detail={detail} progress={progressText(sources.position, sources.total)} headingRef={headingRef} />
    footer = (
      <>
        <Button variant="secondary" disabled={busy} onClick={() => skip(source.ref)}>{SKIP_ACTION}</Button>
        <Button variant="primary" ref={mainRef} aria-busy={busy || undefined} onClick={() => void check(source)}>{SOURCE_CHECKED_ACTION}</Button>
      </>
    )
  } else if (finding) {
    body = (
      <Disagreement key={finding.record.ref} finding={finding} detail={detail} progress={progressText(findings.position, findings.total, 'Finding')} headingRef={headingRef}>
        <TextArea label={WHY_STILL_HOLDS} value={whyText} onChange={(text) => setWhy({ ref: finding.record.ref, text })} error={whyError} note={nearLimit(stillHoldsWhy(findingWhy(finding), whyText), LIMITS.rationale)} spellCheck />
      </Disagreement>
    )
    footer = (
      <>
        <Button variant="secondary" disabled={busy} onClick={() => skip(finding.record.ref)}>{SKIP_ACTION}</Button>
        <Button variant="secondary" disabled={busy} onClick={() => void decide(finding, false)}>{RULE_OUT_ACTION}</Button>
        <Button variant="primary" aria-busy={busy || undefined} onClick={() => void decide(finding, true)}>{STILL_HOLDS_ACTION}</Button>
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
    // An answer stands on a finding that holds, as Home says.
    const writeNext = left === 0 && !hasAnswer && holds && onWriteAnswer
    const findNext = left === 0 && !hasAnswer && !holds && onAddFinding
    const next = writeNext || findNext
    body = (
      <div className="check-end">
        <p className="check-end-heading" role="status">{left === 0 ? endText(decided > 0) : skippedText(sources.left, findings.left)}</p>
        {writeNext && <p>The next step is to write the answer.</p>}
        {findNext && <p>{ADD_FINDING_NEED}</p>}
      </div>
    )
    footer = (
      <>
        {left > 0 && <Button variant="secondary" onClick={() => setSkipped([])}>{goBackText(sources.left, findings.left)}</Button>}
        <Button variant={next ? 'secondary' : 'primary'} ref={next ? undefined : mainRef} onClick={onClose}>Done</Button>
        {writeNext && <Button variant="primary" ref={mainRef} onClick={onWriteAnswer}>{WRITE_THE_ANSWER}</Button>}
        {findNext && <Button variant="primary" ref={mainRef} onClick={onAddFinding}>{ADD_FINDING}</Button>}
      </>
    )
  }

  return (
    <Dialog
      title={finding ? DECIDE_TITLE : CHECK_SOURCES_TITLE}
      className={['check-sources', stage === 'end' ? 'is-end' : ''].filter(Boolean).join(' ')}
      onClose={onClose}
      initialFocus={source || finding ? headingRef : mainRef}
      dirty={basis.trim() !== '' || whyText.trim() !== ''}
      footer={footer}
    >
      {error && <p className="dialog-summary" role="alert">{error}</p>}
      {body}
    </Dialog>
  )
}

/** One source, laid out the same way every time so the buttons never move. */
function Source({ evidence: e, detail, progress, headingRef }: { evidence: Evidence; detail: InvestigationDetail; progress: string; headingRef: RefObject<HTMLHeadingElement | null> }) {

  const records = new Map([...detail.claims, ...detail.hypotheses].map((r) => [r.ref, r] as const))
  const findings = usedBy(e, detail).filter((u) => {
    const r = records.get(u.ref)
    return r && !isReplaced(r) && !isRuledOut(r)
  })
  return (
    <div className="check-source">
      <p className="check-progress" role="status">{progress}</p>
      <h3 className="check-source-title" ref={headingRef} tabIndex={-1}>{SOURCE_QUESTION}</h3>
      <div>
        <h4 className="check-source-title">{e.title}</h4>
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
            <dd><RawWords invRef={detail.investigation.ref} evidence={e} /></dd>
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

/** One finding where sources disagree: the question, the finding, then its sources both ways with who checked each and their exact words; children sit last. */
function Disagreement({ finding: { record: r }, detail, progress, headingRef, children }: { finding: Finding; detail: InvestigationDetail; progress: string; headingRef: RefObject<HTMLHeadingElement | null>; children?: ReactNode }) {
  const byRef = evidenceByRef(detail)
  const side = (relation: 'supports' | 'contradicts') => r.links.filter((l) => l.relation === relation)
  return (
    <div className="check-source">
      <p className="check-progress" role="status">{progress}</p>
      <h3 className="check-source-title" ref={headingRef} tabIndex={-1}>{STILL_HOLDS_QUESTION}</h3>
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
                    {e?.has_raw_detail && <RawWords invRef={detail.investigation.ref} evidence={e} />}
                  </dd>
                )
              })}
            </div>
          ),
        )}
      </dl>
      {children}
    </div>
  )
}

/**
 * A source's exact words. They are what a person checks against, so they
 * load without asking, again when the source changes.
 */
function RawWords({ invRef, evidence: e }: { invRef: string; evidence: Evidence }) {
  const [raw, setRaw] = useState<string | null>(null)
  const [rawError, setRawError] = useState('')
  useEffect(() => {
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
  }, [invRef, e.ref, e.updated_at])
  return raw !== null ? <pre className="raw-detail">{raw}</pre> : <>{rawError || 'Loading the exact words...'}</>
}
