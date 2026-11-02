import { act, render, screen, within } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { Api } from '../../api/api'
import type { InvestigationSummary } from '../../api/types'
import { AppDataProvider, LIST_LIMIT } from '../../state/appData'
import { summaries } from '../../test/fixtures'
import { mockApi } from '../../test/mockApi'
import { ToastProvider } from '../ui/Toaster'
import { DataOpsProvider } from './DataOps'
import { LIST_CAPPED_COPY, Sidebar } from './Sidebar'

const mocks = vi.hoisted(() => ({ api: {} as Api }))
vi.mock('../../api/api', () => ({ api: mocks.api }))

function renderSidebar() {
  return render(
    <ToastProvider>
      <AppDataProvider>
        <DataOpsProvider>
          <Sidebar onNew={() => {}} />
        </DataOpsProvider>
      </AppDataProvider>
    </ToastProvider>,
  )
}

describe('Sidebar', () => {
  beforeEach(() => {
    Object.assign(mocks.api, mockApi())
  })
  afterEach(() => {
    vi.useRealTimers()
  })

  it('shows skeleton rows only after 300 ms of loading', async () => {
    vi.useFakeTimers()
    let resolve: (v: InvestigationSummary[]) => void = () => {}
    Object.assign(
      mocks.api,
      mockApi({ listInvestigations: () => new Promise<InvestigationSummary[]>((r) => (resolve = r)) }),
    )
    const { container } = renderSidebar()
    await act(async () => {
      await vi.advanceTimersByTimeAsync(0)
    })
    expect(mocks.api.listInvestigations).toHaveBeenCalled()
    await act(async () => {
      await vi.advanceTimersByTimeAsync(299)
    })
    expect(container.querySelectorAll('.inv-skeleton')).toHaveLength(0)
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1)
    })
    expect(container.querySelectorAll('.inv-skeleton')).toHaveLength(3)

    await act(async () => {
      resolve(summaries)
      await vi.advanceTimersByTimeAsync(0)
    })
    expect(container.querySelectorAll('.inv-skeleton')).toHaveLength(0)
    expect(screen.getByText('September cloud bill up 38%')).toBeInTheDocument()
  })

  it('shows no skeleton when the list loads quickly', async () => {
    const { container } = renderSidebar()
    expect(await screen.findByText('September cloud bill up 38%')).toBeInTheDocument()
    expect(container.querySelectorAll('.inv-skeleton')).toHaveLength(0)
  })

  it('marks the selected, active and archived rows', async () => {
    renderSidebar()
    const nav = await screen.findByRole('navigation', { name: 'Cases' })
    const first = await within(nav).findByRole('button', { current: true })
    expect(first).toHaveTextContent('September cloud bill up 38%')
    expect(first).toHaveTextContent("Your AI's current case")
    expect(first).toHaveTextContent('INV-0001')
    expect(within(first).getByRole('img', { name: 'Confidence: High' })).toBeInTheDocument()

    const archived = within(nav).getByText('31 former employees still have active SSO accounts').closest('button')!
    expect(archived).toHaveTextContent('Archived')
    expect(archived).not.toHaveAttribute('aria-current')
    expect(archived).not.toHaveTextContent("Your AI's current case")
    expect(archived).toHaveClass('is-archived')

    const blocked = within(nav).getByText('Checkout p99 doubled after the region move').closest('button')!
    expect(blocked).toHaveTextContent('Blocked')
    expect(within(blocked).getByText('Not assessed')).toBeInTheDocument()
  })

  it('asks for the engine maximum, not the default 100, and says when the list is capped', async () => {
    const many: InvestigationSummary[] = Array.from({ length: LIST_LIMIT }, (_, i) => ({
      ...summaries[1],
      ref: `INV-${String(i + 1).padStart(4, '0')}`,
      title: `Investigation ${i + 1}`,
      is_active: false,
    }))
    Object.assign(mocks.api, mockApi({ listInvestigations: async () => many }))
    renderSidebar()
    expect(await screen.findByText(LIST_CAPPED_COPY)).toBeInTheDocument()
    expect(LIST_CAPPED_COPY).toBe('Showing the 1,000 most recently updated cases. Search to find older ones.')
    expect(mocks.api.listInvestigations).toHaveBeenCalledWith({ include_archived: false, limit: LIST_LIMIT })
  })

  it('shows no cap note for a short list', async () => {
    renderSidebar()
    expect(await screen.findByText('Checkout p99 doubled after the region move')).toBeInTheDocument()
    expect(screen.queryByText(LIST_CAPPED_COPY)).toBeNull()
  })

  it('says so when there are no investigations', async () => {
    Object.assign(mocks.api, mockApi({ listInvestigations: async () => [] }))
    renderSidebar()
    expect(await screen.findByText('No cases yet.')).toBeInTheDocument()
  })
})
