// How we got here (the history): newest first, each row a plain sentence
// naming who did it and the source or finding by its words (its code shows
// only when codes are on), a second line in plain words when it adds
// something (lib/activity activitySummary; the stored engine text is never
// shown), and a relative time with the absolute time in its title,
// re-rendered once a minute (useNow). The last three rows show; Show all
// opens the rest and reloads the detail with up to 200.
import { useState } from 'react'
import { ScrollText } from 'lucide-react'
import type { Activity } from '../../api/types'
import { activitySentence, activitySummary } from '../../lib/activity'
import { ImportedTag } from '../record/ImportedTag'
import { CodedText, RecordId } from '../record/RecordId'
import { formatAbsolute, formatRelative } from '../../lib/relativeTime'
import { useNow } from '../../hooks/useNow'
import { useAppData } from '../../state/appData'
import { Button } from '../ui/Button'
import { SectionList } from './SectionList'
import './ActivityList.css'

export const MAX_ACTIVITY_LIMIT = 200
/** How many rows show before Show all. */
export const HISTORY_SHOWN = 3

export interface ActivityListProps {
  /** detail.recent_activity, newest first. */
  activities: Activity[]
  /** Opens the drawer for a source or finding; without it they are plain text. */
  onOpenRef?: (ref: string) => void
  /** The title or statement of a record, shown in place of its code. */
  nameOf?: (ref: string) => string | undefined
  /** The question's conclusion_reviewed_at, so the row where the person checked the answer says so. */
  conclusionReviewedAt?: string
}

/** The first words of a title or statement that fit in max characters, ending in three periods when cut. */
export function shortName(text: string, max = 60): string {
  const t = text.trim().replace(/\s+/g, ' ')
  if (t.length <= max) return t
  const cut = t.slice(0, max - 3)
  const space = cut.lastIndexOf(' ')
  return (space > 0 ? cut.slice(0, space) : cut) + '...'
}

export function ActivityList({ activities, onOpenRef, nameOf, conclusionReviewedAt }: ActivityListProps) {
  const { activityLimit, setActivityLimit } = useAppData()
  const now = useNow()
  const [all, setAll] = useState(false)
  // The page may hold fewer rows than there are; then the count is not known.
  const partial = activityLimit < MAX_ACTIVITY_LIMIT && activities.length >= activityLimit
  const toggle = () => {
    if (!all && partial) setActivityLimit(MAX_ACTIVITY_LIMIT)
    setAll(!all)
  }
  return (
    <SectionList<Activity>
      id="history"
      icon={ScrollText}
      kind="history"
      heading="How we got here"
      items={all ? activities : activities.slice(0, HISTORY_SHOWN)}
      itemKey={(a) => String(a.id)}
      renderItem={(a) => <ActivityRow activity={a} onOpenRef={onOpenRef} nameOf={nameOf} now={now} conclusionReviewedAt={conclusionReviewedAt} />}
      footer={
        activities.length > HISTORY_SHOWN ? (
          <div className="activity-more">
            <Button variant="ghost" aria-expanded={all} onClick={toggle}>
              {all ? `Show only the last ${HISTORY_SHOWN}` : partial ? 'Show all' : `Show all ${activities.length}`}
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
  nameOf,
  now,
  conclusionReviewedAt,
}: {
  activity: Activity
  onOpenRef?: (ref: string) => void
  nameOf?: (ref: string) => string | undefined
  now: Date
  conclusionReviewedAt?: string
}) {
  const s = activitySentence(a)
  const plain = activitySummary(a, { conclusionReviewedAt })
  const name = a.target_ref ? nameOf?.(a.target_ref) : undefined
  // Something to find out left for later is named by its words.
  if (a.action === 'question.defer' && name) s.segments = [{ text: `left "${shortName(name)}" for later` }]
  const named = Boolean(name) && s.segments.some((seg) => 'ref' in seg || a.action === 'question.defer')
  // The second line shows only when it adds to the first.
  const showSummary = plain !== '' && !(named && plain === name)
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
          {s.segments.map((seg, i) => {
            if ('text' in seg) return <span key={i}>{seg.text}</span>
            if ('id' in seg) return <span key={i}> <RecordId>{seg.id}</RecordId></span>
            // A record that is gone has no words to show, so its code stays visible.
            const label = name ? (
              <>
                <RecordId>{seg.ref}</RecordId> {shortName(name)}
              </>
            ) : (
              <span className="tabular">{seg.ref}</span>
            )
            return onOpenRef ? (
              <button key={i} type="button" className="activity-ref" title={name} onClick={() => onOpenRef(seg.ref)}>
                {label}
              </button>
            ) : (
              <span key={i}>{label}</span>
            )
          })}
        </p>
        {showSummary && (
          <p className="activity-summary">
            <CodedText text={plain} nameOf={(ref) => { const words = nameOf?.(ref); return words && shortName(words, 40) }} />
          </p>
        )}
      </div>
      <time className="activity-time tabular" dateTime={a.created_at} title={formatAbsolute(a.created_at)}>
        {formatRelative(a.created_at, now)}
      </time>
    </div>
  )
}
