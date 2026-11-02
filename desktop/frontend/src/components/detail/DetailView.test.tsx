import { act, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { Api } from '../../api/api'
import type { InvestigationDetail } from '../../api/types'
import { AppDataProvider } from '../../state/appData'
import { CopyContextProvider } from '../../state/copyContext'
import { detail, questions, summaries } from '../../test/fixtures'
import { archivedDetail, emptyDetail, manyRuledOutDetail, partialDetail } from '../../test/fixtures.detail'
import { mockApi } from '../../test/mockApi'
import { ToastProvider } from '../ui/Toaster'
import { DetailView } from './DetailView'

const mocks = vi.hoisted(() => ({ api: {} as Api }))
vi.mock('../../api/api', () => ({ api: mocks.api }))

function renderDetail(d: InvestigationDetail = detail) {
  Object.assign(
    mocks.api,
    mockApi({
      listInvestigations: async () => [{ ...summaries[0], ref: d.investigation.ref, is_active: true }],
      getInvestigationDetail: async () => d,
    }),
  )
  return render(
    <ToastProvider>
      <AppDataProvider>
        <CopyContextProvider>
          <DetailView />
        </CopyContextProvider>
      </AppDataProvider>
    </ToastProvider>,
  )
}

const section = (name: string) => screen.getByRole('region', { name })

describe('DetailView reading column', () => {
  beforeEach(() => {
    Object.assign(mocks.api, mockApi())
  })

  it('offers only Add evidence beside the Evidence heading, with no Evernote capture', async () => {
    renderDetail()
    await screen.findByRole('heading', { level: 1, name: 'September cloud bill up 38%' })
    expect(within(section('Evidence')).getByRole('button', { name: 'Add evidence' })).toBeInTheDocument()
    expect(within(section('Evidence')).queryByRole('button', { name: 'More ways to add evidence' })).toBeNull()
    expect(screen.queryByText(/evernote/i)).toBeNull()
  })

  it('renders the sections in the D-12 order after the question', async () => {
    renderDetail()
    await screen.findByRole('heading', { level: 1, name: 'September cloud bill up 38%' })
    expect(screen.getByText('The question')).toBeInTheDocument()
    const h2 = screen.getAllByRole('heading', { level: 2 }).map((h) => h.textContent)
    expect(h2).toEqual([
      'Current conclusion',
      'What we believe',
      'What might explain it',
      'Evidence',
      'Still open',
      'History',
    ])
    const label = screen.getByText('The question')
    const conclusion = screen.getByRole('heading', { level: 2, name: 'Current conclusion' })
    expect(label.compareDocumentPosition(conclusion) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  })

  it('shows the conclusion panel with the large meter, label and Why: rationale', async () => {
    renderDetail()
    const panel = await screen.findByRole('region', { name: 'Current conclusion' })
    expect(within(panel).getByText('Debug logging requested for payments only reached every service and drove most of the increase.')).toBeInTheDocument()
    expect(within(panel).getByText('High confidence')).toBeInTheDocument()
    expect(panel.querySelector('.confidence-lg')).not.toBeNull()
    expect(within(panel).getByText('Why:')).toBeInTheDocument()
    const rationale = 'The change, the pricing page and the billing export agree; the remaining 29% is still open.'
    expect(within(panel).getByText(rationale)).toBeInTheDocument()
    // Confidence is never shown as a percentage; only the rationale's own figure carries one.
    expect(panel.textContent!.replace(rationale, '')).not.toMatch(/%/)
  })

  it('collapses ruled-out records under singular disclosures', async () => {
    const user = userEvent.setup()
    renderDetail()
    await screen.findByRole('region', { name: 'What we believe' })
    const believe = section('What we believe')
    expect(within(believe).queryByText('The increase comes from the August autoscaling change adding compute nodes.')).toBeNull()
    await user.click(within(believe).getByRole('button', { name: 'Show 1 superseded or rejected claim' }))
    const c3 = believe.querySelector('[data-ref="C-003"]') as HTMLElement
    expect(within(c3).getByText('Superseded by C-002')).toBeInTheDocument()

    const explain = section('What might explain it')
    expect(within(explain).getByRole('button', { name: 'Show 1 ruled-out hypothesis' })).toBeInTheDocument()
    expect(explain.querySelector('[data-ref="H-002"]')).toBeNull()

    const open = section('Still open')
    expect(within(open).queryByText('Resolved')).toBeNull()
    await user.click(within(open).getByRole('button', { name: 'Show 1 resolved question' }))
    expect(within(open).getByText('Resolved')).toBeInTheDocument()
    expect(within(open).getByRole('button', { name: 'Hide 1 resolved question' })).toBeInTheDocument()
  })

  it('uses plural and singular disclosure copy by count', async () => {
    renderDetail(manyRuledOutDetail())
    await screen.findByRole('region', { name: 'What we believe' })
    expect(
      within(section('What we believe')).getByRole('button', { name: 'Show 3 superseded or rejected claims' }),
    ).toBeInTheDocument()
    expect(
      within(section('Still open')).getByRole('button', { name: 'Show 1 resolved question' }),
    ).toBeInTheDocument()
  })

  it('lists open explanations and deferred questions under Still open without expanding anything', async () => {
    const user = userEvent.setup()
    renderDetail()
    await screen.findByRole('region', { name: 'Still open' })
    const open = section('Still open')
    const list = within(open).getByRole('list', { name: 'Explanations still open' })
    expect(within(open).getByRole('heading', { level: 3, name: 'Explanations still open' })).toBeInTheDocument()
    // H-001 is Open; H-003 is Supported and H-002 Disproven, so neither repeats here.
    expect(list.querySelector('[data-ref="H-001"]')).not.toBeNull()
    expect(open.querySelector('[data-ref="H-003"]')).toBeNull()
    expect(open.querySelector('[data-ref="H-002"]')).toBeNull()
    expect(list.querySelector('.record-foot')).toBeNull()
    // The explanations come before the questions, which have their own subheading.
    const q1 = open.querySelector('[data-ref="Q-001"]') as HTMLElement
    expect(list.compareDocumentPosition(q1) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    const questionsHeading = within(open).getByRole('heading', { level: 3, name: 'Questions' })
    expect(list.compareDocumentPosition(questionsHeading) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    expect(questionsHeading.compareDocumentPosition(q1) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    const deferred = open.querySelector('[data-ref="Q-003"]') as HTMLElement
    expect(within(deferred).getByText('Deferred')).toBeInTheDocument()
    expect(within(deferred).getByText('Waiting for the quarter close on October 10.')).toBeInTheDocument()
    await user.click(within(list).getByRole('button', { name: /^Open hypothesis H-001:/ }))
    expect(await screen.findByRole('dialog', { name: 'Hypothesis H-001' })).toBeInTheDocument()
  })

  it('keeps the Still open empty copy when nothing is open', async () => {
    renderDetail({ ...detail, hypotheses: detail.hypotheses.filter((h) => h.status !== 'open'), questions: [questions[1]] })
    await screen.findByRole('region', { name: 'Still open' })
    const open = section('Still open')
    expect(within(open).getByText('Nothing is open. Add a question when something still needs an answer.')).toBeInTheDocument()
    expect(within(open).queryByText('Explanations still open')).toBeNull()
    expect(within(open).queryByRole('heading', { level: 3, name: 'Questions' })).toBeNull()
    expect(within(open).getByRole('button', { name: 'Show 1 resolved question' })).toBeInTheDocument()
  })

  it('renders every empty section copy for a new investigation', async () => {
    renderDetail(emptyDetail())
    await screen.findByRole('heading', { level: 1, name: 'New investigation' })
    for (const text of [
      'No question written yet.',
      'No conclusion yet. When the evidence points somewhere, write it here.',
      'No claims yet. Add one when the evidence supports a clear statement.',
      'No hypotheses yet. Add one to test a possible explanation.',
      'No evidence yet. Add what you found and where you found it.',
      'Nothing is open. Add a question when something still needs an answer.',
      'No history yet.',
    ]) {
      expect(screen.getByText(text)).toBeInTheDocument()
    }
  })

  it('guides an empty case to its first finding, through the AI or by hand', async () => {
    const user = userEvent.setup()
    renderDetail(emptyDetail())
    const guide = await screen.findByRole('region', { name: 'Get your first finding' })
    expect(guide).toHaveTextContent('Ask your AI to work this question and save what it finds here. Connect it once, then use the first finding prompt.')
    const connect = within(guide).getByRole('button', { name: 'Connect AI clients...' })
    const add = within(guide).getByRole('button', { name: 'Add evidence myself' })
    await user.tab()
    while (document.activeElement !== connect) await user.tab()
    await user.tab()
    expect(add).toHaveFocus()
    await user.click(connect)
    const dialog = await screen.findByRole('dialog', { name: 'Connect AI clients' })
    await user.click(within(dialog).getByRole('button', { name: 'Done' }))
    await user.click(add)
    expect(await screen.findByRole('dialog', { name: 'Add evidence' })).toBeInTheDocument()
  })

  it('hides the first finding guide once the case has any record', async () => {
    renderDetail({ ...emptyDetail(), questions: [questions[0]] })
    await screen.findByRole('heading', { level: 1, name: 'New investigation' })
    expect(screen.queryByRole('region', { name: 'Get your first finding' })).not.toBeInTheDocument()
  })

  it('renders the missing question and conclusion copy beside the rest of the detail', async () => {
    renderDetail(partialDetail())
    await screen.findByRole('region', { name: 'What we believe' })
    expect(screen.getByText('No question written yet.')).toBeInTheDocument()
    expect(screen.getByText('No conclusion yet. When the evidence points somewhere, write it here.')).toBeInTheDocument()
    expect(section('Evidence').querySelector('[data-ref="E-001"]')).not.toBeNull()
  })

  it('shows the archived banner and unarchives with a toast', async () => {
    const user = userEvent.setup()
    renderDetail(archivedDetail())
    expect(
      await screen.findByText('This case is archived. It is hidden from the list and nothing was deleted.'),
    ).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Unarchive' }))
    expect(mocks.api.unarchiveInvestigation).toHaveBeenCalledWith('INV-0003')
    expect(await screen.findByText('Unarchived INV-0003.')).toBeInTheDocument()
  })

  it('opens Edit title and question from the question block and from Write the question', async () => {
    const user = userEvent.setup()
    renderDetail()
    await user.click(await screen.findByRole('button', { name: 'Edit title and question' }))
    const dialog = screen.getByRole('dialog', { name: 'Edit title and question' })
    expect(within(dialog).getByLabelText('Title')).toHaveValue('September cloud bill up 38%')
    await user.click(within(dialog).getByRole('button', { name: 'Cancel' }))
    expect(screen.queryByRole('dialog')).toBeNull()
  })

  it('offers Write the question when the problem statement is empty', async () => {
    const user = userEvent.setup()
    renderDetail(partialDetail())
    await user.click(await screen.findByRole('button', { name: 'Write the question' }))
    expect(screen.getByRole('dialog', { name: 'Edit title and question' })).toBeInTheDocument()
  })

  it('shows the status picker in the header', async () => {
    renderDetail()
    expect(await screen.findByRole('button', { name: 'Status: Investigating' })).toBeInTheDocument()
  })

  it('wraps long text in the reading column and clamps only evidence summaries', async () => {
    renderDetail()
    await screen.findByRole('region', { name: 'Evidence' })
    expect(document.querySelectorAll('.evidence-summary.is-clamped').length).toBe(4)
    expect(document.querySelectorAll('.record-statement.is-clamped').length).toBe(0)
  })
})

describe('DetailView loading', () => {
  afterEach(() => {
    vi.useRealTimers()
  })

  it('shows nothing before 300 ms and then the header plus skeleton blocks', async () => {
    vi.useFakeTimers()
    Object.assign(
      mocks.api,
      mockApi({
        listInvestigations: async () => [summaries[0]],
        getInvestigationDetail: () => new Promise<InvestigationDetail>(() => {}),
      }),
    )
    render(
      <ToastProvider>
        <AppDataProvider>
          <CopyContextProvider>
            <DetailView />
          </CopyContextProvider>
        </AppDataProvider>
      </ToastProvider>,
    )
    // Let startup, the list load and the detail request begin.
    await act(async () => {
      await vi.advanceTimersByTimeAsync(0)
    })
    expect(document.querySelector('.detail[aria-busy="true"]')).not.toBeNull()
    await act(async () => {
      await vi.advanceTimersByTimeAsync(299)
    })
    expect(document.querySelector('.detail-skeleton')).toBeNull()
    await act(async () => {
      await vi.advanceTimersByTimeAsync(2)
    })
    expect(document.querySelector('.detail-skeleton')).not.toBeNull()
    expect(screen.getByText('September cloud bill up 38%')).toBeInTheDocument()
  })
})
