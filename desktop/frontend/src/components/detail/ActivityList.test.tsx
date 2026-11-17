import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Api } from '../../api/api'
import type { Activity, InvestigationDetail } from '../../api/types'
import { AppDataProvider, useAppData } from '../../state/appData'
import { claude, detail, human, summaries } from '../../test/fixtures'
import { act as row, manyActivity } from '../../test/fixtures.detail'
import { mockApi } from '../../test/mockApi'
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
      <AppDataProvider>
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
      row(claude, 'evidence.add', 'E-005', 'Trace store query'),
      row(human, 'question.resolve', 'Q-001', 'Intentional business logic.'),
    ])
    const history = await screen.findByRole('region', { name: 'History' })
    const items = within(history).getAllByRole('listitem')
    expect(items[0]).toHaveTextContent('Claude Code (AI) added evidence E-005')
    expect(items[1]).toHaveTextContent('You (Casebook) resolved Q-001')
    expect(within(items[0]).getByText('Claude Code (AI)')).toHaveAttribute('title', 'AI, claude-code')
    expect(within(items[1]).getByText('You (Casebook)')).toHaveAttribute('title', 'Human, casebook-ui')
  })

  it('shows the engine summary on a second line as text, omitted when empty or equal to the target', async () => {
    renderList([
      row(claude, 'evidence.add', 'E-005', '**not markup** <b>x</b>'),
      row(human, 'claim.update', 'C-001', 'C-001'),
      row(human, 'conclusion.update', 'INV-0001', ''),
    ])
    const history = await screen.findByRole('region', { name: 'History' })
    const items = within(history).getAllByRole('listitem')
    expect(within(items[0]).getByText('**not markup** <b>x</b>')).toBeInTheDocument()
    expect(items[0].querySelector('b')).toBeNull()
    expect(items[1].querySelector('.activity-summary')).toBeNull()
    expect(items[2].querySelector('.activity-summary')).toBeNull()
  })

  it('rewrites engine summaries in plain words and keeps the stored text in the title', async () => {
    const raw = 'Updated claim C-001: statement, verification; verification reset to ai_generated after AI edit'
    renderList([row(claude, 'claim.update', 'C-001', raw), row(human, 'active.set', 'INV-0001', 'Set active investigation to INV-0001')])
    const history = await screen.findByRole('region', { name: 'History' })
    const items = within(history).getAllByRole('listitem')
    const reset = within(items[0]).getByText('Edited claim C-001 (statement). Its verification was reset to AI generated after the edit; it is back in Review findings.')
    expect(reset).toHaveAttribute('title', raw)
    expect(within(items[1]).getByText("Made INV-0001 your AI's current case")).toHaveAttribute('title', 'Set active investigation to INV-0001')
    expect(history).not.toHaveTextContent('ai_generated')
  })

  it('shows the relative time with the absolute time in the title', async () => {
    renderList([{ ...row(claude, 'evidence.add', 'E-005'), created_at: '2026-01-15T14:00:00.000Z' }])
    const history = await screen.findByRole('region', { name: 'History' })
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
    const history = await screen.findByRole('region', { name: 'History' })
    await user.click(within(history).getByRole('button', { name: 'E-005' }))
    expect(onOpenRef).toHaveBeenCalledWith('E-005')
    expect(within(history).getByRole('button', { name: 'C-001' })).toBeInTheDocument()
    expect(within(history).queryByRole('button', { name: 'Q-001' })).toBeNull()
  })

  it('renders refs as plain text without onOpenRef', async () => {
    renderList([row(claude, 'evidence.add', 'E-005')])
    const history = await screen.findByRole('region', { name: 'History' })
    expect(within(history).queryByRole('button')).toBeNull()
  })

  it('offers Show more history at 20 rows and reloads with activity_limit 200', async () => {
    const user = userEvent.setup()
    renderList(manyActivity(20))
    const history = await screen.findByRole('region', { name: 'History' })
    expect(within(history).getAllByRole('listitem')).toHaveLength(20)
    await user.click(within(history).getByRole('button', { name: 'Show more history' }))
    await waitFor(() =>
      expect(mocks.api.getInvestigationDetail).toHaveBeenLastCalledWith('INV-0001', { activity_limit: 200 }),
    )
  })

  it('hides Show more history with fewer than 20 rows', async () => {
    renderList(manyActivity(19))
    const history = await screen.findByRole('region', { name: 'History' })
    expect(within(history).queryByRole('button', { name: 'Show more history' })).toBeNull()
  })

  it('says No history yet. when empty', async () => {
    renderList([])
    const history = await screen.findByRole('region', { name: 'History' })
    expect(within(history).getByText('No history yet.')).toBeInTheDocument()
  })
})
