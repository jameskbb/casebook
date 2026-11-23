// The question in Literata 28: the background text when there is some,
// otherwise the title, which is the question itself.
import { Pencil } from 'lucide-react'
import type { Investigation } from '../../api/types'

import { IconButton } from '../ui/IconButton'
import './QuestionBlock.css'

export interface QuestionBlockProps {
  investigation: Investigation
  /** Opens the Edit question dialog; the edit control shows only when set. */
  onEdit?: () => void
}

export function QuestionBlock({ investigation, onEdit }: QuestionBlockProps) {
  const text = investigation.problem_statement.trim() ? investigation.problem_statement : investigation.title
  return (
    <section className="question-block" aria-labelledby="question-block-label">
      <div className="question-block-head">
        <p className="question-block-label" id="question-block-label">
          The question
        </p>
        {onEdit && <IconButton icon={Pencil} label="Edit question" onClick={onEdit} />}
      </div>
      <p className="question-block-text">{text}</p>
    </section>
  )
}
