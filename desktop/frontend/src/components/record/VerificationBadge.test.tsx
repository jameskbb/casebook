import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { verificationHelp } from '../../lib/labels'
import { VerificationBadge } from './VerificationBadge'

describe('VerificationBadge', () => {
  it('explains the label in a tooltip', () => {
    render(<VerificationBadge verification="ai_generated" />)
    expect(screen.getByText('AI generated')).toHaveAttribute('title', verificationHelp.ai_generated)
  })
  it('explains Disputed too', () => {
    render(<VerificationBadge verification="disputed" />)
    expect(screen.getByText('Disputed')).toHaveAttribute('title', verificationHelp.disputed)
  })
  it('explains superseded, the value the engine returns for a replaced record', () => {
    render(<VerificationBadge verification="superseded" />)
    expect(screen.getByText('Superseded')).toHaveAttribute('title', verificationHelp.superseded)
  })
})
