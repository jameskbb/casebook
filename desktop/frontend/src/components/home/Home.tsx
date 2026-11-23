// Home: what there is to check today. Shown when no question is open and at
// least one exists. One list of the questions that need a person (a source
// they have not checked, or a finding where sources disagree), then the
// answers already in hand, then questions still waiting for a first finding.
// Everything comes from the question list; nothing is stored for this view.
import { CircleAlert } from 'lucide-react'
import type { InvestigationSummary } from '../../api/types'
import { useNow } from '../../hooks/useNow'
import { countOf, plural } from '../../lib/plural'
import { checkedLine, homeLists, questionChecks } from '../../lib/questions'
import { useAppData } from '../../state/appData'
import { NO_ANSWER_YET } from '../record/StatusLabel'
import { Button } from '../ui/Button'
import '../record/VerificationBadge.css'
import './Home.css'

const today = new Intl.DateTimeFormat('en-US', { weekday: 'long', month: 'long', day: 'numeric' })

export const ALL_CHECKED_HEADING = 'Everything is checked'
export const ALL_CHECKED_BODY = 'You have checked every source, and no sources disagree.'

/** "2 questions need you", "1 question needs you". */
export function needYouHeading(n: number): string {
  return n === 0 ? ALL_CHECKED_HEADING : `${n} ${plural(n, 'question needs you', 'questions need you')}`
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
        {needYou.length === 0 && <p className="home-note">{ALL_CHECKED_BODY}</p>}
      </header>

      {needYou.length > 0 && (
        <ul className="home-list" aria-label="Questions that need you">
          {needYou.map((q) => (
            <NeedsYou key={q.ref} question={q} onOpen={() => select(q.ref)} />
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
                <p className="home-checked">{checkedLine(q) || 'No sources'}</p>
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
                <p className="home-checked">Nothing to check yet. Ask your AI to work on it.</p>
              </li>
            ))}
          </ul>
        </section>
      )}
    </div>
  )
}

function NeedsYou({ question: q, onOpen }: { question: InvestigationSummary; onOpen: () => void }) {
  const c = questionChecks(q)
  return (
    <li className="home-todo">
      <div className="home-todo-main">
        <h2 className="home-todo-title">{q.title}</h2>
        <p className="home-needs">
          {c.sourcesTotal > 0 && (
            <span>
              <strong>
                {c.checkedByYou} of {c.sourcesTotal}
              </strong>{' '}
              {plural(c.sourcesTotal, 'source', 'sources')} checked by you
            </span>
          )}
          {c.disagree > 0 && (
            <span className="badge badge-disputed">
              <CircleAlert className="badge-icon" size={16} strokeWidth={1.75} aria-hidden="true" />
              {countOf(c.disagree, 'finding where sources disagree', 'findings where sources disagree')}
            </span>
          )}
          {!c.answered && <span>{NO_ANSWER_YET}</span>}
        </p>
      </div>
      <Button variant="primary" aria-label={`Check: ${q.title}`} onClick={onOpen}>
        Check
      </Button>
    </li>
  )
}
