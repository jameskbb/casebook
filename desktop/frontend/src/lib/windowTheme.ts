// The page theme at startup and the native window frame (D-14). startTheme
// runs once from main.tsx, before App renders either the main window or the
// startup error screen, so the stored choice reaches the title bar in both
// and exactly one listener follows the system scheme.
import { api } from '../api/api'
import { applyTheme, getThemePreference, watchSystemTheme, type ThemePreference } from './theme'

/** Matches the window frame to a theme choice. Errors are ignored: the page theme still applies. */
export function tellWindow(pref: ThemePreference): void {
  api.setWindowTheme(pref).catch(() => {})
}

/**
 * Applies the stored theme, tells the window, and follows the system scheme
 * while System is chosen. Returns a function that stops watching.
 */
export function startTheme(): () => void {
  const pref = getThemePreference()
  applyTheme(pref)
  tellWindow(pref)
  return watchSystemTheme()
}
