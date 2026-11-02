// Question row (UI-SPEC Layout, Question row). Questions are not drawers: the
// row is focusable by script only (tabIndex -1) so a newly added question or
// a search hit can take focus. Resolved rows mute the question and show the
// resolution below in ink; deferred rows show the defer note. Plan 03 fills
// the actions slot (Resolve, Defer, Reopen, Edit question).
import type { ReactNode } from 'react'
import type { Question } from '../../api/types'
import { ImportedTag } from '../record/ImportedTag'
import { kindRuleClass } from '../record/KindRule'
import { RecordStatusLabel } from '../record/StatusLabel'
import './RecordRow.css'
import './QuestionRow.css'

export interface QuestionRowProps {
  question: Question
  /** Controls on the right of the row. */
  actions?: ReactNode
}

export function QuestionRow({ question: q, actions }: QuestionRowProps) {
  const open = q.status === 'open'
  return (
    <div className={['detail-row', 'question-row', kindRuleClass('question', q)].join(' ')} data-ref={q.ref} tabIndex={-1}>
      <div className="question-row-main">
        <p className="row-line">
          <span className="row-ref tabular">{q.ref}</span>
          <RecordStatusLabel kind="question" status={q.status} />
          {q.imported && <ImportedTag />}
        </p>
        <p className={['question-row-text', open ? '' : 'is-closed'].filter(Boolean).join(' ')}>{q.question}</p>
        {q.status === 'resolved' && q.resolution && <p className="question-row-answer">{q.resolution}</p>}
        {q.status === 'deferred' && q.defer_note && <p className="question-row-note">{q.defer_note}</p>}
      </div>
      {actions && <div className="question-row-actions">{actions}</div>}
    </div>
  )
}
