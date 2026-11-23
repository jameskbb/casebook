// After a backup is brought back: which one is now shown and where the
// backup of what was there before went, with "Copy location" and "Done".
import { useRef } from 'react'
import { Copy } from 'lucide-react'
import { Button } from '../ui/Button'
import { Dialog } from '../ui/Dialog'
import './RestoreConfirmDialog.css'

export interface RestoreResultDialogProps {
  fileName: string
  safetyCopyPath: string
  /** Replaces the usual sentence about where the earlier questions went. */
  body?: string
  onCopyPath: () => void
  onDone: () => void
}

export function RestoreResultDialog({ fileName, safetyCopyPath, body, onCopyPath, onDone }: RestoreResultDialogProps) {
  const doneRef = useRef<HTMLButtonElement>(null)
  return (
    <Dialog
      title="Your backup is open"
      size="confirm"
      onClose={onDone}
      initialFocus={doneRef}
      footer={
        <>
          <Button variant="secondary" icon={Copy} onClick={onCopyPath}>
            Copy location
          </Button>
          <Button variant="primary" ref={doneRef} onClick={onDone}>
            Done
          </Button>
        </>
      }
    >
      <p className="dialog-text dialog-text-wrap">
        {body ?? `Casebook now shows what was in ${fileName}. What you had before was saved to ${safetyCopyPath}.`}
      </p>
    </Dialog>
  )
}
