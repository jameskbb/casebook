// The question (UI-SPEC Layout, reading column item 1): the problem statement
// in Literata 28, or "No question written yet." when it is empty.
import { Pencil } from 'lucide-react'
import type { ReactNode } from 'react'
import type { Investigation } from '../../api/types'
import { Button } from '../ui/Button'
import { IconButton } from '../ui/IconButton'
import './QuestionBlock.css'

export interface QuestionBlockProps {
  investigation: Investigation
  /** Opens the Edit title and question dialog; the edit controls show only when set. */
  onEdit?: () => void
  actions?: ReactNode
}

export function QuestionBlock({ investigation, onEdit, actions }: QuestionBlockProps) {
  const text = investigation.problem_statement.trim()
  return (
    <section className="question-block" aria-labelledby="question-block-label">
      <div className="question-block-head">
        <p className="question-block-label" id="question-block-label">
          The question
        </p>
        <div className="question-block-actions">{actions}{onEdit && text && <IconButton icon={Pencil} label="Edit title and question" onClick={onEdit} />}</div>
      </div>
      {text ? (
        <p className="question-block-text">{investigation.problem_statement}</p>
      ) : (
        <div className="question-block-empty">
          <p>No question written yet.</p>
          {onEdit && (
            <Button variant="ghost" icon={Pencil} onClick={onEdit}>
              Write the question
            </Button>
          )}
        </div>
      )}
    </section>
  )
}
