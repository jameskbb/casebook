// TextField: label (14/600) above, optional hint, 16px input, error state with
// CircleAlert, aria-invalid and aria-describedby.
import { useId, type ComponentPropsWithRef, type ReactNode } from 'react'
import { CircleAlert } from 'lucide-react'
import './Field.css'

export interface FieldChromeProps {
  label: string
  hint?: string
  error?: string
  /** Extra line under the hint, e.g. the near-limit counter. */
  note?: ReactNode
}

export interface TextFieldProps extends FieldChromeProps, Omit<ComponentPropsWithRef<'input'>, 'onChange' | 'value'> {
  value: string
  onChange: (value: string) => void
}

/** Ids for the hint, note and error lines of a field. */
export function useFieldIds(id?: string) {
  const auto = useId()
  const base = id ?? `field-${auto}`
  return { id: base, hint: `${base}-hint`, note: `${base}-note`, error: `${base}-error` }
}

export function describedBy(ids: ReturnType<typeof useFieldIds>, p: FieldChromeProps): string | undefined {
  const parts = [p.hint ? ids.hint : '', p.note ? ids.note : '', p.error ? ids.error : ''].filter(Boolean)
  return parts.length ? parts.join(' ') : undefined
}

export function FieldChrome({
  ids,
  label,
  hint,
  error,
  note,
  children,
}: FieldChromeProps & { ids: ReturnType<typeof useFieldIds>; children: ReactNode }) {
  return (
    <div className={['field', error ? 'field-invalid' : ''].filter(Boolean).join(' ')}>
      <label className="field-label" htmlFor={ids.id}>
        {label}
      </label>
      {hint && (
        <p className="field-hint" id={ids.hint}>
          {hint}
        </p>
      )}
      {children}
      {note && (
        <p className="field-note tabular" id={ids.note}>
          {note}
        </p>
      )}
      {error && (
        <p className="field-error" id={ids.error}>
          <CircleAlert size={16} strokeWidth={1.75} aria-hidden="true" />
          <span>{error}</span>
        </p>
      )}
    </div>
  )
}

export function TextField({ label, hint, error, note, value, onChange, id, className, ...rest }: TextFieldProps) {
  const ids = useFieldIds(id)
  return (
    <FieldChrome ids={ids} label={label} hint={hint} error={error} note={note}>
      <input
        {...rest}
        id={ids.id}
        className={['field-input', className ?? ''].filter(Boolean).join(' ')}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        aria-invalid={error ? true : undefined}
        aria-describedby={describedBy(ids, { label, hint, error, note })}
      />
    </FieldChrome>
  )
}
