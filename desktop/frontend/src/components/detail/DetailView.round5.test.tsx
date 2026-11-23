// What the third tester found, kept fixed: a finding where sources disagree
// has its own Decide button that opens the check on it, and a finding not
// tested yet can be left for later from its row.
import { useEffect, useRef } from 'react'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { Api } from '../../api/api'
import type { InvestigationDetail } from '../../api/types'
import { AppDataProvider, useAppData } from '../../state/appData'
import { CopyContextProvider } from '../../state/copyContext'
import { detail, summaries } from '../../test/fixtures'
import { mockApi } from '../../test/mockApi'
import { ToastProvider } from '../ui/Toaster'
import { CHECK_SOURCES_TITLE, DISAGREE_HEADING } from './CheckSources'
import { DECIDE_ACTION } from './ConclusionPanel'
import { DetailView } from './DetailView'

const mocks = vi.hoisted(() => ({ api: {} as Api }))
vi.mock('../../api/api', () => ({ api: mocks.api }))

function Open() {
  const { listStatus, select } = useAppData()
  const done = useRef(false)
  useEffect(() => {
    if (done.current || listStatus !== 'ready') return
    done.current = true
    select('INV-0001')
  }, [listStatus, select])
  return null
}

function renderPage(d: InvestigationDetail) {
  Object.assign(mocks.api, mockApi({ listInvestigations: async () => [summaries[0]], getInvestigationDetail: async () => d }))
  const ui = (
    <ToastProvider>
      <AppDataProvider>
        <Open />
        <CopyContextProvider>
          <DetailView />
        </CopyContextProvider>
      </AppDataProvider>
    </ToastProvider>
  )
  const view = render(ui)
  return { user: userEvent.setup(), view, ui }
}

describe('round 5 on the question page', () => {
  beforeEach(() => {
    Object.assign(mocks.api, mockApi())
    window.localStorage.clear()
  })
  afterEach(() => window.localStorage.clear())

  it('opens the check on a finding where sources disagree from its own Decide button, before any source', async () => {
    // Sources are still unchecked: the button goes straight to the finding.
    const { user } = renderPage(detail)
    const disagree = await screen.findByRole('region', { name: 'Where sources disagree' })
    const statement = 'Callers waited too long and hung up before buying.'
    await user.click(within(disagree).getByRole('button', { name: `${DECIDE_ACTION}: ${statement}` }))
    const dialog = await screen.findByRole('dialog', { name: CHECK_SOURCES_TITLE })
    expect(within(dialog).getByRole('heading', { name: DISAGREE_HEADING })).toBeInTheDocument()
    expect(within(dialog).getByText(statement)).toBeInTheDocument()
  })

  it('puts a finding not tested yet under Show 1 left for later, and keeps it there next time', async () => {
    const { user, view, ui } = renderPage(detail)
    const open = await screen.findByRole('region', { name: 'Still to find out' })
    const row = open.querySelector<HTMLElement>('[data-ref="H-001"]')!
    await user.click(within(row).getByRole('button', { name: 'Later' }))
    expect(open.querySelector('[data-ref="H-001"]')).toBeNull()
    expect(within(open).getByRole('button', { name: 'Show 1 left for later' })).toBeInTheDocument()
    expect(mocks.api.updateHypothesis).not.toHaveBeenCalled()
    view.unmount()
    render(ui)
    const again = await screen.findByRole('region', { name: 'Still to find out' })
    await waitFor(() => expect(within(again).getByRole('button', { name: 'Show 1 left for later' })).toBeInTheDocument())
    await user.click(within(again).getByRole('button', { name: 'Show 1 left for later' }))
    const hidden = again.querySelector<HTMLElement>('[data-ref="H-001"]')!
    expect(within(hidden).queryByRole('button', { name: 'Later' })).toBeNull()
    expect(within(hidden).getByRole('button', { name: 'Rule this out' })).toBeInTheDocument()
  })
})
