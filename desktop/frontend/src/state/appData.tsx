// Shared app data: startup state, the investigation list, the selection and
// the open investigation's detail. Every component reads it through
// useAppData(); writes go through useWrite(), which calls refresh().
//
// Live refresh (D-10): the provider polls ChangeToken through useChangeToken
// and calls refresh() when the token moves. refresh() reads the token before
// reloading and stores it as the baseline afterwards, so an own write
// reloads once and the next poll stays quiet, while a write that lands during
// the reload still moves the token and reloads again. Reloads are silent and
// keep the selection; components keep their own editor state.
//
// A failed load never counts as handled: the baseline is marked stale so the
// next poll reloads again. A background detail reload that fails keeps the
// last loaded detail on screen with detailStatus 'error' (DetailView shows a
// banner), except not_found, which clears it: the investigation is gone (for
// example a restore in another app), so it must not stay editable.
import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import { api } from '../api/api'
import { useChangeToken } from '../hooks/useChangeToken'
import { useRequestNavigate } from './dirtyGuard'
import { ApiError, toApiError } from '../api/errors'
import type { InvestigationDetail, InvestigationSummary, StartupInfo } from '../api/types'

export const DEFAULT_ACTIVITY_LIMIT = 20
/**
 * How many investigations the sidebar asks for: the engine maximum
 * (docs/api.md section 7). Without a limit the engine returns only 100, and
 * older investigations would vanish from the list without a word.
 */
export const LIST_LIMIT = 1000

export type ListStatus = 'loading' | 'ready' | 'error'
export type DetailStatus = 'idle' | 'loading' | 'ready' | 'error'

export interface SelectOptions {
  /** A record ref (E-001, C-002, H-001) to open in a drawer once loaded. */
  openRecord?: string
  /** The caller already asked "Discard this draft?" (restore does, before it replaces data). */
  discarded?: boolean
}

export interface AppData {
  startup: StartupInfo | null
  /** Asks the engine again whether the data opened (after startup recovery). */
  retryStartup(): void
  dataLocation: string
  investigations: InvestigationSummary[]
  listStatus: ListStatus
  includeArchived: boolean
  setIncludeArchived(v: boolean): void
  selectedRef: string | null
  select(ref: string | null, opts?: SelectOptions): void
  pendingOpen: string | null
  clearPendingOpen(): void
  detail: InvestigationDetail | null
  detailStatus: DetailStatus
  detailError: ApiError | null
  activityLimit: number
  setActivityLimit(n: number): void
  /** Increments after every completed refresh. */
  dataVersion: number
  /** Reloads the list and the open detail. */
  refresh(): Promise<void>
  /** True after three consecutive ChangeToken failures, until the next success. */
  pollTrouble: boolean
}

/**
 * The investigation to open at launch (and after a restore): the active one,
 * else the most recently updated non-archived one, else none. The list is
 * already ordered newest updated first.
 */
export function pickLaunchSelection(list: InvestigationSummary[]): string | null {
  const active = list.find((i) => i.is_active)
  if (active) return active.ref
  const open = list.find((i) => !i.archived)
  return open ? open.ref : null
}

const AppDataContext = createContext<AppData | null>(null)

export function AppDataProvider({ children }: { children: ReactNode }) {
  const [startup, setStartup] = useState<StartupInfo | null>(null)
  const [startupAttempt, setStartupAttempt] = useState(0)
  const retryStartup = useCallback(() => setStartupAttempt((n) => n + 1), [])
  const [dataLocation, setDataLocation] = useState('')
  const [investigations, setInvestigations] = useState<InvestigationSummary[]>([])
  const [listStatus, setListStatus] = useState<ListStatus>('loading')
  const [includeArchived, setIncludeArchivedState] = useState(false)
  const [selectedRef, setSelectedRef] = useState<string | null>(null)
  const [pendingOpen, setPendingOpen] = useState<string | null>(null)
  const [detail, setDetail] = useState<InvestigationDetail | null>(null)
  const [detailStatus, setDetailStatus] = useState<DetailStatus>('idle')
  const [detailError, setDetailError] = useState<ApiError | null>(null)
  const [activityLimit, setActivityLimitState] = useState(DEFAULT_ACTIVITY_LIMIT)
  const [dataVersion, setDataVersion] = useState(0)

  // Latest values for async callbacks.
  const selectedRefRef = useRef<string | null>(null)
  const includeArchivedRef = useRef(false)
  const activityLimitRef = useRef(DEFAULT_ACTIVITY_LIMIT)
  const listSeq = useRef(0)
  const detailSeq = useRef(0)
  const mounted = useRef(true)
  const refreshRef = useRef<() => Promise<void>>(async () => {})
  // True until a list load succeeds and the launch selection is picked; a
  // failed first load picks it on the first reload that succeeds.
  const launchPending = useRef(true)

  const { setBaseline, markStale, trouble: pollTrouble } = useChangeToken({
    enabled: startup?.ok === true,
    onChange: () => void refreshRef.current(),
  })

  /** The current token, or null when it cannot be read. */
  const readToken = useCallback(async (): Promise<string | null> => {
    try {
      return await api.changeToken()
    } catch {
      return null
    }
  }, [])

  useEffect(() => {
    mounted.current = true
    return () => {
      mounted.current = false
    }
  }, [])

  /**
   * Loads the list. Resolves to the list, to null when the load failed, or
   * to undefined when a newer load superseded this one.
   */
  const loadList = useCallback(async (): Promise<InvestigationSummary[] | null | undefined> => {
    const seq = ++listSeq.current
    try {
      const list = await api.listInvestigations({ include_archived: includeArchivedRef.current, limit: LIST_LIMIT })
      if (!mounted.current || seq !== listSeq.current) return undefined
      setInvestigations(list)
      setListStatus('ready')
      return list
    } catch {
      if (!mounted.current || seq !== listSeq.current) return undefined
      setListStatus('error')
      return null
    }
  }, [])

  /**
   * Loads the detail of ref. Resolves to false only when the load failed;
   * a missing investigation (not_found) is a successful answer.
   */
  const loadDetail = useCallback(async (ref: string | null, keepPrevious: boolean): Promise<boolean> => {
    const seq = ++detailSeq.current
    if (!ref) {
      setDetail(null)
      setDetailStatus('idle')
      setDetailError(null)
      return true
    }
    if (!keepPrevious) {
      setDetail(null)
      setDetailStatus('loading')
    }
    try {
      const d = await api.getInvestigationDetail(ref, { activity_limit: activityLimitRef.current })
      if (!mounted.current || seq !== detailSeq.current) return true
      setDetail(d)
      setDetailStatus('ready')
      setDetailError(null)
      return true
    } catch (e) {
      if (!mounted.current || seq !== detailSeq.current) return true
      const err = toApiError(e)
      setDetailError(err)
      setDetailStatus('error')
      if (!keepPrevious || err.kind === 'not_found') setDetail(null)
      return err.kind === 'not_found'
    }
  }, [])

  /** Loads a detail the user (or startup) asked for; a failure is retried by the next poll. */
  const openDetail = useCallback(
    (ref: string | null) => {
      void loadDetail(ref, false).then((ok) => {
        if (!ok) markStale()
      })
    },
    [loadDetail, markStale],
  )

  /** Opens the launch selection once, after the first list load that succeeds. */
  const pickLaunch = useCallback(
    (list: InvestigationSummary[]) => {
      if (!launchPending.current) return
      launchPending.current = false
      if (selectedRefRef.current !== null) return
      const ref = pickLaunchSelection(list)
      if (ref) {
        selectedRefRef.current = ref
        setSelectedRef(ref)
        openDetail(ref)
      }
    },
    [openDetail],
  )

  // Startup: learn whether the database opened, then load the list and pick
  // the launch selection.
  useEffect(() => {
    let cancelled = false
    ;(async () => {
      let info: StartupInfo
      try {
        info = await api.startupState()
      } catch (e) {
        info = { ok: false, kind: 'other', message: toApiError(e).message, db_path: '' }
      }
      if (cancelled) return
      setStartup(info)
      setDataLocation(info.db_path)
      if (!info.ok) return
      api.dataLocation().then(
        (loc) => {
          if (!cancelled && loc) setDataLocation(loc)
        },
        () => {},
      )
      const token = await readToken()
      const list = await loadList()
      if (list === null) markStale()
      else if (token !== null) setBaseline(token)
      if (cancelled || !list) return
      pickLaunch(list)
    })()
    return () => {
      cancelled = true
    }
  }, [loadList, pickLaunch, readToken, setBaseline, markStale, startupAttempt])

  // Opening another investigation unmounts every editor on the page, so it
  // asks before discarding unsaved input (dirtyGuard.tsx).
  const requestNavigate = useRequestNavigate()
  const select = useCallback(
    (ref: string | null, opts?: SelectOptions) => {
      if (ref === selectedRefRef.current) {
        setPendingOpen(opts?.openRecord ?? null)
        if (ref) void loadDetail(ref, true)
        return
      }
      const go = () => {
        setPendingOpen(opts?.openRecord ?? null)
        launchPending.current = false
        selectedRefRef.current = ref
        setSelectedRef(ref)
        activityLimitRef.current = DEFAULT_ACTIVITY_LIMIT
        setActivityLimitState(DEFAULT_ACTIVITY_LIMIT)
        openDetail(ref)
      }
      if (opts?.discarded) go()
      else requestNavigate(go)
    },
    [loadDetail, openDetail, requestNavigate],
  )

  const clearPendingOpen = useCallback(() => setPendingOpen(null), [])

  const refresh = useCallback(async () => {
    const token = await readToken()
    const [list, detailOk] = await Promise.all([loadList(), loadDetail(selectedRefRef.current, true)])
    // Only a reload that fully succeeded handles the change; otherwise the
    // next poll tries again.
    if (list === null || !detailOk) markStale()
    else if (token !== null) setBaseline(token)
    if (list && mounted.current) pickLaunch(list)
    if (mounted.current) setDataVersion((v) => v + 1)
  }, [loadList, loadDetail, readToken, setBaseline, markStale, pickLaunch])
  refreshRef.current = refresh

  const setIncludeArchived = useCallback(
    (v: boolean) => {
      includeArchivedRef.current = v
      setIncludeArchivedState(v)
      void loadList()
    },
    [loadList],
  )

  const setActivityLimit = useCallback(
    (n: number) => {
      activityLimitRef.current = n
      setActivityLimitState(n)
      void loadDetail(selectedRefRef.current, true)
    },
    [loadDetail],
  )

  const value = useMemo<AppData>(
    () => ({
      startup,
      retryStartup,
      dataLocation,
      investigations,
      listStatus,
      includeArchived,
      setIncludeArchived,
      selectedRef,
      select,
      pendingOpen,
      clearPendingOpen,
      detail,
      detailStatus,
      detailError,
      activityLimit,
      setActivityLimit,
      dataVersion,
      refresh,
      pollTrouble,
    }),
    [
      startup,
      retryStartup,
      dataLocation,
      investigations,
      listStatus,
      includeArchived,
      setIncludeArchived,
      selectedRef,
      select,
      pendingOpen,
      clearPendingOpen,
      detail,
      detailStatus,
      detailError,
      activityLimit,
      setActivityLimit,
      dataVersion,
      refresh,
      pollTrouble,
    ],
  )

  return <AppDataContext.Provider value={value}>{children}</AppDataContext.Provider>
}

export function useAppData(): AppData {
  const ctx = useContext(AppDataContext)
  if (!ctx) throw new Error('useAppData must be used inside AppDataProvider')
  return ctx
}
