// Copy for the restore confirmation: what the chosen backup holds, with
// singular forms
// for counts of 1.
import type { BackupInfo } from '../api/types'
import { formatAbsolute } from './relativeTime'

/** "1 claim", "0 claims", "6 claims". */
export function countPhrase(n: number, one: string, many: string): string {
  return `${n} ${n === 1 ? one : many}`
}

/**
 * "{file name}, saved {date}, contains 3 questions, 7 sources,
 * 9 findings and 5 things to find out."
 */
export function restoreSummary(fileName: string, info: BackupInfo): string {
  const c = info.counts
  const saved = formatAbsolute(info.created_at)
  const parts = [
    countPhrase(c.investigations, 'question', 'questions'),
    countPhrase(c.evidence, 'source', 'sources'),
    countPhrase(c.claims + c.hypotheses, 'finding', 'findings'),
  ]
  const counts = `${parts.join(', ')} and ${countPhrase(c.questions, 'thing to find out', 'things to find out')}`
  return saved ? `${fileName}, saved ${saved}, contains ${counts}.` : `${fileName} contains ${counts}.`
}
