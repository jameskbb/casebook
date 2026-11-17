// Test data for Plan 04: search hits, archived-only lists, and the import,
// backup and restore results. fixtures.ts stays unchanged.
import type { BackupInfo, ImportOutcome, InvestigationSummary, RestorePick, SearchHit } from '../api/types'
import { backupInfo, summaries } from './fixtures'

export const hits: SearchHit[] = [
  {
    investigation_ref: 'INV-0001',
    investigation_title: 'September cloud bill up 38%',
    record_type: 'claim',
    record_ref: 'C-001',
    snippet: 'A shared platform default moved every service to **debug** logging on August 28',
    rank: -4.2,
  },
  {
    investigation_ref: 'INV-0002',
    investigation_title: 'Checkout p99 doubled after the region move',
    record_type: 'investigation',
    record_ref: 'INV-0002',
    snippet: 'Sampled checkout traces with **debug** spans show the fraud call at 470 ms',
    rank: -3.1,
  },
  {
    investigation_ref: 'INV-0001',
    investigation_title: 'September cloud bill up 38%',
    record_type: 'evidence',
    record_ref: 'E-003',
    snippet: '...214 services ingesting **debug** logs since August 28',
    rank: -2.5,
  },
]

/** A hit whose snippet carries markup that must render as literal text. */
export const markupHit: SearchHit = {
  investigation_ref: 'INV-0001',
  investigation_title: 'September cloud bill up 38%',
  record_type: 'question',
  record_ref: 'Q-001',
  snippet: '<img src=x onerror="alert(1)"> is **debug**',
  rank: -1,
}

export const archivedHit: SearchHit = {
  investigation_ref: 'INV-0003',
  investigation_title: '31 former employees still have active SSO accounts',
  record_type: 'hypothesis',
  record_ref: 'H-001',
  snippet: 'The sync job logs a **debug** line and exits 0 on a 404',
  rank: -2,
}

export const openSummaries: InvestigationSummary[] = summaries.filter((s) => !s.archived)
export const archivedSummaries: InvestigationSummary[] = summaries.filter((s) => s.archived)

export const importSameRef: ImportOutcome = {
  cancelled: false,
  file_name: 'INV-0004-september-cloud-bill-up-38.json',
  title: 'September cloud bill up 38%',
  result: { investigation_ref: 'INV-0004', title: 'September cloud bill up 38%', original_ref: 'INV-0004', ref_changed: false, counts: backupInfo.counts },
}

export const importChangedRef: ImportOutcome = {
  cancelled: false,
  file_name: 'INV-0001-september-cloud-bill-up-38.json',
  title: 'September cloud bill up 38%',
  result: { investigation_ref: 'INV-0004', title: 'September cloud bill up 38%', original_ref: 'INV-0001', ref_changed: true, counts: backupInfo.counts },
}

export const cancelledImport: ImportOutcome = {
  cancelled: true,
  file_name: '',
  title: '',
  result: { investigation_ref: '', title: '', original_ref: '', ref_changed: false, counts: backupInfo.counts },
}

/** A backup with exactly one of everything, for the singular forms. */
export const singleBackup: BackupInfo = {
  ...backupInfo,
  counts: {
    investigations: 1,
    evidence: 1,
    claims: 1,
    hypotheses: 1,
    questions: 1,
    claim_links: 1,
    hypothesis_links: 0,
    activity: 3,
  },
}

export const restorePick: RestorePick = {
  cancelled: false,
  path: backupInfo.path,
  file_name: 'casebook-backup-2026-10-02.db',
  info: backupInfo,
}

export const cancelledPick: RestorePick = { cancelled: true, path: '', file_name: '', info: backupInfo }

export const LONG_DB_PATH = 'C:\\Users\\Ada Lovelace\\AppData\\Local\\Casebook\\data\\casebook.db'
