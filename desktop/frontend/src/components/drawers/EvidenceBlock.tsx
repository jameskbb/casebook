// One linked piece of evidence inside a claim or hypothesis drawer (UI-SPEC
// Drawers, group 4): title 16/600, the summary, then the source type, source
// label, reference and captured date, omitting whatever is missing. The
// whole block opens the evidence drawer through a stretched button under the
// content (the Plan 02 row pattern), so the reference stays selectable plain
// text and the block's own actions stay separate buttons.
import type { ReactNode } from 'react'
import type { Evidence } from '../../api/types'
import { sourceTypeLabels } from '../../lib/labels'
import { formatAbsolute } from '../../lib/relativeTime'
import { kindRuleClass } from '../record/KindRule'
import { VerificationBadge } from '../record/VerificationBadge'
import { BreakablePath } from '../ui/BreakablePath'
import '../detail/RecordRow.css'
import './EvidenceBlock.css'

export interface EvidenceBlockProps {
  evidence: Evidence
  onOpen: (ref: string) => void
  /** Ghost actions under the block (Mark as contradicting, Unlink). */
  actions?: ReactNode
}

export function EvidenceBlock({ evidence: e, onOpen, actions }: EvidenceBlockProps) {
  const captured = formatAbsolute(e.captured_at)
  return (
    <div className={`evidence-block ${kindRuleClass('evidence', e)}`} data-ref={e.ref}>
      <button
        type="button"
        className="row-hit evidence-block-hit"
        aria-label={`Open evidence ${e.ref}: ${e.title}`}
        onClick={() => onOpen(e.ref)}
      />
      <div className="evidence-block-content">
        <p className="evidence-block-line">
          <span className="evidence-block-ref tabular">{e.ref}</span>
          <VerificationBadge verification={e.verification} />
        </p>
        <p className="evidence-block-title">{e.title}</p>
        {e.summary && <p className="evidence-block-summary">{e.summary}</p>}
        <p className="evidence-block-source">
          <span>{sourceTypeLabels[e.source_type] ?? e.source_type}</span>
          {e.source_label && <span>{e.source_label}</span>}
        </p>
        {e.source_ref && (
          <p className="evidence-block-reference">
            <BreakablePath text={e.source_ref} />
          </p>
        )}
        {captured && (
          <p className="evidence-block-captured tabular">
            <span className="evidence-block-captured-label">Captured</span> {captured}
          </p>
        )}
      </div>
      {actions && <div className="evidence-block-actions">{actions}</div>}
    </div>
  )
}
