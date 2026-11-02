// Current conclusion (UI-12; UI-SPEC Layout item 2, D-12, D-15): the one
// bold element on the screen. View mode shows the conclusion in Literata 20,
// the large confidence meter with its long label, and the rationale after
// "Why:". Edit mode is inline in the same panel: the conclusion, a
// confidence picker and the rationale, saved together through
// UpdateConclusion. Clearing asks first. Saves show no toast; the panel
// returns to view mode with the saved value.
//
// Edit-in-place safety: the editor snapshots the stored values when it
// opens. A poll-driven reload never touches unsaved input; when the stored
// conclusion, confidence or rationale changed elsewhere it shows "This
// changed in another app." with "Show latest". Without unsaved input the
// editor simply follows the latest stored values.
import { useEffect, useRef, useState, type KeyboardEvent } from 'react'
import { Pencil } from 'lucide-react'
import { api } from '../../api/api'
import { useChangedElsewhere } from '../../hooks/useChangedElsewhere'
import type { Claim, Confidence, Investigation, Verification } from '../../api/types'
import { LIMITS, nearLimit } from '../../lib/bytes'
import { formatAbsolute } from '../../lib/relativeTime'
import { splitFieldErrors } from '../../lib/formErrors'
import { claimStatusLabels, confidenceLabels, confidenceOptions, verificationLabels } from '../../lib/labels'
import { useDirtyEditor } from '../../state/dirtyGuard'
import { useWrite } from '../../state/useWrite'
import { VerificationBadge } from '../record/VerificationBadge'
import { Select } from '../ui/Select'
import { ConfidenceMeter } from '../record/ConfidenceMeter'
import { Banner } from '../ui/Banner'
import { Button } from '../ui/Button'
import { ConfirmDialog } from '../ui/ConfirmDialog'
import { SegmentedControl } from '../ui/SegmentedControl'
import { TextArea } from '../ui/TextArea'
import './ConclusionPanel.css'

const FIELDS = ['conclusion', 'confidence', 'rationale', 'verification', 'supporting_claims', 'contradicting_claims', 'review_basis', 'review'] as const

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
}

export function ConclusionPanel({ investigation: inv, claims = [] }: ConclusionPanelProps) {
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
          confidence: draft.confidence,
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
    return claim ? `${ref}: ${claim.statement} (${verificationLabels[claim.verification]}, ${claimStatusLabels[claim.status]})` : `${ref}: claim unavailable`
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
          Current conclusion
        </h2>
        {!editing && has && !reviewSnapshot && (
          <Button variant="ghost" icon={Pencil} ref={editButtonRef} onClick={begin}>
            Update conclusion
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
            label="Conclusion"
            value={draft.conclusion}
            onChange={(conclusion) => setDraft((d) => ({ ...d, conclusion }))}
            error={fieldErrors.conclusion}
            note={nearLimit(draft.conclusion, LIMITS.conclusion)}
            spellCheck
          />
          <SegmentedControl<Confidence>
            label="Confidence"
            options={confidenceOptions.map((c) => ({ value: c, label: confidenceLabels[c] }))}
            value={draft.confidence}
            onChange={(confidence) => setDraft((d) => ({ ...d, confidence }))}
            error={fieldErrors.confidence}
          />
          <TextArea
            label="Why this confidence (optional)"
            value={draft.rationale}
            onChange={(rationale) => setDraft((d) => ({ ...d, rationale }))}
            error={fieldErrors.rationale}
            note={nearLimit(draft.rationale, LIMITS.conclusionRationale)}
            spellCheck
          />
          <Select label="Conclusion verification" value={draft.verification}
            options={(['ai_generated', 'human_verified', 'source_verified', 'disputed'] as Verification[]).map((v) => ({ value: v, label: verificationLabels[v] }))}
            onChange={(verification) => setDraft((d) => ({ ...d, verification: verification as Verification }))}
            error={fieldErrors.verification}
            hint="This label covers the conclusion statement. Record a review after checking its selected claims and evidence." />
          {claims.length > 0 && <fieldset className="conclusion-claim-selection">
            <legend>Claims behind this conclusion</legend>
            <p>Choose the claims you considered. Include contradictions so the next reader can check them.</p>
            {claims.map((claim) => <Select key={claim.ref} label={claimLabel(claim.ref)}
              value={draft.supporting.includes(claim.ref) ? 'supports' : draft.contradicting.includes(claim.ref) ? 'contradicts' : ''}
              options={[{ value: '', label: 'Not selected' }, { value: 'supports', label: 'Supports conclusion' }, { value: 'contradicts', label: 'Contradicts conclusion' }]}
              onChange={(role) => setDraft((d) => ({ ...d, supporting: [...d.supporting.filter((r) => r !== claim.ref), ...(role === 'supports' ? [claim.ref] : [])].sort(), contradicting: [...d.contradicting.filter((r) => r !== claim.ref), ...(role === 'contradicts' ? [claim.ref] : [])].sort() }))} />)}
            {(fieldErrors.supporting_claims || fieldErrors.contradicting_claims) && <p role="alert">{fieldErrors.supporting_claims || fieldErrors.contradicting_claims}</p>}
          </fieldset>}
          <div className="conclusion-actions">
            {has && (
              <Button variant="ghost" className="btn-ghost-danger" onClick={() => setConfirmClear(true)}>
                Clear conclusion
              </Button>
            )}
            <div className="conclusion-actions-main">
              <Button variant="secondary" onClick={finish} disabled={busy}>
                Cancel
              </Button>
              <Button variant="primary" busy={busy} busyLabel="Saving..." onClick={() => void save()}>
                Save conclusion
              </Button>
            </div>
          </div>
        </div>
      ) : has ? (
        <div className="conclusion-view">
          <p className="conclusion-text">{inv.conclusion}</p>
          <ConfidenceMeter confidence={inv.conclusion_confidence} size="lg" label="long" />
          <VerificationBadge verification={inv.conclusion_verification || 'ai_generated'} />
          {inv.conclusion_verification === 'disputed' && <Banner>The conclusion is disputed. Review the contradictory claims and their evidence before relying on it.</Banner>}
          {(inv.conclusion_supporting_claims?.length ?? 0) > 0 && <details className="conclusion-basis"><summary>Supporting claims: {claimSummary(inv.conclusion_supporting_claims)}</summary><ul>{inv.conclusion_supporting_claims.map((ref) => <li key={ref}>{claimLabel(ref)}</li>)}</ul></details>}
          {(inv.conclusion_contradicting_claims?.length ?? 0) > 0 && <details className="conclusion-basis"><summary>Contradicting claims: {claimSummary(inv.conclusion_contradicting_claims)}</summary><ul>{inv.conclusion_contradicting_claims.map((ref) => <li key={ref}>{claimLabel(ref)}</li>)}</ul></details>}
          {!(inv.conclusion_supporting_claims?.length || inv.conclusion_contradicting_claims?.length) && <p className="conclusion-review-record">No claims selected as the basis.</p>}
          {inv.conclusion_reviewed_at ? <p className="conclusion-review-record">Reviewed <time dateTime={inv.conclusion_reviewed_at} title={inv.conclusion_reviewed_at}>{formatAbsolute(inv.conclusion_reviewed_at)}</time>.</p> : <p className="conclusion-review-record">No review has been recorded.</p>}
          {(inv.conclusion_needs_review || !inv.conclusion_reviewed_at) && <Banner>{inv.conclusion_verification === 'disputed' ? 'Review needed: the conclusion is disputed.' : inv.conclusion_reviewed_at ? 'Selected claims or their evidence changed since this review. The verification label still describes the conclusion statement that was checked.' : 'Review needed: check the conclusion against its selected claims and evidence.'}</Banner>}
          {summary && <p className="conclusion-summary" role="alert">{summary}</p>}
          {reviewSnapshot ? <div className="conclusion-review-form">
            {reviewSnapshot.conclusion_review_token !== inv.conclusion_review_token && <Banner>This conclusion or its basis changed in another app. Cancel and review the latest records.</Banner>}
            <TextArea label="What did you check?" value={reviewBasis} onChange={setReviewBasis} error={fieldErrors.review_basis} note={nearLimit(reviewBasis, LIMITS.conclusionRationale)} />
            <div className="conclusion-actions"><Button variant="secondary" onClick={() => { setReviewSnapshot(null); setReviewBasis(''); setSummary(undefined) }} disabled={busy}>Cancel review</Button>
              <Button variant="primary" onClick={() => void recordReview()} busy={busy} disabled={!reviewBasis.trim() || reviewSnapshot.conclusion_review_token !== inv.conclusion_review_token}>Verify and record review</Button></div>
          </div> : <Button variant="secondary" onClick={() => { setReviewSnapshot(inv); setReviewBasis(''); setSummary(undefined) }}>Review conclusion</Button>}
          {(inv.conclusion_review_basis || inv.conclusion_rationale.trim()) && <details className="conclusion-basis">
            <summary>Review and confidence details</summary>
            {inv.conclusion_review_basis && <p className="conclusion-rationale">{inv.conclusion_review_basis}</p>}
            {inv.conclusion_rationale.trim() !== '' && <p className="conclusion-rationale"><span className="conclusion-why">Why:</span> {inv.conclusion_rationale}</p>}
          </details>}
        </div>
      ) : (
        <div className="conclusion-empty">
          <p className="conclusion-empty-text">No conclusion yet. When the evidence points somewhere, write it here.</p>
          <Button variant="ghost" icon={Pencil} ref={editButtonRef} onClick={begin}>
            Write a conclusion
          </Button>
        </div>
      )}

      {confirmClear && (
        <ConfirmDialog
          title="Clear the current conclusion?"
          body="This removes the conclusion, its confidence and its rationale. The history keeps a record that it changed."
          confirmLabel="Clear conclusion"
          busy={clearing}
          busyLabel="Clearing..."
          onConfirm={() => void clear()}
          onCancel={() => setConfirmClear(false)}
        />
      )}
    </section>
  )
}
