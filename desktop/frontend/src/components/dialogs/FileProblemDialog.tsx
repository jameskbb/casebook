// A file the user picked cannot be used (UI-SPEC Errors: "This file can't be
// imported", "This file can't be restored"): the reason, then "Choose
// another file..." to pick again or Close.
import { useRef } from 'react'
import { Button } from '../ui/Button'
import { Dialog } from '../ui/Dialog'

export interface FileProblemDialogProps {
  title: string
  body: string
  onChooseAnother: () => void
  onClose: () => void
}

export function FileProblemDialog({ title, body, onChooseAnother, onClose }: FileProblemDialogProps) {
  const closeRef = useRef<HTMLButtonElement>(null)
  return (
    <Dialog
      title={title}
      size="confirm"
      onClose={onClose}
      initialFocus={closeRef}
      footer={
        <>
          <Button variant="secondary" onClick={onChooseAnother}>
            Choose another file...
          </Button>
          <Button variant="primary" ref={closeRef} onClick={onClose}>
            Close
          </Button>
        </>
      }
    >
      <p className="dialog-text dialog-text-wrap">{body}</p>
    </Dialog>
  )
}
