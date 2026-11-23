// Finding row (a claim or a hypothesis). Line 1: ref, label, status when not
// the default. Then the statement in Literata, full text. Line 3: source
// chips. A ruled-out row (disproven hypothesis, rejected claim) explains
// itself first: "Ruled out by E-005:" and that source's full title, from its
// first contradicting link. With onOpen the whole row opens
// the record through a stretched button under the content, so the chips stay
// separate buttons (no nested controls); without it nothing is clickable.
import type { Claim, ClaimStatus, Hypothesis, HypothesisStatus } from '../../api/types'
import { statementMarks } from '../../lib/recordState'

import { EvidenceChip } from '../record/EvidenceChip'
import { ImportedTag } from '../record/ImportedTag'
import { kindRuleClass } from '../record/KindRule'
import { RecordStatusLabel } from '../record/StatusLabel'
import { VerificationBadge } from '../record/VerificationBadge'
import './RecordRow.css'

export type RecordRowProps =
  | { kind: 'claim'; record: Claim; onOpen?: (ref: string) => void }
  | { kind: 'hypothesis'; record: Hypothesis; onOpen?: (ref: string) => void }

const RETIRED = new Set(['superseded', 'rejected', 'disproven'])
const RULED_OUT = new Set(['rejected', 'disproven'])

export function RecordRow(props: RecordRowProps) {
  const { kind, record: r, onOpen } = props
  const retired = RETIRED.has(r.status)
  const ruledOutBy = RULED_OUT.has(r.status) ? r.links.find((l) => l.relation === 'contradicts') : undefined
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
        {(ruledOutBy || r.links.length > 0) && <div className="row-line record-foot">
          {ruledOutBy && (
            <span className="record-ruled-out-by">
              Ruled out by <span className="tabular">{ruledOutBy.evidence_ref}</span>
              {ruledOutBy.evidence_title ? `: ${ruledOutBy.evidence_title}` : ''}
            </span>
          )}
          {r.links.length > 0 && (
            <span className="chip-list">
              {r.links.map((l) => (
                <EvidenceChip
                  key={l.evidence_ref}
                  target={l.evidence_ref}
                  relation={l.relation}
                  linkKind={kind}
                  text={l.evidence_title}
                  onOpen={onOpen}
                />
              ))}
            </span>
          )}
        </div>}
      </div>
    </div>
  )
}
