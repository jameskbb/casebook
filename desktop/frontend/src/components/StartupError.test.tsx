import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Api } from '../api/api'
import { backupInfo } from '../test/fixtures'
import { mockApi } from '../test/mockApi'
import App from '../App'
import { FOLDER_COPY, RECOVER_WARNING } from './StartupError'

const mocks = vi.hoisted(() => ({ api: {} as Api }))
vi.mock('../api/api', () => ({ api: mocks.api }))

const PATH = 'C:\\Users\\Ada\\AppData\\Local\\Casebook\\casebook.db'

describe('StartupError', () => {
  beforeEach(() => {
    Object.assign(mocks.api, mockApi())
  })

  it('tells the user to update Casebook when the data is too new', async () => {
    Object.assign(
      mocks.api,
      mockApi({
        startupState: async () => ({ ok: false, kind: 'schema_too_new', message: 'schema too new', db_path: PATH }),
      }),
    )
    const user = userEvent.setup()
    render(<App />)
    expect(await screen.findByRole('heading', { name: 'This data needs a newer Casebook' })).toBeInTheDocument()
    expect(
      screen.getByText(
        `The data file at ${PATH} was saved by a newer version of Casebook. Update Casebook to open it. Nothing has been changed.`,
      ),
    ).toBeInTheDocument()
    const buttons = screen.getAllByRole('button')
    expect(buttons.map((b) => b.textContent)).toEqual(['Open data folder', 'Copy path', 'Close Casebook'])
    expect(screen.queryByRole('button', { name: /Restore from backup/ })).not.toBeInTheDocument()
    expect(screen.queryByRole('navigation')).not.toBeInTheDocument()
    expect(mocks.api.listInvestigations).not.toHaveBeenCalled()

    await user.click(screen.getByRole('button', { name: 'Close Casebook' }))
    expect(mocks.api.quit).toHaveBeenCalledTimes(1)
  })

  it('offers the pre-upgrade backup when going back to this version', async () => {
    Object.assign(
      mocks.api,
      mockApi({
        startupState: async () => ({
          ok: false,
          kind: 'schema_too_new',
          message: 'schema too new',
          db_path: PATH,
          recoverable: true,
        }),
      }),
    )
    render(<App />)
    expect(await screen.findByRole('heading', { name: 'This data needs a newer Casebook' })).toBeInTheDocument()
    expect(
      screen.getByText(
        `The data file at ${PATH} was saved by a newer version of Casebook. Update Casebook to open it. Nothing has been changed. To keep using this version instead, restore the casebook-pre-upgrade backup from the backups folder next to the data file.`,
      ),
    ).toBeInTheDocument()
    const buttons = screen.getAllByRole('button')
    expect(buttons.map((b) => b.textContent)).toEqual([
      'Open data folder',
      'Copy path',
      'Restore from backup...',
      'Close Casebook',
    ])
  })

  it('shows the engine message for any other failure', async () => {
    Object.assign(
      mocks.api,
      mockApi({
        startupState: async () => ({ ok: false, kind: 'other', message: 'open C:\\x\\casebook.db: access is denied', db_path: PATH }),
      }),
    )
    render(<App />)
    expect(await screen.findByRole('heading', { name: 'Casebook could not open its data' })).toBeInTheDocument()
    expect(
      screen.getByText(
        'Open C:\\x\\casebook.db: access is denied. Close Casebook and try again.',
      ),
    ).toBeInTheDocument()
    // Without recoverable the engine said a backup cannot help.
    expect(screen.queryByRole('button', { name: /Restore from backup/ })).not.toBeInTheDocument()
  })

  it('opens the data folder and copies the path without an open database (AR-11)', async () => {
    Object.assign(
      mocks.api,
      mockApi({
        startupState: async () => ({ ok: false, kind: 'other', message: 'cannot open', db_path: PATH, recoverable: true }),
      }),
    )
    const user = userEvent.setup()
    render(<App />)
    expect(await screen.findByText(PATH)).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Open data folder' }))
    expect(mocks.api.openDataFolder).toHaveBeenCalledTimes(1)
    await user.click(screen.getByRole('button', { name: 'Copy path' }))
    expect(mocks.api.copyDataLocation).toHaveBeenCalledTimes(1)
    expect(await screen.findByText('Path copied to the clipboard.')).toBeInTheDocument()
  })

  it('says so when the folder cannot be opened', async () => {
    Object.assign(
      mocks.api,
      mockApi({
        startupState: async () => ({ ok: false, kind: 'other', message: 'cannot open', db_path: PATH }),
        openDataFolder: async () => Promise.reject(new Error('{"kind":"other","field":"folder","message":"no"}')),
      }),
    )
    const user = userEvent.setup()
    render(<App />)
    await user.click(await screen.findByRole('button', { name: 'Open data folder' }))
    expect(await screen.findByText(FOLDER_COPY)).toBeInTheDocument()
  })

  it('restores a backup in place of the file it could not open, then starts normally (AR-11)', async () => {
    let opened = false
    Object.assign(
      mocks.api,
      mockApi({
        startupState: async () =>
          opened
            ? { ok: true, kind: '', message: '', db_path: PATH }
            : { ok: false, kind: 'other', message: 'cannot open', db_path: PATH, recoverable: true },
        recoverFromBackup: async () => {
          opened = true
          return {
            source_path: 'C:\\b.db',
            safety_copy_path: 'C:\\Casebook\\backups\\casebook-unopened-1.db',
            source_schema_version: 3,
            counts: { investigations: 1, evidence: 0, claims: 0, hypotheses: 0, questions: 0, claim_links: 0, hypothesis_links: 0, activity: 1 },
          }
        },
      }),
    )
    const user = userEvent.setup()
    render(<App />)
    await user.click(await screen.findByRole('button', { name: 'Restore from backup...' }))
    const dialog = await screen.findByRole('dialog')
    expect(within(dialog).getByText(RECOVER_WARNING)).toBeInTheDocument()
    await user.click(within(dialog).getByRole('button', { name: 'Replace all data' }))
    expect(mocks.api.recoverFromBackup).toHaveBeenCalledWith(backupInfo.path)
    const done = await screen.findByRole('dialog', { name: 'Restore complete' })
    expect(within(done).getByText(/The file it could not open was moved to C:\\Casebook\\backups\\casebook-unopened-1.db/)).toBeInTheDocument()
    await user.click(within(done).getByRole('button', { name: 'Done' }))
    expect(await screen.findByRole('navigation')).toBeInTheDocument()
    expect(mocks.api.startupState).toHaveBeenCalledTimes(2)
  })

  it('reports a refused backup and changes nothing', async () => {
    Object.assign(
      mocks.api,
      mockApi({
        startupState: async () => ({ ok: false, kind: 'other', message: 'cannot open', db_path: PATH, recoverable: true }),
        pickBackupToRecover: async () =>
          Promise.reject(new Error('{"kind":"conflict","message":"That is the data file Casebook could not open. Choose a backup file instead."}')),
      }),
    )
    const user = userEvent.setup()
    render(<App />)
    await user.click(await screen.findByRole('button', { name: 'Restore from backup...' }))
    expect(await screen.findByRole('alert')).toHaveTextContent(
      'That is the data file Casebook could not open. Choose a backup file instead. Nothing was changed.',
    )
    expect(mocks.api.recoverFromBackup).not.toHaveBeenCalled()
  })
})
