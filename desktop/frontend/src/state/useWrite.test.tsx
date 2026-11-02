import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Api } from '../api/api'
import { ApiError } from '../api/errors'
import { ToastProvider } from '../components/ui/Toaster'
import { mockApi } from '../test/mockApi'
import { AppDataProvider, useAppData } from './appData'
import { useWrite } from './useWrite'

const mocks = vi.hoisted(() => ({ api: {} as Api }))
vi.mock('../api/api', () => ({ api: mocks.api }))

function Harness({ fn, onResult }: { fn: () => Promise<string>; onResult: (v: unknown) => void }) {
  const write = useWrite()
  const { listStatus } = useAppData()
  if (listStatus !== 'ready') return null
  return (
    <button
      type="button"
      onClick={() => {
        write(fn).then(onResult, (e: unknown) => onResult(e))
      }}
    >
      Save
    </button>
  )
}

function renderHarness(fn: () => Promise<string>) {
  const onResult = vi.fn()
  render(
    <ToastProvider>
      <AppDataProvider>
        <Harness fn={fn} onResult={onResult} />
      </AppDataProvider>
    </ToastProvider>,
  )
  return onResult
}

function renderToggleable(fn: () => Promise<string>) {
  const onResult = vi.fn()
  const view = (shown: boolean) => (
    <ToastProvider>
      <AppDataProvider>{shown && <Harness fn={fn} onResult={onResult} />}</AppDataProvider>
    </ToastProvider>
  )
  const r = render(view(true))
  return { onResult, hide: () => r.rerender(view(false)) }
}

describe('useWrite stale Retry toasts (WR-01)', () => {
  beforeEach(() => {
    Object.assign(mocks.api, mockApi())
  })

  it('dismisses the Retry toast once a later write from the same component succeeds', async () => {
    const fn = vi
      .fn<() => Promise<string>>()
      .mockRejectedValueOnce(new ApiError('busy', 'Casebook database is busy; try again'))
      .mockResolvedValue('saved')
    const user = userEvent.setup()
    renderHarness(fn)
    await user.click(await screen.findByRole('button', { name: 'Save' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('Casebook is busy')

    // The user presses Save again instead of Retry; it succeeds.
    await user.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() => expect(fn).toHaveBeenCalledTimes(2))
    await waitFor(() => expect(screen.queryByRole('alert')).not.toBeInTheDocument())
    expect(fn).toHaveBeenCalledTimes(2)
  })

  it('dismisses the Retry toast when the component unmounts, so a closed form never submits again', async () => {
    const fn = vi.fn<() => Promise<string>>().mockRejectedValueOnce(new ApiError('busy', 'busy')).mockResolvedValue('saved')
    const user = userEvent.setup()
    const { hide } = renderToggleable(fn)
    await user.click(await screen.findByRole('button', { name: 'Save' }))
    expect(await screen.findByRole('alert')).toBeInTheDocument()
    hide()
    await waitFor(() => expect(screen.queryByRole('alert')).not.toBeInTheDocument())
    expect(fn).toHaveBeenCalledTimes(1)
  })

  it('keeps toasts without Retry after unmount', async () => {
    const fn = vi.fn<() => Promise<string>>().mockRejectedValue(new ApiError('conflict', 'conflict: claim C-001 is superseded'))
    const user = userEvent.setup()
    const { hide } = renderToggleable(fn)
    await user.click(await screen.findByRole('button', { name: 'Save' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('Claim C-001 is superseded')
    hide()
    expect(screen.getByRole('alert')).toBeInTheDocument()
  })
})

describe('useWrite', () => {
  beforeEach(() => {
    Object.assign(mocks.api, mockApi())
  })

  it('returns the value and refreshes after a successful write', async () => {
    const user = userEvent.setup()
    const onResult = renderHarness(async () => 'saved')
    await user.click(await screen.findByRole('button', { name: 'Save' }))
    await waitFor(() => expect(onResult).toHaveBeenCalledWith('saved'))
    expect(mocks.api.listInvestigations).toHaveBeenCalledTimes(2)
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('offers Retry on busy and repeats the same call', async () => {
    const fn = vi
      .fn<() => Promise<string>>()
      .mockRejectedValueOnce(new ApiError('busy', 'Casebook database is busy; try again'))
      .mockResolvedValueOnce('second time')
    const user = userEvent.setup()
    const onResult = renderHarness(fn)
    await user.click(await screen.findByRole('button', { name: 'Save' }))
    const toast = await screen.findByRole('alert')
    expect(toast).toHaveTextContent('Casebook is busy with another change. Nothing was saved.')
    expect(onResult).toHaveBeenCalledWith(undefined)

    await user.click(within(toast).getByRole('button', { name: 'Retry' }))
    await waitFor(() => expect(fn).toHaveBeenCalledTimes(2))
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('offers Retry on internal errors with the generic copy', async () => {
    const user = userEvent.setup()
    renderHarness(async () => {
      throw new ApiError('internal', 'casebook: internal storage error')
    })
    await user.click(await screen.findByRole('button', { name: 'Save' }))
    const toast = await screen.findByRole('alert')
    expect(toast).toHaveTextContent(
      'Something went wrong and nothing was saved. Try again. If it keeps happening, back up your data and restart Casebook.',
    )
    expect(within(toast).getByRole('button', { name: 'Retry' })).toBeInTheDocument()
  })

  it('appends the refresh hint on not_found and offers Refresh', async () => {
    const user = userEvent.setup()
    renderHarness(async () => {
      throw new ApiError('not_found', 'not found: claim C-009 in INV-0001')
    })
    await user.click(await screen.findByRole('button', { name: 'Save' }))
    const toast = await screen.findByRole('alert')
    expect(toast).toHaveTextContent('Claim C-009 in INV-0001. It may have changed in another app.')
    const before = vi.mocked(mocks.api.listInvestigations).mock.calls.length
    await user.click(within(toast).getByRole('button', { name: 'Refresh' }))
    await waitFor(() => expect(vi.mocked(mocks.api.listInvestigations).mock.calls.length).toBeGreaterThan(before))
  })

  it('rethrows validation errors for the form to show', async () => {
    const user = userEvent.setup()
    const onResult = renderHarness(async () => {
      throw new ApiError('validation', 'is required', 'title')
    })
    await user.click(await screen.findByRole('button', { name: 'Save' }))
    await waitFor(() => expect(onResult).toHaveBeenCalled())
    expect(onResult.mock.calls[0][0]).toMatchObject({ kind: 'validation', field: 'title' })
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })
})
