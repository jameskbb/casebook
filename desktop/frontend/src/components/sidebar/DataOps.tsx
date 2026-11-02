// Data operations (UI-20, UI-21; D-06, D-07): import an investigation, back
// up all data, and restore a backup. Every file is chosen in a native dialog
// in Go; a cancelled dialog shows nothing. The provider owns the flows and
// their dialogs so the sidebar footer and the empty main pane run the same
// ones.
//
// Restore is two-step: PickBackupToRestore validates the chosen file, the
// confirmation shows what it holds, and only "Replace all data" calls
// RestoreBackup with the pinned path. The result names the safety copy, then
// the app reloads and reselects as at launch.
import { createContext, useCallback, useContext, useMemo, useRef, useState, type ReactNode } from 'react'
import { api } from '../../api/api'
import { asSentence, CLIPBOARD_COPY, cleanMessage, errorCopy, toApiError } from '../../api/errors'
import type { RestorePick } from '../../api/types'
import { LIST_LIMIT, pickLaunchSelection, useAppData } from '../../state/appData'
import { useRequestNavigate } from '../../state/dirtyGuard'
import { useWrite } from '../../state/useWrite'
import { FileProblemDialog } from '../dialogs/FileProblemDialog'
import { RestoreConfirmDialog } from '../dialogs/RestoreConfirmDialog'
import { RestoreResultDialog } from '../dialogs/RestoreResultDialog'
import { useToast } from '../ui/Toaster'

export interface DataOps {
  importInvestigation(): Promise<void>
  backUp(): Promise<void>
  restore(): Promise<void>
}

export const BACKUP_EXISTS_COPY = 'A file with that name already exists. Choose a new name.'
export const PATH_COPIED = 'Path copied to the clipboard'

type Problem = { kind: 'import' | 'restore'; body: string }

const DataOpsContext = createContext<DataOps | null>(null)

/** "{message}." without doubling a closing period. */
function withPeriod(msg: string): string {
  const m = msg.trim()
  return /[.!?]$/.test(m) ? m : `${m}.`
}

export function DataOpsProvider({ children }: { children: ReactNode }) {
  const { refresh, select, includeArchived } = useAppData()
  const requestNavigate = useRequestNavigate()
  const write = useWrite()
  const toast = useToast()
  const [problem, setProblem] = useState<Problem | null>(null)
  const [pick, setPick] = useState<RestorePick | null>(null)
  const [restoring, setRestoring] = useState(false)
  const [result, setResult] = useState<{ fileName: string; safetyCopyPath: string } | null>(null)
  const includeArchivedRef = useRef(includeArchived)
  includeArchivedRef.current = includeArchived
  const importRef = useRef<() => Promise<void>>(async () => {})

  const showError = useCallback((e: unknown) => toast.error(errorCopy(toApiError(e)).message), [toast])

  const copyPath = useCallback(
    async (path: string) => {
      try {
        await api.copyText(path)
        toast.announce(PATH_COPIED)
      } catch {
        toast.error(CLIPBOARD_COPY)
      }
    },
    [toast],
  )

  const importInvestigation = useCallback(async () => {
    let out
    try {
      out = await write(() => api.importInvestigation(), { onRetry: () => void importRef.current() })
    } catch (e) {
      const err = toApiError(e)
      if (err.kind === 'validation') {
        const what = err.field ? `${err.field}: ${err.message}` : cleanMessage(err.message)
        setProblem({ kind: 'import', body: `${withPeriod(what)} Nothing was added.` })
      } else {
        showError(err)
      }
      return
    }
    if (!out || out.cancelled) return
    const r = out.result
    toast.success(
      r.ref_changed
        ? `Imported "${out.title}" as ${r.investigation_ref} because ${r.original_ref} was already in use.`
        : `Imported "${out.title}" as ${r.investigation_ref}.`,
    )
    select(r.investigation_ref)
  }, [write, toast, select, showError])
  importRef.current = importInvestigation

  const backUp = useCallback(async () => {
    let out
    try {
      out = await api.backUp()
    } catch (e) {
      const err = toApiError(e)
      toast.error(err.kind === 'conflict' ? BACKUP_EXISTS_COPY : errorCopy(err).message)
      return
    }
    if (out.cancelled) return
    const path = out.info.path
    toast.success(`Backup saved to ${path}.`, { label: 'Copy path', onClick: () => void copyPath(path) })
  }, [toast, copyPath])

  const restore = useCallback(async () => {
    let picked: RestorePick
    try {
      picked = await api.pickBackupToRestore()
    } catch (e) {
      const err = toApiError(e)
      if (err.kind === 'conflict' || err.kind === 'not_found') {
        setProblem({ kind: 'restore', body: `${asSentence(err.message)} Nothing was changed.` })
      } else {
        showError(err)
      }
      return
    }
    if (picked.cancelled) return
    setPick(picked)
  }, [showError])

  // The only call site of RestoreBackup: the "Replace all data" handler. A
  // restore replaces what every editor shows, so unsaved input is offered
  // for discarding first (AR-24); keeping it leaves the data untouched.
  const confirmRestore = () => {
    if (!pick || restoring) return
    requestNavigate(() => void runRestore(), 'all')
  }

  const runRestore = async () => {
    if (!pick || restoring) return
    setRestoring(true)
    try {
      const res = await api.restoreBackup(pick.path)
      setResult({ fileName: pick.file_name, safetyCopyPath: res.safety_copy_path })
    } catch (e) {
      const err = toApiError(e)
      toast.error(err.kind === 'validation' ? `${asSentence(err.message)} Nothing was changed.` : errorCopy(err).message)
      return
    } finally {
      setRestoring(false)
      setPick(null)
    }
    try {
      const list = await api.listInvestigations({ include_archived: includeArchivedRef.current, limit: LIST_LIMIT })
      select(pickLaunchSelection(list), { discarded: true })
    } catch {
      // The refresh below still reloads the list.
    }
    await refresh()
  }

  const chooseAnother = () => {
    const kind = problem?.kind
    setProblem(null)
    if (kind === 'import') void importInvestigation()
    else if (kind === 'restore') void restore()
  }

  const value = useMemo<DataOps>(() => ({ importInvestigation, backUp, restore }), [importInvestigation, backUp, restore])

  return (
    <DataOpsContext.Provider value={value}>
      {children}
      {problem && (
        <FileProblemDialog
          title={problem.kind === 'import' ? "This file can't be imported" : "This file can't be restored"}
          body={problem.body}
          onChooseAnother={chooseAnother}
          onClose={() => setProblem(null)}
        />
      )}
      {pick && (
        <RestoreConfirmDialog
          fileName={pick.file_name}
          info={pick.info}
          busy={restoring}
          onConfirm={() => void confirmRestore()}
          onCancel={() => {
            if (!restoring) setPick(null)
          }}
        />
      )}
      {result && (
        <RestoreResultDialog
          fileName={result.fileName}
          safetyCopyPath={result.safetyCopyPath}
          onCopyPath={() => void copyPath(result.safetyCopyPath)}
          onDone={() => setResult(null)}
        />
      )}
    </DataOpsContext.Provider>
  )
}

export function useDataOps(): DataOps {
  const ctx = useContext(DataOpsContext)
  if (!ctx) throw new Error('useDataOps must be used inside DataOpsProvider')
  return ctx
}
