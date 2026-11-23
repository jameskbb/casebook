// A brand new question, with nothing in it yet: two buttons to start by
// hand, and one line saying an AI can fill it in. It goes away once the
// question has a source, a finding or something to find out. Connect your
// AI... opens the same window as the Settings menu's entry.
import { useState } from 'react'
import { Plus } from 'lucide-react'
import type { InvestigationDetail } from '../../api/types'
import { ConnectClientsDialog } from '../dialogs/ConnectClientsDialog'
import { Button } from '../ui/Button'
import './FirstFindingGuide.css'

export const ADD_SOURCE = 'Add a source'
export const ADD_FINDING = 'Add a finding'

/** True when the question has nothing in it yet. */
export function caseHasNoRecords(detail: InvestigationDetail): boolean {
  return detail.evidence.length === 0 && detail.claims.length === 0 && detail.hypotheses.length === 0 && detail.questions.length === 0
}

export function FirstFindingGuide({ onAddSource, onAddFinding }: { onAddSource: () => void; onAddFinding: () => void }) {
  const [connectOpen, setConnectOpen] = useState(false)
  return (
    <section className="first-finding-guide" aria-label="Start here">
      <div className="first-finding-actions">
        <Button variant="primary" icon={Plus} onClick={onAddSource}>{ADD_SOURCE}</Button>
        <Button variant="secondary" icon={Plus} onClick={onAddFinding}>{ADD_FINDING}</Button>
      </div>
      <p>
        Your AI can add sources and findings here too.{' '}
        <button type="button" className="text-button" onClick={() => setConnectOpen(true)}>Connect your AI...</button>
      </p>
      {connectOpen && <ConnectClientsDialog onClose={() => setConnectOpen(false)} />}
    </section>
  )
}
