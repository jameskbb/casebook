// The detail header's More menu (UI-SPEC Component Contract, MoreMenu; D-06,
// D-20): Edit title and question..., Copy case as text (the context pack
// without the AI data framing Copy for your AI adds, for notes and
// documents; the readable write-up is Share summary in the header), Export
// as JSON..., and Archive or Unarchive case. Export runs the native save
// dialog in Go; a cancelled dialog shows nothing. Archiving is reversible and deletes
// nothing, so it has no confirmation, only a toast with Undo. An archived
// investigation also offers Delete case... (AR-10), which says
// what it removes and cannot be undone.
import { useState } from 'react'
import { Archive, ArchiveRestore, Download, Ellipsis, FileText, Pencil, Trash2 } from 'lucide-react'
import { api } from '../../api/api'
import { errorCopy, toApiError } from '../../api/errors'
import type { Investigation, InvestigationDetail, RecordCounts } from '../../api/types'
import { countPhrase } from '../../lib/restoreCopy'
import { useAppData } from '../../state/appData'
import { useCopyContext } from '../../state/copyContext'
import { useWrite } from '../../state/useWrite'
import { DeleteDialog } from '../dialogs/DeleteDialog'
import { EditInvestigationDialog } from '../dialogs/EditInvestigationDialog'
import { IconButton } from '../ui/IconButton'
import { Menu, type MenuItem } from '../ui/Menu'
import { useToast } from '../ui/Toaster'

export const ARCHIVED_COPY = (ref: string) => `Archived ${ref}. It is hidden from the list and nothing was deleted.`

export const deleteInvestigationBody = (inv: Investigation, counts: RecordCounts | null) => {
  const what = counts
    ? `everything in it: ${countPhrase(counts.evidence, 'piece of evidence', 'pieces of evidence')}, ${countPhrase(counts.claims, 'claim', 'claims')}, ${countPhrase(counts.hypotheses, 'hypothesis', 'hypotheses')}, ${countPhrase(counts.questions, 'question', 'questions')} and its history`
    : 'everything in it and its history'
  return `${inv.ref} "${inv.title}" will be removed with ${what}. Export it first if you might need it again.`
}

export function MoreMenu({ investigation: inv }: { investigation: Investigation }) {
  const write = useWrite()
  const toast = useToast()
  const { detail, investigations, select } = useAppData()
  const { copy } = useCopyContext()
  const [editOpen, setEditOpen] = useState(false)
  const [deleteOpen, setDeleteOpen] = useState(false)
  const ref = inv.ref
  const archived = inv.archived || inv.status === 'archived'

  const showError = (e: unknown) => toast.error(errorCopy(toApiError(e)).message)

  const exportJson = async () => {
    try {
      const res = await api.exportInvestigation(ref)
      if (res.cancelled) return
      toast.success(`Exported ${ref} to ${res.file_name}.`)
    } catch (e) {
      showError(e)
    }
  }

  const unarchive = async () => {
    try {
      const done = await write(() => api.unarchiveInvestigation(ref))
      if (done) toast.success(`Unarchived ${ref}.`)
    } catch (e) {
      showError(e)
    }
  }

  const archive = async () => {
    try {
      const done = await write(() => api.archiveInvestigation(ref))
      if (!done) return
      toast.success(ARCHIVED_COPY(ref), {
        label: 'Undo',
        onClick: () => {
          void write(() => api.unarchiveInvestigation(ref)).catch(showError)
        },
      })
    } catch (e) {
      showError(e)
    }
  }

  const items: MenuItem[] = [
    { label: 'Edit title and question...', icon: Pencil, onSelect: () => setEditOpen(true) },
    { label: 'Copy case as text', icon: FileText, onSelect: () => void copy(ref, { plain: true }) },
    { label: 'Export as JSON...', icon: Download, onSelect: () => void exportJson() },
    archived
      ? { label: 'Unarchive case', icon: ArchiveRestore, onSelect: () => void unarchive() }
      : { label: 'Archive case', icon: Archive, onSelect: () => void archive() },
  ]
  // Delete is offered only once archived (the engine refuses otherwise), so
  // archive stays the everyday way to put an investigation away.
  if (archived) {
    items.push({ label: 'Delete case...', icon: Trash2, separatorBefore: true, onSelect: () => setDeleteOpen(true) })
  }
  const counts = investigations.find((i) => i.ref === ref)?.counts ?? (detail?.investigation.ref === ref ? countsOf(detail) : null)

  return (
    <>
      <Menu
        label="More actions"
        align="end"
        items={items}
        trigger={(p) => <IconButton {...p} icon={Ellipsis} label="More actions" />}
      />
      {editOpen && <EditInvestigationDialog investigation={inv} onClose={() => setEditOpen(false)} />}
      {deleteOpen && (
        <DeleteDialog
          title={`Delete ${ref}?`}
          body={deleteInvestigationBody(inv, counts)}
          confirmLabel="Delete case"
          onConfirm={async () => {
            const done = await write(() => api.deleteInvestigation(ref).then(() => true), { rethrowConflict: true })
            if (done) {
              toast.success(`Deleted ${ref}.`)
              select(null)
            }
            return Boolean(done)
          }}
          onClose={() => setDeleteOpen(false)}
        />
      )}
    </>
  )
}

function countsOf(d: InvestigationDetail): RecordCounts {
  return {
    evidence: d.evidence.length,
    claims: d.claims.length,
    hypotheses: d.hypotheses.length,
    questions: d.questions.length,
    open_questions: d.questions.filter((q) => q.status === 'open').length,
  }
}
