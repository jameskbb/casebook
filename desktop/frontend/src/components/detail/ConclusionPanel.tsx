// The answer: the one bold element on the screen. View mode shows the answer
// in Literata 20, how many of the question's sources a person has checked,
// its label, and the rationale after "Why:". Edit mode is inline in the same
// panel: the answer and the rationale, saved together through
// UpdateConclusion. Confidence is stored and passed through unchanged, never
// shown. Clearing asks first. Saves show no toast; the panel returns to view
// mode with the saved value.
//
// Edit-in-place safety: the editor snapshots the stored values when it
// opens. A poll-driven reload never touches unsaved input; when the stored
// answer or rationale changed elsewhere it shows "This
// changed in another app." with "Show latest". Without unsaved input the
// editor simply follows the latest stored values.
import { useEffect, useRef, useState, type KeyboardEvent } from 'react'
import { Pencil } from 'lucide-react'
import { api } from '../../api/api'
import { useChangedElsewhere } from '../../hooks/useChangedElsewhere'
import type { Claim, Confidence, Evidence, Investigation, Verification } from '../../api/types'
import { LIMITS, nearLimit } from '../../lib/bytes'
import { formatAbsolute } from '../../lib/relativeTime'
import { splitFieldErrors } from '../../lib/formErrors'
import { claimStatusLabels, confidenceOptions, verificationLabels } from '../../lib/labels'
import { plural } from '../../lib/plural'
import { sourceChecks } from '../../lib/records'
import { useDirtyEditor } from '../../state/dirtyGuard'
import { useWrite } from '../../state/useWrite'
import { VerificationBadge } from '../record/VerificationBadge'
import { Select } from '../ui/Select'

import { Banner } from '../ui/Banner'
import { Button } from '../ui/Button'
import { ConfirmDialog } from '../ui/ConfirmDialog'

import { TextArea } from '../ui/TextArea'
import './ConclusionPanel.css'

const FIELDS = ['conclusion', 'rationale', 'verification', 'supporting_claims', 'contradicting_claims', 'review_basis', 'review'] as const

interface Draft {
  conclusion: string
  confidence: Confidence
  rationale: string
  verification: Verification
  supporting: string[]
  contradicting: string[]
  token: string
}

function draftOf(inv: Investigation): Draft {
  return {
    conclusion: inv.conclusion,
    confidence: confidenceOptions.includes(inv.conclusion_confidence) ? inv.conclusion_confidence : 'unknown',
    rationale: inv.conclusion_rationale,
    verification: inv.conclusion_verification || 'ai_generated',
    supporting: inv.conclusion_supporting_claims ?? [],
    contradicting: inv.conclusion_contradicting_claims ?? [],
    token: inv.conclusion_review_token ?? '',
  }
}

const sameDraft = (a: Draft, b: Draft) =>
  a.conclusion === b.conclusion && a.confidence === b.confidence && a.rationale === b.rationale && a.verification === b.verification && JSON.stringify(a.supporting) === JSON.stringify(b.supporting) && JSON.stringify(a.contradicting) === JSON.stringify(b.contradicting) && a.token === b.token

export interface ConclusionPanelProps {
  investigation: Investigation
  claims?: Claim[]
  /** The question's sources, for the count of how many a person has checked. */
  evidence?: Evidence[]
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

export function ConclusionPanel({ investigation: inv, claims = [], evidence = [] }: ConclusionPanelProps) {
  const write = useWrite()
  const [editing, setEditing] = useState(false)
  const [draft, setDraft] = useState<Draft>(() => draftOf(inv))
  const [snapshot, setSnapshot] = useState<Draft>(() => draftOf(inv))
  const [reviewSnapshot, setReviewSnapshot] = useState<Investigation | null>(null)
  const [reviewBasis, setReviewBasis] = useState('')
  const [busy, setBusy] = useState(false)
  const [confirmClear, setConfirmClear] = useState(false)
  const [clearing, setClearing] = useState(false)
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({})
  const [summary, setSummary] = useState<string | undefined>()
  const [focusTarget, setFocusTarget] = useState<'editor' | 'button' | null>(null)
  const editorRef = useRef<HTMLDivElement>(null)
  const editButtonRef = useRef<HTMLButtonElement>(null)

  const has = inv.conclusion.trim() !== ''
  const stored = draftOf(inv)
  const dirty = editing && !sameDraft(draft, snapshot)
  useDirtyEditor(dirty || (reviewSnapshot !== null && reviewBasis.trim() !== ''), 'detail')
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

  // Move focus into the editor when it opens and back to the entry button
  // when it closes.
  useEffect(() => {
    if (focusTarget === 'editor') editorRef.current?.querySelector('textarea')?.focus()
    if (focusTarget === 'button') editButtonRef.current?.focus()
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
    setBusy(true)
    setFieldErrors({})
    setSummary(undefined)
    try {
      const saved = await write(() =>
        api.updateConclusion(inv.ref, {
          conclusion: draft.conclusion,
          // Nobody edits confidence here: the stored value rides along, and an
          // emptied answer carries none (the engine refuses one).
          confidence: draft.conclusion.trim() ? draft.confidence : '',
          rationale: draft.rationale,
          ...(draft.conclusion.trim() ? { verification: draft.verification, supporting_claims: draft.supporting, contradicting_claims: draft.contradicting } : {}),
        }),
      )
      if (saved) finish()
    } catch (e) {
      showErrors(e)
    } finally {
      setBusy(false)
    }
  }

  const recordReview = async () => {
    if (!reviewSnapshot || busy) return
    setBusy(true)
    setSummary(undefined)
    try {
      const saved = await write(() => api.updateConclusion(inv.ref, {
        conclusion: reviewSnapshot.conclusion,
        confidence: reviewSnapshot.conclusion_confidence,
        rationale: reviewSnapshot.conclusion_rationale,
        verification: 'human_verified',
        review: true,
        review_basis: reviewBasis,
        expected_review_token: reviewSnapshot.conclusion_review_token,
      }))
      if (saved) { setReviewSnapshot(null); setReviewBasis('') }
    } catch (e) { showErrors(e) } finally { setBusy(false) }
  }

  const claimLabel = (ref: string) => {
    const claim = claims.find((c) => c.ref === ref)
    return claim ? `${ref}: ${claim.statement} (${verificationLabels[claim.verification]}, ${claimStatusLabels[claim.status]})` : `${ref}: finding unavailable`
  }

  const claimSummary = (refs: string[]) => refs.slice(0, 3).map((ref) => {
    const claim = claims.find((c) => c.ref === ref)
    return `${ref} (${claim ? verificationLabels[claim.verification] : 'unavailable'})`
  }).join('; ') + (refs.length > 3 ? `; ${refs.length - 3} more` : '')

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
        {!editing && has && !reviewSnapshot && (
          <Button variant="ghost" icon={Pencil} ref={editButtonRef} onClick={begin}>
            Edit answer
          </Button>
        )}
      </div>

      {editing ? (
        <div className="conclusion-editor" ref={editorRef} onKeyDown={onEditorKeyDown}>
          {changedElsewhere && (
            <Banner
              className="changed-elsewhere"
              action={
                <Button variant="ghost" onClick={showLatest}>
                  Show latest
                </Button>
              }
            >
              This changed in another app.
            </Banner>
          )}
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
            label="Why (optional)"
            value={draft.rationale}
            onChange={(rationale) => setDraft((d) => ({ ...d, rationale }))}
            error={fieldErrors.rationale}
            note={nearLimit(draft.rationale, LIMITS.conclusionRationale)}
            spellCheck
          />
          <Select label="Label" value={draft.verification}
            options={(['ai_generated', 'human_verified', 'source_verified', 'disputed'] as Verification[]).map((v) => ({ value: v, label: verificationLabels[v] }))}
            onChange={(verification) => setDraft((d) => ({ ...d, verification: verification as Verification }))}
            error={fieldErrors.verification}
            hint="This label covers the answer text. Record a review after checking the findings behind it and their sources." />
          {claims.length > 0 && <fieldset className="conclusion-claim-selection">
            <legend>Findings behind this answer</legend>
            <p>Choose the findings you considered. Include the ones that contradict it so the next reader can check them.</p>
            {claims.map((claim) => <Select key={claim.ref} label={claimLabel(claim.ref)}
              value={draft.supporting.includes(claim.ref) ? 'supports' : draft.contradicting.includes(claim.ref) ? 'contradicts' : ''}
              options={[{ value: '', label: 'Not selected' }, { value: 'supports', label: 'Supports the answer' }, { value: 'contradicts', label: 'Contradicts the answer' }]}
              onChange={(role) => setDraft((d) => ({ ...d, supporting: [...d.supporting.filter((r) => r !== claim.ref), ...(role === 'supports' ? [claim.ref] : [])].sort(), contradicting: [...d.contradicting.filter((r) => r !== claim.ref), ...(role === 'contradicts' ? [claim.ref] : [])].sort() }))} />)}
            {(fieldErrors.supporting_claims || fieldErrors.contradicting_claims) && <p role="alert">{fieldErrors.supporting_claims || fieldErrors.contradicting_claims}</p>}
          </fieldset>}
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
          <p className="conclusion-text">{inv.conclusion}</p>
          <SourcesChecked evidence={evidence} />
          <VerificationBadge verification={inv.conclusion_verification || 'ai_generated'} />
          {inv.conclusion_verification === 'disputed' && <Banner>Sources disagree about this answer. Read the findings that contradict it and their sources before relying on it.</Banner>}
          {(inv.conclusion_supporting_claims?.length ?? 0) > 0 && <details className="conclusion-basis"><summary>Findings that support it: {claimSummary(inv.conclusion_supporting_claims)}</summary><ul>{inv.conclusion_supporting_claims.map((ref) => <li key={ref}>{claimLabel(ref)}</li>)}</ul></details>}
          {(inv.conclusion_contradicting_claims?.length ?? 0) > 0 && <details className="conclusion-basis"><summary>Findings that contradict it: {claimSummary(inv.conclusion_contradicting_claims)}</summary><ul>{inv.conclusion_contradicting_claims.map((ref) => <li key={ref}>{claimLabel(ref)}</li>)}</ul></details>}
          {!(inv.conclusion_supporting_claims?.length || inv.conclusion_contradicting_claims?.length) && <p className="conclusion-review-record">No findings are linked to this answer.</p>}
          {inv.conclusion_reviewed_at ? <p className="conclusion-review-record">Reviewed <time dateTime={inv.conclusion_reviewed_at} title={inv.conclusion_reviewed_at}>{formatAbsolute(inv.conclusion_reviewed_at)}</time>.</p> : <p className="conclusion-review-record">You have not reviewed this answer.</p>}
          {(inv.conclusion_needs_review || !inv.conclusion_reviewed_at) && <Banner>{inv.conclusion_verification === 'disputed' ? 'Review needed: sources disagree about this answer.' : inv.conclusion_reviewed_at ? 'The findings behind this answer, or their sources, changed since your review. The label still describes the answer text that was checked.' : 'Review needed: check the answer against the findings behind it and their sources.'}</Banner>}
          {summary && <p className="conclusion-summary" role="alert">{summary}</p>}
          {reviewSnapshot ? <div className="conclusion-review-form">
            {reviewSnapshot.conclusion_review_token !== inv.conclusion_review_token && <Banner>This answer or the findings behind it changed in another app. Cancel and review the latest version.</Banner>}
            <TextArea label="What did you check?" value={reviewBasis} onChange={setReviewBasis} error={fieldErrors.review_basis} note={nearLimit(reviewBasis, LIMITS.conclusionRationale)} />
            <div className="conclusion-actions"><Button variant="secondary" onClick={() => { setReviewSnapshot(null); setReviewBasis(''); setSummary(undefined) }} disabled={busy}>Cancel review</Button>
              <Button variant="primary" onClick={() => void recordReview()} busy={busy} disabled={!reviewBasis.trim() || reviewSnapshot.conclusion_review_token !== inv.conclusion_review_token}>I checked this answer</Button></div>
          </div> : <Button variant="secondary" onClick={() => { setReviewSnapshot(inv); setReviewBasis(''); setSummary(undefined) }}>Review the answer</Button>}
          {(inv.conclusion_review_basis || inv.conclusion_rationale.trim()) && <details className="conclusion-basis">
            <summary>Review notes</summary>
            {inv.conclusion_review_basis && <p className="conclusion-rationale">{inv.conclusion_review_basis}</p>}
            {inv.conclusion_rationale.trim() !== '' && <p className="conclusion-rationale"><span className="conclusion-why">Why:</span> {inv.conclusion_rationale}</p>}
          </details>}
        </div>
      ) : (
        <div className="conclusion-empty">
          <p className="conclusion-empty-text">No answer yet. Write it here when the findings back one.</p>
          <Button variant="ghost" icon={Pencil} ref={editButtonRef} onClick={begin}>
            Write the answer
          </Button>
        </div>
      )}

      {confirmClear && (
        <ConfirmDialog
          title="Clear the answer?"
          body="This removes the answer and its notes. The history keeps a record that it changed."
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
