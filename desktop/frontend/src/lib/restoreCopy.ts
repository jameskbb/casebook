// Copy for the restore confirmation (UI-SPEC Destructive and replacing
// actions, Restore row): what the chosen backup holds, with singular forms
// for counts of 1.
import type { BackupInfo } from '../api/types'
import { formatAbsolute } from './relativeTime'

/** "1 claim", "0 claims", "6 claims". */
export function countPhrase(n: number, one: string, many: string): string {
  return `${n} ${n === 1 ? one : many}`
}

/**
 * "{file name}, saved {date}, contains 3 cases, 7 pieces of
 * evidence, 6 claims, 3 hypotheses and 5 questions."
 */
export function restoreSummary(fileName: string, info: BackupInfo): string {
  const c = info.counts
  const saved = formatAbsolute(info.created_at)
  const parts = [
    countPhrase(c.investigations, 'case', 'cases'),
    countPhrase(c.evidence, 'piece of evidence', 'pieces of evidence'),
    countPhrase(c.claims, 'claim', 'claims'),
    countPhrase(c.hypotheses, 'hypothesis', 'hypotheses'),
  ]
  const counts = `${parts.join(', ')} and ${countPhrase(c.questions, 'question', 'questions')}`
  return saved ? `${fileName}, saved ${saved}, contains ${counts}.` : `${fileName} contains ${counts}.`
}
