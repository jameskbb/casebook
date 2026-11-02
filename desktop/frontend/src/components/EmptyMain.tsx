// The main pane with no investigations at all (UI-SPEC Empty states, D-24).
import { useRef, useState } from 'react'
import { BookOpen, Inbox, Plug, Plus, Upload } from 'lucide-react'
import { ConnectClientsDialog } from './dialogs/ConnectClientsDialog'
import { SampleCase } from './onboarding/SampleCase'
import { Button } from './ui/Button'
import { EmptyState } from './ui/EmptyState'
import './EmptyMain.css'

export interface EmptyMainProps {
  onNew: () => void
  /** Shows Import case... (wired by Plan 04). */
  onImport?: () => void
}

export function EmptyMain({ onNew, onImport }: EmptyMainProps) {
  const [sampleOpen, setSampleOpen] = useState(false)
  const [connectOpen, setConnectOpen] = useState(false)
  const sampleButton = useRef<HTMLButtonElement>(null)
  const closeSample = () => {
    setSampleOpen(false)
    requestAnimationFrame(() => sampleButton.current?.focus())
  }
  const startCase = () => {
    setSampleOpen(false)
    onNew()
  }
  return (
    <>
      {sampleOpen ? (
        <SampleCase onBack={closeSample} onNew={startCase} onConnect={() => setConnectOpen(true)} />
      ) : (
        <div className="empty-main">
          <EmptyState
            icon={Inbox}
            heading="Start your first case"
            body="Connect your AI, then ask it a question that needs digging. What it finds lands here for you to check and share."
            actions={
              <>
                <Button variant="primary" icon={Plug} onClick={() => setConnectOpen(true)}>Connect your AI</Button>
                <Button variant="secondary" icon={Plus} onClick={onNew}>New case</Button>
                {onImport && <Button variant="ghost" icon={Upload} onClick={onImport}>Import case...</Button>}
              </>
            }
          />
          <div className="empty-main-guide">
            <h3>See what a case gives you</h3>
            <p>Why did Northeast margin fall six points? Follow a finding back to its source before you take it to your team.</p>
            <Button ref={sampleButton} variant="secondary" icon={BookOpen} onClick={() => setSampleOpen(true)}>Open fictional margin case</Button>
            <p className="empty-main-sample-note">Read-only example. Nothing is added to your cases.</p>
          </div>
        </div>
      )}
      {connectOpen && <ConnectClientsDialog onClose={() => setConnectOpen(false)} />}
    </>
  )
}
