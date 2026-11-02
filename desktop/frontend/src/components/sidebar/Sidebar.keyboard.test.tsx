import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Api } from '../../api/api'
import { AppDataProvider, useAppData } from '../../state/appData'
import { hits } from '../../test/fixtures.dataops'
import { mockApi } from '../../test/mockApi'
import { ToastProvider } from '../ui/Toaster'
import { DataOpsProvider } from './DataOps'
import { Sidebar } from './Sidebar'

const mocks = vi.hoisted(() => ({ api: {} as Api }))
vi.mock('../../api/api', () => ({ api: mocks.api }))

function Probe() {
  const { selectedRef, pendingOpen } = useAppData()
  return <p data-testid="probe">{`selected:${selectedRef ?? ''} pending:${pendingOpen ?? ''}`}</p>
}

function renderSidebar() {
  return render(
    <ToastProvider>
      <AppDataProvider>
        <DataOpsProvider>
          <Sidebar onNew={() => {}} />
          <Probe />
        </DataOpsProvider>
      </AppDataProvider>
    </ToastProvider>,
  )
}

async function rows() {
  const nav = await screen.findByRole('navigation', { name: 'Cases' })
  await within(nav).findByRole('button', { current: true })
  const list = within(nav).getByRole('list', { name: 'Case list' })
  return within(list).getAllByRole('button')
}

describe('Sidebar keyboard', () => {
  beforeEach(() => {
    Object.assign(mocks.api, mockApi({ search: async () => hits }))
  })

  it('makes the list one Tab stop on the selected row', async () => {
    renderSidebar()
    const r = await rows()
    expect(r).toHaveLength(3)
    expect(r.map((b) => b.tabIndex)).toEqual([0, -1, -1])
    expect(r[0]).toHaveAttribute('aria-current', 'true')
  })

  it('moves focus with Up, Down, Home and End and selects with Enter', async () => {
    const user = userEvent.setup()
    renderSidebar()
    const r = await rows()
    r[0].focus()
    await user.keyboard('{ArrowDown}')
    expect(r[1]).toHaveFocus()
    expect(r[1].tabIndex).toBe(0)
    expect(r[0].tabIndex).toBe(-1)
    await user.keyboard('{ArrowDown}{ArrowDown}')
    expect(r[2]).toHaveFocus()
    await user.keyboard('{ArrowUp}')
    expect(r[1]).toHaveFocus()
    await user.keyboard('{Home}')
    expect(r[0]).toHaveFocus()
    await user.keyboard('{End}')
    expect(r[2]).toHaveFocus()
    await user.keyboard('{ArrowUp}{Enter}')
    expect(screen.getByTestId('probe')).toHaveTextContent('selected:INV-0002')
    expect(r[1]).toHaveAttribute('aria-current', 'true')
  })

  it('moves through search hits as one Tab stop and opens one with Enter', async () => {
    const user = userEvent.setup()
    renderSidebar()
    await rows()
    const field = screen.getByRole('searchbox', { name: 'Search cases' })
    await user.type(field, 'debug')
    const results = await screen.findByRole('region', { name: 'Search results' })
    const buttons = within(results).getAllByRole('button')
    expect(buttons.map((b) => b.tabIndex)).toEqual([0, -1, -1])

    // Down from the field enters the results.
    field.focus()
    await user.keyboard('{ArrowDown}')
    expect(buttons[0]).toHaveFocus()
    await user.keyboard('{ArrowDown}')
    expect(buttons[1]).toHaveFocus()
    await user.keyboard('{End}')
    expect(buttons[2]).toHaveFocus()
    await user.keyboard('{Home}{ArrowDown}{Enter}')
    expect(screen.getByTestId('probe')).toHaveTextContent('selected:INV-0001 pending:E-003')
  })
})
