// Confidence meter (D-15): three segments, Low fills 1, Medium 2, High 3;
// Unknown shows three outlines and "Not assessed". Never a number.
import type { Confidence } from '../../api/types'
import { confidenceLabels, confidenceLongLabels } from '../../lib/labels'
import './ConfidenceMeter.css'

const FILLED: Record<Confidence, number> = { unknown: 0, low: 1, medium: 2, high: 3 }

export interface ConfidenceMeterProps {
  confidence: Confidence
  /** sm is 14x8 segments (rows); lg is 20x10, filled with the accent (conclusion). */
  size?: 'sm' | 'lg'
  /** Text next to the meter: short ("High"), long ("High confidence") or none. */
  label?: 'short' | 'long' | 'none'
  className?: string
}

export function ConfidenceMeter({ confidence, size = 'sm', label = 'short', className }: ConfidenceMeterProps) {
  const filled = FILLED[confidence] ?? 0
  const short = confidenceLabels[confidence] ?? confidenceLabels.unknown
  const text = label === 'long' ? confidenceLongLabels[confidence] : short
  return (
    <span className={['confidence', `confidence-${size}`, className ?? ''].filter(Boolean).join(' ')}>
      <span className="confidence-meter" role="img" aria-label={`Confidence: ${short}`} data-filled={filled}>
        {[0, 1, 2].map((i) => (
          <span key={i} className={i < filled ? 'confidence-seg is-filled' : 'confidence-seg'} />
        ))}
      </span>
      {label !== 'none' && (
        <span className="confidence-label" aria-hidden="true">
          {text}
        </span>
      )}
    </span>
  )
}
