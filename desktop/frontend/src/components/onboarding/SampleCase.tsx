// This fictional example is presentation only. It never reads or writes a case.
import { useEffect, useRef, useState } from 'react'
import { ArrowLeft, ChevronDown, FileText, Plug, Plus, Quote } from 'lucide-react'
import type { Claim } from '../../api/types'
import { RecordRow } from '../detail/RecordRow'
import { VerificationBadge } from '../record/VerificationBadge'
import { Button } from '../ui/Button'
import './SampleCase.css'

const sampleClaim: Claim = {
  id: 0,
  ref: 'C-002',
  investigation_ref: 'fictional-sample',
  statement: 'Implementation services account for $1.9M of the $2.3M rise in cost of delivery.',
  confidence: 'medium',
  confidence_rationale: 'The cost export identifies the largest increase, but the reason for the extra hours still needs review.',
  status: 'active',
  verification: 'ai_generated',
  superseded_by: '',
  created_at: '',
  updated_at: '',
  created_by: { kind: 'ai', client: 'sample' },
  links: [{
    evidence_ref: 'E-005',
    relation: 'supports',
    evidence_title: 'Northeast implementation services cost export',
    created_at: '',
    created_by: { kind: 'ai', client: 'sample' },
  }],
}

interface SampleCaseProps {
  onBack: () => void
  onNew: () => void
  onConnect: () => void
}

export function SampleCase({ onBack, onNew, onConnect }: SampleCaseProps) {
  const [sourceOpen, setSourceOpen] = useState(false)
  const [shareOpen, setShareOpen] = useState(false)
  const sourceRef = useRef<HTMLDivElement>(null)
  const headingRef = useRef<HTMLHeadingElement>(null)
  useEffect(() => { headingRef.current?.focus() }, [])
  const openSource = () => {
    setSourceOpen(true)
    // The source is already in the case. Expand it and move to its heading.
    requestAnimationFrame(() => {
      sourceRef.current?.focus()
      sourceRef.current?.scrollIntoView?.({ block: 'nearest' })
    })
  }
  return (
    <article className="sample-case" aria-label="Fictional margin case">
      <header className="sample-toolbar">
        <Button variant="ghost" icon={ArrowLeft} onClick={onBack}>Back to your cases</Button>
        <span className="sample-readonly">Fictional example · Read-only</span>
      </header>
      <div className="sample-reading">
        <div className="sample-intro">
          <h1 ref={headingRef} tabIndex={-1}>Northeast margin down 6 points in Q3</h1>
          <p>What explains the decline, and what can we confidently take to the regional review?</p>
          <p className="sample-note">All figures, sources and findings below are invented. This example is never saved to your database.</p>
        </div>
        <section className="sample-section" aria-labelledby="sample-finding">
          <h2 id="sample-finding"><Quote size={20} strokeWidth={1.75} aria-hidden="true" />Start with one finding</h2>
          <p>Your AI saves a concise claim and links the evidence it used. Confidence expresses uncertainty; the verification label says what has been checked.</p>
          <RecordRow kind="claim" record={sampleClaim} onOpen={openSource} />
        </section>
        <section className="sample-section" aria-labelledby="sample-evidence">
          <h2 id="sample-evidence"><FileText size={20} strokeWidth={1.75} aria-hidden="true" />Review the evidence</h2>
          <div className="sample-source" ref={sourceRef} tabIndex={-1}>
            <div className="sample-source-head"><span className="sample-ref">E-005</span><VerificationBadge verification="ai_generated" /></div>
            <h3>Northeast implementation services cost export</h3>
            <p>Finance cost export, Q2 compared with Q3, Northeast. Fictional source.</p>
            <Button variant="secondary" icon={ChevronDown} aria-expanded={sourceOpen} onClick={() => setSourceOpen((open) => !open)}>
              {sourceOpen ? 'Hide source excerpt' : 'Read source excerpt'}
            </Button>
            {sourceOpen && <div className="sample-source-detail">
              <table>
                <caption>Illustrative cost comparison</caption>
                <thead><tr><th scope="col">Measure</th><th scope="col">Q3 compared with Q2</th></tr></thead>
                <tbody>
                  <tr><th scope="row">Cost of delivery</th><td>+$2.3M</td></tr>
                  <tr><th scope="row">Implementation services</th><td>+$1.9M</td></tr>
                  <tr><th scope="row">Contractor hours</th><td>+78%</td></tr>
                  <tr><th scope="row">Contractor rate</th><td>Flat</td></tr>
                </tbody>
              </table>
              <p>Check the period, region and calculation against the original workbook. The export supports the size of the implementation cost increase, but does not explain why the extra hours were needed.</p>
              <p>In your own case, open the claim, review its linked evidence, then choose Verify myself only after you have checked it. Use Dispute when a finding does not hold up.</p>
            </div>}
          </div>
          <p><strong>Still open:</strong> Can onboarding be charged on the Q4 deals already signed under the July playbook?</p>
        </section>
        <section className="sample-section" aria-labelledby="sample-share">
          <h2 id="sample-share">Show your team what you found</h2>
          <p>Write a conclusion that names the evidence and the remaining uncertainty. Share summary keeps verification labels and source references beside the findings.</p>
          <Button variant="secondary" aria-expanded={shareOpen} onClick={() => setShareOpen((open) => !open)}>{shareOpen ? 'Hide example summary' : 'See example summary'}</Button>
          {shareOpen && <div className="sample-summary">
            <h3>Fictional summary preview</h3>
            <p><strong>Conclusion · AI generated · Medium confidence:</strong> Implementation services are the largest recorded contributor to the cost increase behind the margin decline. The effect of onboarding policy still needs review.</p>
            <p><strong>C-002 · AI generated · Medium confidence:</strong> Implementation services account for $1.9M of the $2.3M cost rise. Supports: E-005, Northeast implementation services cost export (AI generated).</p>
            <p><strong>Open question:</strong> Can onboarding be charged on the Q4 deals already signed under the July playbook?</p>
            <p className="sample-note">Review the preview for sensitive details before copying or saving a real summary. Casebook prepares a file or clipboard text; you choose where it goes.</p>
          </div>}
        </section>
        <footer className="sample-next">
          <h2>Try it with your own question</h2>
          <p>Connect an AI client, create a case, and ask it to save one finding with a source. Return here to review the evidence before sharing.</p>
          <div className="sample-actions"><Button variant="primary" icon={Plug} onClick={onConnect}>Connect your AI</Button><Button variant="secondary" icon={Plus} onClick={onNew}>New case</Button></div>
        </footer>
      </div>
    </article>
  )
}
