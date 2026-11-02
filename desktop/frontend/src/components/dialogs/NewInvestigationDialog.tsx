// New investigation (UI-04; UI-SPEC Copywriting, New investigation row).
import { useEffect, useRef, useState } from 'react'
import { api } from '../../api/api'
import { LIMITS, nearLimit } from '../../lib/bytes'
import { splitFieldErrors } from '../../lib/formErrors'
import { useAppData } from '../../state/appData'
import { useWrite } from '../../state/useWrite'
import { Checkbox } from '../ui/Checkbox'
import { Dialog } from '../ui/Dialog'
import { TextArea } from '../ui/TextArea'
import { TextField } from '../ui/TextField'

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
  const [makeActive, setMakeActive] = useState(() => !investigations.some((i) => i.is_active))
  const [busy, setBusy] = useState(false)
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({})
  const [summary, setSummary] = useState<string | undefined>()
  const [errorTick, setErrorTick] = useState(0)
  const fieldsRef = useRef<HTMLDivElement>(null)
  const submitRef = useRef<() => Promise<void>>(async () => {})

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
          const created = await api.createInvestigation({ title, problem_statement: question, make_active: makeActive })
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
      title="New case"
      onClose={onClose}
      onSubmit={() => void submit()}
      submitLabel="Create case"
      busyLabel="Creating..."
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
        <TextField
          label="Title"
          hint="A short name you'll recognize, like Northeast margin down 6 points in Q3"
          value={title}
          onChange={setTitle}
          error={fieldErrors.title}
          note={nearLimit(title, LIMITS.title)}
          autoComplete="off"
          spellCheck
        />
        <TextArea
          label="Primary question (optional)"
          hint="What are we trying to figure out?"
          value={question}
          onChange={setQuestion}
          error={fieldErrors.problem_statement}
          note={nearLimit(question, LIMITS.problemStatement)}
          spellCheck
        />
        <Checkbox label="Make this your AI's current case" checked={makeActive} onChange={setMakeActive} />
      </div>
    </Dialog>
  )
}
