// After a restore (D-07): which backup is now shown and where the safety copy
// of the previous data went, with "Copy path" and "Done".
import { useRef } from 'react'
import { Copy } from 'lucide-react'
import { Button } from '../ui/Button'
import { Dialog } from '../ui/Dialog'
import './RestoreConfirmDialog.css'

export interface RestoreResultDialogProps {
  fileName: string
  safetyCopyPath: string
  /** Replaces the default sentence about the safety copy. */
  body?: string
  onCopyPath: () => void
  onDone: () => void
}

export function RestoreResultDialog({ fileName, safetyCopyPath, body, onCopyPath, onDone }: RestoreResultDialogProps) {
  const doneRef = useRef<HTMLButtonElement>(null)
  return (
    <Dialog
      title="Restore complete"
      size="confirm"
      onClose={onDone}
      initialFocus={doneRef}
      footer={
        <>
          <Button variant="secondary" icon={Copy} onClick={onCopyPath}>
            Copy path
          </Button>
          <Button variant="primary" ref={doneRef} onClick={onDone}>
            Done
          </Button>
        </>
      }
    >
      <p className="dialog-text dialog-text-wrap">
        {body ?? `Casebook now shows the data from ${fileName}. Your previous data was saved to ${safetyCopyPath}.`}
      </p>
    </Dialog>
  )
}
