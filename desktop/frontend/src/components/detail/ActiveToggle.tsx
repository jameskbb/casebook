// The AI marker. Off: a secondary button with a pin, "Have my AI work on
// this". On: the primary tint with an 8px accent dot, "Your AI is working on
// this", aria-pressed true. Clicking it when on clears the marker
// (ClearActiveInvestigation is the only way to unset it, docs/api.md section
// 5), but only while it is still on this question: if an AI moved it since
// the last reload, nothing changes and the page reloads. The button is busy
// while the call runs.
import { useState } from 'react'
import { Pin } from 'lucide-react'
import { api } from '../../api/api'
import { errorCopy, toApiError } from '../../api/errors'
import type { Investigation } from '../../api/types'
import { useWrite } from '../../state/useWrite'
import { useToast } from '../ui/Toaster'
import '../ui/Button.css'
import './ActiveToggle.css'

export const ACTIVE_ON_LABEL = 'Your AI is working on this'
export const ACTIVE_OFF_LABEL = 'Have my AI work on this'
export const ACTIVE_TITLE_OFF = 'Your AI will save what it finds under this question.'
export const ACTIVE_TITLE_ON = 'Your AI saves what it finds under this question. Click to stop.'
export const ACTIVE_SET_COPY = 'Your AI will save what it finds here.'
export const ACTIVE_CLEARED_COPY = 'Your AI is no longer working on this question.'

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
        if (cleared !== undefined) toast.success(ACTIVE_CLEARED_COPY)
      } else {
        const done = await write(() => api.setActiveInvestigation(inv.ref))
        if (done) toast.success(ACTIVE_SET_COPY)
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
      <span className="btn-label">{on ? ACTIVE_ON_LABEL : ACTIVE_OFF_LABEL}</span>
    </button>
  )
}
