// Keeps Tab and Shift+Tab inside a container (dialogs and drawers).

const FOCUSABLE = [
  'a[href]',
  'button:not([disabled])',
  'input:not([disabled]):not([type="hidden"])',
  'select:not([disabled])',
  'textarea:not([disabled])',
  '[tabindex]:not([tabindex="-1"])',
].join(',')

/** The tabbable elements inside container, in DOM order. */
export function focusableIn(container: HTMLElement): HTMLElement[] {
  return Array.from(container.querySelectorAll<HTMLElement>(FOCUSABLE)).filter(
    (el) => !el.hasAttribute('inert') && el.getAttribute('aria-hidden') !== 'true',
  )
}

/**
 * Traps Tab focus inside container until the returned function is called.
 * Focus that lands outside the container is pulled back in on the next Tab.
 */
export function trapFocus(container: HTMLElement): () => void {
  const onKeyDown = (e: KeyboardEvent) => {
    if (e.key !== 'Tab') return
    const items = focusableIn(container)
    if (items.length === 0) {
      e.preventDefault()
      container.focus()
      return
    }
    const first = items[0]
    const last = items[items.length - 1]
    const active = document.activeElement as HTMLElement | null
    const inside = active ? container.contains(active) : false
    if (e.shiftKey) {
      if (!inside || active === first) {
        e.preventDefault()
        last.focus()
      }
    } else if (!inside || active === last) {
      e.preventDefault()
      first.focus()
    }
  }
  container.addEventListener('keydown', onKeyDown)
  return () => container.removeEventListener('keydown', onKeyDown)
}
