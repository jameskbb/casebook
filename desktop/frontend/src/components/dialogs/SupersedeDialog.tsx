// Replace a finding: choose the newer finding that takes its place. Only
// another finding that was not itself replaced can (the engine refuses
// anything else). No extra confirmation: choosing the newer one is the
// confirmation, and the old one stays under Show replaced findings.
import { useId, useRef, useState } from 'react'
import { CircleAlert } from 'lucide-react'
import { api } from '../../api/api'
import type { Claim, Hypothesis } from '../../api/types'
import { splitFieldErrors } from '../../lib/formErrors'
import { useWrite } from '../../state/useWrite'
import { Button } from '../ui/Button'
import { Dialog } from '../ui/Dialog'
import './LinkEvidenceDialog.css'
import './SupersedeDialog.css'

export type SupersedeDialogProps = {
  invRef: string
  onClose: () => void
} & (
  | { kind: 'claim'; record: Claim; records: Claim[] }
  | { kind: 'hypothesis'; record: Hypothesis; records: Hypothesis[] }
)

export function SupersedeDialog(props: SupersedeDialogProps) {
  const { kind, record, invRef, onClose } = props
  const write = useWrite()
  const name = useId()
  const errorId = useId()
  const noun = 'finding'
  const records: (Claim | Hypothesis)[] = props.records
  const candidates = records.filter((r) => r.ref !== record.ref && r.status !== 'superseded')
  const [chosen, setChosen] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const [choiceError, setChoiceError] = useState<string | undefined>()
  const [summary, setSummary] = useState<string | undefined>()
  const firstRef = useRef<HTMLInputElement>(null)
  const title = 'Replace with a newer finding'

  if (candidates.length === 0) {
    return (
      <Dialog
        title={title}
        size="confirm"
        onClose={onClose}
        footer={
          <Button variant="secondary" onClick={onClose}>
            Close
          </Button>
        }
      >
        <p className="dialog-text">
          {`There is no other ${noun} to replace it with. Add the new ${noun} first, then replace this one.`}
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
      setChoiceError(`Choose the ${noun} that replaces it.`)
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
            ? api.supersedeClaim(invRef, record.ref, chosen)
            : api.supersedeHypothesis(invRef, record.ref, chosen),
        { rethrowConflict: true, onRetry: () => void submitRef.current() },
      )
      if (saved) onClose()
    } catch (e) {
      setSummary(splitFieldErrors(e, []).summary)
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
      submitLabel={`Replace ${noun}`}
      busyLabel="Replacing..."
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
      <p className="dialog-text">
        {`Choose the ${noun} that replaces it. The old one stays here, crossed out.`}
      </p>
      <fieldset
        className={['choice-field', choiceError ? 'is-invalid' : ''].filter(Boolean).join(' ')}
        aria-describedby={choiceError ? errorId : undefined}
      >
        <legend className="sr-only">The newer finding</legend>
        <div className="choice-list">
          {candidates.map((c, i) => (
            <label key={c.ref} className={['choice', chosen === c.ref ? 'is-selected' : ''].filter(Boolean).join(' ')}>
              <input
                ref={i === 0 ? firstRef : undefined}
                type="radio"
                name={name}
                value={c.ref}
                checked={chosen === c.ref}
                onChange={() => {
                  setChosen(c.ref)
                  setChoiceError(undefined)
                }}
              />
              <span className="choice-body">
                <span className="choice-ref tabular">{c.ref}</span>
                <span className="choice-statement">{c.statement}</span>
              </span>
            </label>
          ))}
        </div>
        {choiceError && (
          <p className="field-error" id={errorId}>
            <CircleAlert size={16} strokeWidth={1.75} aria-hidden="true" />
            <span>{choiceError}</span>
          </p>
        )}
      </fieldset>
    </Dialog>
  )
}
