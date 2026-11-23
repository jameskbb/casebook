// A short code (C-001, E-004, INV-0001). Hidden until the person turns on
// "Show the codes your AI uses" (lib/showIds.ts, base.css); it stays in the
// page so tests and an AI reading the screen can still find it.
import type { ReactNode } from 'react'

export function RecordId({ children }: { children: ReactNode }) {
  return <span className="record-id tabular">{children}</span>
}

const CODE = '(?:INV-\\d+|[ECHQ]-\\d+)'
// "(E-007)" or "(H-001, E-006)" with the space before it, or one bare code.
const TOKEN = new RegExp(` ?\\((?:\\s*${CODE}\\s*(?:,|;|and)?)+\\)|( ?)\\b(${CODE})\\b`, 'g')

/** What a bare code stands for when codes are hidden and its own words are not known. */
const NOUN: Record<string, string> = { E: 'a source', C: 'a finding', H: 'a finding', Q: 'a follow-up question', I: 'this question' }

/**
 * Text someone wrote (a note, how you know, an answer to a follow-up
 * question) that may name codes. With codes hidden it still reads as a
 * sentence: a bracket holding only codes goes away with its brackets, and a
 * bare code is replaced by that finding's or source's own words, or by "a
 * finding" or "a source". With codes on, the text shows as written.
 */
export function CodedText({ text, nameOf }: { text: string; nameOf?: (ref: string) => string | undefined }) {
  const parts: ReactNode[] = []
  let last = 0
  for (const m of text.matchAll(TOKEN)) {
    parts.push(text.slice(last, m.index))
    last = m.index + m[0].length
    if (!m[2]) {
      parts.push(<RecordId key={m.index}>{m[0]}</RecordId>)
      continue
    }
    const name = nameOf?.(m[2])
    let words = name ? `"${name}"` : NOUN[m[2][0]]
    // The first word of a sentence takes a capital.
    if (!name && /(^|[.!?]\s*)$/.test(text.slice(0, m.index) + m[1])) words = words[0].toUpperCase() + words.slice(1)
    parts.push(
      <RecordId key={m.index}>{m[0]}</RecordId>,
      <span key={`w${m.index}`} className="codes-off">
        {m[1]}
        {words}
      </span>,
    )
  }
  if (last === 0) return <>{text}</>
  parts.push(text.slice(last))
  return <>{parts}</>
}
