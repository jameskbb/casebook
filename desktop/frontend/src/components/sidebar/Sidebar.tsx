// The investigation sidebar (UI-SPEC Layout, main window): wordmark, New
// case, the search field, the Show archived switch, the scrolling
// list (grouped search hits while a query is present) and the footer with
// the data operations, theme and data file. Everything but the list stays
// fixed while the list scrolls.
//
// Search (D-19): typing waits 200 ms, then calls Search with the switch's
// include_archived; an empty or blank query shows the list without a call.
// While a newer search runs the previous results stay visible, and a data
// change (dataVersion) reruns the current search. The list and the hits are
// each one Tab stop (roving tabindex); Down from the field enters them.
import { useEffect, useId, useRef, useState, type KeyboardEvent } from 'react'
import { CircleAlert, Plus, Search, X } from 'lucide-react'
import { api } from '../../api/api'
import { cleanMessage, errorCopy, toApiError } from '../../api/errors'
import type { SearchHit } from '../../api/types'
import { useDelayedFlag } from '../../hooks/useDelayedFlag'
import { useNow } from '../../hooks/useNow'
import { useRovingFocus } from '../../hooks/useRovingFocus'
import { hasOpenLayer } from '../../lib/layers'
import { LIST_LIMIT, useAppData } from '../../state/appData'
import { Button } from '../ui/Button'
import { IconButton } from '../ui/IconButton'
import { Switch } from '../ui/Switch'
import { InvestigationRow } from './InvestigationRow'
import { SearchResults, type SearchResultsHandle } from './SearchResults'
import { SidebarFooter } from './SidebarFooter'
import './Sidebar.css'

export const SEARCH_DEBOUNCE_MS = 200
export const SEARCH_LIMIT = 50
export const SEARCH_FIELD_ID = 'sidebar-search'
const SEARCH_LABEL = 'Search cases'
export const LIST_ERROR_COPY = 'Casebook could not load the list.'
export const LIST_CAPPED_COPY = `Showing the ${LIST_LIMIT.toLocaleString('en-US')} most recently updated cases. Search to find older ones.`

export interface SidebarProps {
  onNew: () => void
}

interface SearchResult {
  query: string
  hits: SearchHit[]
}

export function Sidebar({ onNew }: SidebarProps) {
  const { investigations, listStatus, selectedRef, select, includeArchived, setIncludeArchived, dataVersion, refresh } =
    useAppData()
  const [retrying, setRetrying] = useState(false)
  const retryList = async () => {
    setRetrying(true)
    try {
      await refresh()
    } finally {
      setRetrying(false)
    }
  }
  const now = useNow()
  // Skeleton rows appear only when the first load takes longer than 300 ms.
  const loading = listStatus === 'loading' && investigations.length === 0
  const showSkeleton = useDelayedFlag(loading)

  const [query, setQuery] = useState('')
  const [debounced, setDebounced] = useState('')
  const [result, setResult] = useState<SearchResult | null>(null)
  const [searchError, setSearchError] = useState<string | null>(null)
  const searchSeq = useRef(0)
  const fieldRef = useRef<HTMLInputElement>(null)
  const resultsRef = useRef<SearchResultsHandle>(null)
  const errorId = useId()

  const trimmed = query.trim()
  const searching = trimmed !== ''

  useEffect(() => {
    if (!trimmed) {
      setDebounced('')
      return
    }
    const t = setTimeout(() => setDebounced(trimmed), SEARCH_DEBOUNCE_MS)
    return () => clearTimeout(t)
  }, [trimmed])

  useEffect(() => {
    const seq = ++searchSeq.current
    if (!debounced) {
      setResult(null)
      setSearchError(null)
      return
    }
    api.search(debounced, { include_archived: includeArchived, limit: SEARCH_LIMIT }).then(
      (hits) => {
        if (seq !== searchSeq.current) return
        setResult({ query: debounced, hits })
        setSearchError(null)
      },
      (e) => {
        if (seq !== searchSeq.current) return
        const err = toApiError(e)
        setSearchError(err.kind === 'validation' ? cleanMessage(err.message) : errorCopy(err).message)
        setResult(null)
      },
    )
  }, [debounced, includeArchived, dataVersion])

  // Drop late answers after unmount.
  useEffect(
    () => () => {
      searchSeq.current++
    },
    [],
  )

  // With the switch off and nothing listed, learn whether archived
  // investigations exist so the empty line can point to the switch.
  const listEmpty = listStatus === 'ready' && investigations.length === 0
  const [archivedExist, setArchivedExist] = useState<boolean | null>(null)
  useEffect(() => {
    if (!listEmpty || includeArchived) {
      setArchivedExist(null)
      return
    }
    let cancelled = false
    api.listInvestigations({ include_archived: true, limit: 1 }).then(
      (list) => {
        if (!cancelled) setArchivedExist(list.length > 0)
      },
      () => {
        if (!cancelled) setArchivedExist(false)
      },
    )
    return () => {
      cancelled = true
    }
  }, [listEmpty, includeArchived, dataVersion])

  const selectedIndex = investigations.findIndex((i) => i.ref === selectedRef)
  const roving = useRovingFocus(investigations.length, selectedIndex)
  const showResults = searching && result !== null

  const clearQuery = () => {
    setQuery('')
    fieldRef.current?.focus()
  }

  const onFieldKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Escape' && query !== '' && !hasOpenLayer()) {
      e.preventDefault()
      setQuery('')
    } else if (e.key === 'ArrowDown' && !e.altKey) {
      e.preventDefault()
      if (showResults) resultsRef.current?.focus()
      else roving.focusCurrent()
    }
  }

  let emptyLine: string | null = null
  if (listEmpty) {
    if (includeArchived || archivedExist === false) emptyLine = 'No cases yet.'
    else if (archivedExist) emptyLine = 'No open cases. Turn on Show archived to see archived ones.'
  }

  return (
    <nav className="sidebar" aria-label="Cases">
      <div className="sidebar-header">
        <p className="sidebar-wordmark">Casebook</p>
        <Button variant="secondary" icon={Plus} fullWidth onClick={onNew}>
          New case
        </Button>
      </div>
      <div className="sidebar-tools">
        <div className={['sidebar-search', searchError ? 'is-invalid' : ''].filter(Boolean).join(' ')}>
          <Search className="sidebar-search-icon" size={16} strokeWidth={1.75} aria-hidden="true" />
          <input
            ref={fieldRef}
            id={SEARCH_FIELD_ID}
            type="search"
            className="sidebar-search-input"
            placeholder={SEARCH_LABEL}
            aria-label={SEARCH_LABEL}
            aria-invalid={searchError ? true : undefined}
            aria-describedby={searchError ? errorId : undefined}
            autoComplete="off"
            spellCheck={false}
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onKeyDown={onFieldKeyDown}
          />
          {query === '' ? (
            <kbd className="sidebar-search-kbd" aria-hidden="true">
              /
            </kbd>
          ) : (
            <IconButton icon={X} label="Clear search" className="sidebar-search-clear" onClick={clearQuery} />
          )}
        </div>
        {searchError && (
          <p className="sidebar-search-error" id={errorId}>
            <CircleAlert size={16} strokeWidth={1.75} aria-hidden="true" />
            <span>{searchError}</span>
          </p>
        )}
        <Switch label="Show archived" checked={includeArchived} onChange={setIncludeArchived} />
      </div>
      <div className="sidebar-list" aria-busy={loading || undefined}>
        {showResults && result ? (
          <SearchResults query={result.query} hits={result.hits} handle={resultsRef} />
        ) : (
          <>
            {showSkeleton && (
              <ul className="sidebar-rows" aria-hidden="true">
                {[0, 1, 2].map((i) => (
                  <li key={i} className="inv-skeleton">
                    <span className="inv-skeleton-bar inv-skeleton-title" />
                    <span className="inv-skeleton-bar inv-skeleton-meta" />
                    <span className="inv-skeleton-bar inv-skeleton-meta" />
                  </li>
                ))}
              </ul>
            )}
            {emptyLine && <p className="sidebar-empty">{emptyLine}</p>}
            {listStatus === 'error' && (
              <div className="sidebar-load-error" role="alert">
                <p className="sidebar-search-error">
                  <CircleAlert size={16} strokeWidth={1.75} aria-hidden="true" />
                  <span>{LIST_ERROR_COPY}</span>
                </p>
                <Button variant="secondary" busy={retrying} busyLabel="Trying again..." onClick={() => void retryList()}>
                  Try again
                </Button>
              </div>
            )}
            {investigations.length >= LIST_LIMIT && <p className="sidebar-empty">{LIST_CAPPED_COPY}</p>}
            {investigations.length > 0 && (
              <ul className="sidebar-rows" aria-label="Case list" onKeyDown={roving.onKeyDown}>
                {investigations.map((inv, i) => (
                  <InvestigationRow
                    key={inv.ref}
                    investigation={inv}
                    selected={inv.ref === selectedRef}
                    onSelect={(ref) => select(ref)}
                    now={now}
                    roving={roving.itemProps(i)}
                  />
                ))}
              </ul>
            )}
          </>
        )}
      </div>
      <SidebarFooter />
    </nav>
  )
}
