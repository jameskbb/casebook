// Failed loads are visible and retried (WR-03, WR-04): a list load that
// fails shows an error with Try again and is retried by the next poll, and a
// background detail reload that fails keeps the last version with a banner,
// or clears it when the investigation is gone.
import { act, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { Api } from './api/api'
import { detail } from './test/fixtures'
import { mockApi } from './test/mockApi'
import { DETAIL_GONE_COPY, DETAIL_STALE_COPY } from './components/detail/DetailView'
import { LIST_ERROR_COPY } from './components/sidebar/Sidebar'
import App from './App'

const mocks = vi.hoisted(() => ({ api: {} as Api }))
vi.mock('./api/api', () => ({ api: mocks.api }))

const TITLE = 'September cloud bill up 38%'
const internal = () => JSON.stringify({ kind: 'internal', message: 'database is locked' })

async function tick(ms: number) {
  await act(async () => {
    await vi.advanceTimersByTimeAsync(ms)
  })
}

describe('App failed loads', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    Object.defineProperty(document, 'visibilityState', { configurable: true, get: () => 'visible' })
    vi.spyOn(document, 'hasFocus').mockImplementation(() => true)
  })
  afterEach(() => {
    vi.useRealTimers()
    vi.restoreAllMocks()
  })

  it('shows a failed first list load and retries it on the next poll even when the token did not move', async () => {
    let failList = true
    Object.assign(
      mocks.api,
      mockApi({
        listInvestigations: async (o) => {
          if (failList) throw internal()
          return mockApi().listInvestigations(o)
        },
      }),
    )
    render(<App />)
    await tick(0)
    expect(screen.getByRole('alert')).toHaveTextContent(LIST_ERROR_COPY)
    expect(screen.getByRole('button', { name: 'Try again' })).toBeInTheDocument()

    failList = false
    await tick(1500)
    expect(screen.queryByText(LIST_ERROR_COPY)).toBeNull()
    expect(screen.getByRole('heading', { level: 1, name: TITLE })).toBeInTheDocument()
  })

  it('reloads the list from Try again', async () => {
    let failList = true
    Object.assign(
      mocks.api,
      mockApi({
        listInvestigations: async (o) => {
          if (failList) throw internal()
          return mockApi().listInvestigations(o)
        },
      }),
    )
    render(<App />)
    await tick(0)
    failList = false
    fireEvent.click(screen.getByRole('button', { name: 'Try again' }))
    await tick(0)
    expect(screen.queryByText(LIST_ERROR_COPY)).toBeNull()
    expect(screen.getByText('Checkout p99 doubled after the region move')).toBeInTheDocument()
  })

  it('keeps the last detail with a banner when a background reload fails, then recovers', async () => {
    let token = '1.1'
    let failDetail = false
    Object.assign(
      mocks.api,
      mockApi({
        changeToken: async () => token,
        getInvestigationDetail: async () => {
          if (failDetail) throw internal()
          return detail
        },
      }),
    )
    render(<App />)
    await tick(0)
    expect(screen.getByRole('heading', { level: 1, name: TITLE })).toBeInTheDocument()

    token = '1.2'
    failDetail = true
    await tick(1500)
    expect(screen.getByText(DETAIL_STALE_COPY)).toBeInTheDocument()
    expect(screen.getByRole('heading', { level: 1, name: TITLE })).toBeInTheDocument()

    // The token does not move again, but the failed reload is retried.
    failDetail = false
    const calls = vi.mocked(mocks.api.getInvestigationDetail).mock.calls.length
    await tick(1500)
    expect(vi.mocked(mocks.api.getInvestigationDetail).mock.calls.length).toBeGreaterThan(calls)
    expect(screen.queryByText(DETAIL_STALE_COPY)).toBeNull()

    // Once handled, a quiet token does not reload again.
    const settled = vi.mocked(mocks.api.getInvestigationDetail).mock.calls.length
    await tick(3000)
    expect(vi.mocked(mocks.api.getInvestigationDetail).mock.calls.length).toBe(settled)
  })

  it('clears the detail when the investigation is gone after a reload', async () => {
    let token = '1.1'
    let gone = false
    Object.assign(
      mocks.api,
      mockApi({
        changeToken: async () => token,
        getInvestigationDetail: async () => {
          if (gone) throw JSON.stringify({ kind: 'not_found', message: 'not found: investigation INV-0001' })
          return detail
        },
      }),
    )
    render(<App />)
    await tick(0)
    expect(screen.getByRole('heading', { level: 1, name: TITLE })).toBeInTheDocument()

    token = '1.2'
    gone = true
    await tick(1500)
    expect(screen.queryByRole('heading', { level: 1, name: TITLE })).toBeNull()
    expect(screen.getByText(DETAIL_GONE_COPY('INV-0001'))).toBeInTheDocument()
  })
})
