import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Api } from './api/api'
import type { InvestigationSummary } from './api/types'
import { emptyDetail, summaries, summaryOf } from './test/fixtures'
import { mockApi } from './test/mockApi'
import App from './App'

const mocks = vi.hoisted(() => ({ api: {} as Api }))
vi.mock('./api/api', () => ({ api: mocks.api }))

describe('App tracer flow', () => {
  let list: InvestigationSummary[]

  beforeEach(() => {
    list = []
    const created = emptyDetail({
      ref: 'INV-0001',
      title: 'Why did calls go up in October?',
      problem_statement: 'Why do the two systems disagree?',
      is_active: true,
    })
    Object.assign(
      mocks.api,
      mockApi({
        listInvestigations: async () => list,
        createInvestigation: async () => {
          list = [summaryOf(created.investigation)]
          return created.investigation
        },
        getInvestigationDetail: async () => created,
      }),
    )
  })

  it('creates an investigation, opens it and copies its context', async () => {
    const user = userEvent.setup()
    render(<App />)

    const nav = await screen.findByRole('navigation', { name: 'Questions' })
    expect(await within(nav).findByText('No questions yet.')).toBeInTheDocument()

    await user.click(within(nav).getByRole('button', { name: 'New question' }))
    const dialog = await screen.findByRole('dialog', { name: 'New question' })
    const title = within(dialog).getByLabelText('Question')
    expect(title).toHaveFocus()
    expect(within(dialog).getByLabelText("Make this your AI's current question")).toBeChecked()

    await user.type(title, 'Why did calls go up in October?')
    await user.click(within(dialog).getByRole('button', { name: 'Add question' }))

    expect(mocks.api.createInvestigation).toHaveBeenCalledWith(
      expect.objectContaining({ title: 'Why did calls go up in October?', make_active: true }),
    )
    const heading = await screen.findByRole('heading', { level: 1, name: 'Why did calls go up in October?' })
    await waitFor(() => expect(heading).toHaveFocus())
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(within(nav).getByRole('button', { current: true })).toHaveTextContent('Why did calls go up in October?')
    expect(screen.getByText('Why do the two systems disagree?')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Copy for your AI' }))
    expect(mocks.api.copyContext).toHaveBeenCalledWith('INV-0001')
    expect(await screen.findByRole('button', { name: 'Copied' })).toBeInTheDocument()
    expect(screen.getByText('Copied for your AI')).toBeInTheDocument()
  })
})

describe('App main pane states', () => {
  it('invites the first investigation when there are none', async () => {
    Object.assign(mocks.api, mockApi({ listInvestigations: async () => [] }))
    render(<App />)
    const main = await screen.findByRole('main')
    expect(await within(main).findByRole('heading', { name: 'Check what your AI tells you' })).toBeInTheDocument()
    expect(
      within(main).getByText(
        'You ask your AI why a number moved, and it gives you an answer. This app keeps what it found, where each finding came from, and which ones you have checked yourself.',
      ),
    ).toBeInTheDocument()
    expect(within(main).getByRole('button', { name: 'Connect your AI' })).toHaveClass('btn-primary')
    expect(within(main).getByRole('button', { name: 'New question' })).toHaveClass('btn-secondary')
    expect(within(main).getByRole('button', { name: 'New question' })).toBeInTheDocument()
    expect(within(main).getByRole('button', { name: 'Add a question from a file...' })).toBeInTheDocument()
    // Three short steps say how to use it.
    const steps = within(within(main).getByRole('list', { name: 'How to use it' })).getAllByRole('listitem')
    expect(steps.map((li) => li.textContent)).toEqual([
      'Connect your AI. You do this once.',
      'Ask your AI a question. Tell it to save what it finds here.',
      'Check the sources and share the answer. Mark each source you have checked yourself.',
    ])
    expect(await screen.findByText('No questions yet.')).toBeInTheDocument()
  })

  it('opens the New investigation dialog from the empty state', async () => {
    Object.assign(mocks.api, mockApi({ listInvestigations: async () => [] }))
    const user = userEvent.setup()
    render(<App />)
    const main = await screen.findByRole('main')
    await user.click(await within(main).findByRole('button', { name: 'New question' }))
    expect(await screen.findByRole('dialog', { name: 'New question' })).toBeInTheDocument()
  })

  it('shows Home at launch, with no question open, and Check opens one', async () => {
    Object.assign(mocks.api, mockApi({ listInvestigations: async () => summaries.filter((s) => !s.archived) }))
    const user = userEvent.setup()
    render(<App />)
    const main = await screen.findByRole('main')
    expect(await within(main).findByRole('heading', { level: 1, name: '2 questions need you' })).toBeInTheDocument()
    expect(mocks.api.getInvestigationDetail).not.toHaveBeenCalled()
    expect(screen.getByRole('button', { name: /^To check/ })).toHaveAttribute('aria-current', 'true')

    await user.click(within(main).getByRole('button', { name: 'Check: Why did calls go up 30% in October when sales stayed the same?' }))
    expect(await screen.findByRole('heading', { level: 1, name: 'Why did calls go up 30% in October when sales stayed the same?' })).toBeInTheDocument()
    expect(mocks.api.getInvestigationDetail).toHaveBeenCalledWith('INV-0001', { activity_limit: 20 })

    // To check goes back to Home.
    await user.click(screen.getByRole('button', { name: /^To check/ }))
    expect(await within(main).findByRole('heading', { level: 1, name: '2 questions need you' })).toBeInTheDocument()
  })
})
