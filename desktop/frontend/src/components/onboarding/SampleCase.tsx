// This made-up example is presentation only. It never reads or writes a question.
import { useEffect, useRef, useState } from 'react'
import { ArrowLeft, ChevronDown, FileText, Plug, Plus, Quote } from 'lucide-react'
import type { Claim } from '../../api/types'
import { RecordRow } from '../detail/RecordRow'
import { VerificationBadge } from '../record/VerificationBadge'
import { Button } from '../ui/Button'
import './SampleCase.css'

const SOURCE_TITLE = 'Carrier invoices, June and July'
const FINDING = 'The carrier raised its rates 30% on July 1.'
const NOT_TESTED = 'The same rate increase raised costs in our other regions.'

const sampleClaim: Claim = {
  id: 0,
  ref: 'C-001',
  investigation_ref: 'fictional-sample',
  statement: FINDING,
  confidence: 'unknown',
  confidence_rationale: '',
  status: 'active',
  verification: 'ai_generated',
  superseded_by: '',
  created_at: '',
  updated_at: '',
  created_by: { kind: 'ai', client: 'sample' },
  links: [{
    evidence_ref: 'E-001',
    relation: 'supports',
    evidence_title: SOURCE_TITLE,
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
    // The source is already on the page. Expand it and move to its heading.
    requestAnimationFrame(() => {
      sourceRef.current?.focus()
      sourceRef.current?.scrollIntoView?.({ block: 'nearest' })
    })
  }
  return (
    <article className="sample-case" aria-label="Margin example">
      <header className="sample-toolbar">
        <Button variant="ghost" icon={ArrowLeft} onClick={onBack}>Back to your questions</Button>
        <span className="sample-readonly">Made-up example · Read-only</span>
      </header>
      <div className="sample-reading">
        <div className="sample-intro">
          <h1 ref={headingRef} tabIndex={-1}>Why did our profit margin fall from 40% to 37% in Q3?</h1>
          <p className="sample-note">Every figure, source and finding below is made up. This example is never saved to your data.</p>
        </div>
        <section className="sample-section" aria-labelledby="sample-finding">
          <h2 id="sample-finding"><Quote size={20} strokeWidth={1.75} aria-hidden="true" />Start with one finding</h2>
          <p>Your AI saves each finding with the source it came from. The label says who has checked it.</p>
          <RecordRow kind="claim" record={sampleClaim} onOpen={openSource} />
        </section>
        <section className="sample-section" aria-labelledby="sample-evidence">
          <h2 id="sample-evidence"><FileText size={20} strokeWidth={1.75} aria-hidden="true" />Check the source</h2>
          <div className="sample-source" ref={sourceRef} tabIndex={-1}>
            <div className="sample-source-head"><span className="sample-ref">E-001</span><VerificationBadge verification="ai_generated" /></div>
            <h3>{SOURCE_TITLE}</h3>
            <p>Invoices from the carrier. Made up for this example.</p>
            <Button variant="secondary" icon={ChevronDown} aria-expanded={sourceOpen} onClick={() => setSourceOpen((open) => !open)}>
              {sourceOpen ? 'Hide the source' : 'Read the source'}
            </Button>
            {sourceOpen && <div className="sample-source-detail">
              <table>
                <caption>Average shipping cost per order</caption>
                <thead><tr><th scope="col">Month</th><th scope="col">Cost per order</th></tr></thead>
                <tbody>
                  <tr><th scope="row">June</th><td>$9.00</td></tr>
                  <tr><th scope="row">July</th><td>$11.70</td></tr>
                  <tr><th scope="row">Change</th><td>+30%</td></tr>
                </tbody>
              </table>
              <p>Open the invoices and check the months and the amounts yourself. They show that the cost went up. They do not show why.</p>
              <p>In your own question, open the finding, read its sources, then choose I checked this. Choose Sources disagree when a source contradicts it.</p>
            </div>}
          </div>
          <p><strong>Not tested yet:</strong> {NOT_TESTED}</p>
        </section>
        <section className="sample-section" aria-labelledby="sample-share">
          <h2 id="sample-share">Share the answer</h2>
          <p>Write the answer once the findings back it. Share the answer keeps each finding's label and its sources beside it.</p>
          <Button variant="secondary" aria-expanded={shareOpen} onClick={() => setShareOpen((open) => !open)}>{shareOpen ? 'Hide the example' : 'See an example'}</Button>
          {shareOpen && <div className="sample-summary">
            <h3>Example of a shared answer</h3>
            <p><strong>Answer · Not checked:</strong> Our carrier raised shipping rates 30% on July 1. Shipping is the only cost that grew faster than sales.</p>
            <p><strong>C-001 · Not checked:</strong> {FINDING} Supports: E-001, {SOURCE_TITLE} (Not checked).</p>
            <p><strong>Not tested yet:</strong> {NOT_TESTED}</p>
            <p className="sample-note">Read the preview for anything sensitive before you copy or save it. Casebook prepares a file or text to paste; you choose where it goes.</p>
          </div>}
        </section>
        <footer className="sample-next">
          <h2>Try it with your own question</h2>
          <p>Connect your AI, add a question, and ask your AI to save one finding with its source. Then come back here and check it.</p>
          <div className="sample-actions"><Button variant="primary" icon={Plug} onClick={onConnect}>Connect your AI</Button><Button variant="secondary" icon={Plus} onClick={onNew}>New question</Button></div>
        </footer>
      </div>
    </article>
  )
}
