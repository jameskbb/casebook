// The labels. A question reads Answered (green) or No answer yet (muted),
// worked out from whether it has an answer; an archived one shows the
// Archived pill tag. A finding shows one label, worked out from its sources
// (lib/records findingMark). Something to find out shows a word only when it
// was left for later. The text is always present; color is never the only
// signal.
import { BadgeCheck, CirclePause } from 'lucide-react'
import { ANSWERED, ARCHIVED, LATER, NO_ANSWER_YET } from '../../lib/labels'
import { findingMark, type Finding, type Sources } from '../../lib/records'
import { RecordId } from './RecordId'
import { DisputedMark } from './VerificationBadge'
import './VerificationBadge.css'
import './StatusLabel.css'

export { ANSWERED, LATER, NO_ANSWER_YET }

/** Answered or No answer yet. Nobody sets it: it follows the answer text. */
export function AnswerState({ answer, archived = false }: { answer: string; archived?: boolean }) {
  if (archived) return <ArchivedTag />
  const answered = answer.trim() !== ''
  return <span className={`status-label ${answered ? 'status-resolved' : 'status-new'}`}>{answered ? ANSWERED : NO_ANSWER_YET}</span>
}

export function ArchivedTag({ className }: { className?: string }) {
  return <span className={['archived-tag', className ?? ''].filter(Boolean).join(' ')}>{ARCHIVED}</span>
}

const NO_SOURCES: Sources = new Map()

/** A finding's one label. A replaced finding names what replaced it when codes are on. */
export function FindingLabel({ finding, sources = NO_SOURCES }: { finding: Finding; sources?: Sources }) {
  const { label, tone } = findingMark(finding, sources)
  const icon = { size: 16, strokeWidth: 1.75, 'aria-hidden': true as const }
  if (tone === 'disagree') return <DisputedMark />
  if (tone === 'checked')
    return (
      <span className="badge badge-verified">
        <BadgeCheck className="badge-icon" {...icon} />
        {label}
      </span>
    )
  const by = finding.record.status === 'superseded' ? finding.record.superseded_by : ''
  return (
    <span className="record-status">
      {label}
      {by && (
        <>
          {' '}
          <RecordId>by {by}</RecordId>
        </>
      )}
    </span>
  )
}

/** Later, on something to find out that was left for later. */
export function LaterLabel() {
  return (
    <span className="record-status record-status-deferred">
      <CirclePause className="record-status-icon" size={16} strokeWidth={1.75} aria-hidden="true" />
      {LATER}
    </span>
  )
}
