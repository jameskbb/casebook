// Menu (UI-SPEC Component Contract, MoreMenu/Menu): a button-anchored
// role="menu" with 36px items and leading icons. Enter, Space, click or Down
// on the button opens it and focuses the first enabled item (the checked one
// when there is one); Up and Down move past disabled items and wrap; Enter
// chooses; Esc closes only the topmost layer (pushLayer); Tab and outside
// clicks close it. Choosing or Esc returns focus to the button.
//
// The menu renders in a portal with fixed positioning so a scrolling
// container (the main pane, a drawer body) never clips it.
import {
  useCallback,
  useEffect,
  useId,
  useLayoutEffect,
  useRef,
  useState,
  type CSSProperties,
  type KeyboardEvent,
  type ReactNode,
  type RefObject,
} from 'react'
import { createPortal } from 'react-dom'
import { Check, type LucideIcon } from 'lucide-react'
import { pushLayer } from '../../lib/layers'
import './Menu.css'

export interface MenuItem {
  label: string
  icon?: LucideIcon
  onSelect: () => void
  disabled?: boolean
  /** Set for single-choice menus (the status picker): menuitemradio with aria-checked. */
  checked?: boolean
  /** A second, muted line under the label (the data file path). */
  detail?: string
  /** Accessible name when it should say more than the visible label. */
  ariaLabel?: string
  /** Tooltip. */
  title?: string
  /** Draw a rule above this item to start a new group. */
  separatorBefore?: boolean
}

export interface MenuTriggerProps {
  ref: RefObject<HTMLButtonElement | null>
  onClick: () => void
  onKeyDown: (e: KeyboardEvent<HTMLButtonElement>) => void
  'aria-haspopup': 'menu'
  'aria-expanded': boolean
  'aria-controls'?: string
}

export interface MenuProps {
  /** The menu's accessible name. */
  label: string
  items: MenuItem[]
  /** Optional, non-interactive content below the actions. */
  footer?: ReactNode
  /** Renders the button; spread the props onto a <button>. */
  trigger: (props: MenuTriggerProps) => ReactNode
  /** Align the menu with the button's left (start) or right (end) edge. */
  align?: 'start' | 'end'
}

const GAP = 4
const EDGE = 8

export function Menu({ label, items, footer, trigger, align = 'start' }: MenuProps) {
  const [open, setOpen] = useState(false)
  const [startAt, setStartAt] = useState<'first' | 'last'>('first')
  const [position, setPosition] = useState<CSSProperties>({ top: 0, left: 0, visibility: 'hidden' })
  const triggerRef = useRef<HTMLButtonElement | null>(null)
  const menuRef = useRef<HTMLDivElement | null>(null)
  const itemRefs = useRef<(HTMLButtonElement | null)[]>([])
  const menuId = useId()
  const radio = items.some((i) => i.checked !== undefined)

  const enabled = items.map((i, n) => (i.disabled ? -1 : n)).filter((n) => n >= 0)

  const close = useCallback((restoreFocus: boolean) => {
    setOpen(false)
    if (restoreFocus) triggerRef.current?.focus()
  }, [])

  const place = useCallback(() => {
    const t = triggerRef.current
    const m = menuRef.current
    if (!t || !m) return
    const r = t.getBoundingClientRect()
    const w = m.offsetWidth
    const h = m.offsetHeight
    let top = r.bottom + GAP
    if (top + h > window.innerHeight - EDGE && r.top - GAP - h >= EDGE) top = r.top - GAP - h
    let left = align === 'end' ? r.right - w : r.left
    left = Math.max(EDGE, Math.min(left, window.innerWidth - w - EDGE))
    setPosition({ top, left })
  }, [align])

  // Place the menu and move focus in once it is rendered.
  useLayoutEffect(() => {
    if (!open) return
    place()
    const checked = items.findIndex((i) => i.checked && !i.disabled)
    const target = checked >= 0 ? checked : startAt === 'last' ? enabled[enabled.length - 1] : enabled[0]
    if (target !== undefined) itemRefs.current[target]?.focus()
    else menuRef.current?.focus()
    // Focus moves once per opening.
  }, [open])

  useEffect(() => {
    if (!open) return
    const pop = pushLayer(() => close(true))
    const onDown = (e: MouseEvent) => {
      const target = e.target as Node
      if (menuRef.current?.contains(target) || triggerRef.current?.contains(target)) return
      close(false)
    }
    const onMove = () => place()
    document.addEventListener('mousedown', onDown, true)
    window.addEventListener('resize', onMove)
    window.addEventListener('scroll', onMove, true)
    return () => {
      pop()
      document.removeEventListener('mousedown', onDown, true)
      window.removeEventListener('resize', onMove)
      window.removeEventListener('scroll', onMove, true)
    }
  }, [open, close, place])

  const focusAt = (from: number, delta: number) => {
    if (enabled.length === 0) return
    const pos = enabled.indexOf(from)
    const next = pos < 0 ? (delta > 0 ? 0 : enabled.length - 1) : (pos + delta + enabled.length) % enabled.length
    itemRefs.current[enabled[next]]?.focus()
  }

  const onMenuKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    const current = itemRefs.current.findIndex((el) => el === document.activeElement)
    switch (e.key) {
      case 'ArrowDown':
        e.preventDefault()
        focusAt(current, 1)
        break
      case 'ArrowUp':
        e.preventDefault()
        focusAt(current, -1)
        break
      case 'Home':
        e.preventDefault()
        if (enabled.length) itemRefs.current[enabled[0]]?.focus()
        break
      case 'End':
        e.preventDefault()
        if (enabled.length) itemRefs.current[enabled[enabled.length - 1]]?.focus()
        break
      case 'Tab':
        e.preventDefault()
        close(true)
        break
    }
  }

  const choose = (item: MenuItem) => {
    if (item.disabled) return
    close(true)
    item.onSelect()
  }

  const triggerProps: MenuTriggerProps = {
    ref: triggerRef,
    onClick: () => {
      setStartAt('first')
      setOpen((v) => !v)
    },
    onKeyDown: (e) => {
      if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
        e.preventDefault()
        setStartAt(e.key === 'ArrowUp' ? 'last' : 'first')
        setOpen(true)
      }
    },
    'aria-haspopup': 'menu',
    'aria-expanded': open,
    ...(open ? { 'aria-controls': menuId } : {}),
  }

  return (
    <>
      {trigger(triggerProps)}
      {open &&
        createPortal(
          <div
            ref={menuRef}
            id={menuId}
            className="menu"
            role="menu"
            aria-label={label}
            tabIndex={-1}
            style={position}
            onKeyDown={onMenuKeyDown}
          >
            {items.map((item, i) => {
              const Icon = item.icon
              const button = (
                <button
                  key={item.label}
                  ref={(el) => {
                    itemRefs.current[i] = el
                  }}
                  type="button"
                  role={radio ? 'menuitemradio' : 'menuitem'}
                  aria-checked={radio ? Boolean(item.checked) : undefined}
                  aria-disabled={item.disabled || undefined}
                  aria-label={item.ariaLabel}
                  title={item.title}
                  disabled={item.disabled}
                  tabIndex={-1}
                  className={['menu-item', item.checked ? 'is-checked' : '', item.detail ? 'has-detail' : '']
                    .filter(Boolean)
                    .join(' ')}
                  onClick={() => choose(item)}
                  onMouseEnter={(e) => {
                    if (!item.disabled) e.currentTarget.focus()
                  }}
                >
                  {Icon && <Icon className="menu-item-icon" size={16} strokeWidth={1.75} aria-hidden="true" />}
                  {item.detail ? (
                    <span className="menu-item-text">
                      <span className="menu-item-label">{item.label}</span>
                      <span className="menu-item-detail tabular">{item.detail}</span>
                    </span>
                  ) : (
                    <span className="menu-item-label">{item.label}</span>
                  )}
                  {item.checked && <Check className="menu-item-check" size={16} strokeWidth={1.75} aria-hidden="true" />}
                </button>
              )
              if (!item.separatorBefore) return button
              return [<div key={`${item.label}-rule`} className="menu-separator" role="separator" />, button]
            })}
            {footer && <div className="menu-footer">{footer}</div>}
          </div>,
          document.body,
        )}
    </>
  )
}
