// What the second tester found, kept fixed: a finding a person adds takes its
// label from its sources, the check is one pass that ends with the answer,
// Decide opens that pass, the answer panel counts the findings as they are
// now, and a finding not tested yet is settled from its row.
import { useEffect, useRef } from 'react'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Api } from '../../api/api'
import type { Claim, ClaimInput, InvestigationDetail } from '../../api/types'
import { AppDataProvider, useAppData } from '../../state/appData'
import { CopyContextProvider } from '../../state/copyContext'
import { claims, detail, evidence, summaries } from '../../test/fixtures'
import { mockApi } from '../../test/mockApi'
import { DECIDED_HOLDS, SOURCES_DISAGREE } from '../../lib/labels'
import { RULE_OUT_ACTION, TURNED_OUT_RIGHT_ACTION } from '../drawers/StatementDrawer'
import { ToastProvider } from '../ui/Toaster'
import { ALL_CHECKED, CHECK_SOURCES_TITLE, DISAGREE_HEADING, SOURCE_CHECKED_ACTION, WRITE_THE_ANSWER } from './CheckSources'
import { CHECK_NEXT_SOURCE, JUST_WROTE_ANSWER, UNDO_ANSWER_CHECK } from './ConclusionPanel'
import { DetailView } from './DetailView'
import { ADD_FINDING } from './FirstFindingGuide'

const mocks = vi.hoisted(() => ({ api: {} as Api }))
vi.mock('../../api/api', () => ({ api: mocks.api }))

/** Opens the question once the list is in, as Home's buttons do. */
function Open({ what }: { what?: string }) {
  const { listStatus, select } = useAppData()
  const done = useRef(false)
  useEffect(() => {
    if (done.current || listStatus !== 'ready') return
    done.current = true
    select('INV-0001', what ? { openRecord: what } : undefined)
  }, [listStatus, select, what])
  return null
}

let current: InvestigationDetail

function renderPage(d: InvestigationDetail, what?: string, api: Partial<Api> = {}) {
  current = d
  Object.assign(
    mocks.api,
    mockApi({
      listInvestigations: async () => [summaries[0]],
      getInvestigationDetail: async () => current,
      ...api,
    }),
  )
  render(
    <ToastProvider>
      <AppDataProvider>
        <Open what={what} />
        <CopyContextProvider>
          <DetailView />
        </CopyContextProvider>
      </AppDataProvider>
    </ToastProvider>,
  )
  return userEvent.setup()
}

const allChecked = () => current.evidence.map((e) => ({ ...e, verification: e.verification === 'superseded' ? e.verification : ('human_verified' as const) }))

describe('round 4 on the question page', () => {
  beforeEach(() => {
    Object.assign(mocks.api, mockApi())
  })

  it('puts a finding a person adds, with one source for and one against, under Where sources disagree', async () => {
    const user = renderPage(detail, undefined, {
      addClaim: async (_inv: string, input: ClaimInput) => {
        // The engine marks a person's write as checked unless told otherwise.
        const added: Claim = {
          ...claims[0],
          id: 9,
          ref: 'C-009',
          statement: input.statement,
          verification: input.verification || 'human_verified',
          links: (input.links ?? []).map((l) => ({ ...claims[0].links[0], evidence_ref: l.evidence_ref, relation: l.relation, evidence_title: evidence.find((e) => e.ref === l.evidence_ref)!.title })),
        }
        current = { ...current, claims: [...current.claims, added] }
        return added
      },
    })
    await user.click(await screen.findByRole('button', { name: ADD_FINDING }))
    const dialog = screen.getByRole('dialog', { name: 'Add finding' })
    await user.type(within(dialog).getByLabelText('Finding'), 'Most of the extra calls came on October 4.')
    const rows = dialog.querySelectorAll<HTMLElement>('.statement-link-row')
    await user.click(within(rows[0]).getByRole('radio', { name: 'Backs it up' }))
    await user.click(within(rows[2]).getByRole('radio', { name: 'Goes against it' }))
    await user.click(within(dialog).getByRole('button', { name: 'Add finding' }))
    expect(mocks.api.addClaim).toHaveBeenCalledWith('INV-0001', expect.objectContaining({ verification: 'ai_generated' }))
    const disagree = await screen.findByRole('region', { name: 'Where sources disagree' })
    const row = await waitFor(() => {
      const r = disagree.querySelector<HTMLElement>('[data-ref="C-009"]')
      expect(r).not.toBeNull()
      return r!
    })
    expect(row).toHaveTextContent(SOURCES_DISAGREE)
    expect(row).not.toHaveTextContent(DECIDED_HOLDS)
  })

  it('opens the check on the finding where sources disagree when Home says Decide', async () => {
    renderPage({ ...detail, evidence: detail.evidence.map((e) => ({ ...e, verification: 'human_verified' as const })) }, 'decide')
    const dialog = await screen.findByRole('dialog', { name: CHECK_SOURCES_TITLE })
    expect(within(dialog).getByRole('heading', { name: DISAGREE_HEADING })).toBeInTheDocument()
    expect(within(dialog).getByText('Callers waited too long and hung up before buying.')).toBeInTheDocument()
  })

  it('opens the check from Home with focus on its main button, and Escape puts focus on Check the next source', async () => {
    const user = renderPage(detail, 'check-sources')
    const dialog = await screen.findByRole('dialog', { name: CHECK_SOURCES_TITLE })
    await waitFor(() => expect(within(dialog).getByRole('button', { name: SOURCE_CHECKED_ACTION })).toHaveFocus())
    await user.keyboard('{Escape}')
    expect(screen.queryByRole('dialog', { name: CHECK_SOURCES_TITLE })).toBeNull()
    await waitFor(() => expect(screen.getByRole('button', { name: CHECK_NEXT_SOURCE })).toHaveFocus())
  })

  it('ends the check with Write the answer, which opens the answer editor', async () => {
    const d = { ...detail, investigation: { ...detail.investigation, conclusion: '', conclusion_rationale: '' }, claims: detail.claims.filter((c) => c.ref !== 'C-004') }
    current = d
    const user = renderPage({ ...d, evidence: allChecked() }, 'check-sources')
    const dialog = await screen.findByRole('dialog', { name: CHECK_SOURCES_TITLE })
    expect(within(dialog).getByText(ALL_CHECKED)).toBeInTheDocument()
    await user.click(within(dialog).getByRole('button', { name: WRITE_THE_ANSWER }))
    expect(screen.queryByRole('dialog', { name: CHECK_SOURCES_TITLE })).toBeNull()
    await waitFor(() => expect(within(screen.getByRole('region', { name: 'The answer' })).getByLabelText('Answer')).toHaveFocus())
  })

  it('settles a finding not tested yet from its row under Still to find out', async () => {
    const user = renderPage(detail)
    const open = await screen.findByRole('region', { name: 'Still to find out' })
    const row = open.querySelector<HTMLElement>('[data-ref="H-001"]')!
    await user.click(within(row).getByRole('button', { name: TURNED_OUT_RIGHT_ACTION }))
    expect(mocks.api.updateHypothesis).toHaveBeenCalledWith('INV-0001', 'H-001', { status: 'supported' })
    await user.click(within(row).getByRole('button', { name: RULE_OUT_ACTION }))
    expect(mocks.api.updateHypothesis).toHaveBeenCalledWith('INV-0001', 'H-001', { status: 'disproven' })
  })

  it('counts only the findings that stand now under the answer', async () => {
    const inv = { ...detail.investigation, conclusion_supporting_claims: ['C-001', 'C-002', 'C-003'], conclusion_contradicting_claims: ['C-004'] }
    // C-004 was ruled out after the answer was saved; C-003 was replaced long ago.
    renderPage({ ...detail, investigation: inv, claims: detail.claims.map((c) => (c.ref === 'C-004' ? { ...c, status: 'rejected' as const, verification: 'ai_generated' as const } : c)) })
    const panel = await screen.findByRole('region', { name: 'The answer' })
    // The same count as What backs this up on the page: C-001, C-002 and H-003, not the stored list.
    const listed = new Set([...screen.getByRole('region', { name: 'What backs this up' }).querySelectorAll('[data-ref]')].map((e) => e.getAttribute('data-ref')).filter((r) => /^[CH]-/.test(r ?? ''))).size
    expect(listed).toBe(3)
    expect(within(panel).getByText('3 findings back it')).toBeInTheDocument()
    expect(within(panel).queryByText(/goes? against it/)).toBeNull()
  })

  it('offers Undo my check on an answer the person checked, and saves it back as not checked', async () => {
    const inv = { ...detail.investigation, conclusion_reviewed_at: '2026-11-08T12:00:00.000Z', conclusion_needs_review: false, conclusion_review_basis: 'Read the call log.' }
    const user = renderPage({ ...detail, investigation: inv })
    const panel = await screen.findByRole('region', { name: 'The answer' })
    await user.click(within(panel).getByRole('button', { name: UNDO_ANSWER_CHECK }))
    expect(mocks.api.updateConclusion).toHaveBeenCalledWith('INV-0001', { conclusion: inv.conclusion, confidence: inv.conclusion_confidence, rationale: inv.conclusion_rationale, verification: 'ai_generated' })
  })

  it('says the person wrote the answer right after they save it', async () => {
    const empty = { ...detail.investigation, conclusion: '', conclusion_rationale: '' }
    const user = renderPage({ ...detail, investigation: empty }, undefined, {
      updateConclusion: async (_ref, input) => {
        current = { ...current, investigation: { ...current.investigation, conclusion: input.conclusion, conclusion_reviewed_at: '', conclusion_needs_review: true } }
        return current.investigation
      },
    })
    const panel = await screen.findByRole('region', { name: 'The answer' })
    await user.click(within(panel).getByRole('button', { name: 'Write the answer' }))
    await user.type(within(panel).getByLabelText('Answer'), 'A billing mistake made the extra calls.')
    await user.click(within(panel).getByRole('button', { name: 'Save answer' }))
    expect(await within(panel).findByText(JUST_WROTE_ANSWER)).toBeInTheDocument()
  })
})
