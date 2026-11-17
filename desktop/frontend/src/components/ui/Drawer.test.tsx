import { useState } from 'react'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { Quote } from 'lucide-react'
import { Drawer } from './Drawer'

describe('Drawer', () => {
  it('is a modal dialog labelled by its heading, with focus on the heading', () => {
    render(
      <Drawer heading="Claim C-001" icon={Quote} kind="claim" onClose={() => {}} contentKey="C-001">
        <button type="button">Inside</button>
      </Drawer>,
    )
    const drawer = screen.getByRole('dialog', { name: 'Claim C-001' })
    expect(drawer).toHaveAttribute('aria-modal', 'true')
    expect(screen.getByRole('heading', { level: 2, name: 'Claim C-001' })).toHaveFocus()
    expect(screen.queryByRole('button', { name: /Back to/ })).toBeNull()
  })

  it('closes on Esc, on the close button and on the scrim', async () => {
    const user = userEvent.setup()
    const onClose = vi.fn()
    const { container } = render(
      <Drawer heading="Claim C-001" onClose={onClose} contentKey="C-001">
        <p>Body</p>
      </Drawer>,
    )
    await user.keyboard('{Escape}')
    expect(onClose).toHaveBeenCalledTimes(1)
    await user.click(screen.getByRole('button', { name: 'Close' }))
    expect(onClose).toHaveBeenCalledTimes(2)
    const scrim = document.querySelector('.drawer-scrim') as HTMLElement
    expect(container).toBeTruthy()
    await user.pointer({ keys: '[MouseLeft>]', target: scrim })
    expect(onClose).toHaveBeenCalledTimes(3)
  })

  it('shows Back to the previous ref and moves focus to the heading when the content changes', async () => {
    const user = userEvent.setup()
    function Harness() {
      const [top, setTop] = useState('C-001')
      return (
        <Drawer
          heading={top === 'C-001' ? 'Claim C-001' : 'Evidence E-001'}
          onClose={() => {}}
          contentKey={top}
          back={top === 'E-001' ? { ref: 'C-001', onClick: () => setTop('C-001') } : undefined}
        >
          <button type="button" onClick={() => setTop('E-001')}>
            Open E-001
          </button>
        </Drawer>
      )
    }
    render(<Harness />)
    await user.click(screen.getByRole('button', { name: 'Open E-001' }))
    expect(screen.getByRole('heading', { name: 'Evidence E-001' })).toHaveFocus()
    await user.click(screen.getByRole('button', { name: 'Back to C-001' }))
    expect(screen.getByRole('heading', { name: 'Claim C-001' })).toHaveFocus()
  })

  it('keeps Tab inside the drawer', async () => {
    const user = userEvent.setup()
    render(
      <>
        <button type="button">Outside</button>
        <Drawer heading="Evidence E-001" onClose={() => {}} contentKey="E-001">
          <button type="button">Last</button>
        </Drawer>
      </>,
    )
    await user.tab()
    expect(screen.getByRole('button', { name: 'Close' })).toHaveFocus()
    await user.tab()
    expect(screen.getByRole('button', { name: 'Last' })).toHaveFocus()
    await user.tab()
    expect(screen.getByRole('button', { name: 'Close' })).toHaveFocus()
  })
})
