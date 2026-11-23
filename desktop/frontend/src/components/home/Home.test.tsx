import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Api } from '../../api/api'
import type { InvestigationSummary } from '../../api/types'
import { AppDataProvider, useAppData } from '../../state/appData'
import { summaries } from '../../test/fixtures'
import { mockApi } from '../../test/mockApi'
import { ToastProvider } from '../ui/Toaster'
import { hideIds } from '../record/RecordId'
import { ALL_CHECKED_BODY, Home, needYouHeading } from './Home'

const mocks = vi.hoisted(() => ({ api: {} as Api }))
vi.mock('../../api/api', () => ({ api: mocks.api }))

const [calls, helpDesk, payroll] = summaries

function Probe() {
  const { selectedRef, listStatus } = useAppData()
  return <p data-testid="probe">{`${listStatus}:${selectedRef ?? ''}`}</p>
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
    expect(needYouHeading(0)).toBe('Everything is checked')
    expect(needYouHeading(1)).toBe('1 question needs you')
    expect(needYouHeading(2)).toBe('2 questions need you')
  })

  it('lists the questions that need you with what is left, and Check opens one', async () => {
    const user = userEvent.setup()
    await renderHome([calls, helpDesk])
    expect(screen.getByRole('heading', { level: 1, name: '2 questions need you' })).toBeInTheDocument()
    const items = within(screen.getByRole('list', { name: 'Questions that need you' })).getAllByRole('listitem')
    expect(items[0]).toHaveTextContent('Why did calls go up 30% in October when sales stayed the same?')
    expect(items[0]).toHaveTextContent('1 of 4 sources checked by you')
    expect(items[0]).toHaveTextContent('1 finding where sources disagree')
    expect(items[0]).not.toHaveTextContent('No answer yet')
    expect(items[1]).toHaveTextContent('0 of 2 sources checked by you')
    expect(items[1]).toHaveTextContent('No answer yet')
    expect(items[1]).not.toHaveTextContent('sources disagree')
    // No codes on Home.
    expect(screen.queryByText(/INV-\d+/)).toBeNull()
    const check = within(items[1]).getByRole('button', { name: 'Check: Why are help desk tickets taking longer to close this month?' })
    expect(check).toHaveClass('btn-primary')
    await user.click(check)
    expect(screen.getByTestId('probe')).toHaveTextContent('ready:INV-0002')
  })

  it('says findings in the plural, and one question in the singular', async () => {
    await renderHome([{ ...calls, findings_disagree: 2 }])
    expect(screen.getByRole('heading', { level: 1, name: '1 question needs you' })).toBeInTheDocument()
    expect(screen.getByText('2 findings where sources disagree')).toBeInTheDocument()
  })

  it('says everything is checked, and lists answers with their text and checked count', async () => {
    const user = userEvent.setup()
    const done = { ...payroll, archived: false, archived_at: '' }
    await renderHome([done])
    expect(screen.getByRole('heading', { level: 1, name: 'Everything is checked' })).toBeInTheDocument()
    expect(screen.getByText(ALL_CHECKED_BODY)).toBeInTheDocument()
    expect(screen.queryByRole('list', { name: 'Questions that need you' })).toBeNull()
    const answered = screen.getByRole('region', { name: 'Answered' })
    expect(answered).toHaveTextContent('Two new hires started a month earlier than the budget assumed.')
    expect(answered).toHaveTextContent('1 of 1 source checked by you')
    await user.click(within(answered).getByRole('button', { name: 'Why was March payroll $18k over budget?' }))
    expect(screen.getByTestId('probe')).toHaveTextContent('ready:INV-0003')
  })

  it('keeps a question with no answer and nothing to check in view, and leaves archived ones out', async () => {
    const fresh = { ...helpDesk, ref: 'INV-0007', title: 'Why did returns double in May?', sources_total: 0, sources_checked_by_you: 0 }
    await renderHome([fresh, payroll])
    const waiting = screen.getByRole('region', { name: 'No answer yet' })
    expect(within(waiting).getByRole('button', { name: 'Why did returns double in May?' })).toBeInTheDocument()
    expect(waiting).toHaveTextContent('Nothing to check yet. Ask your AI to work on it.')
    expect(screen.queryByText('Why was March payroll $18k over budget?')).toBeNull()
    expect(screen.queryByRole('region', { name: 'Answered' })).toBeNull()
  })

  it('shows the day as the eyebrow', async () => {
    await renderHome([calls])
    const today = new Intl.DateTimeFormat('en-US', { weekday: 'long', month: 'long', day: 'numeric' }).format(new Date())
    expect(screen.getByText(today)).toBeInTheDocument()
  })
})

describe('hideIds', () => {
  it('wraps each code in a sentence, with the space before it, and leaves other text alone', () => {
    const { container } = render(<p>{hideIds('Marked C-002: You checked this (see E-014 and INV-0003)')}</p>)
    expect(container.textContent).toBe('Marked C-002: You checked this (see E-014 and INV-0003)')
    expect(Array.from(container.querySelectorAll('.record-id')).map((el) => el.textContent)).toEqual([' C-002', ' E-014', ' INV-0003'])
    const plain = render(<p>{hideIds('Updated the answer')}</p>)
    expect(plain.container.querySelector('.record-id')).toBeNull()
    expect(plain.container.textContent).toBe('Updated the answer')
  })
})
