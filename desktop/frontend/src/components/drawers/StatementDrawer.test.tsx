import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Api } from '../../api/api'
import type { InvestigationDetail } from '../../api/types'
import { formatAbsolute } from '../../lib/relativeTime'
import { AppDataProvider } from '../../state/appData'
import { CopyContextProvider } from '../../state/copyContext'
import { detail, summaries, T0 } from '../../test/fixtures'
import { mockApi } from '../../test/mockApi'
import { OpenAtLaunch } from '../../test/OpenAtLaunch'
import { DetailView } from '../detail/DetailView'
import { ToastProvider } from '../ui/Toaster'

const mocks = vi.hoisted(() => ({ api: {} as Api }))
vi.mock('../../api/api', () => ({ api: mocks.api }))

function renderDetail(d: InvestigationDetail = detail) {
  Object.assign(
    mocks.api,
    mockApi({
      listInvestigations: async () => [summaries[0]],
      getInvestigationDetail: async () => d,
    }),
  )
  return render(
    <ToastProvider>
      <AppDataProvider><OpenAtLaunch />
        <CopyContextProvider>
          <DetailView />
        </CopyContextProvider>
      </AppDataProvider>
    </ToastProvider>,
  )
}

const openClaimButton = (ref: string) => screen.findByRole('button', { name: new RegExp(`^Open finding ${ref}:`) })

describe('StatementDrawer drill-down', () => {
  beforeEach(() => {
    Object.assign(mocks.api, mockApi())
  })

  it('opens the claim drawer from its row with focus on the heading and lists the supporting evidence', async () => {
    const user = userEvent.setup()
    renderDetail()
    await user.click(await openClaimButton('C-001'))

    const drawer = screen.getByRole('dialog', { name: 'Finding C-001' })
    expect(within(drawer).getByRole('heading', { level: 2, name: 'Finding C-001' })).toHaveFocus()
    expect(
      within(drawer).getByText(
        'A billing mistake on October 3 charged some customers twice.',
      ),
    ).toHaveClass('drawer-statement')

    const supporting = within(drawer).getByRole('region', { name: 'Sources that support it' })
    const blocks = supporting.querySelectorAll('[data-ref]')
    expect(Array.from(blocks).map((b) => b.getAttribute('data-ref'))).toEqual(['E-001', 'E-004'])

    const e1 = within(supporting).getByRole('button', { name: 'Open source E-001: Notice dated October 4: 1,900 customers were charged twice' })
      .parentElement as HTMLElement
    expect(within(e1).getByText('Notice dated October 4: 1,900 customers were charged twice')).toBeInTheDocument()
    expect(within(e1).getByText(/charged 1,900 customers twice/)).toBeInTheDocument()
    expect(within(e1).getByText('File')).toBeInTheDocument()
    expect(within(e1).getByText('Email from the billing team')).toBeInTheDocument()
    expect(within(e1).getByText('Shared drive, Billing, notice-2026-10-04.pdf')).toBeInTheDocument()
    expect(within(e1).getByText(formatAbsolute(T0), { exact: false })).toBeInTheDocument()

    const e4 = within(supporting).getByRole('button', { name: /^Open source E-004:/ }).parentElement as HTMLElement
    expect(within(e4).getByText('Something a person said')).toBeInTheDocument()
    expect(within(e4).getByText('Head of billing, in person')).toBeInTheDocument()

    const contradicting = within(drawer).getByRole('region', { name: 'Sources that contradict it' })
    expect(within(contradicting).getByText('Nothing contradicts this yet.')).toBeInTheDocument()
  })

  it('pushes the evidence drawer with Back, returns to the claim and closes the stack with Esc', async () => {
    const user = userEvent.setup()
    renderDetail()
    const row = await openClaimButton('C-001')
    await user.click(row)
    await user.click(
      within(screen.getByRole('dialog')).getByRole('button', { name: 'Open source E-001: Notice dated October 4: 1,900 customers were charged twice' }),
    )

    const evidenceDrawer = screen.getByRole('dialog', { name: 'Source E-001' })
    expect(within(evidenceDrawer).getByRole('heading', { name: 'Source E-001' })).toHaveFocus()
    await user.click(within(evidenceDrawer).getByRole('button', { name: 'Back to C-001' }))

    expect(screen.getByRole('heading', { level: 2, name: 'Finding C-001' })).toHaveFocus()
    expect(screen.queryByRole('button', { name: /^Back to/ })).toBeNull()

    await user.click(
      within(screen.getByRole('dialog')).getByRole('button', { name: 'Open source E-004: Head of billing: the mistake was on October 3 only' }),
    )
    await user.keyboard('{Escape}')
    expect(screen.queryByRole('dialog')).toBeNull()
    expect(row).toHaveFocus()
  })

  it('shows the empty line for a hypothesis with no linked evidence', async () => {
    const user = userEvent.setup()
    renderDetail()
    // H-001 is open, so its row sits under Not tested yet.
    const explain = await screen.findByRole('region', { name: 'Not tested yet' })
    await user.click(within(explain).getByRole('button', { name: /^Open finding H-001:/ }))
    const drawer = screen.getByRole('dialog', { name: 'Finding H-001' })
    expect(within(drawer).getByText('This finding has no sources yet.')).toBeInTheDocument()
    expect(within(drawer).queryByRole('region', { name: 'Sources that contradict it' })).toBeNull()
  })

  it('hides the Status and Verification selects behind More options', async () => {
    const user = userEvent.setup()
    renderDetail()
    await user.click(await openClaimButton('C-001'))
    const status = within(screen.getByRole('dialog', { name: 'Finding C-001' })).getByRole('region', {
      name: 'Check this finding',
    })
    expect(within(status).queryByLabelText('Status')).not.toBeInTheDocument()
    expect(within(status).queryByLabelText('Label')).not.toBeInTheDocument()
    await user.click(within(status).getByRole('button', { name: 'More options', expanded: false }))
    expect(within(status).getByLabelText('Status')).toBeInTheDocument()
    expect(within(status).getByLabelText('Label')).toBeInTheDocument()
    await user.click(within(status).getByRole('button', { name: 'Fewer options', expanded: true }))
    expect(within(status).queryByLabelText('Status')).not.toBeInTheDocument()
    expect(within(status).queryByLabelText('Label')).not.toBeInTheDocument()
  })

  it('offers the finding that replaced it with a button that pushes the replacement', async () => {
    const user = userEvent.setup()
    renderDetail()
    await user.click(await screen.findByRole('button', { name: 'Show 1 replaced finding' }))
    await user.click(await openClaimButton('C-003'))
    const drawer = screen.getByRole('dialog', { name: 'Finding C-003' })
    const status = within(drawer).getByRole('region', { name: 'Check this finding' })
    await user.click(within(status).getByRole('button', { name: 'Open the finding that replaced it C-002' }))
    expect(screen.getByRole('dialog', { name: 'Finding C-002' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Back to C-003' })).toBeInTheDocument()
  })
})
