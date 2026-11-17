// The summary sheet in Share summary: the report drawn from its HTML as
// React elements, on a light sheet in both themes so it reads as the page
// someone will receive.
import { createElement, type ReactNode } from 'react'
import type { NoteNode } from '../../lib/noteTree'
import './NotePaper.css'

const VOID = new Set(['br', 'hr'])

function render(nodes: NoteNode[], key = 'n'): ReactNode[] {
  return nodes.map((n, i) => {
    const k = `${key}.${i}`
    if (n.type === 'text') return n.text
    if (VOID.has(n.tag)) return createElement(n.tag, { key: k, style: n.style })
    return createElement(n.tag, { key: k, style: n.style }, ...render(n.children, k))
  })
}

export interface NotePaperProps {
  nodes: NoteNode[] | null
  /** Shown as plain text when the HTML could not be read. */
  text: string
  label: string
}

export function NotePaper({ nodes, text, label }: NotePaperProps) {
  return (
    <div className="note-paper-frame">
      <div className="note-paper" role="document" aria-label={label} tabIndex={0}>
        {nodes ? <div className="note-paper-body">{render(nodes)}</div> : <pre className="note-paper-text">{text}</pre>}
      </div>
    </div>
  )
}
