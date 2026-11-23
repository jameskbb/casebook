// Archived banner: shown at the top of the reading column for an archived
// question, with Put back in the list.
import { useState } from 'react'
import { Archive } from 'lucide-react'
import { api } from '../../api/api'
import { useWrite } from '../../state/useWrite'
import { Banner } from '../ui/Banner'
import { Button } from '../ui/Button'
import { useToast } from '../ui/Toaster'
import { BACK_IN_LIST_COPY, PUT_BACK_LABEL } from './MoreMenu'
import './ArchivedBanner.css'

export function ArchivedBanner({ investigationRef }: { investigationRef: string }) {
  const write = useWrite()
  const toast = useToast()
  const [busy, setBusy] = useState(false)
  const putBack = async () => {
    setBusy(true)
    try {
      const inv = await write(() => api.unarchiveInvestigation(investigationRef))
      if (inv) toast.success(BACK_IN_LIST_COPY)
    } finally {
      setBusy(false)
    }
  }
  return (
    <Banner
      className="archived-banner"
      icon={Archive}
      action={
        <Button variant="secondary" busy={busy} busyLabel="Putting it back..." onClick={() => void putBack()}>
          {PUT_BACK_LABEL}
        </Button>
      }
    >
      This question is archived. It is hidden from the list and nothing was deleted.
    </Banner>
  )
}
