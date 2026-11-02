import type { ReactNode } from 'react'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Api } from '../../api/api'
import { ApiError } from '../../api/errors'
import type { Claim, Evidence } from '../../api/types'
import { AppDataProvider } from '../../state/appData'
import { evidence, hypotheses } from '../../test/fixtures'
import { newClaim } from '../../test/fixtures.records'
import { mockApi } from '../../test/mockApi'
import { ToastProvider } from '../ui/Toaster'
import { StatementDialog } from './StatementDialog'

const mocks = vi.hoisted(() => ({ api: {} as Api }))
vi.mock('../../api/api', () => ({ api: mocks.api }))

const providers = (ui: ReactNode) => (
  <ToastProvider>
    <AppDataProvider>{ui}</AppDataProvider>
  </ToastProvider>
)

function renderDialog(kind: 'claim' | 'hypothesis', list: Evidence[] = evidence) {
  const onClose = vi.fn()
  const onCreated = vi.fn()
  render(providers(<StatementDialog kind={kind} invRef="INV-0001" evidence={list} onClose={onClose} onCreated={onCreated} />))
  return { onClose, onCreated }
}

const row = (ref: string) => document.querySelector(`.statement-link-row[data-ref="${ref}"]`) as HTMLElement

describe('StatementDialog', () => {
  beforeEach(() => {
    Object.assign(mocks.api, mockApi({ addClaim: async () => newClaim }))
  })

  it('retries with what the form holds now, not the values at failure time (WR-01)', async () => {
    const addClaim = vi
      .fn<Api['addClaim']>()
      .mockRejectedValueOnce(new ApiError('busy', 'busy'))
      .mockResolvedValue(newClaim)
    Object.assign(mocks.api, mockApi({ addClaim }))
    const user = userEvent.setup()
    const { onClose } = renderDialog('claim')
    const statement = screen.getByLabelText('Statement')
    await user.type(statement, 'First wording.')
    await user.click(screen.getByRole('button', { name: 'Add claim' }))
    const toast = await screen.findByRole('alert')

    await user.clear(statement)
    await user.type(statement, 'Second wording.')
    await user.click(within(toast).getByRole('button', { name: 'Retry' }))
    await waitFor(() => expect(addClaim).toHaveBeenCalledTimes(2))
    expect(addClaim.mock.calls[1][1]).toMatchObject({ statement: 'Second wording.' })
    await waitFor(() => expect(onClose).toHaveBeenCalled())
  })

  it('adds a claim with confidence, rationale and supporting and contradicting evidence', async () => {
    const user = userEvent.setup()
    const { onClose, onCreated } = renderDialog('claim')
    expect(screen.getByRole('dialog', { name: 'Add claim' })).toBeInTheDocument()
    expect(screen.getByText('What do we believe? One clear sentence.')).toBeInTheDocument()
    expect(screen.getByRole('radio', { name: 'Not assessed' })).toHaveAttribute('aria-checked', 'true')
    await user.type(screen.getByLabelText('Statement'), 'The log level comes from the chart default.')
    await user.click(screen.getByRole('radio', { name: 'Medium' }))
    await user.type(screen.getByLabelText('Why this confidence (optional)'), 'Code and docs.')
    await user.click(within(row('E-001')).getByRole('radio', { name: 'Supports' }))
    await user.click(within(row('E-003')).getByRole('radio', { name: 'Contradicts' }))
    await user.click(screen.getByRole('button', { name: 'Add claim' }))
    expect(mocks.api.addClaim).toHaveBeenCalledWith('INV-0001', {
      statement: 'The log level comes from the chart default.',
      confidence: 'medium',
      confidence_rationale: 'Code and docs.',
      links: [
        { evidence_ref: 'E-001', relation: 'supports' },
        { evidence_ref: 'E-003', relation: 'contradicts' },
      ],
    })
    await waitFor(() => expect(onCreated).toHaveBeenCalledWith('C-005'))
    expect(onClose).toHaveBeenCalled()
  })

  it('lists every piece of evidence with None selected', () => {
    renderDialog('claim')
    for (const e of evidence) {
      const r = row(e.ref)
      expect(r).toHaveTextContent(e.title)
      expect(within(r).getByRole('radio', { name: 'None' })).toHaveAttribute('aria-checked', 'true')
    }
  })

  it('adds a hypothesis with rationale through addHypothesis', async () => {
    const user = userEvent.setup()
    Object.assign(mocks.api, mockApi({ addHypothesis: async () => ({ ...hypotheses[0], ref: 'H-004' }) }))
    const { onCreated } = renderDialog('hypothesis')
    expect(screen.getByRole('dialog', { name: 'Add hypothesis' })).toBeInTheDocument()
    expect(screen.getByText('What might explain it?')).toBeInTheDocument()
    await user.type(screen.getByLabelText('Statement'), 'Data transfer.')
    await user.type(screen.getByLabelText('Why this confidence (optional)'), 'A hunch.')
    await user.click(screen.getByRole('button', { name: 'Add hypothesis' }))
    expect(mocks.api.addHypothesis).toHaveBeenCalledWith('INV-0001', {
      statement: 'Data transfer.',
      confidence: 'unknown',
      rationale: 'A hunch.',
      links: [],
    })
    await waitFor(() => expect(onCreated).toHaveBeenCalledWith('H-004'))
  })

  it('says so when there is no evidence to link', () => {
    renderDialog('claim', [])
    expect(screen.getByText('No evidence to link yet. You can link it later.')).toBeInTheDocument()
  })

  it('shows Adding... while the call runs', async () => {
    const user = userEvent.setup()
    Object.assign(mocks.api, mockApi({ addClaim: () => new Promise<Claim>(() => {}) }))
    renderDialog('claim')
    await user.type(screen.getByLabelText('Statement'), 'S')
    await user.click(screen.getByRole('button', { name: 'Add claim' }))
    expect(screen.getByRole('button', { name: 'Adding...' })).toBeDisabled()
  })

  it('shows a link error on the evidence row it belongs to', async () => {
    const user = userEvent.setup()
    Object.assign(
      mocks.api,
      mockApi({
        addClaim: async () => {
          throw new ApiError('validation', 'evidence E-003 is not in INV-0001', 'links[1].evidence_ref')
        },
      }),
    )
    renderDialog('claim')
    await user.type(screen.getByLabelText('Statement'), 'S')
    await user.click(within(row('E-001')).getByRole('radio', { name: 'Supports' }))
    await user.click(within(row('E-003')).getByRole('radio', { name: 'Supports' }))
    await user.click(screen.getByRole('button', { name: 'Add claim' }))
    expect(await within(row('E-003')).findByText('evidence E-003 is not in INV-0001')).toBeInTheDocument()
    expect(within(row('E-001')).queryByText('evidence E-003 is not in INV-0001')).toBeNull()
  })

  it('shows a statement error under Statement', async () => {
    const user = userEvent.setup()
    Object.assign(
      mocks.api,
      mockApi({
        addClaim: async () => {
          throw new ApiError('validation', 'is required', 'statement')
        },
      }),
    )
    renderDialog('claim')
    await user.click(screen.getByRole('button', { name: 'Add claim' }))
    expect(await screen.findByText('is required')).toBeInTheDocument()
    expect(screen.getByLabelText('Statement')).toHaveAttribute('aria-invalid', 'true')
  })
})
