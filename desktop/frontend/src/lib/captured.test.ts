import { describe, expect, it } from 'vitest'
import { fromLocalInputValue, isInvalidCaptured, toLocalInputValue } from './captured'

describe('captured date conversion', () => {
  it('formats a timestamp as a local datetime-local value', () => {
    const iso = new Date(2026, 8, 28, 13, 5).toISOString()
    expect(toLocalInputValue(iso)).toBe('2026-09-28T13:05')
  })

  it('pads single-digit months, days, hours and minutes', () => {
    expect(toLocalInputValue(new Date(2026, 0, 2, 3, 4).toISOString())).toBe('2026-01-02T03:04')
  })

  it('gives an empty value for empty or invalid timestamps', () => {
    expect(toLocalInputValue('')).toBe('')
    expect(toLocalInputValue('not a date')).toBe('')
  })

  it('turns a local value into an RFC3339 UTC timestamp', () => {
    expect(fromLocalInputValue('2026-09-28T13:05')).toBe(new Date(2026, 8, 28, 13, 5).toISOString())
    expect(fromLocalInputValue('2026-09-28T13:05')).toMatch(/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}Z$/)
  })

  it('keeps an empty value empty and rejects nonsense', () => {
    expect(fromLocalInputValue('')).toBe('')
    expect(fromLocalInputValue('   ')).toBe('')
    expect(fromLocalInputValue('soon')).toBe('')
  })

  it('round-trips to the minute', () => {
    const iso = '2026-09-28T13:00:00.000Z'
    expect(fromLocalInputValue(toLocalInputValue(iso))).toBe(iso)
  })

  it('rejects years that are not four digits, impossible dates and out-of-range times', () => {
    expect(fromLocalInputValue('20255-09-28T13:05')).toBe('')
    expect(fromLocalInputValue('2026-02-31T10:00')).toBe('')
    expect(fromLocalInputValue('2026-13-01T10:00')).toBe('')
    expect(fromLocalInputValue('2026-09-28T24:00')).toBe('')
    expect(fromLocalInputValue('2026-09-28T13:60')).toBe('')
  })

  it('keeps years below 100 as written instead of shifting them to the 1900s', () => {
    const iso = fromLocalInputValue('0050-06-15T12:00')
    expect(iso).not.toBe('')
    expect(new Date(iso).getFullYear()).toBe(50)
  })

  it('flags only filled-in values that do not parse', () => {
    expect(isInvalidCaptured('')).toBe(false)
    expect(isInvalidCaptured('  ')).toBe(false)
    expect(isInvalidCaptured('2026-09-28T13:05')).toBe(false)
    expect(isInvalidCaptured('20255-09-28T13:05')).toBe(true)
  })
})
