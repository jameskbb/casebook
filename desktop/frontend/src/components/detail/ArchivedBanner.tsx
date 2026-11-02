// Archived banner (UI-SPEC Layout, Investigation detail): shown at the top of
// the reading column for an archived investigation, with Unarchive.
import { useState } from 'react'
import { Archive } from 'lucide-react'
import { api } from '../../api/api'
import { useWrite } from '../../state/useWrite'
import { Banner } from '../ui/Banner'
import { Button } from '../ui/Button'
import { useToast } from '../ui/Toaster'
import './ArchivedBanner.css'

export function ArchivedBanner({ investigationRef }: { investigationRef: string }) {
  const write = useWrite()
  const toast = useToast()
  const [busy, setBusy] = useState(false)
  const unarchive = async () => {
    setBusy(true)
    try {
      const inv = await write(() => api.unarchiveInvestigation(investigationRef))
      if (inv) toast.success(`Unarchived ${investigationRef}.`)
    } finally {
      setBusy(false)
    }
  }
  return (
    <Banner
      className="archived-banner"
      icon={Archive}
      action={
        <Button variant="secondary" busy={busy} busyLabel="Unarchiving..." onClick={() => void unarchive()}>
          Unarchive
        </Button>
      }
    >
      This case is archived. It is hidden from the list and nothing was deleted.
    </Banner>
  )
}
