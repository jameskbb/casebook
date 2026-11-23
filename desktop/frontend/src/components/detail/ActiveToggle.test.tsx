import { render, screen } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Api } from '../../api/api'
import type { Investigation } from '../../api/types'
import { AppDataProvider } from '../../state/appData'
import { investigation as inv } from '../../test/fixtures'
import { mockApi } from '../../test/mockApi'
import { OpenAtLaunch } from '../../test/OpenAtLaunch'
import { ToastProvider } from '../ui/Toaster'
import { ACTIVE_OFF_LABEL, ACTIVE_ON_LABEL, ACTIVE_TITLE_OFF, ACTIVE_TITLE_ON, ActiveToggle } from './ActiveToggle'

const mocks = vi.hoisted(() => ({ api: {} as Api }))
vi.mock('../../api/api', () => ({ api: mocks.api }))

function renderToggle(investigation: Investigation) {
  return render(
    <ToastProvider>
      <AppDataProvider><OpenAtLaunch />
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
    expect(screen.getByRole('button', { name: ACTIVE_OFF_LABEL, pressed: false })).toHaveAttribute('title', ACTIVE_TITLE_OFF)
  })

  it('says the AI is working on this question when on', () => {
    renderToggle({ ...inv, is_active: true })
    expect(screen.getByRole('button', { name: ACTIVE_ON_LABEL, pressed: true })).toHaveAttribute('title', ACTIVE_TITLE_ON)
  })
})
