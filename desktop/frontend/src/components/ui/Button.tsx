// Button (UI-SPEC Component Contract): primary, secondary, ghost and
// destructive variants, 36px, 14/600, optional leading icon. busy swaps the
// label to its busy copy ("Creating...") and disables the button; a spinner
// is never used.
import type { ComponentPropsWithRef } from 'react'
import type { LucideIcon } from 'lucide-react'
import './Button.css'

export type ButtonVariant = 'primary' | 'secondary' | 'ghost' | 'destructive'

export interface ButtonProps extends ComponentPropsWithRef<'button'> {
  variant?: ButtonVariant
  icon?: LucideIcon
  busy?: boolean
  /** Label shown while busy, e.g. "Saving...". */
  busyLabel?: string
  fullWidth?: boolean
}

export function Button({
  variant = 'secondary',
  icon: Icon,
  busy = false,
  busyLabel,
  fullWidth = false,
  disabled,
  className,
  children,
  type = 'button',
  ...rest
}: ButtonProps) {
  const off = Boolean(disabled || busy)
  const classes = ['btn', `btn-${variant}`, fullWidth ? 'btn-full' : '', className ?? ''].filter(Boolean).join(' ')
  return (
    <button {...rest} type={type} className={classes} disabled={off} aria-disabled={off || undefined} aria-busy={busy || undefined}>
      {Icon && <Icon className="btn-icon" size={16} strokeWidth={1.75} aria-hidden="true" />}
      <span className="btn-label">{busy && busyLabel ? busyLabel : children}</span>
    </button>
  )
}
