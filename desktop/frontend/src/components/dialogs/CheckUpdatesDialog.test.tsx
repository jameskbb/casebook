import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Api } from '../../api/api'
import { ApiError } from '../../api/errors'
import type { UpdateCheck } from '../../api/types'
import { ahead, ASSET_URL, NO_RELEASE_MESSAGE, unknownBuild, updateAvailable, upToDate } from '../../test/fixtures.update'
import { mockApi } from '../../test/mockApi'
import { ToastProvider } from '../ui/Toaster'
import { CheckUpdatesDialog, LINK_COPIED, UPDATE_HOW_COPY, UPDATE_INTRO_COPY } from './CheckUpdatesDialog'

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

/** Renders the dialog with checkForUpdates resolving r and chooses Check now. */
async function checkWith(r: UpdateCheck) {
  Object.assign(mocks.api, mockApi({ checkForUpdates: vi.fn(async () => r) }))
  const user = userEvent.setup()
  renderDialog()
  await user.click(screen.getByRole('button', { name: 'Check now' }))
  await screen.findByRole('button', { name: 'Check again' })
  return user
}

describe('CheckUpdatesDialog', () => {
  beforeEach(() => {
    Object.assign(mocks.api, mockApi())
  })

  it('explains the check and asks nothing until Check now', async () => {
    const onClose = renderDialog()
    const dialog = screen.getByRole('dialog', { name: 'Check for updates' })
    expect(within(dialog).getByText(UPDATE_INTRO_COPY)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Check now' })).toHaveFocus()
    expect(screen.queryByRole('button', { name: 'Open release page' })).not.toBeInTheDocument()
    await new Promise((r) => setTimeout(r, 20))
    expect(mocks.api.checkForUpdates).not.toHaveBeenCalled()
    await userEvent.setup().click(within(dialog).getAllByRole('button', { name: 'Close' }).at(-1)!)
    expect(onClose).toHaveBeenCalled()
  })

  it('shows Checking... while the check runs', async () => {
    let finish: (r: UpdateCheck) => void = () => {}
    Object.assign(
      mocks.api,
      mockApi({ checkForUpdates: vi.fn(() => new Promise<UpdateCheck>((resolve) => (finish = resolve))) }),
    )
    const user = userEvent.setup()
    renderDialog()
    await user.click(screen.getByRole('button', { name: 'Check now' }))
    expect(await screen.findByRole('button', { name: 'Checking...' })).toBeDisabled()
    finish(upToDate)
    expect(await screen.findByText('You have the latest version, Casebook v0.5.0.')).toBeInTheDocument()
    expect(mocks.api.checkForUpdates).toHaveBeenCalledTimes(1)
  })

  it('says when this is the latest version', async () => {
    await checkWith(upToDate)
    const line = screen.getByText('You have the latest version, Casebook v0.5.0.')
    expect(line.closest('[aria-live="polite"]')).not.toBeNull()
    expect(screen.queryByText(UPDATE_INTRO_COPY)).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Open release page' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Copy download link' })).not.toBeInTheDocument()
  })

  it('offers the release page and the download link when an update is available', async () => {
    const user = await checkWith(updateAvailable)
    expect(screen.getByText('Casebook v0.5.0 is available. You have v0.4.0.')).toBeInTheDocument()
    expect(screen.getByText(UPDATE_HOW_COPY)).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Open release page' }))
    await waitFor(() => expect(mocks.api.openReleasePage).toHaveBeenCalledTimes(1))
    expect(mocks.api.openReleasePage).toHaveBeenCalledWith()

    await user.click(screen.getByRole('button', { name: 'Copy download link' }))
    await waitFor(() => expect(mocks.api.copyText).toHaveBeenCalledWith(ASSET_URL))
    expect(await screen.findByText(LINK_COPIED)).toBeInTheDocument()
  })

  it('hides Copy download link when the release has no download for this system', async () => {
    await checkWith({ ...updateAvailable, asset: null })
    expect(screen.getByRole('button', { name: 'Open release page' })).toBeInTheDocument()
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
      screen.getByText('You have Casebook v0.6.0-rc.1, which is newer than the latest release, v0.5.0.'),
    ).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Open release page' })).not.toBeInTheDocument()
  })

  it('says when this build is not a release version', async () => {
    await checkWith(unknownBuild)
    expect(
      screen.getByText('This build (dev) is not a release version. The latest release is Casebook v0.5.0.'),
    ).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Open release page' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Copy download link' })).not.toBeInTheDocument()
  })

  it('checks again on request', async () => {
    const user = await checkWith(upToDate)
    await user.click(screen.getByRole('button', { name: 'Check again' }))
    await waitFor(() => expect(mocks.api.checkForUpdates).toHaveBeenCalledTimes(2))
  })

  it('shows the error from Go in the dialog and allows a retry', async () => {
    const check = vi
      .fn<Api['checkForUpdates']>()
      .mockRejectedValueOnce(new ApiError('other', NO_RELEASE_MESSAGE))
      .mockResolvedValueOnce(upToDate)
    Object.assign(mocks.api, mockApi({ checkForUpdates: check }))
    const user = userEvent.setup()
    renderDialog()
    await user.click(screen.getByRole('button', { name: 'Check now' }))
    const dialog = screen.getByRole('dialog', { name: 'Check for updates' })
    expect(await within(dialog).findByRole('alert')).toHaveTextContent(`${NO_RELEASE_MESSAGE}.`)

    await user.click(screen.getByRole('button', { name: 'Check now' }))
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
    await user.click(screen.getByRole('button', { name: 'Open release page' }))
    expect(await screen.findByText('Check for updates first')).toBeInTheDocument()
  })
})
