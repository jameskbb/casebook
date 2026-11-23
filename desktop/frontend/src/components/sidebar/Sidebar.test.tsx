import { act, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
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
    expect(screen.getByText('Why did calls go up 30% in October when sales stayed the same?')).toBeInTheDocument()
  })

  it('shows no skeleton when the list loads quickly', async () => {
    const { container } = renderSidebar()
    expect(await screen.findByText('Why did calls go up 30% in October when sales stayed the same?')).toBeInTheDocument()
    expect(container.querySelectorAll('.inv-skeleton')).toHaveLength(0)
  })

  it('groups questions under No answer yet and Answered, with To check on top', async () => {
    renderSidebar()
    const nav = await screen.findByRole('navigation', { name: 'Questions' })
    const noAnswer = await within(nav).findByRole('list', { name: 'No answer yet' })
    const answered = within(nav).getByRole('list', { name: 'Answered' })
    expect(within(noAnswer).getAllByRole('button').map((b) => b.dataset.ref)).toEqual(['INV-0002'])
    expect(within(answered).getAllByRole('button').map((b) => b.dataset.ref)).toEqual(['INV-0001', 'INV-0003'])
    // Nothing is open at launch: To check is the current item, and it counts what needs a person.
    const home = within(nav).getByRole('button', { current: true })
    expect(home).toHaveTextContent('To check')
    expect(home).toHaveTextContent('2 questions')
  })

  it('shows the checked count on a row with sources and when it was last worked on otherwise', async () => {
    Object.assign(
      mocks.api,
      mockApi({ listInvestigations: async () => [summaries[0], { ...summaries[1], sources_total: 0, sources_checked_by_you: 0 }] }),
    )
    renderSidebar()
    const nav = await screen.findByRole('navigation', { name: 'Questions' })
    const calls = (await within(nav).findByText('Why did calls go up 30% in October when sales stayed the same?')).closest('button')!
    expect(calls).toHaveTextContent('1 of 4 sources checked by you')
    const helpDesk = within(nav).getByText('Why are help desk tickets taking longer to close this month?').closest('button')!
    expect(helpDesk).toHaveTextContent(/Last worked on /)
    expect(helpDesk).not.toHaveTextContent('checked by you')
  })

  it('marks the selected, active and archived rows', async () => {
    const user = userEvent.setup()
    renderSidebar()
    const nav = await screen.findByRole('navigation', { name: 'Questions' })
    await user.click((await within(nav).findByText('Why did calls go up 30% in October when sales stayed the same?')).closest('button')!)
    const first = await within(nav).findByRole('button', { current: true })
    expect(first).toHaveTextContent('Why did calls go up 30% in October when sales stayed the same?')
    expect(first).toHaveTextContent("Your AI's current question")
    expect(first).toHaveTextContent('INV-0001')
    // No status and no confidence on a row: the title and what is checked.
    expect(within(first).queryByRole('img')).toBeNull()
    expect(first).not.toHaveTextContent(/Investigating|Resolved|High/)

    const archived = within(nav).getByText('Why was March payroll $18k over budget?').closest('button')!
    expect(archived).toHaveTextContent('Archived')
    expect(archived).not.toHaveAttribute('aria-current')
    expect(archived).not.toHaveTextContent("Your AI's current question")
    expect(archived).toHaveClass('is-archived')

    const blocked = within(nav).getByText('Why are help desk tickets taking longer to close this month?').closest('button')!
    expect(blocked).toHaveTextContent('INV-0002')
    expect(blocked).not.toHaveTextContent(/Blocked|Not assessed/)
  })

  it('asks for the engine maximum, not the default 100, and says when the list is capped', async () => {
    const many: InvestigationSummary[] = Array.from({ length: LIST_LIMIT }, (_, i) => ({
      ...summaries[1],
      ref: `INV-${String(i + 1).padStart(4, '0')}`,
      title: `Question ${i + 1}`,
      is_active: false,
    }))
    Object.assign(mocks.api, mockApi({ listInvestigations: async () => many }))
    renderSidebar()
    expect(await screen.findByText(LIST_CAPPED_COPY)).toBeInTheDocument()
    expect(LIST_CAPPED_COPY).toBe('Showing the 1,000 most recently updated questions. Search to find older ones.')
    expect(mocks.api.listInvestigations).toHaveBeenCalledWith({ include_archived: false, limit: LIST_LIMIT })
  })

  it('shows no cap note for a short list', async () => {
    renderSidebar()
    expect(await screen.findByText('Why are help desk tickets taking longer to close this month?')).toBeInTheDocument()
    expect(screen.queryByText(LIST_CAPPED_COPY)).toBeNull()
  })

  it('says so when there are no investigations', async () => {
    Object.assign(mocks.api, mockApi({ listInvestigations: async () => [] }))
    renderSidebar()
    expect(await screen.findByText('No questions yet.')).toBeInTheDocument()
  })
})
