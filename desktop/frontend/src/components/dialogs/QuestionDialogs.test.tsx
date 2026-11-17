import type { ReactNode } from 'react'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Api } from '../../api/api'
import { ApiError } from '../../api/errors'
import type { Question } from '../../api/types'
import { AppDataProvider } from '../../state/appData'
import { questions } from '../../test/fixtures'
import { question } from '../../test/fixtures.records'
import { mockApi } from '../../test/mockApi'
import { ToastProvider } from '../ui/Toaster'
import { DeferQuestionDialog } from './DeferQuestionDialog'
import { QuestionDialog } from './QuestionDialog'
import { ResolveQuestionDialog } from './ResolveQuestionDialog'

const mocks = vi.hoisted(() => ({ api: {} as Api }))
vi.mock('../../api/api', () => ({ api: mocks.api }))

const providers = (ui: ReactNode) => (
  <ToastProvider>
    <AppDataProvider>{ui}</AppDataProvider>
  </ToastProvider>
)

const Q001 = questions[0]

describe('QuestionDialog', () => {
  beforeEach(() => {
    Object.assign(mocks.api, mockApi({ addQuestion: async () => question({ ref: 'Q-004', question: 'New?' }) }))
  })

  it('adds a question and reports the new ref', async () => {
    const user = userEvent.setup()
    const onClose = vi.fn()
    const onCreated = vi.fn()
    render(providers(<QuestionDialog invRef="INV-0001" onClose={onClose} onCreated={onCreated} />))
    expect(screen.getByRole('dialog', { name: 'Add question' })).toBeInTheDocument()
    expect(screen.getByText('What still needs an answer?')).toBeInTheDocument()
    await user.type(screen.getByLabelText('Question'), 'Who approves a change to a shared chart default?')
    await user.click(screen.getByRole('button', { name: 'Add question' }))
    expect(mocks.api.addQuestion).toHaveBeenCalledWith('INV-0001', { question: 'Who approves a change to a shared chart default?' })
    await waitFor(() => expect(onCreated).toHaveBeenCalledWith('Q-004'))
    expect(onClose).toHaveBeenCalled()
  })

  it('shows Adding... while the call runs', async () => {
    const user = userEvent.setup()
    Object.assign(mocks.api, mockApi({ addQuestion: () => new Promise<Question>(() => {}) }))
    render(providers(<QuestionDialog invRef="INV-0001" onClose={() => {}} />))
    await user.type(screen.getByLabelText('Question'), 'Q')
    await user.click(screen.getByRole('button', { name: 'Add question' }))
    expect(screen.getByRole('button', { name: 'Adding...' })).toBeDisabled()
  })

  it('edits a question with its text', async () => {
    const user = userEvent.setup()
    const onClose = vi.fn()
    render(providers(<QuestionDialog invRef="INV-0001" question={Q001} onClose={onClose} />))
    expect(screen.getByRole('dialog', { name: 'Edit question Q-001' })).toBeInTheDocument()
    const box = screen.getByLabelText('Question')
    expect(box).toHaveValue(Q001.question)
    await user.clear(box)
    await user.type(box, 'Can payments keep debug level?')
    await user.click(screen.getByRole('button', { name: 'Save changes' }))
    expect(mocks.api.updateQuestion).toHaveBeenCalledWith('INV-0001', 'Q-001', 'Can payments keep debug level?')
    await waitFor(() => expect(onClose).toHaveBeenCalled())
  })

  it('shows a question error under the field', async () => {
    const user = userEvent.setup()
    Object.assign(
      mocks.api,
      mockApi({
        addQuestion: async () => {
          throw new ApiError('validation', 'is required', 'question')
        },
      }),
    )
    render(providers(<QuestionDialog invRef="INV-0001" onClose={() => {}} />))
    await user.click(screen.getByRole('button', { name: 'Add question' }))
    expect(await screen.findByText('is required')).toBeInTheDocument()
    expect(screen.getByLabelText('Question')).toHaveAttribute('aria-invalid', 'true')
  })
})

describe('ResolveQuestionDialog', () => {
  beforeEach(() => {
    Object.assign(mocks.api, mockApi())
  })

  it('requires a resolution, then resolves the question', async () => {
    const user = userEvent.setup()
    const onClose = vi.fn()
    render(providers(<ResolveQuestionDialog invRef="INV-0001" question={Q001} onClose={onClose} />))
    expect(screen.getByRole('dialog', { name: 'Resolve Q-001' })).toBeInTheDocument()
    expect(screen.getByText('What is the answer, and how do we know?')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Resolve question' }))
    expect(mocks.api.resolveQuestion).not.toHaveBeenCalled()
    expect(screen.getByLabelText('Resolution')).toHaveAttribute('aria-invalid', 'true')
    expect(screen.getByText('Write the answer before resolving.')).toBeInTheDocument()

    await user.type(screen.getByLabelText('Resolution'), 'Yes, per the head of platform.')
    await user.click(screen.getByRole('button', { name: 'Resolve question' }))
    expect(mocks.api.resolveQuestion).toHaveBeenCalledWith('INV-0001', 'Q-001', 'Yes, per the head of platform.')
    await waitFor(() => expect(onClose).toHaveBeenCalled())
  })

  it('shows Resolving... while the call runs', async () => {
    const user = userEvent.setup()
    Object.assign(mocks.api, mockApi({ resolveQuestion: () => new Promise<Question>(() => {}) }))
    render(providers(<ResolveQuestionDialog invRef="INV-0001" question={Q001} onClose={() => {}} />))
    await user.type(screen.getByLabelText('Resolution'), 'Yes.')
    await user.click(screen.getByRole('button', { name: 'Resolve question' }))
    expect(screen.getByRole('button', { name: 'Resolving...' })).toBeDisabled()
  })
})

describe('DeferQuestionDialog', () => {
  beforeEach(() => {
    Object.assign(mocks.api, mockApi())
  })

  it('defers with an optional note', async () => {
    const user = userEvent.setup()
    const onClose = vi.fn()
    render(providers(<DeferQuestionDialog invRef="INV-0001" question={Q001} onClose={onClose} />))
    expect(screen.getByRole('dialog', { name: 'Defer Q-001' })).toBeInTheDocument()
    expect(screen.getByText('Why it can wait')).toBeInTheDocument()
    await user.type(screen.getByLabelText('Note (optional)'), 'After the quarter close.')
    await user.click(screen.getByRole('button', { name: 'Defer question' }))
    expect(mocks.api.deferQuestion).toHaveBeenCalledWith('INV-0001', 'Q-001', 'After the quarter close.')
    await waitFor(() => expect(onClose).toHaveBeenCalled())
  })

  it('defers without a note', async () => {
    const user = userEvent.setup()
    render(providers(<DeferQuestionDialog invRef="INV-0001" question={Q001} onClose={() => {}} />))
    await user.click(screen.getByRole('button', { name: 'Defer question' }))
    expect(mocks.api.deferQuestion).toHaveBeenCalledWith('INV-0001', 'Q-001', '')
  })

  it('shows Deferring... while the call runs', async () => {
    const user = userEvent.setup()
    Object.assign(mocks.api, mockApi({ deferQuestion: () => new Promise<Question>(() => {}) }))
    render(providers(<DeferQuestionDialog invRef="INV-0001" question={Q001} onClose={() => {}} />))
    await user.click(screen.getByRole('button', { name: 'Defer question' }))
    expect(screen.getByRole('button', { name: 'Deferring...' })).toBeDisabled()
  })
})
