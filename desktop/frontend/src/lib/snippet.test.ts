import { describe, expect, it } from 'vitest'
import { hitSegments, parseSnippet } from './snippet'

describe('parseSnippet', () => {
  it('splits marked and plain segments', () => {
    expect(parseSnippet('the **debug** level')).toEqual([
      { text: 'the ', mark: false },
      { text: 'debug', mark: true },
      { text: ' level', mark: false },
    ])
  })

  it('handles several marks and a mark at either end', () => {
    expect(parseSnippet('**a** and **b**')).toEqual([
      { text: 'a', mark: true },
      { text: ' and ', mark: false },
      { text: 'b', mark: true },
    ])
  })

  it('treats the rest as plain after an unmatched marker', () => {
    expect(parseSnippet('one **two** three **four')).toEqual([
      { text: 'one ', mark: false },
      { text: 'two', mark: true },
      { text: ' three four', mark: false },
    ])
  })

  it('returns plain text unchanged, markup included', () => {
    expect(parseSnippet('<b>x</b> ...')).toEqual([{ text: '<b>x</b> ...', mark: false }])
  })

  it('returns nothing for an empty snippet', () => {
    expect(parseSnippet('')).toEqual([])
    expect(parseSnippet('****')).toEqual([])
  })
})

describe('hitSegments', () => {
  it('uses engine segments, so a literal ** stays text', () => {
    const segs = hitSegments({
      snippet: 'a ** b **hit**',
      segments: [
        { text: 'a ** b ', match: false },
        { text: 'hit', match: true },
        { text: '', match: false },
      ],
    })
    expect(segs).toEqual([
      { text: 'a ** b ', mark: false },
      { text: 'hit', mark: true },
    ])
  })

  it('falls back to parsing the ** snippet', () => {
    expect(hitSegments({ snippet: 'x **y**' })).toEqual([
      { text: 'x ', mark: false },
      { text: 'y', mark: true },
    ])
  })
})
