import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { verificationHelp } from '../../lib/labels'
import { LabelsLegend } from './LabelsLegend'

describe('LabelsLegend', () => {
  it('lists the four check labels with their meaning and closes', async () => {
    const onClose = vi.fn()
    render(<LabelsLegend onClose={onClose} />)
    expect(screen.getByRole('dialog', { name: 'What the labels mean' })).toBeInTheDocument()
    const terms = screen.getAllByRole('term').map((t) => t.textContent)
    // Exactly four rows: who checked it. No state words, no confidence words.
    expect(terms).toEqual(['Not checked', 'AI checked the source', 'You checked this', 'Sources disagree'])
    expect(terms).not.toContain('Current')
    expect(screen.getAllByRole('definition')).toHaveLength(4)
    expect(screen.getByText(verificationHelp.source_verified)).toBeInTheDocument()
    expect(screen.queryByText(/confidence/i)).toBeNull()
    // The dialog's own X is also named Close; the footer button comes last.
    await userEvent.click(screen.getAllByRole('button', { name: 'Close' }).at(-1)!)
    expect(onClose).toHaveBeenCalled()
  })
})
