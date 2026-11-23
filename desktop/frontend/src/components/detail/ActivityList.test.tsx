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
import { ActivityList, shortName } from './ActivityList'

const mocks = vi.hoisted(() => ({ api: {} as Api }))
vi.mock('../../api/api', () => ({ api: mocks.api }))

type NameOf = (ref: string) => string | undefined

/** The words of the fixture source E-003 and of the findings, as the question page passes them. */
const names: NameOf = (ref) => (ref === detail.evidence[2].ref ? detail.evidence[2].title : detail.claims.find((c) => c.ref === ref)?.statement)

function Harness({ onOpenRef, nameOf }: { onOpenRef?: (ref: string) => void; nameOf?: NameOf }) {
  const { detail: d } = useAppData()
  if (!d) return null
  return <ActivityList activities={d.recent_activity} onOpenRef={onOpenRef} nameOf={nameOf} />
}

function renderList(activities: Activity[], onOpenRef?: (ref: string) => void, nameOf?: NameOf) {
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
        <Harness onOpenRef={onOpenRef} nameOf={nameOf} />
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
      row(claude, 'evidence.add', 'E-005', 'Added evidence E-005 "Call log"'),
      row(human, 'question.resolve', 'Q-001', 'Resolved question Q-001: Yes, says the head of billing.'),
    ])
    const history = await screen.findByRole('region', { name: 'How we got here' })
    const items = within(history).getAllByRole('listitem')
    expect(items[0]).toHaveTextContent('Your AI added the source E-005')
    expect(items[1]).toHaveTextContent('You answered something to find out Q-001')
    // Its code is there for an AI, but hidden from a reader with the other codes.
    expect(within(items[1]).getByText('Q-001')).toHaveClass('record-id')
    expect(within(items[0]).getByText('Your AI')).toHaveAttribute('title', 'AI, Claude Code')
    expect(within(items[1]).getByText('You')).toHaveAttribute('title', 'Person, Casebook')
    // The answer is its second line; the stored sentence around it is not.
    expect(items[1].querySelector('.activity-summary')).toHaveTextContent(/^Yes, says the head of billing\.$/)
  })

  it('names a source or finding by its words, with its code kept for when codes are on', async () => {
    const onOpenRef = vi.fn()
    const user = userEvent.setup()
    renderList(
      [row(claude, 'evidence.add', 'E-003', `Added evidence E-003 "${detail.evidence[2].title}"`), row(human, 'claim.update', 'C-001', 'Updated claim C-001: verification human_verified')],
      onOpenRef,
      names,
    )
    const history = await screen.findByRole('region', { name: 'How we got here' })
    const items = within(history).getAllByRole('listitem')
    expect(items[0]).toHaveTextContent('Your AI added the source E-003 Calls about a bill: 400 in September, 1,070 in October')
    expect(within(items[0]).getByText('E-003')).toHaveClass('record-id')
    // The first line already names the source, so there is no second line.
    expect(items[0].querySelector('.activity-summary')).toBeNull()
    expect(items[1].querySelector('.activity-summary')).toHaveTextContent(/^Decided this still holds\.$/)
    await user.click(within(items[0]).getByRole('button', { name: /Calls about a bill/ }))
    expect(onOpenRef).toHaveBeenCalledWith('E-003')
  })

  it('shows a checkpoint note as written, with each code hidden and its words after it', async () => {
    renderList([row(claude, 'checkpoint', 'INV-0001', 'Added C-003 (E-007).')], undefined, names)
    const history = await screen.findByRole('region', { name: 'How we got here' })
    const note = history.querySelector('.activity-summary') as HTMLElement
    const codes = Array.from(note.querySelectorAll('.record-id'))
    // The bracket holds only a code, so it hides whole; the bare code hides and the finding's words stand in.
    expect(codes.map((c) => c.textContent?.trim())).toEqual(['C-003', '(E-007)'])
    const words = codes[0].nextElementSibling as HTMLElement
    expect(words).toHaveClass('codes-off')
    expect(words).toHaveTextContent(`"${shortName(detail.claims[2].statement, 40)}"`)
    expect(note.querySelectorAll('.codes-off')).toHaveLength(1)
    expect(note.textContent?.startsWith('Added')).toBe(true)
    expect(note.textContent?.endsWith('.')).toBe(true)
  })

  it('shows a note as text, and no second line for a stored summary it does not know', async () => {
    const unknown = 'Frobnicated claim C-001 (status active)'
    renderList([
      row(claude, 'checkpoint', 'INV-0001', '**not markup** <b>x</b>'),
      row(human, 'claim.update', 'C-001', unknown),
      row(claude, 'x.y', 'C-001', unknown),
      row(human, 'conclusion.update', 'INV-0001', ''),
    ])
    const history = await screen.findByRole('region', { name: 'How we got here' })
    await userEvent.click(within(history).getByRole('button', { name: 'Show all 4' }))
    const items = within(history).getAllByRole('listitem')
    expect(within(items[0]).getByText('**not markup** <b>x</b>')).toBeInTheDocument()
    expect(items[0].querySelector('b')).toBeNull()
    expect(items[1].querySelector('.activity-summary')).toBeNull()
    expect(items[2].querySelector('.activity-summary')).toBeNull()
    expect(items[2]).toHaveTextContent('Your AI made a change')
    expect(items[3].querySelector('.activity-summary')).toBeNull()
    expect(history).not.toHaveTextContent('Frobnicated')
    expect(history).not.toHaveTextContent('x.y')
  })

  it('rewrites engine summaries in plain words and keeps the stored text out of the page', async () => {
    const raw = 'Updated claim C-001: statement, verification; verification reset to ai_generated after AI edit'
    const stored = [raw, 'Set active investigation to INV-0001', "Made INV-0001 your AI's current question"]
    renderList([row(claude, 'claim.update', 'C-001', stored[0]), row(human, 'active.set', 'INV-0001', stored[1]), row(human, 'active.set', 'INV-0001', stored[2])])
    const history = await screen.findByRole('region', { name: 'How we got here' })
    const items = within(history).getAllByRole('listitem')
    const reset = items[0].querySelector('.activity-summary') as HTMLElement
    expect(reset).toHaveTextContent(/^Changed the wording\. It changed after it was checked, so it needs checking again\.$/)
    // The first line already says it, so the second is left out.
    expect(items[1]).toHaveTextContent('You chose this question for your AI to work on')
    expect(items[1].querySelector('.activity-summary')).toBeNull()
    expect(items[2].querySelector('.activity-summary')).toBeNull()
    expect(history).not.toHaveTextContent('ai_generated')
    // No tooltip carries the stored text either.
    const titles = Array.from(history.querySelectorAll('[title]')).map((el) => el.getAttribute('title') ?? '')
    for (const s of stored) for (const t of titles) expect(t).not.toContain(s)
    expect(titles.join(' ')).not.toMatch(/ai_generated|verification|investigation/)
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

  it('shows the last three rows, and Show all with the count opens the rest and closes again', async () => {
    const user = userEvent.setup()
    renderList(manyActivity(7))
    const history = await screen.findByRole('region', { name: 'How we got here' })
    expect(within(history).getAllByRole('listitem')).toHaveLength(3)
    await user.click(within(history).getByRole('button', { name: 'Show all 7' }))
    expect(within(history).getAllByRole('listitem')).toHaveLength(7)
    // Everything was already on the page: nothing to load.
    expect(mocks.api.getInvestigationDetail).not.toHaveBeenCalledWith('INV-0001', { activity_limit: 200 })
    await user.click(within(history).getByRole('button', { name: 'Show only the last 3' }))
    expect(within(history).getAllByRole('listitem')).toHaveLength(3)
  })

  it('says Show all without a count when the page may not hold every row, and loads up to 200', async () => {
    const user = userEvent.setup()
    renderList(manyActivity(20))
    const history = await screen.findByRole('region', { name: 'How we got here' })
    await user.click(within(history).getByRole('button', { name: 'Show all' }))
    await waitFor(() =>
      expect(mocks.api.getInvestigationDetail).toHaveBeenLastCalledWith('INV-0001', { activity_limit: 200 }),
    )
    expect(within(history).getAllByRole('listitem')).toHaveLength(20)
  })

  it('offers no Show all with three rows or fewer, and draws nothing when empty', async () => {
    const { unmount } = renderList(manyActivity(3))
    const history = await screen.findByRole('region', { name: 'How we got here' })
    expect(within(history).queryByRole('button', { name: /Show/ })).toBeNull()
    unmount()
    renderList([])
    await waitFor(() => expect(mocks.api.getInvestigationDetail).toHaveBeenCalled())
    expect(screen.queryByRole('region', { name: 'How we got here' })).toBeNull()
  })
})
