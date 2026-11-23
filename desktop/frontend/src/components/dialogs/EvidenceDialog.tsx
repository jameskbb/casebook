// Add and edit a source. Four fields: Name, What it shows, Where to find it
// and Exact words or numbers. The kind of source (Something you saw unless
// changed) and its date (local time, sent as an RFC3339 timestamp) sit under
// More. Where it came from and the extra details an AI stores are not edited
// here; an edit leaves them as they are.
//
// Edit mode loads the exact words through GetEvidence with include_raw_detail
// and sends only the fields that changed; clearing the date sends an empty
// value, which sets it back to the day the source was added. An error
// appears under its field (opening More when it is in there); any other goes
// to the top of the form.
import { useEffect, useRef, useState } from 'react'
import { ChevronDown } from 'lucide-react'
import { api } from '../../api/api'
import { errorCopy, toApiError } from '../../api/errors'
import type { Evidence, EvidenceInput, EvidencePatch, SourceType } from '../../api/types'
import { LIMITS, nearLimit } from '../../lib/bytes'
import { fromLocalInputValue, isInvalidCaptured, toLocalInputValue } from '../../lib/captured'
import { captureHints } from '../../lib/captureHints'
import { splitFieldErrors } from '../../lib/formErrors'
import { sourceTypeLabels, sourceTypeOptions } from '../../lib/labels'
import { useWrite } from '../../state/useWrite'
import { Button } from '../ui/Button'
import { ChangedElsewhereBanner } from '../ui/ChangedElsewhereBanner'
import { Dialog } from '../ui/Dialog'
import { Select } from '../ui/Select'
import { TextArea } from '../ui/TextArea'
import { TextField } from '../ui/TextField'
import { useToast } from '../ui/Toaster'
import './EvidenceDialog.css'

export const CAPTURED_INVALID_COPY = 'Enter a valid date and time.'

const FIELDS = [
  'title',
  'summary',
  'source_type',
  'source_ref',
  'captured_at',
  'raw_detail',
] as const

interface Values {
  title: string
  summary: string
  source_type: SourceType
  source_ref: string
  /** datetime-local value. */
  captured: string
  raw_detail: string
}

const EMPTY: Values = {
  title: '',
  summary: '',
  source_type: 'manual_observation',
  source_ref: '',
  captured: '',
  raw_detail: '',
}

function valuesOf(e: Evidence): Values {
  return {
    title: e.title,
    summary: e.summary,
    source_type: e.source_type,
    source_ref: e.source_ref,
    captured: toLocalInputValue(e.captured_at),
    raw_detail: '',
  }
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
  const [moreOpen, setMoreOpen] = useState(false)
  const [busy, setBusy] = useState(false)
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({})
  const [summary, setSummary] = useState<string | undefined>()
  const [focusInvalid, setFocusInvalid] = useState(false)
  const fieldsRef = useRef<HTMLDivElement>(null)
  const capture = captureHints[values.source_type]
  // A source of a kind a person cannot pick keeps its own kind listed.
  const typeOptions: SourceType[] =
    existing && !sourceTypeOptions.includes(existing.source_type) ? [...sourceTypeOptions, existing.source_type] : sourceTypeOptions

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
    const input: EvidenceInput = { title: v.title, summary: v.summary, source_type: v.source_type }
    if (v.source_ref.trim()) input.source_ref = v.source_ref
    const captured = fromLocalInputValue(v.captured)
    if (captured) input.captured_at = captured
    if (v.raw_detail.trim()) input.raw_detail = v.raw_detail
    return input
  }

  const buildPatch = (): EvidencePatch => {
    const v = values
    const b = baseline
    const patch: EvidencePatch = {}
    if (v.title !== b.title) patch.title = v.title
    if (v.summary !== b.summary) patch.summary = v.summary
    if (v.source_type !== b.source_type) patch.source_type = v.source_type
    if (v.source_ref !== b.source_ref) patch.source_ref = v.source_ref
    if (v.captured !== b.captured) patch.captured_at = v.captured ? fromLocalInputValue(v.captured) : ''
    if (rawState !== 'loading' && rawState !== 'failed' && v.raw_detail !== b.raw_detail) patch.raw_detail = v.raw_detail
    return patch
  }

  // Retry toasts call the latest submit, so a retry sends what the form
  // holds now and sees whether a submit is already running.
  const submitRef = useRef<() => Promise<void>>(async () => {})
  const submit = async () => {
    if (busy) return
    // A filled-in date that does not parse must never be sent as "no date":
    // add would store now and edit would reset the captured time.
    if (isInvalidCaptured(values.captured)) {
      setSummary(undefined)
      setFieldErrors({ captured_at: CAPTURED_INVALID_COPY })
      setMoreOpen(true)
      setFocusInvalid(true)
      return
    }
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
      if (split.fieldErrors.source_type || split.fieldErrors.captured_at) setMoreOpen(true)
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
          hint={capture?.reference}
          value={values.source_ref}
          onChange={set('source_ref')}
          error={fieldErrors.source_ref}
          note={nearLimit(values.source_ref, LIMITS.sourceRef)}
          spellCheck={false}
        />
        <TextArea
          label="Exact words or numbers (optional)"
          hint={capture?.detail}
          value={values.raw_detail}
          onChange={set('raw_detail')}
          error={fieldErrors.raw_detail}
          note={rawState === 'failed' ? 'The saved words could not be loaded, so they cannot be edited now.' : nearLimit(values.raw_detail, LIMITS.rawDetail)}
          disabled={rawState === 'loading' || rawState === 'failed'}
          rows={4}
          spellCheck={false}
        />
        <div className="evidence-more">
          <div>
            <Button variant="ghost" className="disclosure-button" aria-expanded={moreOpen} onClick={() => setMoreOpen((v) => !v)}>
              <ChevronDown className={['disclosure-icon', moreOpen ? 'is-open' : ''].filter(Boolean).join(' ')} size={16} strokeWidth={1.75} aria-hidden="true" />
              More
            </Button>
          </div>
          {moreOpen && (
            <div className="evidence-dialog-row">
              <Select
                label="Kind of source"
                value={values.source_type}
                onChange={(v) => set('source_type')(v as SourceType)}
                options={typeOptions.map((t) => ({ value: t, label: sourceTypeLabels[t] }))}
                error={fieldErrors.source_type}
              />
              <TextField
                label="Date of the source (optional)"
                hint={existing ? 'Leave empty for the day it was added' : 'Leave empty for today'}
                type="datetime-local"
                value={values.captured}
                onChange={set('captured')}
                error={fieldErrors.captured_at}
              />
            </div>
          )}
        </div>
      </div>
    </Dialog>
  )
}
