import { act, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { Api } from '../../api/api'
import type { InvestigationDetail } from '../../api/types'
import { AppDataProvider } from '../../state/appData'
import { CopyContextProvider } from '../../state/copyContext'
import { claims, detail, evidence, questions, summaries } from '../../test/fixtures'
import { archivedDetail, emptyDetail, manyRuledOutDetail, partialDetail } from '../../test/fixtures.detail'
import { bareEvidence, detailWith, detailWithBareEvidence } from '../../test/fixtures.records'
import { mockApi } from '../../test/mockApi'
import { OpenAtLaunch } from '../../test/OpenAtLaunch'
import { ADD_TO_FIND_OUT } from '../dialogs/QuestionDialog'
import { LATER } from '../record/StatusLabel'
import { ToastProvider } from '../ui/Toaster'
import { ALL_SOURCES_USED_COPY, DetailView } from './DetailView'
import { BACK_IN_LIST_COPY, CHECK_FINDINGS_LABEL, PUT_BACK_LABEL } from './MoreMenu'

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
      'Backs this: E-001 Notice dated October 4: 1,900 customers were charged twice Email from the billing team Not checked',
      'Backs this: E-004 Head of billing: the mistake was on October 3 only Head of billing, in person You checked this',
    ])
    // The code is in the page for an AI and for tests, but hidden from a reader.
    expect(within(sources[0]).getByText('E-001')).toHaveClass('record-id')
    // A source that goes against a finding carries the minus class, under that finding.
    const c4 = section('Where sources disagree').querySelector('[data-ref="C-004"]') as HTMLElement
    const against = within(c4).getByRole('button', { name: /^Goes against this:/ })
    expect(against).toHaveClass('is-against')
    expect(against).toHaveTextContent('Call log')
    // A finding with no sources has no list.
    const h1 = section('Still to find out').querySelector('[data-ref="H-001"]') as HTMLElement
    expect(within(h1).queryByRole('list')).toBeNull()
  })

  it('has no Review findings button: the walk-through opens from the More menu and goes back to the question', async () => {
    const user = userEvent.setup()
    renderDetail()
    await screen.findByRole('region', { name: 'What backs this up' })
    expect(screen.queryByRole('button', { name: /review findings/i })).toBeNull()
    expect(screen.queryByRole('button', { name: CHECK_FINDINGS_LABEL })).toBeNull()
    expect(document.body).not.toHaveTextContent(/review findings/i)
    await user.click(screen.getByRole('button', { name: 'More' }))
    await user.click(screen.getByRole('menuitem', { name: CHECK_FINDINGS_LABEL }))
    const walk = await screen.findByRole('region', { name: CHECK_FINDINGS_LABEL })
    expect(within(walk).getByRole('heading', { level: 2, name: CHECK_FINDINGS_LABEL })).toBeInTheDocument()
    // The walk-through takes the place of the reading column; the header stays.
    expect(screen.queryByRole('region', { name: 'What backs this up' })).toBeNull()
    expect(screen.getByRole('heading', { level: 1 })).toBeInTheDocument()
    await user.click(within(walk).getByRole('button', { name: 'Back to the question' }))
    expect(screen.queryByRole('region', { name: CHECK_FINDINGS_LABEL })).toBeNull()
    expect(screen.getByRole('region', { name: 'What backs this up' })).toBeInTheDocument()
  })

  it('lists only the sources no finding uses under Other sources, and says so when there are none', async () => {
    const { unmount } = renderDetail()
    await screen.findByRole('region', { name: 'Other sources' })
    expect(section('Other sources').querySelectorAll('.evidence-row')).toHaveLength(0)
    expect(within(section('Other sources')).getByText(ALL_SOURCES_USED_COPY)).toBeInTheDocument()
    unmount()
    renderDetail(detailWithBareEvidence())
    await screen.findByRole('region', { name: 'Other sources' })
    const rows = section('Other sources').querySelectorAll('.evidence-row')
    expect(Array.from(rows).map((r) => r.getAttribute('data-ref'))).toEqual(['E-009'])
  })

  it('lists a source whose only finding is replaced under Other sources, and not one under a finding that stands', async () => {
    // E-009 backs only C-003, which C-002 replaced, so it would otherwise sit behind Show replaced findings.
    const lone = { ...bareEvidence, claim_links: [{ ref: 'C-003', relation: 'supports' as const }] }
    renderDetail(
      detailWith({
        evidence: [...evidence, lone],
        claims: claims.map((c) =>
          c.ref === 'C-003' ? { ...c, links: [...c.links, { ...c.links[0], evidence_ref: lone.ref, evidence_title: lone.title, relation: 'supports' as const }] } : c,
        ),
      }),
    )
    const other = await screen.findByRole('region', { name: 'Other sources' })
    const refs = Array.from(other.querySelectorAll('.evidence-row')).map((r) => r.getAttribute('data-ref'))
    // E-001 backs C-001, which stands. E-003 is used by the replaced C-003 too, but also by C-002, which stands.
    expect(refs).toEqual(['E-009'])
    expect(within(other).getByText(lone.title)).toBeInTheDocument()
    expect(within(other).queryByText(ALL_SOURCES_USED_COPY)).toBeNull()
    expect(section('What backs this up').querySelector('button[data-ref="E-001"]')).not.toBeNull()
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
      'Still to find out',
      'Other sources',
      'How we got here',
    ])
    const label = screen.getByText('The question')
    const conclusion = screen.getByRole('heading', { level: 2, name: 'The answer' })
    expect(label.compareDocumentPosition(conclusion) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
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
    expect(
      within(section('What backs this up')).getByRole('button', { name: 'Show 1 replaced finding' }),
    ).toBeInTheDocument()
    // The two rejected claims are ruled out, in the open.
    expect(section('Ruled out').querySelectorAll('.record-row')).toHaveLength(3)
    expect(
      within(section('Still to find out')).getByRole('button', { name: 'Show 1 answered' }),
    ).toBeInTheDocument()
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
    // Exactly one add button for both kinds.
    const adds = within(open).getAllByRole('button', { name: /^Add/ })
    expect(adds).toHaveLength(1)
    expect(adds[0]).toHaveAccessibleName(ADD_TO_FIND_OUT)
    expect(ADD_TO_FIND_OUT).toBe('Add something to find out')
    const later = open.querySelector('[data-ref="Q-003"]') as HTMLElement
    expect(within(later).getByText(LATER)).toBeInTheDocument()
    expect(within(later).getByText('Waiting for the refund list on November 10.')).toBeInTheDocument()
    await user.click(within(open).getByRole('button', { name: /^Open finding H-001:/ }))
    expect(await screen.findByRole('dialog', { name: 'Finding H-001' })).toBeInTheDocument()
  })

  it('keeps the Still to find out empty copy and its add button when only answered questions are left', async () => {
    renderDetail({ ...detail, hypotheses: detail.hypotheses.filter((h) => h.status !== 'open'), questions: [questions[1]] })
    await screen.findByRole('region', { name: 'Still to find out' })
    const open = section('Still to find out')
    expect(within(open).getByText('Nothing is left to find out.')).toBeInTheDocument()
    expect(within(open).getAllByRole('button', { name: ADD_TO_FIND_OUT })).toHaveLength(1)
    expect(within(open).getByRole('button', { name: 'Show 1 answered' })).toBeInTheDocument()
  })

  it('renders every empty section copy for a new question', async () => {
    renderDetail(emptyDetail())
    await screen.findByRole('heading', { level: 1, name: 'Why did returns double in May?' })
    // With no sources at all the section is plainly Sources.
    expect(screen.getByRole('region', { name: 'Sources' })).toBeInTheDocument()
    for (const text of [
      'No answer yet. Write it here when the findings back one.',
      'No findings yet. Your AI adds them here, or you can add one.',
      'Nothing is left to find out.',
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
    expect(guide).toHaveTextContent('Ask your AI to work on this question and save what it finds here. Connect your AI once. Then copy the starter message and paste it into your AI.')
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
