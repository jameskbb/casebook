// What the labels mean (plain-reader round 1): one sentence per label a
// claim, hypothesis or piece of evidence shows in the reading column, so a
// person can defend a line without knowing the engine's words for it.
// Confidence and Current follow the verification rows so a reader can tell
// "who checked it" apart from "how sure" and "still stands on it".
import { claimStatusLabels, confidenceHelp, confidenceLabels, hypothesisStatusLabels, statusHelp, verificationHelp } from '../../lib/labels'
import { Button } from '../ui/Button'
import { Dialog } from '../ui/Dialog'
import './LabelsLegend.css'

const ROWS: [string, string][] = [
  ['AI generated', verificationHelp.ai_generated],
  ['Source verified', verificationHelp.source_verified],
  ['Human verified', verificationHelp.human_verified],
  ['Disputed', verificationHelp.disputed],
  ['High, Medium, Low confidence', confidenceHelp.rated],
  [confidenceLabels.unknown, confidenceHelp.notAssessed],
  [claimStatusLabels.active, statusHelp.current],
  [hypothesisStatusLabels.open, statusHelp.open],
  [hypothesisStatusLabels.supported, statusHelp.supported],
  [hypothesisStatusLabels.disproven, statusHelp.disproven],
  ['Superseded', verificationHelp.superseded],
  ['Rejected', statusHelp.rejected],
]

export function LabelsLegend({ onClose }: { onClose: () => void }) {
  return (
    <Dialog title="What the labels mean" size="confirm" onClose={onClose} footer={<Button onClick={onClose}>Close</Button>}>
      <dl className="labels-legend">
        {ROWS.map(([label, help]) => (
          <div key={label} className="labels-legend-row">
            <dt>{label}</dt>
            <dd>{help}</dd>
          </div>
        ))}
      </dl>
      <p className="labels-legend-note">Only a person can mark a finding Human verified.</p>
    </Dialog>
  )
}
