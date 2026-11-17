import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Api } from '../../api/api'
import { AppDataProvider, useAppData } from '../../state/appData'
import { backupInfo, DB_PATH } from '../../test/fixtures'
import { cancelledImport, importChangedRef, importSameRef, LONG_DB_PATH } from '../../test/fixtures.dataops'
import { mockApi } from '../../test/mockApi'
import { truncateMiddle } from '../../lib/truncatePath'
import { ToastProvider } from '../ui/Toaster'
import App from '../../App'
import { DataOpsProvider } from './DataOps'
import { SidebarFooter } from './SidebarFooter'

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

/** Opens the Data menu and chooses an item. */
async function choose(user: ReturnType<typeof userEvent.setup>, name: string) {
  await user.click(screen.getByRole('button', { name: 'Data' }))
  await user.click(await screen.findByRole('menuitem', { name }))
}

async function ready() {
  await waitFor(() => expect(screen.getByTestId('selected')).toHaveTextContent('INV-0001'))
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
    await choose(user, 'Import case...')
    await waitFor(() => expect(mocks.api.importInvestigation).toHaveBeenCalled())
    await new Promise((r) => setTimeout(r, 20))
    expect(screen.queryByRole('status')).not.toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    expect(screen.getByTestId('selected')).toHaveTextContent('INV-0001')
  })

  it('opens an imported investigation that kept its ref', async () => {
    Object.assign(mocks.api, { importInvestigation: vi.fn(async () => importSameRef) })
    renderFooter()
    const user = await ready()
    await choose(user, 'Import case...')
    expect(await screen.findByText('Imported "September cloud bill up 38%" as INV-0004.')).toBeInTheDocument()
    await waitFor(() => expect(screen.getByTestId('selected')).toHaveTextContent('INV-0004'))
  })

  it('says why the ref changed', async () => {
    Object.assign(mocks.api, { importInvestigation: vi.fn(async () => importChangedRef) })
    renderFooter()
    const user = await ready()
    await choose(user, 'Import case...')
    expect(
      await screen.findByText('Imported "September cloud bill up 38%" as INV-0004 because INV-0001 was already in use.'),
    ).toBeInTheDocument()
    await waitFor(() => expect(screen.getByTestId('selected')).toHaveTextContent('INV-0004'))
  })

  it('explains a file that cannot be imported', async () => {
    Object.assign(mocks.api, {
      importInvestigation: vi.fn(async () => {
        throw JSON.stringify({ kind: 'validation', field: 'version', message: 'unsupported version 2' })
      }),
    })
    renderFooter()
    const user = await ready()
    await choose(user, 'Import case...')
    const dialog = await screen.findByRole('dialog', { name: "This file can't be imported" })
    expect(dialog).toHaveTextContent('version: unsupported version 2. Nothing was added.')
    await user.click(within(dialog).getByRole('button', { name: 'Choose another file...' }))
    await waitFor(() => expect(mocks.api.importInvestigation).toHaveBeenCalledTimes(2))
    const again = await screen.findByRole('dialog', { name: "This file can't be imported" })
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
    await choose(user, 'Back up all data...')
    await waitFor(() => expect(mocks.api.backUp).toHaveBeenCalled())
    await new Promise((r) => setTimeout(r, 20))
    expect(screen.queryByText(/Backup saved/)).not.toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('says where the backup went and copies the path', async () => {
    renderFooter()
    const user = await ready()
    await choose(user, 'Back up all data...')
    const toast = (await screen.findByText(`Backup saved to ${backupInfo.path}.`)).closest('.toast') as HTMLElement
    await user.click(within(toast).getByRole('button', { name: 'Copy path' }))
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
    await choose(user, 'Back up all data...')
    expect(await screen.findByRole('alert')).toHaveTextContent('A file with that name already exists. Choose a new name.')
  })

  it('reports a native dialog failure', async () => {
    Object.assign(mocks.api, {
      backUp: vi.fn(async () => {
        throw JSON.stringify({ kind: 'other', field: 'dialog', message: 'boom' })
      }),
    })
    renderFooter()
    const user = await ready()
    await choose(user, 'Back up all data...')
    expect(await screen.findByRole('alert')).toHaveTextContent('Could not open the file window. Try again.')
  })
})

describe('SidebarFooter data file', () => {
  async function openData() {
    const user = userEvent.setup()
    renderFooter()
    await waitFor(() => expect(screen.getByTestId('selected')).toHaveTextContent('INV-0001'))
    await user.click(screen.getByRole('button', { name: 'Data' }))
    return user
  }

  it('keeps the footer to the Data menu and the theme switch', async () => {
    Object.assign(mocks.api, mockApi())
    renderFooter()
    await waitFor(() => expect(screen.getByTestId('selected')).toHaveTextContent('INV-0001'))
    expect(screen.getByRole('button', { name: 'Data' })).toHaveAttribute('aria-haspopup', 'menu')
    expect(screen.getByRole('radiogroup', { name: 'Theme' })).toBeInTheDocument()
    expect(screen.queryByRole('menuitem')).not.toBeInTheDocument()
  })

  it('lists the data actions, the data file and Connect AI clients in the Data menu', async () => {
    Object.assign(mocks.api, mockApi())
    await openData()
    const menu = await screen.findByRole('menu', { name: 'Data' })
    const names = within(menu)
      .getAllByRole('menuitem')
      .map((el) => el.textContent)
    expect(names).toEqual([
      'Import case...',
      'Back up all data...',
      'Restore from backup...',
      `Data file${truncateMiddle(DB_PATH)}`,
      'Connect AI clients...',
      'Check for updates...',
      'Versionv0.1.0',
    ])
  })

  it('opens Check for updates from the Data menu without checking', async () => {
    Object.assign(mocks.api, mockApi())
    const user = await openData()
    await user.click(await screen.findByRole('menuitem', { name: 'Check for updates...' }))
    const dialog = await screen.findByRole('dialog', { name: 'Check for updates' })
    expect(within(dialog).getByRole('button', { name: 'Check now' })).toBeInTheDocument()
    expect(mocks.api.checkForUpdates).not.toHaveBeenCalled()
  })

  it('copies the version from the Data menu', async () => {
    Object.assign(mocks.api, mockApi())
    const user = await openData()
    await user.click(await screen.findByRole('menuitem', { name: 'Casebook version v0.1.0. Copy it' }))
    await waitFor(() => expect(mocks.api.copyText).toHaveBeenCalledWith('Casebook v0.1.0'))
  })

  it('shows a short path with the full path in its title and name', async () => {
    Object.assign(mocks.api, mockApi({ dataLocation: async () => LONG_DB_PATH }))
    await openData()
    const item = await screen.findByRole('menuitem', { name: new RegExp('^Data file') })
    expect(item).toHaveTextContent(truncateMiddle(LONG_DB_PATH))
    expect(item).toHaveAttribute('title', LONG_DB_PATH)
    expect(item).toHaveAccessibleName(`Data file ${LONG_DB_PATH}. Copy the path`)
  })

  it('shows a short path unchanged', async () => {
    Object.assign(mocks.api, mockApi({ dataLocation: async () => 'D:\\Casebook\\casebook.db' }))
    await openData()
    expect(await screen.findByRole('menuitem', { name: /^Data file/ })).toHaveTextContent('D:\\Casebook\\casebook.db')
  })

  it('shortens the default per-user path', async () => {
    Object.assign(mocks.api, mockApi())
    await openData()
    const item = await screen.findByRole('menuitem', { name: /^Data file/ })
    expect(item).toHaveTextContent('C:\\Users\\Ada...a\\Local\\Casebook\\casebook.db')
    expect(item).toHaveAttribute('title', DB_PATH)
  })

  it('copies the full path when the data file is chosen', async () => {
    Object.assign(mocks.api, mockApi())
    const user = await openData()
    await user.click(await screen.findByRole('menuitem', { name: /^Data file/ }))
    expect(mocks.api.copyText).toHaveBeenCalledWith(DB_PATH)
    expect(await screen.findByText('Path copied to the clipboard.')).toBeInTheDocument()
  })
})

describe('Empty main pane import', () => {
  it('runs the same import flow from the empty state', async () => {
    Object.assign(
      mocks.api,
      mockApi({ listInvestigations: async () => [], importInvestigation: async () => importSameRef }),
    )
    const user = userEvent.setup()
    render(<App />)
    const main = await screen.findByRole('main')
    await user.click(await within(main).findByRole('button', { name: 'Import case...' }))
    expect(mocks.api.importInvestigation).toHaveBeenCalledTimes(1)
    expect(await screen.findByText('Imported "September cloud bill up 38%" as INV-0004.')).toBeInTheDocument()
  })
})
