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
      <AppDataProvider>
        <CopyContextProvider>
          <DetailView />
        </CopyContextProvider>
      </AppDataProvider>
    </ToastProvider>,
  )
}

async function openEvidence(ref: string) {
  const user = userEvent.setup()
  await user.click(await screen.findByRole('button', { name: new RegExp(`^Open evidence ${ref}:`) }))
  return { user, drawer: screen.getByRole('dialog', { name: `Evidence ${ref}` }) }
}

/** The definition list rows of the Source group as [label, value] pairs. */
function sourceRows(drawer: HTMLElement): [string, string][] {
  const source = within(drawer).getByRole('region', { name: 'Source' })
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
    expect(within(drawer).getByText('Log ingestion is 71% of the September increase')).toHaveClass('evidence-drawer-title')
    expect(within(drawer).getByText(/In the billing export, log ingestion rose/)).toBeInTheDocument()
    const rows = sourceRows(drawer)
    expect(rows.map(([label]) => label)).toEqual(['Source type', 'Source', 'Captured'])
    expect(rows[0][1]).toBe('SQL query')
    expect(rows[1][1]).toBe('Billing export query')
  })

  it('shows the reference as selectable text, never a link', async () => {
    renderDetail()
    const { drawer } = await openEvidence('E-001')
    const rows = sourceRows(drawer)
    expect(rows.find(([l]) => l === 'Reference')?.[1]).toBe('platform-charts/service/values.yaml')
    expect(drawer.querySelector('a')).toBeNull()
  })

  it('omits the source rows, raw detail and metadata controls a bare record does not have', async () => {
    renderDetail(detailWithBareEvidence())
    const { drawer } = await openEvidence('E-009')
    expect(sourceRows(drawer).map(([l]) => l)).toEqual(['Source type'])
    expect(within(drawer).queryByRole('button', { name: 'Show raw detail' })).toBeNull()
    expect(within(drawer).queryByRole('button', { name: 'Show metadata' })).toBeNull()
    expect(within(drawer).getByText('No claim or hypothesis uses this evidence yet.')).toBeInTheDocument()
  })

  it('fetches raw detail on demand with include_raw_detail and hides it again', async () => {
    renderDetail()
    const { user, drawer } = await openEvidence('E-003')
    expect(mocks.api.getEvidence).not.toHaveBeenCalled()
    await user.click(within(drawer).getByRole('button', { name: 'Show raw detail' }))
    expect(mocks.api.getEvidence).toHaveBeenCalledWith('INV-0001', 'E-003', { include_raw_detail: true })
    const pre = await within(drawer).findByText(/SELECT line_item/)
    expect(pre.tagName).toBe('PRE')
    expect(pre).toHaveClass('raw-detail')
    expect(pre.textContent).toBe(E003_RAW)
    await user.click(within(drawer).getByRole('button', { name: 'Hide raw detail' }))
    expect(within(drawer).queryByText(/SELECT line_item/)).toBeNull()
    expect(within(drawer).getByRole('button', { name: 'Show raw detail' })).toBeInTheDocument()
  })

  it('fetches raw detail once when the record changed after the detail loaded (no refetch loop)', async () => {
    renderDetail(detail, {
      getEvidence: async (_inv, ref) => ({ ...evidenceWith(ref), updated_at: '2099-01-01T00:00:00.000Z', raw_detail: 'newer' }),
    })
    const { user, drawer } = await openEvidence('E-003')
    await user.click(within(drawer).getByRole('button', { name: 'Show raw detail' }))
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
    await user.click(within(drawer).getByRole('button', { name: 'Show raw detail' }))
    const pre = await within(drawer).findByText(/row 0000: value=/)
    expect(pre).toHaveClass('raw-detail')
    expect(pre.textContent?.length).toBe(65536)
  })

  it('shows no loading line before 300 ms while raw detail loads', async () => {
    let resolve: (e: Evidence) => void = () => {}
    renderDetail(detail, { getEvidence: () => new Promise<Evidence>((r) => (resolve = r)) })
    const { drawer } = await openEvidence('E-003')
    vi.useFakeTimers()
    fireEvent.click(within(drawer).getByRole('button', { name: 'Show raw detail' }))
    await act(async () => {
      await vi.advanceTimersByTimeAsync(299)
    })
    expect(within(drawer).queryByText('Loading raw detail...')).toBeNull()
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1)
    })
    expect(within(drawer).getByText('Loading raw detail...')).toBeInTheDocument()
    await act(async () => {
      resolve({ ...evidenceWith('E-003'), raw_detail: E003_RAW })
      await vi.advanceTimersByTimeAsync(0)
    })
    expect(within(drawer).queryByText('Loading raw detail...')).toBeNull()
    expect(within(drawer).getByText(/SELECT line_item/)).toBeInTheDocument()
  })

  it('pretty-prints metadata behind Show metadata', async () => {
    renderDetail()
    const { user, drawer } = await openEvidence('E-003')
    expect(within(drawer).queryByText(/affected_records/)).toBeNull()
    await user.click(within(drawer).getByRole('button', { name: 'Show metadata' }))
    const pre = within(drawer).getByText(/affected_records/)
    expect(pre.tagName).toBe('PRE')
    expect(pre.textContent).toBe('{\n  "affected_records": 214\n}')
  })

  it('shows metadata that is not valid JSON as it is', async () => {
    const d = detailWith({ evidence: evidence.map((e) => (e.ref === 'E-003' ? { ...e, metadata: 'not json {' } : e)) })
    renderDetail(d)
    const { user, drawer } = await openEvidence('E-003')
    await user.click(within(drawer).getByRole('button', { name: 'Show metadata' }))
    expect(within(drawer).getByText('not json {').tagName).toBe('PRE')
  })

  it('lists the claims and hypotheses that use it, each opening its drawer', async () => {
    renderDetail()
    const { user, drawer } = await openEvidence('E-003')
    const usedBy = within(drawer).getByRole('region', { name: 'Used by' })
    const items = within(usedBy).getAllByRole('button')
    expect(items.map((b) => b.textContent)).toEqual([
      'C-002SupportsLog ingestion, billed per GB, is $96,400 of the $135,800 increase; compute and network are flat.',
      'C-003ContradictsSuperseded by C-002The increase comes from the August autoscaling change adding compute nodes.',
      "C-004ContradictsThe payments team's own debug logs are most of the $96,400.",
      'H-002ContradictsDisprovenData transfer from the region move doubled the network line.',
    ])
    expect(items[1].querySelector('.used-by-statement')).toHaveClass('is-retired')
    expect(items[0].querySelector('.used-by-statement')).not.toHaveClass('is-retired')
    await user.click(items[0])
    expect(screen.getByRole('dialog', { name: 'Claim C-002' })).toBeInTheDocument()
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
    await user.click(within(drawer).getByRole('button', { name: 'Verify' }))
    expect(mocks.api.setEvidenceVerification).toHaveBeenCalledWith('INV-0001', 'E-003', 'human_verified')
    await user.click(within(drawer).getByRole('button', { name: 'Dispute' }))
    expect(mocks.api.setEvidenceVerification).toHaveBeenCalledWith('INV-0001', 'E-003', 'disputed')
  })

  it('shows Verified disabled when the evidence is already verified', async () => {
    renderDetail()
    const { drawer } = await openEvidence('E-004')
    expect(within(drawer).getByRole('button', { name: 'Verified' })).toBeDisabled()
  })

  it('offers Mark superseded (after a confirmation, AR-27) and Mark source verified in the More menu', async () => {
    renderDetail()
    const { user, drawer } = await openEvidence('E-003')
    await user.click(within(drawer).getByRole('button', { name: 'More actions' }))
    await user.click(screen.getByRole('menuitem', { name: 'Mark superseded' }))
    let confirm = screen.getByRole('dialog', { name: 'Mark E-003 superseded?' })
    expect(confirm).toHaveTextContent(supersedeEvidenceBody('E-003'))
    await user.click(within(confirm).getByRole('button', { name: 'Cancel' }))
    expect(mocks.api.setEvidenceVerification).not.toHaveBeenCalled()
    await user.click(within(drawer).getByRole('button', { name: 'More actions' }))
    await user.click(screen.getByRole('menuitem', { name: 'Mark superseded' }))
    confirm = screen.getByRole('dialog', { name: 'Mark E-003 superseded?' })
    await user.click(within(confirm).getByRole('button', { name: 'Mark superseded' }))
    expect(mocks.api.setEvidenceVerification).toHaveBeenCalledWith('INV-0001', 'E-003', 'superseded')
    await user.click(within(drawer).getByRole('button', { name: 'More actions' }))
    await user.click(screen.getByRole('menuitem', { name: 'Mark source verified' }))
    expect(mocks.api.setEvidenceVerification).toHaveBeenCalledWith('INV-0001', 'E-003', 'source_verified')
  })

  it('Edit evidence opens the edit dialog over the drawer', async () => {
    renderDetail()
    const { user, drawer } = await openEvidence('E-003')
    await user.click(within(drawer).getByRole('button', { name: 'Edit evidence' }))
    expect(screen.getByRole('dialog', { name: 'Edit evidence E-003' })).toBeInTheDocument()
    expect(mocks.api.getEvidence).toHaveBeenCalledWith('INV-0001', 'E-003', { include_raw_detail: true })
  })
})
