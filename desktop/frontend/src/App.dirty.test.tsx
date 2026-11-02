// Unsaved drawer and conclusion edits are never dropped by navigation
// (WR-02): push, back, close (X, Esc, scrim) and opening another
// investigation ask "Discard this draft?" first.
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Api } from './api/api'
import { mockApi } from './test/mockApi'
import App from './App'

const mocks = vi.hoisted(() => ({ api: {} as Api }))
vi.mock('./api/api', () => ({ api: mocks.api }))

const DISCARD = 'Discard this draft?'

async function openClaimWithDraft() {
  const user = userEvent.setup()
  render(<App />)
  await user.click(await screen.findByRole('button', { name: /^Open claim C-001:/ }))
  const drawer = screen.getByRole('dialog', { name: 'Claim C-001' })
  await user.click(within(drawer).getByRole('button', { name: 'Edit statement' }))
  await user.type(within(drawer).getByLabelText('Statement'), ' Draft words.')
  return { user, drawer }
}

describe('dirty drawer and conclusion editors', () => {
  beforeEach(() => {
    Object.assign(mocks.api, mockApi())
  })

  it('asks before the scrim closes a drawer with a dirty editor, and Keep editing keeps the draft', async () => {
    const { user } = await openClaimWithDraft()
    fireEvent.mouseDown(document.querySelector('.drawer-scrim')!)
    const confirm = screen.getByRole('dialog', { name: DISCARD })
    await user.click(within(confirm).getByRole('button', { name: 'Keep editing' }))
    expect(screen.queryByRole('dialog', { name: DISCARD })).toBeNull()
    const drawer = screen.getByRole('dialog', { name: 'Claim C-001' })
    expect((within(drawer).getByLabelText('Statement') as HTMLTextAreaElement).value).toContain('Draft words.')
  })

  it('closes after Discard', async () => {
    const { user, drawer } = await openClaimWithDraft()
    await user.click(within(drawer).getByRole('button', { name: 'Close' }))
    await user.click(within(screen.getByRole('dialog', { name: DISCARD })).getByRole('button', { name: 'Discard' }))
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Claim C-001' })).toBeNull())
    expect(mocks.api.updateClaim).not.toHaveBeenCalled()
  })

  it('asks before pushing another drawer over a dirty editor', async () => {
    const { user, drawer } = await openClaimWithDraft()
    const block = within(drawer).getAllByRole('button', { name: /^Open evidence E-/ })[0]
    await user.click(block)
    expect(screen.getByRole('dialog', { name: DISCARD })).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Keep editing' }))
    expect(screen.getByRole('dialog', { name: 'Claim C-001' })).toBeInTheDocument()
    expect((screen.getByLabelText('Statement') as HTMLTextAreaElement).value).toContain('Draft words.')

    await user.click(within(screen.getByRole('dialog', { name: 'Claim C-001' })).getAllByRole('button', { name: /^Open evidence E-/ })[0])
    await user.click(screen.getByRole('button', { name: 'Discard' }))
    await waitFor(() => expect(screen.getByRole('dialog', { name: /^Evidence E-/ })).toBeInTheDocument())
  })

  it('closes a clean drawer at once', async () => {
    const user = userEvent.setup()
    render(<App />)
    await user.click(await screen.findByRole('button', { name: /^Open claim C-001:/ }))
    fireEvent.mouseDown(document.querySelector('.drawer-scrim')!)
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Claim C-001' })).toBeNull())
    expect(screen.queryByRole('dialog', { name: DISCARD })).toBeNull()
  })

  it('asks before opening another investigation over a dirty conclusion, and only then switches', async () => {
    const user = userEvent.setup()
    render(<App />)
    const panel = await screen.findByRole('region', { name: 'Current conclusion' })
    await user.click(within(panel).getByRole('button', { name: 'Update conclusion' }))
    await user.type(within(panel).getByLabelText('Conclusion'), ' Unsaved.')

    const other = screen.getByText('Checkout p99 doubled after the region move').closest('button')!
    await user.click(other)
    expect(screen.getByRole('dialog', { name: DISCARD })).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Keep editing' }))
    expect(mocks.api.getInvestigationDetail).not.toHaveBeenCalledWith('INV-0002', expect.anything())
    const box = within(screen.getByRole('region', { name: 'Current conclusion' })).getByLabelText('Conclusion')
    expect((box as HTMLTextAreaElement).value).toContain('Unsaved.')

    await user.click(other)
    await user.click(screen.getByRole('button', { name: 'Discard' }))
    await waitFor(() => expect(mocks.api.getInvestigationDetail).toHaveBeenCalledWith('INV-0002', expect.anything()))
  })

  it('does not ask when closing a clean drawer over a dirty conclusion', async () => {
    const user = userEvent.setup()
    render(<App />)
    const panel = await screen.findByRole('region', { name: 'Current conclusion' })
    await user.click(within(panel).getByRole('button', { name: 'Update conclusion' }))
    await user.type(within(panel).getByLabelText('Conclusion'), ' Unsaved.')
    await user.click(screen.getByRole('button', { name: /^Open claim C-001:/ }))
    await user.click(within(screen.getByRole('dialog', { name: 'Claim C-001' })).getByRole('button', { name: 'Close' }))
    expect(screen.queryByRole('dialog', { name: DISCARD })).toBeNull()
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Claim C-001' })).toBeNull())
  })
})

describe('restore and unsaved drafts (AR-24)', () => {
  beforeEach(() => {
    Object.assign(mocks.api, mockApi())
  })

  it('asks about a dirty draft before replacing any data, and Keep editing replaces nothing', async () => {
    const { user } = await openClaimWithDraft()
    await user.click(screen.getByRole('button', { name: 'Data' }))
    await user.click(screen.getByRole('menuitem', { name: 'Restore from backup...' }))
    const confirm = await screen.findByRole('dialog', { name: 'Replace all Casebook data with this backup?' })
    await user.click(within(confirm).getByRole('button', { name: 'Replace all data' }))
    const discard = screen.getByRole('dialog', { name: DISCARD })
    expect(mocks.api.restoreBackup).not.toHaveBeenCalled()
    await user.click(within(discard).getByRole('button', { name: 'Keep editing' }))
    expect(mocks.api.restoreBackup).not.toHaveBeenCalled()

    await user.click(
      within(screen.getByRole('dialog', { name: 'Replace all Casebook data with this backup?' })).getByRole('button', {
        name: 'Replace all data',
      }),
    )
    await user.click(within(screen.getByRole('dialog', { name: DISCARD })).getByRole('button', { name: 'Discard' }))
    await waitFor(() => expect(mocks.api.restoreBackup).toHaveBeenCalledTimes(1))
    expect(await screen.findByRole('dialog', { name: 'Restore complete' })).toBeInTheDocument()
    // Discard was asked once, before the restore, never after it.
    expect(screen.queryByRole('dialog', { name: DISCARD })).toBeNull()
  })
})
