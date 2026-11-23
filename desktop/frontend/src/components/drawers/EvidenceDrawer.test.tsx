import { act, fireEvent, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { Api } from '../../api/api'
import type { Evidence, InvestigationDetail } from '../../api/types'
import { AppDataProvider } from '../../state/appData'
import { CopyContextProvider } from '../../state/copyContext'
import { detail, evidence, summaries } from '../../test/fixtures'
import { bigRawDetail, detailWith, detailWithBareEvidence, E003_RAW, evidenceWith } from '../../test/fixtures.records'
import { mockApi } from '../../test/mockApi'
import { OpenAtLaunch } from '../../test/OpenAtLaunch'
import { findSourceButton } from '../../test/openQuestion'
import { DetailView } from '../detail/DetailView'
import { supersedeEvidenceBody } from './EvidenceDrawer'
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

/** The definition list rows of the Source group as [label, value] pairs. */
function sourceRows(drawer: HTMLElement): [string, string][] {
  const source = within(drawer).getByRole('region', { name: 'Where it came from' })
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
    expect(rows.map(([label]) => label)).toEqual(['Kind of source', 'From', 'As of'])
    expect(rows[0][1]).toBe('Data query')
    expect(rows[1][1]).toBe('Call log')
  })

  it('shows the reference as selectable text, never a link', async () => {
    renderDetail()
    const { drawer } = await openEvidence('E-001')
    const rows = sourceRows(drawer)
    expect(rows.find(([l]) => l === 'Where to find it')?.[1]).toBe('Shared drive, Billing, notice-2026-10-04.pdf')
    expect(drawer.querySelector('a')).toBeNull()
  })

  it('omits the source rows, raw detail and metadata controls a bare record does not have', async () => {
    renderDetail(detailWithBareEvidence())
    const { drawer } = await openEvidence('E-009')
    expect(sourceRows(drawer).map(([l]) => l)).toEqual(['Kind of source'])
    expect(within(drawer).queryByRole('button', { name: 'Show source detail' })).toBeNull()
    expect(within(drawer).queryByRole('button', { name: 'Show extra details' })).toBeNull()
    expect(within(drawer).getByText('No finding uses this source yet.')).toBeInTheDocument()
  })

  it('fetches raw detail on demand with include_raw_detail and hides it again', async () => {
    renderDetail()
    const { user, drawer } = await openEvidence('E-003')
    expect(mocks.api.getEvidence).not.toHaveBeenCalled()
    await user.click(within(drawer).getByRole('button', { name: 'Show source detail' }))
    expect(mocks.api.getEvidence).toHaveBeenCalledWith('INV-0001', 'E-003', { include_raw_detail: true })
    const pre = await within(drawer).findByText(/Reason for the call/)
    expect(pre.tagName).toBe('PRE')
    expect(pre).toHaveClass('raw-detail')
    expect(pre.textContent).toBe(E003_RAW)
    await user.click(within(drawer).getByRole('button', { name: 'Hide source detail' }))
    expect(within(drawer).queryByText(/Reason for the call/)).toBeNull()
    expect(within(drawer).getByRole('button', { name: 'Show source detail' })).toBeInTheDocument()
  })

  it('fetches raw detail once when the record changed after the detail loaded (no refetch loop)', async () => {
    renderDetail(detail, {
      getEvidence: async (_inv, ref) => ({ ...evidenceWith(ref), updated_at: '2099-01-01T00:00:00.000Z', raw_detail: 'newer' }),
    })
    const { user, drawer } = await openEvidence('E-003')
    await user.click(within(drawer).getByRole('button', { name: 'Show source detail' }))
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
    await user.click(within(drawer).getByRole('button', { name: 'Show source detail' }))
    const pre = await within(drawer).findByText(/row 0000: value=/)
    expect(pre).toHaveClass('raw-detail')
    expect(pre.textContent?.length).toBe(65536)
  })

  it('shows no loading line before 300 ms while raw detail loads', async () => {
    let resolve: (e: Evidence) => void = () => {}
    renderDetail(detail, { getEvidence: () => new Promise<Evidence>((r) => (resolve = r)) })
    const { drawer } = await openEvidence('E-003')
    vi.useFakeTimers()
    fireEvent.click(within(drawer).getByRole('button', { name: 'Show source detail' }))
    await act(async () => {
      await vi.advanceTimersByTimeAsync(299)
    })
    expect(within(drawer).queryByText('Loading source detail...')).toBeNull()
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1)
    })
    expect(within(drawer).getByText('Loading source detail...')).toBeInTheDocument()
    await act(async () => {
      resolve({ ...evidenceWith('E-003'), raw_detail: E003_RAW })
      await vi.advanceTimersByTimeAsync(0)
    })
    expect(within(drawer).queryByText('Loading source detail...')).toBeNull()
    expect(within(drawer).getByText(/Reason for the call/)).toBeInTheDocument()
  })

  it('pretty-prints metadata behind Show metadata', async () => {
    renderDetail()
    const { user, drawer } = await openEvidence('E-003')
    expect(within(drawer).queryByText(/calls_counted/)).toBeNull()
    await user.click(within(drawer).getByRole('button', { name: 'Show extra details' }))
    const pre = within(drawer).getByText(/calls_counted/)
    expect(pre.tagName).toBe('PRE')
    expect(pre.textContent).toBe('{\n  "calls_counted": 5520\n}')
  })

  it('shows metadata that is not valid JSON as it is', async () => {
    const d = detailWith({ evidence: evidence.map((e) => (e.ref === 'E-003' ? { ...e, metadata: 'not json {' } : e)) })
    renderDetail(d)
    const { user, drawer } = await openEvidence('E-003')
    await user.click(within(drawer).getByRole('button', { name: 'Show extra details' }))
    expect(within(drawer).getByText('not json {').tagName).toBe('PRE')
  })

  it('lists the claims and hypotheses that use it, each opening its drawer', async () => {
    renderDetail()
    const { user, drawer } = await openEvidence('E-003')
    const usedBy = within(drawer).getByRole('region', { name: 'Findings that use this source' })
    const items = within(usedBy).getAllByRole('button')
    expect(items.map((b) => b.textContent)).toEqual([
      'C-002SupportsThe extra calls were about a bill. About the same number of people called to buy.',
      'C-003ContradictsReplaced by C-002The extra calls were people calling to buy.',
      'C-004ContradictsCallers waited too long and hung up before buying.',
      'H-002ContradictsRuled outA new ad brought in more callers.',
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

  it('Verify sets human_verified and Dispute sets disputed', async () => {
    renderDetail()
    const { user, drawer } = await openEvidence('E-003')
    await user.click(within(drawer).getByRole('button', { name: 'I checked this' }))
    expect(mocks.api.setEvidenceVerification).toHaveBeenCalledWith('INV-0001', 'E-003', 'human_verified')
    await user.click(within(drawer).getByRole('button', { name: 'Sources disagree' }))
    expect(mocks.api.setEvidenceVerification).toHaveBeenCalledWith('INV-0001', 'E-003', 'disputed')
  })

  it('shows Verified disabled when the evidence is already verified', async () => {
    renderDetail()
    const { drawer } = await openEvidence('E-004')
    expect(within(drawer).getByRole('button', { name: 'You checked this' })).toBeDisabled()
  })

  it('offers Mark replaced (after a confirmation) and Mark as checked by AI in the More menu', async () => {
    renderDetail()
    const { user, drawer } = await openEvidence('E-003')
    await user.click(within(drawer).getByRole('button', { name: 'More actions' }))
    await user.click(screen.getByRole('menuitem', { name: 'Mark replaced' }))
    let confirm = screen.getByRole('dialog', { name: 'Mark this source as replaced?' })
    expect(confirm).toHaveTextContent(supersedeEvidenceBody)
    expect(confirm).not.toHaveTextContent('E-003')
    await user.click(within(confirm).getByRole('button', { name: 'Cancel' }))
    expect(mocks.api.setEvidenceVerification).not.toHaveBeenCalled()
    await user.click(within(drawer).getByRole('button', { name: 'More actions' }))
    await user.click(screen.getByRole('menuitem', { name: 'Mark replaced' }))
    confirm = screen.getByRole('dialog', { name: 'Mark this source as replaced?' })
    await user.click(within(confirm).getByRole('button', { name: 'Mark replaced' }))
    expect(mocks.api.setEvidenceVerification).toHaveBeenCalledWith('INV-0001', 'E-003', 'superseded')
    await user.click(within(drawer).getByRole('button', { name: 'More actions' }))
    await user.click(screen.getByRole('menuitem', { name: 'Mark as checked by AI' }))
    expect(mocks.api.setEvidenceVerification).toHaveBeenCalledWith('INV-0001', 'E-003', 'source_verified')
  })

  it('Edit source opens the edit dialog over the drawer', async () => {
    renderDetail()
    const { user, drawer } = await openEvidence('E-003')
    await user.click(within(drawer).getByRole('button', { name: 'Edit source' }))
    expect(screen.getByRole('dialog', { name: 'Edit source' })).toBeInTheDocument()
    expect(mocks.api.getEvidence).toHaveBeenCalledWith('INV-0001', 'E-003', { include_raw_detail: true })
  })
})
