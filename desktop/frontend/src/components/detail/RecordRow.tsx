// Finding row (a claim or a hypothesis). Line 1: its one label, worked out
// from its sources. Then the statement in Literata, full text. Then its
// sources, nested: a tag saying Backs it or Goes against it, the source's
// name and its own check label, so a person reads a finding and its sources
// in one place. With onOpen the whole row opens the finding through a
// stretched button under the content, and each source is its own button (no
// nested controls); without it nothing is clickable. actions sit on the row
// below its sources.
import type { ReactNode } from 'react'
import type { Evidence, EvidenceLink } from '../../api/types'
import { relationTags } from '../../lib/labels'
import { isReplaced, isRuledOut, sourcesDisagree, type Finding, type Sources } from '../../lib/records'
import { ImportedTag } from '../record/ImportedTag'
import { kindRuleClass } from '../record/KindRule'
import { RecordId } from '../record/RecordId'
import { FindingLabel } from '../record/StatusLabel'
import { VerificationBadge } from '../record/VerificationBadge'
import './RecordRow.css'

export type RecordRowProps = Finding & {
  onOpen?: (ref: string) => void
  /** The question's sources by code: the finding's label and each nested source's label come from them. */
  sources?: Sources
  /** Buttons on the row, for example This turned out right and Rule this out. */
  actions?: ReactNode
}

const NO_SOURCES: Sources = new Map()

export function RecordRow({ onOpen, sources, actions, ...finding }: RecordRowProps) {
  const r = finding.record
  const retired = isReplaced(r) || isRuledOut(r)
  // The hatched rule goes with the label Sources disagree, never with the stored value alone.
  const rule = kindRuleClass(finding.kind, { status: sourcesDisagree(r, sources ?? NO_SOURCES) ? 'disputed' : r.status })
  return (
    <div
      className={['detail-row', 'record-row', rule, onOpen ? 'is-openable' : '', retired ? 'is-retired' : '']
        .filter(Boolean)
        .join(' ')}
      data-ref={r.ref}
    >
      {onOpen && (
        <button
          type="button"
          className="row-hit"
          aria-label={`Open finding ${r.ref}: ${r.statement}`}
          onClick={() => onOpen(r.ref)}
        />
      )}
      <div className="row-content">
        <p className="row-line">
          <span className="row-ref tabular">{r.ref}</span>
          <FindingLabel finding={finding} sources={sources} />
          {r.imported && <ImportedTag />}
        </p>
        <p className={['record-statement', retired ? 'is-retired' : ''].filter(Boolean).join(' ')}>{r.statement}</p>
        {r.links.length > 0 && (
          <ul className="finding-sources" aria-label="Sources for this finding">
            {r.links.map((l) => (
              <li key={l.evidence_ref}>
                <FindingSource link={l} source={sources?.get(l.evidence_ref)} onOpen={onOpen} />
              </li>
            ))}
          </ul>
        )}
        {actions && <div className="record-row-actions">{actions}</div>}
      </div>
    </div>
  )
}

function FindingSource({ link: l, source, onOpen }: { link: EvidenceLink; source?: Evidence; onOpen?: (ref: string) => void }) {
  const against = l.relation === 'contradicts'
  // Read out as "Backs it: <name> <check label>".
  const content = (
    <>
      <span className="finding-source-tag">
        {relationTags[l.relation]}
        <span className="sr-only">:</span>
      </span>{' '}
      <span className="finding-source-title">
        <RecordId>{l.evidence_ref}</RecordId> {l.evidence_title}
      </span>{' '}
      {source && <VerificationBadge verification={source.verification} />}
    </>
  )
  const className = ['finding-source', against ? 'is-against' : ''].filter(Boolean).join(' ')
  if (!onOpen) {
    return <span className={className}>{content}</span>
  }
  return (
    <button type="button" className={className} data-ref={l.evidence_ref} onClick={() => onOpen(l.evidence_ref)}>
      {content}
    </button>
  )
}
