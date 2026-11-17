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
      title: 'Cloud bill up in September',
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

    const nav = await screen.findByRole('navigation', { name: 'Cases' })
    expect(await within(nav).findByText('No cases yet.')).toBeInTheDocument()

    await user.click(within(nav).getByRole('button', { name: 'New case' }))
    const dialog = await screen.findByRole('dialog', { name: 'New case' })
    const title = within(dialog).getByLabelText('Title')
    expect(title).toHaveFocus()
    expect(within(dialog).getByLabelText("Make this your AI's current case")).toBeChecked()

    await user.type(title, 'Cloud bill up in September')
    await user.click(within(dialog).getByRole('button', { name: 'Create case' }))

    expect(mocks.api.createInvestigation).toHaveBeenCalledWith(
      expect.objectContaining({ title: 'Cloud bill up in September', make_active: true }),
    )
    const heading = await screen.findByRole('heading', { level: 1, name: 'Cloud bill up in September' })
    await waitFor(() => expect(heading).toHaveFocus())
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(within(nav).getByRole('button', { current: true })).toHaveTextContent('Cloud bill up in September')
    expect(screen.getByText('Why do the two systems disagree?')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Copy for your AI' }))
    expect(mocks.api.copyContext).toHaveBeenCalledWith('INV-0001')
    expect(await screen.findByRole('button', { name: 'Copied' })).toBeInTheDocument()
    expect(screen.getByText('Context copied to the clipboard')).toBeInTheDocument()
  })
})

describe('App main pane states', () => {
  it('invites the first investigation when there are none', async () => {
    Object.assign(mocks.api, mockApi({ listInvestigations: async () => [] }))
    render(<App />)
    const main = await screen.findByRole('main')
    expect(await within(main).findByRole('heading', { name: 'Start your first case' })).toBeInTheDocument()
    expect(
      within(main).getByText(
        'Connect your AI, then ask it a question that needs digging. What it finds lands here for you to check and share.',
      ),
    ).toBeInTheDocument()
    expect(within(main).getByRole('button', { name: 'Connect your AI' })).toHaveClass('btn-primary')
    expect(within(main).getByRole('button', { name: 'New case' })).toHaveClass('btn-secondary')
    expect(within(main).getByRole('button', { name: 'New case' })).toBeInTheDocument()
    expect(within(main).getByRole('button', { name: 'Import case...' })).toBeInTheDocument()
    expect(await screen.findByText('No cases yet.')).toBeInTheDocument()
  })

  it('opens the New investigation dialog from the empty state', async () => {
    Object.assign(mocks.api, mockApi({ listInvestigations: async () => [] }))
    const user = userEvent.setup()
    render(<App />)
    const main = await screen.findByRole('main')
    await user.click(await within(main).findByRole('button', { name: 'New case' }))
    expect(await screen.findByRole('dialog', { name: 'New case' })).toBeInTheDocument()
  })

  it('asks to choose an investigation when none is selected', async () => {
    Object.assign(mocks.api, mockApi({ listInvestigations: async () => [summaries[2]] }))
    render(<App />)
    expect(await screen.findByText('Choose a case from the list.')).toBeInTheDocument()
  })

  it('opens the active investigation at launch', async () => {
    Object.assign(mocks.api, mockApi())
    render(<App />)
    expect(await screen.findByRole('heading', { level: 1, name: 'September cloud bill up 38%' })).toBeInTheDocument()
    expect(mocks.api.getInvestigationDetail).toHaveBeenCalledWith('INV-0001', { activity_limit: 20 })
  })
})
