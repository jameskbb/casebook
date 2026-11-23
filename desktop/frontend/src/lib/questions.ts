// Pure helpers over the question list (InvestigationSummary rows): who has
// an answer, how many sources a person has checked, and which questions
// still need a person. Fields a backend may not send yet read as empty.
import type { InvestigationSummary } from '../api/types'
import { sourcesCheckedLabel } from './labels'
import { countOf } from './plural'

export interface QuestionChecks {
  answer: string
  answered: boolean
  sourcesTotal: number
  checkedByYou: number
  disagree: number
  /** Findings under What backs this up. */
  backing: number
  /** Findings a person decided hold while a source still goes against them. */
  decided: number
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
    backing: Math.max(0, q.findings_backing ?? 0),
    decided: Math.max(0, q.findings_decided ?? 0),
  }
}

/** What Home opens the question on: the source check, the first finding to decide, Add finding, the answer editor, or the answer check. */
export type NeedKind = 'check-sources' | 'decide' | 'add-finding' | 'write-answer' | 'check-answer'

/** Why a question needs a person: the row's one line and its button. */
export interface Need {
  kind: NeedKind
  line: string
  button: string
}

/** Home's line, and the answer card's, for a question with sources but no finding that holds. */
export const ADD_FINDING_NEED = 'Add a finding from your sources.'

/** The one thing a question needs from a person next, or null. The first match wins. */
export function needOf(q: InvestigationSummary): Need | null {
  if (q.archived) return null
  const c = questionChecks(q)
  if (c.checkedByYou < c.sourcesTotal) return { kind: 'check-sources', line: sourcesCheckedLabel(c.checkedByYou, c.sourcesTotal), button: 'Check' }
  if (c.disagree > 0) return { kind: 'decide', line: countOf(c.disagree, 'finding where sources disagree', 'findings where sources disagree'), button: 'Decide' }
  // The answer stands on a finding that holds; the answer card says the same.
  if (c.sourcesTotal > 0 && !c.answered && c.backing === 0) return { kind: 'add-finding', line: ADD_FINDING_NEED, button: 'Add a finding' }
  if (c.sourcesTotal > 0 && !c.answered) return { kind: 'write-answer', line: 'Every source is checked. Write the answer.', button: 'Write the answer' }
  // Only a backend that says so: one that leaves the field out never nags.
  if (c.answered && q.answer_checked_by_you === false) return { kind: 'check-answer', line: 'The answer is not checked yet.', button: 'Check the answer' }
  return null
}

export function needsYou(q: InvestigationSummary): boolean {
  return needOf(q) !== null
}

/** "1 of 3 sources checked by you"; "" when the question has no sources. */
export function checkedLine(q: InvestigationSummary): string {
  const c = questionChecks(q)
  return c.sourcesTotal === 0 ? '' : sourcesCheckedLabel(c.checkedByYou, c.sourcesTotal)
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
  needYou: InvestigationSummary[]
  /** Answered, with nothing left to do. */
  answered: InvestigationSummary[]
  /** No answer and no sources yet. */
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
