// Something to find out, under Still to find out. It opens no drawer: the row
// takes focus by script only (tabIndex -1) so a newly added one or a search
// hit can be focused. An answered one mutes the question and shows the
// answer below; one left for later says Later with its note.
import type { ReactNode } from 'react'
import type { Question } from '../../api/types'
import { ImportedTag } from '../record/ImportedTag'
import { CodedText } from '../record/RecordId'
import { kindRuleClass } from '../record/KindRule'
import { LaterLabel } from '../record/StatusLabel'
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
          {q.status === 'deferred' && <LaterLabel />}
          {q.imported && <ImportedTag />}
        </p>
        <p className={['question-row-text', open ? '' : 'is-closed'].filter(Boolean).join(' ')}><CodedText text={q.question} /></p>
        {q.status === 'resolved' && q.resolution && <p className="question-row-answer"><CodedText text={q.resolution} /></p>}
        {q.status === 'deferred' && q.defer_note && <p className="question-row-note"><CodedText text={q.defer_note} /></p>}
      </div>
      {actions && <div className="question-row-actions">{actions}</div>}
    </div>
  )
}
