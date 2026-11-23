import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Api } from '../../api/api'
import { ApiError, REQUIRED_COPY } from '../../api/errors'
import { AppDataProvider } from '../../state/appData'
import { claims, hypotheses } from '../../test/fixtures'
import { claim, hypothesis } from '../../test/fixtures.records'
import { mockApi } from '../../test/mockApi'
import { ToastProvider } from '../ui/Toaster'
import { ReplaceFindingDialog, REPLACE_TITLE } from './ReplaceFindingDialog'
import type { Finding } from '../../lib/records'

const mocks = vi.hoisted(() => ({ api: {} as Api }))
vi.mock('../../api/api', () => ({ api: mocks.api }))

const NEW = 'The newer finding'

function renderReplace(finding: Finding) {
  const onClose = vi.fn()
  render(
    <ToastProvider>
      <AppDataProvider>
        <ReplaceFindingDialog finding={finding} invRef="INV-0001" onClose={onClose} />
      </AppDataProvider>
    </ToastProvider>,
  )
  return { onClose }
}

describe('ReplaceFindingDialog', () => {
  beforeEach(() => {
    Object.assign(mocks.api, mockApi({ addClaim: async () => claim({ ref: 'C-010' }), addHypothesis: async () => hypothesis({ ref: 'H-010' }) }))
  })

  it('has one field, shows the old finding, and works with no other finding on the page', () => {
    renderReplace({ kind: 'claim', record: claims[1] })
    const dialog = screen.getByRole('dialog', { name: REPLACE_TITLE })
    expect(dialog).toHaveTextContent(claims[1].statement)
    expect(dialog.querySelectorAll('textarea, input, select')).toHaveLength(1)
    expect(screen.getByLabelText(NEW)).toHaveFocus()
  })

  it('adds the newer claim with the same sources, then replaces the old one with it', async () => {
    const user = userEvent.setup()
    const { onClose } = renderReplace({ kind: 'claim', record: claims[1] })
    await user.type(screen.getByLabelText(NEW), 'The extra calls were about a bill sent twice.')
    await user.click(screen.getByRole('button', { name: 'Replace finding' }))
    expect(mocks.api.addClaim).toHaveBeenCalledWith('INV-0001', {
      statement: 'The extra calls were about a bill sent twice.',
      confidence: 'unknown',
      confidence_rationale: '',
      links: [
        { evidence_ref: 'E-002', relation: 'supports' },
        { evidence_ref: 'E-003', relation: 'supports' },
      ],
    })
    await waitFor(() => expect(mocks.api.supersedeClaim).toHaveBeenCalledWith('INV-0001', 'C-002', 'C-010'))
    expect(vi.mocked(mocks.api.addClaim).mock.invocationCallOrder[0]).toBeLessThan(vi.mocked(mocks.api.supersedeClaim).mock.invocationCallOrder[0])
    await waitFor(() => expect(onClose).toHaveBeenCalled())
  })

  it('a hypothesis that turned out right is replaced by one that also did', async () => {
    const user = userEvent.setup()
    renderReplace({ kind: 'hypothesis', record: hypotheses[2] })
    await user.type(screen.getByLabelText(NEW), 'Sales stayed the same.')
    await user.click(screen.getByRole('button', { name: 'Replace finding' }))
    expect(mocks.api.addHypothesis).toHaveBeenCalledWith('INV-0001', expect.objectContaining({ statement: 'Sales stayed the same.', status: 'supported' }))
    await waitFor(() => expect(mocks.api.supersedeHypothesis).toHaveBeenCalledWith('INV-0001', 'H-003', 'H-010'))
  })

  it('shows an empty finding error under the field and replaces nothing', async () => {
    const user = userEvent.setup()
    Object.assign(mocks.api, mockApi({ addClaim: async () => { throw new ApiError('validation', 'is required', 'statement') } }))
    const { onClose } = renderReplace({ kind: 'claim', record: claims[1] })
    await user.click(screen.getByRole('button', { name: 'Replace finding' }))
    expect(await screen.findByText(REQUIRED_COPY)).toBeInTheDocument()
    expect(mocks.api.supersedeClaim).not.toHaveBeenCalled()
    expect(onClose).not.toHaveBeenCalled()
  })
})
