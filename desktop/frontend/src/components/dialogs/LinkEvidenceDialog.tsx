// Link evidence (UI-09, UI-10; UI-SPEC Copywriting, Link evidence row and
// Empty states): a radio list of the investigation's evidence (ref, title,
// source label) where evidence already linked to this record is disabled and
// says how it is linked, then the relation (Supports by default). Linking
// again with the other relation is done from the drawer (Mark as ...), so
// linked items stay disabled here.
import { useId, useRef, useState } from 'react'
import { CircleAlert, Plus } from 'lucide-react'
import { api } from '../../api/api'
import type { Claim, Evidence, Hypothesis, Relation } from '../../api/types'
import { splitFieldErrors } from '../../lib/formErrors'
import { useWrite } from '../../state/useWrite'
import { Button } from '../ui/Button'
import { Dialog } from '../ui/Dialog'
import { SegmentedControl } from '../ui/SegmentedControl'
import './LinkEvidenceDialog.css'

export interface LinkEvidenceDialogProps {
  kind: 'claim' | 'hypothesis'
  record: Claim | Hypothesis
  /** Every piece of evidence in the investigation, by ref. */
  evidence: Evidence[]
  invRef: string
  onClose: () => void
  /** Offered when the investigation has no evidence yet. */
  onAddEvidence?: () => void
  /** Evidence to select when the dialog opens (one that was just added). */
  initialRef?: string
}

const RELATIONS: { value: Relation; label: string }[] = [
  { value: 'supports', label: 'Supports' },
  { value: 'contradicts', label: 'Contradicts' },
]

export function LinkEvidenceDialog({
  kind,
  record,
  evidence,
  invRef,
  onClose,
  onAddEvidence,
  initialRef,
}: LinkEvidenceDialogProps) {
  const write = useWrite()
  const name = useId()
  const errorId = useId()
  const linked = new Map(record.links.map((l) => [l.evidence_ref, l.relation]))
  const available = evidence.filter((e) => !linked.has(e.ref))
  const [chosen, setChosen] = useState<string | null>(
    initialRef && available.some((e) => e.ref === initialRef) ? initialRef : null,
  )
  const [relation, setRelation] = useState<Relation>('supports')
  const [busy, setBusy] = useState(false)
  const [choiceError, setChoiceError] = useState<string | undefined>()
  const [summary, setSummary] = useState<string | undefined>()
  const firstRef = useRef<HTMLInputElement>(null)
  const title = `Link evidence to ${record.ref}`

  if (evidence.length === 0 || available.length === 0) {
    const noEvidence = evidence.length === 0
    return (
      <Dialog
        title={title}
        onClose={onClose}
        footer={
          <>
            <Button variant="secondary" onClick={onClose}>
              Close
            </Button>
            {noEvidence && onAddEvidence && (
              <Button variant="primary" icon={Plus} onClick={onAddEvidence}>
                Add evidence
              </Button>
            )}
          </>
        }
      >
        <p className="dialog-text">
          {noEvidence
            ? 'This case has no evidence yet. Add evidence first.'
            : `Every piece of evidence is already linked to ${record.ref}.`}
        </p>
      </Dialog>
    )
  }

  // Retry toasts call the latest submit, so a retry sends what the form
  // holds now and sees whether a submit is already running.
  const submitRef = useRef<() => Promise<void>>(async () => {})
  const submit = async () => {
    if (busy) return
    if (!chosen) {
      setChoiceError('Choose the evidence to link.')
      firstRef.current?.focus()
      return
    }
    setBusy(true)
    setChoiceError(undefined)
    setSummary(undefined)
    try {
      const saved = await write<Claim | Hypothesis>(
        () =>
          kind === 'claim'
            ? api.linkEvidenceToClaim(invRef, record.ref, chosen, relation)
            : api.linkEvidenceToHypothesis(invRef, record.ref, chosen, relation),
        { rethrowConflict: true, onRetry: () => void submitRef.current() },
      )
      if (saved) onClose()
    } catch (e) {
      const split = splitFieldErrors(e, [])
      setSummary(split.summary)
    } finally {
      setBusy(false)
    }
  }

  submitRef.current = submit

  return (
    <Dialog
      title={title}
      onClose={onClose}
      onSubmit={() => void submit()}
      submitLabel="Link evidence"
      busyLabel="Linking..."
      busy={busy}
      initialFocus={firstRef}
      summary={
        summary ? (
          <p className="dialog-summary" role="alert">
            {summary}
          </p>
        ) : undefined
      }
    >
      <fieldset
        className={['choice-field', choiceError ? 'is-invalid' : ''].filter(Boolean).join(' ')}
        aria-describedby={choiceError ? errorId : undefined}
      >
        <legend className="field-label">Evidence</legend>
        <div className="choice-list">
          {evidence.map((e) => {
            const as = linked.get(e.ref)
            const isFirst = e.ref === (chosen ?? available[0]?.ref)
            return (
              <label
                key={e.ref}
                className={['choice', as ? 'is-disabled' : '', chosen === e.ref ? 'is-selected' : ''].filter(Boolean).join(' ')}
              >
                <input
                  ref={isFirst ? firstRef : undefined}
                  type="radio"
                  name={name}
                  value={e.ref}
                  checked={chosen === e.ref}
                  disabled={Boolean(as)}
                  onChange={() => {
                    setChosen(e.ref)
                    setChoiceError(undefined)
                  }}
                />
                <span className="choice-body">
                  <span className="choice-line">
                    <span className="choice-ref tabular">{e.ref}</span>
                    {e.source_label && <span className="choice-meta">{e.source_label}</span>}
                  </span>
                  <span className="choice-title">{e.title}</span>
                  {as && (
                    <span className="choice-note">
                      {`Already linked as ${as === 'supports' ? 'supporting' : 'contradicting'}`}
                    </span>
                  )}
                </span>
              </label>
            )
          })}
        </div>
        {choiceError && (
          <p className="field-error" id={errorId}>
            <CircleAlert size={16} strokeWidth={1.75} aria-hidden="true" />
            <span>{choiceError}</span>
          </p>
        )}
      </fieldset>
      <SegmentedControl<Relation> label="Relation" options={RELATIONS} value={relation} onChange={setRelation} />
    </Dialog>
  )
}
