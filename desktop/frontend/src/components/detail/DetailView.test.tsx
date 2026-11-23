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

  it('offers only Add source beside the Sources heading, with no Evernote capture', async () => {
    renderDetail()
    await screen.findByRole('heading', { level: 1, name: 'September cloud bill up 38%' })
    expect(within(section('Sources')).getByRole('button', { name: 'Add source' })).toBeInTheDocument()
    expect(within(section('Sources')).queryByRole('button', { name: 'More ways to add evidence' })).toBeNull()
    expect(screen.queryByText(/evernote/i)).toBeNull()
  })

  it('renders the sections in reading order after the question', async () => {
    renderDetail()
    await screen.findByRole('heading', { level: 1, name: 'September cloud bill up 38%' })
    expect(screen.getByText('The question')).toBeInTheDocument()
    const h2 = screen.getAllByRole('heading', { level: 2 }).map((h) => h.textContent)
    expect(h2).toEqual([
      'The answer',
      'What backs this up',
      'Where sources disagree',
      'Ruled out',
      'Not tested yet',
      'Sources',
      'How we got here',
    ])
    const label = screen.getByText('The question')
    const conclusion = screen.getByRole('heading', { level: 2, name: 'The answer' })
    expect(label.compareDocumentPosition(conclusion) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  })

  it('shows the answer with the count of sources checked, no confidence, and the Why: rationale', async () => {
    renderDetail()
    const panel = await screen.findByRole('region', { name: 'The answer' })
    expect(within(panel).getByText('Debug logging requested for payments only reached every service and drove most of the increase.')).toBeInTheDocument()
    expect(within(panel).queryByText(/confidence/i)).toBeNull()
    const live = detail.evidence.filter((e) => e.verification !== 'superseded')
    const byYou = live.filter((e) => e.verification === 'human_verified').length
    expect(panel.querySelector('.conclusion-checked')).toHaveTextContent(`${byYou} of ${live.length} sources checked by you`)
    expect(within(panel).getByText('Why:')).toBeInTheDocument()
    const rationale = 'The change, the pricing page and the billing export agree; the remaining 29% is still open.'
    expect(within(panel).getByText(rationale)).toBeInTheDocument()
  })

  it('collapses replaced findings, and shows disagreements and ruled-out findings in their own sections', async () => {
    const user = userEvent.setup()
    renderDetail()
    await screen.findByRole('region', { name: 'What backs this up' })
    const believe = section('What backs this up')
    expect(within(believe).queryByText('The increase comes from the August autoscaling change adding compute nodes.')).toBeNull()
    await user.click(within(believe).getByRole('button', { name: 'Show 1 replaced finding' }))
    const c3 = believe.querySelector('[data-ref="C-003"]') as HTMLElement
    expect(within(c3).getByText('Replaced by C-002')).toBeInTheDocument()

    // A supported hypothesis backs the answer beside the claims; a disputed
    // claim and a disproven hypothesis each move to their own section.
    expect(believe.querySelector('[data-ref="H-003"]')).not.toBeNull()
    expect(believe.querySelector('[data-ref="C-004"]')).toBeNull()
    expect(section('Where sources disagree').querySelector('[data-ref="C-004"]')).not.toBeNull()
    expect(section('Ruled out').querySelector('[data-ref="H-002"]')).not.toBeNull()

    const open = section('Not tested yet')
    expect(within(open).queryByText('Resolved')).toBeNull()
    await user.click(within(open).getByRole('button', { name: 'Show 1 resolved follow-up question' }))
    expect(within(open).getByText('Resolved')).toBeInTheDocument()
    expect(within(open).getByRole('button', { name: 'Hide 1 resolved follow-up question' })).toBeInTheDocument()
  })

  it('uses plural and singular disclosure copy by count', async () => {
    renderDetail(manyRuledOutDetail())
    await screen.findByRole('region', { name: 'What backs this up' })
    expect(
      within(section('What backs this up')).getByRole('button', { name: 'Show 1 replaced finding' }),
    ).toBeInTheDocument()
    // The two rejected claims are ruled out, in the open.
    expect(section('Ruled out').querySelectorAll('[data-ref]')).toHaveLength(3)
    expect(
      within(section('Not tested yet')).getByRole('button', { name: 'Show 1 resolved follow-up question' }),
    ).toBeInTheDocument()
  })

  it('lists open explanations and deferred questions under Not tested yet without expanding anything', async () => {
    const user = userEvent.setup()
    renderDetail()
    await screen.findByRole('region', { name: 'Not tested yet' })
    const open = section('Not tested yet')
    const list = within(open).getByRole('list', { name: 'Findings to test' })
    expect(within(open).getByRole('heading', { level: 3, name: 'Findings to test' })).toBeInTheDocument()
    // H-001 is Open; H-003 is Supported and H-002 Disproven, so neither repeats here.
    expect(list.querySelector('[data-ref="H-001"]')).not.toBeNull()
    expect(open.querySelector('[data-ref="H-003"]')).toBeNull()
    expect(open.querySelector('[data-ref="H-002"]')).toBeNull()
    // The findings come before the follow-up questions, which have their own subheading.
    const q1 = open.querySelector('[data-ref="Q-001"]') as HTMLElement
    expect(list.compareDocumentPosition(q1) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    const questionsHeading = within(open).getByRole('heading', { level: 3, name: 'Follow-up questions' })
    expect(list.compareDocumentPosition(questionsHeading) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    expect(questionsHeading.compareDocumentPosition(q1) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    const deferred = open.querySelector('[data-ref="Q-003"]') as HTMLElement
    expect(within(deferred).getByText('Deferred')).toBeInTheDocument()
    expect(within(deferred).getByText('Waiting for the quarter close on October 10.')).toBeInTheDocument()
    await user.click(within(list).getByRole('button', { name: /^Open finding H-001:/ }))
    expect(await screen.findByRole('dialog', { name: 'Finding H-001' })).toBeInTheDocument()
  })

  it('keeps the Not tested yet empty copy when nothing is open', async () => {
    renderDetail({ ...detail, hypotheses: detail.hypotheses.filter((h) => h.status !== 'open'), questions: [questions[1]] })
    await screen.findByRole('region', { name: 'Not tested yet' })
    const open = section('Not tested yet')
    expect(within(open).getByText('Nothing is waiting to be tested.')).toBeInTheDocument()
    expect(within(open).queryByText('Findings to test')).toBeNull()
    expect(within(open).queryByRole('heading', { level: 3, name: 'Follow-up questions' })).toBeNull()
    expect(within(open).getByRole('button', { name: 'Show 1 resolved follow-up question' })).toBeInTheDocument()
  })

  it('renders every empty section copy for a new investigation', async () => {
    renderDetail(emptyDetail())
    await screen.findByRole('heading', { level: 1, name: 'New investigation' })
    for (const text of [
      'No answer yet. Write it here when the findings back one.',
      'No findings yet. Add one when a source backs a clear statement.',
      'Nothing is waiting to be tested.',
      'No sources yet. Add what you found and where you found it.',
      'Nothing has happened yet.',
    ]) {
      expect(screen.getByText(text)).toBeInTheDocument()
    }
    // Sections that only exist when something is in them stay off an empty page.
    expect(screen.queryByRole('region', { name: 'Where sources disagree' })).toBeNull()
    expect(screen.queryByRole('region', { name: 'Ruled out' })).toBeNull()
  })

  it('guides an empty case to its first finding, through the AI or by hand', async () => {
    const user = userEvent.setup()
    renderDetail(emptyDetail())
    const guide = await screen.findByRole('region', { name: 'Get your first finding' })
    expect(guide).toHaveTextContent('Ask your AI to work this question and save what it finds here. Connect it once, then use the first finding prompt.')
    const connect = within(guide).getByRole('button', { name: 'Connect AI clients...' })
    const add = within(guide).getByRole('button', { name: 'Add a source myself' })
    await user.tab()
    while (document.activeElement !== connect) await user.tab()
    await user.tab()
    expect(add).toHaveFocus()
    await user.click(connect)
    const dialog = await screen.findByRole('dialog', { name: 'Connect AI clients' })
    await user.click(within(dialog).getByRole('button', { name: 'Done' }))
    await user.click(add)
    expect(await screen.findByRole('dialog', { name: 'Add source' })).toBeInTheDocument()
  })

  it('hides the first finding guide once the case has any record', async () => {
    renderDetail({ ...emptyDetail(), questions: [questions[0]] })
    await screen.findByRole('heading', { level: 1, name: 'New investigation' })
    expect(screen.queryByRole('region', { name: 'Get your first finding' })).not.toBeInTheDocument()
  })

  it('shows the title as the question when there is no background, and the missing answer copy', async () => {
    renderDetail(partialDetail())
    await screen.findByRole('region', { name: 'What backs this up' })
    expect(screen.getByRole('region', { name: 'The question' })).toHaveTextContent('September cloud bill up 38%')
    expect(screen.getByText('No answer yet. Write it here when the findings back one.')).toBeInTheDocument()
    expect(section('Sources').querySelector('[data-ref="E-001"]')).not.toBeNull()
  })

  it('shows the archived banner and unarchives with a toast', async () => {
    const user = userEvent.setup()
    renderDetail(archivedDetail())
    expect(
      await screen.findByText('This question is archived. It is hidden from the list and nothing was deleted.'),
    ).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Unarchive' }))
    expect(mocks.api.unarchiveInvestigation).toHaveBeenCalledWith('INV-0003')
    expect(await screen.findByText('Unarchived INV-0003.')).toBeInTheDocument()
  })

  it('opens Edit question from the question block', async () => {
    const user = userEvent.setup()
    renderDetail()
    await user.click(await screen.findByRole('button', { name: 'Edit question' }))
    const dialog = screen.getByRole('dialog', { name: 'Edit question' })
    expect(within(dialog).getByLabelText('Question')).toHaveValue('September cloud bill up 38%')
    await user.click(within(dialog).getByRole('button', { name: 'Cancel' }))
    expect(screen.queryByRole('dialog')).toBeNull()
  })

  it('still offers Edit question when there is no background', async () => {
    const user = userEvent.setup()
    renderDetail(partialDetail())
    await user.click(await screen.findByRole('button', { name: 'Edit question' }))
    expect(screen.getByRole('dialog', { name: 'Edit question' })).toBeInTheDocument()
  })

  it('shows Answered or No answer yet in the header, with no status picker', async () => {
    const { unmount } = renderDetail()
    const header = (await screen.findByRole('heading', { level: 1 })).closest('header') as HTMLElement
    expect(within(header).getByText('Answered')).toBeInTheDocument()
    expect(within(header).queryByRole('button', { name: /^Status/ })).toBeNull()
    unmount()
    renderDetail(partialDetail())
    const empty = (await screen.findByRole('heading', { level: 1 })).closest('header') as HTMLElement
    expect(within(empty).getByText('No answer yet')).toBeInTheDocument()
  })

  it('wraps long text in the reading column and clamps only evidence summaries', async () => {
    renderDetail()
    await screen.findByRole('region', { name: 'Sources' })
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
