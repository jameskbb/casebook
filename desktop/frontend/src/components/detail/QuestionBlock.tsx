// The question in Literata 28: the title, which is the question itself. The
// background the person wrote sits under it, smaller, and is left out when
// there is none.
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
  const background = investigation.problem_statement.trim()
  return (
    <section className="question-block" aria-labelledby="question-block-label">
      <div className="question-block-head">
        <p className="question-block-label" id="question-block-label">
          The question
        </p>
        {onEdit && <IconButton icon={Pencil} label="Edit question" onClick={onEdit} />}
      </div>
      <p className="question-block-text">{investigation.title}</p>
      {background && (
        <div className="question-block-background">
          <p className="question-block-label">Background</p>
          <p className="question-block-background-text">{background}</p>
        </div>
      )}
    </section>
  )
}
