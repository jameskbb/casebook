// Theme switch (D-14; UI-SPEC Accessibility Floor, Theme): System, Light and
// Dark. The choice is stored as casebook.theme, applied as data-theme on
// <html>, and sent to Go so the window title bar matches. Following the
// operating system scheme live, and telling the window the stored choice on
// start, happen once in startTheme (lib/windowTheme.ts), not here.
import { useState } from 'react'
import { Monitor, Moon, Sun } from 'lucide-react'
import { getThemePreference, setThemePreference, type ThemePreference } from '../../lib/theme'
import { tellWindow } from '../../lib/windowTheme'
import { SegmentedControl } from '../ui/SegmentedControl'

const OPTIONS = [
  { value: 'system' as const, label: 'System', icon: Monitor },
  { value: 'light' as const, label: 'Light', icon: Sun },
  { value: 'dark' as const, label: 'Dark', icon: Moon },
]

export function ThemeSwitch() {
  const [pref, setPref] = useState<ThemePreference>(getThemePreference)

  const choose = (next: ThemePreference) => {
    setThemePreference(next)
    setPref(next)
    tellWindow(next)
  }

  return (
    <SegmentedControl
      label="Theme"
      hideLabel
      className="theme-switch"
      iconOnly
      options={OPTIONS}
      value={pref}
      onChange={choose}
    />
  )
}
