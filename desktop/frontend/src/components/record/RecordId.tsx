// A record's short code (C-001, E-004, INV-0001). Hidden until the person
// turns on "Show the codes your AI uses" (lib/showIds.ts, base.css); it stays
// in the page so tests and an AI reading the screen can still find it.
import { Fragment, type ReactNode } from 'react'

export function RecordId({ children }: { children: ReactNode }) {
  return <span className="record-id tabular">{children}</span>
}

const ID_IN_TEXT = / ?\b(?:INV-\d+|[ECHQ]-\d+)\b/g

/** Text with every code in it (and the space before it) wrapped in RecordId. */
export function hideIds(text: string): ReactNode {
  const parts: ReactNode[] = []
  let last = 0
  for (const m of text.matchAll(ID_IN_TEXT)) {
    if (m.index > last) parts.push(<Fragment key={last}>{text.slice(last, m.index)}</Fragment>)
    parts.push(<RecordId key={`id${m.index}`}>{m[0]}</RecordId>)
    last = m.index + m[0].length
  }
  if (parts.length === 0) return text
  if (last < text.length) parts.push(<Fragment key={last}>{text.slice(last)}</Fragment>)
  return parts
}
