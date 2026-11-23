// A source's one label: who has checked it. An icon plus the words, so color
// is never the only signal.
import { BadgeCheck, CircleAlert, CircleDashed } from 'lucide-react'
import type { Verification } from '../../api/types'
import { REPLACED, SOURCES_DISAGREE, sourceLabel } from '../../lib/labels'
import './VerificationBadge.css'

export function VerificationBadge({ verification }: { verification: Verification }) {
  const label = sourceLabel(verification)
  if (label === REPLACED) return <span className="badge badge-muted">{label}</span>
  const checked = verification === 'human_verified' || verification === 'source_verified'
  const Icon = checked ? BadgeCheck : CircleDashed
  return (
    <span className={`badge ${checked ? 'badge-verified' : 'badge-ai'}`}>
      <Icon className="badge-icon" size={16} strokeWidth={1.75} aria-hidden="true" />
      {label}
    </span>
  )
}

/** CircleAlert plus "Sources disagree" in its own hue, 14/600. */
export function DisputedMark() {
  return (
    <span className="badge badge-disputed">
      <CircleAlert className="badge-icon" size={16} strokeWidth={1.75} aria-hidden="true" />
      {SOURCES_DISAGREE}
    </span>
  )
}
