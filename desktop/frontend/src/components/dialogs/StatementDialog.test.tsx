import type { ReactNode } from 'react'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Api } from '../../api/api'
import { ApiError, NOT_SAVED_COPY, REQUIRED_COPY } from '../../api/errors'
import type { Claim, Evidence } from '../../api/types'
import { AppDataProvider } from '../../state/appData'
import { evidence } from '../../test/fixtures'
import { newClaim } from '../../test/fixtures.records'
import { mockApi } from '../../test/mockApi'
import { OpenAtLaunch } from '../../test/OpenAtLaunch'
import { ToastProvider } from '../ui/Toaster'
import { StatementDialog } from './StatementDialog'

const mocks = vi.hoisted(() => ({ api: {} as Api }))
vi.mock('../../api/api', () => ({ api: mocks.api }))

const providers = (ui: ReactNode) => (
  <ToastProvider>
    <AppDataProvider><OpenAtLaunch />{ui}</AppDataProvider>
  </ToastProvider>
)

function renderDialog(list: Evidence[] = evidence) {
  const onClose = vi.fn()
  const onCreated = vi.fn()
  render(providers(<StatementDialog invRef="INV-0001" evidence={list} onClose={onClose} onCreated={onCreated} />))
  return { onClose, onCreated }
}

const row = (ref: string) => document.querySelector(`.statement-link-row[data-ref="${ref}"]`) as HTMLElement

describe('StatementDialog', () => {
  beforeEach(() => {
    Object.assign(mocks.api, mockApi({ addClaim: async () => newClaim }))
  })

  it('retries with what the form holds now, not the values at failure time', async () => {
    const addClaim = vi
      .fn<Api['addClaim']>()
      .mockRejectedValueOnce(new ApiError('busy', 'busy'))
      .mockResolvedValue(newClaim)
    Object.assign(mocks.api, mockApi({ addClaim }))
    const user = userEvent.setup()
    const { onClose } = renderDialog()
    const statement = screen.getByLabelText('Finding')
    await user.type(statement, 'First wording.')
    await user.click(screen.getByRole('button', { name: 'Add finding' }))
    const toast = await screen.findByRole('alert')

    await user.clear(statement)
    await user.type(statement, 'Second wording.')
    await user.click(within(toast).getByRole('button', { name: 'Try again' }))
    await waitFor(() => expect(addClaim).toHaveBeenCalledTimes(2))
    expect(addClaim.mock.calls[1][1]).toMatchObject({ statement: 'Second wording.' })
    await waitFor(() => expect(onClose).toHaveBeenCalled())
  })

  it('adds a claim with its rationale and supporting and contradicting sources, and asks for no confidence', async () => {
    const user = userEvent.setup()
    const { onClose, onCreated } = renderDialog()
    expect(screen.getByRole('dialog', { name: 'Add finding' })).toBeInTheDocument()
    expect(screen.getByText('What did you find? One clear sentence.')).toBeInTheDocument()
    expect(screen.queryByRole('radiogroup', { name: 'Confidence' })).toBeNull()
    await user.type(screen.getByLabelText('Finding'), 'The bills went out twice because a file was loaded two times.')
    await user.type(screen.getByLabelText('Why you think so (optional)'), 'Code and docs.')
    await user.click(within(row('E-001')).getByRole('radio', { name: 'Backs it up' }))
    await user.click(within(row('E-003')).getByRole('radio', { name: 'Goes against it' }))
    await user.click(screen.getByRole('button', { name: 'Add finding' }))
    expect(mocks.api.addClaim).toHaveBeenCalledWith('INV-0001', {
      statement: 'The bills went out twice because a file was loaded two times.',
      confidence: 'unknown',
      confidence_rationale: 'Code and docs.',
      verification: 'ai_generated',
      links: [
        { evidence_ref: 'E-001', relation: 'supports' },
        { evidence_ref: 'E-003', relation: 'contradicts' },
      ],
    })
    expect(mocks.api.addHypothesis).not.toHaveBeenCalled()
    await waitFor(() => expect(onCreated).toHaveBeenCalledWith('C-005'))
    expect(onClose).toHaveBeenCalled()
  })

  it('lists every source with None, Backs it up and Goes against it, None selected', () => {
    renderDialog()
    for (const e of evidence) {
      const r = row(e.ref)
      expect(r).toHaveTextContent(e.title)
      expect(within(r).getAllByRole('radio').map((x) => x.textContent)).toEqual(['None', 'Backs it up', 'Goes against it'])
      expect(within(r).getByRole('radio', { name: 'None' })).toHaveAttribute('aria-checked', 'true')
    }
  })

  it('says so when there is no evidence to link', () => {
    renderDialog([])
    expect(screen.getByText('No sources yet. You can add one to this finding later.')).toBeInTheDocument()
  })

  it('shows Adding... while the call runs', async () => {
    const user = userEvent.setup()
    Object.assign(mocks.api, mockApi({ addClaim: () => new Promise<Claim>(() => {}) }))
    renderDialog()
    await user.type(screen.getByLabelText('Finding'), 'S')
    await user.click(screen.getByRole('button', { name: 'Add finding' }))
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
    renderDialog()
    await user.type(screen.getByLabelText('Finding'), 'S')
    await user.click(within(row('E-001')).getByRole('radio', { name: 'Backs it up' }))
    await user.click(within(row('E-003')).getByRole('radio', { name: 'Backs it up' }))
    await user.click(screen.getByRole('button', { name: 'Add finding' }))
    expect(await within(row('E-003')).findByText(NOT_SAVED_COPY)).toBeInTheDocument()
    expect(within(row('E-001')).queryByText(NOT_SAVED_COPY)).toBeNull()
    expect(screen.queryByText(/is not in INV-0001/)).toBeNull()
  })

  it('shows a finding error under Finding in plain words', async () => {
    const user = userEvent.setup()
    Object.assign(
      mocks.api,
      mockApi({
        addClaim: async () => {
          throw new ApiError('validation', 'is required', 'statement')
        },
      }),
    )
    renderDialog()
    await user.click(screen.getByRole('button', { name: 'Add finding' }))
    expect(await screen.findByText(REQUIRED_COPY)).toBeInTheDocument()
    expect(screen.queryByText('is required')).toBeNull()
    expect(screen.getByLabelText('Finding')).toHaveAttribute('aria-invalid', 'true')
  })
})
