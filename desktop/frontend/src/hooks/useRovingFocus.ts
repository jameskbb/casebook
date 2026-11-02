// Roving tabindex (UI-SPEC Keyboard map): a list of buttons is one Tab stop.
// Up and Down move focus by one item, Home and End jump to the ends, and the
// item that last had focus keeps tabindex 0. Enter and Space activate the
// focused button natively.
import { useCallback, useRef, useState, type KeyboardEvent } from 'react'

export interface RovingItemProps {
  tabIndex: number
  ref: (el: HTMLButtonElement | null) => void
  onFocus: () => void
  'data-roving-item': true
}

export interface RovingFocus {
  itemProps(index: number): RovingItemProps
  onKeyDown(e: KeyboardEvent<HTMLElement>): void
  /** Focuses the item that holds the Tab stop. */
  focusCurrent(): void
}

/**
 * count is the number of items; preferred is the item that holds the Tab stop
 * until the user moves focus (the selected row, or the first item).
 */
export function useRovingFocus(count: number, preferred = 0): RovingFocus {
  const [current, setCurrent] = useState<number | null>(null)
  const refs = useRef<(HTMLButtonElement | null)[]>([])
  const fallback = Math.min(Math.max(preferred, 0), Math.max(count - 1, 0))
  const index = current !== null && current < count ? current : fallback

  const focusAt = useCallback(
    (i: number) => {
      if (count === 0) return
      const n = Math.min(Math.max(i, 0), count - 1)
      setCurrent(n)
      refs.current[n]?.focus()
    },
    [count],
  )

  const onKeyDown = (e: KeyboardEvent<HTMLElement>) => {
    const from = refs.current.findIndex((el) => el !== null && el === document.activeElement)
    if (from < 0 || e.altKey || e.ctrlKey || e.metaKey) return
    switch (e.key) {
      case 'ArrowDown':
        focusAt(from + 1)
        break
      case 'ArrowUp':
        focusAt(from - 1)
        break
      case 'Home':
        focusAt(0)
        break
      case 'End':
        focusAt(count - 1)
        break
      default:
        return
    }
    e.preventDefault()
  }

  return {
    itemProps: (i: number) => ({
      tabIndex: i === index ? 0 : -1,
      ref: (el: HTMLButtonElement | null) => {
        refs.current[i] = el
      },
      onFocus: () => setCurrent(i),
      'data-roving-item': true,
    }),
    onKeyDown,
    focusCurrent: () => focusAt(index),
  }
}
