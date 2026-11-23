import { act, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { Api } from '../../api/api'
import type { InvestigationDetail } from '../../api/types'
import { AppDataProvider } from '../../state/appData'
import { CopyContextProvider } from '../../state/copyContext'
import { detail, questions, summaries } from '../../test/fixtures'
import { archivedDetail, emptyDetail, manyRuledOutDetail, partialDetail } from '../../test/fixtures.detail'
import { detailWithBareEvidence } from '../../test/fixtures.records'
import { mockApi } from '../../test/mockApi'
import { OpenAtLaunch } from '../../test/OpenAtLaunch'
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
      <AppDataProvider><OpenAtLaunch />
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

  it('offers only Add source in the sources section, with no Evernote capture', async () => {
    renderDetail()
    await screen.findByRole('heading', { level: 1, name: 'Why did calls go up 30% in October when sales stayed the same?' })
    expect(within(section('Other sources')).getByRole('button', { name: 'Add source' })).toBeInTheDocument()
    expect(within(section('Other sources')).queryByRole('button', { name: 'More ways to add evidence' })).toBeNull()
    expect(screen.queryByText(/evernote/i)).toBeNull()
  })

  it('nests each finding\'s sources under it, with a sign, where it came from and its own check label', async () => {
    renderDetail()
    const backing = await screen.findByRole('region', { name: 'What backs this up' })
    const c1 = backing.querySelector('[data-ref="C-001"]') as HTMLElement
    const sources = within(within(c1).getByRole('list', { name: 'Sources for this finding' })).getAllByRole('listitem')
    expect(sources.map((li) => li.textContent?.replace(/\s+/g, ' ').trim())).toEqual([
      'Supports this: E-001 Notice dated October 4: 1,900 customers were charged twice Email from the billing team Not checked',
      'Supports this: E-004 Head of billing: the mistake was on October 3 only Head of billing, in person You checked this',
    ])
    // The code is in the page for an AI and for tests, but hidden from a reader.
    expect(within(sources[0]).getByText('E-001')).toHaveClass('record-id')
    // A source that contradicts a finding carries the minus class, under the finding it contradicts.
    const c4 = section('Where sources disagree').querySelector('[data-ref="C-004"]') as HTMLElement
    const against = within(c4).getByRole('button', { name: /^Contradicts this:/ })
    expect(against).toHaveClass('is-against')
    expect(against).toHaveTextContent('Call log')
    // A finding with no sources has no list.
    const h1 = section('Not tested yet').querySelector('[data-ref="H-001"]') as HTMLElement
    expect(within(h1).queryByRole('list')).toBeNull()
  })

  it('lists only the sources no finding uses under Other sources, and says so when there are none', async () => {
    const { unmount } = renderDetail()
    await screen.findByRole('region', { name: 'Other sources' })
    expect(section('Other sources').querySelectorAll('.evidence-row')).toHaveLength(0)
    expect(within(section('Other sources')).getByText('Every source is under the finding it backs or contradicts.')).toBeInTheDocument()
    unmount()
    renderDetail(detailWithBareEvidence())
    await screen.findByRole('region', { name: 'Other sources' })
    const rows = section('Other sources').querySelectorAll('.evidence-row')
    expect(Array.from(rows).map((r) => r.getAttribute('data-ref'))).toEqual(['E-009'])
  })

  it('renders the sections in reading order after the question', async () => {
    renderDetail()
    await screen.findByRole('heading', { level: 1, name: 'Why did calls go up 30% in October when sales stayed the same?' })
    expect(screen.getByText('The question')).toBeInTheDocument()
    const h2 = screen.getAllByRole('heading', { level: 2 }).map((h) => h.textContent)
    expect(h2).toEqual([
      'The answer',
      'What backs this up',
      'Where sources disagree',
      'Ruled out',
      'Not tested yet',
      'Other sources',
      'How we got here',
    ])
    const label = screen.getByText('The question')
    const conclusion = screen.getByRole('heading', { level: 2, name: 'The answer' })
    expect(label.compareDocumentPosition(conclusion) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  })

  it('shows the answer with the count of sources checked, no confidence, and the Why: rationale', async () => {
    renderDetail()
    const panel = await screen.findByRole('region', { name: 'The answer' })
    expect(within(panel).getByText('The extra calls were customers asking about a bill. A billing mistake on October 3 charged some customers twice. About the same number of people called to buy, so sales stayed the same.')).toBeInTheDocument()
    expect(within(panel).queryByText(/confidence/i)).toBeNull()
    const live = detail.evidence.filter((e) => e.verification !== 'superseded')
    const byYou = live.filter((e) => e.verification === 'human_verified').length
    expect(panel.querySelector('.conclusion-checked')).toHaveTextContent(`${byYou} of ${live.length} sources checked by you`)
    expect(within(panel).getByText('Why:')).toBeInTheDocument()
    const rationale = "The billing team's notice and the call log agree. Whether long waits cost any sales is still open."
    expect(within(panel).getByText(rationale)).toBeInTheDocument()
  })

  it('collapses replaced findings, and shows disagreements and ruled-out findings in their own sections', async () => {
    const user = userEvent.setup()
    renderDetail()
    await screen.findByRole('region', { name: 'What backs this up' })
    const believe = section('What backs this up')
    expect(within(believe).queryByText('The extra calls were people calling to buy.')).toBeNull()
    await user.click(within(believe).getByRole('button', { name: 'Show 1 replaced finding' }))
    const c3 = believe.querySelector('[data-ref="C-003"]') as HTMLElement
    expect(c3.querySelector('.record-status')).toHaveTextContent('Replaced by C-002')
    expect(within(c3).getByText('by C-002')).toHaveClass('record-id')

    // A supported hypothesis backs the answer beside the claims; a disputed
    // claim and a disproven hypothesis each move to their own section.
    expect(believe.querySelector('[data-ref="H-003"]')).not.toBeNull()
    expect(believe.querySelector('[data-ref="C-004"]')).toBeNull()
    expect(section('Where sources disagree').querySelector('[data-ref="C-004"]')).not.toBeNull()
    expect(section('Ruled out').querySelector('[data-ref="H-002"]')).not.toBeNull()

    const open = section('Not tested yet')
    expect(within(open).queryByText('Answered')).toBeNull()
    await user.click(within(open).getByRole('button', { name: 'Show 1 answered follow-up question' }))
    expect(within(open).getByText('Answered')).toBeInTheDocument()
    expect(within(open).getByRole('button', { name: 'Hide 1 answered follow-up question' })).toBeInTheDocument()
  })

  it('uses plural and singular disclosure copy by count', async () => {
    renderDetail(manyRuledOutDetail())
    await screen.findByRole('region', { name: 'What backs this up' })
    expect(
      within(section('What backs this up')).getByRole('button', { name: 'Show 1 replaced finding' }),
    ).toBeInTheDocument()
    // The two rejected claims are ruled out, in the open.
    expect(section('Ruled out').querySelectorAll('.record-row')).toHaveLength(3)
    expect(
      within(section('Not tested yet')).getByRole('button', { name: 'Show 1 answered follow-up question' }),
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
    expect(within(deferred).getByText('Set aside')).toBeInTheDocument()
    expect(within(deferred).getByText('Waiting for the refund list on November 10.')).toBeInTheDocument()
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
    expect(within(open).getByRole('button', { name: 'Show 1 answered follow-up question' })).toBeInTheDocument()
  })

  it('renders every empty section copy for a new question', async () => {
    renderDetail(emptyDetail())
    await screen.findByRole('heading', { level: 1, name: 'Why did returns double in May?' })
    // With no sources at all the section is plainly Sources.
    expect(screen.getByRole('region', { name: 'Sources' })).toBeInTheDocument()
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
    expect(guide).toHaveTextContent('Ask your AI to work on this question and save what it finds here. Connect it once, then copy the message it offers to start with.')
    const connect = within(guide).getByRole('button', { name: 'Connect your AI...' })
    const add = within(guide).getByRole('button', { name: 'Add a source myself' })
    await user.tab()
    while (document.activeElement !== connect) await user.tab()
    await user.tab()
    expect(add).toHaveFocus()
    await user.click(connect)
    const dialog = await screen.findByRole('dialog', { name: 'Connect your AI' })
    await user.click(within(dialog).getByRole('button', { name: 'Done' }))
    await user.click(add)
    expect(await screen.findByRole('dialog', { name: 'Add source' })).toBeInTheDocument()
  })

  it('hides the first finding guide once the case has any record', async () => {
    renderDetail({ ...emptyDetail(), questions: [questions[0]] })
    await screen.findByRole('heading', { level: 1, name: 'Why did returns double in May?' })
    expect(screen.queryByRole('region', { name: 'Get your first finding' })).not.toBeInTheDocument()
  })

  it('shows the title as the question when there is no background, and the missing answer copy', async () => {
    renderDetail(partialDetail())
    await screen.findByRole('region', { name: 'What backs this up' })
    expect(screen.getByRole('region', { name: 'The question' })).toHaveTextContent('Why did calls go up 30% in October when sales stayed the same?')
    expect(screen.getByText('No answer yet. Write it here when the findings back one.')).toBeInTheDocument()
    expect(section('What backs this up').querySelector('button[data-ref="E-001"]')).not.toBeNull()
  })

  it('shows the archived banner and unarchives with a toast', async () => {
    const user = userEvent.setup()
    renderDetail(archivedDetail())
    expect(
      await screen.findByText('This question is archived. It is hidden from the list and nothing was deleted.'),
    ).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Unarchive' }))
    expect(mocks.api.unarchiveInvestigation).toHaveBeenCalledWith('INV-0003')
    expect(await screen.findByText('Unarchived this question. It is back in the list.')).toBeInTheDocument()
  })

  it('opens Edit question from the question block', async () => {
    const user = userEvent.setup()
    renderDetail()
    await user.click(await screen.findByRole('button', { name: 'Edit question' }))
    const dialog = screen.getByRole('dialog', { name: 'Edit question' })
    expect(within(dialog).getByLabelText('Question')).toHaveValue('Why did calls go up 30% in October when sales stayed the same?')
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

  it('wraps long text in the reading column and clamps only source summaries', async () => {
    renderDetail(detailWithBareEvidence())
    await screen.findByRole('region', { name: 'Other sources' })
    expect(document.querySelectorAll('.evidence-summary.is-clamped').length).toBe(1)
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
        <AppDataProvider><OpenAtLaunch />
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
    expect(screen.getByText('Why did calls go up 30% in October when sales stayed the same?')).toBeInTheDocument()
  })
})
