// TextArea: same chrome as TextField; grows with its content from 3 to 12
// rows (CSS field-sizing).
import type { ComponentPropsWithRef } from 'react'
import { describedBy, FieldChrome, useFieldIds, type FieldChromeProps } from './TextField'
import './Field.css'

export interface TextAreaProps extends FieldChromeProps, Omit<ComponentPropsWithRef<'textarea'>, 'onChange' | 'value'> {
  value: string
  onChange: (value: string) => void
}

export function TextArea({ label, hint, error, note, value, onChange, id, className, rows = 3, ...rest }: TextAreaProps) {
  const ids = useFieldIds(id)
  return (
    <FieldChrome ids={ids} label={label} hint={hint} error={error} note={note}>
      <textarea
        {...rest}
        id={ids.id}
        rows={rows}
        className={['field-input', 'field-textarea', className ?? ''].filter(Boolean).join(' ')}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        aria-invalid={error ? true : undefined}
        aria-describedby={describedBy(ids, { label, hint, error, note })}
      />
    </FieldChrome>
  )
}
