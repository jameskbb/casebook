// One investigation in the sidebar list:
// title (2 lines) and the "Your AI's current question" marker; then the ref,
// the Archived tag and the relative updated time with the absolute time in its title. The list gives
// each row its roving tabindex props, so the whole list is one Tab stop.
import type { InvestigationSummary } from '../../api/types'
import type { RovingItemProps } from '../../hooks/useRovingFocus'
import { formatAbsolute, formatRelative } from '../../lib/relativeTime'
import { ArchivedTag } from '../record/StatusLabel'
import './InvestigationRow.css'

export interface InvestigationRowProps {
  investigation: InvestigationSummary
  selected: boolean
  onSelect: (ref: string) => void
  now?: Date
  /** Roving tabindex wiring from the list (one Tab stop for all rows). */
  roving?: RovingItemProps
}

export function InvestigationRow({ investigation: inv, selected, onSelect, now, roving }: InvestigationRowProps) {
  const classes = ['inv-row', selected ? 'is-selected' : '', inv.archived ? 'is-archived' : ''].filter(Boolean).join(' ')
  return (
    <li className="inv-row-item">
      <button
        {...roving}
        type="button"
        className={classes}
        aria-current={selected ? 'true' : undefined}
        data-ref={inv.ref}
        onClick={() => onSelect(inv.ref)}
      >
        <span className="inv-row-line inv-row-top">
          <span className="inv-row-title">{inv.title}</span>
          {inv.is_active && <ActiveMarker />}
        </span>
        <span className="inv-row-line inv-row-meta">
          <span className="inv-row-ref tabular">{inv.ref}</span>
          {inv.archived && <ArchivedTag />}
          <span className="inv-row-time tabular" title={formatAbsolute(inv.updated_at)}>
            {formatRelative(inv.updated_at, now)}
          </span>
        </span>
      </button>
    </li>
  )
}

/** The question your AI tools start from: an 8px accent dot and the words "Your AI's current question". */
export function ActiveMarker({ className }: { className?: string }) {
  return (
    <span
      className={['active-marker', className ?? ''].filter(Boolean).join(' ')}
      title="Your AI tools start from this question."
    >
      <span className="active-dot" aria-hidden="true" />
      Your AI's current question
    </span>
  )
}
