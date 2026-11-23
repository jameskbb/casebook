// Share the answer, in the answer panel. It is the primary action once
// nothing is left to check; while sources wait, Check the next source is.
import { useState } from 'react'
import { ScrollText } from 'lucide-react'
import { Button } from '../ui/Button'
import { ShareSummaryDialog } from './ShareSummaryDialog'

export function ShareSummaryButton({ investigationRef, title, variant = 'primary' }: { investigationRef: string; title: string; variant?: 'primary' | 'secondary' }) {
  const [open, setOpen] = useState(false)
  return (
    <>
      <Button variant={variant} icon={ScrollText} className="share-summary-btn" onClick={() => setOpen(true)}>
        Share the answer
      </Button>
      {open && <ShareSummaryDialog invRef={investigationRef} title={title} onClose={() => setOpen(false)} />}
    </>
  )
}
