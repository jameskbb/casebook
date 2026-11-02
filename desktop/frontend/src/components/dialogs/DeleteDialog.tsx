// Delete confirmation (AR-10; UI-SPEC Destructive and replacing actions):
// says exactly what is removed and that it cannot be undone. When something
// still depends on the record (evidence still linked, a claim that replaces
// another) the dialog explains what to do first and offers no Delete. A
// conflict the engine still returns (the data changed in another app) is
// shown in the dialog in plain words; nothing was deleted then.
import { useRef, useState } from 'react'
import { TriangleAlert } from 'lucide-react'
import { asSentence, errorCopy, toApiError } from '../../api/errors'
import { Button } from '../ui/Button'
import { Dialog } from '../ui/Dialog'
import './RestoreConfirmDialog.css'
import './DeleteDialog.css'

export const IRREVERSIBLE = 'This cannot be undone.'

export interface DeleteDialogProps {
  title: string
  /** What is removed, one or two sentences. */
  body: string
  /** Set when the record cannot be deleted yet: what to do first. */
  blocker?: string | null
  confirmLabel: string
  /** Runs the delete; resolves true when it was done. */
  onConfirm: () => Promise<boolean>
  onClose: () => void
}

export function DeleteDialog({ title, body, blocker, confirmLabel, onConfirm, onClose }: DeleteDialogProps) {
  const safeRef = useRef<HTMLButtonElement>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const confirm = async () => {
    if (busy) return
    setBusy(true)
    setError(null)
    try {
      if (await onConfirm()) onClose()
    } catch (e) {
      const err = toApiError(e)
      setError(`${asSentence(err.kind === 'conflict' ? err.message : errorCopy(err).message)} Nothing was deleted.`)
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog
      title={title}
      size="confirm"
      onClose={() => {
        if (!busy) onClose()
      }}
      initialFocus={safeRef}
      footer={
        blocker ? (
          <Button variant="primary" ref={safeRef} onClick={onClose}>
            OK
          </Button>
        ) : (
          <>
            <Button variant="secondary" ref={safeRef} onClick={onClose} disabled={busy}>
              Cancel
            </Button>
            <Button variant="destructive" busy={busy} busyLabel="Deleting..." onClick={() => void confirm()}>
              {confirmLabel}
            </Button>
          </>
        )
      }
    >
      {blocker ? (
        <p className="dialog-text dialog-text-wrap">{blocker}</p>
      ) : (
        <>
          <p className="dialog-text dialog-text-wrap">{body}</p>
          <div className="restore-warning">
            <TriangleAlert className="restore-warning-icon" size={16} strokeWidth={1.75} aria-hidden="true" />
            <p>{IRREVERSIBLE}</p>
          </div>
        </>
      )}
      {error && (
        <p className="dialog-text dialog-text-wrap delete-error" role="alert">
          {error}
        </p>
      )}
    </Dialog>
  )
}

/** "C-001", "C-001 and C-002", "C-001, C-002 and H-001". */
export function refList(refs: string[]): string {
  if (refs.length <= 1) return refs.join('')
  return `${refs.slice(0, -1).join(', ')} and ${refs[refs.length - 1]}`
}
