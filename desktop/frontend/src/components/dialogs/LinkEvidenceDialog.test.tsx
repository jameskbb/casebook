import type { ReactNode } from 'react'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Api } from '../../api/api'
import { ApiError } from '../../api/errors'
import type { Claim, Evidence, Hypothesis } from '../../api/types'
import { AppDataProvider } from '../../state/appData'
import { claims, evidence, hypotheses } from '../../test/fixtures'
import { claim } from '../../test/fixtures.records'
import { mockApi } from '../../test/mockApi'
import { ToastProvider } from '../ui/Toaster'
import { LinkEvidenceDialog } from './LinkEvidenceDialog'

const mocks = vi.hoisted(() => ({ api: {} as Api }))
vi.mock('../../api/api', () => ({ api: mocks.api }))

const providers = (ui: ReactNode) => (
  <ToastProvider>
    <AppDataProvider>{ui}</AppDataProvider>
  </ToastProvider>
)

function renderDialog(
  record: Claim | Hypothesis,
  list: Evidence[] = evidence,
  props: { onClose?: () => void; onAddEvidence?: () => void } = {},
) {
  const onClose = props.onClose ?? vi.fn()
  const kind = record.ref.startsWith('C') ? 'claim' : 'hypothesis'
  render(
    providers(
      <LinkEvidenceDialog
        kind={kind}
        record={record}
        evidence={list}
        invRef="INV-0001"
        onClose={onClose}
        onAddEvidence={props.onAddEvidence}
      />,
    ),
  )
  return { onClose }
}

describe('LinkEvidenceDialog', () => {
  beforeEach(() => {
    Object.assign(mocks.api, mockApi())
  })

  it('lists the evidence with already linked items disabled and labelled', () => {
    renderDialog(claims[0])
    const dialog = screen.getByRole('dialog', { name: 'Link evidence to C-001' })
    const e1 = within(dialog).getByRole('radio', { name: /E-001/ })
    expect(e1).toBeDisabled()
    expect(e1.closest('label')).toHaveTextContent('Already linked as supporting')
    expect(within(dialog).getByRole('radio', { name: /E-004/ })).toBeDisabled()
    expect(within(dialog).getByRole('radio', { name: /E-002/ })).toBeEnabled()
    expect(within(dialog).getByRole('radio', { name: /E-002/ }).closest('label')).toHaveTextContent(
      'Logs are billed per GB ingested in the month',
    )
    expect(within(dialog).getByRole('radio', { name: 'Supports' })).toHaveAttribute('aria-checked', 'true')
  })

  it('labels evidence linked as contradicting', () => {
    renderDialog(claims[3])
    expect(screen.getByRole('radio', { name: /E-003/ }).closest('label')).toHaveTextContent(
      'Already linked as contradicting',
    )
  })

  it('links the chosen evidence with the chosen relation and closes', async () => {
    const user = userEvent.setup()
    const { onClose } = renderDialog(claims[0])
    await user.click(screen.getByRole('radio', { name: /E-002/ }))
    await user.click(screen.getByRole('button', { name: 'Link evidence' }))
    expect(mocks.api.linkEvidenceToClaim).toHaveBeenCalledWith('INV-0001', 'C-001', 'E-002', 'supports')
    await waitFor(() => expect(onClose).toHaveBeenCalled())
  })

  it('links contradicting evidence to a hypothesis', async () => {
    const user = userEvent.setup()
    renderDialog(hypotheses[0])
    expect(screen.getByRole('dialog', { name: 'Link evidence to H-001' })).toBeInTheDocument()
    await user.click(screen.getByRole('radio', { name: /E-003/ }))
    await user.click(screen.getByRole('radio', { name: 'Contradicts' }))
    await user.click(screen.getByRole('button', { name: 'Link evidence' }))
    expect(mocks.api.linkEvidenceToHypothesis).toHaveBeenCalledWith('INV-0001', 'H-001', 'E-003', 'contradicts')
  })

  it('asks for a choice before linking', async () => {
    const user = userEvent.setup()
    renderDialog(claims[0])
    await user.click(screen.getByRole('button', { name: 'Link evidence' }))
    expect(screen.getByText('Choose the evidence to link.')).toBeInTheDocument()
    expect(mocks.api.linkEvidenceToClaim).not.toHaveBeenCalled()
  })

  it('shows Linking... while the call runs', async () => {
    const user = userEvent.setup()
    Object.assign(mocks.api, mockApi({ linkEvidenceToClaim: () => new Promise<Claim>(() => {}) }))
    renderDialog(claims[0])
    await user.click(screen.getByRole('radio', { name: /E-002/ }))
    await user.click(screen.getByRole('button', { name: 'Link evidence' }))
    expect(screen.getByRole('button', { name: 'Linking...' })).toBeDisabled()
  })

  it('shows a conflict inside the dialog', async () => {
    const user = userEvent.setup()
    Object.assign(
      mocks.api,
      mockApi({
        linkEvidenceToClaim: async () => {
          throw new ApiError('conflict', 'conflict: evidence E-002 belongs to another investigation')
        },
      }),
    )
    renderDialog(claims[0])
    await user.click(screen.getByRole('radio', { name: /E-002/ }))
    await user.click(screen.getByRole('button', { name: 'Link evidence' }))
    expect(await screen.findByText('Evidence E-002 belongs to another investigation')).toBeInTheDocument()
  })

  it('says so when the investigation has no evidence and offers Add evidence', async () => {
    const user = userEvent.setup()
    const onAddEvidence = vi.fn()
    renderDialog(claim({ links: [] }), [], { onAddEvidence })
    expect(screen.getByText('This case has no evidence yet. Add evidence first.')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Add evidence' }))
    expect(onAddEvidence).toHaveBeenCalled()
  })

  it('says so when every piece of evidence is already linked', () => {
    renderDialog(claims[0], evidence.filter((e) => e.ref === 'E-001' || e.ref === 'E-004'))
    expect(screen.getByText('Every piece of evidence is already linked to C-001.')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Link evidence' })).toBeNull()
    expect(screen.getAllByRole('button', { name: 'Close' })).toHaveLength(2)
  })
})
