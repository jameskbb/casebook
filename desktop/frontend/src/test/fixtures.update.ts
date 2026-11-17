// Check for updates fixtures (internal/update Result shapes).
import type { UpdateCheck } from '../api/types'

export const RELEASE_URL = 'https://github.com/jameskbb/casebook/releases/tag/v0.5.0'
export const ASSET_URL =
  'https://github.com/jameskbb/casebook/releases/download/v0.5.0/Casebook-v0.5.0-windows-amd64-portable.zip'

export const upToDate: UpdateCheck = {
  current: 'v0.5.0',
  latest: 'v0.5.0',
  state: 'up_to_date',
  release_url: RELEASE_URL,
  published_at: '2026-10-01T12:00:00Z',
  asset: { name: 'Casebook-v0.5.0-windows-amd64-portable.zip', url: ASSET_URL, size_bytes: 31457280 },
  checked_at: '2026-10-14T09:00:00Z',
}

export const updateAvailable: UpdateCheck = { ...upToDate, current: 'v0.4.0', state: 'update_available' }

export const ahead: UpdateCheck = { ...upToDate, current: 'v0.6.0-rc.1', state: 'ahead' }

export const unknownBuild: UpdateCheck = { ...upToDate, current: 'dev', state: 'unknown' }

export const NO_RELEASE_MESSAGE =
  'GitHub lists no release for jameskbb/casebook: the repository may be private, or no release is published yet'
