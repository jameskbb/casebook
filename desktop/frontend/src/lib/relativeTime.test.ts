import { describe, expect, it } from 'vitest'
import { formatAbsolute, formatRelative } from './relativeTime'

// Friday, October 2, 2026, 12:00 local time.
const now = new Date(2026, 9, 2, 12, 0, 0)
const ago = (ms: number) => new Date(now.getTime() - ms).toISOString()
const MIN = 60_000
const HOUR = 60 * MIN

describe('formatRelative', () => {
  it('reads recent times in minutes and hours', () => {
    expect(formatRelative(ago(30_000), now)).toBe('just now')
    expect(formatRelative(ago(MIN), now)).toBe('1 minute ago')
    expect(formatRelative(ago(5 * MIN), now)).toBe('5 minutes ago')
    expect(formatRelative(ago(59 * MIN), now)).toBe('59 minutes ago')
    expect(formatRelative(ago(HOUR), now)).toBe('1 hour ago')
    expect(formatRelative(ago(3 * HOUR), now)).toBe('3 hours ago')
    expect(formatRelative(ago(23 * HOUR), now)).toBe('23 hours ago')
  })

  it('reads older times as yesterday, a weekday or a date', () => {
    expect(formatRelative(new Date(2026, 9, 1, 8, 0).toISOString(), now)).toBe('yesterday')
    expect(formatRelative(new Date(2026, 8, 28, 12, 0).toISOString(), now)).toBe('Monday')
    expect(formatRelative(new Date(2026, 0, 15, 9, 0).toISOString(), now)).toBe('Jan 15')
    expect(formatRelative(new Date(2025, 0, 15, 9, 0).toISOString(), now)).toBe('Jan 15, 2025')
  })

  it('treats future times as just now and empty input as nothing', () => {
    expect(formatRelative(new Date(now.getTime() + 5 * MIN).toISOString(), now)).toBe('just now')
    expect(formatRelative('', now)).toBe('')
    expect(formatRelative('not a date', now)).toBe('')
  })
})

describe('formatAbsolute', () => {
  it('uses the long date and short time', () => {
    expect(formatAbsolute(new Date(2026, 0, 15, 9, 0).toISOString())).toMatch(/^January 15, 2026 at 9:00\sAM$/)
    expect(formatAbsolute('')).toBe('')
  })
})
