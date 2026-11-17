// The empty-case guide: a case with no evidence,
// claims, hypotheses or questions shows one block that leads to connecting
// an AI, so the path the empty screen started is still on screen after New
// case. It disappears once the case has any record. Connect AI clients...
// opens the same dialog as the Data menu's entry.
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
      <p>Ask your AI to work this question and save what it finds here. Connect it once, then use the first finding prompt.</p>
      <div className="first-finding-actions">
        <Button variant="primary" icon={Plug} onClick={() => setConnectOpen(true)}>Connect AI clients...</Button>
        <Button variant="secondary" icon={Plus} onClick={onAddEvidence}>Add evidence myself</Button>
      </div>
      {connectOpen && <ConnectClientsDialog onClose={() => setConnectOpen(false)} />}
    </section>
  )
}
