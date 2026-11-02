// Verification badge (UI-SPEC Record kind visual language): an icon plus the
// enum label, so color is never the only signal. The title explains the label
// in one plain sentence.
import { BadgeCheck, Bot, CircleAlert } from 'lucide-react'
import type { Verification } from '../../api/types'
import { verificationHelp, verificationLabels } from '../../lib/labels'
import './VerificationBadge.css'

export function VerificationBadge({ verification }: { verification: Verification }) {
  const label = verificationLabels[verification] ?? verification
  const help = verificationHelp[verification]
  switch (verification) {
    case 'human_verified':
    case 'source_verified':
      return (
        <span className="badge badge-verified" title={help}>
          <BadgeCheck className="badge-icon" size={16} strokeWidth={1.75} aria-hidden="true" />
          {label}
        </span>
      )
    case 'ai_generated':
      return (
        <span className="badge badge-ai" title={help}>
          <Bot className="badge-icon" size={16} strokeWidth={1.75} aria-hidden="true" />
          {label}
        </span>
      )
    case 'disputed':
      return <DisputedMark />
    default:
      return (
        <span className="badge badge-muted" title={help}>
          {label}
        </span>
      )
  }
}

/** CircleAlert plus "Disputed" in the disputed hue, 14/600. */
export function DisputedMark({ title = verificationHelp.disputed }: { title?: string }) {
  return (
    <span className="badge badge-disputed" title={title}>
      <CircleAlert className="badge-icon" size={16} strokeWidth={1.75} aria-hidden="true" />
      Disputed
    </span>
  )
}
