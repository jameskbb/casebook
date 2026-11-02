// Theme preference (UI-SPEC Accessibility Floor): System follows
// prefers-color-scheme live; Light and Dark are fixed. The choice is stored
// in localStorage under casebook.theme and applied as data-theme on <html>.

export type ThemePreference = 'system' | 'light' | 'dark'
export type ResolvedTheme = 'light' | 'dark'

const STORAGE_KEY = 'casebook.theme'
const DARK_QUERY = '(prefers-color-scheme: dark)'

function isPreference(v: unknown): v is ThemePreference {
  return v === 'system' || v === 'light' || v === 'dark'
}

export function getThemePreference(): ThemePreference {
  try {
    const v = window.localStorage.getItem(STORAGE_KEY)
    return isPreference(v) ? v : 'system'
  } catch {
    return 'system'
  }
}

export function setThemePreference(pref: ThemePreference): void {
  try {
    window.localStorage.setItem(STORAGE_KEY, pref)
  } catch {
    // Storage unavailable: the choice still applies for this session.
  }
  applyTheme(pref)
}

function systemPrefersDark(): boolean {
  return typeof window.matchMedia === 'function' && window.matchMedia(DARK_QUERY).matches
}

export function resolveTheme(pref: ThemePreference): ResolvedTheme {
  if (pref === 'system') return systemPrefersDark() ? 'dark' : 'light'
  return pref
}

export function applyTheme(pref: ThemePreference): ResolvedTheme {
  const resolved = resolveTheme(pref)
  document.documentElement.dataset.theme = resolved
  return resolved
}

/**
 * Re-applies the theme whenever the system scheme changes while the
 * preference is System. Returns a function that stops watching.
 */
export function watchSystemTheme(onChange?: (theme: ResolvedTheme) => void): () => void {
  if (typeof window.matchMedia !== 'function') return () => {}
  const mq = window.matchMedia(DARK_QUERY)
  const handler = () => {
    if (getThemePreference() === 'system') {
      const t = applyTheme('system')
      onChange?.(t)
    }
  }
  mq.addEventListener('change', handler)
  return () => mq.removeEventListener('change', handler)
}
