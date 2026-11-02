// ConfirmDialog (UI-SPEC Destructive and replacing actions): 440px, title,
// body, a safe button that takes the initial focus, and the confirm button
// (destructive by default).
import { useRef, type ReactNode } from 'react'
import { Button, type ButtonVariant } from './Button'
import { Dialog } from './Dialog'

export interface ConfirmDialogProps {
  title: string
  body: ReactNode
  confirmLabel: string
  onConfirm: () => void
  onCancel: () => void
  /** Label of the safe button (initial focus). */
  cancelLabel?: string
  confirmVariant?: ButtonVariant
  busy?: boolean
  busyLabel?: string
  /** Extra content under the body, e.g. a warning block. */
  children?: ReactNode
}

export function ConfirmDialog({
  title,
  body,
  confirmLabel,
  onConfirm,
  onCancel,
  cancelLabel = 'Cancel',
  confirmVariant = 'destructive',
  busy = false,
  busyLabel,
  children,
}: ConfirmDialogProps) {
  const safeRef = useRef<HTMLButtonElement>(null)
  return (
    <Dialog
      title={title}
      size="confirm"
      onClose={onCancel}
      initialFocus={safeRef}
      footer={
        <>
          <Button variant="secondary" ref={safeRef} onClick={onCancel}>
            {cancelLabel}
          </Button>
          <Button variant={confirmVariant} busy={busy} busyLabel={busyLabel} onClick={onConfirm}>
            {confirmLabel}
          </Button>
        </>
      }
    >
      {typeof body === 'string' ? <p className="dialog-text">{body}</p> : body}
      {children}
    </Dialog>
  )
}
