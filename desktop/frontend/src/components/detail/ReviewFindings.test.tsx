import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Api } from '../../api/api'
import { ApiError, CHANGED_COPY, WENT_WRONG_COPY } from '../../api/errors'
import type { Evidence, ReviewFinding } from '../../api/types'
import { detail } from '../../test/fixtures'
import { reviewFinding } from '../../test/fixtures.review'
import { mockApi } from '../../test/mockApi'
import { CHECKED_ACTION, CONTRADICTING_TITLE, DISAGREE_ACTION, FINDING_CHANGED_COPY, SETTLE_ACTION, SETTLE_COPY, SUPPORTING_TITLE } from '../drawers/StatementDrawer'
import { SHOW_NEW_VERSION } from '../ui/ChangedElsewhereBanner'
import { CHECK_FINDINGS_LABEL } from './MoreMenu'
import { ReviewFindings, sourceWarnings } from './ReviewFindings'

const mocks = vi.hoisted(() => ({ api: {} as Api, write: vi.fn() }))
vi.mock('../../api/api', () => ({ api: mocks.api }))
vi.mock('../../state/useWrite', () => ({ useWrite: () => mocks.write }))

function mount() { return render(<ReviewFindings detail={detail} onClose={vi.fn()} />) }

describe('Check findings one by one', () => {
  beforeEach(() => {
    Object.assign(mocks.api, mockApi())
    mocks.write.mockReset().mockImplementation((fn: () => unknown) => fn())
  })
  it('shows one finding with the sources that back it up and go against it, and their exact words', async () => {
    const user = userEvent.setup()
    mount()
    expect(await screen.findByText(reviewFinding.statement)).toBeInTheDocument()
    expect(screen.getByRole('region', { name: SUPPORTING_TITLE })).toHaveTextContent(reviewFinding.evidence[0].evidence.title)
    expect(screen.getByRole('region', { name: CONTRADICTING_TITLE })).toHaveTextContent(reviewFinding.evidence[1].evidence.title)
    await user.click(screen.getByText('Show the exact words', { selector: 'summary' }))
    expect(screen.getByText('Source detail inspected by the person')).toBeVisible()
    expect(screen.getByRole('region', { name: CHECK_FINDINGS_LABEL })).toBeInTheDocument()
    expect(screen.getByRole('heading', { level: 2, name: CHECK_FINDINGS_LABEL })).toBeInTheDocument()
    expect(screen.queryByText(/review|verif/i)).not.toBeInTheDocument()
  })
  it('checks only the finding as shown and waits for Next', async () => {
    const user = userEvent.setup()
    mount()
    await screen.findByText(reviewFinding.statement)
    await user.click(screen.getByRole('button', { name: CHECKED_ACTION }))
    expect(mocks.api.reviewStatement).toHaveBeenCalledWith(detail.investigation.ref, {
      kind: 'claim', ref: reviewFinding.ref, basis: reviewFinding.basis, verification: 'human_verified', settle_dispute: false,
    })
    expect(await screen.findByText('You checked this finding.')).toBeInTheDocument()
    expect(screen.getByText(reviewFinding.statement)).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Next' }))
    expect(screen.getByText('No more findings in this list')).toBeInTheDocument()
  })
  it('skips without writing, and a finding only the AI checked can be checked by a person', async () => {
    const user = userEvent.setup()
    Object.assign(mocks.api, mockApi({ getReviewFindings: async () => [reviewFinding, { ...reviewFinding, ref: 'C-010', verification: 'source_verified', filters: ['source_checked'] }] }))
    mount()
    await screen.findByText(reviewFinding.statement)
    await user.click(screen.getByRole('button', { name: 'Skip' }))
    expect(mocks.api.reviewStatement).not.toHaveBeenCalled()
    await user.selectOptions(screen.getByLabelText('Which findings'), 'source_checked')
    expect(screen.getByRole('button', { name: CHECKED_ACTION })).toBeEnabled()
    await user.click(screen.getByRole('button', { name: CHECKED_ACTION }))
    expect(mocks.api.reviewStatement).toHaveBeenCalledWith(detail.investigation.ref, expect.objectContaining({ ref: 'C-010', verification: 'human_verified' }))
  })
  it('asks before checking a finding whose sources disagree', async () => {
    const user = userEvent.setup()
    Object.assign(mocks.api, mockApi({ getReviewFindings: async () => [{ ...reviewFinding, verification: 'disputed', status: 'disputed', filters: ['disputed'] }] }))
    mount()
    await waitFor(() => expect(screen.queryByText('Loading the findings...')).not.toBeInTheDocument())
    await user.selectOptions(screen.getByLabelText('Which findings'), 'disputed')
    await user.click(screen.getByRole('button', { name: CHECKED_ACTION }))
    expect(mocks.api.reviewStatement).not.toHaveBeenCalled()
    expect(screen.getByText(SETTLE_COPY)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: DISAGREE_ACTION })).toBeDisabled()
    await user.click(screen.getByRole('button', { name: SETTLE_ACTION }))
    expect(mocks.api.reviewStatement).toHaveBeenCalledWith(detail.investigation.ref, expect.objectContaining({ settle_dispute: true }))
  })
  it('holds the finding as shown when it changes elsewhere, until Show the new version', async () => {
    const user = userEvent.setup()
    const view = mount()
    await screen.findByText(reviewFinding.statement)
    vi.mocked(mocks.api.getReviewFindings).mockResolvedValue([{ ...reviewFinding, statement: 'An AI changed the finding', basis: 'new-snapshot' }])
    view.rerender(<ReviewFindings detail={{ ...detail }} onClose={vi.fn()} />)
    await screen.findByText(FINDING_CHANGED_COPY)
    expect(screen.getByRole('button', { name: CHECKED_ACTION })).toBeDisabled()
    expect(screen.getByText(reviewFinding.statement)).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: SHOW_NEW_VERSION }))
    await user.click(screen.getByRole('button', { name: CHECKED_ACTION }))
    expect(mocks.api.reviewStatement).toHaveBeenCalledWith(detail.investigation.ref, expect.objectContaining({ basis: 'new-snapshot' }))
  })
  it('says in plain words that the finding changed, without moving on or trying again', async () => {
    const user = userEvent.setup()
    vi.mocked(mocks.api.reviewStatement).mockRejectedValue(new ApiError('conflict', 'This finding changed; show latest.'))
    mount()
    await screen.findByText(reviewFinding.statement)
    await user.click(screen.getByRole('button', { name: CHECKED_ACTION }))
    expect(await screen.findByText(CHANGED_COPY)).toBeInTheDocument()
    expect(screen.queryByText(/show latest/)).not.toBeInTheDocument()
    expect(screen.getByText(reviewFinding.statement)).toBeInTheDocument()
    expect(mocks.write).toHaveBeenCalledWith(expect.any(Function), { rethrowConflict: true, retry: false })
  })
  it('shows a plain message and Try again when the first load fails, and Try again loads again', async () => {
    Object.assign(mocks.api, mockApi({ getReviewFindings: async () => { throw new Error('The case could not be loaded') } }))
    const user = userEvent.setup()
    mount()
    expect(await screen.findByText(WENT_WRONG_COPY)).toBeInTheDocument()
    expect(screen.queryByText('The case could not be loaded')).not.toBeInTheDocument()
    expect(screen.queryByText('No more findings in this list')).not.toBeInTheDocument()
    expect(within(screen.getByRole('region', { name: CHECK_FINDINGS_LABEL })).queryByRole('button', { name: CHECKED_ACTION })).not.toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Try again' }))
    expect(mocks.api.getReviewFindings).toHaveBeenCalledTimes(2)
  })
  it('names the lists of findings in plain words', async () => {
    mount()
    await screen.findByText(reviewFinding.statement)
    const filter = screen.getByLabelText('Which findings')
    expect(within(filter).getAllByRole('option').map((o) => o.textContent)).toEqual([
      'Not checked yet',
      'Only your AI checked',
      'Sources disagree',
      'No source backs it',
      'All findings',
    ])
    expect(filter).toHaveValue('unchecked')
  })
  it('saves that sources disagree and says so', async () => {
    const user = userEvent.setup()
    mount()
    await screen.findByText(reviewFinding.statement)
    await user.click(screen.getByRole('button', { name: DISAGREE_ACTION }))
    expect(mocks.api.reviewStatement).toHaveBeenCalledWith(detail.investigation.ref, {
      kind: 'claim', ref: reviewFinding.ref, basis: reviewFinding.basis, verification: 'disputed', settle_dispute: false,
    })
    expect(await screen.findByText('Saved: sources disagree.')).toBeInTheDocument()
  })
  it('lists what to know under Before you check, in words and without codes', async () => {
    const replaced: ReviewFinding = { ...reviewFinding, evidence: [{ relation: 'supports', evidence: { ...reviewFinding.evidence[0].evidence, verification: 'superseded' } }] }
    Object.assign(mocks.api, mockApi({ getReviewFindings: async () => [replaced] }))
    mount()
    await screen.findByText(reviewFinding.statement)
    const warnings = screen.getByRole('heading', { level: 4, name: 'Before you check' }).parentElement as HTMLElement
    expect(within(warnings).getAllByRole('listitem').map((li) => li.textContent)).toEqual(sourceWarnings(replaced))
  })
  it('says there are no findings yet on a question with none', async () => {
    Object.assign(mocks.api, mockApi({ getReviewFindings: async () => [] }))
    mount()
    expect(await screen.findByRole('heading', { name: 'No findings yet' })).toBeInTheDocument()
    expect(screen.getByText('Findings your AI saves, and ones you add, show here for you to check against their sources.')).toBeInTheDocument()
    expect(screen.queryByText('No more findings in this list')).not.toBeInTheDocument()
  })
})

describe('sourceWarnings', () => {
  const source: Evidence = reviewFinding.evidence[0].evidence
  const backedBy = (...evidence: Evidence[]): ReviewFinding => ({
    ...reviewFinding,
    // The engine's own warnings carry codes; none of that text may come through.
    warnings: ['E-001 is superseded', 'supporting evidence E-001 is contradicted'],
    evidence: evidence.map((e) => ({ relation: 'supports' as const, evidence: e })),
  })
  const CODE = /\b(INV|[ECHQ])-\d+/

  it('says so when no source backs the finding, even when one goes against it', () => {
    expect(sourceWarnings({ ...reviewFinding, evidence: [] })).toEqual(['No source backs this finding.'])
    expect(sourceWarnings({ ...reviewFinding, evidence: [{ relation: 'contradicts', evidence: source }] })).toEqual(['No source backs this finding.'])
  })

  it('names a backing source that was replaced', () => {
    expect(sourceWarnings(backedBy({ ...source, verification: 'superseded' }))).toEqual([`"${source.title}" was replaced by a newer source.`])
  })

  it('names a backing source that sources disagree about', () => {
    expect(sourceWarnings(backedBy({ ...source, verification: 'disputed' }))).toEqual([`Sources disagree about "${source.title}".`])
  })

  it('has nothing to say about sound backing sources, and never shows a code or the engine text', () => {
    expect(sourceWarnings(backedBy(source, { ...source, ref: 'E-009', verification: 'human_verified' }))).toEqual([])
    const all = [
      ...sourceWarnings(backedBy({ ...source, verification: 'superseded' }, { ...source, ref: 'E-009', title: 'Call log', verification: 'disputed' })),
      ...sourceWarnings({ ...reviewFinding, evidence: [] }),
    ]
    expect(all).toHaveLength(3)
    for (const w of all) {
      expect(w).not.toMatch(CODE)
      expect(w).not.toMatch(/superseded|contradicted|evidence/)
    }
  })
})
