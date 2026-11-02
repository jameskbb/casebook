import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Api } from '../../api/api'
import { ApiError } from '../../api/errors'
import { detail } from '../../test/fixtures'
import { reviewFinding } from '../../test/fixtures.review'
import { mockApi } from '../../test/mockApi'
import { ReviewFindings } from './ReviewFindings'

const mocks = vi.hoisted(() => ({ api: {} as Api, write: vi.fn() }))
vi.mock('../../api/api', () => ({ api: mocks.api }))
vi.mock('../../state/useWrite', () => ({ useWrite: () => mocks.write }))

function mount() { return render(<ReviewFindings detail={detail} onClose={vi.fn()} />) }

describe('focused human review', () => {
  beforeEach(() => {
    Object.assign(mocks.api, mockApi())
    mocks.write.mockReset().mockImplementation((fn: () => unknown) => fn())
  })
  it('shows one statement and both evidence relationships with verification and source detail', async () => {
    const user = userEvent.setup()
    mount()
    expect(await screen.findByText(reviewFinding.statement)).toBeInTheDocument()
    expect(screen.getByRole('region', { name: 'Supporting evidence' })).toHaveTextContent(reviewFinding.evidence[0].evidence.title)
    expect(screen.getByRole('region', { name: 'Contradicting evidence' })).toHaveTextContent(reviewFinding.evidence[1].evidence.title)
    await user.click(screen.getByText('Source detail', { selector: 'summary' }))
    expect(screen.getByText('Source detail inspected by the person')).toBeVisible()
    expect(screen.queryByText(/verify all/i)).not.toBeInTheDocument()
  })
  it('verifies only the displayed snapshot and waits for Next', async () => {
    const user = userEvent.setup()
    mount()
    await screen.findByText(reviewFinding.statement)
    await user.click(screen.getByRole('button', { name: 'Verify myself' }))
    expect(mocks.api.reviewStatement).toHaveBeenCalledWith(detail.investigation.ref, {
      kind: 'claim', ref: reviewFinding.ref, basis: reviewFinding.basis, verification: 'human_verified', settle_dispute: false,
    })
    expect(await screen.findByText(`You verified ${reviewFinding.ref}.`)).toBeInTheDocument()
    expect(screen.getByText(reviewFinding.statement)).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Next' }))
    expect(screen.getByText('No more findings in this filter')).toBeInTheDocument()
  })
  it('skips without writing and source checked findings can be checked by a person', async () => {
    const user = userEvent.setup()
    Object.assign(mocks.api, mockApi({ getReviewFindings: async () => [reviewFinding, { ...reviewFinding, ref: 'C-010', verification: 'source_verified', filters: ['source_checked'] }] }))
    mount()
    await screen.findByText(reviewFinding.statement)
    await user.click(screen.getByRole('button', { name: 'Skip' }))
    expect(mocks.api.reviewStatement).not.toHaveBeenCalled()
    await user.selectOptions(screen.getByLabelText('Findings to review'), 'source_checked')
    expect(screen.getByRole('button', { name: 'Verify myself' })).toBeEnabled()
    await user.click(screen.getByRole('button', { name: 'Verify myself' }))
    expect(mocks.api.reviewStatement).toHaveBeenCalledWith(detail.investigation.ref, expect.objectContaining({ ref: 'C-010', verification: 'human_verified' }))
  })
  it('requires an explicit dispute settlement', async () => {
    const user = userEvent.setup()
    Object.assign(mocks.api, mockApi({ getReviewFindings: async () => [{ ...reviewFinding, verification: 'disputed', status: 'disputed', filters: ['disputed'] }] }))
    mount()
    await waitFor(() => expect(screen.queryByText('Loading latest findings...')).not.toBeInTheDocument())
    await user.selectOptions(screen.getByLabelText('Findings to review'), 'disputed')
    await user.click(screen.getByRole('button', { name: 'Verify myself' }))
    expect(mocks.api.reviewStatement).not.toHaveBeenCalled()
    await user.click(screen.getByRole('button', { name: 'Verify and settle dispute' }))
    expect(mocks.api.reviewStatement).toHaveBeenCalledWith(detail.investigation.ref, expect.objectContaining({ settle_dispute: true }))
  })
  it('holds the visible snapshot on external edits until Show latest', async () => {
    const user = userEvent.setup()
    const view = mount()
    await screen.findByText(reviewFinding.statement)
    vi.mocked(mocks.api.getReviewFindings).mockResolvedValue([{ ...reviewFinding, statement: 'An AI changed the finding', basis: 'new-snapshot' }])
    view.rerender(<ReviewFindings detail={{ ...detail }} onClose={vi.fn()} />)
    await screen.findByText('This finding or its evidence changed. Read the latest version before reviewing.')
    expect(screen.getByRole('button', { name: 'Verify myself' })).toBeDisabled()
    expect(screen.getByText(reviewFinding.statement)).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Show latest' }))
    await user.click(screen.getByRole('button', { name: 'Verify myself' }))
    expect(mocks.api.reviewStatement).toHaveBeenCalledWith(detail.investigation.ref, expect.objectContaining({ basis: 'new-snapshot' }))
  })
  it('reports a concurrent edit refusal without advancing or retrying verification', async () => {
    const user = userEvent.setup()
    vi.mocked(mocks.api.reviewStatement).mockRejectedValue(new ApiError('conflict', 'This finding changed; show latest.'))
    mount()
    await screen.findByText(reviewFinding.statement)
    await user.click(screen.getByRole('button', { name: 'Verify myself' }))
    expect(await screen.findByText('This finding changed; show latest.')).toBeInTheDocument()
    expect(screen.getByText(reviewFinding.statement)).toBeInTheDocument()
    expect(mocks.write).toHaveBeenCalledWith(expect.any(Function), { rethrowConflict: true, retry: false })
  })
  it('keeps a failed first read visible with reload', async () => {
    Object.assign(mocks.api, mockApi({ getReviewFindings: async () => { throw new Error('The case could not be loaded') } }))
    mount()
    expect(await screen.findByText('The case could not be loaded')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Reload findings' })).toBeInTheDocument()
    expect(screen.queryByText('No more findings in this filter')).not.toBeInTheDocument()
    expect(within(screen.getByRole('region', { name: 'Review findings' })).queryByRole('button', { name: 'Verify myself' })).not.toBeInTheDocument()
  })
  it('names each filter after the label it selects and explains Support needs attention', async () => {
    const user = userEvent.setup()
    mount()
    await screen.findByText(reviewFinding.statement)
    const filter = screen.getByLabelText('Findings to review')
    expect(within(filter).getAllByRole('option').map((o) => o.textContent)).toEqual([
      'AI generated, not yet checked by you',
      'Source verified, not yet checked by you',
      'Disputed',
      'Support needs attention',
      'All current findings',
    ])
    const gloss = 'Findings with no supporting evidence, or whose supporting evidence is disputed or superseded.'
    expect(screen.queryByText(gloss)).not.toBeInTheDocument()
    await user.selectOptions(filter, 'support_attention')
    expect(screen.getByText(gloss)).toBeInTheDocument()
    expect(filter).toHaveAccessibleDescription(gloss)
    await user.selectOptions(filter, 'all')
    expect(screen.queryByText(gloss)).not.toBeInTheDocument()
  })
  it('says nothing is waiting on a case with no findings at all', async () => {
    Object.assign(mocks.api, mockApi({ getReviewFindings: async () => [] }))
    mount()
    expect(await screen.findByRole('heading', { name: 'Nothing to review yet' })).toBeInTheDocument()
    expect(screen.getByText('Findings your AI saves, and claims you add, appear here for you to check against their evidence.')).toBeInTheDocument()
    expect(screen.queryByText('No more findings in this filter')).not.toBeInTheDocument()
  })
})
