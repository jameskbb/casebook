// What the third tester found, kept fixed: a finding where sources disagree
// has its own Decide button that opens the check on it, and a finding not
// tested yet offers This turned out right and Rule this out, with no Later.
import { useEffect, useRef } from 'react'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Api } from '../../api/api'
import type { InvestigationDetail } from '../../api/types'
import { AppDataProvider, useAppData } from '../../state/appData'
import { CopyContextProvider } from '../../state/copyContext'
import { detail, summaries } from '../../test/fixtures'
import { mockApi } from '../../test/mockApi'
import { ToastProvider } from '../ui/Toaster'
import { DECIDE_TITLE } from './CheckSources'
import { STILL_HOLDS_QUESTION } from '../drawers/StatementDrawer'
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
  })

  it('opens the check on a finding where sources disagree from its own Decide button, before any source', async () => {
    // Sources are still unchecked: the button goes straight to the finding.
    const { user } = renderPage(detail)
    const disagree = await screen.findByRole('region', { name: 'Where sources disagree' })
    const statement = 'Callers waited too long and hung up before buying.'
    // A real button on the row, quieter than the answer card's main one.
    expect(within(disagree).getByRole('button', { name: `${DECIDE_ACTION}: ${statement}` })).toHaveClass('btn-secondary')
    await user.click(within(disagree).getByRole('button', { name: `${DECIDE_ACTION}: ${statement}` }))
    const dialog = await screen.findByRole('dialog', { name: DECIDE_TITLE })
    expect(within(dialog).getByRole('heading', { name: STILL_HOLDS_QUESTION })).toHaveFocus()
    expect(within(dialog).getByText(statement)).toBeInTheDocument()
  })

  it('gives focus back to the Decide button that opened the check on Escape, without moving the page', async () => {
    const { user } = renderPage(detail)
    const disagree = await screen.findByRole('region', { name: 'Where sources disagree' })
    const decide = within(disagree).getByRole('button', { name: `${DECIDE_ACTION}: Callers waited too long and hung up before buying.` })
    await user.click(decide)
    await screen.findByRole('dialog', { name: DECIDE_TITLE })
    const focus = vi.spyOn(HTMLElement.prototype, 'focus')
    await user.keyboard('{Escape}')
    await waitFor(() => expect(document.activeElement).toBe(decide))
    expect(focus.mock.instances.filter((el) => (el as unknown as HTMLElement).closest('.conclusion-panel'))).toHaveLength(0)
    expect(focus.mock.calls.at(-1)?.[0]).toEqual({ preventScroll: true })
    focus.mockRestore()
  })

  it('offers This turned out right and Rule this out on a finding not tested yet, and no Later', async () => {
    renderPage(detail)
    const open = await screen.findByRole('region', { name: 'Still to find out' })
    const row = open.querySelector<HTMLElement>('[data-ref="H-001"]')!
    expect(within(row).getByRole('button', { name: 'This turned out right' })).toBeInTheDocument()
    expect(within(row).getByRole('button', { name: 'Rule this out' })).toBeInTheDocument()
    expect(within(row).queryByRole('button', { name: 'Later' })).toBeNull()
    expect(within(open).queryByRole('button', { name: /left for later/ })).toBeNull()
  })
})
