import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { Api } from '../../api/api'
import type { ListOptions, SearchHit } from '../../api/types'
import { AppDataProvider, useAppData } from '../../state/appData'
import { archivedHit, archivedSummaries, hits, markupHit } from '../../test/fixtures.dataops'
import { mockApi } from '../../test/mockApi'
import { ToastProvider } from '../ui/Toaster'
import { DataOpsProvider } from './DataOps'
import { Sidebar } from './Sidebar'

const mocks = vi.hoisted(() => ({ api: {} as Api }))
vi.mock('../../api/api', () => ({ api: mocks.api }))

/** Shows the selection and the pending record so tests can see select(). */
function Probe() {
  const { selectedRef, pendingOpen, refresh } = useAppData()
  return (
    <div>
      <p data-testid="probe">{`selected:${selectedRef ?? ''} pending:${pendingOpen ?? ''}`}</p>
      <button type="button" onClick={() => void refresh()}>
        Probe refresh
      </button>
    </div>
  )
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

async function typeQuery(text: string) {
  const user = userEvent.setup()
  const field = await screen.findByRole('searchbox', { name: 'Search cases' })
  await user.type(field, text)
  return { user, field }
}

describe('Sidebar search', () => {
  beforeEach(() => {
    Object.assign(mocks.api, mockApi({ search: async () => hits }))
  })
  afterEach(() => {
    vi.useRealTimers()
  })

  it('debounces typing by 200 ms and searches once', async () => {
    vi.useFakeTimers()
    renderSidebar()
    await act(async () => {
      await vi.advanceTimersByTimeAsync(0)
    })
    const field = screen.getByRole('searchbox', { name: 'Search cases' })
    fireEvent.change(field, { target: { value: 'deb' } })
    await act(async () => {
      await vi.advanceTimersByTimeAsync(100)
    })
    fireEvent.change(field, { target: { value: 'debug' } })
    await act(async () => {
      await vi.advanceTimersByTimeAsync(199)
    })
    expect(mocks.api.search).not.toHaveBeenCalled()
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1)
    })
    expect(mocks.api.search).toHaveBeenCalledTimes(1)
    expect(mocks.api.search).toHaveBeenCalledWith('debug', { include_archived: false, limit: 50 })
  })

  it('does not search for an empty or blank query', async () => {
    renderSidebar()
    await typeQuery('   ')
    await new Promise((r) => setTimeout(r, 260))
    expect(mocks.api.search).not.toHaveBeenCalled()
    expect(screen.getByText('Checkout p99 doubled after the region move')).toBeInTheDocument()
  })

  it('groups hits by investigation in rank order with highlighted snippets', async () => {
    renderSidebar()
    await typeQuery('debug')
    const results = await screen.findByRole('region', { name: 'Search results' })
    const groups = within(results).getAllByRole('group')
    expect(groups).toHaveLength(2)
    expect(groups[0]).toHaveAccessibleName('September cloud bill up 38% INV-0001')
    expect(within(groups[0]).getByText("Your AI's current case")).toBeInTheDocument()
    expect(groups[1]).toHaveAccessibleName('Checkout p99 doubled after the region move INV-0002')
    expect(within(groups[1]).queryByText("Your AI's current case")).not.toBeInTheDocument()

    const first = within(groups[0]).getAllByRole('button')
    expect(first).toHaveLength(2)
    expect(first[0]).toHaveTextContent('C-001')
    expect(first[1]).toHaveTextContent('E-003')
    const mark = first[0].querySelector('mark')
    expect(mark).not.toBeNull()
    expect(mark).toHaveTextContent('debug')
    expect(first[0]).toHaveTextContent('A shared platform default moved every service to debug logging on August 28')
    // The list is replaced by the results.
    expect(screen.queryByRole('button', { current: true })).not.toBeInTheDocument()
  })

  it('renders markup inside a snippet as literal text', async () => {
    Object.assign(mocks.api, mockApi({ search: async () => [markupHit] }))
    const { container } = renderSidebar()
    await typeQuery('debug')
    const results = await screen.findByRole('region', { name: 'Search results' })
    await within(results).findByText(/onerror/)
    expect(container.querySelector('img')).toBeNull()
    expect(results).toHaveTextContent('<img src=x onerror="alert(1)"> is debug')
  })

  it('says so when nothing matches', async () => {
    Object.assign(mocks.api, mockApi({ search: async () => [] }))
    renderSidebar()
    await typeQuery('debug')
    expect(await screen.findByText('No matches for "debug"')).toBeInTheDocument()
    expect(
      screen.getByText('Try fewer or different words. Archived cases are searched only when Show archived is on.'),
    ).toBeInTheDocument()
  })

  it('shows a validation error under the field', async () => {
    Object.assign(
      mocks.api,
      mockApi({
        search: async () => {
          throw JSON.stringify({ kind: 'validation', field: 'query', message: 'must be at most 500 bytes' })
        },
      }),
    )
    renderSidebar()
    await typeQuery('debug')
    expect(await screen.findByText('Must be at most 500 bytes')).toBeInTheDocument()
  })

  it('opens the record of a claim hit and only the investigation for an investigation hit', async () => {
    renderSidebar()
    const { user } = await typeQuery('debug')
    const results = await screen.findByRole('region', { name: 'Search results' })
    const claimHit = within(results).getByRole('button', { name: /C-001/ })
    claimHit.focus()
    await user.keyboard('{Enter}')
    expect(screen.getByTestId('probe')).toHaveTextContent('selected:INV-0001 pending:C-001')

    const invHit = within(results).getByRole('button', { name: /INV-0002/ })
    await user.click(invHit)
    expect(screen.getByTestId('probe')).toHaveTextContent('selected:INV-0002 pending:')
    expect(screen.getByTestId('probe')).not.toHaveTextContent('pending:C')
  })

  it('clears the query with Esc and with the Clear search button', async () => {
    renderSidebar()
    const { user, field } = await typeQuery('debug')
    await screen.findByRole('region', { name: 'Search results' })
    field.focus()
    await user.keyboard('{Escape}')
    expect(field).toHaveValue('')
    expect(screen.queryByRole('region', { name: 'Search results' })).not.toBeInTheDocument()
    expect(screen.getByText('Checkout p99 doubled after the region move')).toBeInTheDocument()

    await user.type(field, 'debug')
    await screen.findByRole('region', { name: 'Search results' })
    await user.click(screen.getByRole('button', { name: 'Clear search' }))
    expect(field).toHaveValue('')
    expect(field).toHaveFocus()
    expect(screen.queryByRole('region', { name: 'Search results' })).not.toBeInTheDocument()
  })

  it('shows the / hint only while the field is empty', async () => {
    const { container } = renderSidebar()
    await screen.findByText('Checkout p99 doubled after the region move')
    expect(container.querySelector('.sidebar-search-kbd')).toHaveTextContent('/')
    expect(screen.queryByRole('button', { name: 'Clear search' })).not.toBeInTheDocument()
    const user = userEvent.setup()
    await user.type(screen.getByRole('searchbox', { name: 'Search cases' }), 'c')
    expect(container.querySelector('.sidebar-search-kbd')).toBeNull()
    expect(screen.getByRole('button', { name: 'Clear search' })).toBeInTheDocument()
  })

  it('includes archived investigations when Show archived is on', async () => {
    Object.assign(mocks.api, mockApi({ search: async () => [archivedHit] }))
    renderSidebar()
    const user = userEvent.setup()
    const toggle = await screen.findByRole('switch', { name: 'Show archived' })
    expect(toggle).toHaveAttribute('aria-checked', 'false')
    await user.click(toggle)
    expect(toggle).toHaveAttribute('aria-checked', 'true')
    await waitFor(() =>
      expect(mocks.api.listInvestigations).toHaveBeenCalledWith(expect.objectContaining({ include_archived: true })),
    )
    await user.type(screen.getByRole('searchbox', { name: 'Search cases' }), 'debug')
    await waitFor(() => expect(mocks.api.search).toHaveBeenCalledWith('debug', { include_archived: true, limit: 50 }))
    const group = await screen.findByRole('group', { name: /INV-0003/ })
    expect(within(group).getByText('Archived')).toBeInTheDocument()
  })

  it('points to Show archived when only archived investigations exist', async () => {
    Object.assign(
      mocks.api,
      mockApi({ listInvestigations: async (o?: ListOptions) => (o?.include_archived ? archivedSummaries : []) }),
    )
    renderSidebar()
    expect(
      await screen.findByText('No open cases. Turn on Show archived to see archived ones.'),
    ).toBeInTheDocument()
    expect(mocks.api.listInvestigations).toHaveBeenCalledWith({ include_archived: true, limit: 1 })
    expect(screen.queryByText('No cases yet.')).not.toBeInTheDocument()
  })

  it('reruns the search when the data changes and keeps the old results meanwhile', async () => {
    let resolveSecond: (v: SearchHit[]) => void = () => {}
    let calls = 0
    Object.assign(
      mocks.api,
      mockApi({
        search: () => {
          calls++
          if (calls === 1) return Promise.resolve(hits)
          return new Promise<SearchHit[]>((r) => (resolveSecond = r))
        },
      }),
    )
    renderSidebar()
    const { user } = await typeQuery('debug')
    const results = await screen.findByRole('region', { name: 'Search results' })
    expect(within(results).getAllByRole('group')).toHaveLength(2)

    await user.click(screen.getByRole('button', { name: 'Probe refresh' }))
    await waitFor(() => expect(mocks.api.search).toHaveBeenCalledTimes(2))
    expect(within(results).getAllByRole('group')).toHaveLength(2)

    await act(async () => resolveSecond([hits[1]]))
    await waitFor(() => expect(within(screen.getByRole('region', { name: 'Search results' })).getAllByRole('group')).toHaveLength(1))
  })
})
