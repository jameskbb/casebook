// Copy for your AI (UI-19, D-20): one shared state so the header button, the
// Ctrl+Shift+C shortcut and the More menu's Copy case as text give the
// same feedback. "Copied" shows for 2000 ms only after the Go clipboard call
// succeeded, and the polite live region announces it. A failure shows the
// Errors table toast (clipboard copy with Retry).
//
// Copy for your AI copies the pack framed as data for an AI chat (the line that
// says it is stored data, not instructions, and the BEGIN and END CASEBOOK
// DATA markers), as casebook_get_context returns it. Plain copies the bare
// Markdown pack, for notes, tickets or documents.
import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import { api } from '../api/api'
import { errorCopy, toApiError } from '../api/errors'
import { useToast } from '../components/ui/Toaster'

export type CopyStatus = 'idle' | 'copying' | 'copied'

export interface CopyContextState {
  status: CopyStatus
  copy(ref: string, opts?: CopyOptions): Promise<void>
}

export interface CopyOptions {
  /** Copy the bare Markdown pack, without the AI data framing. */
  plain?: boolean
}

export const COPIED_MS = 2000
export const COPIED_ANNOUNCEMENT = 'Context copied to the clipboard'
export const COPIED_PLAIN_ANNOUNCEMENT = 'Copied the case as text.'

const CopyContextContext = createContext<CopyContextState | null>(null)

export function CopyContextProvider({ children }: { children: ReactNode }) {
  const toast = useToast()
  const [status, setStatus] = useState<CopyStatus>('idle')
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const copyRef = useRef<(ref: string, opts?: CopyOptions) => Promise<void>>(async () => {})

  useEffect(
    () => () => {
      if (timer.current) clearTimeout(timer.current)
    },
    [],
  )

  const copy = useCallback(
    async (ref: string, opts?: CopyOptions) => {
      const plain = opts?.plain === true
      if (timer.current) {
        clearTimeout(timer.current)
        timer.current = null
      }
      setStatus('copying')
      try {
        if (plain) await api.copyContextPlain(ref)
        else await api.copyContext(ref)
      } catch (e) {
        setStatus('idle')
        const copy = errorCopy(toApiError(e))
        toast.error(
          copy.message,
          copy.action === 'retry' ? { label: 'Retry', onClick: () => void copyRef.current(ref, opts) } : undefined,
        )
        return
      }
      setStatus('copied')
      toast.announce(plain ? COPIED_PLAIN_ANNOUNCEMENT : COPIED_ANNOUNCEMENT)
      timer.current = setTimeout(() => {
        timer.current = null
        setStatus('idle')
      }, COPIED_MS)
    },
    [toast],
  )

  copyRef.current = copy

  const value = useMemo(() => ({ status, copy }), [status, copy])
  return <CopyContextContext.Provider value={value}>{children}</CopyContextContext.Provider>
}

export function useCopyContext(): CopyContextState {
  const ctx = useContext(CopyContextContext)
  if (!ctx) throw new Error('useCopyContext must be used inside CopyContextProvider')
  return ctx
}
