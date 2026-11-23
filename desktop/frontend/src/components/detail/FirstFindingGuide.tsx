// The guide on a question with nothing in it yet: one block that leads to
// connecting an AI. It goes away once the question has a source, a finding
// or a follow-up question. Connect your AI... opens the same window as the
// Settings menu's entry.
import { useState } from 'react'
import { Plug, Plus } from 'lucide-react'
import type { InvestigationDetail } from '../../api/types'
import { ConnectClientsDialog } from '../dialogs/ConnectClientsDialog'
import { Button } from '../ui/Button'
import './FirstFindingGuide.css'

/** True when the case has no record of any kind yet. */
export function caseHasNoRecords(detail: InvestigationDetail): boolean {
  return detail.evidence.length === 0 && detail.claims.length === 0 && detail.hypotheses.length === 0 && detail.questions.length === 0
}

export function FirstFindingGuide({ onAddEvidence }: { onAddEvidence: () => void }) {
  const [connectOpen, setConnectOpen] = useState(false)
  return (
    <section className="first-finding-guide" aria-labelledby="first-finding-heading">
      <h2 id="first-finding-heading">Get your first finding</h2>
      <p>Ask your AI to work on this question and save what it finds here. Connect your AI once. Then copy the starter message and paste it into your AI.</p>
      <div className="first-finding-actions">
        <Button variant="primary" icon={Plug} onClick={() => setConnectOpen(true)}>Connect your AI...</Button>
        <Button variant="secondary" icon={Plus} onClick={onAddEvidence}>Add a source myself</Button>
      </div>
      {connectOpen && <ConnectClientsDialog onClose={() => setConnectOpen(false)} />}
    </section>
  )
}
