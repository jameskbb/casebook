// State labels. A question reads Answered (green) or No answer yet (muted),
// worked out from whether it has an answer; an archived one shows the
// Archived pill tag. A finding shows a word only when it is not the usual
// state, and a follow-up question only when it was left for later. The text
// is always present; color is never the only signal.
import { CircleCheck, CirclePause, Lightbulb } from 'lucide-react'
import type { ClaimStatus, HypothesisStatus, QuestionStatus } from '../../api/types'
import { claimStatusLabels, hypothesisStatusLabels } from '../../lib/labels'
import { RecordId } from './RecordId'
import { DisputedMark } from './VerificationBadge'
import './StatusLabel.css'

export const ANSWERED = 'Answered'
export const NO_ANSWER_YET = 'No answer yet'
export const LATER = 'Later'

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
 * The state of a finding or follow-up question. The usual states render
 * nothing (a finding that stands, an open or answered follow-up question);
 * a finding being tested always says Not tested yet or Looks right so far.
 */
export function RecordStatusLabel(props: RecordStatusLabelProps) {
  const icon = { size: 16, strokeWidth: 1.75, 'aria-hidden': true as const, className: 'record-status-icon' }
  switch (props.status) {
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
          {hypothesisStatusLabels.supported}
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
    case 'deferred':
      return (
        <span className="record-status record-status-deferred">
          <CirclePause {...icon} />
          {LATER}
        </span>
      )
    default:
      return null
  }
}
