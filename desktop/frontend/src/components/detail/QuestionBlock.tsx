// The question in Literata 28: the background text when there is some,
// otherwise the title, which is the question itself.
import { Pencil } from 'lucide-react'
import type { ReactNode } from 'react'
import type { Investigation } from '../../api/types'

import { IconButton } from '../ui/IconButton'
import './QuestionBlock.css'

export interface QuestionBlockProps {
  investigation: Investigation
  /** Opens the Edit question dialog; the edit control shows only when set. */
  onEdit?: () => void
  actions?: ReactNode
}

export function QuestionBlock({ investigation, onEdit, actions }: QuestionBlockProps) {
  const text = investigation.problem_statement.trim() ? investigation.problem_statement : investigation.title
  return (
    <section className="question-block" aria-labelledby="question-block-label">
      <div className="question-block-head">
        <p className="question-block-label" id="question-block-label">
          The question
        </p>
        <div className="question-block-actions">{actions}{onEdit && <IconButton icon={Pencil} label="Edit question" onClick={onEdit} />}</div>
      </div>
      <p className="question-block-text">{text}</p>
    </section>
  )
}
