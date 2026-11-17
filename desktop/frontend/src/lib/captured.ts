// Captured date conversion for the evidence dialog. The engine stores UTC
// RFC3339 timestamps; a datetime-local input works in local time to the
// minute ("2026-09-28T13:05").

const pad = (n: number) => String(n).padStart(2, '0')

/** A stored timestamp as a datetime-local value in local time; "" when empty or invalid. */
export function toLocalInputValue(iso: string): string {
  if (!iso) return ''
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}

/**
 * A datetime-local value (local time) as an RFC3339 UTC timestamp; "" when
 * empty or invalid. Only four digit years are valid (WebView2 accepts up to
 * 275760, so a mistyped "20255" must not pass), and a day that does not
 * exist in its month is invalid rather than rolled into the next month.
 * Callers must treat a non-empty value that gives "" as a field error, never
 * as "no date".
 */
export function fromLocalInputValue(value: string): string {
  const v = value.trim()
  const m = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})(?::(\d{2}))?$/.exec(v)
  if (!m) return ''
  const [y, mo, d, h, mi, s] = m.slice(1).map((x) => (x === undefined ? 0 : Number(x)))
  if (mo < 1 || mo > 12 || h > 23 || mi > 59 || s > 59) return ''
  const date = new Date(2000, mo - 1, d, h, mi, s)
  // new Date maps years 0 to 99 to 1900 to 1999; set the year explicitly.
  date.setFullYear(y, mo - 1, d)
  if (Number.isNaN(date.getTime()) || date.getFullYear() !== y || date.getMonth() !== mo - 1 || date.getDate() !== d) {
    return ''
  }
  return date.toISOString()
}

/** True when value is filled in but is not a valid date and time. */
export function isInvalidCaptured(value: string): boolean {
  return value.trim() !== '' && fromLocalInputValue(value) === ''
}
