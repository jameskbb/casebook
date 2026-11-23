// Step two of bringing back a backup: what it holds and a plain warning.
// Cancel has the initial focus; only "Replace everything" goes on to
// RestoreBackup.
import { TriangleAlert } from 'lucide-react'
import type { BackupInfo } from '../../api/types'
import { restoreSummary } from '../../lib/restoreCopy'
import { ConfirmDialog } from '../ui/ConfirmDialog'
import './RestoreConfirmDialog.css'

export const RESTORE_WARNING =
  'Everything in Casebook now will be replaced, including what your AI added. Casebook saves a backup of what you have now first.'

export interface RestoreConfirmDialogProps {
  fileName: string
  info: BackupInfo
  busy?: boolean
  /** Replaces the usual warning (at startup it says what happens to the file that would not open). */
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
      title="Replace everything in Casebook with this backup?"
      body={<p className="dialog-text dialog-text-wrap">{restoreSummary(fileName, info)}</p>}
      confirmLabel="Replace everything"
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
