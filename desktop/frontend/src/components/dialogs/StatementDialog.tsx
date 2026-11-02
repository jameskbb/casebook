// Add a claim or hypothesis (UI-08, UI-10; UI-SPEC Copywriting, Add claim
// and Add hypothesis rows): the statement, a confidence picker (Not assessed
// by default), the rationale, and one row per piece of evidence with None,
// Supports or Contradicts, so the record and its links land in one call.
// An error on links[i] shows on the row of the i-th linked evidence.
import { useRef, useState } from 'react'
import { api } from '../../api/api'
import { toApiError } from '../../api/errors'
import type { Claim, Confidence, Evidence, EvidenceLinkInput, Hypothesis, Relation } from '../../api/types'
import { LIMITS, nearLimit } from '../../lib/bytes'
import { splitFieldErrors } from '../../lib/formErrors'
import { confidenceLabels, confidenceOptions } from '../../lib/labels'
import { useWrite } from '../../state/useWrite'
import { Dialog } from '../ui/Dialog'
import { SegmentedControl } from '../ui/SegmentedControl'
import { TextArea } from '../ui/TextArea'
import './StatementDialog.css'

type Choice = 'none' | Relation

const CHOICES: { value: Choice; label: string }[] = [
  { value: 'none', label: 'None' },
  { value: 'supports', label: 'Supports' },
  { value: 'contradicts', label: 'Contradicts' },
]

const LINK_FIELD = /^links\[(\d+)\]\./

export interface StatementDialogProps {
  kind: 'claim' | 'hypothesis'
  invRef: string
  /** Every piece of evidence in the investigation, by ref. */
  evidence: Evidence[]
  onClose: () => void
  /** Called with the new record's ref before the dialog closes. */
  onCreated?: (ref: string) => void
}

export function StatementDialog({ kind, invRef, evidence, onClose, onCreated }: StatementDialogProps) {
  const write = useWrite()
  const [statement, setStatement] = useState('')
  const [confidence, setConfidence] = useState<Confidence>('unknown')
  const [rationale, setRationale] = useState('')
  const [choices, setChoices] = useState<Record<string, Choice>>({})
  const [busy, setBusy] = useState(false)
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({})
  const [rowErrors, setRowErrors] = useState<Record<string, string>>({})
  const [summary, setSummary] = useState<string | undefined>()
  const rationaleField = kind === 'claim' ? 'confidence_rationale' : 'rationale'
  const noun = kind === 'claim' ? 'claim' : 'hypothesis'

  const links: EvidenceLinkInput[] = evidence
    .filter((e) => (choices[e.ref] ?? 'none') !== 'none')
    .map((e) => ({ evidence_ref: e.ref, relation: choices[e.ref] as Relation }))
  const dirty = statement !== '' || rationale !== '' || confidence !== 'unknown' || links.length > 0

  // Retry toasts call the latest submit, so a retry sends what the form
  // holds now and sees whether a submit is already running.
  const submitRef = useRef<() => Promise<void>>(async () => {})
  const submit = async () => {
    if (busy) return
    const sent = links
    setBusy(true)
    setFieldErrors({})
    setRowErrors({})
    setSummary(undefined)
    try {
      const saved = await write<Claim | Hypothesis>(
        () =>
          kind === 'claim'
            ? api.addClaim(invRef, { statement, confidence, confidence_rationale: rationale, links: sent })
            : api.addHypothesis(invRef, { statement, confidence, rationale, links: sent }),
        { onRetry: () => void submitRef.current() },
      )
      if (saved) {
        onCreated?.(saved.ref)
        onClose()
      }
    } catch (e) {
      const err = toApiError(e)
      const m = err.kind === 'validation' && err.field ? LINK_FIELD.exec(err.field) : null
      const row = m ? sent[Number(m[1])] : undefined
      if (row) {
        setRowErrors({ [row.evidence_ref]: err.message })
      } else {
        const split = splitFieldErrors(err, ['statement', 'confidence', rationaleField])
        setFieldErrors(split.fieldErrors)
        setSummary(split.summary)
      }
    } finally {
      setBusy(false)
    }
  }

  submitRef.current = submit

  return (
    <Dialog
      title={`Add ${noun}`}
      onClose={onClose}
      onSubmit={() => void submit()}
      submitLabel={`Add ${noun}`}
      busyLabel="Adding..."
      busy={busy}
      dirty={dirty}
      summary={
        summary ? (
          <p className="dialog-summary" role="alert">
            {summary}
          </p>
        ) : undefined
      }
    >
      <div className="dialog-fields">
        <TextArea
          label="Statement"
          hint={kind === 'claim' ? 'What do we believe? One clear sentence.' : 'What might explain it?'}
          value={statement}
          onChange={setStatement}
          error={fieldErrors.statement}
          note={nearLimit(statement, LIMITS.statement)}
          className="statement-input"
          spellCheck
        />
        <SegmentedControl<Confidence>
          label="Confidence"
          options={confidenceOptions.map((c) => ({ value: c, label: confidenceLabels[c] }))}
          value={confidence}
          onChange={setConfidence}
          error={fieldErrors.confidence}
        />
        <TextArea
          label="Why this confidence (optional)"
          value={rationale}
          onChange={setRationale}
          error={fieldErrors[rationaleField]}
          note={nearLimit(rationale, LIMITS.rationale)}
          spellCheck
        />
        <fieldset className="statement-links">
          <legend className="field-label">Evidence</legend>
          {evidence.length === 0 ? (
            <p className="statement-links-empty">No evidence to link yet. You can link it later.</p>
          ) : (
            <ul className="statement-link-list">
              {evidence.map((e) => (
                <li key={e.ref} className="statement-link-row" data-ref={e.ref}>
                  <span className="statement-link-text">
                    <span className="statement-link-ref tabular">{e.ref}</span>
                    <span className="statement-link-title">{e.title}</span>
                  </span>
                  <SegmentedControl<Choice>
                    label={`${e.ref}: ${e.title}`}
                    hideLabel
                    options={CHOICES}
                    value={choices[e.ref] ?? 'none'}
                    onChange={(c) => {
                      setChoices((prev) => ({ ...prev, [e.ref]: c }))
                      setRowErrors((prev) => {
                        if (!prev[e.ref]) return prev
                        const next = { ...prev }
                        delete next[e.ref]
                        return next
                      })
                    }}
                    error={rowErrors[e.ref]}
                    className="statement-link-choice"
                  />
                </li>
              ))}
            </ul>
          )}
        </fieldset>
      </div>
    </Dialog>
  )
}
