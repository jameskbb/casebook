import { afterEach, describe, expect, it, vi } from 'vitest'
import { copyRich } from './richCopy'

class FakeItem {
  constructor(public parts: Record<string, Blob>) {}
}

describe('copyRich', () => {
  afterEach(() => vi.unstubAllGlobals())

  it('puts the html and the plain text on the clipboard in one item', async () => {
    const write = vi.fn(async () => undefined)
    vi.stubGlobal('ClipboardItem', FakeItem)
    vi.stubGlobal('navigator', { clipboard: { write } })
    expect(await copyRich('<h1>Why?</h1>', 'Why?')).toBe(true)
    const [[items]] = write.mock.calls as unknown as [[FakeItem[]]]
    expect(items).toHaveLength(1)
    expect(Object.keys(items[0].parts)).toEqual(['text/html', 'text/plain'])
    expect(await items[0].parts['text/html'].text()).toBe('<h1>Why?</h1>')
    expect(await items[0].parts['text/plain'].text()).toBe('Why?')
  })

  it('says false when the window has no rich clipboard, or the write is refused', async () => {
    vi.stubGlobal('ClipboardItem', undefined)
    expect(await copyRich('<p>a</p>', 'a')).toBe(false)
    vi.stubGlobal('ClipboardItem', FakeItem)
    vi.stubGlobal('navigator', { clipboard: { write: async () => Promise.reject(new Error('denied')) } })
    expect(await copyRich('<p>a</p>', 'a')).toBe(false)
  })
})
