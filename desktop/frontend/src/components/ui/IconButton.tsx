// IconButton: a 32px ghost square. label is required and becomes both the
// accessible name and the tooltip.
import type { ComponentPropsWithRef } from 'react'
import type { LucideIcon } from 'lucide-react'
import './IconButton.css'

export interface IconButtonProps extends Omit<ComponentPropsWithRef<'button'>, 'children'> {
  icon: LucideIcon
  label: string
}

export function IconButton({ icon: Icon, label, className, type = 'button', disabled, ...rest }: IconButtonProps) {
  return (
    <button
      {...rest}
      type={type}
      disabled={disabled}
      aria-disabled={disabled || undefined}
      aria-label={label}
      title={label}
      className={['icon-btn', className ?? ''].filter(Boolean).join(' ')}
    >
      <Icon size={16} strokeWidth={1.75} aria-hidden="true" />
    </button>
  )
}
