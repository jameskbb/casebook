import { describe, expect, it } from 'vitest'
import { claimStatusLabels, verificationHelp, verificationLabels, verificationOptions } from './labels'

describe('labels', () => {
  it('calls an active claim Current, so it never collides with the AI toggle', () => {
    expect(claimStatusLabels.active).toBe('Current')
  })
  it('has a one-sentence explanation for every verification value', () => {
    for (const v of verificationOptions) {
      expect(verificationHelp[v], v).toMatch(/^[A-Z].*\.$/)
      expect(verificationHelp[v].length, v).toBeLessThan(120)
    }
    expect(Object.keys(verificationHelp).sort()).toEqual(Object.keys(verificationLabels).sort())
  })
})
