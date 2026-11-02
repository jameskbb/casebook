// Use with my AI / Your AI's current case (UI-18; UI-SPEC Component
// Contract, ActiveToggle). Off: a secondary button with Pin "Use with my AI".
// On: the primary tint with an 8px accent dot and "Your AI's current case",
// aria-pressed true. Clicking it when on clears the active investigation
// (ClearActiveInvestigation is the only way to unset it, docs/api.md section
// 5), but only while it is still this one: if an AI client or the CLI chose
// another since the last reload, nothing changes and the view refreshes. The
// button is busy while the call runs.
import { useState } from 'react'
import { Pin } from 'lucide-react'
import { api } from '../../api/api'
import { errorCopy, toApiError } from '../../api/errors'
import type { Investigation } from '../../api/types'
import { useWrite } from '../../state/useWrite'
import { useToast } from '../ui/Toaster'
import '../ui/Button.css'
import './ActiveToggle.css'

export const ACTIVE_TITLE_OFF = 'Click so your AI tools start from this case.'
export const NOT_ACTIVE_COPY = (ref: string) =>
  `${ref} was no longer your AI's current case, so nothing changed.`
export const ACTIVE_TITLE_ON = 'Your AI tools start from this case. Click to stop.'

export function ActiveToggle({ investigation: inv }: { investigation: Investigation }) {
  const write = useWrite()
  const toast = useToast()
  const [busy, setBusy] = useState(false)
  const on = inv.is_active

  const toggle = async () => {
    if (busy) return
    setBusy(true)
    try {
      if (on) {
        const cleared = await write(() => api.clearActiveInvestigation(inv.ref))
        if (cleared === true) toast.success("No case is your AI's current case now.")
        else if (cleared === false) toast.success(NOT_ACTIVE_COPY(inv.ref))
      } else {
        const done = await write(() => api.setActiveInvestigation(inv.ref))
        if (done) toast.success(`${inv.ref} is now your AI's current case.`)
      }
    } catch (e) {
      toast.error(errorCopy(toApiError(e)).message)
    } finally {
      setBusy(false)
    }
  }

  return (
    <button
      type="button"
      className={['btn', 'active-toggle', on ? 'is-on' : 'btn-secondary'].join(' ')}
      aria-pressed={on}
      title={on ? ACTIVE_TITLE_ON : ACTIVE_TITLE_OFF}
      disabled={busy}
      aria-busy={busy || undefined}
      onClick={() => void toggle()}
    >
      {on ? (
        <span className="active-toggle-dot" aria-hidden="true" />
      ) : (
        <Pin className="btn-icon" size={16} strokeWidth={1.75} aria-hidden="true" />
      )}
      <span className="btn-label">{on ? "Your AI's current case" : 'Use with my AI'}</span>
    </button>
  )
}
