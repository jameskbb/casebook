// The sticky 56px header: the title and Answered or No answer yet on the
// left (the question's code too, when codes are on); on the right, 8px
// apart, the AI toggle, Copy for your AI and the More menu. Share the answer
// lives in the answer panel. The title is
// the page's h1 and takes focus after a question is created. The
// title truncates first; the right cluster never wraps or shrinks, so the
// header holds at the 640px minimum main pane.
import { useEffect, useRef, type ReactNode } from 'react'
import type { Investigation } from '../../api/types'
import { ActiveToggle } from './ActiveToggle'
import { CopyContextButton } from './CopyContextButton'
import { MoreMenu } from './MoreMenu'
import { AnswerState } from '../record/StatusLabel'
import './DetailHeader.css'

export interface DetailHeaderProps {
  investigation: Investigation
  /** Move focus to the title once (after creating the investigation). */
  focusTitle?: boolean
  onTitleFocused?: () => void
  /** Controls placed after the answer state. */
  leftExtra?: ReactNode
  /** Controls placed between the AI toggle and Copy for your AI. */
  rightExtra?: ReactNode
  /** Controls placed after the More menu. */
  trailing?: ReactNode
}

export function DetailHeader({
  investigation: inv,
  focusTitle = false,
  onTitleFocused,
  leftExtra,
  rightExtra,
  trailing,
}: DetailHeaderProps) {
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
        {leftExtra}
      </div>
      <div className="detail-header-right">
        <ActiveToggle investigation={inv} />
        {rightExtra}
        <CopyContextButton investigationRef={inv.ref} />
        <MoreMenu investigation={inv} />
        {trailing}
      </div>
    </header>
  )
}
