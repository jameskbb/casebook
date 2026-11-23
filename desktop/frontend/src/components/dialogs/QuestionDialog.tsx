// Add something to find out, or edit a follow-up question. Adding asks one
// thing, "What do you still need to find out?", and saves it as a follow-up
// question unless the person says it is a guess to test, which saves it as a
// finding not tested yet.
import { useRef, useState } from 'react'
import { api } from '../../api/api'
import type { Question } from '../../api/types'
import { LIMITS, nearLimit } from '../../lib/bytes'
import { splitFieldErrors } from '../../lib/formErrors'
import { useWrite } from '../../state/useWrite'
import { Dialog } from '../ui/Dialog'
import { SegmentedControl } from '../ui/SegmentedControl'
import { TextArea } from '../ui/TextArea'

export const ADD_TO_FIND_OUT = 'Add something to find out'

type Kind = 'question' | 'guess'

const KINDS: { value: Kind; label: string }[] = [
  { value: 'question', label: 'A question to answer' },
  { value: 'guess', label: 'A guess to test' },
]

export interface QuestionDialogProps {
  invRef: string
  /** The follow-up question to edit; absent to add something to find out. */
  question?: Question
  onClose: () => void
  /** Called with the code of what was added before the dialog closes (add only). */
  onCreated?: (ref: string) => void
}

export function QuestionDialog({ invRef, question, onClose, onCreated }: QuestionDialogProps) {
  const write = useWrite()
  const initial = question?.question ?? ''
  const [text, setText] = useState(initial)
  const [kind, setKind] = useState<Kind>('question')
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
          question
            ? api.updateQuestion(invRef, question.ref, text)
            : kind === 'guess'
              ? api.addHypothesis(invRef, { statement: text, confidence: 'unknown', rationale: '', links: [] })
              : api.addQuestion(invRef, { question: text }),
        { onRetry: () => void submitRef.current() },
      )
      if (saved) {
        if (!question) onCreated?.(saved.ref)
        onClose()
      }
    } catch (e) {
      const split = splitFieldErrors(e, ['question', 'statement'])
      setError(split.fieldErrors.question ?? split.fieldErrors.statement)
      setSummary(split.summary)
    } finally {
      setBusy(false)
    }
  }

  submitRef.current = submit

  return (
    <Dialog
      title={question ? 'Edit follow-up question' : ADD_TO_FIND_OUT}
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
          label={question ? 'Follow-up question' : 'What do you still need to find out?'}
          value={text}
          onChange={setText}
          error={error}
          note={nearLimit(text, LIMITS.question)}
          spellCheck
        />
        {!question && <SegmentedControl<Kind> label="Is this a guess to test or a question to answer?" options={KINDS} value={kind} onChange={setKind} />}
      </div>
    </Dialog>
  )
}
