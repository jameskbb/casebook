// Toasts (UI-SPEC Component Contract, Toaster): bottom right, at most three
// stacked, 360px wide. Success toasts sit in the app's one polite live region
// and dismiss after 4000 ms; error toasts use role="alert" and stay until
// dismissed. announce() speaks a message through the same polite region
// without showing a toast ("Context copied to the clipboard").
import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import { CircleAlert, X } from 'lucide-react'
import './Toaster.css'

export interface ToastAction {
  label: string
  onClick: () => void
}

export interface ToastApi {
  /** Shows a success toast and returns its id. */
  success(message: string, action?: ToastAction): number
  /** Shows an error toast and returns its id. */
  error(message: string, action?: ToastAction): number
  /** Removes a toast if it is still shown. */
  dismiss(id: number): void
  /** Speaks a message in the polite live region without a visible toast. */
  announce(message: string): void
}

interface ToastItem {
  id: number
  kind: 'success' | 'error'
  message: string
  action?: ToastAction
}

export const SUCCESS_TOAST_MS = 4000
const MAX_TOASTS = 3

const ToastContext = createContext<ToastApi | null>(null)

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<ToastItem[]>([])
  const [announcement, setAnnouncement] = useState({ id: 0, text: '' })
  const nextId = useRef(1)
  const timers = useRef(new Map<number, ReturnType<typeof setTimeout>>())

  const dismiss = useCallback((id: number) => {
    const t = timers.current.get(id)
    if (t) clearTimeout(t)
    timers.current.delete(id)
    setToasts((list) => list.filter((x) => x.id !== id))
  }, [])

  const push = useCallback(
    (kind: ToastItem['kind'], message: string, action?: ToastAction): number => {
      const id = nextId.current++
      setToasts((list) => {
        const next = [...list, { id, kind, message, action }]
        const dropped = next.slice(0, Math.max(0, next.length - MAX_TOASTS))
        for (const d of dropped) {
          const t = timers.current.get(d.id)
          if (t) clearTimeout(t)
          timers.current.delete(d.id)
        }
        return next.slice(-MAX_TOASTS)
      })
      if (kind === 'success') {
        timers.current.set(
          id,
          setTimeout(() => dismiss(id), SUCCESS_TOAST_MS),
        )
      }
      return id
    },
    [dismiss],
  )

  useEffect(() => {
    const map = timers.current
    return () => {
      for (const t of map.values()) clearTimeout(t)
      map.clear()
    }
  }, [])

  const api = useMemo<ToastApi>(
    () => ({
      success: (message, action) => push('success', message, action),
      error: (message, action) => push('error', message, action),
      dismiss,
      announce: (text) => setAnnouncement((a) => ({ id: a.id + 1, text })),
    }),
    [push, dismiss],
  )

  const success = toasts.filter((t) => t.kind === 'success')
  const errors = toasts.filter((t) => t.kind === 'error')

  return (
    <ToastContext.Provider value={api}>
      {children}
      <div className="toaster">
        <div className="toaster-errors">
          {errors.map((t) => (
            <ToastView key={t.id} toast={t} onDismiss={dismiss} />
          ))}
        </div>
        <div className="toaster-polite" aria-live="polite">
          {success.map((t) => (
            <ToastView key={t.id} toast={t} onDismiss={dismiss} />
          ))}
          <span className="sr-only" key={announcement.id}>
            {announcement.text}
          </span>
        </div>
      </div>
    </ToastContext.Provider>
  )
}

function ToastView({ toast, onDismiss }: { toast: ToastItem; onDismiss: (id: number) => void }) {
  const isError = toast.kind === 'error'
  return (
    <div className={`toast toast-${toast.kind}`} role={isError ? 'alert' : 'status'}>
      {isError && <CircleAlert className="toast-icon" size={16} strokeWidth={1.75} aria-hidden="true" />}
      <p className="toast-message">{toast.message}</p>
      {toast.action && (
        <button
          type="button"
          className="toast-action"
          onClick={() => {
            onDismiss(toast.id)
            toast.action?.onClick()
          }}
        >
          {toast.action.label}
        </button>
      )}
      <button
        type="button"
        className="toast-dismiss"
        aria-label="Dismiss"
        title="Dismiss"
        onClick={() => onDismiss(toast.id)}
      >
        <X size={16} strokeWidth={1.75} aria-hidden="true" />
      </button>
    </div>
  )
}

export function useToast(): ToastApi {
  const ctx = useContext(ToastContext)
  if (!ctx) throw new Error('useToast must be used inside ToastProvider')
  return ctx
}
