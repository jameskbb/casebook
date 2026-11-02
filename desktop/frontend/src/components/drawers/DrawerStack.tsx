// The drill-down drawer stack (D-21, UI-15; UI-SPEC Drawers). open(ref)
// replaces the stack, push(ref) adds a level with "Back to <previous>",
// back() pops one level, closeAll() closes everything and gives focus back
// to the row, chip or History ref that opened the first drawer.
//
// The stack holds refs only. Every render reads the records from the latest
// detail, so the open drawers survive every poll reload and always show the
// current values. A ref missing from the detail shows a not-found line. The
// stack belongs to one investigation: opening another one hides it.
//
// Every move (open, push, back, close) replaces the drawer body, so each
// goes through requestNavigate: with unsaved input in a drawer editor the
// user is asked "Discard this draft?" first (WR-02).
import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import { FileText, Lightbulb, Quote, type LucideIcon } from 'lucide-react'
import type { InvestigationDetail } from '../../api/types'
import { useAppData } from '../../state/appData'
import { useRequestNavigate } from '../../state/dirtyGuard'
import { Drawer } from '../ui/Drawer'
import { EvidenceDrawer } from './EvidenceDrawer'
import { StatementDrawer } from './StatementDrawer'

export interface DrawerStackApi {
  /** Refs from the first drawer to the one on top. */
  stack: string[]
  open(ref: string): void
  push(ref: string): void
  back(): void
  closeAll(): void
}

const DrawerStackContext = createContext<DrawerStackApi | null>(null)

type DrawerKind = 'claim' | 'hypothesis' | 'evidence'

/** The drawer kind for a record ref, or null for refs without a drawer (Q, INV). */
export function drawerKind(ref: string): DrawerKind | null {
  if (/^C-\d+$/.test(ref)) return 'claim'
  if (/^H-\d+$/.test(ref)) return 'hypothesis'
  if (/^E-\d+$/.test(ref)) return 'evidence'
  return null
}

const KIND_META: Record<DrawerKind, { noun: string; icon: LucideIcon }> = {
  claim: { noun: 'Claim', icon: Quote },
  hypothesis: { noun: 'Hypothesis', icon: Lightbulb },
  evidence: { noun: 'Evidence', icon: FileText },
}

interface StackState {
  inv: string | null
  refs: string[]
}

export function DrawerStackProvider({ children }: { children: ReactNode }) {
  const { detail } = useAppData()
  const invRef = detail?.investigation.ref ?? null
  const [state, setState] = useState<StackState>({ inv: null, refs: [] })
  const openerRef = useRef<HTMLElement | null>(null)
  const firstRef = useRef<string | null>(null)
  const [restoreFocus, setRestoreFocus] = useState(false)

  // A stack opened in another investigation is not shown here.
  const refs = state.inv === invRef ? state.refs : []

  const requestNavigate = useRequestNavigate()

  const open = useCallback(
    (ref: string) => {
      if (!drawerKind(ref)) return
      // Remember the opener now: a discard confirmation would take focus.
      const active = document.activeElement
      const opener = active instanceof HTMLElement && active !== document.body ? active : null
      requestNavigate(() => {
        openerRef.current = opener
        firstRef.current = ref
        setState({ inv: invRef, refs: [ref] })
      }, 'drawer')
    },
    [invRef, requestNavigate],
  )

  const push = useCallback(
    (ref: string) => {
      if (!drawerKind(ref)) return
      requestNavigate(() => {
        setState((s) => {
          const current = s.inv === invRef ? s.refs : []
          if (current.length === 0) return { inv: invRef, refs: [ref] }
          if (current[current.length - 1] === ref) return s
          return { inv: invRef, refs: [...current, ref] }
        })
      }, 'drawer')
    },
    [invRef, requestNavigate],
  )

  const back = useCallback(() => {
    requestNavigate(() => {
      setState((s) => (s.refs.length > 1 ? { ...s, refs: s.refs.slice(0, -1) } : s))
    }, 'drawer')
  }, [requestNavigate])

  const closeAll = useCallback(() => {
    requestNavigate(() => {
      setState((s) => ({ ...s, refs: [] }))
      setRestoreFocus(true)
    }, 'drawer')
  }, [requestNavigate])

  // Give focus back once the drawer has unmounted: to the opener when it is
  // still on screen, else to the row of the first record.
  useEffect(() => {
    if (!restoreFocus) return
    setRestoreFocus(false)
    const opener = openerRef.current
    if (opener && opener.isConnected) {
      opener.focus()
      return
    }
    const ref = firstRef.current
    if (!ref) return
    const row = document.querySelector<HTMLElement>(`[data-ref="${ref}"]`)
    const target = row?.querySelector<HTMLElement>('.row-hit') ?? row
    target?.focus()
  }, [restoreFocus])

  const value = useMemo<DrawerStackApi>(
    () => ({ stack: refs, open, push, back, closeAll }),
    [refs, open, push, back, closeAll],
  )

  return (
    <DrawerStackContext.Provider value={value}>
      {children}
      {detail && refs.length > 0 && (
        <TopDrawer detail={detail} refs={refs} push={push} back={back} closeAll={closeAll} />
      )}
    </DrawerStackContext.Provider>
  )
}

interface TopDrawerProps {
  detail: InvestigationDetail
  refs: string[]
  push: (ref: string) => void
  back: () => void
  closeAll: () => void
}

function TopDrawer({ detail, refs, push, back, closeAll }: TopDrawerProps) {
  const top = refs[refs.length - 1]
  const previous = refs.length > 1 ? refs[refs.length - 2] : undefined
  const kind = drawerKind(top) ?? 'evidence'
  const meta = KIND_META[kind]
  let body: ReactNode
  if (kind === 'evidence') {
    const e = detail.evidence.find((x) => x.ref === top)
    body = e ? <EvidenceDrawer evidence={e} detail={detail} onOpenRef={push} onDeleted={closeAll} /> : <NotFound refText={top} />
  } else if (kind === 'claim') {
    const c = detail.claims.find((x) => x.ref === top)
    body = c ? <StatementDrawer kind="claim" record={c} detail={detail} onOpenRef={push} onDeleted={closeAll} /> : <NotFound refText={top} />
  } else {
    const h = detail.hypotheses.find((x) => x.ref === top)
    body = h ? (
      <StatementDrawer kind="hypothesis" record={h} detail={detail} onOpenRef={push} onDeleted={closeAll} />
    ) : (
      <NotFound refText={top} />
    )
  }
  return (
    <Drawer
      heading={`${meta.noun} ${top}`}
      icon={meta.icon}
      kind={kind}
      back={previous ? { ref: previous, onClick: back } : undefined}
      onClose={closeAll}
      contentKey={`${refs.length}:${top}`}
    >
      {body}
    </Drawer>
  )
}

function NotFound({ refText }: { refText: string }) {
  return <p className="drawer-empty">{`${refText} was not found. It may have changed in another app.`}</p>
}

export function useDrawers(): DrawerStackApi {
  const ctx = useContext(DrawerStackContext)
  if (!ctx) throw new Error('useDrawers must be used inside DrawerStackProvider')
  return ctx
}
