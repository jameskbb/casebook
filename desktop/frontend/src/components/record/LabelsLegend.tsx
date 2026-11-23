// What the labels mean: one sentence for each of the four check labels a
// finding or source can show. The other words on the page (Ruled out, Not
// tested yet) are section headings and explain themselves there.
import { verificationHelp, verificationLabels } from '../../lib/labels'
import { Button } from '../ui/Button'
import { Dialog } from '../ui/Dialog'
import './LabelsLegend.css'

const ROWS = ['ai_generated', 'source_verified', 'human_verified', 'disputed'] as const

export function LabelsLegend({ onClose }: { onClose: () => void }) {
  return (
    <Dialog title="What the labels mean" size="confirm" onClose={onClose} footer={<Button onClick={onClose}>Close</Button>}>
      <dl className="labels-legend">
        {ROWS.map((v) => (
          <div key={v} className="labels-legend-row">
            <dt>{verificationLabels[v]}</dt>
            <dd>{verificationHelp[v]}</dd>
          </div>
        ))}
      </dl>
    </Dialog>
  )
}
