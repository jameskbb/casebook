// The full-window startup error (UI-SPEC Errors, Startup rows): shown instead
// of the app when the database could not be opened. It never writes on its
// own. It shows where the data file is, with Open data folder and Copy path,
// and (AR-11) when a backup can fix the problem it offers Restore from
// backup: Casebook moves the file it could not open into the backups folder
// (nothing is deleted), opens the backup in its place and then starts
// normally. A file from a newer Casebook says to update Casebook, and also
// offers the restore for a person going back to this version: the newer
// Casebook wrote a casebook-pre-upgrade backup before it upgraded the data.
import { useState } from 'react'
import { Copy, FolderOpen, RotateCcw, TriangleAlert } from 'lucide-react'
import { api } from '../api/api'
import { asSentence, CLIPBOARD_COPY, errorCopy, toApiError } from '../api/errors'
import type { RestorePick, StartupInfo } from '../api/types'
import { useAppData } from '../state/appData'
import { RestoreConfirmDialog } from './dialogs/RestoreConfirmDialog'
import { RestoreResultDialog } from './dialogs/RestoreResultDialog'
import { Button } from './ui/Button'
import { useToast } from './ui/Toaster'
import './StartupError.css'

export const RECOVER_WARNING =
  'Casebook moves the data file it could not open into the backups folder next to it (nothing is deleted), then opens this backup in its place.'
export const FOLDER_COPY = 'Could not open the data folder. Copy the path and open it yourself.'

export function StartupError({ info }: { info: StartupInfo }) {
  const toast = useToast()
  const { retryStartup } = useAppData()
  const [pick, setPick] = useState<RestorePick | null>(null)
  const [restoring, setRestoring] = useState(false)
  const [result, setResult] = useState<{ fileName: string; moved: string } | null>(null)
  const [problem, setProblem] = useState<string | null>(null)

  const tooNew = info.kind === 'schema_too_new'
  const canRestore = info.recoverable === true
  const heading = tooNew ? 'This data needs a newer Casebook' : 'Casebook could not open its data'
  const body = tooNew
    ? `The data file at ${info.db_path} was saved by a newer version of Casebook. Update Casebook to open it. Nothing has been changed.${
        canRestore
          ? ' To keep using this version instead, restore the casebook-pre-upgrade backup from the backups folder next to the data file.'
          : ''
      }`
    : `${asSentence(info.message || 'The data file could not be opened')} Close Casebook and try again.${
        canRestore ? ' If this keeps happening, restore a backup.' : ''
      }`

  const openFolder = async () => {
    try {
      await api.openDataFolder()
    } catch {
      toast.error(FOLDER_COPY)
    }
  }

  const copyPath = async (path: string, viaLocation: boolean) => {
    try {
      if (viaLocation) await api.copyDataLocation()
      else await api.copyText(path)
      toast.success('Path copied to the clipboard.')
    } catch {
      toast.error(CLIPBOARD_COPY)
    }
  }

  const choose = async () => {
    setProblem(null)
    try {
      const picked = await api.pickBackupToRecover()
      if (!picked.cancelled) setPick(picked)
    } catch (e) {
      setProblem(`${errorCopy(toApiError(e)).message.replace(/\.?$/, '.')} Nothing was changed.`)
    }
  }

  const confirm = async () => {
    if (!pick || restoring) return
    setRestoring(true)
    try {
      const res = await api.recoverFromBackup(pick.path)
      setResult({ fileName: pick.file_name, moved: res.safety_copy_path })
    } catch (e) {
      setProblem(`${errorCopy(toApiError(e)).message.replace(/\.?$/, '.')} Nothing was changed.`)
    } finally {
      setRestoring(false)
      setPick(null)
    }
  }

  return (
    <main className="startup-error">
      <div className="startup-error-panel">
        <TriangleAlert className="startup-error-icon" size={48} strokeWidth={1.75} aria-hidden="true" />
        <h1 className="startup-error-heading">{heading}</h1>
        <p className="startup-error-body">{body}</p>
        {info.db_path && (
          <div className="startup-error-location">
            <p className="startup-error-path">
              <span className="startup-error-path-label">Data file</span>
              <code className="startup-error-path-text">{info.db_path}</code>
            </p>
            <div className="startup-error-actions">
              <Button variant="ghost" icon={FolderOpen} onClick={() => void openFolder()}>
                Open data folder
              </Button>
              <Button variant="ghost" icon={Copy} onClick={() => void copyPath(info.db_path, true)}>
                Copy path
              </Button>
            </div>
          </div>
        )}
        {problem && (
          <p className="startup-error-problem" role="alert">
            {problem}
          </p>
        )}
        <div className="startup-error-actions">
          {canRestore && (
            <Button variant="secondary" icon={RotateCcw} onClick={() => void choose()}>
              Restore from backup...
            </Button>
          )}
          <Button variant="secondary" onClick={() => void api.quit().catch(() => {})}>
            Close Casebook
          </Button>
        </div>
      </div>
      {pick && (
        <RestoreConfirmDialog
          fileName={pick.file_name}
          info={pick.info}
          busy={restoring}
          warning={RECOVER_WARNING}
          onConfirm={() => void confirm()}
          onCancel={() => {
            if (!restoring) setPick(null)
          }}
        />
      )}
      {result && (
        <RestoreResultDialog
          fileName={result.fileName}
          safetyCopyPath={result.moved}
          body={`Casebook now uses the data from ${result.fileName}. The file it could not open was moved to ${result.moved}.`}
          onCopyPath={() => void copyPath(result.moved, false)}
          onDone={() => {
            setResult(null)
            retryStartup()
          }}
        />
      )}
    </main>
  )
}
