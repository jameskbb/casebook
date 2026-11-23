import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Api } from '../../api/api'
import type { Activity, InvestigationDetail } from '../../api/types'
import { AppDataProvider, useAppData } from '../../state/appData'
import { claude, detail, human, summaries } from '../../test/fixtures'
import { act as row, manyActivity } from '../../test/fixtures.detail'
import { mockApi } from '../../test/mockApi'
import { OpenAtLaunch } from '../../test/OpenAtLaunch'
import { ToastProvider } from '../ui/Toaster'
import { ActivityList } from './ActivityList'

const mocks = vi.hoisted(() => ({ api: {} as Api }))
vi.mock('../../api/api', () => ({ api: mocks.api }))

function Harness({ onOpenRef }: { onOpenRef?: (ref: string) => void }) {
  const { detail: d } = useAppData()
  if (!d) return null
  return <ActivityList activities={d.recent_activity} onOpenRef={onOpenRef} />
}

function renderList(activities: Activity[], onOpenRef?: (ref: string) => void) {
  const d: InvestigationDetail = { ...detail, recent_activity: activities }
  Object.assign(
    mocks.api,
    mockApi({
      listInvestigations: async () => [summaries[0]],
      getInvestigationDetail: async () => d,
    }),
  )
  return render(
    <ToastProvider>
      <AppDataProvider><OpenAtLaunch />
        <Harness onOpenRef={onOpenRef} />
      </AppDataProvider>
    </ToastProvider>,
  )
}

describe('ActivityList', () => {
  beforeEach(() => {
    Object.assign(mocks.api, mockApi())
  })

  it('reads each row as a plain sentence with the actor tooltip', async () => {
    renderList([
      row(claude, 'evidence.add', 'E-005', 'Call log'),
      row(human, 'question.resolve', 'Q-001', 'Yes, says the head of billing.'),
    ])
    const history = await screen.findByRole('region', { name: 'How we got here' })
    const items = within(history).getAllByRole('listitem')
    expect(items[0]).toHaveTextContent('Claude Code added the source E-005')
    expect(items[1]).toHaveTextContent('You answered a follow-up question Q-001')
    // The follow-up question's code is there for an AI, but hidden from a reader with the other codes.
    expect(within(items[1]).getByText('Q-001')).toHaveClass('record-id')
    expect(within(items[0]).getByText('Claude Code')).toHaveAttribute('title', 'AI, claude-code')
    expect(within(items[1]).getByText('You')).toHaveAttribute('title', 'Person, casebook-ui')
  })

  it('names a source or finding by its words, with its code kept for when codes are on', async () => {
    const onOpenRef = vi.fn()
    const user = userEvent.setup()
    const d: InvestigationDetail = { ...detail, recent_activity: [row(claude, 'evidence.add', 'E-003', detail.evidence[2].title), row(human, 'claim.update', 'C-001', 'Updated claim C-001: verification human_verified')] }
    Object.assign(mocks.api, mockApi({ listInvestigations: async () => [summaries[0]], getInvestigationDetail: async () => d }))
    function Named() {
      const { detail: loaded } = useAppData()
      if (!loaded) return null
      const names = new Map([[loaded.evidence[2].ref, loaded.evidence[2].title], [loaded.claims[0].ref, loaded.claims[0].statement]])
      return <ActivityList activities={loaded.recent_activity} onOpenRef={onOpenRef} nameOf={(ref) => names.get(ref)} />
    }
    render(<ToastProvider><AppDataProvider><OpenAtLaunch /><Named /></AppDataProvider></ToastProvider>)
    const history = await screen.findByRole('region', { name: 'How we got here' })
    const items = within(history).getAllByRole('listitem')
    expect(items[0]).toHaveTextContent('Claude Code added the source E-003 Calls about a bill: 400 in September, 1,070 in October')
    expect(within(items[0]).getByText('E-003')).toHaveClass('record-id')
    // The second line would only repeat the title, so it is left out.
    expect(items[0].querySelector('.activity-summary')).toBeNull()
    // A code inside the second line is hidden the same way.
    const marked = items[1].querySelector('.activity-summary') as HTMLElement
    expect(marked).toHaveTextContent('Marked C-001: You checked this')
    expect(within(marked).getByText('C-001')).toHaveClass('record-id')
    await user.click(within(items[0]).getByRole('button', { name: /Calls about a bill/ }))
    expect(onOpenRef).toHaveBeenCalledWith('E-003')
  })

  it('shows the engine summary on a second line as text, omitted when empty or equal to the target', async () => {
    renderList([
      row(claude, 'evidence.add', 'E-005', '**not markup** <b>x</b>'),
      row(human, 'claim.update', 'C-001', 'C-001'),
      row(human, 'conclusion.update', 'INV-0001', ''),
    ])
    const history = await screen.findByRole('region', { name: 'How we got here' })
    const items = within(history).getAllByRole('listitem')
    expect(within(items[0]).getByText('**not markup** <b>x</b>')).toBeInTheDocument()
    expect(items[0].querySelector('b')).toBeNull()
    expect(items[1].querySelector('.activity-summary')).toBeNull()
    expect(items[2].querySelector('.activity-summary')).toBeNull()
  })

  it('rewrites engine summaries in plain words and keeps the stored text in the title', async () => {
    const raw = 'Updated claim C-001: statement, verification; verification reset to ai_generated after AI edit'
    renderList([row(claude, 'claim.update', 'C-001', raw), row(human, 'active.set', 'INV-0001', 'Set active investigation to INV-0001')])
    const history = await screen.findByRole('region', { name: 'How we got here' })
    const items = within(history).getAllByRole('listitem')
    const reset = items[0].querySelector('.activity-summary') as HTMLElement
    expect(reset).toHaveTextContent('Edited finding C-001 (wording). It changed after it was checked, so it reads Not checked again and is back in Review findings.')
    expect(reset).toHaveAttribute('title', raw)
    // The first line already says it, so the second is left out.
    expect(items[1]).toHaveTextContent("You made this your AI's current question")
    expect(items[1].querySelector('.activity-summary')).toBeNull()
    expect(history).not.toHaveTextContent('ai_generated')
  })

  it('shows the relative time with the absolute time in the title', async () => {
    renderList([{ ...row(claude, 'evidence.add', 'E-005'), created_at: '2026-01-15T14:00:00.000Z' }])
    const history = await screen.findByRole('region', { name: 'How we got here' })
    const time = history.querySelector('time') as HTMLElement
    expect(time).toHaveAttribute('dateTime', '2026-01-15T14:00:00.000Z')
    expect(time.getAttribute('title')).toMatch(/January 15, 2026 at/)
  })

  it('renders E, C and H refs as text buttons only when onOpenRef is passed', async () => {
    const onOpenRef = vi.fn()
    const user = userEvent.setup()
    renderList(
      [row(claude, 'evidence.add', 'E-005'), row(human, 'question.resolve', 'Q-001'), row(human, 'link.update', 'C-001')],
      onOpenRef,
    )
    const history = await screen.findByRole('region', { name: 'How we got here' })
    await user.click(within(history).getByRole('button', { name: 'E-005' }))
    expect(onOpenRef).toHaveBeenCalledWith('E-005')
    expect(within(history).getByRole('button', { name: 'C-001' })).toBeInTheDocument()
    expect(within(history).queryByRole('button', { name: 'Q-001' })).toBeNull()
  })

  it('renders refs as plain text without onOpenRef', async () => {
    renderList([row(claude, 'evidence.add', 'E-005')])
    const history = await screen.findByRole('region', { name: 'How we got here' })
    expect(within(history).queryByRole('button')).toBeNull()
  })

  it('offers Show more at 20 rows and reloads with activity_limit 200', async () => {
    const user = userEvent.setup()
    renderList(manyActivity(20))
    const history = await screen.findByRole('region', { name: 'How we got here' })
    expect(within(history).getAllByRole('listitem')).toHaveLength(20)
    await user.click(within(history).getByRole('button', { name: 'Show more' }))
    await waitFor(() =>
      expect(mocks.api.getInvestigationDetail).toHaveBeenLastCalledWith('INV-0001', { activity_limit: 200 }),
    )
  })

  it('hides Show more with fewer than 20 rows', async () => {
    renderList(manyActivity(19))
    const history = await screen.findByRole('region', { name: 'How we got here' })
    expect(within(history).queryByRole('button', { name: 'Show more' })).toBeNull()
  })

  it('says Nothing has happened yet. when empty', async () => {
    renderList([])
    const history = await screen.findByRole('region', { name: 'How we got here' })
    expect(within(history).getByText('Nothing has happened yet.')).toBeInTheDocument()
  })
})
