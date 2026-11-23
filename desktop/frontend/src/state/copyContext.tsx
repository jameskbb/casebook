// Copy to paste into your AI: one shared copy so the More menu item and the
// Ctrl+Shift+C shortcut give the same feedback. It copies the question with
// its findings and sources, framed as stored data for an AI chat, as
// casebook_get_context returns it. A toast says it worked; a failure offers
// Try again.
import { createContext, useCallback, useContext, useMemo, useRef, type ReactNode } from 'react'
import { api } from '../api/api'
import { errorCopy, toApiError } from '../api/errors'
import { useToast } from '../components/ui/Toaster'

export interface CopyContextState {
  copy(ref: string): Promise<void>
}

export const COPIED_FOR_AI = 'Copied. Paste it into your AI.'

const CopyContextContext = createContext<CopyContextState | null>(null)

export function CopyContextProvider({ children }: { children: ReactNode }) {
  const toast = useToast()
  const copyRef = useRef<(ref: string) => Promise<void>>(async () => {})

  const copy = useCallback(
    async (ref: string) => {
      try {
        await api.copyContext(ref)
      } catch (e) {
        const copy = errorCopy(toApiError(e))
        toast.error(copy.message, copy.action === 'retry' ? { label: 'Try again', onClick: () => void copyRef.current(ref) } : undefined)
        return
      }
      toast.success(COPIED_FOR_AI)
    },
    [toast],
  )

  copyRef.current = copy

  const value = useMemo(() => ({ copy }), [copy])
  return <CopyContextContext.Provider value={value}>{children}</CopyContextContext.Provider>
}

export function useCopyContext(): CopyContextState {
  const ctx = useContext(CopyContextContext)
  if (!ctx) throw new Error('useCopyContext must be used inside CopyContextProvider')
  return ctx
}
