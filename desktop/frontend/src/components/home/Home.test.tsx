import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Api } from '../../api/api'
import type { InvestigationSummary } from '../../api/types'
import { AppDataProvider, useAppData } from '../../state/appData'
import { summaries } from '../../test/fixtures'
import { mockApi } from '../../test/mockApi'
import { ToastProvider } from '../ui/Toaster'
import { ALL_CHECKED_BODY, ALL_CHECKED_DECIDED_BODY, answeredLine, Home, needYouHeading, NO_SOURCES_YET, NOTHING_NEEDS_YOU } from './Home'

const mocks = vi.hoisted(() => ({ api: {} as Api }))
vi.mock('../../api/api', () => ({ api: mocks.api }))

const [calls, helpDesk, cancels] = summaries

function Probe() {
  const { selectedRef, listStatus, pendingOpen } = useAppData()
  return <p data-testid="probe">{`${listStatus}:${selectedRef ?? ''}${pendingOpen ? ` ${pendingOpen}` : ''}`}</p>
}

async function renderHome(list: InvestigationSummary[]) {
  Object.assign(mocks.api, mockApi({ listInvestigations: async () => list }))
  render(
    <ToastProvider>
      <AppDataProvider>
        <Home />
        <Probe />
      </AppDataProvider>
    </ToastProvider>,
  )
  await waitFor(() => expect(screen.getByTestId('probe')).toHaveTextContent('ready:'))
}

describe('Home', () => {
  beforeEach(() => {
    Object.assign(mocks.api, mockApi())
  })

  it('words the heading for none, one and many', () => {
    expect(needYouHeading(0)).toBe('Nothing needs you right now.')
    expect(needYouHeading(1)).toBe('1 question needs you')
    expect(needYouHeading(2)).toBe('2 questions need you')
  })

  it('gives each question that needs you one line and one button, which opens it on that', async () => {
    const user = userEvent.setup()
    await renderHome([calls, helpDesk])
    expect(screen.getByRole('heading', { level: 1, name: '2 questions need you' })).toBeInTheDocument()
    const items = within(screen.getByRole('list', { name: 'Questions that need you' })).getAllByRole('listitem')
    expect(items[0]).toHaveTextContent('Why did calls go up 30% in October when sales stayed the same?')
    // One line: the first thing it needs. The finding where sources disagree waits its turn.
    expect(items[0].querySelector('.home-needs')).toHaveTextContent(/^1 of 4 sources checked by you$/)
    expect(items[1].querySelector('.home-needs')).toHaveTextContent(/^0 of 2 sources checked by you$/)
    // No codes on Home.
    expect(screen.queryByText(/INV-\d+/)).toBeNull()
    const check = within(items[1]).getByRole('button', { name: 'Check: Why are help desk tickets taking longer to close this month?' })
    expect(check).toHaveClass('btn-primary')
    await user.click(check)
    expect(screen.getByTestId('probe')).toHaveTextContent('ready:INV-0002 check-sources')
  })

  it('asks for a decision, then the answer, then the check of the answer, as each comes up', async () => {
    const user = userEvent.setup()
    const checked = { sources_total: 4, sources_checked_by_you: 4, findings_disagree: 0 }
    await renderHome([
      { ...calls, ...checked, ref: 'INV-0011', title: 'One', findings_disagree: 2 },
      { ...calls, ...checked, ref: 'INV-0012', title: 'Two', conclusion: '' },
      { ...calls, ...checked, ref: 'INV-0013', title: 'Three', answer_checked_by_you: false },
      { ...calls, ...checked, ref: 'INV-0014', title: 'Four', answer_checked_by_you: true },
    ])
    expect(screen.getByRole('heading', { level: 1, name: '3 questions need you' })).toBeInTheDocument()
    const items = within(screen.getByRole('list', { name: 'Questions that need you' })).getAllByRole('listitem')
    expect(items.map((li) => li.querySelector('.home-needs')?.textContent)).toEqual([
      '2 findings where sources disagree',
      'Every source is checked. Write the answer.',
      'The answer is not checked yet.',
    ])
    expect(screen.getByRole('button', { name: 'Decide: One' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Check the answer: Three' })).toBeInTheDocument()
    // Fully checked: it is under Answered, not in the list.
    expect(within(screen.getByRole('region', { name: 'Answered' })).getByRole('button', { name: 'Four' })).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Write the answer: Two' }))
    expect(screen.getByTestId('probe')).toHaveTextContent('ready:INV-0012 write-answer')
  })

  it('says nothing needs you, and lists answers with their text and checked count', async () => {
    const user = userEvent.setup()
    const done = { ...cancels, archived: false, archived_at: '' }
    await renderHome([done])
    expect(screen.getByRole('heading', { level: 1, name: NOTHING_NEEDS_YOU })).toBeInTheDocument()
    expect(screen.getByText(ALL_CHECKED_BODY)).toBeInTheDocument()
    expect(screen.queryByRole('list', { name: 'Questions that need you' })).toBeNull()
    const answered = screen.getByRole('region', { name: 'Answered' })
    expect(answered).toHaveTextContent('Customers who joined with two free months in January cancelled when they had to start paying in March.')
    expect(answered).toHaveTextContent('1 of 1 source checked by you')
    await user.click(within(answered).getByRole('button', { name: 'Why did more customers cancel in March?' }))
    expect(screen.getByTestId('probe')).toHaveTextContent('ready:INV-0003')
  })

  it('says a finding where sources disagree was decided only when a person decided one, and never that no sources disagree', async () => {
    const done = { ...cancels, archived: false, archived_at: '' }
    await renderHome([{ ...done, findings_decided: 1 }])
    expect(screen.getByText(ALL_CHECKED_DECIDED_BODY)).toBeInTheDocument()
    expect(ALL_CHECKED_DECIDED_BODY).toBe('You have checked every source and every answer, and decided every finding where sources disagree.')
    expect(ALL_CHECKED_BODY).toBe('You have checked every source and every answer.')
    expect(screen.queryByText(/no sources disagree/)).toBeNull()
  })

  it('asks for a finding when every source is checked but none holds, and opens Add finding', async () => {
    const user = userEvent.setup()
    await renderHome([{ ...helpDesk, sources_checked_by_you: 2, findings_backing: 0 }])
    const items = within(screen.getByRole('list', { name: 'Questions that need you' })).getAllByRole('listitem')
    expect(items[0].querySelector('.home-needs')).toHaveTextContent(/^Add a finding from your sources.$/)
    await user.click(screen.getByRole('button', { name: 'Add a finding: Why are help desk tickets taking longer to close this month?' }))
    expect(screen.getByTestId('probe')).toHaveTextContent('ready:INV-0002 add-finding')
  })

  it('never lets things still to find out make a question need you, and says how many are open on its line', async () => {
    const done = { ...cancels, archived: false, archived_at: '', counts: { ...cancels.counts, questions: 3, open_questions: 2 } }
    await renderHome([done])
    expect(screen.getByRole('heading', { level: 1, name: NOTHING_NEEDS_YOU })).toBeInTheDocument()
    expect(screen.getByRole('region', { name: 'Answered' })).toHaveTextContent('Everything is checked. 2 things are still to find out.')
    expect(answeredLine({ ...done, counts: { ...done.counts, open_questions: 1 } })).toBe('Everything is checked. 1 thing is still to find out.')
  })

  it('keeps a question with no answer and nothing to check in view, and leaves archived ones out', async () => {
    const fresh = { ...helpDesk, ref: 'INV-0007', title: 'Why did returns double in May?', sources_total: 0, sources_checked_by_you: 0 }
    await renderHome([fresh, cancels])
    const waiting = screen.getByRole('region', { name: 'No answer yet' })
    expect(within(waiting).getByRole('button', { name: 'Why did returns double in May?' })).toBeInTheDocument()
    expect(waiting).toHaveTextContent(NO_SOURCES_YET)
    expect(screen.queryByText('Why did more customers cancel in March?')).toBeNull()
    expect(screen.queryByRole('region', { name: 'Answered' })).toBeNull()
  })

  it('shows the day as the eyebrow', async () => {
    await renderHome([calls])
    const today = new Intl.DateTimeFormat('en-US', { weekday: 'long', month: 'long', day: 'numeric' }).format(new Date())
    expect(screen.getByText(today)).toBeInTheDocument()
  })
})
