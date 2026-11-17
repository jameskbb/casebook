import { render } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { BreakablePath } from './BreakablePath'

describe('BreakablePath', () => {
  it('offers a line break after every slash and keeps the text', () => {
    const { container } = render(
      <p>
        <BreakablePath text="internal/checkout/totals_test.go:41" />
      </p>,
    )
    const p = container.querySelector('p')!
    expect(p.textContent).toBe('internal/checkout/totals_test.go:41')
    expect(p.querySelectorAll('wbr')).toHaveLength(2)
    expect(p.innerHTML).toBe('internal/<wbr>checkout/<wbr>totals_test.go:41')
  })

  it('treats backslashes the same and leaves a plain name alone', () => {
    const { container } = render(
      <p>
        <BreakablePath text={'C:\\Users\\casebook.db'} />
        <span>
          <BreakablePath text="casebook" />
        </span>
      </p>,
    )
    expect(container.querySelectorAll('p > wbr')).toHaveLength(2)
    expect(container.querySelector('span')!.innerHTML).toBe('casebook')
  })
})
