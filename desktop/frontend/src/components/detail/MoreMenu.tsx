// The header's More menu: Edit question..., Check findings one by one, Copy
// to paste into your AI, Save a copy to send to someone..., and Archive
// question or Put back in the list. Saving runs the native save window in Go;
// a cancelled one shows nothing. Archiving can be undone and deletes nothing,
// so it has no confirmation, only a toast with Undo. An archived question
// also offers Delete question..., which says what it removes and cannot be
// undone.
import { useState } from 'react'
import { Archive, ArchiveRestore, ClipboardCopy, Download, Ellipsis, ListChecks, Pencil, Trash2 } from 'lucide-react'
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

export const ARCHIVED_COPY = 'Archived this question. It is hidden from the list and nothing was deleted.'

export const BACK_IN_LIST_COPY = 'This question is back in the list.'
export const PUT_BACK_LABEL = 'Put back in the list'
export const CHECK_FINDINGS_LABEL = 'Check findings one by one'
export const COPY_FOR_AI_LABEL = 'Copy to paste into your AI'
export const SAVE_COPY_LABEL = 'Save a copy to send to someone...'

export const deleteInvestigationBody = (inv: Investigation, counts: RecordCounts | null) => {
  const what = counts
    ? `everything in it: ${countPhrase(counts.evidence, 'source', 'sources')}, ${countPhrase(counts.claims + counts.hypotheses, 'finding', 'findings')}, ${countPhrase(counts.questions, 'follow-up question', 'follow-up questions')} and its history`
    : 'everything in it and its history'
  return `"${inv.title}" will be removed with ${what}. Save a copy first if you might need it again.`
}

export function MoreMenu({ investigation: inv, onCheckFindings }: { investigation: Investigation; onCheckFindings?: () => void }) {
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
      toast.success(`Saved a copy as ${res.file_name}.`)
    } catch (e) {
      showError(e)
    }
  }

  const putBack = async () => {
    try {
      const done = await write(() => api.unarchiveInvestigation(ref))
      if (done) toast.success(BACK_IN_LIST_COPY)
    } catch (e) {
      showError(e)
    }
  }

  const archive = async () => {
    try {
      const done = await write(() => api.archiveInvestigation(ref))
      if (!done) return
      toast.success(ARCHIVED_COPY, {
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
    { label: 'Edit question...', icon: Pencil, onSelect: () => setEditOpen(true) },
    ...(onCheckFindings ? [{ label: CHECK_FINDINGS_LABEL, icon: ListChecks, onSelect: onCheckFindings }] : []),
    { label: COPY_FOR_AI_LABEL, icon: ClipboardCopy, onSelect: () => void copy(ref) },
    { label: SAVE_COPY_LABEL, icon: Download, onSelect: () => void exportJson() },
    archived
      ? { label: PUT_BACK_LABEL, icon: ArchiveRestore, onSelect: () => void putBack() }
      : { label: 'Archive question', icon: Archive, onSelect: () => void archive() },
  ]
  // Delete is offered only once archived (the engine refuses otherwise), so
  // archive stays the everyday way to put a question away.
  if (archived) {
    items.push({ label: 'Delete question...', icon: Trash2, separatorBefore: true, onSelect: () => setDeleteOpen(true) })
  }
  const counts = investigations.find((i) => i.ref === ref)?.counts ?? (detail?.investigation.ref === ref ? countsOf(detail) : null)

  return (
    <>
      <Menu
        label="More"
        align="end"
        items={items}
        trigger={(p) => <IconButton {...p} icon={Ellipsis} label="More" />}
      />
      {editOpen && <EditInvestigationDialog investigation={inv} onClose={() => setEditOpen(false)} />}
      {deleteOpen && (
        <DeleteDialog
          title="Delete this question?"
          body={deleteInvestigationBody(inv, counts)}
          confirmLabel="Delete question"
          onConfirm={async () => {
            const done = await write(() => api.deleteInvestigation(ref).then(() => true), { rethrowConflict: true })
            if (done) {
              toast.success('Deleted the question.')
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
