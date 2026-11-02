import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Api } from '../../api/api'
import type { InvestigationSummary } from '../../api/types'
import { AppDataProvider, useAppData } from '../../state/appData'
import { backupInfo, restoreResult, summaries } from '../../test/fixtures'
import { cancelledPick, restorePick } from '../../test/fixtures.dataops'
import { mockApi } from '../../test/mockApi'
import { restoreSummary } from '../../lib/restoreCopy'
import { DataOpsProvider, useDataOps } from '../sidebar/DataOps'
import { ToastProvider } from '../ui/Toaster'
import { FileProblemDialog } from './FileProblemDialog'
import { RestoreConfirmDialog } from './RestoreConfirmDialog'
import { RestoreResultDialog } from './RestoreResultDialog'

const mocks = vi.hoisted(() => ({ api: {} as Api }))
vi.mock('../../api/api', () => ({ api: mocks.api }))

const WARNING =
  'Everything currently in Casebook will be replaced, including changes made by AI tools. Casebook saves a safety copy of your current data first.'

describe('RestoreConfirmDialog', () => {
  it('shows the counts and the warning with Cancel focused', async () => {
    const user = userEvent.setup()
    const onCancel = vi.fn()
    const onConfirm = vi.fn()
    render(
      <RestoreConfirmDialog fileName="casebook-backup-2026-10-02.db" info={backupInfo} onCancel={onCancel} onConfirm={onConfirm} />,
    )
    const dialog = screen.getByRole('dialog', { name: 'Replace all Casebook data with this backup?' })
    expect(dialog).toHaveTextContent(restoreSummary('casebook-backup-2026-10-02.db', backupInfo))
    expect(dialog).toHaveTextContent(WARNING)
    const cancel = within(dialog).getByRole('button', { name: 'Cancel' })
    expect(cancel).toHaveFocus()
    await user.click(cancel)
    expect(onCancel).toHaveBeenCalledTimes(1)
    expect(onConfirm).not.toHaveBeenCalled()
    await user.click(within(dialog).getByRole('button', { name: 'Replace all data' }))
    expect(onConfirm).toHaveBeenCalledTimes(1)
  })

  it('says Replacing... while busy', () => {
    render(<RestoreConfirmDialog fileName="b.db" info={backupInfo} busy onCancel={() => {}} onConfirm={() => {}} />)
    expect(screen.getByRole('button', { name: 'Replacing...' })).toBeDisabled()
  })
})

describe('RestoreResultDialog and FileProblemDialog', () => {
  it('shows where the previous data went', async () => {
    const user = userEvent.setup()
    const onCopy = vi.fn()
    const onDone = vi.fn()
    render(
      <RestoreResultDialog
        fileName="casebook-backup-2026-10-02.db"
        safetyCopyPath={restoreResult.safety_copy_path}
        onCopyPath={onCopy}
        onDone={onDone}
      />,
    )
    const dialog = screen.getByRole('dialog', { name: 'Restore complete' })
    expect(dialog).toHaveTextContent(
      `Casebook now shows the data from casebook-backup-2026-10-02.db. Your previous data was saved to ${restoreResult.safety_copy_path}.`,
    )
    await user.click(within(dialog).getByRole('button', { name: 'Copy path' }))
    expect(onCopy).toHaveBeenCalledTimes(1)
    await user.click(within(dialog).getByRole('button', { name: 'Done' }))
    expect(onDone).toHaveBeenCalledTimes(1)
  })

  it('offers another file or Close', async () => {
    const user = userEvent.setup()
    const onChoose = vi.fn()
    const onClose = vi.fn()
    render(<FileProblemDialog title="This file can't be restored" body="Bad. Nothing was changed." onChooseAnother={onChoose} onClose={onClose} />)
    const dialog = screen.getByRole('dialog', { name: "This file can't be restored" })
    await user.click(within(dialog).getByRole('button', { name: 'Choose another file...' }))
    expect(onChoose).toHaveBeenCalledTimes(1)
    await user.click(within(dialog).getAllByRole('button', { name: 'Close' }).at(-1)!)
    expect(onClose).toHaveBeenCalled()
  })
})

function Trigger() {
  const { restore } = useDataOps()
  const { selectedRef } = useAppData()
  return (
    <div>
      <button type="button" onClick={() => void restore()}>
        Restore from backup...
      </button>
      <p data-testid="selected">{selectedRef ?? ''}</p>
    </div>
  )
}

function renderFlow() {
  return render(
    <ToastProvider>
      <AppDataProvider>
        <DataOpsProvider>
          <Trigger />
        </DataOpsProvider>
      </AppDataProvider>
    </ToastProvider>,
  )
}

describe('two-step restore', () => {
  let restored: boolean
  const afterRestore: InvestigationSummary[] = [
    { ...summaries[1], is_active: true },
    { ...summaries[0], is_active: false },
  ]

  beforeEach(() => {
    restored = false
    Object.assign(
      mocks.api,
      mockApi({
        listInvestigations: async () => (restored ? afterRestore : summaries),
        restoreBackup: async () => {
          restored = true
          return restoreResult
        },
      }),
    )
  })

  async function start() {
    const user = userEvent.setup()
    renderFlow()
    await waitFor(() => expect(screen.getByTestId('selected')).toHaveTextContent('INV-0001'))
    await user.click(screen.getByRole('button', { name: 'Restore from backup...' }))
    return user
  }

  it('shows nothing when the pick is cancelled', async () => {
    Object.assign(mocks.api, { pickBackupToRestore: vi.fn(async () => cancelledPick) })
    await start()
    await waitFor(() => expect(mocks.api.pickBackupToRestore).toHaveBeenCalled())
    await new Promise((r) => setTimeout(r, 20))
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    expect(mocks.api.restoreBackup).not.toHaveBeenCalled()
  })

  it('explains a file that is not a valid backup', async () => {
    Object.assign(mocks.api, {
      pickBackupToRestore: vi.fn(async () => {
        throw JSON.stringify({ kind: 'conflict', message: 'conflict: not a valid Casebook backup (unexpected table notes)' })
      }),
    })
    const user = await start()
    const dialog = await screen.findByRole('dialog', { name: "This file can't be restored" })
    expect(dialog).toHaveTextContent('Not a valid Casebook backup (unexpected table notes). Nothing was changed.')
    await user.click(within(dialog).getByRole('button', { name: 'Choose another file...' }))
    await waitFor(() => expect(mocks.api.pickBackupToRestore).toHaveBeenCalledTimes(2))
    const again = await screen.findByRole('dialog', { name: "This file can't be restored" })
    await user.click(within(again).getAllByRole('button', { name: 'Close' }).at(-1)!)
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    expect(mocks.api.restoreBackup).not.toHaveBeenCalled()
  })

  it('does nothing when the confirmation is cancelled', async () => {
    const user = await start()
    const dialog = await screen.findByRole('dialog', { name: 'Replace all Casebook data with this backup?' })
    expect(within(dialog).getByRole('button', { name: 'Cancel' })).toHaveFocus()
    await user.click(within(dialog).getByRole('button', { name: 'Cancel' }))
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    expect(mocks.api.restoreBackup).not.toHaveBeenCalled()
  })

  it('replaces the data only after Replace all data, then shows the safety copy and reselects', async () => {
    const user = await start()
    const dialog = await screen.findByRole('dialog', { name: 'Replace all Casebook data with this backup?' })
    expect(dialog).toHaveTextContent(restoreSummary(restorePick.file_name, restorePick.info))
    expect(mocks.api.restoreBackup).not.toHaveBeenCalled()
    await user.click(within(dialog).getByRole('button', { name: 'Replace all data' }))
    expect(mocks.api.restoreBackup).toHaveBeenCalledWith(restorePick.path)

    const result = await screen.findByRole('dialog', { name: 'Restore complete' })
    expect(result).toHaveTextContent(
      `Casebook now shows the data from casebook-backup-2026-10-02.db. Your previous data was saved to ${restoreResult.safety_copy_path}.`,
    )
    await waitFor(() => expect(screen.getByTestId('selected')).toHaveTextContent('INV-0002'))
    await user.click(within(result).getByRole('button', { name: 'Copy path' }))
    expect(mocks.api.copyText).toHaveBeenCalledWith(restoreResult.safety_copy_path)
    await user.click(within(result).getByRole('button', { name: 'Done' }))
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
  })
})
