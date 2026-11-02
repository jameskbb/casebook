// Relative and absolute time formatting (UI-SPEC Copywriting, Relative time).

const MINUTE = 60_000
const HOUR = 60 * MINUTE

const weekday = new Intl.DateTimeFormat('en-US', { weekday: 'long' })
const monthDay = new Intl.DateTimeFormat('en-US', { month: 'short', day: 'numeric' })
const monthDayYear = new Intl.DateTimeFormat('en-US', { month: 'short', day: 'numeric', year: 'numeric' })
const absolute = new Intl.DateTimeFormat('en-US', { dateStyle: 'long', timeStyle: 'short' })

function parse(iso: string): Date | null {
  if (!iso) return null
  const d = new Date(iso)
  return Number.isNaN(d.getTime()) ? null : d
}

function startOfDay(d: Date): number {
  return new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime()
}

/**
 * "just now", "1 minute ago", "N minutes ago", "N hours ago", "yesterday",
 * a weekday within the last week, then "Jan 15" this year or "Jan 15, 2025".
 * Future times (clock skew) read "just now". Empty or invalid input gives "".
 */
export function formatRelative(iso: string, now: Date = new Date()): string {
  const d = parse(iso)
  if (!d) return ''
  const diff = now.getTime() - d.getTime()
  if (diff < MINUTE) return 'just now'
  if (diff < HOUR) {
    const m = Math.floor(diff / MINUTE)
    return m === 1 ? '1 minute ago' : `${m} minutes ago`
  }
  if (diff < 24 * HOUR) {
    const h = Math.floor(diff / HOUR)
    return h === 1 ? '1 hour ago' : `${h} hours ago`
  }
  const days = Math.round((startOfDay(now) - startOfDay(d)) / (24 * HOUR))
  if (days <= 1) return 'yesterday'
  if (days < 7) return weekday.format(d)
  if (d.getFullYear() === now.getFullYear()) return monthDay.format(d)
  return monthDayYear.format(d)
}

/** "January 15, 2026 at 9:00 AM" in local time; "" for empty input. */
export function formatAbsolute(iso: string): string {
  const d = parse(iso)
  return d ? absolute.format(d) : ''
}
