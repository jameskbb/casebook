// Grouped search results (UI-SPEC Component Contract, SearchResults; D-19).
// Hits are grouped under their investigation in rank order (a group appears
// where its best hit ranks). Each group header shows the title, the ref, the
// "Your AI's current case" marker and the Archived tag; each hit is a button with its kind
// icon, record ref and snippet. Snippets are built from text segments and
// <mark> elements only, so stored text can never become markup (T-02-01).
// Activating a hit opens its investigation and, for evidence, claims,
// hypotheses and questions, that record (select with openRecord).
import { Fragment, useId, useImperativeHandle, type Ref } from 'react'
import { CircleQuestionMark, FileText, Folder, Lightbulb, Quote, SearchX, type LucideIcon } from 'lucide-react'
import type { RecordType, SearchHit } from '../../api/types'
import { useRovingFocus } from '../../hooks/useRovingFocus'
import { hitSegments } from '../../lib/snippet'
import { useAppData } from '../../state/appData'
import { ArchivedTag } from '../record/StatusLabel'
import { ActiveMarker } from './InvestigationRow'
import './SearchResults.css'

const KIND: Record<RecordType, { icon: LucideIcon; label: string }> = {
  investigation: { icon: Folder, label: 'Case' },
  evidence: { icon: FileText, label: 'Evidence' },
  claim: { icon: Quote, label: 'Claim' },
  hypothesis: { icon: Lightbulb, label: 'Hypothesis' },
  question: { icon: CircleQuestionMark, label: 'Question' },
}

export const NO_MATCHES_HINT =
  'Try fewer or different words. Archived cases are searched only when Show archived is on.'

export interface SearchResultsHandle {
  /** Moves focus into the results (the hit holding the Tab stop). */
  focus(): void
}

export interface SearchResultsProps {
  /** The query these hits answer (shown in the no-matches heading). */
  query: string
  hits: SearchHit[]
  handle?: Ref<SearchResultsHandle>
}

interface Group {
  ref: string
  title: string
  hits: { hit: SearchHit; index: number }[]
}

function groupHits(hits: SearchHit[]): Group[] {
  const groups = new Map<string, Group>()
  hits.forEach((hit, index) => {
    let g = groups.get(hit.investigation_ref)
    if (!g) {
      g = { ref: hit.investigation_ref, title: hit.investigation_title, hits: [] }
      groups.set(hit.investigation_ref, g)
    }
    g.hits.push({ hit, index })
  })
  return [...groups.values()]
}

export function SearchResults({ query, hits, handle }: SearchResultsProps) {
  const { investigations, select } = useAppData()
  const groups = groupHits(hits)
  // Hits in display order (grouped), so Up and Down follow what is on screen.
  const ordered = groups.flatMap((g) => g.hits)
  const roving = useRovingFocus(ordered.length)
  const baseId = useId()
  useImperativeHandle(handle, () => ({ focus: () => roving.focusCurrent() }), [roving])

  if (hits.length === 0) {
    return (
      <div className="search-empty">
        <SearchX className="search-empty-icon" size={20} strokeWidth={1.75} aria-hidden="true" />
        <p className="search-empty-heading">No matches for "{query}"</p>
        <p className="search-empty-body">{NO_MATCHES_HINT}</p>
      </div>
    )
  }

  const open = (hit: SearchHit) => {
    if (hit.record_type === 'investigation') select(hit.investigation_ref)
    else select(hit.investigation_ref, { openRecord: hit.record_ref })
  }

  let position = 0
  return (
    <section className="search-results" aria-label="Search results" onKeyDown={roving.onKeyDown}>
      {groups.map((g, gi) => {
        const summary = investigations.find((i) => i.ref === g.ref)
        const titleId = `${baseId}-g${gi}-title`
        const refId = `${baseId}-g${gi}-ref`
        return (
          <div key={g.ref} className="search-group" role="group" aria-labelledby={`${titleId} ${refId}`}>
            <div className="search-group-header">
              <p className="search-group-line">
                <span className="search-group-title" id={titleId}>
                  {g.title}
                </span>
                {summary?.is_active && <ActiveMarker />}
              </p>
              <p className="search-group-line search-group-meta">
                <span className="search-group-ref tabular" id={refId}>
                  {g.ref}
                </span>
                {summary?.archived && <ArchivedTag />}
              </p>
            </div>
            <ul className="search-hits">
              {g.hits.map(({ hit }) => {
                const i = position++
                const kind = KIND[hit.record_type] ?? KIND.investigation
                const Icon = kind.icon
                return (
                  <li key={`${hit.record_type}-${hit.record_ref}`}>
                    <button
                      {...roving.itemProps(i)}
                      type="button"
                      className={`search-hit search-hit-${hit.record_type}`}
                      onClick={() => open(hit)}
                    >
                      <span className="search-hit-line">
                        <Icon className="search-hit-icon" size={16} strokeWidth={1.75} aria-hidden="true" />
                        <span className="sr-only">{kind.label} </span>
                        <span className="search-hit-ref tabular">{hit.record_ref}</span>
                      </span>
                      <span className="search-hit-snippet">
                        {hitSegments(hit).map((seg, n) =>
                          seg.mark ? <mark key={n}>{seg.text}</mark> : <Fragment key={n}>{seg.text}</Fragment>,
                        )}
                      </span>
                    </button>
                  </li>
                )
              })}
            </ul>
          </div>
        )
      })}
    </section>
  )
}
