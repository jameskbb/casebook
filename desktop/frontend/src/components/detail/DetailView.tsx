// The investigation detail (UI-05, D-12): sticky header, then the reading
// column in this exact order: the question, the current conclusion, What we
// believe, What might explain it, Evidence, Still open, History.
//
// Reloads keep the previous content on screen (the column is never keyed on
// dataVersion, so scroll position and open editors survive). A first load
// shows nothing for 300 ms, then the header plus skeleton blocks.
//
// Drill-down (D-21, UI-15): the column sits inside a DrawerStackProvider.
// Claim, hypothesis and evidence rows, evidence chips and E, C and H refs in
// History open the drawer stack. A search hit's record (pendingOpen) opens
// its drawer once the matching detail is loaded; a question hit focuses its
// row instead.
//
// Adding and questions (UI-07, UI-08, UI-10, UI-11): each section's add
// action opens its dialog; open questions offer Resolve, Defer and Edit
// question, resolved and deferred ones Reopen. Focus moves to a new row.
import { useEffect, useRef, useState } from 'react'
import { CircleQuestionMark, FileText, Lightbulb, Pencil, Quote, Trash2 } from 'lucide-react'
import { api } from '../../api/api'
import { cleanMessage } from '../../api/errors'
import type { Claim, Evidence, Hypothesis, InvestigationDetail, Question } from '../../api/types'
import { useDelayedFlag } from '../../hooks/useDelayedFlag'
import { openHypotheses, partitionClaims, partitionHypotheses, partitionQuestions } from '../../lib/records'
import { useAppData } from '../../state/appData'
import { useRequestNavigate } from '../../state/dirtyGuard'
import { useWrite } from '../../state/useWrite'
import { drawerKind, DrawerStackProvider, useDrawers } from '../drawers/DrawerStack'
import { DeferQuestionDialog } from '../dialogs/DeferQuestionDialog'
import { DeleteDialog } from '../dialogs/DeleteDialog'
import { EditInvestigationDialog } from '../dialogs/EditInvestigationDialog'
import { EvidenceDialog } from '../dialogs/EvidenceDialog'
import { QuestionDialog } from '../dialogs/QuestionDialog'
import { ResolveQuestionDialog } from '../dialogs/ResolveQuestionDialog'
import { StatementDialog } from '../dialogs/StatementDialog'
import { Banner } from '../ui/Banner'
import { Button } from '../ui/Button'
import { IconButton } from '../ui/IconButton'
import { LabelsLegend } from '../record/LabelsLegend'
import { useToast } from '../ui/Toaster'
import { ActivityList } from './ActivityList'
import { ArchivedBanner } from './ArchivedBanner'
import { ConclusionPanel } from './ConclusionPanel'
import { DetailHeader } from './DetailHeader'
import { EvidenceRow } from './EvidenceRow'
import { caseHasNoRecords, FirstFindingGuide } from './FirstFindingGuide'
import { QuestionBlock } from './QuestionBlock'
import { QuestionRow } from './QuestionRow'
import { RecordRow } from './RecordRow'
import { ReviewFindings } from './ReviewFindings'
import { SectionList } from './SectionList'
import './DetailView.css'

export interface DetailViewProps {
  focusTitle?: boolean
  onTitleFocused?: () => void
}

export function DetailView({ focusTitle = false, onTitleFocused }: DetailViewProps) {
  const { detail, detailStatus, detailError, investigations, selectedRef } = useAppData()
  const showSkeleton = useDelayedFlag(!detail && detailStatus === 'loading')
  const [editOpen, setEditOpen] = useState(false)
  const [reviewRef, setReviewRef] = useState<string | null>(null)
  const requestNavigate = useRequestNavigate()

  if (!detail) {
    if (detailStatus === 'error' && detailError) {
      return (
        <div className="detail detail-message">
          <p>
            {detailError.kind === 'not_found' && selectedRef
              ? DETAIL_GONE_COPY(selectedRef)
              : cleanMessage(detailError.message)}
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
        {reviewRef === inv.ref ? <ReviewFindings key={inv.ref} detail={detail} onClose={() => setReviewRef(null)} /> : <div className="reading-column">
          {detailStatus === 'error' && <StaleDetailBanner />}
          {inv.archived && <ArchivedBanner investigationRef={inv.ref} />}
          <QuestionBlock investigation={inv} onEdit={() => setEditOpen(true)} actions={<Button variant="secondary" onClick={() => requestNavigate(() => setReviewRef(inv.ref), 'detail')}>Review findings</Button>} />
          <ConclusionPanel key={inv.ref} investigation={inv} claims={detail.claims} />
          <ReadingSections detail={detail} />
        </div>}
        {editOpen && <EditInvestigationDialog investigation={inv} onClose={() => setEditOpen(false)} />}
      </div>
    </DrawerStackProvider>
  )
}

export const deleteQuestionBody = (q: Question) =>
  `${q.ref} will be removed from this case${q.status === 'resolved' ? ' with its answer' : ''}. Use this for a question asked by mistake; to set one aside, defer it instead.`

export const DETAIL_GONE_COPY = (ref: string) =>
  `Casebook could not find ${ref}. It may have changed in another app. Choose a case from the list.`
export const DETAIL_STALE_COPY = 'Casebook could not reload this case. Showing the last loaded version.'

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

/**
 * Consumes pendingOpen (a search hit's record) once the matching detail is
 * loaded: E, C and H refs open their drawer, a question focuses its row.
 */
function usePendingOpen(detail: InvestigationDetail) {
  const { pendingOpen, clearPendingOpen, selectedRef } = useAppData()
  const { open } = useDrawers()
  const invRef = detail.investigation.ref
  useEffect(() => {
    if (!pendingOpen || invRef !== selectedRef) return
    if (drawerKind(pendingOpen)) {
      open(pendingOpen)
    } else {
      const row = document.querySelector<HTMLElement>(`[data-ref="${pendingOpen}"]`)
      if (row) {
        row.focus()
        row.scrollIntoView?.({ block: 'center' })
      } else {
        // A resolved question sits behind its disclosure.
        document.querySelector<HTMLElement>('[aria-labelledby="section-questions"] .section-disclosure')?.focus()
      }
    }
    clearPendingOpen()
  }, [pendingOpen, invRef, selectedRef, open, clearPendingOpen])
}

type RecordDialog =
  | { type: 'evidence' | 'claim' | 'hypothesis' | 'question' }
  | { type: 'editQuestion' | 'resolve' | 'defer' | 'delete'; question: Question }

/**
 * The record sections plus their add and question dialogs. After a record is
 * added (or a question changes state) focus moves to its row; a question
 * that moved behind its disclosure leaves focus on the disclosure.
 */
function ReadingSections({ detail }: { detail: InvestigationDetail }) {
  const { open } = useDrawers()
  const write = useWrite()
  const toast = useToast()
  usePendingOpen(detail)
  const invRef = detail.investigation.ref
  const [dialog, setDialog] = useState<RecordDialog | null>(null)
  const [focusRef, setFocusRef] = useState<string | null>(null)
  const [reopening, setReopening] = useState<string | null>(null)
  const [legendOpen, setLegendOpen] = useState(false)
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

  const questionActions = (q: Question) =>
    q.status === 'open' ? (
      <>
        <Button variant="ghost" onClick={() => setDialog({ type: 'resolve', question: q })}>
          Resolve
        </Button>
        <Button variant="ghost" onClick={() => setDialog({ type: 'defer', question: q })}>
          Defer
        </Button>
        <IconButton icon={Pencil} label="Edit question" onClick={() => setDialog({ type: 'editQuestion', question: q })} />
        <IconButton icon={Trash2} label={`Delete question ${q.ref}`} onClick={() => setDialog({ type: 'delete', question: q })} />
      </>
    ) : (
      <>
        <Button variant="ghost" busy={reopening === q.ref} busyLabel="Reopening..." onClick={() => void reopen(q)}>
          Reopen
        </Button>
        <IconButton icon={Trash2} label={`Delete question ${q.ref}`} onClick={() => setDialog({ type: 'delete', question: q })} />
      </>
    )

  const claims = partitionClaims(detail.claims)
  const hypotheses = partitionHypotheses(detail.hypotheses)
  const questions = partitionQuestions(detail.questions)
  const stillOpen = openHypotheses(detail.hypotheses)
  const close = () => setDialog(null)
  return (
    <div className="reading-sections" ref={sectionsRef}>
      {caseHasNoRecords(detail) && !detail.investigation.archived && <FirstFindingGuide onAddEvidence={() => setDialog({ type: 'evidence' })} />}
      <SectionList<Claim>
        id="claims"
        icon={Quote}
        kind="claim"
        heading="What we believe"
        addLabel="Add claim"
        onAdd={() => setDialog({ type: 'claim' })}
        headingExtra={
          <button type="button" className="text-button" onClick={() => setLegendOpen(true)}>
            What the labels mean
          </button>
        }
        emptyText="No claims yet. Add one when the evidence supports a clear statement."
        items={claims.visible}
        ruledOut={claims.ruledOut}
        ruledOutNoun={['superseded or rejected claim', 'superseded or rejected claims']}
        itemKey={(c) => c.ref}
        renderItem={(c) => <RecordRow kind="claim" record={c} onOpen={open} />}
      />
      <SectionList<Hypothesis>
        id="hypotheses"
        icon={Lightbulb}
        kind="hypothesis"
        heading="What might explain it"
        addLabel="Add hypothesis"
        onAdd={() => setDialog({ type: 'hypothesis' })}
        emptyText="No hypotheses yet. Add one to test a possible explanation."
        items={hypotheses.visible}
        ruledOut={hypotheses.ruledOut}
        ruledOutNoun={['ruled-out hypothesis', 'ruled-out hypotheses']}
        itemKey={(h) => h.ref}
        renderItem={(h) => <RecordRow kind="hypothesis" record={h} onOpen={open} />}
      />
      <SectionList<Evidence>
        id="evidence"
        icon={FileText}
        kind="evidence"
        heading="Evidence"
        addLabel="Add evidence"
        onAdd={() => setDialog({ type: 'evidence' })}
        emptyText="No evidence yet. Add what you found and where you found it."
        items={detail.evidence}
        itemKey={(e) => e.ref}
        renderItem={(e) => <EvidenceRow evidence={e} detail={detail} onOpen={open} />}
      />
      <SectionList<Question>
        id="questions"
        icon={CircleQuestionMark}
        kind="question"
        heading="Still open"
        addLabel="Add question"
        onAdd={() => setDialog({ type: 'question' })}
        emptyText="Nothing is open. Add a question when something still needs an answer."
        emptyWithRuledOut
        lead={
          stillOpen.length > 0 ? (
            <>
              <div className="section-lead">
                <h3 className="section-subheading" id="still-open-explanations">Explanations still open</h3>
                <ul className="section-rows" aria-labelledby="still-open-explanations">
                  {stillOpen.map((h) => (
                    <li key={h.ref}>
                      <RecordRow kind="hypothesis" record={h} onOpen={open} compact />
                    </li>
                  ))}
                </ul>
              </div>
              {questions.visible.length > 0 && <h3 className="section-subheading section-subheading-rows">Questions</h3>}
            </>
          ) : undefined
        }
        items={questions.visible}
        ruledOut={questions.ruledOut}
        ruledOutNoun={['resolved question', 'resolved questions']}
        itemKey={(q) => q.ref}
        renderItem={(q) => <QuestionRow question={q} actions={questionActions(q)} />}
      />
      <ActivityList activities={detail.recent_activity} onOpenRef={open} conclusionReviewedAt={detail.investigation.conclusion_reviewed_at} />

      {legendOpen && <LabelsLegend onClose={() => setLegendOpen(false)} />}

      {dialog?.type === 'evidence' && (
        <EvidenceDialog mode="add" invRef={invRef} onClose={close} onSaved={(e) => setFocusRef(e.ref)} />
      )}
      {(dialog?.type === 'claim' || dialog?.type === 'hypothesis') && (
        <StatementDialog
          kind={dialog.type}
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
          title={`Delete question ${dialog.question.ref}?`}
          body={deleteQuestionBody(dialog.question)}
          confirmLabel="Delete question"
          onConfirm={async () => {
            const ref = dialog.question.ref
            const done = await write(() => api.deleteQuestion(invRef, ref).then(() => true), { rethrowConflict: true })
            if (done) toast.success(`Deleted question ${ref}.`)
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
          <span className="detail-header-ref tabular">{refText}</span>
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
