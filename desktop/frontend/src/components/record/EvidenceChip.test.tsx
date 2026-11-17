import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { EvidenceChip, shortLabel } from './EvidenceChip'

describe('shortLabel', () => {
  it('keeps a short title whole', () => {
    expect(shortLabel('Deal desk CSV')).toBe('Deal desk CSV')
  })
  it('cuts at a word boundary and ends with three periods', () => {
    expect(shortLabel('Three accounts moved to white-glove onboarding in July carry $1.4M of the $1.9M')).toBe('Three accounts moved to...')
  })
  it('cuts a single long word hard', () => {
    expect(shortLabel('a'.repeat(60))).toBe('a'.repeat(25) + '...')
  })
})

describe('EvidenceChip', () => {
  it('shows the title first and the ref second, and keeps the full title in the tooltip', () => {
    render(<EvidenceChip target="E-008" relation="supports" linkKind="claim" text="July 6 playbook update: white-glove onboarding included free" />)
    const chip = screen.getByTitle('E-008 supports this: July 6 playbook update: white-glove onboarding included free')
    expect(chip.textContent).toBe('July 6 playbook update:...E-008')
    expect(chip.querySelector('.chip-ref')?.textContent).toBe('E-008')
  })
})
