// Dialog (UI-SPEC Dialogs): centered and modal, 560px for forms or 440px for
// confirmations, title 20/600, close X, footer Cancel then submit. Focus is
// trapped, starts on the first field (or initialFocus), and returns to the
// opener on close. Esc closes the topmost layer only; Ctrl+Enter (Cmd+Enter
// on macOS) submits. A dirty dialog asks "Discard this draft?" before it
// closes.
import { useEffect, useId, useLayoutEffect, useRef, useState, type ReactNode, type RefObject } from 'react'
import { createPortal } from 'react-dom'
import { X } from 'lucide-react'
import { pushLayer } from '../../lib/layers'
import { focusableIn, trapFocus } from '../../lib/focusTrap'
import { Button, type ButtonVariant } from './Button'
import { IconButton } from './IconButton'
import './Dialog.css'

export interface DialogProps {
  title: string
  onClose: () => void
  children?: ReactNode
  /** "form" is 560px wide, "confirm" 440px. */
  size?: 'form' | 'confirm'
  /** Called on submit (Enter in a field, the submit button, Ctrl+Enter). */
  onSubmit?: () => void
  submitLabel?: string
  busyLabel?: string
  busy?: boolean
  submitVariant?: ButtonVariant
  submitDisabled?: boolean
  cancelLabel?: string
  /** Replaces the default Cancel and submit footer. */
  footer?: ReactNode
  /** Element to focus first instead of the first field. */
  initialFocus?: RefObject<HTMLElement | null>
  /** Error summary shown above the fields. */
  summary?: ReactNode
  /** Unsaved input: closing asks "Discard this draft?" first. */
  dirty?: boolean
  className?: string
}

export function Dialog({
  title,
  onClose,
  children,
  size = 'form',
  onSubmit,
  submitLabel,
  busyLabel,
  busy = false,
  submitVariant = 'primary',
  submitDisabled = false,
  cancelLabel = 'Cancel',
  footer,
  initialFocus,
  summary,
  dirty = false,
  className,
}: DialogProps) {
  const titleId = useId()
  const panelRef = useRef<HTMLDivElement>(null)
  const formRef = useRef<HTMLFormElement>(null)
  const bodyRef = useRef<HTMLDivElement>(null)
  const keepEditingRef = useRef<HTMLButtonElement>(null)
  const [confirmingDiscard, setConfirmingDiscard] = useState(false)
  const [discardAccepted, setDiscardAccepted] = useState(false)
  const onCloseRef = useRef(onClose)
  onCloseRef.current = onClose
  const busyRef = useRef(busy)
  busyRef.current = busy

  // Every close path (Esc, X, Cancel, the scrim) goes through requestClose,
  // which asks before throwing away unsaved input.
  const requestClose = () => {
    if (dirty) setConfirmingDiscard(true)
    else onClose()
  }
  const requestCloseRef = useRef(requestClose)
  requestCloseRef.current = requestClose

  // Remember the opener, move focus in, and give focus back on close.
  useLayoutEffect(() => {
    const opener = document.activeElement as HTMLElement | null
    const panel = panelRef.current
    if (panel) {
      const target =
        initialFocus?.current ??
        bodyRef.current?.querySelector<HTMLElement>('input:not([type="hidden"]), textarea, select') ??
        focusableIn(panel)[0] ??
        panel
      target.focus()
    }
    return () => {
      if (opener && opener.isConnected) opener.focus()
    }
    // Focus moves once, when the dialog opens.
  }, [])

  // Discard closes the confirmation first (its focus goes back into this
  // dialog), then this dialog (focus goes back to the opener).
  useEffect(() => {
    if (discardAccepted) onCloseRef.current()
  }, [discardAccepted])

  useEffect(() => {
    const panel = panelRef.current
    const release = panel ? trapFocus(panel) : () => {}
    const pop = pushLayer(() => requestCloseRef.current())
    return () => {
      pop()
      release()
    }
  }, [])

  const submit = () => {
    if (busyRef.current || submitDisabled) return
    onSubmit?.()
  }

  const content = (
    <div className="dialog-layer">
      <div className="dialog-scrim" aria-hidden="true" onMouseDown={() => requestCloseRef.current()} />
      <div
        ref={panelRef}
        className={['dialog', `dialog-${size}`, className ?? ''].filter(Boolean).join(' ')}
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        tabIndex={-1}
      >
        <form
          ref={formRef}
          className="dialog-form"
          noValidate
          onSubmit={(e) => {
            e.preventDefault()
            submit()
          }}
          onKeyDown={(e) => {
            if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) {
              e.preventDefault()
              submit()
            }
          }}
        >
          <div className="dialog-header">
            <h2 className="dialog-title" id={titleId}>
              {title}
            </h2>
            <IconButton icon={X} label="Close" onClick={() => requestCloseRef.current()} />
          </div>
          <div className="dialog-body" ref={bodyRef}>
            {summary}
            {children}
          </div>
          {footer !== undefined ? (
            footer && <div className="dialog-footer">{footer}</div>
          ) : (
            <div className="dialog-footer">
              <Button variant="secondary" onClick={() => requestCloseRef.current()}>
                {cancelLabel}
              </Button>
              {submitLabel && (
                <Button
                  type="submit"
                  variant={submitVariant}
                  busy={busy}
                  busyLabel={busyLabel}
                  disabled={submitDisabled}
                >
                  {submitLabel}
                </Button>
              )}
            </div>
          )}
        </form>
      </div>
      {confirmingDiscard && (
        <Dialog
          title="Discard this draft?"
          size="confirm"
          onClose={() => setConfirmingDiscard(false)}
          initialFocus={keepEditingRef}
          footer={
            <>
              <Button variant="secondary" ref={keepEditingRef} onClick={() => setConfirmingDiscard(false)}>
                Keep editing
              </Button>
              <Button
                variant="destructive"
                onClick={() => {
                  setConfirmingDiscard(false)
                  setDiscardAccepted(true)
                }}
              >
                Discard
              </Button>
            </>
          }
        >
          <p className="dialog-text">What you typed has not been saved.</p>
        </Dialog>
      )}
    </div>
  )
  return createPortal(content, document.body)
}
