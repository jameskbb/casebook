// Findings not tested yet that a person left for later, per question. Only
// this window remembers them (localStorage, casebook.later.<question code>):
// the finding itself does not change, so the AI and the shared answer still
// see it as not tested yet.

const key = (invRef: string) => `casebook.later.${invRef}`

export function getLater(invRef: string): string[] {
  try {
    const refs: unknown = JSON.parse(window.localStorage.getItem(key(invRef)) ?? '[]')
    return Array.isArray(refs) ? refs.filter((r): r is string => typeof r === 'string') : []
  } catch {
    return []
  }
}

export function setLater(invRef: string, refs: string[]): void {
  try {
    window.localStorage.setItem(key(invRef), JSON.stringify(refs))
  } catch {
    // Storage unavailable: the choice still applies until the window closes.
  }
}
