// Copy for your AI (UI-19, D-20): secondary; Share summary is the primary
// action on the detail screen. Copies the case the way AI tools read it.
// Shows "Copied" for 2000 ms only after the Go clipboard call succeeded; it
// stays enabled while a copy is in flight.
import { Check, ClipboardCopy } from 'lucide-react'
import { useCopyContext } from '../../state/copyContext'
import { Button } from '../ui/Button'

export function CopyContextButton({ investigationRef }: { investigationRef: string }) {
  const { status, copy } = useCopyContext()
  const copied = status === 'copied'
  return (
    <Button
      variant="secondary"
      icon={copied ? Check : ClipboardCopy}
      className="copy-context-btn"
      title="Copies the case the way your AI tools read it, to paste into a chat."
      onClick={() => void copy(investigationRef)}
    >
      {copied ? 'Copied' : 'Copy for your AI'}
    </Button>
  )
}
