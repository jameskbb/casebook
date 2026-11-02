import type { ReactNode } from 'react'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Api } from '../../api/api'
import { AppDataProvider } from '../../state/appData'
import { investigation } from '../../test/fixtures'
import { archivedDetail } from '../../test/fixtures.detail'
import { mockApi } from '../../test/mockApi'
import { ToastProvider } from '../ui/Toaster'
import { StatusPicker } from './StatusPicker'

const mocks = vi.hoisted(() => ({ api: {} as Api }))
vi.mock('../../api/api', () => ({ api: mocks.api }))

const wrap = (ui: ReactNode) =>
  render(
    <ToastProvider>
      <AppDataProvider>{ui}</AppDataProvider>
    </ToastProvider>,
  )

describe('StatusPicker', () => {
  beforeEach(() => {
    Object.assign(mocks.api, mockApi())
  })

  it('lists New, Investigating, Blocked and Resolved with the current one checked', async () => {
    const user = userEvent.setup()
    wrap(<StatusPicker investigation={investigation} />)
    const button = screen.getByRole('button', { name: 'Status: Investigating' })
    expect(button).toHaveTextContent('Investigating')
    await user.click(button)
    const items = screen.getAllByRole('menuitemradio')
    expect(items.map((i) => i.textContent)).toEqual(['New', 'Investigating', 'Blocked', 'Resolved'])
    expect(screen.getByRole('menuitemradio', { name: 'Investigating' })).toHaveAttribute('aria-checked', 'true')
    expect(screen.queryByRole('menuitemradio', { name: 'Archived' })).toBeNull()
  })

  it('sets the status through updateInvestigation', async () => {
    const user = userEvent.setup()
    wrap(<StatusPicker investigation={investigation} />)
    await user.click(screen.getByRole('button', { name: 'Status: Investigating' }))
    await user.click(screen.getByRole('menuitemradio', { name: 'Blocked' }))
    expect(mocks.api.updateInvestigation).toHaveBeenCalledWith('INV-0001', { status: 'blocked' })
  })

  it('makes no call when the current status is chosen again', async () => {
    const user = userEvent.setup()
    wrap(<StatusPicker investigation={investigation} />)
    await user.click(screen.getByRole('button', { name: 'Status: Investigating' }))
    await user.click(screen.getByRole('menuitemradio', { name: 'Investigating' }))
    expect(mocks.api.updateInvestigation).not.toHaveBeenCalled()
  })

  it('shows Archived disabled for an archived investigation', () => {
    wrap(<StatusPicker investigation={archivedDetail().investigation} />)
    const button = screen.getByRole('button', { name: 'Status: Archived' })
    expect(button).toBeDisabled()
    expect(button).toHaveTextContent('Archived')
  })
})
