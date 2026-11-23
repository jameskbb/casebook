// One source inside a finding drawer: name 16/600, what it shows, then
// where to find it when that is known. The whole block opens the source drawer through a stretched
// button under the content, so where to find it stays selectable plain text
// and the block's own actions stay separate buttons.
import type { ReactNode } from 'react'
import type { Evidence } from '../../api/types'
import { kindRuleClass } from '../record/KindRule'
import { VerificationBadge } from '../record/VerificationBadge'
import { BreakablePath } from '../ui/BreakablePath'
import '../detail/RecordRow.css'
import './EvidenceBlock.css'

export interface EvidenceBlockProps {
  evidence: Evidence
  onOpen: (ref: string) => void
  /** Ghost actions under the block. */
  actions?: ReactNode
}

export function EvidenceBlock({ evidence: e, onOpen, actions }: EvidenceBlockProps) {
  return (
    <div className={`evidence-block ${kindRuleClass('evidence', e)}`} data-ref={e.ref}>
      <button
        type="button"
        className="row-hit evidence-block-hit"
        aria-label={`Open source ${e.ref}: ${e.title}`}
        onClick={() => onOpen(e.ref)}
      />
      <div className="evidence-block-content">
        <p className="evidence-block-line">
          <span className="evidence-block-ref tabular">{e.ref}</span>
          <VerificationBadge verification={e.verification} />
        </p>
        <p className="evidence-block-title">{e.title}</p>
        {e.summary && <p className="evidence-block-summary">{e.summary}</p>}
        {e.source_ref && (
          <p className="evidence-block-reference">
            <BreakablePath text={e.source_ref} />
          </p>
        )}
      </div>
      {actions && <div className="evidence-block-actions">{actions}</div>}
    </div>
  )
}
