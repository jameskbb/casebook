import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Api } from '../api/api'
import { FILE_COPY } from '../api/errors'
import { backupInfo } from '../test/fixtures'
import { mockApi } from '../test/mockApi'
import App from '../App'
import { FOLDER_COPY, RECOVER_WARNING } from './StartupError'

const mocks = vi.hoisted(() => ({ api: {} as Api }))
vi.mock('../api/api', () => ({ api: mocks.api }))

const PATH = 'C:\\Users\\Ada\\AppData\\Local\\Casebook\\casebook.db'
const TOO_NEW = 'Your questions were saved by a newer Casebook'
const NOT_OPENED = 'Casebook could not open your questions'

describe('StartupError', () => {
  beforeEach(() => {
    Object.assign(mocks.api, mockApi())
  })

  it('tells the user to update Casebook when the questions are too new', async () => {
    Object.assign(
      mocks.api,
      mockApi({
        startupState: async () => ({ ok: false, kind: 'schema_too_new', message: 'schema too new', db_path: PATH }),
      }),
    )
    const user = userEvent.setup()
    render(<App />)
    expect(await screen.findByRole('heading', { name: TOO_NEW })).toBeInTheDocument()
    expect(screen.getByText('Update Casebook to open them. Nothing was changed.')).toBeInTheDocument()
    expect(screen.queryByText(/schema too new/)).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Bring back a backup/ })).not.toBeInTheDocument()
    expect(screen.queryByRole('navigation')).not.toBeInTheDocument()
    expect(mocks.api.listInvestigations).not.toHaveBeenCalled()

    await user.click(screen.getByRole('button', { name: 'Close Casebook' }))
    expect(mocks.api.quit).toHaveBeenCalledTimes(1)
  })

  it('offers the backup made before the update when going back to this version', async () => {
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
    expect(await screen.findByRole('heading', { name: TOO_NEW })).toBeInTheDocument()
    expect(
      screen.getByText(
        'Update Casebook to open them. Nothing was changed. To keep using this version, bring back the backup Casebook made before the update.',
      ),
    ).toBeInTheDocument()
    const buttons = screen.getAllByRole('button')
    expect(buttons.map((b) => b.textContent)).toEqual([
      'Open the folder',
      'Copy location',
      'Bring back a backup...',
      'Close Casebook',
    ])
  })

  it('never shows the engine message for any other failure', async () => {
    Object.assign(
      mocks.api,
      mockApi({
        startupState: async () => ({ ok: false, kind: 'other', message: 'open C:\\x\\casebook.db: access is denied', db_path: PATH }),
      }),
    )
    render(<App />)
    expect(await screen.findByRole('heading', { name: NOT_OPENED })).toBeInTheDocument()
    expect(screen.getByText('Close Casebook and try again.')).toBeInTheDocument()
    expect(screen.getByRole('main')).not.toHaveTextContent(/access is denied/i)
    expect(screen.getByRole('main')).not.toHaveTextContent('C:\\x\\casebook.db')
    // Without recoverable the engine said a backup cannot help.
    expect(screen.queryByRole('button', { name: /Bring back a backup/ })).not.toBeInTheDocument()
  })

  it('points to a backup when one can help', async () => {
    Object.assign(
      mocks.api,
      mockApi({
        startupState: async () => ({ ok: false, kind: 'other', message: 'cannot open', db_path: PATH, recoverable: true }),
      }),
    )
    render(<App />)
    expect(await screen.findByRole('heading', { name: NOT_OPENED })).toBeInTheDocument()
    expect(screen.getByText('Close Casebook and try again. If this keeps happening, bring back a backup.')).toBeInTheDocument()
    expect(screen.queryByText(/cannot open/)).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Bring back a backup...' })).toBeInTheDocument()
  })

  it('keeps the file location under Technical details, with the folder and copy actions', async () => {
    Object.assign(
      mocks.api,
      mockApi({
        startupState: async () => ({ ok: false, kind: 'other', message: 'cannot open', db_path: PATH, recoverable: true }),
      }),
    )
    const user = userEvent.setup()
    render(<App />)
    const path = await screen.findByText(PATH)
    const details = path.closest('details')!
    expect(details.querySelector('summary')).toHaveTextContent('Technical details')
    expect(path).not.toBeVisible()
    // Outside the disclosure nothing names the file.
    const outside = screen.getByRole('main').cloneNode(true) as HTMLElement
    outside.querySelector('details')!.remove()
    expect(outside).not.toHaveTextContent('casebook.db')

    await user.click(screen.getByText('Technical details'))
    expect(path).toBeVisible()
    expect(within(details).getByText('Where your questions are kept')).toBeVisible()
    await user.click(within(details).getByRole('button', { name: 'Open the folder' }))
    expect(mocks.api.openDataFolder).toHaveBeenCalledTimes(1)
    await user.click(within(details).getByRole('button', { name: 'Copy location' }))
    expect(mocks.api.copyDataLocation).toHaveBeenCalledTimes(1)
    expect(await screen.findByText('Location copied.')).toBeInTheDocument()
  })

  it('shows no Technical details when the location is not known', async () => {
    Object.assign(
      mocks.api,
      mockApi({ startupState: async () => ({ ok: false, kind: 'other', message: 'cannot open', db_path: '' }) }),
    )
    render(<App />)
    expect(await screen.findByRole('heading', { name: NOT_OPENED })).toBeInTheDocument()
    expect(screen.queryByText('Technical details')).not.toBeInTheDocument()
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
    await user.click(await screen.findByText('Technical details'))
    await user.click(screen.getByRole('button', { name: 'Open the folder' }))
    expect(await screen.findByText(FOLDER_COPY)).toBeInTheDocument()
  })

  it('brings back a backup in place of the file it could not open, then starts normally', async () => {
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
    await user.click(await screen.findByRole('button', { name: 'Bring back a backup...' }))
    const dialog = await screen.findByRole('dialog', { name: 'Replace everything in Casebook with this backup?' })
    expect(within(dialog).getByText(RECOVER_WARNING)).toBeInTheDocument()
    expect(mocks.api.recoverFromBackup).not.toHaveBeenCalled()
    await user.click(within(dialog).getByRole('button', { name: 'Replace everything' }))
    expect(mocks.api.recoverFromBackup).toHaveBeenCalledWith(backupInfo.path)
    const done = await screen.findByRole('dialog', { name: 'Your backup is open' })
    expect(within(done).getByText(/The file it could not open was kept at C:\\Casebook\\backups\\casebook-unopened-1.db/)).toBeInTheDocument()
    await user.click(within(done).getByRole('button', { name: 'Copy location' }))
    expect(mocks.api.copyText).toHaveBeenCalledWith('C:\\Casebook\\backups\\casebook-unopened-1.db')
    await user.click(within(done).getByRole('button', { name: 'Done' }))
    expect(await screen.findByRole('navigation')).toBeInTheDocument()
    expect(mocks.api.startupState).toHaveBeenCalledTimes(2)
  })

  it('does nothing when no backup is chosen', async () => {
    Object.assign(
      mocks.api,
      mockApi({
        startupState: async () => ({ ok: false, kind: 'other', message: 'cannot open', db_path: PATH, recoverable: true }),
        pickBackupToRecover: async () => ({ cancelled: true, path: '', file_name: '', info: backupInfo }),
      }),
    )
    const user = userEvent.setup()
    render(<App />)
    await user.click(await screen.findByRole('button', { name: 'Bring back a backup...' }))
    await new Promise((r) => setTimeout(r, 20))
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    expect(mocks.api.recoverFromBackup).not.toHaveBeenCalled()
  })

  it('reports a refused backup in plain words and changes nothing', async () => {
    Object.assign(
      mocks.api,
      mockApi({
        startupState: async () => ({ ok: false, kind: 'other', message: 'cannot open', db_path: PATH, recoverable: true }),
        pickBackupToRecover: async () =>
          Promise.reject(new Error('{"kind":"conflict","message":"conflict: not a valid Casebook backup (unexpected table notes)"}')),
      }),
    )
    const user = userEvent.setup()
    render(<App />)
    await user.click(await screen.findByRole('button', { name: 'Bring back a backup...' }))
    const alert = await screen.findByRole('alert')
    expect(alert).toHaveTextContent(`${FILE_COPY} Nothing was changed.`)
    expect(alert).not.toHaveTextContent(/unexpected table|conflict/)
    expect(mocks.api.recoverFromBackup).not.toHaveBeenCalled()
  })

  it('never shows the engine text when the chosen file is the one that would not open', async () => {
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
    await user.click(await screen.findByRole('button', { name: 'Bring back a backup...' }))
    const alert = await screen.findByRole('alert')
    expect(alert).toHaveTextContent(/ Nothing was changed\.$/)
    expect(alert).not.toHaveTextContent(/data file/)
    expect(mocks.api.recoverFromBackup).not.toHaveBeenCalled()
  })
})
