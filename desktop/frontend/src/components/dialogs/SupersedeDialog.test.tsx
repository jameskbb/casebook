import type { ReactNode } from 'react'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Api } from '../../api/api'
import type { Claim, Hypothesis } from '../../api/types'
import { AppDataProvider } from '../../state/appData'
import { claims, hypotheses } from '../../test/fixtures'
import { mockApi } from '../../test/mockApi'
import { ToastProvider } from '../ui/Toaster'
import { SupersedeDialog } from './SupersedeDialog'

const mocks = vi.hoisted(() => ({ api: {} as Api }))
vi.mock('../../api/api', () => ({ api: mocks.api }))

const providers = (ui: ReactNode) => (
  <ToastProvider>
    <AppDataProvider>{ui}</AppDataProvider>
  </ToastProvider>
)

function renderClaim(record: Claim, records: Claim[] = claims) {
  const onClose = vi.fn()
  render(providers(<SupersedeDialog kind="claim" record={record} records={records} invRef="INV-0001" onClose={onClose} />))
  return { onClose }
}

function renderHypothesis(record: Hypothesis, records: Hypothesis[] = hypotheses) {
  const onClose = vi.fn()
  render(
    providers(<SupersedeDialog kind="hypothesis" record={record} records={records} invRef="INV-0001" onClose={onClose} />),
  )
  return { onClose }
}

describe('SupersedeDialog', () => {
  beforeEach(() => {
    Object.assign(mocks.api, mockApi())
  })

  it('explains the change and lists the other claims that are not superseded', () => {
    renderClaim(claims[3])
    expect(screen.getByRole('dialog', { name: 'Supersede C-004' })).toBeInTheDocument()
    expect(
      screen.getByText('Choose the claim that replaces it. C-004 stays in the history, marked as superseded.'),
    ).toBeInTheDocument()
    const options = screen.getAllByRole('radio').map((r) => r.closest('label')?.textContent)
    expect(options).toEqual([`C-001${claims[0].statement}`, `C-002${claims[1].statement}`])
  })

  it('supersedes with the chosen claim', async () => {
    const user = userEvent.setup()
    const { onClose } = renderClaim(claims[3])
    await user.click(screen.getByRole('radio', { name: /C-002/ }))
    await user.click(screen.getByRole('button', { name: 'Supersede claim' }))
    expect(mocks.api.supersedeClaim).toHaveBeenCalledWith('INV-0001', 'C-004', 'C-002')
    await waitFor(() => expect(onClose).toHaveBeenCalled())
  })

  it('asks for a replacement before superseding', async () => {
    const user = userEvent.setup()
    renderClaim(claims[3])
    await user.click(screen.getByRole('button', { name: 'Supersede claim' }))
    expect(screen.getByText('Choose the claim that replaces it.')).toBeInTheDocument()
    expect(mocks.api.supersedeClaim).not.toHaveBeenCalled()
  })

  it('shows Superseding... while the call runs', async () => {
    const user = userEvent.setup()
    Object.assign(mocks.api, mockApi({ supersedeClaim: () => new Promise<Claim>(() => {}) }))
    renderClaim(claims[3])
    await user.click(screen.getByRole('radio', { name: /C-001/ }))
    await user.click(screen.getByRole('button', { name: 'Supersede claim' }))
    expect(screen.getByRole('button', { name: 'Superseding...' })).toBeDisabled()
  })

  it('says there is no other claim when none can replace it', async () => {
    const user = userEvent.setup()
    const { onClose } = renderClaim(claims[0], [claims[0], claims[2]])
    expect(
      screen.getByText('There is no other claim to replace it with. Add the new claim first, then supersede this one.'),
    ).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Supersede claim' })).toBeNull()
    await user.click(screen.getAllByRole('button', { name: 'Close' }).at(-1) as HTMLElement)
    expect(onClose).toHaveBeenCalled()
  })

  it('uses the hypothesis copy and call', async () => {
    const user = userEvent.setup()
    renderHypothesis(hypotheses[0])
    expect(screen.getByRole('dialog', { name: 'Supersede H-001' })).toBeInTheDocument()
    expect(
      screen.getByText('Choose the hypothesis that replaces it. H-001 stays in the history, marked as superseded.'),
    ).toBeInTheDocument()
    await user.click(screen.getByRole('radio', { name: /H-003/ }))
    await user.click(screen.getByRole('button', { name: 'Supersede hypothesis' }))
    expect(mocks.api.supersedeHypothesis).toHaveBeenCalledWith('INV-0001', 'H-001', 'H-003')
  })

  it('says there is no other hypothesis when none can replace it', () => {
    renderHypothesis(hypotheses[0], [hypotheses[0]])
    expect(
      screen.getByText(
        'There is no other hypothesis to replace it with. Add the new hypothesis first, then supersede this one.',
      ),
    ).toBeInTheDocument()
  })
})
