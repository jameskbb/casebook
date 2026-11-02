// EmptyState (UI-SPEC Empty states): the full variant centers a 48px muted
// icon, a Literata heading, a body and actions; the inline variant is one
// quiet line with an optional action.
import type { ReactNode } from 'react'
import type { LucideIcon } from 'lucide-react'
import './EmptyState.css'

export interface EmptyStateProps {
  variant?: 'full' | 'inline'
  icon?: LucideIcon
  heading?: string
  body: ReactNode
  actions?: ReactNode
  className?: string
}

export function EmptyState({ variant = 'full', icon: Icon, heading, body, actions, className }: EmptyStateProps) {
  if (variant === 'inline') {
    return (
      <div className={['empty-inline', className ?? ''].filter(Boolean).join(' ')}>
        <p className="empty-inline-text">{body}</p>
        {actions}
      </div>
    )
  }
  return (
    <div className={['empty-full', className ?? ''].filter(Boolean).join(' ')}>
      {Icon && <Icon className="empty-full-icon" size={48} strokeWidth={1.75} aria-hidden="true" />}
      {heading && <h2 className="empty-full-heading">{heading}</h2>}
      <p className="empty-full-body">{body}</p>
      {actions && <div className="empty-full-actions">{actions}</div>}
    </div>
  )
}
