// What the labels mean: every label the page can show, in the words used,
// one line each (lib/labels LEGEND).
import { LEGEND } from '../../lib/labels'
import { Button } from '../ui/Button'
import { Dialog } from '../ui/Dialog'
import './LabelsLegend.css'

export function LabelsLegend({ onClose }: { onClose: () => void }) {
  return (
    <Dialog title="What the labels mean" onClose={onClose} footer={<Button onClick={onClose}>Close</Button>}>
      {LEGEND.map((group) => (
        <section key={group.heading} className="labels-legend-group">
          <h3 className="labels-legend-heading">{group.heading}</h3>
          <dl className="labels-legend">
            {group.rows.map((row) => (
              <div key={row.label} className="labels-legend-row">
                <dt>{row.label}</dt>
                <dd>{row.meaning}</dd>
              </div>
            ))}
          </dl>
        </section>
      ))}
    </Dialog>
  )
}
