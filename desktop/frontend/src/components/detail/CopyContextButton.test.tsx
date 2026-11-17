import { act, fireEvent, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { Api } from '../../api/api'
import { ApiError } from '../../api/errors'
import { CopyContextProvider } from '../../state/copyContext'
import { mockApi } from '../../test/mockApi'
import { ToastProvider } from '../ui/Toaster'
import { CopyContextButton } from './CopyContextButton'

const mocks = vi.hoisted(() => ({ api: {} as Api }))
vi.mock('../../api/api', () => ({ api: mocks.api }))

function renderButton() {
  return render(
    <ToastProvider>
      <CopyContextProvider>
        <CopyContextButton investigationRef="INV-0001" />
      </CopyContextProvider>
    </ToastProvider>,
  )
}

describe('CopyContextButton', () => {
  beforeEach(() => {
    Object.assign(mocks.api, mockApi())
  })
  afterEach(() => {
    vi.useRealTimers()
  })

  it('stays enabled and shows Copied only after a slow copy finishes, for 2000 ms', async () => {
    vi.useFakeTimers()
    let resolve: () => void = () => {}
    Object.assign(mocks.api, mockApi({ copyContext: () => new Promise<void>((r) => (resolve = r)) }))
    renderButton()

    // fireEvent: user-event's own timers do not run under fake timers.
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: 'Copy for your AI' }))
      await vi.advanceTimersByTimeAsync(0)
    })
    expect(mocks.api.copyContext).toHaveBeenCalledWith('INV-0001')
    const button = screen.getByRole('button', { name: 'Copy for your AI' })
    expect(button).toBeEnabled()
    expect(button).not.toHaveClass('btn-primary')
    expect(screen.queryByRole('button', { name: 'Copied' })).not.toBeInTheDocument()

    await act(async () => {
      resolve()
      await vi.advanceTimersByTimeAsync(0)
    })
    expect(screen.getByRole('button', { name: 'Copied' })).toBeInTheDocument()
    expect(screen.getByText('Context copied to the clipboard')).toBeInTheDocument()

    await act(async () => {
      await vi.advanceTimersByTimeAsync(1999)
    })
    expect(screen.getByRole('button', { name: 'Copied' })).toBeInTheDocument()
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1)
    })
    expect(screen.getByRole('button', { name: 'Copy for your AI' })).toBeInTheDocument()
  })

  it('shows the clipboard error toast with Retry when the copy fails', async () => {
    let calls = 0
    Object.assign(
      mocks.api,
      mockApi({
        copyContext: async () => {
          calls++
          if (calls === 1) throw new ApiError('other', 'clipboard is locked', 'clipboard')
        },
      }),
    )
    const user = userEvent.setup()
    renderButton()
    await user.click(screen.getByRole('button', { name: 'Copy for your AI' }))
    const toast = await screen.findByRole('alert')
    expect(toast).toHaveTextContent('Could not copy to the clipboard. Try again.')
    expect(screen.getByRole('button', { name: 'Copy for your AI' })).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Retry' }))
    expect(mocks.api.copyContext).toHaveBeenCalledTimes(2)
    expect(await screen.findByRole('button', { name: 'Copied' })).toBeInTheDocument()
  })
})
