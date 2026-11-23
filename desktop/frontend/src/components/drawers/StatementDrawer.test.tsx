import { render, screen, waitFor, within } from '@testing-library/react'
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
import { CONTRADICTING_TITLE, DECIDE_TITLES, FINDING_CHANGED_COPY, REPLACE_ACTION, RULE_OUT_ACTION, STILL_HOLDS_ACTION, SUPPORTING_TITLE } from './StatementDrawer'

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

    const supporting = within(drawer).getByRole('region', { name: SUPPORTING_TITLE })
    const blocks = supporting.querySelectorAll('[data-ref]')
    expect(Array.from(blocks).map((b) => b.getAttribute('data-ref'))).toEqual(['E-001', 'E-004'])

    const e1 = within(supporting).getByRole('button', { name: 'Open source E-001: Notice dated October 4: 1,900 customers were charged twice' })
      .parentElement as HTMLElement
    expect(within(e1).getByText('Notice dated October 4: 1,900 customers were charged twice')).toBeInTheDocument()
    expect(within(e1).getByText(/charged 1,900 customers twice/)).toBeInTheDocument()
    expect(within(e1).getByText('Shared drive, Billing, notice-2026-10-04.pdf')).toBeInTheDocument()
    // Where it came from and its date are not shown: Where to find it says enough.
    expect(within(e1).queryByText('Email from the billing team')).toBeNull()
    expect(within(e1).queryByText(formatAbsolute(T0), { exact: false })).toBeNull()

    const contradicting = within(drawer).getByRole('region', { name: CONTRADICTING_TITLE })
    expect(within(contradicting).getByText('Nothing goes against this yet.')).toBeInTheDocument()
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
    // H-001 is open, so its row sits under Still to find out.
    const explain = await screen.findByRole('region', { name: 'Still to find out' })
    await user.click(within(explain).getByRole('button', { name: /^Open finding H-001:/ }))
    const drawer = screen.getByRole('dialog', { name: 'Finding H-001' })
    expect(within(drawer).getByText('This finding has no sources yet.')).toBeInTheDocument()
    expect(within(drawer).queryByRole('region', { name: CONTRADICTING_TITLE })).toBeNull()
  })

  it('offers no check of its own, More options, Status or Label on a finding', async () => {
    const user = userEvent.setup()
    renderDetail()
    await user.click(await openClaimButton('C-002'))
    const status = within(screen.getByRole('dialog', { name: 'Finding C-002' })).getByRole('region', {
      name: DECIDE_TITLES.standing,
    })
    expect(within(status).getAllByRole('button').map((b) => b.textContent)).toEqual([RULE_OUT_ACTION, REPLACE_ACTION])
    expect(screen.queryByRole('button', { name: 'I checked this' })).toBeNull()
    expect(screen.queryByRole('button', { name: 'Sources disagree' })).toBeNull()
    expect(within(status).queryByRole('button', { name: 'More options' })).toBeNull()
    expect(within(status).queryByLabelText('Status')).toBeNull()
    expect(within(status).queryByLabelText('Label')).toBeNull()
  })

  it('takes in the person own save without warning that the finding changed', async () => {
    const user = userEvent.setup()
    let current: InvestigationDetail = { ...detail, review_bases: [{ ref: 'C-004', basis: 'as-read' }] }
    renderDetail(current)
    mocks.api.getInvestigationDetail = vi.fn(async () => current)
    mocks.api.updateClaim = vi.fn(async () => {
      current = { ...current, review_bases: [{ ref: 'C-004', basis: 'after-my-edit' }] }
      return detail.claims[3]
    })
    await user.click(await openClaimButton('C-004'))
    await user.click(screen.getByRole('button', { name: 'Edit finding' }))
    await user.type(screen.getByLabelText('Finding'), ' Mine.')
    await user.click(screen.getByRole('button', { name: 'Save finding' }))
    await waitFor(() => expect(screen.queryByLabelText('Finding')).toBeNull())
    await waitFor(() => expect(mocks.api.getInvestigationDetail).toHaveBeenCalled())
    expect(screen.queryByText(FINDING_CHANGED_COPY)).toBeNull()
    expect(screen.getByRole('button', { name: STILL_HOLDS_ACTION })).toBeEnabled()
  })

  it('offers the finding that replaced it with a button that pushes the replacement', async () => {
    const user = userEvent.setup()
    renderDetail()
    await user.click(await screen.findByRole('button', { name: 'Show 1 replaced finding' }))
    await user.click(await openClaimButton('C-003'))
    const drawer = screen.getByRole('dialog', { name: 'Finding C-003' })
    const status = within(drawer).getByRole('region', { name: DECIDE_TITLES.replaced })
    await user.click(within(status).getByRole('button', { name: 'Open the finding that replaced it C-002' }))
    expect(screen.getByRole('dialog', { name: 'Finding C-002' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Back to C-003' })).toBeInTheDocument()
  })
})
