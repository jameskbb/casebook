import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Api } from '../../api/api'
import type { InvestigationDetail } from '../../api/types'
import { AppDataProvider, useAppData } from '../../state/appData'
import { CopyContextProvider } from '../../state/copyContext'
import { activity, claims, detail, human, questions, summaries } from '../../test/fixtures'
import { act as activityRow } from '../../test/fixtures.detail'
import { detailWith, detailWithBareEvidence, detailWithNewClaim, newClaim, question } from '../../test/fixtures.records'
import { mockApi } from '../../test/mockApi'
import { OpenAtLaunch } from '../../test/OpenAtLaunch'
import { findSourceButton } from '../../test/openQuestion'
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
      <AppDataProvider><OpenAtLaunch />
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

  it('opens the source drawer directly from a source nested under a finding', async () => {
    const user = userEvent.setup()
    renderDetail()
    const nested = await screen.findAllByRole('button', {
      name: 'Contradicts this: E-003 Calls about a bill: 400 in September, 1,070 in October Call log Not checked',
    })
    await user.click(nested[0])
    expect(screen.getByRole('dialog', { name: 'Source E-003' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /^Back to/ })).toBeNull()
  })

  it('opens the drawer from a source row under Other sources', async () => {
    const user = userEvent.setup()
    renderDetail(detailWithBareEvidence())
    const other = await screen.findByRole('region', { name: 'Other sources' })
    await user.click(within(other).getByRole('button', { name: /^Open source E-009:/ }))
    expect(screen.getByRole('dialog', { name: 'Source E-009' })).toBeInTheDocument()
  })

  it('puts focus back on the nested source after its drawer closes', async () => {
    const user = userEvent.setup()
    renderDetail()
    const nested = await findSourceButton('E-002')
    await user.click(nested)
    await user.click(within(screen.getByRole('dialog', { name: 'Source E-002' })).getByRole('button', { name: 'Close' }))
    await waitFor(() => expect(nested).toHaveFocus())
  })

  it('names sources and findings in How we got here as buttons that open drawers, and leaves follow-up questions as text', async () => {
    const user = userEvent.setup()
    renderDetail(
      detailWith({
        recent_activity: [activityRow(human, 'question.resolve', 'Q-001', 'Answered.'), ...activity],
      }),
    )
    const history = await screen.findByRole('region', { name: 'How we got here' })
    expect(within(history).queryByRole('button', { name: 'Q-001' })).toBeNull()
    expect(within(history).getByText('answered a follow-up question')).toBeInTheDocument()
    await user.click(within(history).getByRole('button', { name: 'E-003 Calls about a bill: 400 in September, 1,070 in October' }))
    expect(screen.getByRole('dialog', { name: 'Source E-003' })).toBeInTheDocument()
  })

  it('opens the drawer for a pending record from a search hit and clears it', async () => {
    const user = userEvent.setup()
    renderDetail()
    await screen.findByRole('heading', { level: 1 })
    await user.click(screen.getByRole('button', { name: 'Search hit C-002' }))
    expect(await screen.findByRole('dialog', { name: 'Finding C-002' })).toBeInTheDocument()
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
    await user.click(await screen.findByRole('button', { name: /^Open finding C-002:/ }))
    current = detailWith({
      claims: claims.map((c) => (c.ref === 'C-002' ? { ...c, statement: 'Rewritten by Codex.' } : c)),
    })
    await user.click(screen.getByRole('button', { name: 'Reload' }))
    const drawer = await screen.findByRole('dialog', { name: 'Finding C-002' })
    await waitFor(() => expect(within(drawer).getByText('Rewritten by Codex.')).toBeInTheDocument())
  })

  it('says a record was not found when it disappears from the latest detail', async () => {
    const user = userEvent.setup()
    renderDetail()
    await user.click(await screen.findByRole('button', { name: /^Open finding C-004:/ }))
    current = detailWith({ claims: claims.filter((c) => c.ref !== 'C-004') })
    await user.click(screen.getByRole('button', { name: 'Reload' }))
    const drawer = screen.getByRole('dialog', { name: 'Finding C-004' })
    expect(await within(drawer).findByText(/^This was not found\. It may have changed in another app\./)).toBeInTheDocument()
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
    const claimsSection = await screen.findByRole('region', { name: 'What backs this up' })
    await user.click(within(claimsSection).getByRole('button', { name: 'Add finding' }))
    await user.type(screen.getByLabelText('Finding'), 'The bills went out twice because a file was loaded two times.')
    await user.click(within(screen.getByRole('dialog', { name: 'Add finding' })).getByRole('button', { name: 'Add finding' }))
    await waitFor(() =>
      expect(document.querySelector('.reading-column [data-ref="C-005"] .row-hit')).toHaveFocus(),
    )
  })

  it('opens the add dialogs from every section', async () => {
    const user = userEvent.setup()
    renderDetail()
    const cases: [string, string][] = [
      ['Not tested yet', 'Add finding to test'],
      ['Sources', 'Add source'],
      ['Not tested yet', 'Add follow-up question'],
    ]
    for (const [section, label] of cases) {
      // Every fixture source sits under a finding, so the section for the rest reads Other sources.
      const region = await screen.findByRole('region', { name: section === 'Sources' ? 'Other sources' : section })
      await user.click(within(region).getByRole('button', { name: label }))
      expect(screen.getByRole('dialog', { name: label })).toBeInTheDocument()
      await user.click(within(screen.getByRole('dialog', { name: label })).getByRole('button', { name: 'Cancel' }))
    }
  })

  it('shows Answer, Set aside and Edit on open follow-up questions', async () => {
    const user = userEvent.setup()
    renderDetail()
    const row = (await screen.findByRole('region', { name: 'Not tested yet' })).querySelector('[data-ref="Q-001"]') as HTMLElement
    await user.click(within(row).getByRole('button', { name: 'Answer' }))
    expect(screen.getByRole('dialog', { name: 'Answer this follow-up question' })).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Cancel' }))
    await user.click(within(row).getByRole('button', { name: 'Set aside' }))
    expect(screen.getByRole('dialog', { name: 'Set this follow-up question aside' })).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Cancel' }))
    await user.click(within(row).getByRole('button', { name: 'Edit follow-up question' }))
    expect(screen.getByRole('dialog', { name: 'Edit follow-up question' })).toBeInTheDocument()
    expect(within(row).queryByRole('button', { name: 'Reopen' })).toBeNull()
  })

  it('reopens resolved and deferred questions', async () => {
    const user = userEvent.setup()
    renderDetail()
    const section = await screen.findByRole('region', { name: 'Not tested yet' })
    const deferred = section.querySelector('[data-ref="Q-003"]') as HTMLElement
    expect(within(deferred).getByRole('button', { name: 'Reopen' })).toBeInTheDocument()
    await user.click(within(section).getByRole('button', { name: 'Show 1 answered follow-up question' }))
    const resolved = section.querySelector('[data-ref="Q-002"]') as HTMLElement
    expect(within(resolved).queryByRole('button', { name: 'Answer' })).toBeNull()
    await user.click(within(resolved).getByRole('button', { name: 'Reopen' }))
    expect(mocks.api.reopenQuestion).toHaveBeenCalledWith('INV-0001', 'Q-002')
  })

  it('focuses a newly added question', async () => {
    const user = userEvent.setup()
    renderDetail()
    const added = question({ id: 4, ref: 'Q-004', question: 'Who signs off a refund over $100?' })
    vi.mocked(mocks.api.addQuestion).mockImplementation(async () => {
      current = detailWith({ questions: [...questions, added] })
      return added
    })
    const section = await screen.findByRole('region', { name: 'Not tested yet' })
    await user.click(within(section).getByRole('button', { name: 'Add follow-up question' }))
    await user.type(screen.getByLabelText('Follow-up question'), 'Who signs off a refund over $100?')
    await user.click(within(screen.getByRole('dialog', { name: 'Add follow-up question' })).getByRole('button', { name: 'Add follow-up question' }))
    await waitFor(() => expect(document.querySelector('[data-ref="Q-004"]')).toHaveFocus())
  })
})

