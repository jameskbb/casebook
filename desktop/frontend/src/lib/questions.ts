// Pure helpers over the question list (InvestigationSummary rows): who has
// an answer, how many sources a person has checked, and which questions
// still need a person. Fields a backend may not send yet read as empty.
import type { InvestigationSummary } from '../api/types'
import { plural } from './plural'

export interface QuestionChecks {
  answer: string
  answered: boolean
  sourcesTotal: number
  checkedByYou: number
  disagree: number
}

export function questionChecks(q: InvestigationSummary): QuestionChecks {
  const answer = (q.conclusion ?? '').trim()
  const sourcesTotal = Math.max(0, q.sources_total ?? 0)
  return {
    answer,
    answered: answer !== '',
    sourcesTotal,
    checkedByYou: Math.min(sourcesTotal, Math.max(0, q.sources_checked_by_you ?? 0)),
    disagree: Math.max(0, q.findings_disagree ?? 0),
  }
}

/** A question needs you when a source is not checked by you, or sources disagree about a finding. */
export function needsYou(q: InvestigationSummary): boolean {
  const c = questionChecks(q)
  return !q.archived && (c.checkedByYou < c.sourcesTotal || c.disagree > 0)
}

/** "1 of 3 sources checked by you"; "" when the question has no sources. */
export function checkedLine(q: InvestigationSummary): string {
  const c = questionChecks(q)
  if (c.sourcesTotal === 0) return ''
  return `${c.checkedByYou} of ${c.sourcesTotal} ${plural(c.sourcesTotal, 'source', 'sources')} checked by you`
}

export interface QuestionGroups {
  noAnswer: InvestigationSummary[]
  answered: InvestigationSummary[]
}

/** The sidebar groups, each in the order the list arrived (newest first). */
export function groupQuestions(list: InvestigationSummary[]): QuestionGroups {
  const g: QuestionGroups = { noAnswer: [], answered: [] }
  for (const q of list) (questionChecks(q).answered ? g.answered : g.noAnswer).push(q)
  return g
}

export interface HomeLists {
  /** Sources left to check, or findings where sources disagree. */
  needYou: InvestigationSummary[]
  /** Answered, with nothing left to check. */
  answered: InvestigationSummary[]
  /** No answer and nothing to check yet. */
  waiting: InvestigationSummary[]
}

/** What Home shows. Archived questions are left out. */
export function homeLists(list: InvestigationSummary[]): HomeLists {
  const h: HomeLists = { needYou: [], answered: [], waiting: [] }
  for (const q of list) {
    if (q.archived) continue
    if (needsYou(q)) h.needYou.push(q)
    else if (questionChecks(q).answered) h.answered.push(q)
    else h.waiting.push(q)
  }
  return h
}
