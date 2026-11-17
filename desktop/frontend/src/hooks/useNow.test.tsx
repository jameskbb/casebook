import { act, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { formatRelative } from '../lib/relativeTime'
import { NOW_TICK_MS, useNow } from './useNow'

function Label({ iso }: { iso: string }) {
  const now = useNow()
  return <span>{formatRelative(iso, now)}</span>
}

describe('useNow', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-10-02T12:00:30.000Z'))
  })
  afterEach(() => {
    vi.useRealTimers()
  })

  it('ages relative labels once a minute with no data change', async () => {
    render(<Label iso="2026-10-02T12:00:00.000Z" />)
    expect(screen.getByText('just now')).toBeInTheDocument()
    await act(async () => {
      await vi.advanceTimersByTimeAsync(NOW_TICK_MS)
    })
    expect(screen.getByText('1 minute ago')).toBeInTheDocument()
    await act(async () => {
      await vi.advanceTimersByTimeAsync(2 * NOW_TICK_MS)
    })
    expect(screen.getByText('3 minutes ago')).toBeInTheDocument()
  })

  it('stops ticking on unmount', () => {
    const { unmount } = render(<Label iso="2026-10-02T12:00:00.000Z" />)
    unmount()
    expect(vi.getTimerCount()).toBe(0)
  })
})
