import { render, screen } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Api } from '../../api/api'
import type { Investigation } from '../../api/types'
import { AppDataProvider } from '../../state/appData'
import { investigation as inv } from '../../test/fixtures'
import { mockApi } from '../../test/mockApi'
import { ToastProvider } from '../ui/Toaster'
import { ActiveToggle } from './ActiveToggle'

const mocks = vi.hoisted(() => ({ api: {} as Api }))
vi.mock('../../api/api', () => ({ api: mocks.api }))

function renderToggle(investigation: Investigation) {
  return render(
    <ToastProvider>
      <AppDataProvider>
        <ActiveToggle investigation={investigation} />
      </AppDataProvider>
    </ToastProvider>,
  )
}

describe('ActiveToggle', () => {
  beforeEach(() => {
    Object.assign(mocks.api, mockApi())
  })

  it('names the AI, not an engine state', () => {
    renderToggle({ ...inv, is_active: false })
    expect(screen.getByRole('button', { name: 'Use with my AI' })).toHaveAttribute(
      'title',
      'Click so your AI tools start from this case.',
    )
  })

  it('reads as the current case when on', () => {
    renderToggle({ ...inv, is_active: true })
    expect(screen.getByRole('button', { name: "Your AI's current case", pressed: true })).toBeInTheDocument()
  })
})
