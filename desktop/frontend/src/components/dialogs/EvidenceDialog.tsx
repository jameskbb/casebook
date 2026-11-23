// Add and edit a source. Four fields: Name, What it shows, Where to find it
// and Exact words or numbers. Adding one also offers a tick, off at first:
// I have looked at this source myself. Ticked, the source is saved as checked
// by the person (the history says so); unticked, it is Not checked and Check
// the next source offers it. The stored kind, date and extra details are not
// edited here; an edit leaves them as they are.
//
// Edit mode loads the exact words through GetEvidence with include_raw_detail
// and sends only the fields that changed. An error appears under its field;
// any other goes to the top of the form.
import { useEffect, useRef, useState } from 'react'
import { api } from '../../api/api'
import { errorCopy, toApiError } from '../../api/errors'
import type { Evidence, EvidenceInput, EvidencePatch } from '../../api/types'
import { LIMITS, nearLimit } from '../../lib/bytes'
import { splitFieldErrors } from '../../lib/formErrors'
import { useWrite } from '../../state/useWrite'
import { ChangedElsewhereBanner } from '../ui/ChangedElsewhereBanner'
import { Checkbox } from '../ui/Checkbox'
import { Dialog } from '../ui/Dialog'
import { TextArea } from '../ui/TextArea'
import { TextField } from '../ui/TextField'
import { useToast } from '../ui/Toaster'
import './EvidenceDialog.css'

export const LOOKED_MYSELF = 'I have looked at this source myself'
export const LOOKED_MYSELF_HINT = 'Leave it off and the source shows as Not checked until you check it.'

const FIELDS = ['title', 'summary', 'source_ref', 'raw_detail'] as const

interface Values {
  title: string
  summary: string
  source_ref: string
  raw_detail: string
}

const EMPTY: Values = { title: '', summary: '', source_ref: '', raw_detail: '' }

function valuesOf(e: Evidence): Values {
  return { title: e.title, summary: e.summary, source_ref: e.source_ref, raw_detail: '' }
}

const same = (a: Values, b: Values) => (Object.keys(a) as (keyof Values)[]).every((k) => a[k] === b[k])

export type EvidenceDialogProps = {
  invRef: string
  onClose: () => void
  /** Called with the saved evidence before the dialog closes. */
  onSaved?: (evidence: Evidence) => void
} & ({ mode: 'add' } | { mode: 'edit'; evidence: Evidence })

type RawState = 'none' | 'loading' | 'ready' | 'failed'

export function EvidenceDialog(props: EvidenceDialogProps) {
  const { invRef, onClose, onSaved } = props
  const existing = props.mode === 'edit' ? props.evidence : null
  const write = useWrite()
  const toast = useToast()
  const [initial] = useState<Values>(() => (existing ? valuesOf(existing) : EMPTY))
  const [baseline, setBaseline] = useState<Values>(initial)
  const [values, setValues] = useState<Values>(initial)
  const [rawState, setRawState] = useState<RawState>(existing?.has_raw_detail ? 'loading' : 'none')
  const [busy, setBusy] = useState(false)
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({})
  const [summary, setSummary] = useState<string | undefined>()
  const [focusInvalid, setFocusInvalid] = useState(false)
  const [looked, setLooked] = useState(false)
  const fieldsRef = useRef<HTMLDivElement>(null)

  // Edit mode: load the stored exact words when the dialog opens (and again
  // on Show the new version).
  const rawRequest = useRef(0)
  const loadRaw = (ev: Evidence) => {
    const id = ++rawRequest.current
    if (!ev.has_raw_detail) {
      setRawState('none')
      return
    }
    setRawState('loading')
    api.getEvidence(invRef, ev.ref, { include_raw_detail: true }).then(
      (full) => {
        if (id !== rawRequest.current) return
        const raw = full.raw_detail ?? ''
        setBaseline((b) => ({ ...b, raw_detail: raw }))
        setValues((v) => ({ ...v, raw_detail: raw }))
        setRawState('ready')
      },
      (err) => {
        if (id !== rawRequest.current) return
        setRawState('failed')
        toast.error(errorCopy(toApiError(err)).message)
      },
    )
  }
  useEffect(() => {
    if (existing?.has_raw_detail) loadRaw(existing)
    return () => {
      rawRequest.current++
    }
    // Loads once when the dialog opens.
  }, [])

  // The stored fields (the exact words aside, which are fetched on demand)
  // changed while the form holds input.
  const dirty = !same(values, baseline)
  const storedNow = existing ? valuesOf(existing) : null
  const withoutRaw = (v: Values) => ({ ...v, raw_detail: '' })
  const changedElsewhere =
    storedNow !== null && dirty && !busy && !same(withoutRaw(storedNow), withoutRaw(baseline))
  const showLatest = () => {
    if (!existing) return
    const latest = valuesOf(existing)
    setBaseline(latest)
    setValues(latest)
    setFieldErrors({})
    setSummary(undefined)
    loadRaw(existing)
  }

  useEffect(() => {
    if (!focusInvalid) return
    setFocusInvalid(false)
    fieldsRef.current?.querySelector<HTMLElement>('[aria-invalid="true"]')?.focus()
  }, [focusInvalid])

  const set = <K extends keyof Values>(key: K) => (value: Values[K]) => setValues((v) => ({ ...v, [key]: value }))

  const buildInput = (): EvidenceInput => {
    const v = values
    // Added by hand. Checked by the person only when they say they looked at it.
    const input: EvidenceInput = { title: v.title, summary: v.summary, source_type: 'manual_observation', verification: looked ? 'human_verified' : 'ai_generated' }
    if (v.source_ref.trim()) input.source_ref = v.source_ref
    if (v.raw_detail.trim()) input.raw_detail = v.raw_detail
    return input
  }

  const buildPatch = (): EvidencePatch => {
    const v = values
    const b = baseline
    const patch: EvidencePatch = {}
    if (v.title !== b.title) patch.title = v.title
    if (v.summary !== b.summary) patch.summary = v.summary
    if (v.source_ref !== b.source_ref) patch.source_ref = v.source_ref
    if (rawState !== 'loading' && rawState !== 'failed' && v.raw_detail !== b.raw_detail) patch.raw_detail = v.raw_detail
    return patch
  }

  // Retry toasts call the latest submit, so a retry sends what the form
  // holds now and sees whether a submit is already running.
  const submitRef = useRef<() => Promise<void>>(async () => {})
  const submit = async () => {
    if (busy) return
    let call: () => Promise<Evidence>
    if (existing) {
      const patch = buildPatch()
      if (Object.keys(patch).length === 0) {
        onClose()
        return
      }
      call = () => api.updateEvidence(invRef, existing.ref, patch)
    } else {
      const input = buildInput()
      call = () => api.addEvidence(invRef, input)
    }
    setBusy(true)
    setFieldErrors({})
    setSummary(undefined)
    try {
      const saved = await write(call, { onRetry: () => void submitRef.current() })
      if (saved) {
        onSaved?.(saved)
        onClose()
      }
    } catch (e) {
      const split = splitFieldErrors(e, FIELDS)
      setFieldErrors(split.fieldErrors)
      setSummary(split.summary)
      setFocusInvalid(true)
    } finally {
      setBusy(false)
    }
  }

  submitRef.current = submit

  return (
    <Dialog
      title={existing ? 'Edit source' : 'Add source'}
      onClose={onClose}
      onSubmit={() => void submit()}
      submitLabel={existing ? 'Save changes' : 'Add source'}
      busyLabel={existing ? 'Saving...' : 'Adding...'}
      busy={busy}
      dirty={dirty}
      className="evidence-dialog"
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
          label="Name"
          hint="Like Call log or Email from the billing team"
          value={values.title}
          onChange={set('title')}
          error={fieldErrors.title}
          note={nearLimit(values.title, LIMITS.title)}
        />
        <TextArea
          label="What it shows"
          hint="One or two sentences"
          value={values.summary}
          onChange={set('summary')}
          error={fieldErrors.summary}
          note={nearLimit(values.summary, LIMITS.evidenceSummary)}
          spellCheck
        />
        <TextField
          label="Where to find it"
          hint="The file, page or person, and the date"
          value={values.source_ref}
          onChange={set('source_ref')}
          error={fieldErrors.source_ref}
          note={nearLimit(values.source_ref, LIMITS.sourceRef)}
          spellCheck={false}
        />
        <TextArea
          label="Exact words or numbers (optional)"
          hint="The lines or numbers that matter"
          value={values.raw_detail}
          onChange={set('raw_detail')}
          error={fieldErrors.raw_detail}
          note={rawState === 'failed' ? 'The saved words could not be loaded, so they cannot be edited now.' : nearLimit(values.raw_detail, LIMITS.rawDetail)}
          disabled={rawState === 'loading' || rawState === 'failed'}
          rows={4}
          spellCheck={false}
        />
        {!existing && (
          <div>
            <Checkbox label={LOOKED_MYSELF} checked={looked} onChange={setLooked} aria-describedby="evidence-looked-hint" />
            <p className="evidence-dialog-note" id="evidence-looked-hint">{LOOKED_MYSELF_HINT}</p>
          </div>
        )}
      </div>
    </Dialog>
  )
}
