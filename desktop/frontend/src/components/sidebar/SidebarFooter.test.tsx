import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Api } from '../../api/api'
import { DIALOG_COPY, FILE_COPY } from '../../api/errors'
import { AppDataProvider, useAppData } from '../../state/appData'
import { backupInfo, DB_PATH } from '../../test/fixtures'
import { cancelledImport, importChangedRef, importSameRef, LONG_DB_PATH } from '../../test/fixtures.dataops'
import { mockApi } from '../../test/mockApi'
import { ToastProvider } from '../ui/Toaster'
import App from '../../App'
import { BACKUP_EXISTS_COPY, DataOpsProvider } from './DataOps'
import { OPEN_SENT_QUESTION_LABEL, SETTINGS_LABEL, SHOW_IDS_LABEL, SidebarFooter } from './SidebarFooter'

const mocks = vi.hoisted(() => ({ api: {} as Api }))
vi.mock('../../api/api', () => ({ api: mocks.api }))

function Probe() {
  const { selectedRef } = useAppData()
  return <p data-testid="selected">{selectedRef ?? ''}</p>
}

function renderFooter() {
  return render(
    <ToastProvider>
      <AppDataProvider>
        <DataOpsProvider>
          <SidebarFooter />
          <Probe />
        </DataOpsProvider>
      </AppDataProvider>
    </ToastProvider>,
  )
}

/** Opens the Settings menu and chooses an item. */
async function choose(user: ReturnType<typeof userEvent.setup>, name: string) {
  await user.click(screen.getByRole('button', { name: SETTINGS_LABEL }))
  await user.click(await screen.findByRole('menuitem', { name }))
}

/** Waits for the list to load. Nothing is selected at launch. */
async function ready() {
  await waitFor(() => expect(mocks.api.listInvestigations).toHaveBeenCalled())
  return userEvent.setup()
}

describe('SidebarFooter import', () => {
  beforeEach(() => {
    Object.assign(mocks.api, mockApi())
  })

  it('shows nothing when the dialog is cancelled', async () => {
    Object.assign(mocks.api, { importInvestigation: vi.fn(async () => cancelledImport) })
    renderFooter()
    const user = await ready()
    await choose(user, OPEN_SENT_QUESTION_LABEL)
    await waitFor(() => expect(mocks.api.importInvestigation).toHaveBeenCalled())
    await new Promise((r) => setTimeout(r, 20))
    expect(screen.queryByRole('status')).not.toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    expect(screen.getByTestId('selected')).toHaveTextContent('')
  })

  it('opens the question it added from a file', async () => {
    Object.assign(mocks.api, { importInvestigation: vi.fn(async () => importSameRef) })
    renderFooter()
    const user = await ready()
    await choose(user, OPEN_SENT_QUESTION_LABEL)
    expect(await screen.findByText('Added "Why did calls go up 30% in October when sales stayed the same?" to your questions.')).toBeInTheDocument()
    await waitFor(() => expect(screen.getByTestId('selected')).toHaveTextContent('INV-0004'))
  })

  it('says the same, with no codes, when the question got a new code', async () => {
    Object.assign(mocks.api, { importInvestigation: vi.fn(async () => importChangedRef) })
    renderFooter()
    const user = await ready()
    await choose(user, OPEN_SENT_QUESTION_LABEL)
    expect(await screen.findByText('Added "Why did calls go up 30% in October when sales stayed the same?" to your questions.')).toBeInTheDocument()
    await waitFor(() => expect(screen.getByTestId('selected')).toHaveTextContent('INV-0004'))
  })

  it('explains a file that cannot be opened, without the engine text', async () => {
    Object.assign(mocks.api, {
      importInvestigation: vi.fn(async () => {
        throw JSON.stringify({ kind: 'validation', field: 'version', message: 'unsupported version 2' })
      }),
    })
    renderFooter()
    const user = await ready()
    await choose(user, OPEN_SENT_QUESTION_LABEL)
    const dialog = await screen.findByRole('dialog', { name: 'Casebook cannot open this file' })
    expect(dialog).toHaveTextContent(`${FILE_COPY} Nothing was added.`)
    expect(dialog).not.toHaveTextContent('unsupported version 2')
    await user.click(within(dialog).getByRole('button', { name: 'Choose another file...' }))
    await waitFor(() => expect(mocks.api.importInvestigation).toHaveBeenCalledTimes(2))
    const again = await screen.findByRole('dialog', { name: 'Casebook cannot open this file' })
    await user.click(within(again).getAllByRole('button', { name: 'Close' }).at(-1)!)
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
  })
})

describe('SidebarFooter backup', () => {
  beforeEach(() => {
    Object.assign(mocks.api, mockApi())
  })

  it('shows nothing when the dialog is cancelled', async () => {
    Object.assign(mocks.api, { backUp: vi.fn(async () => ({ cancelled: true, info: backupInfo })) })
    renderFooter()
    const user = await ready()
    await choose(user, 'Save a backup...')
    await waitFor(() => expect(mocks.api.backUp).toHaveBeenCalled())
    await new Promise((r) => setTimeout(r, 20))
    expect(screen.queryByText(/Backup saved/)).not.toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('names only the backup file, not its folder, and copies the full location', async () => {
    renderFooter()
    const user = await ready()
    await choose(user, 'Save a backup...')
    const toast = (await screen.findByText('Backup saved as casebook-backup-2026-11-06.db.')).closest('.toast') as HTMLElement
    expect(toast).not.toHaveTextContent('Documents')
    await user.click(within(toast).getByRole('button', { name: 'Copy location' }))
    expect(mocks.api.copyText).toHaveBeenCalledWith(backupInfo.path)
  })

  it('refuses to overwrite an existing file', async () => {
    Object.assign(mocks.api, {
      backUp: vi.fn(async () => {
        throw JSON.stringify({ kind: 'conflict', message: 'conflict: backup destination exists' })
      }),
    })
    renderFooter()
    const user = await ready()
    await choose(user, 'Save a backup...')
    expect(await screen.findByRole('alert')).toHaveTextContent(BACKUP_EXISTS_COPY)
  })

  it('reports a native dialog failure', async () => {
    Object.assign(mocks.api, {
      backUp: vi.fn(async () => {
        throw JSON.stringify({ kind: 'other', field: 'dialog', message: 'boom' })
      }),
    })
    renderFooter()
    const user = await ready()
    await choose(user, 'Save a backup...')
    const alert = await screen.findByRole('alert')
    expect(alert).toHaveTextContent(DIALOG_COPY)
    expect(alert).not.toHaveTextContent('boom')
  })
})

describe('SidebarFooter Settings menu', () => {
  beforeEach(() => {
    Object.assign(mocks.api, mockApi())
  })

  async function openSettings() {
    const user = userEvent.setup()
    renderFooter()
    await waitFor(() => expect(mocks.api.listInvestigations).toHaveBeenCalled())
    await user.click(screen.getByRole('button', { name: SETTINGS_LABEL }))
    return user
  }

  it('keeps the footer to the Settings menu and the theme switch', async () => {
    renderFooter()
    await waitFor(() => expect(mocks.api.listInvestigations).toHaveBeenCalled())
    expect(screen.getByRole('button', { name: 'Settings' })).toHaveAttribute('aria-haspopup', 'menu')
    expect(screen.getByRole('radiogroup', { name: 'Theme' })).toBeInTheDocument()
    expect(screen.queryByRole('menuitem')).not.toBeInTheDocument()
  })

  it('lists what a person does, then Technical, with no data file, codes or version item', async () => {
    await openSettings()
    const menu = await screen.findByRole('menu', { name: 'Settings' })
    const names = within(menu)
      .getAllByRole('menuitem')
      .map((el) => el.textContent)
    expect(names).toEqual([
      'Connect your AI...',
      'Open a question someone sent you...',
      'Save a backup...',
      'Bring back a backup...',
      'Look for updates...',
      'Technical...',
    ])
    expect(menu).not.toHaveTextContent(/Data file|codes|Version|v0\.1\.0/)
    expect(menu).not.toHaveTextContent(DB_PATH)
  })

  it('opens Look for updates without looking', async () => {
    const user = await openSettings()
    await user.click(await screen.findByRole('menuitem', { name: 'Look for updates...' }))
    const dialog = await screen.findByRole('dialog', { name: 'Look for updates' })
    expect(within(dialog).getByRole('button', { name: 'Look now' })).toBeInTheDocument()
    expect(mocks.api.checkForUpdates).not.toHaveBeenCalled()
  })

  it('opens Connect your AI', async () => {
    const user = await openSettings()
    await user.click(await screen.findByRole('menuitem', { name: 'Connect your AI...' }))
    expect(await screen.findByRole('dialog', { name: 'Connect your AI' })).toBeInTheDocument()
  })
})

describe('SidebarFooter Technical', () => {
  beforeEach(() => {
    Object.assign(mocks.api, mockApi())
    window.localStorage.removeItem('casebook.showIds')
    delete document.documentElement.dataset.showIds
  })

  async function openTechnical() {
    renderFooter()
    const user = await ready()
    await choose(user, 'Technical...')
    return { user, dialog: await screen.findByRole('dialog', { name: 'Technical' }) }
  }

  it('shows and hides the codes your AI uses, and remembers the choice', async () => {
    const { user, dialog } = await openTechnical()
    const toggle = within(dialog).getByRole('switch', { name: SHOW_IDS_LABEL })
    expect(toggle).toHaveAttribute('aria-checked', 'false')
    await user.click(toggle)
    expect(toggle).toHaveAttribute('aria-checked', 'true')
    expect(document.documentElement.dataset.showIds).toBe('true')
    expect(window.localStorage.getItem('casebook.showIds')).toBe('true')
    await user.click(toggle)
    expect(document.documentElement.dataset.showIds).toBeUndefined()
    expect(window.localStorage.getItem('casebook.showIds')).toBe('false')
  })

  it('shows where the questions are kept in full and copies the location', async () => {
    Object.assign(mocks.api, mockApi({ dataLocation: async () => LONG_DB_PATH }))
    const { user, dialog } = await openTechnical()
    const label = await within(dialog).findByText('Where your questions are kept')
    expect(label.nextElementSibling).toHaveTextContent(LONG_DB_PATH)
    await user.click(within(dialog).getByRole('button', { name: 'Copy location' }))
    expect(mocks.api.copyText).toHaveBeenCalledWith(LONG_DB_PATH)
    expect(await screen.findByText('Location copied.')).toBeInTheDocument()
  })

  it('shows a short version and copies the full one', async () => {
    Object.assign(mocks.api, mockApi({ appVersion: async () => 'v0.8.3-0.2026-abc' }))
    const { user, dialog } = await openTechnical()
    expect(await within(dialog).findByText('v0.8.3')).toBeInTheDocument()
    expect(dialog).not.toHaveTextContent('2026-abc')
    await user.click(within(dialog).getByRole('button', { name: 'Copy version' }))
    await waitFor(() => expect(mocks.api.copyText).toHaveBeenCalledWith('Casebook v0.8.3-0.2026-abc'))
    expect(await screen.findByText('Version copied.')).toBeInTheDocument()
  })

  it('gives focus back to the Settings button when it closes', async () => {
    const { user, dialog } = await openTechnical()
    await user.click(within(dialog).getByRole('button', { name: 'Done' }))
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    expect(screen.getByRole('button', { name: SETTINGS_LABEL })).toHaveFocus()
  })
})

describe('First-open screen import', () => {
  it('runs the same import flow from the first-open screen', async () => {
    Object.assign(
      mocks.api,
      mockApi({ listInvestigations: async () => [], importInvestigation: async () => importSameRef }),
    )
    const user = userEvent.setup()
    render(<App />)
    const main = await screen.findByRole('main')
    await user.click(await within(main).findByRole('button', { name: OPEN_SENT_QUESTION_LABEL }))
    expect(mocks.api.importInvestigation).toHaveBeenCalledTimes(1)
    expect(await screen.findByText('Added "Why did calls go up 30% in October when sales stayed the same?" to your questions.')).toBeInTheDocument()
  })
})
