import { describe, expect, it } from 'vitest'

// Every non-test source file whose strings a person can read: components,
// lib, state, the app shell and the error copy. Vite reads them as raw text.
const sources = import.meta.glob<string>(
  ['./components/**/*.{ts,tsx}', './lib/**/*.{ts,tsx}', './state/**/*.{ts,tsx}', './App.tsx', './api/errors.ts'],
  { query: '?raw', import: 'default', eager: true },
)

// Visible copy says case, never investigation (ADR 0021). Identifiers,
// imports and comments are not checked here: only string literals.
describe('people-facing copy', () => {
  it('reads the source files', () => {
    expect(Object.keys(sources)).toContain('./App.tsx')
    expect(Object.keys(sources)).toContain('./api/errors.ts')
    expect(Object.keys(sources).length).toBeGreaterThan(40)
  })

  it('never shows the word investigation', () => {
    const offenders: string[] = []
    for (const [file, src] of Object.entries(sources)) {
      if (/\.test\./.test(file) || /fixtures|mockApi/.test(file)) continue
      for (const m of src.matchAll(/(['"`])((?:(?!\1).)*?\binvestigations?\b(?:(?!\1).)*?)\1/gi)) {
        // Lowercase identifier-shaped strings are not copy: a field or type
        // name such as 'investigation', or a dotted engine action key such
        // as 'investigation.create' (the event names keep the API noun).
        // Capitalised words like 'Investigations' are labels and are caught.
        if (/^[a-z_]+(\.[a-z_]+)*$/.test(m[2])) continue
        offenders.push(`${file.slice(2)}: ${m[2]}`)
      }
    }
    expect(offenders).toEqual([])
  })

  // Labels retired in plain-reader round 1, as a whole string literal or a
  // whole JSX text node: the buttons, the AI toggle and the sidebar marker
  // now say Copy for your AI, Share summary, Use with my AI, Your AI's
  // current case and Current. Review round 2 retired the filter labels
  // Unchecked and Source checked and the hyphenated AI-generated; the
  // labels read AI generated and Source verified.
  it('never shows a retired label', () => {
    const retired = ['Copy context', 'Copy report', 'Set active', 'Active for AI', 'Copy as plain Markdown', 'Active', 'Unchecked', 'Source checked', 'AI-generated']
    const offenders: string[] = []
    for (const [file, src] of Object.entries(sources)) {
      if (/\.test\./.test(file) || /fixtures|mockApi/.test(file)) continue
      for (const m of src.matchAll(/(['"`])((?:(?!\1).)*?)\1|>\s*([^<>{}]*?)\s*</g)) {
        const text = (m[2] ?? m[3] ?? '').trim()
        if (retired.includes(text)) offenders.push(`${file.slice(2)}: ${text}`)
      }
    }
    expect(offenders).toEqual([])
  })

  // Forms that are never right anywhere in copy, so they are caught inside a
  // sentence too, not only as a whole string or text node: "AI-generated"
  // with a hyphen, and the retired filter words Unchecked and Source checked.
  it('never uses a retired word inside a sentence', () => {
    const banned = [/\bAI-generated\b/, /\bUnchecked\b/, /\bSource checked\b/]
    const offenders: string[] = []
    for (const [file, src] of Object.entries(sources)) {
      if (/\.test\./.test(file) || /fixtures|mockApi/.test(file)) continue
      for (const m of src.matchAll(/(['"`])((?:(?!\1).)*?)\1|>([^<>{}]*?)</g)) {
        const text = m[2] ?? m[3] ?? ''
        if (banned.some((re) => re.test(text))) offenders.push(`${file.slice(2)}: ${text.trim()}`)
      }
    }
    expect(offenders).toEqual([])
  })
})
