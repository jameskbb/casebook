// The current time for relative labels ("just now", "3 minutes ago"). It
// re-renders the caller once a minute so the labels age on a quiet
// database, and returns a fresh Date on every render so a data change never
// shows a time older than the data.
import { useEffect, useState } from 'react'

export const NOW_TICK_MS = 60_000

export function useNow(intervalMs: number = NOW_TICK_MS): Date {
  const [, setTick] = useState(0)
  useEffect(() => {
    const t = setInterval(() => setTick((n) => n + 1), intervalMs)
    return () => clearInterval(t)
  }, [intervalMs])
  return new Date()
}
