import type { ReactNode } from 'react'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Api } from '../../api/api'
import { ApiError, REQUIRED_COPY } from '../../api/errors'
import type { Question } from '../../api/types'
import { AppDataProvider } from '../../state/appData'
import { hypotheses, questions } from '../../test/fixtures'
import { question } from '../../test/fixtures.records'
import { mockApi } from '../../test/mockApi'
import { OpenAtLaunch } from '../../test/OpenAtLaunch'
import { ToastProvider } from '../ui/Toaster'
import { DeferQuestionDialog } from './DeferQuestionDialog'
import { ADD_TO_FIND_OUT, QuestionDialog } from './QuestionDialog'
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

  it('adds a question by default and reports the new ref', async () => {
    const user = userEvent.setup()
    const onClose = vi.fn()
    const onCreated = vi.fn()
    render(providers(<QuestionDialog invRef="INV-0001" onClose={onClose} onCreated={onCreated} />))
    expect(screen.getByRole('dialog', { name: ADD_TO_FIND_OUT })).toBeInTheDocument()
    const kind = screen.getByRole('radiogroup', { name: 'Is this a guess to test or a question to answer?' })
    expect(within(kind).getByRole('radio', { name: 'A question to answer' })).toHaveAttribute('aria-checked', 'true')
    await user.type(screen.getByLabelText('What do you still need to find out?'), 'Who signs off a refund over $100?')
    await user.click(screen.getByRole('button', { name: 'Add' }))
    expect(mocks.api.addQuestion).toHaveBeenCalledWith('INV-0001', { question: 'Who signs off a refund over $100?' })
    expect(mocks.api.addHypothesis).not.toHaveBeenCalled()
    await waitFor(() => expect(onCreated).toHaveBeenCalledWith('Q-004'))
    expect(onClose).toHaveBeenCalled()
  })

  it('adds a guess to test as a finding with no confidence and no sources', async () => {
    const user = userEvent.setup()
    const onCreated = vi.fn()
    Object.assign(mocks.api, mockApi({ addHypothesis: async () => ({ ...hypotheses[0], ref: 'H-004' }) }))
    render(providers(<QuestionDialog invRef="INV-0001" onClose={() => {}} onCreated={onCreated} />))
    await user.type(screen.getByLabelText('What do you still need to find out?'), 'The refunds went out late.')
    await user.click(screen.getByRole('radio', { name: 'A guess to test' }))
    await user.click(screen.getByRole('button', { name: 'Add' }))
    expect(mocks.api.addHypothesis).toHaveBeenCalledWith('INV-0001', {
      statement: 'The refunds went out late.',
      confidence: 'unknown',
      rationale: '',
      links: [],
    })
    expect(mocks.api.addQuestion).not.toHaveBeenCalled()
    await waitFor(() => expect(onCreated).toHaveBeenCalledWith('H-004'))
  })

  it('shows Adding... while the call runs', async () => {
    const user = userEvent.setup()
    Object.assign(mocks.api, mockApi({ addQuestion: () => new Promise<Question>(() => {}) }))
    render(providers(<QuestionDialog invRef="INV-0001" onClose={() => {}} />))
    await user.type(screen.getByLabelText('What do you still need to find out?'), 'Q')
    await user.click(screen.getByRole('button', { name: 'Add' }))
    expect(screen.getByRole('button', { name: 'Adding...' })).toBeDisabled()
  })

  it('edits a question with its text', async () => {
    const user = userEvent.setup()
    const onClose = vi.fn()
    render(providers(<QuestionDialog invRef="INV-0001" question={Q001} onClose={onClose} />))
    expect(screen.getByRole('dialog', { name: 'Edit follow-up question' })).toBeInTheDocument()
    expect(screen.queryByRole('radiogroup')).toBeNull()
    const box = screen.getByLabelText('Follow-up question')
    expect(box).toHaveValue(Q001.question)
    await user.clear(box)
    await user.type(box, 'Can the refunds go out this week?')
    await user.click(screen.getByRole('button', { name: 'Save changes' }))
    expect(mocks.api.updateQuestion).toHaveBeenCalledWith('INV-0001', 'Q-001', 'Can the refunds go out this week?')
    await waitFor(() => expect(onClose).toHaveBeenCalled())
  })

  it('shows a question error under the field in plain words', async () => {
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
    await user.click(screen.getByRole('button', { name: 'Add' }))
    expect(await screen.findByText(REQUIRED_COPY)).toBeInTheDocument()
    expect(screen.queryByText('is required')).toBeNull()
    expect(screen.getByLabelText('What do you still need to find out?')).toHaveAttribute('aria-invalid', 'true')
  })
})

describe('ResolveQuestionDialog', () => {
  beforeEach(() => {
    Object.assign(mocks.api, mockApi())
  })

  it('refuses an empty answer, then saves it', async () => {
    const user = userEvent.setup()
    const onClose = vi.fn()
    render(providers(<ResolveQuestionDialog invRef="INV-0001" question={Q001} onClose={onClose} />))
    expect(screen.getByRole('dialog', { name: 'Answer this follow-up question' })).toBeInTheDocument()
    expect(screen.getByText('What is the answer, and how do you know?')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Save answer' }))
    expect(mocks.api.resolveQuestion).not.toHaveBeenCalled()
    expect(screen.getByLabelText('Answer')).toHaveAttribute('aria-invalid', 'true')
    expect(screen.getByText(REQUIRED_COPY)).toBeInTheDocument()

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

  it('leaves it for later with an optional note', async () => {
    const user = userEvent.setup()
    const onClose = vi.fn()
    render(providers(<DeferQuestionDialog invRef="INV-0001" question={Q001} onClose={onClose} />))
    expect(screen.getByRole('dialog', { name: 'Come back to this later' })).toBeInTheDocument()
    await user.type(screen.getByLabelText('Why it can wait (optional)'), 'After the refund list is ready.')
    await user.click(screen.getByRole('button', { name: 'Later' }))
    expect(mocks.api.deferQuestion).toHaveBeenCalledWith('INV-0001', 'Q-001', 'After the refund list is ready.')
    await waitFor(() => expect(onClose).toHaveBeenCalled())
  })

  it('leaves it for later without a note', async () => {
    const user = userEvent.setup()
    render(providers(<DeferQuestionDialog invRef="INV-0001" question={Q001} onClose={() => {}} />))
    await user.click(screen.getByRole('button', { name: 'Later' }))
    expect(mocks.api.deferQuestion).toHaveBeenCalledWith('INV-0001', 'Q-001', '')
  })

  it('shows Saving... while it is saved for later', async () => {
    const user = userEvent.setup()
    Object.assign(mocks.api, mockApi({ deferQuestion: () => new Promise<Question>(() => {}) }))
    render(providers(<DeferQuestionDialog invRef="INV-0001" question={Q001} onClose={() => {}} />))
    await user.click(screen.getByRole('button', { name: 'Later' }))
    expect(screen.getByRole('button', { name: 'Saving...' })).toBeDisabled()
  })
})
