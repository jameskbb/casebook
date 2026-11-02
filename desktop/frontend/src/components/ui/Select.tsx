// Select (UI-SPEC Component Contract): a native <select> styled to 36px with
// a ChevronDown, sharing the field chrome (label, hint, error) with
// TextField.
import type { ComponentPropsWithRef } from 'react'
import { ChevronDown } from 'lucide-react'
import { describedBy, FieldChrome, useFieldIds, type FieldChromeProps } from './TextField'
import './Field.css'
import './Select.css'

export interface SelectOption {
  value: string
  label: string
  disabled?: boolean
}

export interface SelectProps extends FieldChromeProps, Omit<ComponentPropsWithRef<'select'>, 'onChange' | 'value'> {
  value: string
  onChange: (value: string) => void
  options: SelectOption[]
}

export function Select({ label, hint, error, note, value, onChange, options, id, className, ...rest }: SelectProps) {
  const ids = useFieldIds(id)
  return (
    <FieldChrome ids={ids} label={label} hint={hint} error={error} note={note}>
      <span className="select-wrap">
        <select
          {...rest}
          id={ids.id}
          className={['field-input', 'select', className ?? ''].filter(Boolean).join(' ')}
          value={value}
          onChange={(e) => onChange(e.target.value)}
          aria-invalid={error ? true : undefined}
          aria-describedby={describedBy(ids, { label, hint, error, note })}
        >
          {options.map((o) => (
            <option key={o.value} value={o.value} disabled={o.disabled}>
              {o.label}
            </option>
          ))}
        </select>
        <ChevronDown className="select-chevron" size={16} strokeWidth={1.75} aria-hidden="true" />
      </span>
    </FieldChrome>
  )
}
