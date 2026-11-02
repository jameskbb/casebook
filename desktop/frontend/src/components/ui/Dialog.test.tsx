import { useState } from 'react'
import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { Dialog } from './Dialog'
import { TextField } from './TextField'

function Harness({ onSubmit = () => {} }: { onSubmit?: () => void }) {
  const [open, setOpen] = useState(false)
  const [value, setValue] = useState('')
  return (
    <>
      <button type="button" onClick={() => setOpen(true)}>
        Open form
      </button>
      {open && (
        <Dialog
          title="Add question"
          onClose={() => {
            setOpen(false)
            setValue('')
          }}
          onSubmit={onSubmit}
          submitLabel="Add question"
          dirty={value !== ''}
        >
          <TextField label="Question" value={value} onChange={setValue} />
          <TextField label="Note" value="" onChange={() => {}} />
        </Dialog>
      )}
    </>
  )
}

describe('Dialog', () => {
  it('focuses the first field, keeps Tab inside and returns focus to the opener', async () => {
    const user = userEvent.setup()
    render(<Harness />)
    const opener = screen.getByRole('button', { name: 'Open form' })
    await user.click(opener)
    const dialog = screen.getByRole('dialog', { name: 'Add question' })
    expect(within(dialog).getByLabelText('Question')).toHaveFocus()

    const submit = within(dialog).getByRole('button', { name: 'Add question' })
    submit.focus()
    await user.tab()
    expect(dialog.contains(document.activeElement)).toBe(true)
    await user.tab({ shift: true })
    expect(submit).toHaveFocus()

    await user.keyboard('{Escape}')
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(opener).toHaveFocus()
  })

  it('asks before discarding a draft', async () => {
    const user = userEvent.setup()
    render(<Harness />)
    const opener = screen.getByRole('button', { name: 'Open form' })
    await user.click(opener)
    await user.type(screen.getByLabelText('Question'), 'Why?')

    await user.keyboard('{Escape}')
    const confirm = screen.getByRole('dialog', { name: 'Discard this draft?' })
    expect(within(confirm).getByText('What you typed has not been saved.')).toBeInTheDocument()
    expect(within(confirm).getByRole('button', { name: 'Keep editing' })).toHaveFocus()

    await user.click(within(confirm).getByRole('button', { name: 'Keep editing' }))
    expect(screen.queryByRole('dialog', { name: 'Discard this draft?' })).not.toBeInTheDocument()
    expect(screen.getByLabelText('Question')).toHaveValue('Why?')

    await user.click(screen.getByRole('button', { name: 'Close' }))
    await user.click(screen.getByRole('button', { name: 'Discard' }))
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(opener).toHaveFocus()
  })

  it('submits with Ctrl+Enter', async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn()
    render(<Harness onSubmit={onSubmit} />)
    await user.click(screen.getByRole('button', { name: 'Open form' }))
    await user.type(screen.getByLabelText('Question'), 'Why?')
    await user.keyboard('{Control>}{Enter}{/Control}')
    expect(onSubmit).toHaveBeenCalledTimes(1)
  })
})
