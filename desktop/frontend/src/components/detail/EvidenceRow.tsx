// Source row, for a source no finding uses yet (the others sit under their
// finding). Line 1: its check label. Title 16/600, summary clamped to two
// lines (the full text is in the drawer), then the kind of source and where
// it came from (omitted when empty).
import type { Evidence } from '../../api/types'
import { sourceTypeLabels } from '../../lib/labels'
import { ImportedTag } from '../record/ImportedTag'
import { kindRuleClass } from '../record/KindRule'
import { VerificationBadge } from '../record/VerificationBadge'
import './RecordRow.css'
import './EvidenceRow.css'

export interface EvidenceRowProps {
  evidence: Evidence
  onOpen?: (ref: string) => void
}

export function EvidenceRow({ evidence: e, onOpen }: EvidenceRowProps) {
  const retired = e.verification === 'superseded'
  return (
    <div
      className={['detail-row', 'evidence-row', kindRuleClass('evidence', e), onOpen ? 'is-openable' : '']
        .filter(Boolean)
        .join(' ')}
      data-ref={e.ref}
    >
      {onOpen && (
        <button type="button" className="row-hit" aria-label={`Open source ${e.ref}: ${e.title}`} onClick={() => onOpen(e.ref)} />
      )}
      <div className="row-content">
        <p className="row-line">
          <span className="row-ref tabular">{e.ref}</span>
          <VerificationBadge verification={e.verification} />
          {e.imported && <ImportedTag />}
        </p>
        <p className={['evidence-title', retired ? 'is-retired' : ''].filter(Boolean).join(' ')}>{e.title}</p>
        {e.summary && <p className="evidence-summary is-clamped">{e.summary}</p>}
        <p className="evidence-meta">
          <span>{sourceTypeLabels[e.source_type] ?? e.source_type}</span>
          {e.source_label && <span>{e.source_label}</span>}
        </p>
      </div>
    </div>
  )
}
