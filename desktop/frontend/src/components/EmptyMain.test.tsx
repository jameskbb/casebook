import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Api } from '../api/api'
import { mockApi } from '../test/mockApi'
import { ToastProvider } from './ui/Toaster'
import { EmptyMain } from './EmptyMain'

const mocks = vi.hoisted(() => ({ api: {} as Api }))
vi.mock('../api/api', () => ({ api: mocks.api }))

function renderEmpty() {
  const onNew = vi.fn()
  const onImport = vi.fn()
  render(<ToastProvider><EmptyMain onNew={onNew} onImport={onImport} /></ToastProvider>)
  return { onNew, onImport }
}

describe('first case guidance', () => {
  beforeEach(() => { Object.assign(mocks.api, mockApi()) })

  it('lets a person inspect a finding, its source and sharing labels without creating any records', async () => {
    const user = userEvent.setup()
    const { onNew, onImport } = renderEmpty()
    await user.click(screen.getByRole('button', { name: 'Open fictional margin case' }))
    expect(screen.getByText('Fictional example · Read-only')).toBeInTheDocument()
    expect(screen.getByRole('heading', { level: 1 })).toHaveFocus()
    expect(screen.queryByRole('table')).not.toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: /E-005 supports this/ }))
    expect(screen.getByRole('table', { name: 'Illustrative cost comparison' })).toBeInTheDocument()
    expect(screen.getByText(/only after you have checked it/)).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'See example summary' }))
    expect(screen.getByRole('heading', { name: 'Fictional summary preview' })).toBeInTheDocument()
    expect(screen.getByText(/C-002 · AI generated · Medium confidence/)).toBeInTheDocument()
    expect(onNew).not.toHaveBeenCalled()
    expect(onImport).not.toHaveBeenCalled()
    for (const fn of Object.values(mocks.api)) expect(fn).not.toHaveBeenCalled()
    await user.click(screen.getByRole('button', { name: 'Back to your cases' }))
    expect(screen.getByRole('heading', { name: 'Start your first case' })).toBeInTheDocument()
    await waitFor(() => expect(screen.getByRole('button', { name: 'Open fictional margin case' })).toHaveFocus())
  })

  it('opens client setup directly and returns to the example after setup closes', async () => {
    const user = userEvent.setup()
    renderEmpty()
    await user.click(screen.getByRole('button', { name: 'Connect your AI' }))
    const dialog = await screen.findByRole('dialog', { name: 'Connect AI clients' })
    await user.click(within(dialog).getByRole('button', { name: 'Done' }))
    await user.click(screen.getByRole('button', { name: 'Open fictional margin case' }))
    await user.click(screen.getByRole('button', { name: 'Connect your AI' }))
    await user.click(within(await screen.findByRole('dialog')).getByRole('button', { name: 'Done' }))
    expect(screen.getByRole('heading', { name: 'Northeast margin down 6 points in Q3' })).toBeInTheDocument()
  })

  it('starts a real case through the existing create action', async () => {
    const user = userEvent.setup()
    const { onNew } = renderEmpty()
    await user.click(screen.getByRole('button', { name: 'Open fictional margin case' }))
    await user.click(screen.getByRole('button', { name: 'New case' }))
    expect(onNew).toHaveBeenCalledOnce()
    expect(screen.queryByRole('article')).not.toBeInTheDocument()
    expect(mocks.api.createInvestigation).not.toHaveBeenCalled()
  })
})
