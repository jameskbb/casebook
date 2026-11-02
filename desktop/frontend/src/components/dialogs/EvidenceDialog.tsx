// Add and edit evidence (UI-07; UI-SPEC Copywriting, Add evidence and Edit
// evidence rows). Fields: title, summary, source type (default Manual
// observation), source, reference, captured (local time, converted to an
// RFC3339 timestamp), raw detail behind its own disclosure and metadata behind
// "Advanced". Metadata text goes to the engine as typed; the engine checks
// it is a JSON object.
//
// Edit mode loads the raw detail through GetEvidence with include_raw_detail
// and sends only the fields that changed; clearing Captured sends an empty
// value, which resets it to the time the evidence was added. Validation
// errors appear under their field (opening the disclosure that holds it);
// errors on other fields go to the summary at the top.
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
import { prettyMetadata } from '../../lib/metadata'
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
  'source_label',
  'captured_at',
  'raw_detail',
  'metadata',
] as const

interface Values {
  title: string
  summary: string
  source_type: SourceType
  source_label: string
  source_ref: string
  /** datetime-local value. */
  captured: string
  raw_detail: string
  metadata: string
}

const EMPTY: Values = {
  title: '',
  summary: '',
  source_type: 'manual_observation',
  source_label: '',
  source_ref: '',
  captured: '',
  raw_detail: '',
  metadata: '',
}

function valuesOf(e: Evidence): Values {
  return {
    title: e.title,
    summary: e.summary,
    source_type: e.source_type,
    source_label: e.source_label,
    source_ref: e.source_ref,
    captured: toLocalInputValue(e.captured_at),
    raw_detail: '',
    metadata: prettyMetadata(e.metadata),
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
  const [rawOpen, setRawOpen] = useState(Boolean(existing?.has_raw_detail))
  const [advancedOpen, setAdvancedOpen] = useState(Boolean(existing && existing.metadata))
  const [guideOpen, setGuideOpen] = useState(false)
  const [busy, setBusy] = useState(false)
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({})
  const [summary, setSummary] = useState<string | undefined>()
  const [focusInvalid, setFocusInvalid] = useState(false)
  const [focusRaw, setFocusRaw] = useState(false)
  const fieldsRef = useRef<HTMLDivElement>(null)
  const rawRef = useRef<HTMLTextAreaElement>(null)
  const capture = captureHints[values.source_type]
  // A record of a type new evidence cannot pick keeps its own type listed.
  const typeOptions: SourceType[] =
    existing && !sourceTypeOptions.includes(existing.source_type) ? [...sourceTypeOptions, existing.source_type] : sourceTypeOptions

  // Edit mode: load the stored raw detail when the dialog opens (and again
  // on Show latest).
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

  // AR-23: the stored fields (raw detail aside, which is fetched on demand)
  // changed in another app while the form holds input.
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

  useEffect(() => {
    if (!focusRaw) return
    setFocusRaw(false)
    rawRef.current?.focus()
  }, [focusRaw])

  const set = <K extends keyof Values>(key: K) => (value: Values[K]) => setValues((v) => ({ ...v, [key]: value }))

  const buildInput = (): EvidenceInput => {
    const v = values
    const input: EvidenceInput = { title: v.title, summary: v.summary, source_type: v.source_type }
    if (v.source_label.trim()) input.source_label = v.source_label
    if (v.source_ref.trim()) input.source_ref = v.source_ref
    const captured = fromLocalInputValue(v.captured)
    if (captured) input.captured_at = captured
    if (v.raw_detail.trim()) input.raw_detail = v.raw_detail
    if (v.metadata.trim()) input.metadata = v.metadata
    return input
  }

  const buildPatch = (): EvidencePatch => {
    const v = values
    const b = baseline
    const patch: EvidencePatch = {}
    if (v.title !== b.title) patch.title = v.title
    if (v.summary !== b.summary) patch.summary = v.summary
    if (v.source_type !== b.source_type) patch.source_type = v.source_type
    if (v.source_label !== b.source_label) patch.source_label = v.source_label
    if (v.source_ref !== b.source_ref) patch.source_ref = v.source_ref
    if (v.captured !== b.captured) patch.captured_at = v.captured ? fromLocalInputValue(v.captured) : ''
    if (rawState !== 'loading' && rawState !== 'failed' && v.raw_detail !== b.raw_detail) patch.raw_detail = v.raw_detail
    if (v.metadata !== b.metadata) patch.metadata = v.metadata
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
      if (split.fieldErrors.raw_detail) setRawOpen(true)
      if (split.fieldErrors.metadata) setAdvancedOpen(true)
      setFocusInvalid(true)
    } finally {
      setBusy(false)
    }
  }

  const rawField = (
    <TextArea
      ref={rawRef}
      label="Raw detail (optional)"
      hint={capture?.detail}
      value={values.raw_detail}
      onChange={set('raw_detail')}
      error={fieldErrors.raw_detail}
      note={rawState === 'failed' ? 'The stored raw detail could not be loaded, so it cannot be edited now.' : nearLimit(values.raw_detail, LIMITS.rawDetail)}
      disabled={rawState === 'loading' || rawState === 'failed'}
      rows={6}
      spellCheck={false}
      className="evidence-raw-input"
    />
  )

  submitRef.current = submit

  return (
    <Dialog
      title={existing ? `Edit evidence ${existing.ref}` : 'Add evidence'}
      onClose={onClose}
      onSubmit={() => void submit()}
      submitLabel={existing ? 'Save changes' : 'Add evidence'}
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
          label="Title"
          hint="What is this evidence, in a few words?"
          value={values.title}
          onChange={set('title')}
          error={fieldErrors.title}
          note={nearLimit(values.title, LIMITS.title)}
        />
        <TextArea
          label="Summary"
          hint="What it shows, in one or two sentences"
          value={values.summary}
          onChange={set('summary')}
          error={fieldErrors.summary}
          note={nearLimit(values.summary, LIMITS.evidenceSummary)}
          spellCheck
        />
        <div className="evidence-dialog-row">
          <Select
            label="Source type"
            value={values.source_type}
            onChange={(v) => set('source_type')(v as SourceType)}
            options={typeOptions.map((t) => ({ value: t, label: sourceTypeLabels[t] }))}
            error={fieldErrors.source_type}
          />
          <TextField
            label="Captured (optional)"
            hint={existing ? 'Leave empty to use the time it was added' : 'Leave empty to use now'}
            type="datetime-local"
            value={values.captured}
            onChange={set('captured')}
            error={fieldErrors.captured_at}
          />
        </div>
        <TextField
          label="Source (optional)"
          hint="Where it came from, like Billing export query"
          value={values.source_label}
          onChange={set('source_label')}
          error={fieldErrors.source_label}
          note={nearLimit(values.source_label, LIMITS.sourceLabel)}
        />
        <TextField
          label="Reference (optional)"
          hint={capture?.reference}
          value={values.source_ref}
          onChange={set('source_ref')}
          error={fieldErrors.source_ref}
          note={nearLimit(values.source_ref, LIMITS.sourceRef)}
          spellCheck={false}
        />
        <div className="evidence-capture-guide">
          <Button
            variant="ghost"
            className="disclosure-button"
            aria-expanded={guideOpen}
            aria-controls="evidence-capture-guide-content"
            onClick={() => setGuideOpen((v) => !v)}
          >
            <ChevronDown
              className={['disclosure-icon', guideOpen ? 'is-open' : ''].filter(Boolean).join(' ')}
              size={16}
              strokeWidth={1.75}
              aria-hidden="true"
            />
            Capture details guide
          </Button>
          {guideOpen && (
            <div id="evidence-capture-guide-content" className="evidence-capture-guide-content">
              <p>Keep enough source detail for someone to check this later. Use what applies; observations do not need numerical context.</p>
              <ul>
                <li>For a comparison, record the period, comparison period, scope and filters.</li>
                <li>For a metric, record its definition, units and denominator. Distinguish percent from percentage points.</li>
                <li>Keep the exact sheet and range, page, section or query in Reference, and a concise source excerpt in Raw detail.</li>
              </ul>
              {capture && <>
                <p className="evidence-capture-example-label">Illustrative {sourceTypeLabels[values.source_type].toLowerCase()} example. Nothing is added to your evidence.</p>
                <dl>
                  <dt>Reference</dt>
                  <dd>{capture.exampleReference}</dd>
                  <dt>Raw detail</dt>
                  <dd>{capture.exampleDetail}</dd>
                </dl>
              </>}
              <p>Advanced metadata can hold optional keys such as <code>period</code>, <code>comparison_period</code>, <code>scope</code>, <code>filters</code>, <code>metric_definition</code>, <code>units</code> and <code>denominator</code>. Preserve any keys already there. Keep secrets and transcripts out.</p>
            </div>
          )}
        </div>
        {rawOpen ? (
          rawField
        ) : (
          <div>
            <Button
              variant="ghost"
              className="disclosure-button"
              aria-expanded={false}
              onClick={() => {
                setRawOpen(true)
                setFocusRaw(true)
              }}
            >
              <ChevronDown className="disclosure-icon" size={16} strokeWidth={1.75} aria-hidden="true" />
              Add raw detail
            </Button>
          </div>
        )}
        <div className="evidence-advanced">
          <div>
            <Button
              variant="ghost"
              className="disclosure-button"
              aria-expanded={advancedOpen}
              onClick={() => setAdvancedOpen((v) => !v)}
            >
              <ChevronDown
                className={['disclosure-icon', advancedOpen ? 'is-open' : ''].filter(Boolean).join(' ')}
                size={16}
                strokeWidth={1.75}
                aria-hidden="true"
              />
              Advanced
            </Button>
          </div>
          {advancedOpen && (
            <TextArea
              label="Metadata (optional)"
              hint={'A JSON object, like {"affected_records": 22}'}
              value={values.metadata}
              onChange={set('metadata')}
              error={fieldErrors.metadata}
              note={nearLimit(values.metadata, LIMITS.metadata)}
              spellCheck={false}
            />
          )}
        </div>
      </div>
    </Dialog>
  )
}
