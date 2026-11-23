import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { LEGEND } from '../../lib/labels'
import { LabelsLegend } from './LabelsLegend'

describe('LabelsLegend', () => {
  it('lists every label with its meaning, in groups, and closes', async () => {
    const onClose = vi.fn()
    render(<LabelsLegend onClose={onClose} />)
    expect(screen.getByRole('dialog', { name: 'What the labels mean' })).toBeInTheDocument()
    const rows = LEGEND.flatMap((g) => g.rows)
    // lib/labels.test.ts checks that LEGEND holds every label the page can show.
    expect(screen.getAllByRole('term').map((t) => t.textContent)).toEqual(rows.map((r) => r.label))
    expect(screen.getAllByRole('definition')).toHaveLength(rows.length)
    expect(screen.getAllByRole('heading', { level: 3 }).map((h) => h.textContent)).toEqual(['On a finding', 'On a source', 'Elsewhere'])
    for (const label of ['Not tested yet', 'Ruled out', 'Later', 'Replaced', 'AI checked the source', 'You decided this holds', 'Backs it']) expect(screen.getByText(label)).toBeInTheDocument()
    expect(screen.queryByText(/confidence|looks right/i)).toBeNull()
    // The dialog's own X is also named Close; the footer button comes last.
    await userEvent.click(screen.getAllByRole('button', { name: 'Close' }).at(-1)!)
    expect(onClose).toHaveBeenCalled()
  })
})
