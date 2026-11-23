// First open: the main pane with no questions at all. It says what the app
// is, how to use it in three steps, and offers the made-up example.
import { useRef, useState } from 'react'
import { BookOpen, Inbox, Plug, Plus, Upload } from 'lucide-react'
import { ConnectClientsDialog } from './dialogs/ConnectClientsDialog'
import { SampleCase } from './onboarding/SampleCase'
import { Button } from './ui/Button'
import { EmptyState } from './ui/EmptyState'
import './EmptyMain.css'

export interface EmptyMainProps {
  onNew: () => void
  /** Shows Add a question from a file... */
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
            heading="Check what your AI tells you"
            body="You ask your AI why a number moved, and it gives you an answer. This app keeps what it found, where each finding came from, and which ones you have checked yourself."
          />
          <ol className="empty-main-steps" aria-label="How to use it">
            <li><strong>Connect your AI.</strong> You do this once.</li>
            <li><strong>Ask your AI a question.</strong> Tell it to save what it finds here.</li>
            <li><strong>Check the sources and share the answer.</strong> Mark each source you have checked yourself.</li>
          </ol>
          <div className="empty-main-actions">
            <Button variant="primary" icon={Plug} onClick={() => setConnectOpen(true)}>Connect your AI</Button>
            <Button variant="secondary" icon={Plus} onClick={onNew}>New question</Button>
            {onImport && <Button variant="ghost" icon={Upload} onClick={onImport}>Add a question from a file...</Button>}
          </div>
          <div className="empty-main-guide">
            <h3>See an example</h3>
            <p>Why did calls go up 30% in October when sales stayed the same? Follow one finding back to its source.</p>
            <Button ref={sampleButton} variant="secondary" icon={BookOpen} onClick={() => setSampleOpen(true)}>Open the call center example</Button>
            <p className="empty-main-sample-note">The example is made up. Nothing is added to your questions.</p>
          </div>
        </div>
      )}
      {connectOpen && <ConnectClientsDialog onClose={() => setConnectOpen(false)} />}
    </>
  )
}
