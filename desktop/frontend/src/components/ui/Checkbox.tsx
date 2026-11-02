// Checkbox with its label on the right.
import { useId, type ComponentPropsWithRef } from 'react'
import './Checkbox.css'

export interface CheckboxProps extends Omit<ComponentPropsWithRef<'input'>, 'type' | 'onChange' | 'checked'> {
  label: string
  checked: boolean
  onChange: (checked: boolean) => void
}

export function Checkbox({ label, checked, onChange, id, className, ...rest }: CheckboxProps) {
  const auto = useId()
  const inputId = id ?? `checkbox-${auto}`
  return (
    <div className={['checkbox', className ?? ''].filter(Boolean).join(' ')}>
      <input
        {...rest}
        id={inputId}
        type="checkbox"
        className="checkbox-input"
        checked={checked}
        onChange={(e) => onChange(e.target.checked)}
      />
      <label className="checkbox-label" htmlFor={inputId}>
        {label}
      </label>
    </div>
  )
}
