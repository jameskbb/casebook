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
import { OpenAtLaunch } from '../../test/OpenAtLaunch'
import { ToastProvider } from '../ui/Toaster'
import { DeferQuestionDialog } from './DeferQuestionDialog'
import { QuestionDialog } from './QuestionDialog'
import { ResolveQuestionDialog } from './ResolveQuestionDialog'

const mocks = vi.hoisted(() => ({ api: {} as Api }))
vi.mock('../../api/api', () => ({ api: mocks.api }))

const providers = (ui: ReactNode) => (
  <ToastProvider>
    <AppDataProvider><OpenAtLaunch />{ui}</AppDataProvider>
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
    expect(screen.getByRole('dialog', { name: 'Add follow-up question' })).toBeInTheDocument()
    expect(screen.getByText('What still needs an answer?')).toBeInTheDocument()
    await user.type(screen.getByLabelText('Follow-up question'), 'Who signs off a refund over $100?')
    await user.click(screen.getByRole('button', { name: 'Add follow-up question' }))
    expect(mocks.api.addQuestion).toHaveBeenCalledWith('INV-0001', { question: 'Who signs off a refund over $100?' })
    await waitFor(() => expect(onCreated).toHaveBeenCalledWith('Q-004'))
    expect(onClose).toHaveBeenCalled()
  })

  it('shows Adding... while the call runs', async () => {
    const user = userEvent.setup()
    Object.assign(mocks.api, mockApi({ addQuestion: () => new Promise<Question>(() => {}) }))
    render(providers(<QuestionDialog invRef="INV-0001" onClose={() => {}} />))
    await user.type(screen.getByLabelText('Follow-up question'), 'Q')
    await user.click(screen.getByRole('button', { name: 'Add follow-up question' }))
    expect(screen.getByRole('button', { name: 'Adding...' })).toBeDisabled()
  })

  it('edits a question with its text', async () => {
    const user = userEvent.setup()
    const onClose = vi.fn()
    render(providers(<QuestionDialog invRef="INV-0001" question={Q001} onClose={onClose} />))
    expect(screen.getByRole('dialog', { name: 'Edit follow-up question' })).toBeInTheDocument()
    const box = screen.getByLabelText('Follow-up question')
    expect(box).toHaveValue(Q001.question)
    await user.clear(box)
    await user.type(box, 'Can the refunds go out this week?')
    await user.click(screen.getByRole('button', { name: 'Save changes' }))
    expect(mocks.api.updateQuestion).toHaveBeenCalledWith('INV-0001', 'Q-001', 'Can the refunds go out this week?')
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
    await user.click(screen.getByRole('button', { name: 'Add follow-up question' }))
    expect(await screen.findByText('is required')).toBeInTheDocument()
    expect(screen.getByLabelText('Follow-up question')).toHaveAttribute('aria-invalid', 'true')
  })
})

describe('ResolveQuestionDialog', () => {
  beforeEach(() => {
    Object.assign(mocks.api, mockApi())
  })

  it('requires an answer, then saves it', async () => {
    const user = userEvent.setup()
    const onClose = vi.fn()
    render(providers(<ResolveQuestionDialog invRef="INV-0001" question={Q001} onClose={onClose} />))
    expect(screen.getByRole('dialog', { name: 'Answer this follow-up question' })).toBeInTheDocument()
    expect(screen.getByText('What is the answer, and how do we know?')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Save answer' }))
    expect(mocks.api.resolveQuestion).not.toHaveBeenCalled()
    expect(screen.getByLabelText('Answer')).toHaveAttribute('aria-invalid', 'true')
    expect(screen.getByText('Write the answer first.')).toBeInTheDocument()

    await user.type(screen.getByLabelText('Answer'), 'Yes, says the head of billing.')
    await user.click(screen.getByRole('button', { name: 'Save answer' }))
    expect(mocks.api.resolveQuestion).toHaveBeenCalledWith('INV-0001', 'Q-001', 'Yes, says the head of billing.')
    await waitFor(() => expect(onClose).toHaveBeenCalled())
  })

  it('shows Saving... while the answer is saved', async () => {
    const user = userEvent.setup()
    Object.assign(mocks.api, mockApi({ resolveQuestion: () => new Promise<Question>(() => {}) }))
    render(providers(<ResolveQuestionDialog invRef="INV-0001" question={Q001} onClose={() => {}} />))
    await user.type(screen.getByLabelText('Answer'), 'Yes.')
    await user.click(screen.getByRole('button', { name: 'Save answer' }))
    expect(screen.getByRole('button', { name: 'Saving...' })).toBeDisabled()
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
    expect(screen.getByRole('dialog', { name: 'Set this follow-up question aside' })).toBeInTheDocument()
    expect(screen.getByText('Why it can wait')).toBeInTheDocument()
    await user.type(screen.getByLabelText('Note (optional)'), 'After the refund list is ready.')
    await user.click(screen.getByRole('button', { name: 'Set aside' }))
    expect(mocks.api.deferQuestion).toHaveBeenCalledWith('INV-0001', 'Q-001', 'After the refund list is ready.')
    await waitFor(() => expect(onClose).toHaveBeenCalled())
  })

  it('defers without a note', async () => {
    const user = userEvent.setup()
    render(providers(<DeferQuestionDialog invRef="INV-0001" question={Q001} onClose={() => {}} />))
    await user.click(screen.getByRole('button', { name: 'Set aside' }))
    expect(mocks.api.deferQuestion).toHaveBeenCalledWith('INV-0001', 'Q-001', '')
  })

  it('shows Saving... while it is set aside', async () => {
    const user = userEvent.setup()
    Object.assign(mocks.api, mockApi({ deferQuestion: () => new Promise<Question>(() => {}) }))
    render(providers(<DeferQuestionDialog invRef="INV-0001" question={Q001} onClose={() => {}} />))
    await user.click(screen.getByRole('button', { name: 'Set aside' }))
    expect(screen.getByRole('button', { name: 'Saving...' })).toBeDisabled()
  })
})
