// Switch (UI-SPEC Component Contract): a labelled on/off control used for
// "Show archived". The label sits on the left and the switch on the right;
// the whole row is clickable. The on track is --ink-muted, never the accent.
import { useId } from 'react'
import './Switch.css'

export interface SwitchProps {
  label: string
  checked: boolean
  onChange: (checked: boolean) => void
  disabled?: boolean
  className?: string
}

export function Switch({ label, checked, onChange, disabled = false, className }: SwitchProps) {
  const id = useId()
  return (
    <div className={['switch-row', className ?? ''].filter(Boolean).join(' ')}>
      <label className="switch-label" htmlFor={id}>
        {label}
      </label>
      <button
        id={id}
        type="button"
        role="switch"
        aria-checked={checked}
        disabled={disabled}
        className={['switch', checked ? 'is-on' : ''].filter(Boolean).join(' ')}
        onClick={() => onChange(!checked)}
      >
        <span className="switch-thumb" aria-hidden="true" />
      </button>
    </div>
  )
}
