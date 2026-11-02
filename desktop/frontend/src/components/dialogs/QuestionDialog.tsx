// Add or edit a question (UI-11; UI-SPEC Copywriting, Add question and Edit
// question rows): one field, "What still needs an answer?".
import { useRef, useState } from 'react'
import { api } from '../../api/api'
import type { Question } from '../../api/types'
import { LIMITS, nearLimit } from '../../lib/bytes'
import { splitFieldErrors } from '../../lib/formErrors'
import { useWrite } from '../../state/useWrite'
import { Dialog } from '../ui/Dialog'
import { TextArea } from '../ui/TextArea'

export interface QuestionDialogProps {
  invRef: string
  /** The question to edit; absent to add one. */
  question?: Question
  onClose: () => void
  /** Called with the new question's ref before the dialog closes (add only). */
  onCreated?: (ref: string) => void
}

export function QuestionDialog({ invRef, question, onClose, onCreated }: QuestionDialogProps) {
  const write = useWrite()
  const initial = question?.question ?? ''
  const [text, setText] = useState(initial)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | undefined>()
  const [summary, setSummary] = useState<string | undefined>()

  // Retry toasts call the latest submit, so a retry sends what the form
  // holds now and sees whether a submit is already running.
  const submitRef = useRef<() => Promise<void>>(async () => {})
  const submit = async () => {
    if (busy) return
    if (question && text === initial) {
      onClose()
      return
    }
    setBusy(true)
    setError(undefined)
    setSummary(undefined)
    try {
      const saved = await write(
        () => (question ? api.updateQuestion(invRef, question.ref, text) : api.addQuestion(invRef, { question: text })),
        { onRetry: () => void submitRef.current() },
      )
      if (saved) {
        if (!question) onCreated?.(saved.ref)
        onClose()
      }
    } catch (e) {
      const split = splitFieldErrors(e, ['question'])
      setError(split.fieldErrors.question)
      setSummary(split.summary)
    } finally {
      setBusy(false)
    }
  }

  submitRef.current = submit

  return (
    <Dialog
      title={question ? `Edit question ${question.ref}` : 'Add question'}
      size="confirm"
      onClose={onClose}
      onSubmit={() => void submit()}
      submitLabel={question ? 'Save changes' : 'Add question'}
      busyLabel={question ? 'Saving...' : 'Adding...'}
      busy={busy}
      dirty={text !== initial}
      summary={
        summary ? (
          <p className="dialog-summary" role="alert">
            {summary}
          </p>
        ) : undefined
      }
    >
      <TextArea
        label="Question"
        hint="What still needs an answer?"
        value={text}
        onChange={setText}
        error={error}
        note={nearLimit(text, LIMITS.question)}
        spellCheck
      />
    </Dialog>
  )
}
