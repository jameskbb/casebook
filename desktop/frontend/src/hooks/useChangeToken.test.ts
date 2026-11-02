import { act, renderHook } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { Api } from '../api/api'
import { AppDataProvider, useAppData } from '../state/appData'
import { mockApi } from '../test/mockApi'
import { useChangeToken } from './useChangeToken'

const mocks = vi.hoisted(() => ({ api: {} as Api }))
vi.mock('../api/api', () => ({ api: mocks.api }))

let visibility: DocumentVisibilityState = 'visible'
let focused = true

function setVisibility(v: DocumentVisibilityState) {
  visibility = v
  document.dispatchEvent(new Event('visibilitychange'))
}

const tick = async (ms: number) => {
  await act(async () => {
    await vi.advanceTimersByTimeAsync(ms)
  })
}

describe('useChangeToken', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    visibility = 'visible'
    focused = true
    Object.defineProperty(document, 'visibilityState', { configurable: true, get: () => visibility })
    vi.spyOn(document, 'hasFocus').mockImplementation(() => focused)
    Object.assign(mocks.api, mockApi())
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.restoreAllMocks()
  })

  it('polls every 1500 ms while visible and focused', async () => {
    renderHook(() => useChangeToken({ onChange: () => {} }))
    await tick(1499)
    expect(mocks.api.changeToken).toHaveBeenCalledTimes(0)
    await tick(1)
    expect(mocks.api.changeToken).toHaveBeenCalledTimes(1)
    await tick(1500)
    expect(mocks.api.changeToken).toHaveBeenCalledTimes(2)
  })

  it('makes no calls while hidden and polls at once when shown again', async () => {
    visibility = 'hidden'
    renderHook(() => useChangeToken({ onChange: () => {} }))
    await tick(6000)
    expect(mocks.api.changeToken).toHaveBeenCalledTimes(0)
    act(() => setVisibility('visible'))
    await tick(0)
    expect(mocks.api.changeToken).toHaveBeenCalledTimes(1)
    act(() => setVisibility('hidden'))
    await tick(6000)
    expect(mocks.api.changeToken).toHaveBeenCalledTimes(1)
  })

  it('polls every 5000 ms while visible but blurred, and immediately on focus', async () => {
    focused = false
    renderHook(() => useChangeToken({ onChange: () => {} }))
    await tick(4999)
    expect(mocks.api.changeToken).toHaveBeenCalledTimes(0)
    await tick(1)
    expect(mocks.api.changeToken).toHaveBeenCalledTimes(1)
    focused = true
    act(() => {
      window.dispatchEvent(new Event('focus'))
    })
    await tick(0)
    expect(mocks.api.changeToken).toHaveBeenCalledTimes(2)
    await tick(1500)
    expect(mocks.api.changeToken).toHaveBeenCalledTimes(3)
    focused = false
    act(() => {
      window.dispatchEvent(new Event('blur'))
    })
    await tick(4999)
    expect(mocks.api.changeToken).toHaveBeenCalledTimes(3)
    await tick(1)
    expect(mocks.api.changeToken).toHaveBeenCalledTimes(4)
  })

  it('a blurred window beside a terminal still sees a change', async () => {
    focused = false
    let token = '1.1'
    Object.assign(mocks.api, mockApi({ changeToken: async () => token }))
    const onChange = vi.fn()
    const { result } = renderHook(() => useChangeToken({ onChange }))
    act(() => result.current.setBaseline('1.1'))
    token = '1.2'
    await tick(5000)
    expect(onChange).toHaveBeenCalledWith('1.2')
  })

  it('stops while hidden even when blurred', async () => {
    focused = false
    renderHook(() => useChangeToken({ onChange: () => {} }))
    act(() => setVisibility('hidden'))
    await tick(20000)
    expect(mocks.api.changeToken).toHaveBeenCalledTimes(0)
  })

  it('markStale makes the next poll report a change for an equal token', async () => {
    const onChange = vi.fn()
    const { result } = renderHook(() => useChangeToken({ onChange }))
    act(() => result.current.setBaseline('1.1'))
    await tick(1500)
    expect(onChange).not.toHaveBeenCalled()
    act(() => result.current.markStale())
    await tick(1500)
    expect(onChange).toHaveBeenCalledTimes(1)
    await tick(3000)
    expect(onChange).toHaveBeenCalledTimes(1)
  })

  it('fires onChange once when the token moves and never for equal tokens', async () => {
    let token = '7.40'
    Object.assign(mocks.api, mockApi({ changeToken: async () => token }))
    const onChange = vi.fn()
    const { result } = renderHook(() => useChangeToken({ onChange }))
    act(() => result.current.setBaseline('7.40'))
    await tick(3000)
    expect(onChange).not.toHaveBeenCalled()
    token = '8.41'
    await tick(1500)
    expect(onChange).toHaveBeenCalledTimes(1)
    await tick(3000)
    expect(onChange).toHaveBeenCalledTimes(1)
  })

  it('takes the first token as the baseline when none was set', async () => {
    const onChange = vi.fn()
    renderHook(() => useChangeToken({ onChange }))
    await tick(4500)
    expect(onChange).not.toHaveBeenCalled()
  })

  it('reports trouble after three consecutive failures and clears it on the next success', async () => {
    let fail = true
    Object.assign(
      mocks.api,
      mockApi({
        changeToken: async () => {
          if (fail) throw new Error('database is locked')
          return '1.1'
        },
      }),
    )
    const onTrouble = vi.fn()
    const { result } = renderHook(() => useChangeToken({ onChange: () => {}, onTrouble }))
    await tick(3000)
    expect(result.current.trouble).toBe(false)
    await tick(1500)
    expect(result.current.trouble).toBe(true)
    expect(onTrouble).toHaveBeenLastCalledWith(true)
    fail = false
    await tick(1500)
    expect(result.current.trouble).toBe(false)
    expect(onTrouble).toHaveBeenLastCalledWith(false)
  })

  it('never overlaps two calls', async () => {
    Object.assign(mocks.api, mockApi({ changeToken: () => new Promise<string>(() => {}) }))
    renderHook(() => useChangeToken({ onChange: () => {} }))
    await tick(6000)
    act(() => {
      window.dispatchEvent(new Event('focus'))
    })
    await tick(0)
    expect(mocks.api.changeToken).toHaveBeenCalledTimes(1)
  })

  it('stops polling on unmount', async () => {
    const { unmount } = renderHook(() => useChangeToken({ onChange: () => {} }))
    unmount()
    await tick(6000)
    expect(mocks.api.changeToken).toHaveBeenCalledTimes(0)
  })
})

describe('AppDataProvider live refresh', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    visibility = 'visible'
    focused = true
    Object.defineProperty(document, 'visibilityState', { configurable: true, get: () => visibility })
    vi.spyOn(document, 'hasFocus').mockImplementation(() => focused)
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.restoreAllMocks()
  })

  async function mount(token: () => string) {
    Object.assign(mocks.api, mockApi({ changeToken: async () => token() }))
    const hook = renderHook(() => useAppData(), { wrapper: AppDataProvider })
    await tick(0)
    expect(hook.result.current.detail).not.toBeNull()
    return hook
  }

  it('reloads the list and the open detail only when the token changes', async () => {
    let token = '1.1'
    const { result } = await mount(() => token)
    const lists = vi.mocked(mocks.api.listInvestigations).mock.calls.length
    const details = vi.mocked(mocks.api.getInvestigationDetail).mock.calls.length
    const version = result.current.dataVersion
    await tick(4500)
    expect(mocks.api.listInvestigations).toHaveBeenCalledTimes(lists)
    expect(mocks.api.getInvestigationDetail).toHaveBeenCalledTimes(details)

    token = '2.9'
    await tick(1500)
    expect(mocks.api.listInvestigations).toHaveBeenCalledTimes(lists + 1)
    expect(mocks.api.getInvestigationDetail).toHaveBeenCalledTimes(details + 1)
    expect(mocks.api.getInvestigationDetail).toHaveBeenLastCalledWith('INV-0001', { activity_limit: 20 })
    expect(result.current.dataVersion).toBe(version + 1)
  })

  it('stores the token after an own write so the next poll does not reload again', async () => {
    let token = '1.1'
    const { result } = await mount(() => token)
    // An own write moves the token; useWrite then calls refresh().
    token = '1.2'
    await act(async () => {
      await result.current.refresh()
    })
    const details = vi.mocked(mocks.api.getInvestigationDetail).mock.calls.length
    await tick(4500)
    expect(mocks.api.getInvestigationDetail).toHaveBeenCalledTimes(details)
  })

  it('exposes pollTrouble after three failed polls', async () => {
    let fail = false
    Object.assign(
      mocks.api,
      mockApi({
        changeToken: async () => {
          if (fail) throw new Error('busy')
          return '1.1'
        },
      }),
    )
    const { result } = renderHook(() => useAppData(), { wrapper: AppDataProvider })
    await tick(0)
    expect(result.current.pollTrouble).toBe(false)
    fail = true
    await tick(4500)
    expect(result.current.pollTrouble).toBe(true)
    fail = false
    await tick(1500)
    expect(result.current.pollTrouble).toBe(false)
  })
})
