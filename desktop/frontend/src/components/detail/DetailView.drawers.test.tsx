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
import { ADD_TO_FIND_OUT } from '../dialogs/QuestionDialog'
import { ADD_FINDING, ADD_SOURCE } from './FirstFindingGuide'
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
      name: 'Goes against it: E-003 Calls about a bill: 400 in September, 1,070 in October Not checked',
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

  it('names sources and findings in How we got here as buttons that open drawers, and leaves what is still to find out as text', async () => {
    const user = userEvent.setup()
    renderDetail(
      detailWith({
        recent_activity: [activityRow(human, 'question.resolve', 'Q-001', 'Answered.'), ...activity],
      }),
    )
    const history = await screen.findByRole('region', { name: 'How we got here' })
    expect(within(history).queryByRole('button', { name: 'Q-001' })).toBeNull()
    expect(within(history).getByText('answered something to find out')).toBeInTheDocument()
    // Only the last three rows show until Show all opens the rest.
    await user.click(within(history).getByRole('button', { name: /^Show all/ }))
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

  it('says a record is no longer here when it disappears from the latest detail', async () => {
    const user = userEvent.setup()
    renderDetail()
    await user.click(await screen.findByRole('button', { name: /^Open finding C-004:/ }))
    current = detailWith({ claims: claims.filter((c) => c.ref !== 'C-004') })
    await user.click(screen.getByRole('button', { name: 'Reload' }))
    const drawer = screen.getByRole('dialog', { name: 'Finding C-004' })
    expect(await within(drawer).findByText(/^This is no longer here\. Your AI may have changed it\./)).toBeInTheDocument()
  })
})

describe('DetailView adding and question actions', () => {
  beforeEach(() => {
    Object.assign(mocks.api, mockApi())
  })

  it('adds a finding from the add row and moves focus to the new row', async () => {
    const user = userEvent.setup()
    renderDetail()
    vi.mocked(mocks.api.addClaim).mockImplementation(async () => {
      current = detailWithNewClaim()
      return newClaim
    })
    await screen.findByRole('region', { name: 'What backs this up' })
    await user.click(within(screen.getByRole('group', { name: 'Add to this question' })).getByRole('button', { name: ADD_FINDING }))
    await user.type(screen.getByLabelText('Finding'), 'The bills went out twice because a file was loaded two times.')
    await user.click(within(screen.getByRole('dialog', { name: 'Add finding' })).getByRole('button', { name: 'Add finding' }))
    await waitFor(() =>
      expect(document.querySelector('.reading-column [data-ref="C-005"] .row-hit')).toHaveFocus(),
    )
  })

  it('opens each add dialog from the add row', async () => {
    const user = userEvent.setup()
    renderDetail()
    const add = await screen.findByRole('group', { name: 'Add to this question' })
    const cases: [string, string][] = [
      [ADD_FINDING, 'Add finding'],
      [ADD_SOURCE, 'Add source'],
      [ADD_TO_FIND_OUT, ADD_TO_FIND_OUT],
    ]
    for (const [button, title] of cases) {
      await user.click(within(add).getByRole('button', { name: button }))
      const dialog = screen.getByRole('dialog', { name: title })
      await user.click(within(dialog).getByRole('button', { name: 'Cancel' }))
    }
  })

  it('offers Answer, Later and Edit on something open to find out', async () => {
    const user = userEvent.setup()
    renderDetail()
    const row = (await screen.findByRole('region', { name: 'Still to find out' })).querySelector('[data-ref="Q-001"]') as HTMLElement
    await user.click(within(row).getByRole('button', { name: 'Answer' }))
    expect(screen.getByRole('dialog', { name: 'What did you find out?' })).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Cancel' }))
    await user.click(within(row).getByRole('button', { name: 'Later' }))
    const later = screen.getByRole('dialog', { name: 'Come back to this later' })
    expect(within(later).getByLabelText('Why it can wait (optional)')).toBeInTheDocument()
    await user.click(within(later).getByRole('button', { name: 'Cancel' }))
    await user.click(within(row).getByRole('button', { name: /^Edit: / }))
    expect(screen.getByRole('dialog', { name: 'Edit what to find out' })).toBeInTheDocument()
    expect(within(row).queryByRole('button', { name: /Set aside|Reopen|Ask again/ })).toBeNull()
  })

  it('offers Answer and no Later on something left for later', async () => {
    const user = userEvent.setup()
    renderDetail()
    const row = (await screen.findByRole('region', { name: 'Still to find out' })).querySelector('[data-ref="Q-003"]') as HTMLElement
    expect(within(row).queryByRole('button', { name: 'Later' })).toBeNull()
    expect(within(row).queryByRole('button', { name: /Reopen|Ask again/ })).toBeNull()
    expect(within(row).queryByRole('button', { name: /^Edit: / })).toBeNull()
    expect(within(row).getByRole('button', { name: /^Delete: / })).toBeInTheDocument()
    await user.click(within(row).getByRole('button', { name: 'Answer' }))
    expect(screen.getByRole('dialog', { name: 'What did you find out?' })).toBeInTheDocument()
  })

  it('keeps answered ones behind Show N answered, where Ask again reopens one', async () => {
    const user = userEvent.setup()
    renderDetail()
    const section = await screen.findByRole('region', { name: 'Still to find out' })
    expect(section.querySelector('[data-ref="Q-002"]')).toBeNull()
    expect(within(section).queryByRole('button', { name: 'Ask again' })).toBeNull()
    await user.click(within(section).getByRole('button', { name: 'Show 1 answered' }))
    const answered = section.querySelector('[data-ref="Q-002"]') as HTMLElement
    expect(within(answered).queryByRole('button', { name: 'Answer' })).toBeNull()
    expect(within(answered).queryByRole('button', { name: 'Later' })).toBeNull()
    await user.click(within(answered).getByRole('button', { name: 'Ask again' }))
    expect(mocks.api.reopenQuestion).toHaveBeenCalledTimes(1)
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
    await screen.findByRole('region', { name: 'Still to find out' })
    await user.click(screen.getByRole('button', { name: ADD_TO_FIND_OUT }))
    await user.type(screen.getByLabelText('What do you still need to find out?'), 'Who signs off a refund over $100?')
    await user.click(within(screen.getByRole('dialog', { name: ADD_TO_FIND_OUT })).getByRole('button', { name: 'Add' }))
    expect(mocks.api.addQuestion).toHaveBeenCalledWith('INV-0001', expect.objectContaining({ question: 'Who signs off a refund over $100?' }))
    expect(mocks.api.addHypothesis).not.toHaveBeenCalled()
    await waitFor(() => expect(document.querySelector('[data-ref="Q-004"]')).toHaveFocus())
  })
})

