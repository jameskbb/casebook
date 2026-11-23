import { act, fireEvent, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { Api } from '../../api/api'
import type { Evidence, InvestigationDetail } from '../../api/types'
import { AppDataProvider } from '../../state/appData'
import { CopyContextProvider } from '../../state/copyContext'
import { formatAbsolute } from '../../lib/relativeTime'
import { detail, evidence, summaries, T0 } from '../../test/fixtures'
import { bigRawDetail, detailWith, detailWithBareEvidence, E003_RAW, evidenceWith } from '../../test/fixtures.records'
import { mockApi } from '../../test/mockApi'
import { OpenAtLaunch } from '../../test/OpenAtLaunch'
import { findSourceButton } from '../../test/openQuestion'
import { DetailView } from '../detail/DetailView'
import { REPLACED_SOURCE_LABEL, supersedeEvidenceBody } from './EvidenceDrawer'
import { ToastProvider } from '../ui/Toaster'

const mocks = vi.hoisted(() => ({ api: {} as Api }))
vi.mock('../../api/api', () => ({ api: mocks.api }))

function renderDetail(d: InvestigationDetail = detail, overrides: Partial<Api> = {}) {
  Object.assign(
    mocks.api,
    mockApi({
      listInvestigations: async () => [summaries[0]],
      getInvestigationDetail: async () => d,
      getEvidence: async (_inv, ref, o) => {
        const e = d.evidence.find((x) => x.ref === ref) ?? evidence[0]
        return o?.include_raw_detail && e.has_raw_detail ? { ...e, raw_detail: ref === 'E-003' ? E003_RAW : 'raw' } : e
      },
      ...overrides,
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

async function openEvidence(ref: string) {
  const user = userEvent.setup()
  await user.click(await findSourceButton(ref))
  return { user, drawer: screen.getByRole('dialog', { name: `Source ${ref}` }) }
}

/** The definition list rows of About this source as [label, value] pairs. */
function sourceRows(drawer: HTMLElement): [string, string][] {
  const source = within(drawer).getByRole('region', { name: 'About this source' })
  return Array.from(source.querySelectorAll('dt')).map((dt) => [
    dt.textContent ?? '',
    dt.nextElementSibling?.textContent ?? '',
  ])
}

describe('EvidenceDrawer', () => {
  beforeEach(() => {
    Object.assign(mocks.api, mockApi())
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('shows the title, full summary and the source details, omitting missing ones', async () => {
    renderDetail()
    const { drawer } = await openEvidence('E-003')
    expect(within(drawer).getByText('Calls about a bill: 400 in September, 1,070 in October')).toHaveClass('evidence-drawer-title')
    expect(within(drawer).getByText(/In the call log, calls about a bill went from 400/)).toBeInTheDocument()
    const rows = sourceRows(drawer)
    expect(rows.map(([label]) => label)).toEqual(['Kind of source', 'From', 'Dated'])
    expect(rows[0][1]).toBe('Numbers your AI looked up')
    expect(rows[1][1]).toBe('Call log')
    expect(rows[2][1]).toBe(formatAbsolute(T0))
  })

  it('shows the reference as selectable text, never a link', async () => {
    renderDetail()
    const { drawer } = await openEvidence('E-001')
    const rows = sourceRows(drawer)
    expect(rows.find(([l]) => l === 'Where to find it')?.[1]).toBe('Shared drive, Billing, notice-2026-10-04.pdf')
    expect(drawer.querySelector('a')).toBeNull()
  })

  it('omits the rows, exact words and extra details a bare source does not have', async () => {
    renderDetail(detailWithBareEvidence())
    const { drawer } = await openEvidence('E-009')
    expect(sourceRows(drawer).map(([l]) => l)).toEqual(['Kind of source'])
    expect(within(drawer).queryByRole('button', { name: 'Show the exact words' })).toBeNull()
    expect(within(drawer).queryByRole('region', { name: 'Extra details' })).toBeNull()
    expect(drawer.querySelector('.codes-only')).toBeNull()
    expect(within(drawer).getByText('No finding uses this source yet.')).toBeInTheDocument()
  })

  it('Show the exact words loads and shows the text, and hides it again', async () => {
    renderDetail()
    const { user, drawer } = await openEvidence('E-003')
    expect(mocks.api.getEvidence).not.toHaveBeenCalled()
    await user.click(within(drawer).getByRole('button', { name: 'Show the exact words' }))
    expect(mocks.api.getEvidence).toHaveBeenCalledWith('INV-0001', 'E-003', { include_raw_detail: true })
    const pre = await within(drawer).findByText(/Reason for the call/)
    expect(pre.tagName).toBe('PRE')
    expect(pre).toHaveClass('raw-detail')
    expect(pre.textContent).toBe(E003_RAW)
    await user.click(within(drawer).getByRole('button', { name: 'Hide the exact words' }))
    expect(within(drawer).queryByText(/Reason for the call/)).toBeNull()
    expect(within(drawer).getByRole('button', { name: 'Show the exact words' })).toBeInTheDocument()
  })

  it('fetches raw detail once when the record changed after the detail loaded (no refetch loop)', async () => {
    renderDetail(detail, {
      getEvidence: async (_inv, ref) => ({ ...evidenceWith(ref), updated_at: '2099-01-01T00:00:00.000Z', raw_detail: 'newer' }),
    })
    const { user, drawer } = await openEvidence('E-003')
    await user.click(within(drawer).getByRole('button', { name: 'Show the exact words' }))
    expect(await within(drawer).findByText('newer')).toHaveClass('raw-detail')
    await act(async () => {
      await new Promise((r) => setTimeout(r, 50))
    })
    expect(mocks.api.getEvidence).toHaveBeenCalledTimes(1)
  })

  it('renders a 64 KiB raw detail in full inside the capped pre block (backstop)', async () => {
    const raw = bigRawDetail()
    expect(raw.length).toBe(65536)
    renderDetail(detail, {
      getEvidence: async (_inv, ref) => ({ ...evidenceWith(ref), raw_detail: raw }),
    })
    const { user, drawer } = await openEvidence('E-001')
    await user.click(within(drawer).getByRole('button', { name: 'Show the exact words' }))
    const pre = await within(drawer).findByText(/row 0000: value=/)
    expect(pre).toHaveClass('raw-detail')
    expect(pre.textContent?.length).toBe(65536)
  })

  it('shows no loading line before 300 ms while raw detail loads', async () => {
    let resolve: (e: Evidence) => void = () => {}
    renderDetail(detail, { getEvidence: () => new Promise<Evidence>((r) => (resolve = r)) })
    const { drawer } = await openEvidence('E-003')
    vi.useFakeTimers()
    fireEvent.click(within(drawer).getByRole('button', { name: 'Show the exact words' }))
    await act(async () => {
      await vi.advanceTimersByTimeAsync(299)
    })
    expect(within(drawer).queryByText('Loading the exact words...')).toBeNull()
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1)
    })
    expect(within(drawer).getByText('Loading the exact words...')).toBeInTheDocument()
    await act(async () => {
      resolve({ ...evidenceWith('E-003'), raw_detail: E003_RAW })
      await vi.advanceTimersByTimeAsync(0)
    })
    expect(within(drawer).queryByText('Loading the exact words...')).toBeNull()
    expect(within(drawer).getByText(/Reason for the call/)).toBeInTheDocument()
  })

  it('shows stored extra details only inside the group that shows with codes on', async () => {
    renderDetail()
    const { drawer } = await openEvidence('E-003')
    const pre = within(drawer).getByText(/calls_counted/)
    expect(pre.tagName).toBe('PRE')
    expect(pre.textContent).toBe('{\n  "calls_counted": 5520\n}')
    expect(pre.closest('.codes-only')).toBe(within(drawer).getByRole('region', { name: 'Extra details' }))
    expect(within(drawer).queryByRole('button', { name: 'Show extra details' })).toBeNull()
  })

  it('shows extra details that are not valid JSON as they are', async () => {
    const d = detailWith({ evidence: evidence.map((e) => (e.ref === 'E-003' ? { ...e, metadata: 'not json {' } : e)) })
    renderDetail(d)
    const { drawer } = await openEvidence('E-003')
    const pre = within(drawer).getByText('not json {')
    expect(pre.tagName).toBe('PRE')
    expect(pre.closest('.codes-only')).not.toBeNull()
  })

  it('lists the claims and hypotheses that use it, each opening its drawer', async () => {
    renderDetail()
    const { user, drawer } = await openEvidence('E-003')
    const usedBy = within(drawer).getByRole('region', { name: 'Findings that use this source' })
    const items = within(usedBy).getAllByRole('button')
    expect(items.map((b) => b.textContent)).toEqual([
      'C-002Backs it upThe extra calls were about a bill. About the same number of people called to buy.',
      'C-003Goes against itReplaced by C-002The extra calls were people calling to buy.',
      'C-004Goes against itCallers waited too long and hung up before buying.',
      'H-002Goes against itRuled outA new ad brought in more callers.',
    ])
    expect(items[1].querySelector('.used-by-statement')).toHaveClass('is-retired')
    expect(items[0].querySelector('.used-by-statement')).not.toHaveClass('is-retired')
    await user.click(items[0])
    expect(screen.getByRole('dialog', { name: 'Finding C-002' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Back to E-003' })).toBeInTheDocument()
  })
})

describe('EvidenceDrawer actions', () => {
  beforeEach(() => {
    Object.assign(mocks.api, mockApi())
  })

  it('I checked this sets human_verified and Sources disagree sets disputed', async () => {
    renderDetail()
    const { user, drawer } = await openEvidence('E-003')
    await user.click(within(drawer).getByRole('button', { name: 'I checked this' }))
    expect(mocks.api.setEvidenceVerification).toHaveBeenCalledWith('INV-0001', 'E-003', 'human_verified')
    await user.click(within(drawer).getByRole('button', { name: 'Sources disagree' }))
    expect(mocks.api.setEvidenceVerification).toHaveBeenCalledWith('INV-0001', 'E-003', 'disputed')
  })

  it('a source the AI checked can be checked by the person', async () => {
    const d = detailWith({ evidence: evidence.map((e) => (e.ref === 'E-003' ? { ...e, verification: 'source_verified' as const } : e)) })
    renderDetail(d)
    const { user, drawer } = await openEvidence('E-003')
    const check = within(drawer).getByRole('button', { name: 'I checked this' })
    expect(check).toBeEnabled()
    await user.click(check)
    expect(mocks.api.setEvidenceVerification).toHaveBeenCalledWith('INV-0001', 'E-003', 'human_verified')
  })

  it('a source the person checked has no I checked this button', async () => {
    renderDetail()
    const { drawer } = await openEvidence('E-004')
    expect(within(drawer).queryByRole('button', { name: 'I checked this' })).toBeNull()
    expect(within(drawer).queryByRole('button', { name: 'You checked this' })).toBeNull()
    expect(within(drawer).getByText('You checked this')).toBeInTheDocument()
  })

  it('the More menu holds only A newer source replaces this (after a confirmation) and Delete source...', async () => {
    renderDetail()
    const { user, drawer } = await openEvidence('E-003')
    await user.click(within(drawer).getByRole('button', { name: 'More' }))
    expect(screen.getAllByRole('menuitem').map((m) => m.textContent)).toEqual([REPLACED_SOURCE_LABEL, 'Delete source...'])
    expect(screen.queryByRole('menuitem', { name: 'Mark as checked by AI' })).toBeNull()
    await user.click(screen.getByRole('menuitem', { name: REPLACED_SOURCE_LABEL }))
    let confirm = screen.getByRole('dialog', { name: 'Did a newer source replace this one?' })
    expect(confirm).toHaveTextContent(supersedeEvidenceBody)
    expect(confirm).not.toHaveTextContent('E-003')
    await user.click(within(confirm).getByRole('button', { name: 'Cancel' }))
    expect(mocks.api.setEvidenceVerification).not.toHaveBeenCalled()
    await user.click(within(drawer).getByRole('button', { name: 'More' }))
    await user.click(screen.getByRole('menuitem', { name: REPLACED_SOURCE_LABEL }))
    confirm = screen.getByRole('dialog', { name: 'Did a newer source replace this one?' })
    await user.click(within(confirm).getByRole('button', { name: 'Yes, it was replaced' }))
    expect(mocks.api.setEvidenceVerification).toHaveBeenCalledWith('INV-0001', 'E-003', 'superseded')
    expect(mocks.api.setEvidenceVerification).toHaveBeenCalledTimes(1)
  })

  it('Delete source... opens the delete confirmation', async () => {
    renderDetail(detailWithBareEvidence())
    const { user, drawer } = await openEvidence('E-009')
    await user.click(within(drawer).getByRole('button', { name: 'More' }))
    await user.click(screen.getByRole('menuitem', { name: 'Delete source...' }))
    expect(screen.getByRole('dialog', { name: 'Delete this source?' })).toBeInTheDocument()
  })

  it('Edit opens the edit dialog over the drawer', async () => {
    renderDetail()
    const { user, drawer } = await openEvidence('E-003')
    await user.click(within(drawer).getByRole('button', { name: 'Edit' }))
    expect(screen.getByRole('dialog', { name: 'Edit source' })).toBeInTheDocument()
    expect(mocks.api.getEvidence).toHaveBeenCalledWith('INV-0001', 'E-003', { include_raw_detail: true })
  })
})
