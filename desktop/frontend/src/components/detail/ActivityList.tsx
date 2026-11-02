// History (UI-23, D-23; UI-SPEC Layout, History): newest first, each row a
// plain sentence with the actor kind and client, the engine summary on a
// second line as text (common shapes rewritten in plain words by
// activitySummary, the stored text kept in its title), and a relative time with the absolute
// time in its title, re-rendered once a minute (useNow). Twenty rows by default; the show-more button reloads
// the detail with up to 200.
import { ScrollText } from 'lucide-react'
import type { Activity } from '../../api/types'
import { activitySentence, activitySummary } from '../../lib/activity'
import { ImportedTag } from '../record/ImportedTag'
import { formatAbsolute, formatRelative } from '../../lib/relativeTime'
import { useNow } from '../../hooks/useNow'
import { useAppData } from '../../state/appData'
import { Button } from '../ui/Button'
import { SectionList } from './SectionList'
import './ActivityList.css'

export const MAX_ACTIVITY_LIMIT = 200

export interface ActivityListProps {
  /** detail.recent_activity, newest first. */
  activities: Activity[]
  /** Opens the drawer for an E, C or H ref; without it refs are plain text. */
  onOpenRef?: (ref: string) => void
  /** The case's conclusion_reviewed_at, so the review row reads as a review. */
  conclusionReviewedAt?: string
}

export function ActivityList({ activities, onOpenRef, conclusionReviewedAt }: ActivityListProps) {
  const { activityLimit, setActivityLimit } = useAppData()
  const now = useNow()
  const more = activityLimit < MAX_ACTIVITY_LIMIT && activities.length >= activityLimit
  return (
    <SectionList<Activity>
      id="history"
      icon={ScrollText}
      kind="history"
      heading="History"
      emptyText="No history yet."
      items={activities}
      itemKey={(a) => String(a.id)}
      renderItem={(a) => <ActivityRow activity={a} onOpenRef={onOpenRef} now={now} conclusionReviewedAt={conclusionReviewedAt} />}
      footer={
        more ? (
          <div className="activity-more">
            <Button variant="ghost" onClick={() => setActivityLimit(MAX_ACTIVITY_LIMIT)}>
              Show more history
            </Button>
          </div>
        ) : null
      }
    />
  )
}

function ActivityRow({
  activity: a,
  onOpenRef,
  now,
  conclusionReviewedAt,
}: {
  activity: Activity
  onOpenRef?: (ref: string) => void
  now: Date
  conclusionReviewedAt?: string
}) {
  const s = activitySentence(a)
  const summary = a.summary.trim()
  const plain = activitySummary(a, { conclusionReviewedAt })
  const showSummary = summary !== '' && summary !== a.target_ref
  return (
    <div className="activity-row">
      <div className="activity-main">
        <p className="activity-sentence">
          {s.imported && (
            <>
              <ImportedTag />{' '}
            </>
          )}
          <span className="activity-actor" title={s.tooltip}>
            {s.actor}
          </span>{' '}
          {s.segments.map((seg, i) =>
            'ref' in seg ? (
              onOpenRef ? (
                <button key={i} type="button" className="activity-ref tabular" onClick={() => onOpenRef(seg.ref)}>
                  {seg.ref}
                </button>
              ) : (
                <span key={i} className="tabular">
                  {seg.ref}
                </span>
              )
            ) : (
              <span key={i}>{seg.text}</span>
            ),
          )}
        </p>
        {showSummary && (plain === summary
          ? <p className="activity-summary">{a.summary}</p>
          : <p className="activity-summary" title={a.summary}>{plain}</p>)}
      </div>
      <time className="activity-time tabular" dateTime={a.created_at} title={formatAbsolute(a.created_at)}>
        {formatRelative(a.created_at, now)}
      </time>
    </div>
  )
}
