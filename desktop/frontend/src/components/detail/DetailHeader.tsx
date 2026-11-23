// The sticky 56px header: the title and Answered or No answer yet on the
// left (the question's code too, when codes are on); on the right the plain
// "Your AI is working on this" marker while it is on (never on an archived
// question) and the More menu, which turns it on and off. Share the answer
// lives in the answer panel. The title is the page's h1 and takes focus after a question is
// created. The title truncates first; the right cluster never wraps or
// shrinks, so the header holds at the 640px minimum main pane.
import { useEffect, useRef } from 'react'
import type { Investigation } from '../../api/types'
import { ActiveMarker } from '../sidebar/InvestigationRow'
import { MoreMenu } from './MoreMenu'
import { AnswerState } from '../record/StatusLabel'
import './DetailHeader.css'

export interface DetailHeaderProps {
  investigation: Investigation
  /** Move focus to the title once (after creating the question). */
  focusTitle?: boolean
  onTitleFocused?: () => void
}

export function DetailHeader({ investigation: inv, focusTitle = false, onTitleFocused }: DetailHeaderProps) {
  const titleRef = useRef<HTMLHeadingElement>(null)
  useEffect(() => {
    if (focusTitle && titleRef.current) {
      titleRef.current.focus()
      onTitleFocused?.()
    }
  }, [focusTitle, inv.ref, onTitleFocused])

  return (
    <header className="detail-header">
      <div className="detail-header-left">
        <span className="detail-header-ref tabular">{inv.ref}</span>
        <h1 className="detail-header-title" ref={titleRef} tabIndex={-1} title={inv.title}>
          {inv.title}
        </h1>
        <AnswerState answer={inv.conclusion} archived={inv.archived} />
      </div>
      <div className="detail-header-right">
        {inv.is_active && !inv.archived && <ActiveMarker />}
        <MoreMenu investigation={inv} />
      </div>
    </header>
  )
}
