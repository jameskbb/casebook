// Finding row (a claim or a hypothesis). Line 1: its check label and its
// status when not the default. Then the statement in Literata, full text.
// Then its sources, nested: a plus for one that backs it, a minus for one
// that contradicts it, the source's title, where it came from and its own
// check label, so a person reads a finding and its sources in one place.
// With onOpen the whole row opens the finding through a stretched button
// under the content, and each source is its own button (no nested controls);
// without it nothing is clickable.
import { Minus, Plus } from 'lucide-react'
import type { Claim, ClaimStatus, Evidence, EvidenceLink, Hypothesis, HypothesisStatus } from '../../api/types'
import { sourceTypeLabels } from '../../lib/labels'
import { statementMarks } from '../../lib/recordState'

import { ImportedTag } from '../record/ImportedTag'
import { kindRuleClass } from '../record/KindRule'
import { RecordId } from '../record/RecordId'
import { RecordStatusLabel } from '../record/StatusLabel'
import { VerificationBadge } from '../record/VerificationBadge'
import './RecordRow.css'

interface Common {
  onOpen?: (ref: string) => void
  /** The question's sources by code, for each nested source's label and origin. */
  sources?: Map<string, Evidence>
}

export type RecordRowProps = Common & ({ kind: 'claim'; record: Claim } | { kind: 'hypothesis'; record: Hypothesis })

const RETIRED = new Set(['superseded', 'rejected', 'disproven'])

export function RecordRow(props: RecordRowProps) {
  const { kind, record: r, onOpen, sources } = props
  const retired = RETIRED.has(r.status)
  // One coherent state: the badge and the status label never say
  // the same thing twice or contradict each other.
  const marks = statementMarks(kind, r)

  return (
    <div
      className={['detail-row', 'record-row', kindRuleClass(kind, r), onOpen ? 'is-openable' : '', retired ? 'is-retired' : '']
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
          {marks.badge && <VerificationBadge verification={marks.badge} />}
          {r.imported && <ImportedTag />}
          {props.kind === 'claim' ? (
            <RecordStatusLabel kind="claim" status={marks.status as ClaimStatus} supersededBy={r.superseded_by} />
          ) : (
            <RecordStatusLabel kind="hypothesis" status={marks.status as HypothesisStatus} supersededBy={r.superseded_by} />
          )}
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
      </div>
    </div>
  )
}

function FindingSource({ link: l, source, onOpen }: { link: EvidenceLink; source?: Evidence; onOpen?: (ref: string) => void }) {
  const against = l.relation === 'contradicts'
  const Sign = against ? Minus : Plus
  const from = source ? source.source_label || (sourceTypeLabels[source.source_type] ?? '') : ''
  // The name a screen reader says: "Supports this: <title> <where from> <check label>".
  const content = (
    <>
      <Sign className="finding-source-sign" size={16} strokeWidth={2} aria-hidden="true" />
      <span className="finding-source-text">
        <span className="finding-source-title">
          <span className="sr-only">{against ? 'Contradicts this:' : 'Supports this:'}</span> <RecordId>{l.evidence_ref}</RecordId>{' '}
          {l.evidence_title}
        </span>{' '}
        {from && <span className="finding-source-from">{from}</span>}
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
