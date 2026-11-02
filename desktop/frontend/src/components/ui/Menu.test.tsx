import { useState } from 'react'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Archive, Download, Pencil } from 'lucide-react'
import { describe, expect, it, vi } from 'vitest'
import { Dialog } from './Dialog'
import { Menu, type MenuItem } from './Menu'

function items(onSelect: (name: string) => void): MenuItem[] {
  return [
    { label: 'Edit title and question...', icon: Pencil, onSelect: () => onSelect('edit') },
    { label: 'Export as JSON...', icon: Download, onSelect: () => onSelect('export'), disabled: true },
    { label: 'Archive case', icon: Archive, onSelect: () => onSelect('archive') },
  ]
}

function renderMenu(onSelect = vi.fn()) {
  render(
    <Menu
      label="More actions"
      items={items(onSelect)}
      trigger={(p) => (
        <button type="button" {...p}>
          More
        </button>
      )}
    />,
  )
  return onSelect
}

describe('Menu', () => {
  it('opens on click with role menu and focuses the first enabled item', async () => {
    const user = userEvent.setup()
    renderMenu()
    const button = screen.getByRole('button', { name: 'More' })
    expect(button).toHaveAttribute('aria-haspopup', 'menu')
    expect(button).toHaveAttribute('aria-expanded', 'false')
    await user.click(button)
    expect(screen.getByRole('menu', { name: 'More actions' })).toBeInTheDocument()
    expect(button).toHaveAttribute('aria-expanded', 'true')
    expect(screen.getByRole('menuitem', { name: 'Edit title and question...' })).toHaveFocus()
    expect(screen.getByRole('menuitem', { name: 'Export as JSON...' })).toHaveAttribute('aria-disabled', 'true')
  })

  it('opens with Enter, moves with Up and Down past disabled items and chooses with Enter', async () => {
    const user = userEvent.setup()
    const onSelect = renderMenu()
    screen.getByRole('button', { name: 'More' }).focus()
    await user.keyboard('{Enter}')
    expect(screen.getByRole('menuitem', { name: 'Edit title and question...' })).toHaveFocus()
    await user.keyboard('{ArrowDown}')
    expect(screen.getByRole('menuitem', { name: 'Archive case' })).toHaveFocus()
    await user.keyboard('{ArrowDown}')
    expect(screen.getByRole('menuitem', { name: 'Edit title and question...' })).toHaveFocus()
    await user.keyboard('{ArrowUp}')
    expect(screen.getByRole('menuitem', { name: 'Archive case' })).toHaveFocus()
    await user.keyboard('{Enter}')
    expect(onSelect).toHaveBeenCalledWith('archive')
    expect(screen.queryByRole('menu')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'More' })).toHaveFocus()
  })

  it('closes with Esc and returns focus to its button', async () => {
    const user = userEvent.setup()
    const onSelect = renderMenu()
    await user.click(screen.getByRole('button', { name: 'More' }))
    await user.keyboard('{Escape}')
    expect(screen.queryByRole('menu')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'More' })).toHaveFocus()
    expect(onSelect).not.toHaveBeenCalled()
  })

  it('closes only itself when Esc is pressed inside a dialog', async () => {
    const user = userEvent.setup()
    function InDialog() {
      const [open, setOpen] = useState(true)
      return open ? (
        <Dialog title="Edit" onClose={() => setOpen(false)}>
          <Menu
            label="Options"
            items={items(() => {})}
            trigger={(p) => (
              <button type="button" {...p}>
                Options
              </button>
            )}
          />
        </Dialog>
      ) : null
    }
    render(<InDialog />)
    await user.click(screen.getByRole('button', { name: 'Options' }))
    expect(screen.getByRole('menu')).toBeInTheDocument()
    await user.keyboard('{Escape}')
    expect(screen.queryByRole('menu')).not.toBeInTheDocument()
    expect(screen.getByRole('dialog', { name: 'Edit' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Options' })).toHaveFocus()
  })

  it('closes on a click outside', async () => {
    const user = userEvent.setup()
    render(
      <>
        <Menu
          label="More actions"
          items={items(() => {})}
          trigger={(p) => (
            <button type="button" {...p}>
              More
            </button>
          )}
        />
        <p>Outside</p>
      </>,
    )
    await user.click(screen.getByRole('button', { name: 'More' }))
    await user.click(screen.getByText('Outside'))
    expect(screen.queryByRole('menu')).not.toBeInTheDocument()
  })
})
