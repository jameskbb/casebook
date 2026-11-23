// Nothing is open at launch (the main pane shows Home), so a test that needs
// the question page opens a question from the sidebar list, as a person would.
import { fireEvent, screen, waitFor } from '@testing-library/react'

function row(ref: string): HTMLElement | null {
  return document.querySelector<HTMLElement>(`nav [data-ref="${ref}"]`)
}

/** Clicks the question's row now; for tests on fake timers, after the list has loaded. */
export function openQuestionNow(ref = 'INV-0001'): void {
  const el = row(ref)
  if (!el) throw new Error(`no row for ${ref} in the question list`)
  fireEvent.click(el)
}

/** Waits for the question's row, then clicks it. */
export async function openQuestion(ref = 'INV-0001'): Promise<void> {
  await screen.findByRole('navigation', { name: 'Questions' })
  await waitFor(() => {
    if (!row(ref)) throw new Error(`no row for ${ref} in the question list`)
  })
  openQuestionNow(ref)
}

/**
 * The first control on the question page that opens this source: where it
 * sits under a finding, or its own row under Other sources.
 */
export async function findSourceButton(ref: string): Promise<HTMLElement> {
  const find = () => document.querySelector<HTMLElement>(`button[data-ref="${ref}"], [data-ref="${ref}"] > .row-hit`)
  await waitFor(() => {
    if (!find()) throw new Error(`nothing on the page opens ${ref}`)
  })
  return find()!
}
