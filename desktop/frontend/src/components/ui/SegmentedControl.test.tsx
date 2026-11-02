import { useState } from 'react'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { SegmentedControl } from './SegmentedControl'

type Level = 'unknown' | 'low' | 'medium' | 'high'
const options: { value: Level; label: string }[] = [
  { value: 'unknown', label: 'Not assessed' },
  { value: 'low', label: 'Low' },
  { value: 'medium', label: 'Medium' },
  { value: 'high', label: 'High' },
]

function Harness({ onChange = () => {}, initial = 'low' as Level }: { onChange?: (v: Level) => void; initial?: Level }) {
  const [value, setValue] = useState<Level>(initial)
  return (
    <SegmentedControl<Level>
      label="Confidence"
      options={options}
      value={value}
      onChange={(v) => {
        setValue(v)
        onChange(v)
      }}
    />
  )
}

describe('SegmentedControl', () => {
  it('is a labelled radio group with one checked option', () => {
    render(<Harness />)
    const group = screen.getByRole('radiogroup', { name: 'Confidence' })
    expect(group).toBeInTheDocument()
    expect(screen.getByRole('radio', { name: 'Low' })).toHaveAttribute('aria-checked', 'true')
    expect(screen.getByRole('radio', { name: 'High' })).toHaveAttribute('aria-checked', 'false')
    // Roving tabindex: only the checked option is a Tab stop.
    expect(screen.getByRole('radio', { name: 'Low' })).toHaveAttribute('tabindex', '0')
    expect(screen.getByRole('radio', { name: 'High' })).toHaveAttribute('tabindex', '-1')
  })

  it('moves and selects with Left and Right, wrapping at the ends', async () => {
    const onChange = vi.fn()
    const user = userEvent.setup()
    render(<Harness onChange={onChange} />)
    screen.getByRole('radio', { name: 'Low' }).focus()
    await user.keyboard('{ArrowRight}')
    expect(screen.getByRole('radio', { name: 'Medium' })).toHaveAttribute('aria-checked', 'true')
    expect(screen.getByRole('radio', { name: 'Medium' })).toHaveFocus()
    await user.keyboard('{ArrowLeft}{ArrowLeft}{ArrowLeft}')
    expect(screen.getByRole('radio', { name: 'High' })).toHaveAttribute('aria-checked', 'true')
    expect(onChange).toHaveBeenLastCalledWith('high')
  })

  it('selects on click', async () => {
    const user = userEvent.setup()
    render(<Harness />)
    await user.click(screen.getByRole('radio', { name: 'Not assessed' }))
    expect(screen.getByRole('radio', { name: 'Not assessed' })).toHaveAttribute('aria-checked', 'true')
  })
})
