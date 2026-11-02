// Reads the report HTML the engine renders (contextpack.ReportHTML: h1, h2,
// h3 for the activity days, p, ul, li, strong, and em for the closing line;
// every text node escaped) into the preview nodes NotePaper draws in Share
// summary. Returns null when the markup does not
// parse, and the dialog then shows the Markdown as plain text.
import type { NoteNode } from './noteTree'

const ALLOWED = new Set(['h1', 'h2', 'h3', 'p', 'ul', 'li', 'strong', 'em'])

function convert(n: Node): NoteNode[] {
  if (n.nodeType === Node.TEXT_NODE) return [{ type: 'text', text: n.textContent ?? '' }]
  if (n.nodeType !== Node.ELEMENT_NODE) return []
  const el = n as Element
  const tag = el.localName
  if (!ALLOWED.has(tag)) return Array.from(el.childNodes).flatMap(convert)
  return [{ type: 'el', tag, style: {}, children: Array.from(el.childNodes).flatMap(convert) }]
}

export function parseReportHtml(html: string): NoteNode[] | null {
  const doc = new DOMParser().parseFromString(`<body>${html}</body>`, 'text/html')
  const body = doc.body
  if (!body) return null
  const nodes = Array.from(body.childNodes).flatMap(convert)
  return nodes.length > 0 ? nodes : null
}
