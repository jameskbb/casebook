import { useCallback, useEffect, useRef, useState } from 'react'
import { api } from '../../api/api'
import { cleanMessage, toApiError } from '../../api/errors'
import type { ClaimStatus, HypothesisStatus, InvestigationDetail, ReviewFinding } from '../../api/types'
import { claimStatusLabels, confidenceLabels, hypothesisStatusLabels, sourceTypeLabels } from '../../lib/labels'
import { useWrite } from '../../state/useWrite'
import { VerificationBadge } from '../record/VerificationBadge'
import { Banner } from '../ui/Banner'
import { Button } from '../ui/Button'
import { Select } from '../ui/Select'
import './ReviewFindings.css'

const FILTERS = [
  { value: 'unchecked', label: 'AI generated, not yet checked by you' },
  { value: 'source_checked', label: 'Source verified, not yet checked by you' },
  { value: 'disputed', label: 'Disputed' },
  { value: 'support_attention', label: 'Support needs attention' },
  { value: 'all', label: 'All current findings' },
]

/** The engine's rule for the Support needs attention filter (review.go). */
const SUPPORT_ATTENTION_GLOSS = 'Findings with no supporting evidence, or whose supporting evidence is disputed or superseded.'

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
        setResult(verification === 'human_verified' ? `You verified ${saved.ref}.` : `You disputed ${saved.ref}.`)
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
        <Button variant="secondary" onClick={onClose} disabled={busy}>Back to case</Button>
      </div>
      <p className="review-intro">Check one statement against its evidence. Only your own check marks it as verified by a person.</p>
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
            This finding or its evidence changed. Read the latest version before reviewing.
          </Banner>}
          <div className="review-comparison" aria-busy={busy || undefined}>
            <div className="review-statement">
              <h3 ref={heading} tabIndex={-1}>{current.kind === 'claim' ? 'Claim' : 'Hypothesis'} {current.ref}</h3>
              <div className="review-labels"><VerificationBadge verification={current.verification} /><span>{current.kind === 'claim' ? claimStatusLabels[current.status as ClaimStatus] : hypothesisStatusLabels[current.status as HypothesisStatus]}</span></div>
              <p className="review-statement-text">{current.statement}</p>
              <p className="review-confidence">Confidence: {confidenceLabels[current.confidence]}</p>
              {current.rationale && <p className="review-rationale">{current.rationale}</p>}
              {current.warnings.length > 0 && <div className="review-warnings"><h4>Support needs attention</h4><ul>{current.warnings.map((w) => <li key={w}>{w}</li>)}</ul></div>}
          {result && <p className="review-result" role="status">{result}</p>}
          {settle && <Banner tone="warning" action={<><Button variant="ghost" onClick={() => setSettle(false)}>Cancel</Button><Button variant="secondary" disabled={busy || changed || loading} onClick={() => void review('human_verified', true)}>Verify and settle dispute</Button></>}>Verifying this finding will settle its dispute. Confirm only after checking the statement and evidence.</Banner>}
          <div className="review-actions">
            <Button variant="primary" disabled={busy || loading || changed || current.verification === 'human_verified'} onClick={() => current.verification === 'disputed' ? setSettle(true) : void review('human_verified')}>Verify myself</Button>
            <Button variant="secondary" disabled={busy || loading || changed || current.verification === 'disputed'} onClick={() => void review('disputed')}>Dispute</Button>
            <Button variant="ghost" disabled={busy} onClick={next}>{result ? 'Next' : 'Skip'}</Button>
          </div>
            </div>
            <div className="review-sources">
              {(['supports', 'contradicts'] as const).map((relation) => {
                const evidence = current.evidence.filter((e) => e.relation === relation)
                return <section key={relation} aria-label={relation === 'supports' ? 'Supporting evidence' : 'Contradicting evidence'}>
                  <h3>{relation === 'supports' ? 'Supporting evidence' : 'Contradicting evidence'}</h3>
                  {evidence.length === 0 ? <p className="review-empty">{relation === 'supports' ? 'No supporting evidence is linked.' : 'No contradicting evidence is linked.'}</p> : evidence.map(({ evidence: e }) => <article className="review-evidence" key={e.ref}>
                    <div className="review-labels"><span className="tabular">{e.ref}</span><VerificationBadge verification={e.verification} /></div>
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
      ) : !loading && loaded && (rows.length === 0 ? <div className="review-finished"><h3>Nothing to review yet</h3><p>Findings your AI saves, and claims you add, appear here for you to check against their evidence.</p></div> : <div className="review-finished"><h3>No more findings in this filter</h3><p>Skipped findings keep their verification. Choose another filter or return to the case.</p>{queue.length > 0 && <Button variant="secondary" onClick={() => select(queue[0])}>Start again</Button>}</div>)}
    </section>
  )
}
