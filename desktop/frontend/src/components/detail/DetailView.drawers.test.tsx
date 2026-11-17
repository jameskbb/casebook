import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Api } from '../../api/api'
import type { InvestigationDetail } from '../../api/types'
import { AppDataProvider, useAppData } from '../../state/appData'
import { CopyContextProvider } from '../../state/copyContext'
import { activity, claims, detail, human, questions, summaries } from '../../test/fixtures'
import { act as activityRow } from '../../test/fixtures.detail'
import { detailWith, detailWithNewClaim, newClaim, question } from '../../test/fixtures.records'
import { mockApi } from '../../test/mockApi'
import { ToastProvider } from '../ui/Toaster'
import { DetailView } from './DetailView'

const mocks = vi.hoisted(() => ({ api: {} as Api }))
vi.mock('../../api/api', () => ({ api: mocks.api }))

/** Test controls next to the detail: open a record by ref, reload, show pendingOpen. */
function Controls() {
  const { select, refresh, pendingOpen } = useAppData()
  return (
    <div>
      <button type="button" onClick={() => select('INV-0001', { openRecord: 'C-002' })}>
        Search hit C-002
      </button>
      <button type="button" onClick={() => select('INV-0001', { openRecord: 'Q-001' })}>
        Search hit Q-001
      </button>
      <button type="button" onClick={() => void refresh()}>
        Reload
      </button>
      <output aria-label="pending">{pendingOpen ?? 'none'}</output>
    </div>
  )
}

let current: InvestigationDetail = detail

function renderDetail(d: InvestigationDetail = detail) {
  current = d
  Object.assign(
    mocks.api,
    mockApi({
      listInvestigations: async () => [summaries[0]],
      getInvestigationDetail: async () => current,
    }),
  )
  return render(
    <ToastProvider>
      <AppDataProvider>
        <CopyContextProvider>
          <Controls />
          <DetailView />
        </CopyContextProvider>
      </AppDataProvider>
    </ToastProvider>,
  )
}

describe('DetailView drawers', () => {
  beforeEach(() => {
    Object.assign(mocks.api, mockApi())
  })

  it('opens the evidence drawer directly from a contradicting chip on a claim row', async () => {
    const user = userEvent.setup()
    renderDetail()
    const chips = await screen.findAllByRole('button', {
      name: 'E-003 contradicts this: Log ingestion is 71% of the September increase',
    })
    await user.click(chips[0])
    expect(screen.getByRole('dialog', { name: 'Evidence E-003' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /^Back to/ })).toBeNull()
  })

  it('opens drawers from the evidence row', async () => {
    const user = userEvent.setup()
    renderDetail()
    await user.click(await screen.findByRole('button', { name: /^Open evidence E-002:/ }))
    expect(screen.getByRole('dialog', { name: 'Evidence E-002' })).toBeInTheDocument()
  })

  it('makes E, C and H refs in History buttons that open drawers, and leaves Q refs as text', async () => {
    const user = userEvent.setup()
    renderDetail(
      detailWith({
        recent_activity: [activityRow(human, 'question.resolve', 'Q-001', 'Answered.'), ...activity],
      }),
    )
    const history = await screen.findByRole('region', { name: 'History' })
    expect(within(history).queryByRole('button', { name: 'Q-001' })).toBeNull()
    expect(within(history).getByText('resolved Q-001')).toBeInTheDocument()
    await user.click(within(history).getByRole('button', { name: 'E-003' }))
    expect(screen.getByRole('dialog', { name: 'Evidence E-003' })).toBeInTheDocument()
  })

  it('opens the drawer for a pending record from a search hit and clears it', async () => {
    const user = userEvent.setup()
    renderDetail()
    await screen.findByRole('heading', { level: 1 })
    await user.click(screen.getByRole('button', { name: 'Search hit C-002' }))
    expect(await screen.findByRole('dialog', { name: 'Claim C-002' })).toBeInTheDocument()
    await waitFor(() => expect(screen.getByRole('status', { name: 'pending' })).toHaveTextContent('none'))
  })

  it('focuses the question row for a pending question instead of opening a drawer', async () => {
    const user = userEvent.setup()
    renderDetail()
    await screen.findByRole('heading', { level: 1 })
    await user.click(screen.getByRole('button', { name: 'Search hit Q-001' }))
    await waitFor(() => expect(document.querySelector('[data-ref="Q-001"]')).toHaveFocus())
    expect(screen.queryByRole('dialog')).toBeNull()
    expect(screen.getByRole('status', { name: 'pending' })).toHaveTextContent('none')
  })

  it('keeps the drawer open across a reload and shows the latest record', async () => {
    const user = userEvent.setup()
    renderDetail()
    await user.click(await screen.findByRole('button', { name: /^Open claim C-002:/ }))
    current = detailWith({
      claims: claims.map((c) => (c.ref === 'C-002' ? { ...c, statement: 'Rewritten by Codex.' } : c)),
    })
    await user.click(screen.getByRole('button', { name: 'Reload' }))
    const drawer = await screen.findByRole('dialog', { name: 'Claim C-002' })
    await waitFor(() => expect(within(drawer).getByText('Rewritten by Codex.')).toBeInTheDocument())
  })

  it('says a record was not found when it disappears from the latest detail', async () => {
    const user = userEvent.setup()
    renderDetail()
    await user.click(await screen.findByRole('button', { name: /^Open claim C-004:/ }))
    current = detailWith({ claims: claims.filter((c) => c.ref !== 'C-004') })
    await user.click(screen.getByRole('button', { name: 'Reload' }))
    const drawer = screen.getByRole('dialog', { name: 'Claim C-004' })
    expect(await within(drawer).findByText('C-004 was not found. It may have changed in another app.')).toBeInTheDocument()
  })
})

describe('DetailView adding and question actions', () => {
  beforeEach(() => {
    Object.assign(mocks.api, mockApi())
  })

  it('adds a claim from the section and moves focus to the new row', async () => {
    const user = userEvent.setup()
    renderDetail()
    vi.mocked(mocks.api.addClaim).mockImplementation(async () => {
      current = detailWithNewClaim()
      return newClaim
    })
    const claimsSection = await screen.findByRole('region', { name: 'What we believe' })
    await user.click(within(claimsSection).getByRole('button', { name: 'Add claim' }))
    await user.type(screen.getByLabelText('Statement'), 'The log level comes from the chart default.')
    await user.click(within(screen.getByRole('dialog', { name: 'Add claim' })).getByRole('button', { name: 'Add claim' }))
    await waitFor(() =>
      expect(document.querySelector('.reading-column [data-ref="C-005"] .row-hit')).toHaveFocus(),
    )
  })

  it('opens the add dialogs from every section', async () => {
    const user = userEvent.setup()
    renderDetail()
    const cases: [string, string][] = [
      ['What might explain it', 'Add hypothesis'],
      ['Evidence', 'Add evidence'],
      ['Still open', 'Add question'],
    ]
    for (const [section, label] of cases) {
      const region = await screen.findByRole('region', { name: section })
      await user.click(within(region).getByRole('button', { name: label }))
      expect(screen.getByRole('dialog', { name: label })).toBeInTheDocument()
      await user.click(within(screen.getByRole('dialog', { name: label })).getByRole('button', { name: 'Cancel' }))
    }
  })

  it('shows Resolve, Defer and Edit question on open questions', async () => {
    const user = userEvent.setup()
    renderDetail()
    const row = (await screen.findByRole('region', { name: 'Still open' })).querySelector('[data-ref="Q-001"]') as HTMLElement
    await user.click(within(row).getByRole('button', { name: 'Resolve' }))
    expect(screen.getByRole('dialog', { name: 'Resolve Q-001' })).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Cancel' }))
    await user.click(within(row).getByRole('button', { name: 'Defer' }))
    expect(screen.getByRole('dialog', { name: 'Defer Q-001' })).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Cancel' }))
    await user.click(within(row).getByRole('button', { name: 'Edit question' }))
    expect(screen.getByRole('dialog', { name: 'Edit question Q-001' })).toBeInTheDocument()
    expect(within(row).queryByRole('button', { name: 'Reopen' })).toBeNull()
  })

  it('reopens resolved and deferred questions', async () => {
    const user = userEvent.setup()
    renderDetail()
    const section = await screen.findByRole('region', { name: 'Still open' })
    const deferred = section.querySelector('[data-ref="Q-003"]') as HTMLElement
    expect(within(deferred).getByRole('button', { name: 'Reopen' })).toBeInTheDocument()
    await user.click(within(section).getByRole('button', { name: 'Show 1 resolved question' }))
    const resolved = section.querySelector('[data-ref="Q-002"]') as HTMLElement
    expect(within(resolved).queryByRole('button', { name: 'Resolve' })).toBeNull()
    await user.click(within(resolved).getByRole('button', { name: 'Reopen' }))
    expect(mocks.api.reopenQuestion).toHaveBeenCalledWith('INV-0001', 'Q-002')
  })

  it('focuses a newly added question', async () => {
    const user = userEvent.setup()
    renderDetail()
    const added = question({ id: 4, ref: 'Q-004', question: 'Who approves a change to a shared chart default?' })
    vi.mocked(mocks.api.addQuestion).mockImplementation(async () => {
      current = detailWith({ questions: [...questions, added] })
      return added
    })
    const section = await screen.findByRole('region', { name: 'Still open' })
    await user.click(within(section).getByRole('button', { name: 'Add question' }))
    await user.type(screen.getByLabelText('Question'), 'Who approves a change to a shared chart default?')
    await user.click(within(screen.getByRole('dialog', { name: 'Add question' })).getByRole('button', { name: 'Add question' }))
    await waitFor(() => expect(document.querySelector('[data-ref="Q-004"]')).toHaveFocus())
  })
})

