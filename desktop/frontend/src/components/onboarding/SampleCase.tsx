// This made-up example is presentation only. It never reads or writes a question.
import { useEffect, useRef, useState } from 'react'
import { ArrowLeft, ChevronDown, FileText, Plug, Plus, Quote } from 'lucide-react'
import type { Claim } from '../../api/types'
import { RecordRow } from '../detail/RecordRow'
import { VerificationBadge } from '../record/VerificationBadge'
import { Button } from '../ui/Button'
import './SampleCase.css'

const SOURCE_TITLE = 'Call log, September and October'
const FINDING = 'Almost all of the extra calls were about a bill, not about buying.'
const NOT_TESTED = 'A billing mistake in October charged some customers twice.'

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
  const headingRef = useRef<HTMLHeadingElement>(null)
  useEffect(() => { headingRef.current?.focus() }, [])
  return (
    <article className="sample-case" aria-label="Call center example">
      <header className="sample-toolbar">
        <Button variant="ghost" icon={ArrowLeft} onClick={onBack}>Back to your questions</Button>
        <span className="sample-readonly">Made-up example. Nothing here can be changed.</span>
      </header>
      <div className="sample-reading">
        <div className="sample-intro">
          <h1 ref={headingRef} tabIndex={-1}>Why did calls go up 30% in October when sales stayed the same?</h1>
          <p className="sample-note">Every number, source and finding below is made up. The example is never saved with your questions.</p>
        </div>
        <section className="sample-section" aria-labelledby="sample-finding">
          <h2 id="sample-finding"><Quote size={20} strokeWidth={1.75} aria-hidden="true" />Start with one finding</h2>
          <p>Your AI saves each finding with the source it came from. Each one says who has checked it.</p>
          <RecordRow kind="claim" record={sampleClaim} />
        </section>
        <section className="sample-section" aria-labelledby="sample-evidence">
          <h2 id="sample-evidence"><FileText size={20} strokeWidth={1.75} aria-hidden="true" />Check the source</h2>
          <div className="sample-source">
            <div className="sample-source-head"><span className="sample-ref">E-001</span><VerificationBadge verification="ai_generated" /></div>
            <h3>{SOURCE_TITLE}</h3>
            <p>Every call to the sales line, with the reason for the call. Made up for this example.</p>
            <Button variant="secondary" icon={ChevronDown} aria-expanded={sourceOpen} onClick={() => setSourceOpen((open) => !open)}>
              {sourceOpen ? 'Hide the source' : 'Read the source'}
            </Button>
            {sourceOpen && <div className="sample-source-detail">
              <table>
                <caption>Calls by reason</caption>
                <thead><tr><th scope="col">Reason for the call</th><th scope="col">September</th><th scope="col">October</th></tr></thead>
                <tbody>
                  <tr><th scope="row">Wanted to buy</th><td>2,000</td><td>2,050</td></tr>
                  <tr><th scope="row">Asked about a bill</th><td>400</td><td>1,070</td></tr>
                  <tr><th scope="row">All calls</th><td>2,400</td><td>3,120</td></tr>
                </tbody>
              </table>
              <p>Open the call log and count the calls yourself. It shows what the extra calls were about. It does not show why so many people asked about a bill.</p>
              <p>In your own question, open the finding, read its sources, then choose I checked this. Choose Sources disagree when a source says something different.</p>
            </div>}
          </div>
          <p><strong>Not tested yet:</strong> {NOT_TESTED}</p>
        </section>
        <section className="sample-section" aria-labelledby="sample-share">
          <h2 id="sample-share">Share the answer</h2>
          <p>Write the answer once the findings back it. Share the answer shows each finding with its sources and who checked them.</p>
          <Button variant="secondary" aria-expanded={shareOpen} onClick={() => setShareOpen((open) => !open)}>{shareOpen ? 'Hide the example' : 'See an example'}</Button>
          {shareOpen && <div className="sample-summary">
            <h3>Example of a shared answer</h3>
            <p><strong>Answer, not checked:</strong> The extra calls were customers asking about a bill. About the same number of people called to buy, so sales stayed the same.</p>
            <p><strong>Finding, not checked:</strong> {FINDING} Source: {SOURCE_TITLE} (not checked).</p>
            <p><strong>Not tested yet:</strong> {NOT_TESTED}</p>
            <p className="sample-note">Read it for anything private before you send it. Casebook gives you text to paste or a file. You choose where it goes.</p>
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
