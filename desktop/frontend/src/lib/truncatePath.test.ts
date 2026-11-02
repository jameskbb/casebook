import { describe, expect, it } from 'vitest'
import { truncateMiddle } from './truncatePath'

describe('truncateMiddle', () => {
  it('keeps paths of 43 characters or fewer intact', () => {
    const p43 = 'C:\\Users\\Ada\\AppData\\Local\\Casebook\\case.db'
    expect(p43).toHaveLength(43)
    expect(truncateMiddle(p43)).toBe(p43)
    expect(truncateMiddle('')).toBe('')
  })

  it('keeps the first 12 and last 28 characters of a longer path', () => {
    const p = 'C:\\Users\\Ada Lovelace\\AppData\\Local\\Casebook\\data\\casebook.db'
    const out = truncateMiddle(p)
    expect(out).toBe(`${p.slice(0, 12)}...${p.slice(-28)}`)
    expect(out).toBe('C:\\Users\\Ada...al\\Casebook\\data\\casebook.db')
    expect(out).toHaveLength(43)
  })

  it('accepts other head and tail lengths', () => {
    expect(truncateMiddle('abcdefghijklmnop', 2, 3)).toBe('ab...nop')
  })
})
