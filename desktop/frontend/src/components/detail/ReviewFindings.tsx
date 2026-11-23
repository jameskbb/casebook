import { useCallback, useEffect, useRef, useState } from 'react'
import { api } from '../../api/api'
import { cleanMessage, toApiError } from '../../api/errors'
import type { ClaimStatus, HypothesisStatus, InvestigationDetail, ReviewFinding } from '../../api/types'
import { claimStatusLabels, hypothesisStatusLabels, sourceTypeLabels, verificationLabels } from '../../lib/labels'
import { CHECKED_ACTION, CONTRADICTING_TITLE, SETTLE_ACTION, SETTLE_COPY, SUPPORTING_TITLE } from '../drawers/StatementDrawer'
import { useWrite } from '../../state/useWrite'
import { RecordId } from '../record/RecordId'
import { VerificationBadge } from '../record/VerificationBadge'
import { Banner } from '../ui/Banner'
import { Button } from '../ui/Button'
import { Select } from '../ui/Select'
import './ReviewFindings.css'

const FILTERS = [
  { value: 'unchecked', label: 'Not checked' },
  { value: 'source_checked', label: 'AI checked the source, you have not' },
  { value: 'disputed', label: 'Sources disagree' },
  { value: 'support_attention', label: 'Sources need attention' },
  { value: 'all', label: 'All current findings' },
]

/** The engine's rule for the Sources need attention filter (review.go). */
const SUPPORT_ATTENTION_GLOSS = 'Findings with no supporting source, or whose supporting source is contradicted or replaced.'

/** A deliberate, individual check of a statement beside its source material. */
export function ReviewFindings({ detail, onClose }: { detail: InvestigationDetail; onClose: () => void }) {
  const invRef = detail.investigation.ref
  const write = useWrite()
  const [rows, setRows] = useState<ReviewFinding[]>([])
  const [current, setCurrent] = useState<ReviewFinding | null>(null)
  const [filter, setFilter] = useState('unchecked')
  const [loading, setLoading] = useState(true)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [settle, setSettle] = useState(false)
  const [result, setResult] = useState('')
  const [started, setStarted] = useState(false)
  const [loaded, setLoaded] = useState(false)
  const request = useRef(0)
  const heading = useRef<HTMLHeadingElement>(null)
  const select = (row: ReviewFinding | null) => {
    setCurrent(row)
    setSettle(false)
    setResult('')
    setError('')
  }
  const load = useCallback(async () => {
    const id = ++request.current
    setLoading(true)
    try {
      const next = await api.getReviewFindings(invRef)
      if (id !== request.current) return
      setRows(next)
      setLoaded(true)
    } catch (e) {
      if (id === request.current) setError(cleanMessage(toApiError(e).message))
    } finally {
      if (id === request.current) setLoading(false)
    }
  }, [invRef])
  useEffect(() => {
    void load()
    return () => { request.current++ }
  }, [load, detail])
  const queue = rows.filter((r) => filter === 'all' || r.filters.includes(filter))
  useEffect(() => {
    if (!loading && loaded && !started) {
      select(queue[0] ?? null)
      setStarted(true)
    }
  }, [loading, loaded, started, queue])
  useEffect(() => { heading.current?.focus() }, [current?.ref])

  const latest = current && rows.find((r) => r.ref === current.ref && r.kind === current.kind)
  const changed = Boolean(current && (!latest || latest.basis !== current.basis))
  const next = () => {
    const position = current ? rows.findIndex((r) => r.ref === current.ref && r.kind === current.kind) : -1
    select(queue.find((r) => rows.indexOf(r) > position) ?? null)
  }
  const review = async (verification: 'human_verified' | 'disputed', settleDispute = false) => {
    if (!current || busy || changed || loading) return
    setBusy(true)
    setError('')
    try {
      const saved = await write(() => api.reviewStatement(invRef, {
        kind: current.kind, ref: current.ref, basis: current.basis, verification, settle_dispute: settleDispute,
      }), { rethrowConflict: true, retry: false })
      if (saved) {
        setCurrent(saved)
        setRows((rs) => rs.map((r) => r.ref === saved.ref && r.kind === saved.kind ? saved : r))
        setSettle(false)
        setResult(verification === 'human_verified' ? 'You checked this finding.' : 'Marked this finding: sources disagree.')
      }
    } catch (e) {
      setError(cleanMessage(toApiError(e).message))
      void load()
    } finally { setBusy(false) }
  }

  return (
    <section className="review-findings" aria-label="Review findings">
      <div className="review-toolbar">
        <h2>Review findings</h2>
        <Button variant="secondary" onClick={onClose} disabled={busy}>Back to the question</Button>
      </div>
      <p className="review-intro">Check one finding against its sources. Only your own check marks it "{verificationLabels.human_verified}".</p>
      <Select label="Findings to review" value={filter} options={FILTERS} disabled={busy} note={filter === 'support_attention' ? SUPPORT_ATTENTION_GLOSS : undefined} onChange={(value) => {
        setFilter(value)
        select(rows.find((r) => value === 'all' || r.filters.includes(value)) ?? null)
        setStarted(true)
      }} />
      {error && <Banner tone="warning" action={<Button variant="ghost" onClick={() => void load()}>Reload findings</Button>}>{error}</Banner>}
      {loading && <p role="status">Loading latest findings...</p>}
      {current ? (
        <>
          {changed && <Banner tone="warning" action={<Button variant="secondary" disabled={busy || loading} onClick={() => select(latest || queue[0] || null)}>Show latest</Button>}>
            This finding or its sources changed. Read the latest version before checking it.
          </Banner>}
          <div className="review-comparison" aria-busy={busy || undefined}>
            <div className="review-statement">
              <h3 ref={heading} tabIndex={-1}>Finding <RecordId>{current.ref}</RecordId></h3>
              <div className="review-labels"><VerificationBadge verification={current.verification} /><span>{current.kind === 'claim' ? claimStatusLabels[current.status as ClaimStatus] : hypothesisStatusLabels[current.status as HypothesisStatus]}</span></div>
              <p className="review-statement-text">{current.statement}</p>

              {current.rationale && <p className="review-rationale">{current.rationale}</p>}
              {current.warnings.length > 0 && <div className="review-warnings"><h4>Sources need attention</h4><ul>{current.warnings.map((w) => <li key={w}>{w}</li>)}</ul></div>}
          {result && <p className="review-result" role="status">{result}</p>}
          {settle && <Banner tone="warning" action={<><Button variant="ghost" onClick={() => setSettle(false)}>Cancel</Button><Button variant="secondary" disabled={busy || changed || loading} onClick={() => void review('human_verified', true)}>{SETTLE_ACTION}</Button></>}>{SETTLE_COPY}</Banner>}
          <div className="review-actions">
            <Button variant="primary" disabled={busy || loading || changed || current.verification === 'human_verified'} onClick={() => current.verification === 'disputed' ? setSettle(true) : void review('human_verified')}>{CHECKED_ACTION}</Button>
            <Button variant="secondary" disabled={busy || loading || changed || current.verification === 'disputed'} onClick={() => void review('disputed')}>{verificationLabels.disputed}</Button>
            <Button variant="ghost" disabled={busy} onClick={next}>{result ? 'Next' : 'Skip'}</Button>
          </div>
            </div>
            <div className="review-sources">
              {(['supports', 'contradicts'] as const).map((relation) => {
                const evidence = current.evidence.filter((e) => e.relation === relation)
                return <section key={relation} aria-label={relation === 'supports' ? SUPPORTING_TITLE : CONTRADICTING_TITLE}>
                  <h3>{relation === 'supports' ? SUPPORTING_TITLE : CONTRADICTING_TITLE}</h3>
                  {evidence.length === 0 ? <p className="review-empty">{relation === 'supports' ? 'No source supports it yet.' : 'No source contradicts it.'}</p> : evidence.map(({ evidence: e }) => <article className="review-evidence" key={e.ref}>
                    <div className="review-labels"><RecordId>{e.ref}</RecordId><VerificationBadge verification={e.verification} /></div>
                    <h4>{e.title}</h4>
                    <p>{e.summary}</p>
                    <p className="review-source">{sourceTypeLabels[e.source_type]}{e.source_label && `: ${e.source_label}`}</p>
                    {e.source_ref && <p className="review-reference">{e.source_ref}</p>}
                    {e.raw_detail && <details><summary>Source detail</summary><pre>{e.raw_detail}</pre></details>}
                  </article>)}
                </section>
              })}
            </div>
          </div>

        </>
      ) : !loading && loaded && (rows.length === 0 ? <div className="review-finished"><h3>Nothing to review yet</h3><p>Findings your AI saves, and ones you add, appear here for you to check against their sources.</p></div> : <div className="review-finished"><h3>No more findings in this list</h3><p>Findings you skipped keep their label. Choose another list above or go back to the question.</p>{queue.length > 0 && <Button variant="secondary" onClick={() => select(queue[0])}>Start again</Button>}</div>)}
    </section>
  )
}
