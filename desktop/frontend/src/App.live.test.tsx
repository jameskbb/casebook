import { act, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { Api } from './api/api'
import { mockApi } from './test/mockApi'
import App from './App'

const mocks = vi.hoisted(() => ({ api: {} as Api }))
vi.mock('./api/api', () => ({ api: mocks.api }))

const BANNER = "Casebook can't check for changes right now. It will keep trying."

describe('App live refresh banner', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    Object.defineProperty(document, 'visibilityState', { configurable: true, get: () => 'visible' })
    vi.spyOn(document, 'hasFocus').mockImplementation(() => true)
  })
  afterEach(() => {
    vi.useRealTimers()
    vi.restoreAllMocks()
  })

  it('shows the warning only while polling is in trouble', async () => {
    let fail = false
    Object.assign(
      mocks.api,
      mockApi({
        changeToken: async () => {
          if (fail) throw new Error('database is locked')
          return '1.1'
        },
      }),
    )
    render(<App />)
    await act(async () => {
      await vi.advanceTimersByTimeAsync(0)
    })
    expect(screen.getByRole('heading', { level: 1, name: 'September cloud bill up 38%' })).toBeInTheDocument()
    expect(screen.queryByText(BANNER)).toBeNull()

    fail = true
    await act(async () => {
      await vi.advanceTimersByTimeAsync(3000)
    })
    expect(screen.queryByText(BANNER)).toBeNull()
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1500)
    })
    expect(screen.getByText(BANNER)).toBeInTheDocument()
    expect(screen.getByRole('main')).toContainElement(screen.getByText(BANNER))

    fail = false
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1500)
    })
    expect(screen.queryByText(BANNER)).toBeNull()
  })
})
