// The global keyboard map (D-22; UI-SPEC Interaction Contract, Keyboard map):
// "/" or Ctrl+F focuses the sidebar search and selects its text, Ctrl+N
// opens New case, Ctrl+Shift+C copies context for the open
// investigation. Cmd replaces Ctrl on macOS. While any menu, dialog or drawer
// is open these stand down; Esc belongs to the layer stack (lib/layers) and
// to the search field. "/" typed into a text field is just text.
import { useEffect, useRef } from 'react'
import { hasOpenLayer } from '../lib/layers'

export const SEARCH_FIELD_SELECTOR = '#sidebar-search'

export interface ShortcutHandlers {
  onNew: () => void
  /** Copy for your AI for the open investigation; absent when none is open. */
  onCopyContext?: () => void
}

export function isMacPlatform(): boolean {
  const nav = window.navigator
  return /Mac/i.test(nav.platform || '') || /Mac OS X|Macintosh/.test(nav.userAgent || '')
}

function isTextTarget(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false
  if (target.isContentEditable) return true
  const tag = target.tagName
  return tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT'
}

function focusSearch(): boolean {
  const field = document.querySelector<HTMLInputElement>(SEARCH_FIELD_SELECTOR)
  if (!field) return false
  field.focus()
  field.select()
  return true
}

export function useGlobalShortcuts(handlers: ShortcutHandlers): void {
  const latest = useRef(handlers)
  latest.current = handlers

  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.defaultPrevented || e.isComposing || hasOpenLayer()) return
      const mac = isMacPlatform()
      const mod = mac ? e.metaKey && !e.ctrlKey : e.ctrlKey && !e.metaKey
      const key = e.key.length === 1 ? e.key.toLowerCase() : e.key

      if (e.key === '/' && !e.ctrlKey && !e.metaKey && !e.altKey) {
        if (isTextTarget(e.target)) return
        if (focusSearch()) e.preventDefault()
        return
      }
      if (!mod || e.altKey) return
      if (key === 'f' && !e.shiftKey) {
        if (focusSearch()) e.preventDefault()
      } else if (key === 'n' && !e.shiftKey) {
        e.preventDefault()
        latest.current.onNew()
      } else if (key === 'c' && e.shiftKey) {
        const copy = latest.current.onCopyContext
        if (copy) {
          e.preventDefault()
          copy()
        }
      }
    }
    document.addEventListener('keydown', onKeyDown)
    return () => document.removeEventListener('keydown', onKeyDown)
  }, [])
}
