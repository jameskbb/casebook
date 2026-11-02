// Step two of restore (D-07; UI-SPEC Destructive and replacing actions): what
// the backup holds and a plain warning. Cancel has the initial focus; only
// "Replace all data" goes on to RestoreBackup.
import { TriangleAlert } from 'lucide-react'
import type { BackupInfo } from '../../api/types'
import { restoreSummary } from '../../lib/restoreCopy'
import { ConfirmDialog } from '../ui/ConfirmDialog'
import './RestoreConfirmDialog.css'

export const RESTORE_WARNING =
  'Everything currently in Casebook will be replaced, including changes made by AI tools. Casebook saves a safety copy of your current data first.'

export interface RestoreConfirmDialogProps {
  fileName: string
  info: BackupInfo
  busy?: boolean
  /** Replaces the default warning (startup recovery explains what it moves aside). */
  warning?: string
  onConfirm: () => void
  onCancel: () => void
}

export function RestoreConfirmDialog({
  fileName,
  info,
  busy = false,
  warning = RESTORE_WARNING,
  onConfirm,
  onCancel,
}: RestoreConfirmDialogProps) {
  return (
    <ConfirmDialog
      title="Replace all Casebook data with this backup?"
      body={<p className="dialog-text dialog-text-wrap">{restoreSummary(fileName, info)}</p>}
      confirmLabel="Replace all data"
      busy={busy}
      busyLabel="Replacing..."
      onConfirm={onConfirm}
      onCancel={onCancel}
    >
      <div className="restore-warning">
        <TriangleAlert className="restore-warning-icon" size={16} strokeWidth={1.75} aria-hidden="true" />
        <p>{warning}</p>
      </div>
    </ConfirmDialog>
  )
}
