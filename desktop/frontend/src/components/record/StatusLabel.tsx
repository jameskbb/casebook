// Status labels. Investigations: New muted, Investigating accent, Blocked
// brick, Resolved green, Archived as a pill tag (UI-SPEC Color). Records:
// claim, hypothesis and question statuses per the Record kind visual
// language table. The text is always present; color is never the only
// signal.
import { CircleCheck, CirclePause, Lightbulb } from 'lucide-react'
import type { ClaimStatus, HypothesisStatus, InvestigationStatus, QuestionStatus } from '../../api/types'
import { investigationStatusLabels } from '../../lib/labels'
import { DisputedMark } from './VerificationBadge'
import './StatusLabel.css'

export interface StatusLabelProps {
  status: InvestigationStatus
  /** Shows the Archived tag instead of the status. */
  archived?: boolean
  className?: string
}

export function StatusLabel({ status, archived = false, className }: StatusLabelProps) {
  if (archived || status === 'archived') return <ArchivedTag className={className} />
  return (
    <span className={['status-label', `status-${status}`, className ?? ''].filter(Boolean).join(' ')}>
      {investigationStatusLabels[status] ?? status}
    </span>
  )
}

export function ArchivedTag({ className }: { className?: string }) {
  return <span className={['archived-tag', className ?? ''].filter(Boolean).join(' ')}>Archived</span>
}

export type RecordStatusLabelProps =
  | { kind: 'claim'; status: ClaimStatus; supersededBy?: string }
  | { kind: 'hypothesis'; status: HypothesisStatus; supersededBy?: string }
  | { kind: 'question'; status: QuestionStatus }

/**
 * The status of a claim, hypothesis or question. Default states render
 * nothing (an active claim, an open question); a hypothesis always says Open
 * or Supported because tentative versus established is the point.
 */
export function RecordStatusLabel(props: RecordStatusLabelProps) {
  const icon = { size: 16, strokeWidth: 1.75, 'aria-hidden': true as const, className: 'record-status-icon' }
  switch (props.status) {
    case 'active':
      return null
    case 'open':
      if (props.kind === 'question') return null
      return (
        <span className="record-status record-status-open">
          <Lightbulb {...icon} />
          Open
        </span>
      )
    case 'disputed':
      return <DisputedMark />
    case 'supported':
      return (
        <span className="record-status record-status-supported">
          <CircleCheck {...icon} />
          Supported
        </span>
      )
    case 'superseded': {
      const by = props.supersededBy
      return <span className="record-status">{by ? `Superseded by ${by}` : 'Superseded'}</span>
    }
    case 'rejected':
      return <span className="record-status">Rejected</span>
    case 'disproven':
      return <span className="record-status">Disproven</span>
    case 'resolved':
      return (
        <span className="record-status record-status-resolved">
          <CircleCheck {...icon} />
          Resolved
        </span>
      )
    case 'deferred':
      return (
        <span className="record-status record-status-deferred">
          <CirclePause {...icon} />
          Deferred
        </span>
      )
    default:
      return null
  }
}
