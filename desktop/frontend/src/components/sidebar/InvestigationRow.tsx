// One question in the sidebar list: the title (2 lines); the "Your AI is
// working on this" marker on its own line; then one muted line, "1 of 3 sources checked by
// you" when it has sources, otherwise when it was last worked on, with the
// Archived tag. Its code shows only when codes are on. The list gives each
// row its roving tabindex props, so the whole list is one Tab stop.
import type { InvestigationSummary } from '../../api/types'
import type { RovingItemProps } from '../../hooks/useRovingFocus'
import { AI_WORKING } from '../../lib/labels'
import { checkedLine } from '../../lib/questions'
import { formatAbsolute, formatRelative } from '../../lib/relativeTime'
import { useAppData } from '../../state/appData'
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
  const checked = checkedLine(inv)
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
        <span className="inv-row-title">{inv.title}</span>
        {inv.is_active && !inv.archived && <ActiveMarker />}
        <span className="inv-row-line inv-row-meta">
          <span className="inv-row-ref tabular">{inv.ref}</span>
          {inv.archived && <ArchivedTag />}
          <span className="inv-row-time tabular" title={formatAbsolute(inv.updated_at)}>
            {checked || `Last worked on ${formatRelative(inv.updated_at, now)}`}
          </span>
        </span>
      </button>
    </li>
  )
}

/**
 * The question your AI saves its findings under: an 8px accent dot and the
 * words. A marker, never a control. Never shown while no AI tool is connected.
 */
export function ActiveMarker({ className }: { className?: string }) {
  if (!useAppData().aiConnected) return null
  return (
    <span
      className={['active-marker', className ?? ''].filter(Boolean).join(' ')}
      title="Your AI saves what it finds under this question."
    >
      <span className="active-dot" aria-hidden="true" />
      {AI_WORKING}
    </span>
  )
}
