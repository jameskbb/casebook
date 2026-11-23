// Home: what needs a person today. Shown when no question is open and at
// least one exists. One list of the questions that need a person, each with
// the one thing it needs next and a button that opens the question on it
// (lib/questions needOf); then the answers already in hand; then questions
// with no sources yet. Things still to find out never make a question need
// a person; an answered one with some open says so on its line. Everything
// comes from the question list; nothing is stored for this view. With
// nothing to do, the line under the heading says only what is true: it names
// decided findings only when a person decided one.
import type { InvestigationSummary } from '../../api/types'
import { useNow } from '../../hooks/useNow'
import { plural } from '../../lib/plural'
import { checkedLine, homeLists, needOf, questionChecks } from '../../lib/questions'
import { useAppData } from '../../state/appData'
import { NO_ANSWER_YET } from '../record/StatusLabel'
import { Button } from '../ui/Button'
import './Home.css'

const today = new Intl.DateTimeFormat('en-US', { weekday: 'long', month: 'long', day: 'numeric' })

export const NOTHING_NEEDS_YOU = 'Nothing needs you right now.'
export const ALL_CHECKED_BODY = 'You have checked every source and every answer.'
export const ALL_CHECKED_DECIDED_BODY = 'You have checked every source and every answer, and decided every finding where sources disagree.'

/** What Home says when nothing needs the person: only what is true of the questions shown. */
export function allCheckedBody(list: InvestigationSummary[]): string {
  const decided = list.some((q) => !q.archived && questionChecks(q).decided > 0)
  return decided ? ALL_CHECKED_DECIDED_BODY : ALL_CHECKED_BODY
}
export const NO_SOURCES_YET = 'No sources yet. Ask your AI to work on it, or add one.'

/** An answered question's line: what is still open, else how many sources a person checked. */
export function answeredLine(q: InvestigationSummary): string {
  const open = Math.max(0, q.counts?.open_questions ?? 0)
  if (open > 0) return `Everything is checked. ${open} ${plural(open, 'thing is', 'things are')} still to find out.`
  return checkedLine(q) || 'No sources'
}

/** "2 questions need you", "1 question needs you". */
export function needYouHeading(n: number): string {
  return n === 0 ? NOTHING_NEEDS_YOU : `${n} ${plural(n, 'question needs you', 'questions need you')}`
}

export function Home() {
  const { investigations, select } = useAppData()
  const now = useNow()
  const { needYou, answered, waiting } = homeLists(investigations)
  return (
    <div className="home reading-column">
      <header>
        <p className="home-eyebrow">{today.format(now)}</p>
        <h1 className="home-heading">{needYouHeading(needYou.length)}</h1>
        {needYou.length === 0 && <p className="home-note">{allCheckedBody(investigations)}</p>}
      </header>

      {needYou.length > 0 && (
        <ul className="home-list" aria-label="Questions that need you">
          {needYou.map((q) => (
            <NeedsYou key={q.ref} question={q} onOpen={(what) => select(q.ref, { openRecord: what })} />
          ))}
        </ul>
      )}

      {answered.length > 0 && (
        <section aria-labelledby="home-answered">
          <h2 className="home-section-heading" id="home-answered">
            Answered
          </h2>
          <ul className="home-list">
            {answered.map((q) => (
              <li key={q.ref} className="home-answered">
                <h3>
                  <button type="button" className="text-button" onClick={() => select(q.ref)}>
                    {q.title}
                  </button>
                </h3>
                <p className="home-answer">{questionChecks(q).answer}</p>
                <p className="home-checked">{answeredLine(q)}</p>
              </li>
            ))}
          </ul>
        </section>
      )}

      {waiting.length > 0 && (
        <section aria-labelledby="home-waiting">
          <h2 className="home-section-heading" id="home-waiting">
            {NO_ANSWER_YET}
          </h2>
          <ul className="home-list">
            {waiting.map((q) => (
              <li key={q.ref} className="home-answered">
                <h3>
                  <button type="button" className="text-button" onClick={() => select(q.ref)}>
                    {q.title}
                  </button>
                </h3>
                <p className="home-checked">{NO_SOURCES_YET}</p>
              </li>
            ))}
          </ul>
        </section>
      )}
    </div>
  )
}

function NeedsYou({ question: q, onOpen }: { question: InvestigationSummary; onOpen: (what: string) => void }) {
  const need = needOf(q)
  if (!need) return null
  return (
    <li className="home-todo">
      <div className="home-todo-main">
        <h2 className="home-todo-title">{q.title}</h2>
        <p className="home-needs">{need.line}</p>
      </div>
      <Button variant="primary" aria-label={`${need.button}: ${q.title}`} onClick={() => onOpen(need.kind)}>
        {need.button}
      </Button>
    </li>
  )
}
