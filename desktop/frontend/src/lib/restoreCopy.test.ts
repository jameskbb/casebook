import { describe, expect, it } from 'vitest'
import { formatAbsolute } from './relativeTime'
import { countPhrase, restoreSummary } from './restoreCopy'
import { backupInfo } from '../test/fixtures'
import { singleBackup } from '../test/fixtures.dataops'

describe('restore copy', () => {
  it('uses the singular for 1 and the plural otherwise', () => {
    expect(countPhrase(1, 'claim', 'claims')).toBe('1 claim')
    expect(countPhrase(0, 'claim', 'claims')).toBe('0 claims')
    expect(countPhrase(6, 'claim', 'claims')).toBe('6 claims')
  })

  it('describes a backup with plural counts', () => {
    expect(restoreSummary('casebook-backup-2026-11-06.db', backupInfo)).toBe(
      `casebook-backup-2026-11-06.db, saved ${formatAbsolute(backupInfo.created_at)}, contains 3 questions, 7 sources, 9 findings and 5 things to find out.`,
    )
  })

  it('describes a backup with one of everything', () => {
    expect(restoreSummary('one.db', singleBackup)).toBe(
      `one.db, saved ${formatAbsolute(singleBackup.created_at)}, contains 1 question, 1 source, 2 findings and 1 thing to find out.`,
    )
  })
})
