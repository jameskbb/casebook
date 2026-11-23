// What the labels mean: one sentence per label a finding or source shows in
// the reading column, so a person can defend a line without knowing the
// engine's words for it. Current follows the checking rows so a reader can
// tell "who checked it" apart from "still stands on it".
import { claimStatusLabels, hypothesisStatusLabels, statusHelp, verificationHelp, verificationLabels } from '../../lib/labels'
import { Button } from '../ui/Button'
import { Dialog } from '../ui/Dialog'
import './LabelsLegend.css'

const ROWS: [string, string][] = [
  [verificationLabels.ai_generated, verificationHelp.ai_generated],
  [verificationLabels.source_verified, verificationHelp.source_verified],
  [verificationLabels.human_verified, verificationHelp.human_verified],
  [verificationLabels.disputed, verificationHelp.disputed],
  [claimStatusLabels.active, statusHelp.current],
  [hypothesisStatusLabels.open, statusHelp.open],
  [hypothesisStatusLabels.supported, statusHelp.supported],
  [hypothesisStatusLabels.disproven, statusHelp.ruledOut],
  [verificationLabels.superseded, verificationHelp.superseded],
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
      <p className="labels-legend-note">Only a person can mark a finding "{verificationLabels.human_verified}".</p>
    </Dialog>
  )
}
