// Add something to find out, or reword one. One field, no choice: it is
// saved as an open question under Still to find out.
import { useRef, useState } from 'react'
import { api } from '../../api/api'
import type { Question } from '../../api/types'
import { LIMITS, nearLimit } from '../../lib/bytes'
import { splitFieldErrors } from '../../lib/formErrors'
import { useWrite } from '../../state/useWrite'
import { Dialog } from '../ui/Dialog'
import { TextArea } from '../ui/TextArea'

export const ADD_TO_FIND_OUT = 'Add something to find out'

export interface QuestionDialogProps {
  invRef: string
  /** The one to reword; absent to add one. */
  question?: Question
  onClose: () => void
  /** Called with the code of what was added before the dialog closes (add only). */
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
      const saved = await write<{ ref: string }>(
        () =>
          question ? api.updateQuestion(invRef, question.ref, text) : api.addQuestion(invRef, { question: text }),
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
      title={question ? 'Edit what to find out' : ADD_TO_FIND_OUT}
      size="confirm"
      onClose={onClose}
      onSubmit={() => void submit()}
      submitLabel={question ? 'Save changes' : 'Add'}
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
      <div className="dialog-fields">
        <TextArea
          label="What do you still need to find out?"
          value={text}
          onChange={setText}
          error={error}
          note={nearLimit(text, LIMITS.question)}
          spellCheck
        />
      </div>
    </Dialog>
  )
}
