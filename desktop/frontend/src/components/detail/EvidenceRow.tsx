// Evidence row (UI-SPEC Layout, Evidence row). Line 1: ref and verification.
// Title 16/600, summary clamped to two lines (the full text is in the
// drawer), a meta line with the source type then the source label (omitted
// when empty), then what it supports and contradicts as claim and
// hypothesis chips.
import type { Evidence, InvestigationDetail } from '../../api/types'
import { sourceTypeLabels } from '../../lib/labels'
import { usedBy, type UsedBy } from '../../lib/records'
import { EvidenceChip } from '../record/EvidenceChip'
import { ImportedTag } from '../record/ImportedTag'
import { kindRuleClass } from '../record/KindRule'
import { VerificationBadge } from '../record/VerificationBadge'
import './RecordRow.css'
import './EvidenceRow.css'

export interface EvidenceRowProps {
  evidence: Evidence
  detail: InvestigationDetail
  onOpen?: (ref: string) => void
}

export function EvidenceRow({ evidence: e, detail, onOpen }: EvidenceRowProps) {
  const links = usedBy(e, detail)
  const supports = links.filter((l) => l.relation === 'supports')
  const contradicts = links.filter((l) => l.relation === 'contradicts')
  const retired = e.verification === 'superseded'
  const chips = (list: UsedBy[]) =>
    list.map((l) => (
      <EvidenceChip
        key={l.ref}
        target={l.ref}
        relation={l.relation}
        linkKind={l.kind}
        placement="evidence"
        text={l.statement}
        onOpen={onOpen}
      />
    ))
  return (
    <div
      className={['detail-row', 'evidence-row', kindRuleClass('evidence', e), onOpen ? 'is-openable' : '']
        .filter(Boolean)
        .join(' ')}
      data-ref={e.ref}
    >
      {onOpen && (
        <button type="button" className="row-hit" aria-label={`Open evidence ${e.ref}: ${e.title}`} onClick={() => onOpen(e.ref)} />
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
        {links.length > 0 && (
          <div className="row-line evidence-links">
            {supports.length > 0 && (
              <span className="evidence-link-group">
                <span className="evidence-link-label">Supports</span>
                {chips(supports)}
              </span>
            )}
            {contradicts.length > 0 && (
              <span className="evidence-link-group">
                <span className="evidence-link-label">Contradicts</span>
                {chips(contradicts)}
              </span>
            )}
          </div>
        )}
      </div>
    </div>
  )
}
