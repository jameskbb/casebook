// Check for updates (ADR 0022). Opened from the Data menu; nothing is asked
// until the person chooses Check now. The Go side makes the one request to
// GitHub (internal/update) and never downloads anything: an available update
// is a link to its release page, and the person replaces the program files
// themselves. The result is announced in a polite live region; a failed
// check shows Go's message in the dialog, with Check now to try again.
import { useRef, useState } from 'react'
import { Copy, ExternalLink, RefreshCw } from 'lucide-react'
import { api } from '../../api/api'
import { asSentence, CLIPBOARD_COPY, errorCopy, toApiError } from '../../api/errors'
import type { UpdateCheck } from '../../api/types'
import { Button } from '../ui/Button'
import { Dialog } from '../ui/Dialog'
import { useToast } from '../ui/Toaster'
import './CheckUpdatesDialog.css'

export const UPDATE_INTRO_COPY =
  'Casebook will ask GitHub for its latest release. The request carries no version or identifier and nothing is downloaded; you choose when to update.'
export const UPDATE_HOW_COPY =
  'Download it from the release page, then replace the program files. Your data stays where it is.'
export const LINK_COPIED = 'Download link copied to the clipboard.'

/** The result lines for one check, by state. */
export function updateLines(r: UpdateCheck): string[] {
  switch (r.state) {
    case 'update_available':
      return [`Casebook ${r.latest} is available. You have ${r.current}.`, UPDATE_HOW_COPY]
    case 'up_to_date':
      return [`You have the latest version, Casebook ${r.latest}.`]
    case 'ahead':
      return [`You have Casebook ${r.current}, which is newer than the latest release, ${r.latest}.`]
    case 'unknown':
    default:
      return [`This build (${r.current}) is not a release version. The latest release is Casebook ${r.latest}.`]
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
    } catch (e) {
      // A failed check unpins the release page on the Go side too.
      setResult(null)
      setError(asSentence(errorCopy(toApiError(e)).message))
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
      title="Check for updates"
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
            busyLabel="Checking..."
            onClick={() => void check()}
          >
            {result ? 'Check again' : 'Check now'}
          </Button>
          {assetURL && (
            <Button key="copy" variant="secondary" icon={Copy} onClick={() => void copyLink(assetURL)}>
              Copy download link
            </Button>
          )}
          {offersRelease && (
            <Button key="open" variant="primary" icon={ExternalLink} onClick={() => void openRelease()}>
              Open release page
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
