// Unsaved input outside dialogs (the drawer editors and the inline
// conclusion editor) must not vanish on navigation. Each such editor calls
// useDirtyEditor(dirty); navigation that would unmount it (pushing, going
// back in or closing the drawer stack, opening another investigation) goes
// through requestNavigate, which asks "Discard this draft?" first, like a
// dirty Dialog does. Drawer navigation asks only about drawer editors (the
// conclusion editor survives it); opening another investigation asks about
// all of them. Without a provider every navigation runs at once.
import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import { Button } from '../components/ui/Button'
import { Dialog } from '../components/ui/Dialog'

/** Where an editor lives: the drawer stack, or the detail page under it. */
export type DirtyScope = 'drawer' | 'detail'

interface DirtyGuardApi {
  /** Marks one editor as holding unsaved input; returns the undo. */
  markDirty(id: symbol, scope: DirtyScope): () => void
  /**
   * Runs go now when no editor in scope is dirty ('all' counts every
   * editor), else after the user chooses Discard.
   */
  requestNavigate(go: () => void, scope?: DirtyScope | 'all'): void
}

const DirtyGuardContext = createContext<DirtyGuardApi | null>(null)

export function DirtyGuardProvider({ children }: { children: ReactNode }) {
  const dirty = useRef(new Map<symbol, DirtyScope>())
  const [pending, setPending] = useState<{ go: () => void } | null>(null)
  const keepEditingRef = useRef<HTMLButtonElement>(null)

  const markDirty = useCallback((id: symbol, scope: DirtyScope) => {
    dirty.current.set(id, scope)
    return () => {
      dirty.current.delete(id)
    }
  }, [])

  const requestNavigate = useCallback((go: () => void, scope: DirtyScope | 'all' = 'all') => {
    const blocked = [...dirty.current.values()].some((s) => scope === 'all' || s === scope)
    if (blocked) setPending({ go })
    else go()
  }, [])

  const api = useMemo<DirtyGuardApi>(() => ({ markDirty, requestNavigate }), [markDirty, requestNavigate])

  return (
    <DirtyGuardContext.Provider value={api}>
      {children}
      {pending && (
        <Dialog
          title="Discard this draft?"
          size="confirm"
          onClose={() => setPending(null)}
          initialFocus={keepEditingRef}
          footer={
            <>
              <Button variant="secondary" ref={keepEditingRef} onClick={() => setPending(null)}>
                Keep editing
              </Button>
              <Button
                variant="destructive"
                onClick={() => {
                  const { go } = pending
                  setPending(null)
                  // The editors unregister themselves as they unmount.
                  go()
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
    </DirtyGuardContext.Provider>
  )
}

/** Registers unsaved input while dirty is true and the editor is mounted. */
export function useDirtyEditor(dirty: boolean, scope: DirtyScope): void {
  const ctx = useContext(DirtyGuardContext)
  const id = useRef<symbol | null>(null)
  if (id.current === null) id.current = Symbol('dirty-editor')
  useEffect(() => {
    if (!ctx || !dirty || !id.current) return
    return ctx.markDirty(id.current, scope)
  }, [ctx, dirty, scope])
}

export type RequestNavigate = (go: () => void, scope?: DirtyScope | 'all') => void

/** Wraps navigation that would discard unsaved editors. */
export function useRequestNavigate(): RequestNavigate {
  const ctx = useContext(DirtyGuardContext)
  return useMemo<RequestNavigate>(() => ctx?.requestNavigate ?? ((go) => go()), [ctx])
}
