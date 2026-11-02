// Live refresh (D-10, UI-22; docs/api.md section 13). Polls ChangeToken every
// 1500 ms while the window has focus, every 5000 ms while it is visible but
// not focused (a Casebook window beside the terminal running an AI client
// still updates), once immediately on focus and on becoming visible, and not
// at all while hidden. When the token differs from the last one seen,
// onChange fires once. Tokens compare only within this app session: they live in a ref and
// are never persisted. Three consecutive failures report trouble; the next
// success clears it. Calls never overlap.
import { useCallback, useEffect, useRef, useState } from 'react'
import { api } from '../api/api'

export const POLL_INTERVAL_MS = 1500
export const BLURRED_POLL_INTERVAL_MS = 5000
export const TROUBLE_AFTER_FAILURES = 3

export interface UseChangeTokenOptions {
  /** Interval while the window has focus. */
  intervalMs?: number
  /** Interval while the window is visible but not focused. */
  blurredIntervalMs?: number
  /** Called when the token moved; receives the new token. */
  onChange: (token: string) => void
  /** Called when trouble starts (true) or ends (false). */
  onTrouble?: (trouble: boolean) => void
  /** Polling runs only while enabled (default true). */
  enabled?: boolean
}

export interface ChangeTokenControl {
  /** Records a token already acted on (after an own write and reload). */
  setBaseline: (token: string) => void
  /**
   * Forgets the baseline after a reload failed, so the next successful poll
   * reports a change and the reload is tried again.
   */
  markStale: () => void
  trouble: boolean
}

/** A baseline no token equals: the next poll always reports a change. */
const STALE = Symbol('stale')

function visible(): boolean {
  return document.visibilityState === 'visible'
}

export function useChangeToken({
  intervalMs = POLL_INTERVAL_MS,
  blurredIntervalMs = BLURRED_POLL_INTERVAL_MS,
  onChange,
  onTrouble,
  enabled = true,
}: UseChangeTokenOptions): ChangeTokenControl {
  const [trouble, setTrouble] = useState(false)
  const last = useRef<string | null | typeof STALE>(null)
  const inFlight = useRef(false)
  const failures = useRef(0)
  const troubleRef = useRef(false)
  const onChangeRef = useRef(onChange)
  const onTroubleRef = useRef(onTrouble)
  onChangeRef.current = onChange
  onTroubleRef.current = onTrouble

  const report = useCallback((next: boolean) => {
    if (troubleRef.current === next) return
    troubleRef.current = next
    setTrouble(next)
    onTroubleRef.current?.(next)
  }, [])

  const setBaseline = useCallback((token: string) => {
    last.current = token
  }, [])

  const markStale = useCallback(() => {
    last.current = STALE
  }, [])

  useEffect(() => {
    if (!enabled) return
    let stopped = false
    let timer: ReturnType<typeof setInterval> | null = null

    const poll = async () => {
      if (stopped || inFlight.current || !visible()) return
      inFlight.current = true
      try {
        const token = await api.changeToken()
        if (stopped) return
        failures.current = 0
        report(false)
        if (last.current === null) {
          last.current = token
        } else if (token !== last.current) {
          last.current = token
          onChangeRef.current(token)
        }
      } catch {
        if (stopped) return
        failures.current += 1
        if (failures.current >= TROUBLE_AFTER_FAILURES) report(true)
      } finally {
        inFlight.current = false
      }
    }

    const stop = () => {
      if (timer !== null) clearInterval(timer)
      timer = null
    }
    // (Re)starts the interval for the current state: fast while focused,
    // slow while visible but blurred, none while hidden.
    const schedule = (focused: boolean) => {
      stop()
      if (!visible()) return
      timer = setInterval(() => void poll(), focused ? intervalMs : blurredIntervalMs)
    }
    // Poll at once and restart the interval so the next tick is a full
    // interval after the immediate poll.
    const resume = () => {
      schedule(document.hasFocus())
      void poll()
    }
    const onFocus = () => {
      schedule(true)
      void poll()
    }
    const onBlur = () => schedule(false)
    const onVisibility = () => (visible() ? resume() : stop())

    window.addEventListener('focus', onFocus)
    window.addEventListener('blur', onBlur)
    document.addEventListener('visibilitychange', onVisibility)
    schedule(document.hasFocus())

    return () => {
      stopped = true
      stop()
      window.removeEventListener('focus', onFocus)
      window.removeEventListener('blur', onBlur)
      document.removeEventListener('visibilitychange', onVisibility)
    }
  }, [enabled, intervalMs, blurredIntervalMs, report])

  return { setBaseline, markStale, trouble }
}
