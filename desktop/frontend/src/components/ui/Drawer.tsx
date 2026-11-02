// Drawer (UI-SPEC Drawers, Component Contract Drawer): a 480px right
// slide-over for drill-down, never wider than the main pane minus 48px, full
// window height, over a scrim. It is modal (role dialog, aria-modal,
// labelled by its heading), traps focus and registers one overlay layer, so
// Esc closes the whole stack only when nothing sits above it. The heading
// takes focus when the drawer opens and whenever its content changes (a push
// or Back). A stacked drawer shows "Back to C-001" above the heading.
import { useEffect, useId, useLayoutEffect, useRef, type ReactNode } from 'react'
import { createPortal } from 'react-dom'
import { ArrowLeft, X, type LucideIcon } from 'lucide-react'
import { pushLayer } from '../../lib/layers'
import { trapFocus } from '../../lib/focusTrap'
import { Button } from './Button'
import { IconButton } from './IconButton'
import './Drawer.css'

export interface DrawerBack {
  /** The ref shown in "Back to C-001". */
  ref: string
  onClick: () => void
}

export interface DrawerProps {
  /** "Claim C-001", "Evidence E-003". */
  heading: string
  icon?: LucideIcon
  /** Tints the heading icon with the kind hue. */
  kind?: 'evidence' | 'claim' | 'hypothesis'
  back?: DrawerBack
  /** Closes the whole stack (X, Esc, the scrim). */
  onClose: () => void
  /** Changes when the drawer shows another record; moves focus to the heading. */
  contentKey: string
  children?: ReactNode
}

export function Drawer({ heading, icon: Icon, kind, back, onClose, contentKey, children }: DrawerProps) {
  const headingId = useId()
  const panelRef = useRef<HTMLDivElement>(null)
  const headingRef = useRef<HTMLHeadingElement>(null)
  const onCloseRef = useRef(onClose)
  onCloseRef.current = onClose

  useEffect(() => {
    const panel = panelRef.current
    const release = panel ? trapFocus(panel) : () => {}
    const pop = pushLayer(() => onCloseRef.current())
    return () => {
      pop()
      release()
    }
  }, [])

  // Focus the heading on open and after every push or Back.
  useLayoutEffect(() => {
    headingRef.current?.focus()
  }, [contentKey])

  return createPortal(
    <div className="drawer-layer">
      <div className="drawer-scrim" aria-hidden="true" onMouseDown={() => onCloseRef.current()} />
      <div ref={panelRef} className="drawer" role="dialog" aria-modal="true" aria-labelledby={headingId}>
        <div className="drawer-header">
          {back && (
            <Button variant="ghost" icon={ArrowLeft} className="drawer-back" onClick={back.onClick}>
              {`Back to ${back.ref}`}
            </Button>
          )}
          <div className="drawer-title-row">
            {Icon && (
              <Icon
                className={['drawer-kind-icon', kind ? `drawer-kind-${kind}` : ''].filter(Boolean).join(' ')}
                size={20}
                strokeWidth={1.75}
                aria-hidden="true"
              />
            )}
            <h2 className="drawer-heading tabular" id={headingId} ref={headingRef} tabIndex={-1}>
              {heading}
            </h2>
            <IconButton icon={X} label="Close" onClick={() => onCloseRef.current()} />
          </div>
        </div>
        <div className="drawer-body" key={contentKey}>
          {children}
        </div>
      </div>
    </div>,
    document.body,
  )
}
