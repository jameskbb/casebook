// Whether the short codes an AI uses for records (C-001, E-004, INV-0001)
// show on screen. Off by default: a reader does not need them. The choice is
// stored in localStorage under casebook.showIds and applied as data-show-ids
// on <html>; base.css hides every code until it is set.

const STORAGE_KEY = 'casebook.showIds'

export function getShowIds(): boolean {
  try {
    return window.localStorage.getItem(STORAGE_KEY) === 'true'
  } catch {
    return false
  }
}

export function applyShowIds(show: boolean): void {
  if (show) document.documentElement.dataset.showIds = 'true'
  else delete document.documentElement.dataset.showIds
}

export function setShowIds(show: boolean): void {
  try {
    window.localStorage.setItem(STORAGE_KEY, String(show))
  } catch {
    // Storage unavailable: the choice still applies for this session.
  }
  applyShowIds(show)
}
