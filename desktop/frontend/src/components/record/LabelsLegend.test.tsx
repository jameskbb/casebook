import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { LabelsLegend } from './LabelsLegend'

describe('LabelsLegend', () => {
  it('lists every label with its meaning and closes', async () => {
    const onClose = vi.fn()
    render(<LabelsLegend onClose={onClose} />)
    expect(screen.getByRole('dialog', { name: 'What the labels mean' })).toBeInTheDocument()
    for (const label of ['AI generated', 'Source verified', 'Human verified', 'Disputed', 'High, Medium, Low confidence', 'Not assessed', 'Current', 'Open', 'Supported', 'Disproven', 'Superseded', 'Rejected']) {
      expect(screen.getByText(label)).toBeInTheDocument()
    }
    // Review round 2 added Current (the claim status a person sees on the
    // Review findings screen) and the confidence words, so the legend now
    // separates "who checked it" from "how sure".
    expect(screen.getByText('A claim the case stands on now.')).toBeInTheDocument()
    expect(screen.getByText(/^How sure the author was when writing it\. A judgment, not a check/)).toBeInTheDocument()
    expect(screen.getByText('No confidence recorded.')).toBeInTheDocument()
    expect(screen.getByText('Marked as checked against its source, usually by your AI. Only Human verified means a person vouched for it.')).toBeInTheDocument()
    const terms = screen.getAllByRole('term').map((t) => t.textContent)
    expect(terms.slice(0, 7)).toEqual(['AI generated', 'Source verified', 'Human verified', 'Disputed', 'High, Medium, Low confidence', 'Not assessed', 'Current'])
    // The dialog's own X is also named Close; the footer button comes last.
    await userEvent.click(screen.getAllByRole('button', { name: 'Close' }).at(-1)!)
    expect(onClose).toHaveBeenCalled()
  })
})
