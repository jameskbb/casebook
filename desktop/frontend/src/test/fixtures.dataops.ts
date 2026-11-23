// Test data for search hits, archived-only lists, and the results of adding a
// question from a file, backing up and restoring.
import type { BackupInfo, ImportOutcome, InvestigationSummary, RestorePick, SearchHit } from '../api/types'
import { backupInfo, summaries } from './fixtures'

export const hits: SearchHit[] = [
  {
    investigation_ref: 'INV-0001',
    investigation_title: 'Why did calls go up 30% in October when sales stayed the same?',
    record_type: 'claim',
    record_ref: 'C-001',
    snippet: 'A **billing** mistake on October 3 charged some customers twice.',
    rank: -4.2,
  },
  {
    investigation_ref: 'INV-0002',
    investigation_title: 'Why are help desk tickets taking longer to close this month?',
    record_type: 'investigation',
    record_ref: 'INV-0002',
    snippet: 'Tickets about **billing** took nine days to close, up from four',
    rank: -3.1,
  },
  {
    investigation_ref: 'INV-0001',
    investigation_title: 'Why did calls go up 30% in October when sales stayed the same?',
    record_type: 'evidence',
    record_ref: 'E-003',
    snippet: '...the head of **billing** said the bills went out twice',
    rank: -2.5,
  },
]

/** A hit whose snippet carries markup that must render as literal text. */
export const markupHit: SearchHit = {
  investigation_ref: 'INV-0001',
  investigation_title: 'Why did calls go up 30% in October when sales stayed the same?',
  record_type: 'question',
  record_ref: 'Q-001',
  snippet: '<img src=x onerror="alert(1)"> is **billing**',
  rank: -1,
}

export const archivedHit: SearchHit = {
  investigation_ref: 'INV-0003',
  investigation_title: 'Why was March payroll $18,000 over budget?',
  record_type: 'hypothesis',
  record_ref: 'H-001',
  snippet: 'Two new hires were on the March **billing** run a month early',
  rank: -2,
}

export const openSummaries: InvestigationSummary[] = summaries.filter((s) => !s.archived)
export const archivedSummaries: InvestigationSummary[] = summaries.filter((s) => s.archived)

export const importSameRef: ImportOutcome = {
  cancelled: false,
  file_name: 'INV-0004-why-did-calls-go-up.json',
  title: 'Why did calls go up 30% in October when sales stayed the same?',
  result: { investigation_ref: 'INV-0004', title: 'Why did calls go up 30% in October when sales stayed the same?', original_ref: 'INV-0004', ref_changed: false, counts: backupInfo.counts },
}

export const importChangedRef: ImportOutcome = {
  cancelled: false,
  file_name: 'INV-0001-why-did-calls-go-up.json',
  title: 'Why did calls go up 30% in October when sales stayed the same?',
  result: { investigation_ref: 'INV-0004', title: 'Why did calls go up 30% in October when sales stayed the same?', original_ref: 'INV-0001', ref_changed: true, counts: backupInfo.counts },
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
  file_name: 'casebook-backup-2026-11-06.db',
  info: backupInfo,
}

export const cancelledPick: RestorePick = { cancelled: true, path: '', file_name: '', info: backupInfo }

export const LONG_DB_PATH = 'C:\\Users\\Ada Lovelace\\AppData\\Local\\Casebook\\data\\casebook.db'
