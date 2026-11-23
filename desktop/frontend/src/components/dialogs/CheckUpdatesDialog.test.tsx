import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Api } from '../../api/api'
import { ApiError, WENT_WRONG_COPY } from '../../api/errors'
import type { UpdateCheck } from '../../api/types'
import { ahead, ASSET_URL, NO_RELEASE_MESSAGE, unknownBuild, updateAvailable, upToDate } from '../../test/fixtures.update'
import { mockApi } from '../../test/mockApi'
import { ToastProvider } from '../ui/Toaster'
import { CheckUpdatesDialog, LINK_COPIED, UPDATE_FAILED_COPY, UPDATE_HOW_COPY, UPDATE_INTRO_COPY } from './CheckUpdatesDialog'

const mocks = vi.hoisted(() => ({ api: {} as Api }))
vi.mock('../../api/api', () => ({ api: mocks.api }))

function renderDialog(onClose = vi.fn()) {
  render(
    <ToastProvider>
      <CheckUpdatesDialog onClose={onClose} />
    </ToastProvider>,
  )
  return onClose
}

/** Renders the dialog with checkForUpdates resolving r and chooses Look now. */
async function checkWith(r: UpdateCheck) {
  Object.assign(mocks.api, mockApi({ checkForUpdates: vi.fn(async () => r) }))
  const user = userEvent.setup()
  renderDialog()
  await user.click(screen.getByRole('button', { name: 'Look now' }))
  await screen.findByRole('button', { name: 'Look again' })
  return user
}

describe('CheckUpdatesDialog', () => {
  beforeEach(() => {
    Object.assign(mocks.api, mockApi())
  })

  it('explains the look, without naming GitHub, and asks nothing until Look now', async () => {
    const onClose = renderDialog()
    const dialog = screen.getByRole('dialog', { name: 'Look for updates' })
    expect(within(dialog).getByText(UPDATE_INTRO_COPY)).toBeInTheDocument()
    expect(dialog).not.toHaveTextContent(/github/i)
    expect(screen.getByRole('button', { name: 'Look now' })).toHaveFocus()
    expect(screen.queryByRole('button', { name: 'Open the download page' })).not.toBeInTheDocument()
    await new Promise((r) => setTimeout(r, 20))
    expect(mocks.api.checkForUpdates).not.toHaveBeenCalled()
    await userEvent.setup().click(within(dialog).getAllByRole('button', { name: 'Close' }).at(-1)!)
    expect(onClose).toHaveBeenCalled()
  })

  it('shows Looking... while it looks', async () => {
    let finish: (r: UpdateCheck) => void = () => {}
    Object.assign(
      mocks.api,
      mockApi({ checkForUpdates: vi.fn(() => new Promise<UpdateCheck>((resolve) => (finish = resolve))) }),
    )
    const user = userEvent.setup()
    renderDialog()
    await user.click(screen.getByRole('button', { name: 'Look now' }))
    expect(await screen.findByRole('button', { name: 'Looking...' })).toBeDisabled()
    finish(upToDate)
    expect(await screen.findByText('You have the latest version, Casebook v0.5.0.')).toBeInTheDocument()
    expect(mocks.api.checkForUpdates).toHaveBeenCalledTimes(1)
  })

  it('says when this is the latest version', async () => {
    await checkWith(upToDate)
    const line = screen.getByText('You have the latest version, Casebook v0.5.0.')
    expect(line.closest('[aria-live="polite"]')).not.toBeNull()
    expect(screen.queryByText(UPDATE_INTRO_COPY)).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Open the download page' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Copy download link' })).not.toBeInTheDocument()
  })

  it('offers the release page and the download link when an update is available', async () => {
    const user = await checkWith(updateAvailable)
    expect(screen.getByText('Casebook v0.5.0 is available. You have v0.4.0.')).toBeInTheDocument()
    expect(screen.getByText(UPDATE_HOW_COPY)).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Open the download page' }))
    await waitFor(() => expect(mocks.api.openReleasePage).toHaveBeenCalledTimes(1))
    expect(mocks.api.openReleasePage).toHaveBeenCalledWith()

    await user.click(screen.getByRole('button', { name: 'Copy download link' }))
    await waitFor(() => expect(mocks.api.copyText).toHaveBeenCalledWith(ASSET_URL))
    expect(await screen.findByText(LINK_COPIED)).toBeInTheDocument()
  })

  it('hides Copy download link when the release has no download for this system', async () => {
    await checkWith({ ...updateAvailable, asset: null })
    expect(screen.getByRole('button', { name: 'Open the download page' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Copy download link' })).not.toBeInTheDocument()
  })

  it('hides Copy download link when Go omits the asset', async () => {
    const { asset: _asset, ...noAsset } = updateAvailable
    await checkWith(noAsset)
    expect(screen.queryByRole('button', { name: 'Copy download link' })).not.toBeInTheDocument()
  })

  it('says when this build is newer than the latest release', async () => {
    await checkWith(ahead)
    expect(
      screen.getByText('You have Casebook v0.6.0. The newest version for everyone is v0.5.0.'),
    ).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Open the download page' })).not.toBeInTheDocument()
  })

  it('says when this build is not a release version', async () => {
    await checkWith(unknownBuild)
    expect(
      screen.getByText('This is a test version (dev). The newest version for everyone is Casebook v0.5.0.'),
    ).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Open the download page' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Copy download link' })).not.toBeInTheDocument()
  })

  it('shows a long test version short', async () => {
    await checkWith({ ...unknownBuild, current: 'v0.8.3-0.2026-abc' })
    expect(
      screen.getByText('This is a test version (v0.8.3). The newest version for everyone is Casebook v0.5.0.'),
    ).toBeInTheDocument()
    expect(screen.getByRole('dialog')).not.toHaveTextContent('2026-abc')
  })

  it('looks again on request', async () => {
    const user = await checkWith(upToDate)
    await user.click(screen.getByRole('button', { name: 'Look again' }))
    await waitFor(() => expect(mocks.api.checkForUpdates).toHaveBeenCalledTimes(2))
  })

  it('shows the fixed failure copy, never the message from Go, and allows another try', async () => {
    const check = vi
      .fn<Api['checkForUpdates']>()
      .mockRejectedValueOnce(new ApiError('other', NO_RELEASE_MESSAGE))
      .mockResolvedValueOnce(upToDate)
    Object.assign(mocks.api, mockApi({ checkForUpdates: check }))
    const user = userEvent.setup()
    renderDialog()
    await user.click(screen.getByRole('button', { name: 'Look now' }))
    const dialog = screen.getByRole('dialog', { name: 'Look for updates' })
    expect(await within(dialog).findByRole('alert')).toHaveTextContent(UPDATE_FAILED_COPY)
    expect(dialog).not.toHaveTextContent(NO_RELEASE_MESSAGE)

    await user.click(screen.getByRole('button', { name: 'Look now' }))
    expect(await screen.findByText('You have the latest version, Casebook v0.5.0.')).toBeInTheDocument()
    expect(within(dialog).queryByRole('alert')).not.toBeInTheDocument()
  })

  it('shows a toast when the release page cannot be opened', async () => {
    const user = await checkWith(updateAvailable)
    Object.assign(mocks.api, {
      openReleasePage: vi.fn(async () => {
        throw new ApiError('other', 'Check for updates first')
      }),
    })
    await user.click(screen.getByRole('button', { name: 'Open the download page' }))
    expect(await screen.findByText(WENT_WRONG_COPY)).toBeInTheDocument()
    expect(screen.queryByText('Check for updates first')).not.toBeInTheDocument()
  })
})
