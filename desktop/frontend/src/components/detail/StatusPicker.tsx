// Status picker (UI-06; UI-SPEC Detail header bar): a ghost button showing
// the status label with ChevronDown, opening a menu of New, Investigating,
// Blocked and Resolved. Archived is not in the picker; an archived
// investigation shows "Archived" disabled (Unarchive lives in the banner and
// the More menu).
import { ChevronDown } from 'lucide-react'
import { api } from '../../api/api'
import { errorCopy, toApiError } from '../../api/errors'
import type { Investigation, InvestigationStatus } from '../../api/types'
import { investigationStatusLabels, investigationStatusOptions } from '../../lib/labels'
import { useWrite } from '../../state/useWrite'
import { StatusLabel } from '../record/StatusLabel'
import { Menu } from '../ui/Menu'
import { useToast } from '../ui/Toaster'
import '../ui/Button.css'
import './StatusPicker.css'

export function StatusPicker({ investigation: inv }: { investigation: Investigation }) {
  const write = useWrite()
  const toast = useToast()

  if (inv.archived || inv.status === 'archived') {
    return (
      <button type="button" className="btn btn-ghost status-picker" disabled aria-disabled="true" aria-label="Status: Archived">
        <span className="status-label status-archived">Archived</span>
      </button>
    )
  }

  const setStatus = async (status: InvestigationStatus) => {
    if (status === inv.status) return
    try {
      await write(() => api.updateInvestigation(inv.ref, { status }))
    } catch (e) {
      toast.error(errorCopy(toApiError(e)).message)
    }
  }

  const current = investigationStatusLabels[inv.status] ?? inv.status
  return (
    <Menu
      label="Status"
      items={investigationStatusOptions.map((s) => ({
        label: investigationStatusLabels[s],
        checked: s === inv.status,
        onSelect: () => void setStatus(s),
      }))}
      trigger={(p) => (
        <button {...p} type="button" className="btn btn-ghost status-picker" aria-label={`Status: ${current}`}>
          <StatusLabel status={inv.status} />
          <ChevronDown className="status-picker-chevron" size={16} strokeWidth={1.75} aria-hidden="true" />
        </button>
      )}
    />
  )
}
