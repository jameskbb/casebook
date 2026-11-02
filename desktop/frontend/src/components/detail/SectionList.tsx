// A reading-column section (UI-SPEC Layout, Section heading row; D-12, D-24):
// a 20px kind icon, the h2 heading, an optional add action, then rows
// separated by 1px rules (no cards, no shadows). Ruled-out records collapse
// under a disclosure with count-aware copy. An empty section shows its one
// line of copy, carrying the add action itself so it never appears twice.
import { useId, useState, type ReactNode } from 'react'
import { ChevronDown, Plus, type LucideIcon } from 'lucide-react'
import { plural } from '../../lib/plural'
import { Button } from '../ui/Button'
import './SectionList.css'

export interface SectionListProps<T> {
  /** Stable id used for the heading. */
  id: string
  icon: LucideIcon
  /** Tints the heading icon with the kind hue. */
  kind: 'evidence' | 'claim' | 'hypothesis' | 'question' | 'history'
  heading: string
  /** "Add claim", "Add evidence", ... shown only with onAdd. */
  addLabel?: string
  onAdd?: () => void
  emptyText: string
  /** Show the empty copy when only ruled-out items exist (Still open). */
  emptyWithRuledOut?: boolean
  /** Rendered above the rows (Still open's open explanations); counts as content. */
  lead?: ReactNode
  items: T[]
  ruledOut?: T[]
  /** Singular and plural nouns for the disclosure ("ruled-out hypothesis"). */
  ruledOutNoun?: [string, string]
  itemKey: (item: T) => string
  renderItem: (item: T) => ReactNode
  /** Rendered after the add action in the heading row, for a small text link. */
  headingExtra?: ReactNode
  /** Rendered after the rows (History's Show more). */
  footer?: ReactNode
}

export function SectionList<T>({
  id,
  icon: Icon,
  kind,
  heading,
  addLabel,
  onAdd,
  emptyText,
  emptyWithRuledOut = false,
  lead,
  items,
  ruledOut = [],
  ruledOutNoun,
  itemKey,
  renderItem,
  headingExtra,
  footer,
}: SectionListProps<T>) {
  const [showRuledOut, setShowRuledOut] = useState(false)
  const ruledOutId = useId()
  const headingId = `section-${id}`
  const empty = items.length === 0 && !lead && (ruledOut.length === 0 || emptyWithRuledOut)
  const add =
    onAdd && addLabel ? (
      <Button variant="ghost" icon={Plus} onClick={onAdd}>
        {addLabel}
      </Button>
    ) : null
  const n = ruledOut.length
  const noun = ruledOutNoun ? plural(n, ruledOutNoun[0], ruledOutNoun[1]) : ''

  return (
    <section className={`section section-${kind}`} aria-labelledby={headingId}>
      <div className="section-head">
        <Icon className="section-icon" size={20} strokeWidth={1.75} aria-hidden="true" />
        <h2 className="section-heading" id={headingId}>
          {heading}
        </h2>
        {!empty && add && <div className="section-add">{add}</div>}
        {headingExtra}
      </div>
      {empty && (
        <div className="section-empty">
          <p className="section-empty-text">{emptyText}</p>
          {add}
        </div>
      )}
      {lead}
      {items.length > 0 && (
        <ul className="section-rows">
          {items.map((item) => (
            <li key={itemKey(item)}>{renderItem(item)}</li>
          ))}
        </ul>
      )}
      {n > 0 && ruledOutNoun && (
        <div className="section-ruled-out">
          <button
            type="button"
            className="section-disclosure"
            aria-expanded={showRuledOut}
            aria-controls={ruledOutId}
            onClick={() => setShowRuledOut((v) => !v)}
          >
            <ChevronDown
              className={['section-disclosure-icon', showRuledOut ? 'is-open' : ''].filter(Boolean).join(' ')}
              size={16}
              strokeWidth={1.75}
              aria-hidden="true"
            />
            {`${showRuledOut ? 'Hide' : 'Show'} ${n} ${noun}`}
          </button>
          <div id={ruledOutId} hidden={!showRuledOut}>
            {showRuledOut && (
              <ul className="section-rows">
                {ruledOut.map((item) => (
                  <li key={itemKey(item)}>{renderItem(item)}</li>
                ))}
              </ul>
            )}
          </div>
        </div>
      )}
      {footer}
    </section>
  )
}
