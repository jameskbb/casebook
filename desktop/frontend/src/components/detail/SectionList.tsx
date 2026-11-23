// A reading-column section: a 20px kind icon, the h2 heading, then rows
// separated by 1px rules (no cards, no shadows). A section with nothing in
// it is not drawn at all. Rows a person rarely needs (answered, ruled out,
// replaced) collapse under a Collapsed line with count-aware words.
import { useId, useState, type ReactNode } from 'react'
import { ChevronDown, type LucideIcon } from 'lucide-react'
import { plural } from '../../lib/plural'
import './SectionList.css'

interface Rows<T> {
  items: T[]
  itemKey: (item: T) => string
  renderItem: (item: T) => ReactNode
}

export interface CollapsedProps<T> extends Rows<T> {
  /** Singular and plural words for the line ("Show 2 replaced findings"). */
  noun: [string, string]
}

/** "Show 2 ruled out findings": the rows appear under it when opened. Nothing when there are none. */
export function Collapsed<T>({ items, noun, itemKey, renderItem }: CollapsedProps<T>) {
  const [open, setOpen] = useState(false)
  const id = useId()
  const n = items.length
  if (n === 0) return null
  return (
    <div className="section-ruled-out">
      <button type="button" className="section-disclosure" aria-expanded={open} aria-controls={id} onClick={() => setOpen((v) => !v)}>
        <ChevronDown className={['section-disclosure-icon', open ? 'is-open' : ''].filter(Boolean).join(' ')} size={16} strokeWidth={1.75} aria-hidden="true" />
        {`${open ? 'Hide' : 'Show'} ${n} ${plural(n, noun[0], noun[1])}`}
      </button>
      <div id={id} hidden={!open}>
        {open && (
          <ul className="section-rows">
            {items.map((item) => (
              <li key={itemKey(item)}>{renderItem(item)}</li>
            ))}
          </ul>
        )}
      </div>
    </div>
  )
}

export interface SectionListProps<T> extends Rows<T> {
  /** Stable id used for the heading. */
  id: string
  icon: LucideIcon
  /** Tints the heading icon with the kind hue. */
  kind: 'evidence' | 'claim' | 'hypothesis' | 'question' | 'history'
  heading: string
  /** Rows behind a Collapsed line under the others. */
  collapsed?: T[]
  collapsedNoun?: [string, string]
  /** Rendered after the rows (the history's Show all, Show N left for later). */
  footer?: ReactNode
}

export function SectionList<T>({ id, icon: Icon, kind, heading, items, collapsed = [], collapsedNoun, itemKey, renderItem, footer }: SectionListProps<T>) {
  const headingId = `section-${id}`
  if (items.length === 0 && collapsed.length === 0 && !footer) return null
  return (
    <section className={`section section-${kind}`} aria-labelledby={headingId}>
      <div className="section-head">
        <Icon className="section-icon" size={20} strokeWidth={1.75} aria-hidden="true" />
        <h2 className="section-heading" id={headingId}>
          {heading}
        </h2>
      </div>
      {items.length > 0 && (
        <ul className="section-rows">
          {items.map((item) => (
            <li key={itemKey(item)}>{renderItem(item)}</li>
          ))}
        </ul>
      )}
      {collapsedNoun && <Collapsed items={collapsed} noun={collapsedNoun} itemKey={itemKey} renderItem={renderItem} />}
      {footer}
    </section>
  )
}
