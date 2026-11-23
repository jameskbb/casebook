// Files in and out: open a question someone sent, save a backup, and bring
// one back. Every file is chosen in a native window in Go; a cancelled one
// shows nothing. The provider owns the flows and their dialogs so the
// sidebar footer and the first-open screen run the same ones.
//
// Bringing back a backup is two-step: PickBackupToRestore validates the
// chosen file, the confirmation shows what it holds, and only "Replace
// everything" calls RestoreBackup with the pinned path. The result names the
// backup of what was there before, then the app reloads as at launch.
import { createContext, useCallback, useContext, useMemo, useRef, useState, type ReactNode } from 'react'
import { api } from '../../api/api'
import { CLIPBOARD_COPY, errorCopy, FILE_COPY, NOT_SAVED_COPY, plainError, toApiError } from '../../api/errors'
import type { RestorePick } from '../../api/types'
import { useAppData } from '../../state/appData'
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
export const PATH_COPIED = 'Location copied'
export const BAD_BACKUP_COPY = 'Casebook cannot use this backup. It may not be a Casebook backup. Nothing was changed.'

type Problem = { kind: 'import' | 'restore'; body: string }

const DataOpsContext = createContext<DataOps | null>(null)

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
        // A reason this version does not know still reads as a problem with the file.
        const why = plainError(err)
        setProblem({ kind: 'import', body: `${why === NOT_SAVED_COPY ? FILE_COPY : why} Nothing was added.` })
      } else {
        showError(err)
      }
      return
    }
    if (!out || out.cancelled) return
    const r = out.result
    toast.success(
      `Added "${out.title}" to your questions.`,
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
    toast.success(`Backup saved as ${path.split(/[\\/]/).pop()}.`, { label: 'Copy location', onClick: () => void copyPath(path) })
  }, [toast, copyPath])

  const restore = useCallback(async () => {
    let picked: RestorePick
    try {
      picked = await api.pickBackupToRestore()
    } catch (e) {
      const err = toApiError(e)
      if (err.kind === 'conflict' || err.kind === 'not_found') {
        setProblem({ kind: 'restore', body: BAD_BACKUP_COPY })
      } else {
        showError(err)
      }
      return
    }
    if (picked.cancelled) return
    setPick(picked)
  }, [showError])

  // The only call site of RestoreBackup: the "Replace everything" handler. It
  // replaces what every editor shows, so unsaved input is offered
  // for discarding first; keeping it leaves the data untouched.
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
      toast.error(err.kind === 'validation' ? BAD_BACKUP_COPY : errorCopy(err).message)
      return
    } finally {
      setRestoring(false)
      setPick(null)
    }
    // The open question may not exist in the restored data: go back to Home.
    select(null, { discarded: true })
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
          title={problem.kind === 'import' ? 'Casebook cannot open this file' : 'Casebook cannot use this backup'}
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
