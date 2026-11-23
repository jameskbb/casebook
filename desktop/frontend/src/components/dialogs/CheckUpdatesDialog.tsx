// Look for updates. Opened from the Settings menu; nothing is asked until
// the person chooses Look now. The Go side makes the one request
// (internal/update) and never downloads anything: an update is a link to its
// download page, and the person installs it themselves. The result is
// announced in a polite live region; a failure says so in plain words, with
// Look now to try again.
import { useRef, useState } from 'react'
import { Copy, ExternalLink, RefreshCw } from 'lucide-react'
import { api } from '../../api/api'
import { CLIPBOARD_COPY, errorCopy, toApiError } from '../../api/errors'
import type { UpdateCheck } from '../../api/types'
import { Button } from '../ui/Button'
import { Dialog } from '../ui/Dialog'
import { useToast } from '../ui/Toaster'
import './CheckUpdatesDialog.css'

export const UPDATE_INTRO_COPY =
  'Casebook will look online for a newer version. It sends nothing about you or this computer and downloads nothing. You choose when to update.'
export const UPDATE_HOW_COPY =
  'Download it from the download page, then install it over this one. Your questions stay where they are.'
export const LINK_COPIED = 'Download link copied.'
export const UPDATE_FAILED_COPY = 'Casebook could not look for a newer version. See that you are online, then try again.'

/** v0.8.3 from a longer test version name. */
const short = (v: string) => v.replace(/-.*$/, '')

/** The result lines for one look, by state. */
export function updateLines(r: UpdateCheck): string[] {
  switch (r.state) {
    case 'update_available':
      return [`Casebook ${r.latest} is available. You have ${r.current}.`, UPDATE_HOW_COPY]
    case 'up_to_date':
      return [`You have the latest version, Casebook ${r.latest}.`]
    case 'ahead':
      return [`You have Casebook ${short(r.current)}. The newest version for everyone is ${r.latest}.`]
    case 'unknown':
    default:
      return [`This is a test version (${short(r.current)}). The newest version for everyone is Casebook ${r.latest}.`]
  }
}

export interface CheckUpdatesDialogProps {
  onClose: () => void
}

export function CheckUpdatesDialog({ onClose }: CheckUpdatesDialogProps) {
  const toast = useToast()
  const checkRef = useRef<HTMLButtonElement>(null)
  const [result, setResult] = useState<UpdateCheck | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [checking, setChecking] = useState(false)

  const check = async () => {
    setChecking(true)
    try {
      setResult(await api.checkForUpdates())
      setError(null)
    } catch {
      // A failure forgets the download page on the Go side too.
      setResult(null)
      setError(UPDATE_FAILED_COPY)
    } finally {
      setChecking(false)
    }
  }

  const openRelease = async () => {
    try {
      await api.openReleasePage()
    } catch (e) {
      toast.error(errorCopy(toApiError(e)).message)
    }
  }

  const copyLink = async (url: string) => {
    try {
      await api.copyText(url)
      toast.success(LINK_COPIED)
    } catch {
      toast.error(CLIPBOARD_COPY)
    }
  }

  const offersRelease = result?.state === 'update_available' || result?.state === 'unknown'
  const assetURL = result?.state === 'update_available' ? result.asset?.url : undefined
  return (
    <Dialog
      title="Look for updates"
      onClose={onClose}
      initialFocus={checkRef}
      className="update-dialog"
      footer={
        <>
          <Button key="close" variant={result && !offersRelease ? 'primary' : 'secondary'} onClick={onClose}>
            Close
          </Button>
          <Button
            key="check"
            ref={checkRef}
            variant={result ? 'secondary' : 'primary'}
            icon={result ? RefreshCw : undefined}
            busy={checking}
            busyLabel="Looking..."
            onClick={() => void check()}
          >
            {result ? 'Look again' : 'Look now'}
          </Button>
          {assetURL && (
            <Button key="copy" variant="secondary" icon={Copy} onClick={() => void copyLink(assetURL)}>
              Copy download link
            </Button>
          )}
          {offersRelease && (
            <Button key="open" variant="primary" icon={ExternalLink} onClick={() => void openRelease()}>
              Open the download page
            </Button>
          )}
        </>
      }
    >
      <div className="update-result" aria-live="polite">
        {(result ? updateLines(result) : [UPDATE_INTRO_COPY]).map((line) => (
          <p className="dialog-text" key={line}>
            {line}
          </p>
        ))}
      </div>
      {error && (
        <p className="update-error" role="alert">
          {error}
        </p>
      )}
    </Dialog>
  )
}
