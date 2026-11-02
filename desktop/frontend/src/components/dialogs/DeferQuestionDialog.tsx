// Defer a question (UI-11; UI-SPEC Copywriting, Defer row): an optional note
// on why it can wait.
import { useRef, useState } from 'react'
import { api } from '../../api/api'
import type { Question } from '../../api/types'
import { LIMITS, nearLimit } from '../../lib/bytes'
import { splitFieldErrors } from '../../lib/formErrors'
import { useWrite } from '../../state/useWrite'
import { Dialog } from '../ui/Dialog'
import { TextArea } from '../ui/TextArea'

export interface DeferQuestionDialogProps {
  invRef: string
  question: Question
  onClose: () => void
}

export function DeferQuestionDialog({ invRef, question, onClose }: DeferQuestionDialogProps) {
  const write = useWrite()
  const [note, setNote] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | undefined>()
  const [summary, setSummary] = useState<string | undefined>()

  // Retry toasts call the latest submit, so a retry sends what the form
  // holds now and sees whether a submit is already running.
  const submitRef = useRef<() => Promise<void>>(async () => {})
  const submit = async () => {
    if (busy) return
    setBusy(true)
    setError(undefined)
    setSummary(undefined)
    try {
      const saved = await write(() => api.deferQuestion(invRef, question.ref, note), {
        onRetry: () => void submitRef.current(),
      })
      if (saved) onClose()
    } catch (e) {
      // The engine names the defer note field "note".
      const split = splitFieldErrors(e, ['note'])
      setError(split.fieldErrors.note)
      setSummary(split.summary)
    } finally {
      setBusy(false)
    }
  }

  submitRef.current = submit

  return (
    <Dialog
      title={`Defer ${question.ref}`}
      size="confirm"
      onClose={onClose}
      onSubmit={() => void submit()}
      submitLabel="Defer question"
      busyLabel="Deferring..."
      busy={busy}
      dirty={note !== ''}
      summary={
        summary ? (
          <p className="dialog-summary" role="alert">
            {summary}
          </p>
        ) : undefined
      }
    >
      <p className="dialog-text dialog-quote">{question.question}</p>
      <TextArea
        label="Note (optional)"
        hint="Why it can wait"
        value={note}
        onChange={setNote}
        error={error}
        note={nearLimit(note, LIMITS.deferNote)}
        spellCheck
      />
    </Dialog>
  )
}
