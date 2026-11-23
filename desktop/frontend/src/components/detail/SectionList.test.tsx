import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Quote } from 'lucide-react'
import { describe, expect, it } from 'vitest'
import { Collapsed, SectionList, type SectionListProps } from './SectionList'

type Item = { ref: string; text: string }

function renderList(over: Partial<SectionListProps<Item>> = {}) {
  const props: SectionListProps<Item> = {
    id: 'claims',
    icon: Quote,
    kind: 'claim',
    heading: 'What backs this up',
    items: [],
    itemKey: (i) => i.ref,
    renderItem: (i) => <p>{i.text}</p>,
    ...over,
  }
  return render(<SectionList {...props} />)
}

const items = (n: number, prefix = 'C'): Item[] =>
  Array.from({ length: n }, (_, i) => ({ ref: `${prefix}-00${i + 1}`, text: `${prefix} text ${i + 1}` }))

describe('SectionList', () => {
  it('draws nothing at all when there is nothing in it', () => {
    const { container } = renderList()
    expect(container).toBeEmptyDOMElement()
  })

  it('renders the heading as an h2 over its rows, with no button of its own', () => {
    renderList({ items: items(2) })
    const section = screen.getByRole('region', { name: 'What backs this up' })
    expect(within(section).getByRole('heading', { level: 2, name: 'What backs this up' })).toBeInTheDocument()
    expect(within(section).getAllByRole('listitem')).toHaveLength(2)
    expect(within(section).queryByRole('button')).toBeNull()
  })

  it('keeps the section for rows that are only behind its Show line', async () => {
    const user = userEvent.setup()
    renderList({ heading: 'Still to find out', collapsed: items(2, 'X'), collapsedNoun: ['answered', 'answered'] })
    const section = screen.getByRole('region', { name: 'Still to find out' })
    expect(within(section).queryByRole('listitem')).toBeNull()
    await user.click(within(section).getByRole('button', { name: 'Show 2 answered' }))
    expect(within(section).getAllByRole('listitem')).toHaveLength(2)
  })
})

describe('Collapsed', () => {
  const row = { itemKey: (i: Item) => i.ref, renderItem: (i: Item) => <p>{i.text}</p> }

  it('draws nothing when there are none', () => {
    const { container } = render(<Collapsed items={[]} noun={['ruled out finding', 'ruled out findings']} {...row} />)
    expect(container).toBeEmptyDOMElement()
  })

  it('hides its rows behind one line with count-aware words, and opens and closes', async () => {
    const user = userEvent.setup()
    const { rerender } = render(<Collapsed items={items(1, 'X')} noun={['ruled out finding', 'ruled out findings']} {...row} />)
    const one = screen.getByRole('button', { name: 'Show 1 ruled out finding' })
    expect(one).toHaveAttribute('aria-expanded', 'false')
    expect(screen.queryByText('X text 1')).not.toBeInTheDocument()
    await user.click(one)
    expect(screen.getByRole('button', { name: 'Hide 1 ruled out finding' })).toHaveAttribute('aria-expanded', 'true')
    expect(screen.getByText('X text 1')).toBeInTheDocument()
    rerender(<Collapsed items={items(2, 'X')} noun={['ruled out finding', 'ruled out findings']} {...row} />)
    await user.click(screen.getByRole('button', { name: 'Hide 2 ruled out findings' }))
    expect(screen.getByRole('button', { name: 'Show 2 ruled out findings' })).toBeInTheDocument()
    expect(screen.queryByText('X text 1')).not.toBeInTheDocument()
  })
})
