// Answer a follow-up question. The answer is required, so an empty one is
// caught here before any call.
import { useRef, useState } from 'react'
import { api } from '../../api/api'
import { REQUIRED_COPY } from '../../api/errors'
import type { Question } from '../../api/types'
import { LIMITS, nearLimit } from '../../lib/bytes'
import { splitFieldErrors } from '../../lib/formErrors'
import { useWrite } from '../../state/useWrite'
import { Dialog } from '../ui/Dialog'
import { TextArea } from '../ui/TextArea'

export interface ResolveQuestionDialogProps {
  invRef: string
  question: Question
  onClose: () => void
}

export function ResolveQuestionDialog({ invRef, question, onClose }: ResolveQuestionDialogProps) {
  const write = useWrite()
  const [text, setText] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | undefined>()
  const [summary, setSummary] = useState<string | undefined>()

  // Retry toasts call the latest submit, so a retry sends what the form
  // holds now and sees whether a submit is already running.
  const submitRef = useRef<() => Promise<void>>(async () => {})
  const submit = async () => {
    if (busy) return
    if (text.trim() === '') {
      setError(REQUIRED_COPY)
      return
    }
    setBusy(true)
    setError(undefined)
    setSummary(undefined)
    try {
      const saved = await write(() => api.resolveQuestion(invRef, question.ref, text), {
        onRetry: () => void submitRef.current(),
      })
      if (saved) onClose()
    } catch (e) {
      const split = splitFieldErrors(e, ['resolution'])
      setError(split.fieldErrors.resolution)
      setSummary(split.summary)
    } finally {
      setBusy(false)
    }
  }

  submitRef.current = submit

  return (
    <Dialog
      title="Answer this follow-up question"
      size="confirm"
      onClose={onClose}
      onSubmit={() => void submit()}
      submitLabel="Save answer"
      busyLabel="Saving..."
      busy={busy}
      dirty={text !== ''}
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
        label="Answer"
        hint="What is the answer, and how do you know?"
        value={text}
        onChange={(v) => {
          setText(v)
          if (error && v.trim() !== '') setError(undefined)
        }}
        error={error}
        note={nearLimit(text, LIMITS.resolution)}
        spellCheck
      />
    </Dialog>
  )
}
