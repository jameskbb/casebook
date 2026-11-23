// The full-window startup error: shown instead of the app when the stored
// questions could not be opened. It never writes on its own. When a backup
// can fix the problem it offers Bring back a backup: Casebook keeps the file
// it could not open (nothing is deleted), opens the backup instead and then
// starts as usual. A file from a newer Casebook says to update Casebook, and
// also offers the backup that newer Casebook made before it changed the
// file, for a person going back to this version. Where the file is sits
// under Technical details.
import { useState } from 'react'
import { Copy, FolderOpen, RotateCcw, TriangleAlert } from 'lucide-react'
import { api } from '../api/api'
import { CLIPBOARD_COPY, plainError, toApiError } from '../api/errors'
import type { RestorePick, StartupInfo } from '../api/types'
import { useAppData } from '../state/appData'
import { RestoreConfirmDialog } from './dialogs/RestoreConfirmDialog'
import { RestoreResultDialog } from './dialogs/RestoreResultDialog'
import { Button } from './ui/Button'
import { useToast } from './ui/Toaster'
import './StartupError.css'

export const RECOVER_WARNING =
  'Casebook keeps the file it could not open and opens this backup instead. Nothing is deleted.'
export const FOLDER_COPY = 'Could not open the folder. Copy its location and open it yourself.'

export function StartupError({ info }: { info: StartupInfo }) {
  const toast = useToast()
  const { retryStartup } = useAppData()
  const [pick, setPick] = useState<RestorePick | null>(null)
  const [restoring, setRestoring] = useState(false)
  const [result, setResult] = useState<{ fileName: string; moved: string } | null>(null)
  const [problem, setProblem] = useState<string | null>(null)

  const tooNew = info.kind === 'schema_too_new'
  const canRestore = info.recoverable === true
  const heading = tooNew ? 'Your questions were saved by a newer Casebook' : 'Casebook could not open your questions'
  const body = tooNew
    ? `Update Casebook to open them. Nothing was changed.${canRestore ? ' To keep using this version, bring back the backup Casebook made before the update.' : ''}`
    : `Close Casebook and try again.${canRestore ? ' If this keeps happening, bring back a backup.' : ''}`

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
      toast.success('Location copied.')
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
      setProblem(`${plainError(toApiError(e))} Nothing was changed.`)
    }
  }

  const confirm = async () => {
    if (!pick || restoring) return
    setRestoring(true)
    try {
      const res = await api.recoverFromBackup(pick.path)
      setResult({ fileName: pick.file_name, moved: res.safety_copy_path })
    } catch (e) {
      setProblem(`${plainError(toApiError(e))} Nothing was changed.`)
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
          <details className="startup-error-location">
            <summary>Technical details</summary>
            <p className="startup-error-path">
              <span className="startup-error-path-label">Where your questions are kept</span>
              <code className="startup-error-path-text">{info.db_path}</code>
            </p>
            <div className="startup-error-actions">
              <Button variant="ghost" icon={FolderOpen} onClick={() => void openFolder()}>
                Open the folder
              </Button>
              <Button variant="ghost" icon={Copy} onClick={() => void copyPath(info.db_path, true)}>
                Copy location
              </Button>
            </div>
          </details>
        )}
        {problem && (
          <p className="startup-error-problem" role="alert">
            {problem}
          </p>
        )}
        <div className="startup-error-actions">
          {canRestore && (
            <Button variant="secondary" icon={RotateCcw} onClick={() => void choose()}>
              Bring back a backup...
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
          body={`Casebook now shows what was in ${result.fileName}. The file it could not open was kept at ${result.moved}.`}
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
