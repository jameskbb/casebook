// Check findings one by one: a finding on the left, its sources on the
// right, and I checked this or Sources disagree. Opened from the More menu.
import { useCallback, useEffect, useRef, useState } from 'react'
import { api } from '../../api/api'
import { plainError, toApiError } from '../../api/errors'
import type { HypothesisStatus, InvestigationDetail, ReviewFinding } from '../../api/types'
import { hypothesisStatusLabels, sourceTypeLabels, verificationLabels } from '../../lib/labels'
import { CHECK_FINDINGS_LABEL } from './MoreMenu'
import { CHECKED_ACTION, CONTRADICTING_TITLE, DISAGREE_ACTION, FINDING_CHANGED_COPY, SETTLE_ACTION, SETTLE_COPY, SUPPORTING_TITLE } from '../drawers/StatementDrawer'
import { useWrite } from '../../state/useWrite'
import { CodedText, RecordId } from '../record/RecordId'
import { VerificationBadge } from '../record/VerificationBadge'
import { Banner } from '../ui/Banner'
import { Button } from '../ui/Button'
import { SHOW_NEW_VERSION } from '../ui/ChangedElsewhereBanner'
import { Select } from '../ui/Select'
import './ReviewFindings.css'

// The engine's filter names (review.go), each with the words a person reads.
const FILTERS = [
  { value: 'unchecked', label: 'Not checked yet' },
  { value: 'source_checked', label: 'Only your AI checked' },
  { value: 'disputed', label: 'Sources disagree' },
  { value: 'support_attention', label: 'No source backs it' },
  { value: 'all', label: 'All findings' },
]

/** What to know before checking: written here from the sources shown, so no engine text or code reaches the screen. */
export function sourceWarnings(f: ReviewFinding): string[] {
  const backing = f.evidence.filter((e) => e.relation === 'supports')
  if (backing.length === 0) return ['No source backs this finding.']
  return backing.flatMap(({ evidence: e }) =>
    e.verification === 'superseded' ? [`"${e.title}" was replaced by a newer source.`] : e.verification === 'disputed' ? [`Sources disagree about "${e.title}".`] : [],
  )
}

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
      if (id === request.current) setError(plainError(toApiError(e)))
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
  const check = async (verification: 'human_verified' | 'disputed', settleDispute = false) => {
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
        setResult(verification === 'human_verified' ? 'You checked this finding.' : 'Saved: sources disagree.')
      }
    } catch (e) {
      setError(plainError(toApiError(e)))
      void load()
    } finally { setBusy(false) }
  }
  const warnings = current ? sourceWarnings(current) : []

  return (
    <section className="review-findings" aria-label={CHECK_FINDINGS_LABEL}>
      <div className="review-toolbar">
        <h2>{CHECK_FINDINGS_LABEL}</h2>
        <Button variant="secondary" onClick={onClose} disabled={busy}>Back to the question</Button>
      </div>
      <p className="review-intro">Read a finding next to its sources. It says "{verificationLabels.human_verified}" only after you check it yourself.</p>
      <Select label="Which findings" value={filter} options={FILTERS} disabled={busy} onChange={(value) => {
        setFilter(value)
        select(rows.find((r) => value === 'all' || r.filters.includes(value)) ?? null)
        setStarted(true)
      }} />
      {error && <Banner tone="warning" action={<Button variant="ghost" onClick={() => void load()}>Try again</Button>}>{error}</Banner>}
      {loading && <p role="status">Loading the findings...</p>}
      {current ? (
        <>
          {changed && <Banner tone="warning" action={<Button variant="secondary" disabled={busy || loading} onClick={() => select(latest || queue[0] || null)}>{SHOW_NEW_VERSION}</Button>}>
            {FINDING_CHANGED_COPY}
          </Banner>}
          <div className="review-comparison" aria-busy={busy || undefined}>
            <div className="review-statement">
              <h3 ref={heading} tabIndex={-1}>Finding <RecordId>{current.ref}</RecordId></h3>
              <div className="review-labels"><VerificationBadge verification={current.verification} />{current.kind === 'hypothesis' && <span>{hypothesisStatusLabels[current.status as HypothesisStatus]}</span>}</div>
              <p className="review-statement-text">{current.statement}</p>

              {current.rationale && <p className="review-rationale"><CodedText text={current.rationale} /></p>}
              {warnings.length > 0 && <div className="review-warnings"><h4>Before you check</h4><ul>{warnings.map((w) => <li key={w}>{w}</li>)}</ul></div>}
              {result && <p className="review-result" role="status">{result}</p>}
              {settle && <Banner tone="warning" action={<><Button variant="ghost" onClick={() => setSettle(false)}>Cancel</Button><Button variant="secondary" disabled={busy || changed || loading} onClick={() => void check('human_verified', true)}>{SETTLE_ACTION}</Button></>}>{SETTLE_COPY}</Banner>}
              <div className="review-actions">
                <Button variant="primary" disabled={busy || loading || changed || current.verification === 'human_verified'} onClick={() => current.verification === 'disputed' ? setSettle(true) : void check('human_verified')}>{CHECKED_ACTION}</Button>
                <Button variant="secondary" disabled={busy || loading || changed || current.verification === 'disputed'} onClick={() => void check('disputed')}>{DISAGREE_ACTION}</Button>
                <Button variant="ghost" disabled={busy} onClick={next}>{result ? 'Next' : 'Skip'}</Button>
              </div>
            </div>
            <div className="review-sources">
              {(['supports', 'contradicts'] as const).map((relation) => {
                const evidence = current.evidence.filter((e) => e.relation === relation)
                return <section key={relation} aria-label={relation === 'supports' ? SUPPORTING_TITLE : CONTRADICTING_TITLE}>
                  <h3>{relation === 'supports' ? SUPPORTING_TITLE : CONTRADICTING_TITLE}</h3>
                  {evidence.length === 0 ? <p className="review-empty">{relation === 'supports' ? 'No source backs it yet.' : 'No source goes against it.'}</p> : evidence.map(({ evidence: e }) => <article className="review-evidence" key={e.ref}>
                    <div className="review-labels"><RecordId>{e.ref}</RecordId><VerificationBadge verification={e.verification} /></div>
                    <h4>{e.title}</h4>
                    <p>{e.summary}</p>
                    <p className="review-source">{e.source_label || sourceTypeLabels[e.source_type]}</p>
                    {e.source_ref && <p className="review-reference">{e.source_ref}</p>}
                    {e.raw_detail && <details><summary>Show the exact words</summary><pre>{e.raw_detail}</pre></details>}
                  </article>)}
                </section>
              })}
            </div>
          </div>
        </>
      ) : !loading && loaded && (rows.length === 0 ? <div className="review-finished"><h3>No findings yet</h3><p>Findings your AI saves, and ones you add, show here for you to check against their sources.</p></div> : <div className="review-finished"><h3>No more findings in this list</h3><p>Findings you skipped stay as they were. Choose another list above or go back to the question.</p>{queue.length > 0 && <Button variant="secondary" onClick={() => select(queue[0])}>Start again</Button>}</div>)}
    </section>
  )
}
