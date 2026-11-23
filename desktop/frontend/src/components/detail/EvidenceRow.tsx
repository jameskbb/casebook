// Source row in the question page's Sources list, which shows every source
// once. Line 1: its check label. Then the name 16/600 and where to find it;
// with invRef, Show the exact words under it, the same toggle as the source
// drawer. What it shows is in the drawer the row opens.
import type { Evidence } from '../../api/types'
import { ExactWords } from '../drawers/EvidenceDrawer'
import { ImportedTag } from '../record/ImportedTag'
import { kindRuleClass } from '../record/KindRule'
import { VerificationBadge } from '../record/VerificationBadge'
import './RecordRow.css'
import './EvidenceRow.css'

export interface EvidenceRowProps {
  evidence: Evidence
  onOpen?: (ref: string) => void
  /** The question's ref: the row offers Show the exact words when the source has them. */
  invRef?: string
}

export function EvidenceRow({ evidence: e, onOpen, invRef }: EvidenceRowProps) {
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
        {e.source_ref && <p className="evidence-where">Where to find it: {e.source_ref}</p>}
        {invRef && e.has_raw_detail && <ExactWords evidence={e} invRef={invRef} inline />}
      </div>
    </div>
  )
}
