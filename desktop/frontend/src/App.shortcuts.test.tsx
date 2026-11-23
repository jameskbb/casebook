import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { Api } from './api/api'
import { mockApi } from './test/mockApi'
import { openQuestion } from './test/openQuestion'
import App from './App'

const mocks = vi.hoisted(() => ({ api: {} as Api }))
vi.mock('./api/api', () => ({ api: mocks.api }))

const realPlatform = Object.getOwnPropertyDescriptor(window.navigator, 'platform')

function setPlatform(value: string) {
  Object.defineProperty(window.navigator, 'platform', { value, configurable: true })
}

async function renderApp() {
  const user = userEvent.setup()
  render(<App />)
  await openQuestion()
  await screen.findByRole('heading', { level: 1, name: 'Why did calls go up 30% in October when sales stayed the same?' })
  const search = screen.getByRole('searchbox', { name: 'Search answers and sources' })
  return { user, search }
}

describe('global shortcuts', () => {
  beforeEach(() => {
    Object.assign(mocks.api, mockApi())
    setPlatform('Win32')
  })
  afterEach(() => {
    if (realPlatform) Object.defineProperty(window.navigator, 'platform', realPlatform)
    else delete (window.navigator as { platform?: string }).platform
  })

  it('focuses search with / and selects its text', async () => {
    const { user, search } = await renderApp()
    await user.type(search, 'abc')
    search.blur()
    expect(search).not.toHaveFocus()
    await user.keyboard('/')
    expect(search).toHaveFocus()
    expect(search).toHaveValue('abc')
    expect((search as HTMLInputElement).selectionStart).toBe(0)
    expect((search as HTMLInputElement).selectionEnd).toBe(3)
  })

  it('types / normally inside a text field', async () => {
    const { user, search } = await renderApp()
    await user.click(search)
    await user.keyboard('a/b')
    expect(search).toHaveValue('a/b')
  })

  it('focuses search with Ctrl+F', async () => {
    const { user, search } = await renderApp()
    await user.keyboard('{Control>}f{/Control}')
    expect(search).toHaveFocus()
  })

  it('opens New investigation with Ctrl+N', async () => {
    const { user } = await renderApp()
    await user.keyboard('{Control>}n{/Control}')
    expect(await screen.findByRole('dialog', { name: 'New question' })).toBeInTheDocument()
  })

  it('copies context with Ctrl+Shift+C and shows Copied', async () => {
    const { user } = await renderApp()
    await user.keyboard('{Control>}{Shift>}C{/Shift}{/Control}')
    await waitFor(() => expect(mocks.api.copyContext).toHaveBeenCalledWith('INV-0001'))
    expect(await screen.findByRole('button', { name: 'Copied' })).toBeInTheDocument()
  })

  it('stands down while a dialog is open', async () => {
    const { user, search } = await renderApp()
    await user.keyboard('{Control>}n{/Control}')
    const dialog = await screen.findByRole('dialog', { name: 'New question' })
    const title = within(dialog).getByLabelText('Question')
    expect(title).toHaveFocus()
    await user.keyboard('{Control>}n{/Control}')
    expect(screen.getAllByRole('dialog')).toHaveLength(1)
    // Focus a non-text control in the dialog, then press /.
    within(dialog).getByRole('button', { name: 'Cancel' }).focus()
    await user.keyboard('/')
    expect(search).not.toHaveFocus()
    await user.keyboard('{Control>}f{/Control}')
    expect(search).not.toHaveFocus()
  })

  it('stands down while a menu is open', async () => {
    const { user, search } = await renderApp()
    await user.click(screen.getByRole('button', { name: 'More actions' }))
    await screen.findByRole('menu', { name: 'More actions' })
    await user.keyboard('/')
    expect(search).not.toHaveFocus()
    await user.keyboard('{Control>}n{/Control}')
    expect(screen.queryByRole('dialog', { name: 'New question' })).not.toBeInTheDocument()
  })

  it('uses Cmd instead of Ctrl on macOS', async () => {
    setPlatform('MacIntel')
    const { user, search } = await renderApp()
    await user.keyboard('{Control>}n{/Control}')
    expect(screen.queryByRole('dialog', { name: 'New question' })).not.toBeInTheDocument()
    await user.keyboard('{Meta>}f{/Meta}')
    expect(search).toHaveFocus()
    search.blur()
    await user.keyboard('{Meta>}n{/Meta}')
    expect(await screen.findByRole('dialog', { name: 'New question' })).toBeInTheDocument()
  })
})
