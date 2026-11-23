import { describe, expect, it } from 'vitest'
import { claimStatusLabels, hypothesisStatusLabels, relationLabels, verificationHelp, verificationLabels } from './labels'

describe('labels', () => {
  it('has no word for a finding that stands, and never says Current', () => {
    expect(Object.keys(claimStatusLabels)).not.toContain('active')
    const shown = [claimStatusLabels, hypothesisStatusLabels, verificationLabels].flatMap((l) => Object.values(l))
    expect(shown).not.toContain('Current')
  })
  it('says a source backs a finding or goes against it, and a tested finding looks right so far', () => {
    expect(relationLabels).toEqual({ supports: 'Backs it up', contradicts: 'Goes against it' })
    expect(hypothesisStatusLabels.supported).toBe('Looks right so far')
  })
  it('has a one-sentence explanation for every check label', () => {
    for (const v of Object.keys(verificationLabels) as (keyof typeof verificationLabels)[]) {
      expect(verificationHelp[v], v).toMatch(/^[A-Z].*\.$/)
      expect(verificationHelp[v].length, v).toBeLessThan(120)
    }
    expect(Object.keys(verificationHelp).sort()).toEqual(Object.keys(verificationLabels).sort())
  })
})
