import { act, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { Api } from '../../api/api'
import { mockApi } from '../../test/mockApi'
import { startTheme } from '../../lib/windowTheme'
import { ThemeSwitch } from './ThemeSwitch'

const mocks = vi.hoisted(() => ({ api: {} as Api }))
vi.mock('../../api/api', () => ({ api: mocks.api }))

/** A matchMedia whose dark-scheme answer the test can flip. */
function installMatchMedia(initialDark: boolean) {
  let dark = initialDark
  const listeners = new Set<() => void>()
  const original = window.matchMedia
  window.matchMedia = ((query: string) => ({
    get matches() {
      return query.includes('dark') ? dark : false
    },
    media: query,
    onchange: null,
    addEventListener: (_: string, fn: () => void) => listeners.add(fn),
    removeEventListener: (_: string, fn: () => void) => listeners.delete(fn),
    addListener: () => {},
    removeListener: () => {},
    dispatchEvent: () => false,
  })) as unknown as typeof window.matchMedia
  return {
    setDark(v: boolean) {
      dark = v
      for (const fn of [...listeners]) fn()
    },
    restore() {
      window.matchMedia = original
    },
    listenerCount: () => listeners.size,
  }
}

describe('ThemeSwitch', () => {
  let media: ReturnType<typeof installMatchMedia>

  beforeEach(() => {
    Object.assign(mocks.api, mockApi())
    window.localStorage.clear()
    delete document.documentElement.dataset.theme
    media = installMatchMedia(false)
  })
  afterEach(() => {
    media.restore()
    window.localStorage.clear()
  })

  it('offers System, Light and Dark with System chosen by default', () => {
    render(<ThemeSwitch />)
    const group = screen.getByRole('radiogroup', { name: 'Theme' })
    expect(group).toBeInTheDocument()
    expect(screen.getByRole('radio', { name: 'System' })).toHaveAttribute('aria-checked', 'true')
    expect(screen.getByRole('radio', { name: 'Light' })).toHaveAttribute('aria-checked', 'false')
    expect(screen.getByRole('radio', { name: 'Dark' })).toHaveAttribute('aria-checked', 'false')
  })

  it('stores and applies Dark and tells the window', async () => {
    const user = userEvent.setup()
    render(<ThemeSwitch />)
    await user.click(screen.getByRole('radio', { name: 'Dark' }))
    expect(window.localStorage.getItem('casebook.theme')).toBe('dark')
    expect(document.documentElement.dataset.theme).toBe('dark')
    expect(mocks.api.setWindowTheme).toHaveBeenLastCalledWith('dark')
    expect(screen.getByRole('radio', { name: 'Dark' })).toHaveAttribute('aria-checked', 'true')
  })

  it('matches the window frame to the stored choice on start, before any sidebar renders', async () => {
    window.localStorage.setItem('casebook.theme', 'light')
    const stop = startTheme()
    expect(document.documentElement.dataset.theme).toBe('light')
    await waitFor(() => expect(mocks.api.setWindowTheme).toHaveBeenCalledWith('light'))
    expect(mocks.api.setWindowTheme).toHaveBeenCalledTimes(1)
    render(<ThemeSwitch />)
    expect(screen.getByRole('radio', { name: 'Light' })).toHaveAttribute('aria-checked', 'true')
    expect(mocks.api.setWindowTheme).toHaveBeenCalledTimes(1)
    stop()
  })

  it('registers exactly one system scheme listener', () => {
    const stop = startTheme()
    expect(media.listenerCount()).toBe(1)
    const { unmount } = render(<ThemeSwitch />)
    expect(media.listenerCount()).toBe(1)
    unmount()
    stop()
    expect(media.listenerCount()).toBe(0)
  })

  it('ignores a failing window theme call', async () => {
    Object.assign(mocks.api, {
      setWindowTheme: vi.fn(async () => {
        throw JSON.stringify({ kind: 'internal', message: 'no window' })
      }),
    })
    const user = userEvent.setup()
    render(<ThemeSwitch />)
    await user.click(screen.getByRole('radio', { name: 'Light' }))
    expect(document.documentElement.dataset.theme).toBe('light')
  })

  it('follows the system scheme live while System is chosen', async () => {
    const user = userEvent.setup()
    const stop = startTheme()
    render(<ThemeSwitch />)
    await user.click(screen.getByRole('radio', { name: 'System' }))
    expect(document.documentElement.dataset.theme).toBe('light')
    act(() => media.setDark(true))
    expect(document.documentElement.dataset.theme).toBe('dark')

    await user.click(screen.getByRole('radio', { name: 'Light' }))
    act(() => media.setDark(false))
    act(() => media.setDark(true))
    expect(document.documentElement.dataset.theme).toBe('light')
    stop()
  })
})
