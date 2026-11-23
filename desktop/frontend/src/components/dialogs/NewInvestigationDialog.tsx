// New question. "Have my AI work on this question" is offered only when an
// AI tool is connected, and is off otherwise.
import { useEffect, useRef, useState } from 'react'
import { api } from '../../api/api'
import { LIMITS, nearLimit } from '../../lib/bytes'
import { splitFieldErrors } from '../../lib/formErrors'
import { anyClientConnected } from '../../lib/clients'
import { useAppData } from '../../state/appData'
import { useWrite } from '../../state/useWrite'
import { Checkbox } from '../ui/Checkbox'
import { Dialog } from '../ui/Dialog'
import { QuestionField, TextArea } from '../ui/TextArea'

const FIELDS = ['title', 'problem_statement'] as const

export interface NewInvestigationDialogProps {
  onClose: () => void
  /** Called with the new ref after it is created and opened. */
  onCreated: (ref: string) => void
}

export function NewInvestigationDialog({ onClose, onCreated }: NewInvestigationDialogProps) {
  const { investigations, select } = useAppData()
  const write = useWrite()
  const [title, setTitle] = useState('')
  const [question, setQuestion] = useState('')
  const [aiConnected, setAiConnected] = useState(false)
  const [makeActive, setMakeActive] = useState(false)
  const [busy, setBusy] = useState(false)
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({})
  const [summary, setSummary] = useState<string | undefined>()
  const [errorTick, setErrorTick] = useState(0)
  const fieldsRef = useRef<HTMLDivElement>(null)
  const submitRef = useRef<() => Promise<void>>(async () => {})

  // Ticked once an AI tool turns out to be connected, unless the AI is already on another question.
  useEffect(() => {
    let live = true
    api.clientConnections().then(
      (c) => {
        if (!live || !anyClientConnected(c)) return
        setAiConnected(true)
        setMakeActive(!investigations.some((i) => i.is_active))
      },
      () => {},
    )
    return () => {
      live = false
    }
    // Asked once, when the window opens.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  // The first invalid field takes focus after a failed submit.
  useEffect(() => {
    if (errorTick === 0) return
    fieldsRef.current?.querySelector<HTMLElement>('[aria-invalid="true"]')?.focus()
  }, [errorTick])

  const submit = async () => {
    setBusy(true)
    setFieldErrors({})
    setSummary(undefined)
    try {
      const inv = await write(
        async () => {
          const created = await api.createInvestigation({ title, problem_statement: question, make_active: aiConnected && makeActive })
          select(created.ref)
          return created
        },
        // Retry re-submits the form so a successful retry opens the result.
        { onRetry: () => void submitRef.current() },
      )
      if (inv) {
        onCreated(inv.ref)
        onClose()
        return
      }
    } catch (e) {
      const split = splitFieldErrors(e, FIELDS)
      setFieldErrors(split.fieldErrors)
      setSummary(split.summary)
      setErrorTick((n) => n + 1)
    }
    setBusy(false)
  }
  submitRef.current = submit

  return (
    <Dialog
      title="New question"
      onClose={onClose}
      onSubmit={() => void submit()}
      submitLabel="Add question"
      busyLabel="Adding..."
      busy={busy}
      dirty={title !== '' || question !== ''}
      summary={
        summary ? (
          <p className="dialog-summary" role="alert">
            {summary}
          </p>
        ) : undefined
      }
    >
      <div className="dialog-fields" ref={fieldsRef}>
        <QuestionField
          hint="What do you want to know? For example: Why did calls go up when sales stayed the same?"
          value={title}
          onChange={setTitle}
          error={fieldErrors.title}
          note={nearLimit(title, LIMITS.title)}
        />
        <TextArea
          label="Background (optional)"
          hint="Which months, the numbers, and where to look."
          value={question}
          onChange={setQuestion}
          error={fieldErrors.problem_statement}
          note={nearLimit(question, LIMITS.problemStatement)}
          spellCheck
        />
        {aiConnected && <Checkbox label="Have my AI work on this question" checked={makeActive} onChange={setMakeActive} />}
      </div>
    </Dialog>
  )
}
