// Share summary: the one primary action on the detail screen. The case is
// for showing someone; this is where that happens.
import { useState } from 'react'
import { ScrollText } from 'lucide-react'
import { Button } from '../ui/Button'
import { ShareSummaryDialog } from './ShareSummaryDialog'

export function ShareSummaryButton({ investigationRef, title }: { investigationRef: string; title: string }) {
  const [open, setOpen] = useState(false)
  return (
    <>
      <Button variant="primary" icon={ScrollText} className="share-summary-btn" onClick={() => setOpen(true)}>
        Share summary
      </Button>
      {open && <ShareSummaryDialog invRef={investigationRef} title={title} onClose={() => setOpen(false)} />}
    </>
  )
}
