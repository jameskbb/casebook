// Edit title and question (UI-06; UI-SPEC Copywriting, Edit investigation
// row). Sends an InvestigationPatch with only the fields that changed; no
// change closes without a call. Validation errors go under their field.
import { useEffect, useRef, useState } from 'react'
import { api } from '../../api/api'
import type { Investigation, InvestigationPatch } from '../../api/types'
import { LIMITS, nearLimit } from '../../lib/bytes'
import { splitFieldErrors } from '../../lib/formErrors'
import { useChangedElsewhere } from '../../hooks/useChangedElsewhere'
import { useWrite } from '../../state/useWrite'
import { ChangedElsewhereBanner } from '../ui/ChangedElsewhereBanner'
import { Dialog } from '../ui/Dialog'
import { TextArea } from '../ui/TextArea'
import { TextField } from '../ui/TextField'

const FIELDS = ['title', 'problem_statement'] as const

export interface EditInvestigationDialogProps {
  investigation: Investigation
  onClose: () => void
}

export function EditInvestigationDialog({ investigation: inv, onClose }: EditInvestigationDialogProps) {
  const write = useWrite()
  // The values the dialog opened with; later reloads do not reset the form.
  // When they change in another app while the form holds input, the dialog
  // says so and offers Show latest (AR-23).
  const [initial, setInitial] = useState(() => ({ title: inv.title, problem_statement: inv.problem_statement }))
  const stored = { title: inv.title, problem_statement: inv.problem_statement }
  const [title, setTitle] = useState(initial.title)
  const [question, setQuestion] = useState(initial.problem_statement)
  const [busy, setBusy] = useState(false)
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({})
  const [summary, setSummary] = useState<string | undefined>()
  const [errorTick, setErrorTick] = useState(0)
  const fieldsRef = useRef<HTMLDivElement>(null)
  const submitRef = useRef<() => Promise<void>>(async () => {})

  useEffect(() => {
    if (errorTick === 0) return
    fieldsRef.current?.querySelector<HTMLElement>('[aria-invalid="true"]')?.focus()
  }, [errorTick])

  const patch: InvestigationPatch = {}
  if (title !== initial.title) patch.title = title
  if (question !== initial.problem_statement) patch.problem_statement = question
  const dirty = Object.keys(patch).length > 0
  const changedElsewhere =
    useChangedElsewhere(initial, stored, dirty) && !busy && (stored.title !== title || stored.problem_statement !== question)
  const showLatest = () => {
    setInitial(stored)
    setTitle(stored.title)
    setQuestion(stored.problem_statement)
    setFieldErrors({})
    setSummary(undefined)
  }

  const submit = async () => {
    if (!dirty) {
      onClose()
      return
    }
    setBusy(true)
    setFieldErrors({})
    setSummary(undefined)
    try {
      const saved = await write(() => api.updateInvestigation(inv.ref, patch), {
        onRetry: () => void submitRef.current(),
      })
      if (saved) {
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
      title="Edit title and question"
      onClose={onClose}
      onSubmit={() => void submit()}
      submitLabel="Save changes"
      busyLabel="Saving..."
      busy={busy}
      dirty={dirty}
      summary={
        changedElsewhere || summary ? (
          <>
            {changedElsewhere && <ChangedElsewhereBanner onShowLatest={showLatest} />}
            {summary && (
              <p className="dialog-summary" role="alert">
                {summary}
              </p>
            )}
          </>
        ) : undefined
      }
    >
      <div className="dialog-fields" ref={fieldsRef}>
        <TextField
          label="Title"
          value={title}
          onChange={setTitle}
          error={fieldErrors.title}
          note={nearLimit(title, LIMITS.title)}
          autoComplete="off"
          spellCheck
        />
        <TextArea
          label="Primary question (optional)"
          value={question}
          onChange={setQuestion}
          error={fieldErrors.problem_statement}
          note={nearLimit(question, LIMITS.problemStatement)}
          spellCheck
        />
      </div>
    </Dialog>
  )
}
