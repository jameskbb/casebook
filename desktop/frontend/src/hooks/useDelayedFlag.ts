// True only once `active` has stayed true for `delay` ms, so a loading
// indicator never flashes for fast loads (UI-SPEC Motion: nothing appears
// before 300 ms).
import { useEffect, useState } from 'react'

export const LOADING_DELAY_MS = 300

export function useDelayedFlag(active: boolean, delay: number = LOADING_DELAY_MS): boolean {
  const [shown, setShown] = useState(false)
  useEffect(() => {
    if (!active) {
      setShown(false)
      return
    }
    const t = setTimeout(() => setShown(true), delay)
    return () => clearTimeout(t)
  }, [active, delay])
  return active && shown
}
