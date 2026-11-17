import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import type { Confidence } from '../../api/types'
import { ConfidenceMeter } from './ConfidenceMeter'

describe('ConfidenceMeter', () => {
  const cases: [Confidence, number, string][] = [
    ['low', 1, 'Low'],
    ['medium', 2, 'Medium'],
    ['high', 3, 'High'],
    ['unknown', 0, 'Not assessed'],
  ]
  for (const [level, filled, label] of cases) {
    it(`fills ${filled} segments for ${level}`, () => {
      const { container } = render(<ConfidenceMeter confidence={level} />)
      expect(container.querySelectorAll('.confidence-seg')).toHaveLength(3)
      expect(container.querySelectorAll('.confidence-seg.is-filled')).toHaveLength(filled)
      expect(screen.getByRole('img', { name: `Confidence: ${label}` })).toBeInTheDocument()
      expect(screen.getByText(label)).toBeInTheDocument()
      expect(container.textContent).not.toContain('%')
    })
  }

  it('uses the long label when asked', () => {
    render(<ConfidenceMeter confidence="high" size="lg" label="long" />)
    expect(screen.getByText('High confidence')).toBeInTheDocument()
    expect(screen.getByRole('img', { name: 'Confidence: High' })).toBeInTheDocument()
  })

  it('shows Not assessed in the long form too', () => {
    render(<ConfidenceMeter confidence="unknown" label="long" />)
    expect(screen.getByText('Not assessed')).toBeInTheDocument()
  })
})
