// The answer: the one bold element on the screen. View mode shows the answer
// in Literata 20, how many of the question's sources a person has checked,
// one line saying whether they checked the answer, one row of buttons (Check
// the next source while one is left, then Decide where sources disagree while
// only those findings are left, then Check this answer, then Share the
// answer; the first one that applies is the main button, and Check this
// answer waits until nothing else is left) and how the person knows; the
// findings under What backs this up, and apart from them the findings where
// sources disagree, counted from the page as it is now (ruled out and
// replaced ones are not counted). With no answer and no finding that holds,
// the empty line says what Home says: Add a finding from your sources. Once
// the person checked the answer, Undo my check sets it back to not checked.
// Edit mode is inline in the same panel: the answer and how you know, saved
// together through UpdateConclusion with the findings on the page behind it
// (lib/records answerFindings). Confidence is stored and passed through
// unchanged, never shown. Clearing asks first.
// Saves show no toast; the panel returns to view mode with the saved value.
//
// Edit-in-place safety: the editor snapshots the stored values when it
// opens. A poll-driven reload never touches unsaved input; when the stored
// answer changed meanwhile it says so and offers the new version. Without
// unsaved input the editor simply follows the latest stored values.
import { useEffect, useRef, useState, type KeyboardEvent } from 'react'
import { Pencil, ScrollText } from 'lucide-react'
import { api } from '../../api/api'
import { useChangedElsewhere } from '../../hooks/useChangedElsewhere'
import type { Claim, ConclusionInput, Confidence, Evidence, Hypothesis, Investigation } from '../../api/types'
import { LIMITS, nearLimit } from '../../lib/bytes'
import { formatDate } from '../../lib/relativeTime'
import { splitFieldErrors } from '../../lib/formErrors'
import { confidenceOptions } from '../../lib/labels'
import { countOf, plural } from '../../lib/plural'
import { ADD_FINDING_NEED } from '../../lib/questions'
import { answerFindings, evidenceByRef, groupFindings, sourceChecks, sourcesToCheck } from '../../lib/records'
import { useDirtyEditor } from '../../state/dirtyGuard'
import { useWrite } from '../../state/useWrite'
import { CodedText } from '../record/RecordId'
import { Banner } from '../ui/Banner'
import { Button } from '../ui/Button'
import { ChangedElsewhereBanner, CHANGED_ELSEWHERE_COPY, SHOW_NEW_VERSION } from '../ui/ChangedElsewhereBanner'
import { ConfirmDialog } from '../ui/ConfirmDialog'
import { TextArea } from '../ui/TextArea'
import { ShareSummaryDialog } from './ShareSummaryDialog'
import './ConclusionPanel.css'

const FIELDS = ['conclusion', 'rationale', 'review_basis'] as const

interface Draft {
  conclusion: string
  rationale: string
}

const draftOf = (inv: Investigation): Draft => ({ conclusion: inv.conclusion, rationale: inv.conclusion_rationale })

const sameDraft = (a: Draft, b: Draft) => a.conclusion === b.conclusion && a.rationale === b.rationale

/** The stored confidence, sent back unchanged with every save. */
const storedConfidence = (inv: Investigation): Confidence =>
  confidenceOptions.includes(inv.conclusion_confidence) ? inv.conclusion_confidence : 'unknown'

export interface ConclusionPanelProps {
  investigation: Investigation
  /** The question's claims: a saved answer lists the ones on the page behind it. */
  claims?: Claim[]
  /** The question's sources, for the count of how many a person has checked. */
  evidence?: Evidence[]
  /** The question's hypotheses: one where sources disagree waits for Decide too. */
  hypotheses?: Hypothesis[]
  /** Starts the source check; with it the panel offers Check the next source, or Decide. */
  onCheckSources?: () => void
  /** Offer Share the answer once there is an answer. */
  share?: boolean
  /** The question has nothing in it yet: no Write the answer button. */
  bare?: boolean
  /** Opened from Home to write the answer or to check it. */
  intent?: 'write-answer' | 'check-answer' | null
  onIntentDone?: () => void
}

export const CHECK_NEXT_SOURCE = 'Check the next source'
export const DECIDE_ACTION = 'Decide'
/** The answer card's main button while only findings where sources disagree are left. */
export const DECIDE_WHERE_ACTION = 'Decide where sources disagree'
/** Opens the answer check; CHECKED_ANSWER_ACTION confirms it. */
export const CHECK_THIS_ANSWER = 'Check this answer'
export const SHARE_THE_ANSWER = 'Share the answer'
export const CHECKED_ANSWER_ACTION = 'I checked this answer'
export const NOT_CHECKED_ANSWER = 'You have not checked this answer yet.'
export const JUST_WROTE_ANSWER = 'You wrote this. Check it against the sources before you share it.'
export const UNDO_ANSWER_CHECK = 'Undo my check'
export const CHECK_ANSWER_AGAIN = 'Something changed after you checked this answer. Check it again.'
export const WHAT_DID_YOU_CHECK = 'What did you check? Write one sentence.'

/** True when the person checked the answer and nothing changed since. */
export const answerChecked = (inv: Investigation) => Boolean(inv.conclusion_reviewed_at) && !inv.conclusion_needs_review

/** The save that records a person's own check of the answer as it read to them. */
export function answerCheckInput(inv: Investigation, basis: string): ConclusionInput {
  return {
    conclusion: inv.conclusion,
    confidence: storedConfidence(inv),
    rationale: inv.conclusion_rationale,
    verification: 'human_verified',
    review: true,
    review_basis: basis,
    expected_review_token: inv.conclusion_review_token,
  }
}

/** "1 of 3 sources checked by you", then who checked the rest. A count, not a rating. */
export function SourcesChecked({ evidence }: { evidence: Evidence[] }) {
  const n = sourceChecks(evidence)
  if (n.total === 0) return <p className="conclusion-checked">No sources yet</p>
  const rest = [n.byAi > 0 ? `${n.byAi} checked by AI` : '', n.notChecked > 0 ? `${n.notChecked} not checked` : ''].filter(Boolean).join(', ')
  return (
    <p className="conclusion-checked">
      <span><strong>{n.byYou} of {n.total}</strong> {plural(n.total, 'source', 'sources')} checked by you</span>
      {rest && <span className="conclusion-checked-rest">{rest}</span>}
    </p>
  )
}

export function ConclusionPanel({ investigation: inv, claims = [], evidence = [], hypotheses = [], onCheckSources, share = false, bare = false, intent, onIntentDone }: ConclusionPanelProps) {
  const write = useWrite()
  const [editing, setEditing] = useState(false)
  const [draft, setDraft] = useState<Draft>(() => draftOf(inv))
  const [snapshot, setSnapshot] = useState<Draft>(() => draftOf(inv))
  // The answer as it read when the person pressed I checked this answer.
  const [checking, setChecking] = useState<Investigation | null>(null)
  const [checkBasis, setCheckBasis] = useState('')
  const [busy, setBusy] = useState(false)
  // Set by the person's own save of the answer, for the line under it.
  const [justWrote, setJustWrote] = useState(false)
  const [confirmClear, setConfirmClear] = useState(false)
  const [clearing, setClearing] = useState(false)
  // Lives here, not in the button, so a reload that moves the button cannot close the dialog.
  const [sharing, setSharing] = useState(false)
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({})
  const [summary, setSummary] = useState<string | undefined>()
  const [focusTarget, setFocusTarget] = useState<'editor' | 'button' | 'check' | 'checkButton' | null>(null)
  const editorRef = useRef<HTMLDivElement>(null)
  const editButtonRef = useRef<HTMLButtonElement>(null)
  const checkFormRef = useRef<HTMLDivElement>(null)
  const checkButtonRef = useRef<HTMLButtonElement>(null)

  const has = inv.conclusion.trim() !== ''
  const stored = draftOf(inv)
  const dirty = editing && !sameDraft(draft, snapshot)
  useDirtyEditor(dirty || (checking !== null && checkBasis.trim() !== ''), 'detail')
  // An own save reloads before the editor closes; stored values that already
  // match the draft are not a conflict.
  const changedElsewhere =
    useChangedElsewhere(editing ? snapshot : null, stored, dirty, sameDraft) && !busy && !sameDraft(stored, draft)

  // A clean editor follows reloads; a dirty one keeps its input.
  const storedKey = JSON.stringify(stored)
  useEffect(() => {
    if (!editing || dirty) return
    const latest: Draft = JSON.parse(storedKey)
    setSnapshot((s) => (sameDraft(s, latest) ? s : latest))
    setDraft((d) => (sameDraft(d, latest) ? d : latest))
  }, [storedKey, editing, dirty])

  const showLatest = () => {
    setSnapshot(stored)
    setDraft(stored)
    setFieldErrors({})
    setSummary(undefined)
  }

  // Move focus into a form when it opens and back to its button when it closes.
  useEffect(() => {
    if (focusTarget === 'editor') editorRef.current?.querySelector('textarea')?.focus()
    if (focusTarget === 'button') editButtonRef.current?.focus()
    if (focusTarget === 'check') checkFormRef.current?.querySelector('textarea')?.focus()
    if (focusTarget === 'checkButton') checkButtonRef.current?.focus()
    if (focusTarget) setFocusTarget(null)
  }, [focusTarget])

  const begin = () => {
    setDraft(draftOf(inv))
    setSnapshot(draftOf(inv))
    setFieldErrors({})
    setSummary(undefined)
    setEditing(true)
    setFocusTarget('editor')
  }

  const finish = () => {
    setEditing(false)
    setFieldErrors({})
    setSummary(undefined)
    setFocusTarget('button')
  }

  const showErrors = (e: unknown) => {
    const split = splitFieldErrors(e, FIELDS)
    setFieldErrors(split.fieldErrors)
    setSummary(split.summary)
  }

  const save = async () => {
    if (busy) return
    if (sameDraft(draft, stored)) {
      finish()
      return
    }
    const filled = draft.conclusion.trim() !== ''
    setBusy(true)
    setFieldErrors({})
    setSummary(undefined)
    try {
      const saved = await write(() =>
        api.updateConclusion(inv.ref, {
          conclusion: draft.conclusion,
          // Nobody edits confidence here: the stored value rides along, and an
          // emptied answer carries none (the engine refuses one).
          confidence: filled ? storedConfidence(inv) : '',
          rationale: draft.rationale,
          // The answer rests on the findings on the page now. An answer the
          // person checked and then changed is not checked any more.
          ...(filled ? answerFindings(claims, evidenceByRef({ evidence })) : {}),
          ...(filled && inv.conclusion_verification === 'human_verified' &&
            (draft.conclusion !== inv.conclusion || draft.rationale !== inv.conclusion_rationale)
            ? { verification: 'ai_generated' as const } : {}),
        }),
      )
      if (saved) {
        setJustWrote(filled)
        finish()
      }
    } catch (e) {
      showErrors(e)
    } finally {
      setBusy(false)
    }
  }

  // Takes back the person's check: the same answer, saved as not checked.
  const undoCheck = async () => {
    if (busy) return
    setBusy(true)
    setSummary(undefined)
    try {
      const saved = await write(() =>
        api.updateConclusion(inv.ref, { conclusion: inv.conclusion, confidence: storedConfidence(inv), rationale: inv.conclusion_rationale, verification: 'ai_generated' }),
      )
      if (saved) {
        setJustWrote(false)
        setFocusTarget('checkButton')
      }
    } catch (e) {
      showErrors(e)
    } finally {
      setBusy(false)
    }
  }

  const beginCheck = () => {
    setChecking(inv)
    setCheckBasis('')
    setFieldErrors({})
    setSummary(undefined)
    setFocusTarget('check')
  }

  const endCheck = (focus: boolean) => {
    setChecking(null)
    setCheckBasis('')
    setFieldErrors({})
    setSummary(undefined)
    if (focus) setFocusTarget('checkButton')
  }

  const recordCheck = async () => {
    if (!checking || busy) return
    setBusy(true)
    setFieldErrors({})
    setSummary(undefined)
    try {
      const saved = await write(() => api.updateConclusion(inv.ref, answerCheckInput(checking, checkBasis)))
      if (saved) endCheck(false)
    } catch (e) { showErrors(e) } finally { setBusy(false) }
  }

  // The same groups as What backs this up and Where sources disagree on the page now,
  // so one ruled out or replaced since the save is not counted.
  const sources = evidenceByRef({ evidence })
  const groups = groupFindings(claims, hypotheses, sources)
  const backing = groups.backing.map((f) => f.record)
  const disagreeing = groups.disagree.map((f) => f.record)
  const checkedAt = inv.conclusion_reviewed_at
  const upToDate = answerChecked(inv)
  const checkStale = checking !== null && checking.conclusion_review_token !== inv.conclusion_review_token

  // One row of buttons, in the order a person does them: check the sources,
  // decide where they disagree, check the answer, share it. The first one
  // left is the main button, so Share the answer is never the main one before
  // the checking is done, and the answer cannot be checked before its sources.
  const sourcesLeft = sourcesToCheck(evidence).length > 0
  const decideLeft = disagreeing.length > 0
  const next = Boolean(onCheckSources) && (sourcesLeft || decideLeft)
  const canCheck = has && !upToDate && !sourcesLeft && !decideLeft
  const canShare = share && has
  const actions = (next || canShare || canCheck) && !checking && (
    <div className="conclusion-next">
      {next && <Button variant="primary" onClick={onCheckSources}>{sourcesLeft ? CHECK_NEXT_SOURCE : DECIDE_WHERE_ACTION}</Button>}
      {canCheck && <Button variant={next ? 'secondary' : 'primary'} ref={checkButtonRef} onClick={beginCheck}>{CHECK_THIS_ANSWER}</Button>}
      {canShare && <Button variant={next || canCheck ? 'secondary' : 'primary'} icon={ScrollText} onClick={() => setSharing(true)}>{SHARE_THE_ANSWER}</Button>}
    </div>
  )

  // Home opens the page straight on the answer editor or the answer check.
  useEffect(() => {
    if (!intent) return
    if (intent === 'write-answer') begin()
    else if (canCheck) beginCheck()
    onIntentDone?.()
    // Runs once per intent.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [intent])

  const clear = async () => {
    setClearing(true)
    try {
      const saved = await write(() => api.updateConclusion(inv.ref, { conclusion: '', confidence: '', rationale: '' }))
      setConfirmClear(false)
      if (saved) finish()
    } catch (e) {
      setConfirmClear(false)
      showErrors(e)
    } finally {
      setClearing(false)
    }
  }

  const onEditorKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) {
      e.preventDefault()
      void save()
    }
  }

  return (
    <section className="conclusion-panel" aria-labelledby="conclusion-heading">
      <div className="conclusion-head">
        <h2 className="conclusion-heading" id="conclusion-heading">
          The answer
        </h2>
        {!editing && has && !checking && (
          <Button variant="ghost" icon={Pencil} ref={editButtonRef} onClick={begin}>
            Edit answer
          </Button>
        )}
      </div>

      {editing ? (
        <div className="conclusion-editor" ref={editorRef} onKeyDown={onEditorKeyDown}>
          {changedElsewhere && <ChangedElsewhereBanner onShowLatest={showLatest} />}
          {summary && (
            <p className="conclusion-summary" role="alert">
              {summary}
            </p>
          )}
          <TextArea
            label="Answer"
            value={draft.conclusion}
            onChange={(conclusion) => setDraft((d) => ({ ...d, conclusion }))}
            error={fieldErrors.conclusion}
            note={nearLimit(draft.conclusion, LIMITS.conclusion)}
            spellCheck
          />
          <TextArea
            label="How you know (optional)"
            value={draft.rationale}
            onChange={(rationale) => setDraft((d) => ({ ...d, rationale }))}
            error={fieldErrors.rationale}
            note={nearLimit(draft.rationale, LIMITS.conclusionRationale)}
            spellCheck
          />
          <div className="conclusion-actions">
            {has && (
              <Button variant="ghost" className="btn-ghost-danger" onClick={() => setConfirmClear(true)}>
                Clear answer
              </Button>
            )}
            <div className="conclusion-actions-main">
              <Button variant="secondary" onClick={finish} disabled={busy}>
                Cancel
              </Button>
              <Button variant="primary" busy={busy} busyLabel="Saving..." onClick={() => void save()}>
                Save answer
              </Button>
            </div>
          </div>
        </div>
      ) : has ? (
        <div className="conclusion-view">
          <p className="conclusion-text"><CodedText text={inv.conclusion} /></p>
          <SourcesChecked evidence={evidence} />
          <div className="conclusion-check-line">
            <p className="conclusion-check-state" role="status">
              {!checkedAt ? (justWrote ? JUST_WROTE_ANSWER : NOT_CHECKED_ANSWER) : !upToDate ? CHECK_ANSWER_AGAIN : <>You checked this answer on <time dateTime={checkedAt}>{formatDate(checkedAt)}</time>.</>}
            </p>
            {upToDate && !checking && <Button variant="ghost" busy={busy} busyLabel="Saving..." onClick={() => void undoCheck()}>{UNDO_ANSWER_CHECK}</Button>}
          </div>
          {actions}
          {!checking && summary && <p className="conclusion-summary" role="alert">{summary}</p>}
          {checking && (
            <div className="conclusion-check-form" ref={checkFormRef}>
              {checkStale && <Banner action={<Button variant="ghost" onClick={() => setChecking(inv)}>{SHOW_NEW_VERSION}</Button>}>{CHANGED_ELSEWHERE_COPY}</Banner>}
              {summary && <p className="conclusion-summary" role="alert">{summary}</p>}
              <TextArea label={WHAT_DID_YOU_CHECK} value={checkBasis} onChange={setCheckBasis} error={fieldErrors.review_basis} note={nearLimit(checkBasis, LIMITS.conclusionRationale)} />
              <div className="conclusion-actions">
                <Button variant="secondary" onClick={() => endCheck(true)} disabled={busy}>Cancel</Button>
                <Button variant="primary" onClick={() => void recordCheck()} busy={busy} busyLabel="Saving..." disabled={!checkBasis.trim() || checkStale}>{CHECKED_ANSWER_ACTION}</Button>
              </div>
            </div>
          )}
          {inv.conclusion_verification === 'disputed' && <Banner>Sources disagree about this answer. Read them before you use it.</Banner>}
          {backing.length > 0 && <details className="conclusion-basis"><summary>{countOf(backing.length, 'finding backs it', 'findings back it')}</summary><ul>{backing.map((f) => <li key={f.ref}>{f.statement}</li>)}</ul></details>}
          {disagreeing.length > 0 && <details className="conclusion-basis"><summary>{`${disagreeing.length} where sources disagree`}</summary><ul>{disagreeing.map((f) => <li key={f.ref}>{f.statement}</li>)}</ul></details>}
          {inv.conclusion_rationale.trim() !== '' && <p className="conclusion-rationale"><span className="conclusion-why">How you know:</span> <CodedText text={inv.conclusion_rationale} /></p>}
          {upToDate && inv.conclusion_review_basis && <p className="conclusion-rationale"><span className="conclusion-why">What you checked:</span> <CodedText text={inv.conclusion_review_basis} /></p>}
        </div>
      ) : (
        <div className="conclusion-empty">
          <p className="conclusion-empty-text">{bare || next || backing.length > 0 ? 'No answer yet.' : `No answer yet. ${ADD_FINDING_NEED}`}</p>
          {/* An answer stands on a finding that holds, as Home says. */}
          {!bare && backing.length > 0 && (
            <Button variant="ghost" icon={Pencil} ref={editButtonRef} onClick={begin}>
              Write the answer
            </Button>
          )}
          {actions}
        </div>
      )}

      {sharing && <ShareSummaryDialog invRef={inv.ref} onClose={() => setSharing(false)} />}
      {confirmClear && (
        <ConfirmDialog
          title="Clear the answer?"
          body="This removes the answer and how you know. The history still shows that it changed."
          confirmLabel="Clear answer"
          busy={clearing}
          busyLabel="Clearing..."
          onConfirm={() => void clear()}
          onCancel={() => setConfirmClear(false)}
        />
      )}
    </section>
  )
}
