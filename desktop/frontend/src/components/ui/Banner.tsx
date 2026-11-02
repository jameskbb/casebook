// Banner (UI-SPEC Component Contract): info (the archived banner) and warning
// (live refresh trouble). "boxed" sits in the reading column with a rule and
// rounded corners; "bar" spans the top of the main pane. Text always carries
// the meaning; the icon and fill only reinforce it.
import type { ReactNode } from 'react'
import { Info, TriangleAlert, type LucideIcon } from 'lucide-react'
import './Banner.css'

export interface BannerProps {
  tone?: 'info' | 'warning'
  layout?: 'boxed' | 'bar'
  /** Replaces the tone's default icon (Info or TriangleAlert). */
  icon?: LucideIcon
  children: ReactNode
  /** A control on the right, e.g. Unarchive. */
  action?: ReactNode
  className?: string
}

export function Banner({ tone = 'info', layout = 'boxed', icon, children, action, className }: BannerProps) {
  const Icon = icon ?? (tone === 'warning' ? TriangleAlert : Info)
  return (
    <div
      className={['banner', `banner-${tone}`, `banner-${layout}`, className ?? ''].filter(Boolean).join(' ')}
      role="status"
    >
      <Icon className="banner-icon" size={16} strokeWidth={1.75} aria-hidden="true" />
      <p className="banner-text">{children}</p>
      {action}
    </div>
  )
}
