// Status labels. A question reads Answered (green) or No answer yet (muted),
// worked out from whether it has an answer; an archived one shows the
// Archived pill tag. Records: finding and follow-up question statuses per
// the Record kind visual language table. The text is always present; color
// is never the only signal.
import { CircleCheck, CirclePause, Lightbulb } from 'lucide-react'
import type { ClaimStatus, HypothesisStatus, QuestionStatus } from '../../api/types'
import { claimStatusLabels, hypothesisStatusLabels } from '../../lib/labels'
import { RecordId } from './RecordId'
import { DisputedMark } from './VerificationBadge'
import './StatusLabel.css'

export const ANSWERED = 'Answered'
export const NO_ANSWER_YET = 'No answer yet'

/** Answered or No answer yet. Nobody sets it: it follows the answer text. */
export function AnswerState({ answer, archived = false }: { answer: string; archived?: boolean }) {
  if (archived) return <ArchivedTag />
  const answered = answer.trim() !== ''
  return <span className={`status-label ${answered ? 'status-resolved' : 'status-new'}`}>{answered ? ANSWERED : NO_ANSWER_YET}</span>
}

export function ArchivedTag({ className }: { className?: string }) {
  return <span className={['archived-tag', className ?? ''].filter(Boolean).join(' ')}>Archived</span>
}

export type RecordStatusLabelProps =
  | { kind: 'claim'; status: ClaimStatus; supersededBy?: string }
  | { kind: 'hypothesis'; status: HypothesisStatus; supersededBy?: string }
  | { kind: 'question'; status: QuestionStatus }

/**
 * The status of a finding or follow-up question. Default states render
 * nothing (a current claim, an open question); a hypothesis always says Not
 * tested yet or Supported because tentative versus established is the point.
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
          {hypothesisStatusLabels.open}
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
      return (
        <span className="record-status">
          {claimStatusLabels.superseded}
          {by && (
            <>
              {' '}
              <RecordId>by {by}</RecordId>
            </>
          )}
        </span>
      )
    }
    case 'rejected':
    case 'disproven':
      return <span className="record-status">{claimStatusLabels.rejected}</span>
    case 'resolved':
      return (
        <span className="record-status record-status-resolved">
          <CircleCheck {...icon} />
          Answered
        </span>
      )
    case 'deferred':
      return (
        <span className="record-status record-status-deferred">
          <CirclePause {...icon} />
          Set aside
        </span>
      )
    default:
      return null
  }
}
