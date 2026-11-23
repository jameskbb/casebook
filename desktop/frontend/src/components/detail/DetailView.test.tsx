import { act, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { Api } from '../../api/api'
import type { InvestigationDetail } from '../../api/types'
import { AppDataProvider } from '../../state/appData'
import { CopyContextProvider } from '../../state/copyContext'
import { detail, evidence, questions, summaries } from '../../test/fixtures'
import { archivedDetail, emptyDetail, manyRuledOutDetail, partialDetail } from '../../test/fixtures.detail'
import { bareEvidence, detailWith, detailWithBareEvidence } from '../../test/fixtures.records'
import { mockApi } from '../../test/mockApi'
import { OpenAtLaunch } from '../../test/OpenAtLaunch'
import { ADD_TO_FIND_OUT } from '../dialogs/QuestionDialog'
import { LATER } from '../record/StatusLabel'
import { ToastProvider } from '../ui/Toaster'
import { CHECK_SOURCES_TITLE, SOURCE_CHECKED_ACTION } from './CheckSources'
import { CHECK_NEXT_SOURCE, SHARE_THE_ANSWER } from './ConclusionPanel'
import { DetailView, SOURCES_HEADING } from './DetailView'
import { ADD_FINDING, ADD_SOURCE } from './FirstFindingGuide'
import { BACK_IN_LIST_COPY, PUT_BACK_LABEL } from './MoreMenu'

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

  it('offers one row of add buttons for a source, a finding and something to find out, in the same order as the first screen, with no Evernote capture', async () => {
    renderDetail()
    const add = await screen.findByRole('group', { name: 'Add to this question' })
    expect(within(add).getAllByRole('button').map((b) => b.textContent)).toEqual([ADD_SOURCE, ADD_FINDING, ADD_TO_FIND_OUT, 'What the labels mean'])
    expect(screen.queryByText(/evernote/i)).toBeNull()
  })

  it('nests each finding\'s sources under it, with Backs it or Goes against it in words and its own check label', async () => {
    renderDetail()
    const backing = await screen.findByRole('region', { name: 'What backs this up' })
    const c1 = backing.querySelector('[data-ref="C-001"]') as HTMLElement
    const sources = within(within(c1).getByRole('list', { name: 'Sources for this finding' })).getAllByRole('listitem')
    expect(sources.map((li) => li.textContent?.replace(/\s+/g, ' ').trim())).toEqual([
      'Backs it: E-001 Notice dated October 4: 1,900 customers were charged twice Not checked',
      'Backs it: E-004 Head of billing: the mistake was on October 3 only You checked this',
    ])
    // The code is in the page for an AI and for tests, but hidden from a reader.
    expect(within(sources[0]).getByText('E-001')).toHaveClass('record-id')
    // A source that goes against a finding says so in words, under that finding.
    const c4 = section('Where sources disagree').querySelector('[data-ref="C-004"]') as HTMLElement
    const against = within(c4).getByRole('button', { name: /^Goes against it:/ })
    expect(against).toHaveClass('is-against')
    expect(against).toHaveTextContent('Calls about a bill')
    // A finding with no sources has no list.
    const h1 = section('Still to find out').querySelector('[data-ref="H-001"]') as HTMLElement
    expect(within(h1).queryByRole('list')).toBeNull()
  })

  it('has one way to check: Check the next source opens the source check, and Escape closes it', async () => {
    const user = userEvent.setup()
    renderDetail()
    await screen.findByRole('region', { name: 'What backs this up' })
    expect(document.body).not.toHaveTextContent(/one by one|review findings/i)
    // No finding offers its own check on the page.
    expect(within(section('What backs this up')).queryByRole('button', { name: 'I checked this' })).toBeNull()
    const next = screen.getByRole('button', { name: CHECK_NEXT_SOURCE })
    await user.click(next)
    const check = await screen.findByRole('dialog', { name: CHECK_SOURCES_TITLE })
    expect(within(check).getByRole('button', { name: SOURCE_CHECKED_ACTION })).toHaveFocus()
    await user.keyboard('{Escape}')
    expect(screen.queryByRole('dialog', { name: CHECK_SOURCES_TITLE })).toBeNull()
    expect(next).toHaveFocus()
  })

  it('keeps Share the answer beside the main button while sources are left to check', async () => {
    renderDetail()
    const panel = await screen.findByRole('region', { name: 'The answer' })
    expect(within(panel).getByRole('button', { name: CHECK_NEXT_SOURCE })).toHaveClass('btn-primary')
    expect(within(panel).getByRole('button', { name: SHARE_THE_ANSWER })).toHaveClass('btn-secondary')
  })

  it('lists every source once under Sources, with where to find it, and a replaced one behind its own line', async () => {
    const user = userEvent.setup()
    const replaced = { ...bareEvidence, ref: 'E-010', title: 'An older call count', verification: 'superseded' as const }
    renderDetail(detailWith({ evidence: [...evidence, bareEvidence, replaced] }))
    const list = await screen.findByRole('region', { name: SOURCES_HEADING })
    const refs = Array.from(list.querySelectorAll('.evidence-row')).map((r) => r.getAttribute('data-ref'))
    // Sources a finding uses are listed too, each once.
    expect(refs).toEqual([...evidence.map((e) => e.ref), 'E-009'])
    expect(new Set(refs).size).toBe(refs.length)
    expect(within(list).getByText(`Where to find it: ${evidence[0].source_ref}`)).toBeInTheDocument()
    expect(within(list).getByRole('button', { name: `Open source E-001: ${evidence[0].title}` })).toBeInTheDocument()
    expect(screen.queryByRole('region', { name: 'Other sources' })).toBeNull()
    await user.click(within(list).getByRole('button', { name: 'Show 1 replaced source' }))
    expect(within(list).getByText('An older call count')).toBeInTheDocument()
  })

  it('shows the exact words under a source in Sources when asked, as the source drawer does', async () => {
    const user = userEvent.setup()
    renderDetail()
    Object.assign(mocks.api, { getEvidence: vi.fn(async (_inv: string, ref: string) => ({ ...evidence.find((e) => e.ref === ref)!, raw_detail: 'Calls about a bill: 400, then 1,070.' })) })
    const list = await screen.findByRole('region', { name: SOURCES_HEADING })
    const row = list.querySelector<HTMLElement>('.evidence-row[data-ref="E-003"]')!
    // Only a source with exact words offers them.
    expect(list.querySelector('.evidence-row[data-ref="E-002"] .exact-words')).toBeNull()
    const show = within(row).getByRole('button', { name: 'Show the exact words' })
    expect(show).toHaveAttribute('aria-expanded', 'false')
    await user.click(show)
    expect(await within(row).findByText('Calls about a bill: 400, then 1,070.')).toBeInTheDocument()
    expect(within(row).getByRole('button', { name: 'Hide the exact words' })).toHaveAttribute('aria-expanded', 'true')
  })

  it('renders the sections in reading order after the question, leaving out the empty ones', async () => {
    renderDetail()
    await screen.findByRole('heading', { level: 1, name: 'Why did calls go up 30% in October when sales stayed the same?' })
    const h2 = screen.getAllByRole('heading', { level: 2 }).map((h) => h.textContent)
    expect(h2).toEqual(['The answer', 'What backs this up', 'Where sources disagree', 'Still to find out', 'Sources', 'How we got here'])
    const label = screen.getByText('The question')
    const conclusion = screen.getByRole('heading', { level: 2, name: 'The answer' })
    expect(label.compareDocumentPosition(conclusion) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  })

  it('shows the question as the big line and the background under it, labelled Background', async () => {
    renderDetail()
    const block = await screen.findByRole('region', { name: 'The question' })
    expect(block.querySelector('.question-block-text')).toHaveTextContent(detail.investigation.title)
    expect(within(block).getByText('Background')).toBeInTheDocument()
    expect(block.querySelector('.question-block-background-text')).toHaveTextContent(detail.investigation.problem_statement)
  })

  it('shows the answer with the count of sources checked, no confidence, and how you know', async () => {
    renderDetail()
    const panel = await screen.findByRole('region', { name: 'The answer' })
    expect(within(panel).getByText('The extra calls were customers asking about a bill. A billing mistake on October 3 charged some customers twice. About the same number of people called to buy, so sales stayed the same.')).toBeInTheDocument()
    expect(within(panel).queryByText(/confidence/i)).toBeNull()
    const live = detail.evidence.filter((e) => e.verification !== 'superseded')
    const byYou = live.filter((e) => e.verification === 'human_verified').length
    expect(panel.querySelector('.conclusion-checked')).toHaveTextContent(`${byYou} of ${live.length} sources checked by you`)
    expect(within(panel).getByText('How you know:')).toBeInTheDocument()
    const rationale = "The billing team's notice and the call log agree. Whether long waits cost any sales is still open."
    expect(within(panel).getByText(rationale)).toBeInTheDocument()
  })

  it('collapses replaced and ruled out findings, and shows disagreements in their own section', async () => {
    const user = userEvent.setup()
    renderDetail()
    const believe = await screen.findByRole('region', { name: 'What backs this up' })
    expect(screen.queryByText('The extra calls were people calling to buy.')).toBeNull()
    await user.click(screen.getByRole('button', { name: 'Show 1 replaced finding' }))
    const c3 = document.querySelector('[data-ref="C-003"]') as HTMLElement
    expect(c3.querySelector('.record-status')).toHaveTextContent('Replaced by C-002')
    expect(within(c3).getByText('by C-002')).toHaveClass('record-id')

    // A supported hypothesis backs the answer beside the claims; a disputed
    // claim moves to its own section and a disproven hypothesis behind its line.
    expect(believe.querySelector('[data-ref="H-003"]')).not.toBeNull()
    expect(believe.querySelector('[data-ref="C-004"]')).toBeNull()
    expect(section('Where sources disagree').querySelector('[data-ref="C-004"]')).not.toBeNull()
    expect(screen.queryByRole('region', { name: 'Ruled out' })).toBeNull()
    expect(document.querySelector('[data-ref="H-002"]')).toBeNull()
    await user.click(screen.getByRole('button', { name: 'Show 1 ruled out finding' }))
    expect(document.querySelector('[data-ref="H-002"]')).not.toBeNull()
    expect(screen.getByRole('button', { name: 'Hide 1 ruled out finding' })).toHaveAttribute('aria-expanded', 'true')

    // An answered follow-up question sits behind its own disclosure, with its answer and no state word.
    const open = section('Still to find out')
    expect(open.querySelector('[data-ref="Q-002"]')).toBeNull()
    await user.click(within(open).getByRole('button', { name: 'Show 1 answered' }))
    const answered = open.querySelector('[data-ref="Q-002"]') as HTMLElement
    expect(answered.querySelector('.question-row-answer')).toHaveTextContent('Yes. Both go to the sales line')
    expect(within(open).queryByText('Answered')).toBeNull()
    expect(within(open).getByRole('button', { name: 'Hide 1 answered' })).toBeInTheDocument()
  })

  it('uses plural and singular disclosure copy by count', async () => {
    renderDetail(manyRuledOutDetail())
    await screen.findByRole('region', { name: 'What backs this up' })
    expect(screen.getByRole('button', { name: 'Show 1 replaced finding' })).toBeInTheDocument()
    // Two rejected claims and the disproven hypothesis.
    expect(screen.getByRole('button', { name: 'Show 3 ruled out findings' })).toBeInTheDocument()
    expect(within(section('Still to find out')).getByRole('button', { name: 'Show 1 answered' })).toBeInTheDocument()
  })

  it('lists a finding not tested yet and the follow-up questions together under Still to find out, with one add button', async () => {
    const user = userEvent.setup()
    renderDetail()
    await screen.findByRole('region', { name: 'Still to find out' })
    expect(screen.queryByRole('region', { name: 'Not tested yet' })).toBeNull()
    const open = section('Still to find out')
    // One list, no subheadings: the finding to test first, then the open question, then the one left for later.
    expect(within(open).queryAllByRole('heading', { level: 3 })).toHaveLength(0)
    const lists = Array.from(open.querySelectorAll('ul.section-rows'))
    expect(lists).toHaveLength(1)
    expect(Array.from(lists[0].children).map((li) => li.querySelector('[data-ref]')?.getAttribute('data-ref'))).toEqual(['H-001', 'Q-001', 'Q-003'])
    // H-003 looks right so far and H-002 is ruled out, so neither repeats here.
    expect(open.querySelector('[data-ref="H-003"]')).toBeNull()
    expect(open.querySelector('[data-ref="H-002"]')).toBeNull()
    // Exactly one add button for both kinds, in the add row above the sections.
    expect(within(open).queryAllByRole('button', { name: /^Add/ })).toHaveLength(0)
    expect(screen.getAllByRole('button', { name: ADD_TO_FIND_OUT })).toHaveLength(1)
    expect(ADD_TO_FIND_OUT).toBe('Add something to find out')
    const later = open.querySelector('[data-ref="Q-003"]') as HTMLElement
    expect(within(later).getByText(LATER)).toBeInTheDocument()
    expect(within(later).getByText('Waiting for the refund list on November 10.')).toBeInTheDocument()
    await user.click(within(open).getByRole('button', { name: /^Open finding H-001:/ }))
    expect(await screen.findByRole('dialog', { name: 'Finding H-001' })).toBeInTheDocument()
  })

  it('keeps Still to find out for its Show N answered line when only answered ones are left', async () => {
    renderDetail({ ...detail, hypotheses: detail.hypotheses.filter((h) => h.status !== 'open'), questions: [questions[1]] })
    const open = await screen.findByRole('region', { name: 'Still to find out' })
    expect(within(open).queryByText('Nothing is left to find out.')).toBeNull()
    expect(within(open).getByRole('button', { name: 'Show 1 answered' })).toBeInTheDocument()
  })

  it('leaves Still to find out off the page when there is nothing in it', async () => {
    renderDetail({ ...detail, hypotheses: detail.hypotheses.filter((h) => h.status !== 'open'), questions: [] })
    await screen.findByRole('region', { name: 'What backs this up' })
    expect(screen.queryByRole('region', { name: 'Still to find out' })).toBeNull()
  })

  it('shows a new question with No answer yet, two add buttons and the AI hint, and no empty sections', async () => {
    const user = userEvent.setup()
    renderDetail(emptyDetail())
    await screen.findByRole('heading', { level: 1, name: 'Why did returns double in May?' })
    const panel = screen.getByRole('region', { name: 'The answer' })
    expect(within(panel).getByText('No answer yet.')).toBeInTheDocument()
    expect(within(panel).queryByRole('button')).toBeNull()
    const start = screen.getByRole('region', { name: 'Start here' })
    expect(within(start).getAllByRole('button').map((b) => b.textContent)).toEqual([ADD_SOURCE, ADD_FINDING, 'Connect your AI...'])
    expect(start).toHaveTextContent('Your AI can add sources and findings here too.')
    expect(screen.getAllByRole('heading', { level: 2 }).map((h) => h.textContent)).toEqual(['The answer'])
    expect(screen.queryByRole('group', { name: 'Add to this question' })).toBeNull()
    await user.click(within(start).getByRole('button', { name: ADD_SOURCE }))
    expect(await screen.findByRole('dialog', { name: 'Add source' })).toBeInTheDocument()
  })

  it('hides the first finding guide once the case has any record', async () => {
    renderDetail({ ...emptyDetail(), questions: [questions[0]] })
    await screen.findByRole('heading', { level: 1, name: 'Why did returns double in May?' })
    expect(screen.queryByRole('region', { name: 'Start here' })).not.toBeInTheDocument()
  })

  it('shows the title as the question when there is no background, and the missing answer copy', async () => {
    renderDetail(partialDetail())
    await screen.findByRole('region', { name: 'What backs this up' })
    expect(screen.getByRole('region', { name: 'The question' })).toHaveTextContent('Why did calls go up 30% in October when sales stayed the same?')
    expect(screen.getByText('No answer yet.')).toBeInTheDocument()
    expect(section('What backs this up').querySelector('button[data-ref="E-001"]')).not.toBeNull()
  })

  it('shows the archived banner and puts the question back in the list with a toast', async () => {
    const user = userEvent.setup()
    renderDetail(archivedDetail())
    expect(
      await screen.findByText('This question is archived. It is hidden from the list and nothing was deleted.'),
    ).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Unarchive' })).toBeNull()
    await user.click(screen.getByRole('button', { name: PUT_BACK_LABEL }))
    expect(mocks.api.unarchiveInvestigation).toHaveBeenCalledWith('INV-0003')
    expect(await screen.findByText(BACK_IN_LIST_COPY)).toBeInTheDocument()
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

  it('wraps long text in the reading column and clamps nothing', async () => {
    renderDetail(detailWithBareEvidence())
    await screen.findByRole('region', { name: SOURCES_HEADING })
    expect(document.querySelectorAll('.is-clamped').length).toBe(0)
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
