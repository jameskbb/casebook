import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { LabelsLegend } from './LabelsLegend'

describe('LabelsLegend', () => {
  it('lists every label with its meaning and closes', async () => {
    const onClose = vi.fn()
    render(<LabelsLegend onClose={onClose} />)
    expect(screen.getByRole('dialog', { name: 'What the labels mean' })).toBeInTheDocument()
    const terms = screen.getAllByRole('term').map((t) => t.textContent)
    // Who checked it first, then where the finding stands. No confidence words.
    expect(terms).toEqual(['Not checked', 'AI checked the source', 'You checked this', 'Sources disagree', 'Current', 'Not tested yet', 'Supported', 'Ruled out', 'Replaced'])
    expect(screen.getByText('A finding the answer stands on now.')).toBeInTheDocument()
    expect(screen.getByText('Your AI compared this with its source. You have not checked it yourself.')).toBeInTheDocument()
    expect(screen.queryByText(/confidence/i)).toBeNull()
    // The dialog's own X is also named Close; the footer button comes last.
    await userEvent.click(screen.getAllByRole('button', { name: 'Close' }).at(-1)!)
    expect(onClose).toHaveBeenCalled()
  })
})
