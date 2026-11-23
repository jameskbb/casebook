// The question page: sticky header, then the reading column in this exact
// order: the question, the answer, the add buttons, What backs this up,
// Where sources disagree, the ruled out and replaced findings (each behind
// one line), Still to find out, Other sources, How we got here. A section
// with nothing in it is not drawn. Findings (claims and hypotheses) are
// grouped by what happened to them and each one lists its own sources; Other
// sources holds the ones no finding on the page uses.
//
// Reloads keep the previous content on screen (the column is never keyed on
// dataVersion, so scroll position and open editors survive). A first load
// shows nothing for 300 ms, then the header plus skeleton blocks.
//
// Drill-down: the column sits inside a DrawerStackProvider. Finding rows,
// the sources nested under them, source rows and the records named in How we
// got here open the drawer stack. pendingOpen says what to open once the
// matching detail is loaded: a search hit's record opens its drawer or
// focuses its row; Home's buttons open the check (Decide opens it too: its
// sources are all checked, so it starts on the findings where sources
// disagree), the answer editor or the answer check. When the check closes,
// focus goes back to the answer's first button.
//
// Still to find out holds findings not tested yet and the things a person
// or an AI still wants to find out, in one list. A finding not tested yet
// offers This turned out right and Rule this out on its row; something to
// find out offers Answer and Later. Answered ones collapse under Show N
// answered. Focus moves to a new row.
import { useEffect, useRef, useState } from 'react'
import { CircleAlert, CircleQuestionMark, FileText, Pencil, Plus, Quote, Trash2 } from 'lucide-react'
import { api } from '../../api/api'
import { plainError, toApiError } from '../../api/errors'
import type { Evidence, Hypothesis, InvestigationDetail, Question } from '../../api/types'
import { useDelayedFlag } from '../../hooks/useDelayedFlag'
import { evidenceByRef, groupFindings, partitionQuestions, type Finding } from '../../lib/records'
import { useAppData } from '../../state/appData'
import { useWrite } from '../../state/useWrite'
import { drawerKind, DrawerStackProvider, useDrawers } from '../drawers/DrawerStack'
import { RULE_OUT_ACTION, TURNED_OUT_RIGHT_ACTION } from '../drawers/StatementDrawer'
import { DeferQuestionDialog } from '../dialogs/DeferQuestionDialog'
import { DeleteDialog } from '../dialogs/DeleteDialog'
import { EditInvestigationDialog } from '../dialogs/EditInvestigationDialog'
import { EvidenceDialog } from '../dialogs/EvidenceDialog'
import { ADD_TO_FIND_OUT, QuestionDialog } from '../dialogs/QuestionDialog'
import { ResolveQuestionDialog } from '../dialogs/ResolveQuestionDialog'
import { StatementDialog } from '../dialogs/StatementDialog'
import { Banner } from '../ui/Banner'
import { Button } from '../ui/Button'
import { IconButton } from '../ui/IconButton'
import { LabelsLegend } from '../record/LabelsLegend'
import { useToast } from '../ui/Toaster'
import { ActivityList } from './ActivityList'
import { ArchivedBanner } from './ArchivedBanner'
import { CheckSources } from './CheckSources'
import { ConclusionPanel } from './ConclusionPanel'
import { DetailHeader } from './DetailHeader'
import { RecordId } from '../record/RecordId'
import { EvidenceRow } from './EvidenceRow'
import { ADD_FINDING, ADD_SOURCE, caseHasNoRecords, FirstFindingGuide } from './FirstFindingGuide'
import { QuestionBlock } from './QuestionBlock'
import { QuestionRow } from './QuestionRow'
import { RecordRow } from './RecordRow'
import { Collapsed, SectionList } from './SectionList'
import './DetailView.css'

export interface DetailViewProps {
  focusTitle?: boolean
  onTitleFocused?: () => void
}

export function DetailView({ focusTitle = false, onTitleFocused }: DetailViewProps) {
  const { detail, detailStatus, detailError, investigations, selectedRef } = useAppData()
  const showSkeleton = useDelayedFlag(!detail && detailStatus === 'loading')
  const [editOpen, setEditOpen] = useState(false)

  if (!detail) {
    if (detailStatus === 'error' && detailError) {
      return (
        <div className="detail detail-message">
          <p>
            {detailError.kind === 'not_found' && selectedRef
              ? DETAIL_GONE_COPY
              : plainError(detailError)}
          </p>
        </div>
      )
    }
    if (showSkeleton) {
      const summary = investigations.find((i) => i.ref === selectedRef)
      return <DetailSkeleton refText={summary?.ref ?? selectedRef ?? ''} title={summary?.title ?? ''} />
    }
    return <div className="detail" aria-busy={detailStatus === 'loading' || undefined} />
  }

  const inv = detail.investigation
  return (
    <DrawerStackProvider>
      <div className="detail">
        <DetailHeader investigation={inv} focusTitle={focusTitle} onTitleFocused={onTitleFocused} />
        <div className="reading-column">
          {detailStatus === 'error' && <StaleDetailBanner />}
          {inv.archived && <ArchivedBanner investigationRef={inv.ref} />}
          <QuestionBlock investigation={inv} onEdit={() => setEditOpen(true)} />
          <Page key={inv.ref} detail={detail} />
        </div>
        {editOpen && <EditInvestigationDialog investigation={inv} onClose={() => setEditOpen(false)} />}
      </div>
    </DrawerStackProvider>
  )
}

type AnswerIntent = 'write-answer' | 'check-answer'

/** The answer, the sections and the source check, for one question. */
function Page({ detail }: { detail: InvestigationDetail }) {
  const { open } = useDrawers()
  const { pendingOpen, clearPendingOpen, selectedRef } = useAppData()
  const inv = detail.investigation
  const [checking, setChecking] = useState(false)
  const [answerIntent, setAnswerIntent] = useState<AnswerIntent | null>(null)
  const [focusAnswer, setFocusAnswer] = useState(false)

  // After the check closes (Escape, Done, Not now) focus goes to the answer's
  // first button, Check the next source while one is left: whatever opened
  // the window may be gone by then.
  useEffect(() => {
    if (!focusAnswer) return
    setFocusAnswer(false)
    const panel = document.querySelector('.conclusion-panel')
    ;(panel?.querySelector<HTMLElement>('.btn-primary') ?? panel?.querySelector<HTMLElement>('button'))?.focus()
  }, [focusAnswer])

  // Consumes pendingOpen once the matching detail is loaded.
  useEffect(() => {
    if (!pendingOpen || inv.ref !== selectedRef) return
    if (pendingOpen === 'check-sources' || pendingOpen === 'decide') {
      setChecking(true)
    } else if (pendingOpen === 'write-answer' || pendingOpen === 'check-answer') {
      setAnswerIntent(pendingOpen)
    } else if (drawerKind(pendingOpen)) {
      open(pendingOpen)
    } else {
      const row = document.querySelector<HTMLElement>(`[data-ref="${pendingOpen}"]`)
      if (row) {
        row.focus()
        row.scrollIntoView?.({ block: 'center' })
      } else {
        // An answered one sits behind its Show N answered line.
        document.querySelector<HTMLElement>('[aria-labelledby="section-questions"] .section-disclosure')?.focus()
      }
    }
    clearPendingOpen()
  }, [pendingOpen, inv.ref, selectedRef, open, clearPendingOpen, detail])

  return (
    <>
      <ConclusionPanel
        investigation={inv}
        claims={detail.claims}
        evidence={detail.evidence}
        onCheckSources={() => setChecking(true)}
        share
        bare={caseHasNoRecords(detail)}
        intent={answerIntent}
        onIntentDone={() => setAnswerIntent(null)}
      />
      <ReadingSections detail={detail} />
      {checking && (
        <CheckSources
          detail={detail}
          onClose={() => {
            setChecking(false)
            setFocusAnswer(true)
          }}
          onWriteAnswer={() => {
            setChecking(false)
            setAnswerIntent('write-answer')
          }}
        />
      )}
    </>
  )
}

export const DETAIL_GONE_COPY = 'Casebook could not find this question. Your AI may have changed it. Choose a question from the list.'
export const DETAIL_STALE_COPY = 'Casebook could not load this question again. This is how it looked last time.'

/** Shown when a background reload failed and the last loaded detail stays on screen. */
function StaleDetailBanner() {
  const { refresh } = useAppData()
  const [busy, setBusy] = useState(false)
  const retry = async () => {
    setBusy(true)
    try {
      await refresh()
    } finally {
      setBusy(false)
    }
  }
  return (
    <Banner
      tone="warning"
      action={
        <Button variant="secondary" busy={busy} busyLabel="Trying again..." onClick={() => void retry()}>
          Try again
        </Button>
      }
    >
      {DETAIL_STALE_COPY}
    </Banner>
  )
}

export const OTHER_SOURCES_HEADING = 'Other sources'
export const LABELS_LINK = 'What the labels mean'

/** A row under Still to find out: a finding not tested yet, or something to find out. */
type ToFind = { finding: Finding } | { question: Question }

type RecordDialog =
  | { type: 'evidence' | 'claim' | 'question' }
  | { type: 'editQuestion' | 'resolve' | 'defer' | 'delete'; question: Question }

/**
 * The add buttons, the sections and their dialogs. After something is added
 * (or changes state) focus moves to its row; one that moved behind its Show
 * line leaves focus on that line.
 */
function ReadingSections({ detail }: { detail: InvestigationDetail }) {
  const { open } = useDrawers()
  const write = useWrite()
  const toast = useToast()
  const invRef = detail.investigation.ref
  const [dialog, setDialog] = useState<RecordDialog | null>(null)
  const [focusRef, setFocusRef] = useState<string | null>(null)
  const [reopening, setReopening] = useState<string | null>(null)
  const [legendOpen, setLegendOpen] = useState(false)
  const [testing, setTesting] = useState<string | null>(null)
  const sectionsRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!focusRef) return
    const root = sectionsRef.current
    const row = root?.querySelector<HTMLElement>(`[data-ref="${focusRef}"]`)
    if (row) {
      const target = row.querySelector<HTMLElement>('.row-hit') ?? row
      target.focus()
      row.scrollIntoView?.({ block: 'nearest' })
      setFocusRef(null)
    } else if (focusRef.startsWith('Q-')) {
      root?.querySelector<HTMLElement>('[aria-labelledby="section-questions"] .section-disclosure')?.focus()
      setFocusRef(null)
    }
  }, [focusRef, detail])

  const reopen = async (q: Question) => {
    if (reopening) return
    setReopening(q.ref)
    try {
      const saved = await write(() => api.reopenQuestion(invRef, q.ref))
      if (saved) setFocusRef(q.ref)
    } catch {
      // Reopen has no fields; useWrite already reported anything else.
    } finally {
      setReopening(null)
    }
  }

  // A finding not tested yet: it turned out right, or it is ruled out.
  const settle = async (h: Hypothesis, status: 'supported' | 'disproven') => {
    if (testing) return
    setTesting(h.ref)
    try {
      await write(() => api.updateHypothesis(invRef, h.ref, { status }))
    } catch (e) {
      toast.error(plainError(toApiError(e)))
    } finally {
      setTesting(null)
    }
  }
  const testActions = (h: Hypothesis) => (
    <>
      <Button variant="ghost" disabled={testing === h.ref} onClick={() => void settle(h, 'supported')}>{TURNED_OUT_RIGHT_ACTION}</Button>
      <Button variant="ghost" disabled={testing === h.ref} onClick={() => void settle(h, 'disproven')}>{RULE_OUT_ACTION}</Button>
    </>
  )

  const deleteButton = (q: Question) => (
    <IconButton icon={Trash2} label={`Delete: ${q.question}`} onClick={() => setDialog({ type: 'delete', question: q })} />
  )
  const questionActions = (q: Question) =>
    q.status === 'resolved' ? (
      <>
        <Button variant="ghost" busy={reopening === q.ref} busyLabel="Asking again..." onClick={() => void reopen(q)}>
          Ask again
        </Button>
        {deleteButton(q)}
      </>
    ) : (
      <>
        <Button variant="ghost" onClick={() => setDialog({ type: 'resolve', question: q })}>
          Answer
        </Button>
        {q.status === 'open' && (
          <>
            <Button variant="ghost" onClick={() => setDialog({ type: 'defer', question: q })}>
              Later
            </Button>
            <IconButton icon={Pencil} label={`Edit: ${q.question}`} onClick={() => setDialog({ type: 'editQuestion', question: q })} />
          </>
        )}
        {deleteButton(q)}
      </>
    )

  const sources = evidenceByRef(detail)
  const findings = groupFindings(detail.claims, detail.hypotheses, sources)
  // Sources under a finding are read there; the rest are listed on their own,
  // and so is one whose only findings sit behind Show replaced findings.
  const hidden = new Set(findings.replaced.map((f) => f.record.ref))
  const otherSources = detail.evidence.filter((e) => [...e.claim_links, ...e.hypothesis_links].every((l) => hidden.has(l.ref)))
  const names = new Map<string, string>([
    ...detail.evidence.map((e) => [e.ref, e.title] as const),
    ...detail.claims.map((c) => [c.ref, c.statement] as const),
    ...detail.hypotheses.map((h) => [h.ref, h.statement] as const),
    ...detail.questions.map((q) => [q.ref, q.question] as const),
  ])
  const questions = partitionQuestions(detail.questions)
  const close = () => setDialog(null)
  const findingRow = (f: Finding) => <RecordRow {...f} sources={sources} onOpen={open} />
  const findingKey = (f: Finding) => f.record.ref
  const toFind: ToFind[] = [...findings.notTested.map((finding) => ({ finding })), ...questions.visible.map((question) => ({ question }))]
  return (
    <div className="reading-sections" ref={sectionsRef}>
      {caseHasNoRecords(detail) ? (
        <FirstFindingGuide onAddSource={() => setDialog({ type: 'evidence' })} onAddFinding={() => setDialog({ type: 'claim' })} />
      ) : (
        <div className="reading-add" role="group" aria-label="Add to this question">
          <Button variant="secondary" icon={Plus} onClick={() => setDialog({ type: 'claim' })}>{ADD_FINDING}</Button>
          <Button variant="secondary" icon={Plus} onClick={() => setDialog({ type: 'evidence' })}>{ADD_SOURCE}</Button>
          <Button variant="secondary" icon={Plus} onClick={() => setDialog({ type: 'question' })}>{ADD_TO_FIND_OUT}</Button>
          <button type="button" className="text-button reading-add-legend" onClick={() => setLegendOpen(true)}>
            {LABELS_LINK}
          </button>
        </div>
      )}
      <SectionList<Finding> id="findings" icon={Quote} kind="claim" heading="What backs this up" items={findings.backing} itemKey={findingKey} renderItem={findingRow} />
      <SectionList<Finding> id="disagree" icon={CircleAlert} kind="claim" heading="Where sources disagree" items={findings.disagree} itemKey={findingKey} renderItem={findingRow} />
      <Collapsed items={findings.ruledOut} noun={['ruled out finding', 'ruled out findings']} itemKey={findingKey} renderItem={findingRow} />
      <Collapsed items={findings.replaced} noun={['replaced finding', 'replaced findings']} itemKey={findingKey} renderItem={findingRow} />
      <SectionList<ToFind>
        id="questions"
        icon={CircleQuestionMark}
        kind="question"
        heading="Still to find out"
        items={toFind}
        collapsed={questions.ruledOut.map((question) => ({ question }))}
        collapsedNoun={['answered', 'answered']}
        itemKey={(x) => ('finding' in x ? findingKey(x.finding) : x.question.ref)}
        renderItem={(x) =>
          'finding' in x ? (
            <RecordRow {...x.finding} sources={sources} onOpen={open} actions={x.finding.kind === 'hypothesis' && testActions(x.finding.record)} />
          ) : (
            <QuestionRow question={x.question} actions={questionActions(x.question)} />
          )
        }
      />
      <SectionList<Evidence>
        id="evidence"
        icon={FileText}
        kind="evidence"
        heading={otherSources.length < detail.evidence.length ? OTHER_SOURCES_HEADING : 'Sources'}
        items={otherSources}
        itemKey={(e) => e.ref}
        renderItem={(e) => <EvidenceRow evidence={e} onOpen={open} />}
      />
      <ActivityList
        activities={detail.recent_activity}
        onOpenRef={open}
        nameOf={(ref) => names.get(ref)}
        conclusionReviewedAt={detail.investigation.conclusion_reviewed_at}
      />

      {legendOpen && <LabelsLegend onClose={() => setLegendOpen(false)} />}

      {dialog?.type === 'evidence' && (
        <EvidenceDialog mode="add" invRef={invRef} onClose={close} onSaved={(e) => setFocusRef(e.ref)} />
      )}
      {dialog?.type === 'claim' && (
        <StatementDialog
          invRef={invRef}
          evidence={detail.evidence}
          onClose={close}
          onCreated={setFocusRef}
        />
      )}
      {dialog?.type === 'question' && <QuestionDialog invRef={invRef} onClose={close} onCreated={setFocusRef} />}
      {dialog?.type === 'editQuestion' && (
        <QuestionDialog invRef={invRef} question={dialog.question} onClose={close} />
      )}
      {dialog?.type === 'resolve' && (
        <ResolveQuestionDialog
          invRef={invRef}
          question={dialog.question}
          onClose={() => {
            close()
            setFocusRef(dialog.question.ref)
          }}
        />
      )}
      {dialog?.type === 'delete' && (
        <DeleteDialog
          title="Delete this?"
          confirmLabel="Delete"
          onConfirm={async () => {
            const ref = dialog.question.ref
            const done = await write(() => api.deleteQuestion(invRef, ref).then(() => true), { rethrowConflict: true })
            if (done) toast.success('Deleted it.')
            return Boolean(done)
          }}
          onClose={close}
        />
      )}
      {dialog?.type === 'defer' && (
        <DeferQuestionDialog
          invRef={invRef}
          question={dialog.question}
          onClose={() => {
            close()
            setFocusRef(dialog.question.ref)
          }}
        />
      )}
    </div>
  )
}

/** The header plus quiet skeleton blocks, shown only after 300 ms. */
function DetailSkeleton({ refText, title }: { refText: string; title: string }) {
  return (
    <div className="detail detail-skeleton" aria-busy="true">
      <header className="detail-header">
        <div className="detail-header-left">
          <RecordId>{refText}</RecordId>
          <span className="detail-header-title">{title}</span>
        </div>
      </header>
      <div className="reading-column" aria-hidden="true">
        <div className="skeleton-group">
          <span className="skeleton-bar skeleton-label" />
          <span className="skeleton-bar skeleton-display" />
          <span className="skeleton-bar skeleton-display skeleton-short" />
        </div>
        <div className="skeleton-panel" />
        {[0, 1].map((i) => (
          <div className="skeleton-group" key={i}>
            <span className="skeleton-bar skeleton-heading" />
            <span className="skeleton-bar" />
            <span className="skeleton-bar skeleton-short" />
          </div>
        ))}
      </div>
    </div>
  )
}
