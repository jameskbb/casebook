import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Quote } from 'lucide-react'
import { describe, expect, it, vi } from 'vitest'
import { SectionList, type SectionListProps } from './SectionList'

type Item = { ref: string; text: string }

function renderList(over: Partial<SectionListProps<Item>> = {}) {
  const props: SectionListProps<Item> = {
    id: 'claims',
    icon: Quote,
    kind: 'claim',
    heading: 'What we believe',
    addLabel: 'Add claim',
    emptyText: 'No claims yet. Add one when the evidence supports a clear statement.',
    items: [],
    ruledOut: [],
    ruledOutNoun: ['superseded or rejected claim', 'superseded or rejected claims'],
    itemKey: (i) => i.ref,
    renderItem: (i) => <p>{i.text}</p>,
    ...over,
  }
  return render(<SectionList {...props} />)
}

const items = (n: number, prefix = 'C'): Item[] =>
  Array.from({ length: n }, (_, i) => ({ ref: `${prefix}-00${i + 1}`, text: `${prefix} text ${i + 1}` }))

describe('SectionList', () => {
  it('renders the heading as an h2 and the empty copy with no add button by default', () => {
    renderList()
    expect(screen.getByRole('heading', { level: 2, name: 'What we believe' })).toBeInTheDocument()
    expect(
      screen.getByText('No claims yet. Add one when the evidence supports a clear statement.'),
    ).toBeInTheDocument()
    expect(screen.queryByRole('button')).not.toBeInTheDocument()
  })

  it('puts the add action on the empty line when onAdd is passed', async () => {
    const onAdd = vi.fn()
    const user = userEvent.setup()
    renderList({ onAdd })
    const buttons = screen.getAllByRole('button', { name: 'Add claim' })
    expect(buttons).toHaveLength(1)
    await user.click(buttons[0])
    expect(onAdd).toHaveBeenCalledTimes(1)
  })

  it('shows the heading add action once rows exist', () => {
    renderList({ onAdd: () => {}, items: items(2) })
    const section = screen.getByRole('region', { name: 'What we believe' })
    expect(within(section).getAllByRole('button', { name: 'Add claim' })).toHaveLength(1)
    expect(within(section).getAllByRole('listitem')).toHaveLength(2)
  })

  it('collapses one ruled-out item under a singular disclosure', async () => {
    const user = userEvent.setup()
    renderList({ items: items(1), ruledOut: items(1, 'X') })
    expect(screen.queryByText('X text 1')).not.toBeInTheDocument()
    const toggle = screen.getByRole('button', { name: 'Show 1 superseded or rejected claim' })
    expect(toggle).toHaveAttribute('aria-expanded', 'false')
    await user.click(toggle)
    expect(screen.getByText('X text 1')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Hide 1 superseded or rejected claim' })).toHaveAttribute(
      'aria-expanded',
      'true',
    )
  })

  it('uses the plural disclosure for many ruled-out items', () => {
    renderList({ items: items(1), ruledOut: items(3, 'X') })
    expect(screen.getByRole('button', { name: 'Show 3 superseded or rejected claims' })).toBeInTheDocument()
  })

  it('shows the empty copy alongside the disclosure when asked', () => {
    renderList({
      emptyText: 'Nothing is open. Add a question when something still needs an answer.',
      emptyWithRuledOut: true,
      ruledOutNoun: ['resolved question', 'resolved questions'],
      ruledOut: items(2, 'Q'),
    })
    expect(
      screen.getByText('Nothing is open. Add a question when something still needs an answer.'),
    ).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Show 2 resolved questions' })).toBeInTheDocument()
  })

  it('renders a lead above the rows and does not call the section empty', () => {
    renderList({
      emptyText: 'Nothing is open. Add a question when something still needs an answer.',
      emptyWithRuledOut: true,
      lead: <p>Explanations still open</p>,
      items: [],
    })
    expect(screen.getByText('Explanations still open')).toBeInTheDocument()
    expect(screen.queryByText(/Nothing is open/)).not.toBeInTheDocument()
  })

  it('hides the empty copy when only ruled-out items exist by default', () => {
    renderList({ ruledOut: items(1, 'X') })
    expect(screen.queryByText(/No claims yet/)).not.toBeInTheDocument()
  })
})
