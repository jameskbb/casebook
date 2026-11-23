// Finding drawer body (a claim or a hypothesis):
//   1. the finding in Literata 20 under its one label, why the person thinks
//      so, and "Edit finding", which opens both in an inline editor;
//   2. what a person can decide about it. A finding has no check of its own:
//      its label comes from its sources. Where sources disagree: It still
//      holds (after a confirmation) or Rule this out. Any finding on the
//      page: Rule this out and Replace with a newer finding. One not tested
//      yet: also This turned out right. A replaced or ruled out finding:
//      Bring this back;
//   3. the sources that back it up and go against it (each opens the source
//      drawer), then Add a source to this finding.
//
// Every write goes through useWrite. Saves show no toast; the group simply
// shows the saved value. Edit-in-place safety: the editor snapshots the
// stored values; a reload never touches unsaved input, and when the stored
// values changed meanwhile the group says so and offers the new version.
// While it holds unsaved input, Esc discards that input first; the next Esc
// closes the drawers.
import { useCallback, useEffect, useId, useRef, useState, type KeyboardEvent, type ReactNode, type RefObject } from 'react'
import { Link as LinkIcon, Pencil, Replace, Trash2, Unlink } from 'lucide-react'
import { api } from '../../api/api'
import { ApiError, errorCopy, toApiError } from '../../api/errors'
import type {
  Claim,
  ClaimPatch,
  ClaimStatus,
  Evidence,
  Hypothesis,
  HypothesisPatch,
  HypothesisStatus,
  InvestigationDetail,
  Relation,
} from '../../api/types'
import { useChangedElsewhere } from '../../hooks/useChangedElsewhere'
import { LIMITS, nearLimit } from '../../lib/bytes'
import { splitFieldErrors } from '../../lib/formErrors'
import { pushLayer } from '../../lib/layers'
import { evidenceByRef, isReplaced, isRuledOut, sourcesDisagree, type Finding } from '../../lib/records'
import { useAppData } from '../../state/appData'
import { useDirtyEditor } from '../../state/dirtyGuard'
import { useWrite } from '../../state/useWrite'
import { ImportedTag } from '../record/ImportedTag'
import { RecordId } from '../record/RecordId'
import { FindingLabel } from '../record/StatusLabel'
import { DeleteDialog } from '../dialogs/DeleteDialog'
import { EvidenceDialog } from '../dialogs/EvidenceDialog'
import { LinkEvidenceDialog } from '../dialogs/LinkEvidenceDialog'
import { ReplaceFindingDialog } from '../dialogs/ReplaceFindingDialog'
import { Banner } from '../ui/Banner'
import { Button } from '../ui/Button'
import { ChangedElsewhereBanner, SHOW_NEW_VERSION } from '../ui/ChangedElsewhereBanner'
import { TextArea } from '../ui/TextArea'
import { useToast } from '../ui/Toaster'
import { EvidenceBlock } from './EvidenceBlock'
import './StatementDrawer.css'

export type StatementDrawerProps = {
  detail: InvestigationDetail
  /** Pushes another drawer (evidence, or the replacement record). */
  onOpenRef: (ref: string) => void
  /** Called after the record was deleted (the drawer stack closes). */
  onDeleted?: () => void
} & ({ kind: 'claim'; record: Claim } | { kind: 'hypothesis'; record: Hypothesis })

export const STILL_HOLDS_ACTION = 'It still holds'
export const SETTLE_ACTION = 'Yes, it still holds'
export const SETTLE_COPY = 'The sources disagree about this finding. Choose this only if you read them and the finding still holds.'
export const TURNED_OUT_RIGHT_ACTION = 'This turned out right'
export const RULE_OUT_ACTION = 'Rule this out'
export const REPLACE_ACTION = 'Replace with a newer finding'
export const BRING_BACK_ACTION = 'Bring this back'
export const SUPPORTING_TITLE = 'Sources that back it up'
export const CONTRADICTING_TITLE = 'Sources that go against it'
export const MOVE_TO_AGAINST = 'Move to: goes against it'
export const MOVE_TO_BACKS = 'Move to: backs it up'
export const FINDING_CHANGED_COPY = 'Your AI changed this finding or its sources. Read it again before you decide.'

type Kind = 'claim' | 'hypothesis'
type StatementRecord = Claim | Hypothesis
/** A claim or hypothesis patch; useRecordCalls sends it to the matching call. */
type Patch = Omit<ClaimPatch, 'status'> & Omit<HypothesisPatch, 'status'> & { status?: ClaimStatus | HypothesisStatus }

/** A drawer group: an h3 heading and its content, labelled for assistive technology. */
export function DrawerGroup({
  title,
  children,
  className,
  action,
}: {
  title: string
  children: ReactNode
  className?: string
  /** A control at the right of the heading. */
  action?: ReactNode
}) {
  const id = useId()
  return (
    <section className={['drawer-group', className ?? ''].filter(Boolean).join(' ')} aria-labelledby={id}>
      {action ? (
        <div className="drawer-group-head">
          <h3 className="drawer-group-heading" id={id}>
            {title}
          </h3>
          {action}
        </div>
      ) : (
        <h3 className="drawer-group-heading" id={id}>
          {title}
        </h3>
      )}
      {children}
    </section>
  )
}

/** "Replaced by C-002" where the ref is a text button that opens the replacement. */
export function SupersededBy({ refText, onOpen }: { refText: string; onOpen: (ref: string) => void }) {
  return (
    <p className="superseded-by">
      <button type="button" className="text-button" onClick={() => onOpen(refText)}>
        Open the finding that replaced it <RecordId>{refText}</RecordId>
      </button>
    </p>
  )
}

/** The calls for one claim or hypothesis, by kind. */
function useRecordCalls(kind: Kind, invRef: string, ref: string, basis?: string) {
  const get = (): Promise<StatementRecord> => (kind === 'claim' ? api.getClaim(invRef, ref) : api.getHypothesis(invRef, ref))
  return {
    update: (patch: Patch): Promise<StatementRecord> =>
      kind === 'claim'
        ? api.updateClaim(invRef, ref, patch as ClaimPatch)
        : api.updateHypothesis(invRef, ref, patch as HypothesisPatch),
/** The person read the sources and decided the finding still holds, as it read to them (basis). */
    decideHolds: async (): Promise<StatementRecord> => {
      if (!basis) throw new ApiError('conflict', 'this finding changed; show the new version')
      await api.reviewStatement(invRef, { kind, ref, basis, verification: 'human_verified', settle_dispute: true })
      return get()
    },
    link: (evidenceRef: string, rel: Relation): Promise<StatementRecord> =>
      kind === 'claim'
        ? api.linkEvidenceToClaim(invRef, ref, evidenceRef, rel)
        : api.linkEvidenceToHypothesis(invRef, ref, evidenceRef, rel),
    unlink: (evidenceRef: string): Promise<StatementRecord> =>
      kind === 'claim'
        ? api.unlinkEvidenceFromClaim(invRef, ref, evidenceRef)
        : api.unlinkEvidenceFromHypothesis(invRef, ref, evidenceRef),
  }
}

type Calls = ReturnType<typeof useRecordCalls>

/** While active, Esc runs onEscape instead of closing the drawers. */
function useEscapeLayer(active: boolean, onEscape: () => void) {
  const ref = useRef(onEscape)
  ref.current = onEscape
  useEffect(() => {
    if (!active) return
    return pushLayer(() => ref.current())
  }, [active])
}

export function StatementDrawer(props: StatementDrawerProps) {
  const { kind, record: r, detail, onOpenRef, onDeleted } = props
  const { ownDetail } = useAppData()
  const invRef = detail.investigation.ref
  const latestBasis = detail.review_bases?.find((b) => b.ref === r.ref)?.basis
  const reviewKey = `${invRef}/${r.ref}`
  // What the person has read: the finding and its sources as they stood when
  // the drawer opened. A reload an AI caused must not move it for them (they
  // have to read the new version first); a reload after their own save does,
  // without a word, because they made that change themselves.
  const [reviewSnapshot, setReviewSnapshot] = useState({ key: reviewKey, basis: latestBasis })
  if (reviewSnapshot.key !== reviewKey || (detail === ownDetail && reviewSnapshot.basis !== latestBasis)) {
    setReviewSnapshot({ key: reviewKey, basis: latestBasis })
  }
  const reviewChanged = reviewSnapshot.key === reviewKey && reviewSnapshot.basis !== latestBasis
  const calls = useRecordCalls(kind, invRef, r.ref, reviewSnapshot.key === reviewKey ? reviewSnapshot.basis : undefined)
  const finding = { kind, record: r } as Finding
  const sources = evidenceByRef(detail)
  const disagree = sourcesDisagree(r, sources)
  const [dialog, setDialog] = useState<'link' | 'replace' | 'addEvidence' | null>(null)
  // Evidence added from the link dialog is preselected when it reopens.
  const [linkInitial, setLinkInitial] = useState<string | undefined>()
  const linkButtonRef = useRef<HTMLButtonElement>(null)

  return (
    <>
      <StatementGroup finding={finding} detail={detail} calls={calls} />
      {reviewChanged && disagree && <Banner tone="warning" action={<Button variant="secondary" onClick={() => setReviewSnapshot({ key: reviewKey, basis: latestBasis })}>{SHOW_NEW_VERSION}</Button>}>
        {FINDING_CHANGED_COPY}
      </Banner>}
      <DecideGroup
        kind={kind}
        record={r}
        disagree={disagree}
        calls={calls}
        onOpenRef={onOpenRef}
        onReplace={() => setDialog('replace')}
        reviewChanged={reviewChanged}
      />
      <EvidenceGroups
        record={r}
        detail={detail}
        calls={calls}
        onOpenRef={onOpenRef}
        onLink={() => {
          setLinkInitial(undefined)
          setDialog('link')
        }}
        linkButtonRef={linkButtonRef}
      />
      <DeleteStatement kind={kind} record={r} detail={detail} onDeleted={onDeleted} />
      {dialog === 'link' && (
        <LinkEvidenceDialog
          kind={kind}
          record={r}
          evidence={detail.evidence}
          invRef={invRef}
          initialRef={linkInitial}
          onClose={() => setDialog(null)}
          onAddEvidence={() => setDialog('addEvidence')}
        />
      )}
      {dialog === 'addEvidence' && (
        <EvidenceDialog
          mode="add"
          invRef={invRef}
          onSaved={(e) => setLinkInitial(e.ref)}
          onClose={() => setDialog((d) => (d === 'addEvidence' ? 'link' : d))}
        />
      )}
      {dialog === 'replace' && <ReplaceFindingDialog finding={finding} invRef={invRef} onClose={() => setDialog(null)} onReplaced={onOpenRef} />}
    </>
  )
}

/* Group 1: the finding, why the person thinks so, and the inline editor for both. */

const whyOf = (kind: Kind, r: StatementRecord): string => (kind === 'claim' ? (r as Claim).confidence_rationale : (r as Hypothesis).rationale)

interface Draft {
  statement: string
  why: string
}
const sameDraft = (a: Draft, b: Draft) => a.statement === b.statement && a.why === b.why

function StatementGroup({ finding, detail, calls }: { finding: Finding; detail: InvestigationDetail; calls: Calls }) {
  const { kind, record: r } = finding
  const write = useWrite()
  const retired = isReplaced(r) || isRuledOut(r)
  const whyField = kind === 'claim' ? 'confidence_rationale' : 'rationale'
  const why = whyOf(kind, r)
  const [editing, setEditing] = useState(false)
  const [draft, setDraft] = useState<Draft>({ statement: r.statement, why })
  const [snapshot, setSnapshot] = useState<Draft>(draft)
  const [busy, setBusy] = useState(false)
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [summary, setSummary] = useState<string | undefined>()
  const [focus, setFocus] = useState<'editor' | 'button' | null>(null)
  const editorRef = useRef<HTMLDivElement>(null)
  const buttonRef = useRef<HTMLButtonElement>(null)

  const stored: Draft = { statement: r.statement, why }
  const dirty = editing && !sameDraft(draft, snapshot)
  useDirtyEditor(dirty, 'drawer')
  const changedElsewhere = useChangedElsewhere(editing ? snapshot : null, stored, dirty, sameDraft) && !busy && !sameDraft(stored, draft)

  // A clean editor follows reloads; a dirty one keeps its input.
  useEffect(() => {
    if (!editing || dirty) return
    setSnapshot({ statement: r.statement, why })
    setDraft({ statement: r.statement, why })
  }, [r.statement, why, editing, dirty])

  useEffect(() => {
    if (focus === 'editor') editorRef.current?.querySelector('textarea')?.focus()
    if (focus === 'button') buttonRef.current?.focus()
    if (focus) setFocus(null)
  }, [focus])

  const begin = () => {
    setDraft(stored)
    setSnapshot(stored)
    setErrors({})
    setSummary(undefined)
    setEditing(true)
    setFocus('editor')
  }

  const finish = useCallback(() => {
    setEditing(false)
    setErrors({})
    setSummary(undefined)
    setFocus('button')
  }, [])

  useEscapeLayer(editing, finish)

  const save = async () => {
    if (busy) return
    // Only what changed is sent, so a save never writes over what an AI stored beside it.
    const patch: Patch = {}
    if (draft.statement !== r.statement) patch.statement = draft.statement
    if (draft.why !== why) patch[whyField] = draft.why
    if (Object.keys(patch).length === 0) {
      finish()
      return
    }
    setBusy(true)
    setErrors({})
    setSummary(undefined)
    try {
      const saved = await write(() => calls.update(patch))
      if (saved) finish()
    } catch (e) {
      const split = splitFieldErrors(e, ['statement', whyField])
      setErrors(split.fieldErrors)
      setSummary(split.summary)
    } finally {
      setBusy(false)
    }
  }

  const onKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) {
      e.preventDefault()
      void save()
    }
  }

  return (
    <div className="drawer-group statement-group">
      <p className="drawer-meta-line">
        <FindingLabel finding={finding} sources={evidenceByRef(detail)} />
        {r.imported && <ImportedTag />}
      </p>
      {editing ? (
        <div className="statement-editor" ref={editorRef} onKeyDown={onKeyDown}>
          {changedElsewhere && (
            <ChangedElsewhereBanner
              onShowLatest={() => {
                setSnapshot(stored)
                setDraft(stored)
                setErrors({})
              }}
            />
          )}
          {summary && (
            <p className="drawer-summary" role="alert">
              {summary}
            </p>
          )}
          <TextArea
            label="Finding"
            value={draft.statement}
            onChange={(statement) => setDraft((d) => ({ ...d, statement }))}
            error={errors.statement}
            note={nearLimit(draft.statement, LIMITS.statement)}
            spellCheck
          />
          <TextArea
            label="Why you think so (optional)"
            value={draft.why}
            onChange={(text) => setDraft((d) => ({ ...d, why: text }))}
            error={errors[whyField]}
            note={nearLimit(draft.why, LIMITS.rationale)}
            spellCheck
          />
          <div className="drawer-form-actions">
            <Button variant="secondary" onClick={finish} disabled={busy}>
              Cancel
            </Button>
            <Button variant="primary" busy={busy} busyLabel="Saving..." onClick={() => void save()}>
              Save finding
            </Button>
          </div>
        </div>
      ) : (
        <>
          <p className={['drawer-statement', retired ? 'is-retired' : ''].filter(Boolean).join(' ')}>{r.statement}</p>
          {why.trim() !== '' && (
            <p className="drawer-text">
              <span className="drawer-why">Why you think so:</span> {why}
            </p>
          )}
          <div>
            <Button variant="ghost" icon={Pencil} className="drawer-edit-button" ref={buttonRef} onClick={begin}>
              Edit finding
            </Button>
          </div>
        </>
      )}
    </div>
  )
}

/* Group 2: what a person decides about the finding. The heading says where
   it stands, in the words of its label. */

export const DECIDE_TITLES = {
  disagree: 'Sources disagree. You decide.',
  notTested: 'Not tested yet',
  standing: 'If this finding is no longer right',
  ruledOut: 'This finding was ruled out',
  replaced: 'This finding was replaced',
}

function DecideGroup({
  kind,
  record: r,
  disagree,
  calls,
  onOpenRef,
  onReplace,
  reviewChanged,
}: {
  kind: Kind
  record: StatementRecord
  disagree: boolean
  calls: Calls
  onOpenRef: (ref: string) => void
  onReplace: () => void
  reviewChanged: boolean
}) {
  const write = useWrite()
  const [busy, setBusy] = useState(false)
  const [summary, setSummary] = useState<string | undefined>()
  const [confirming, setConfirming] = useState(false)

  const run = async (fn: () => Promise<unknown>) => {
    if (busy) return
    setBusy(true)
    setSummary(undefined)
    try {
      // A change the engine refuses is shown here in plain words rather than as a toast.
      await write(fn, { rethrowConflict: true })
    } catch (e) {
      setSummary(splitFieldErrors(e, []).summary)
    } finally {
      setBusy(false)
    }
  }

  const retired = isReplaced(r) || isRuledOut(r)
  const notTested = kind === 'hypothesis' && r.status === 'open'
  const turnedOutRight = () => calls.update({ status: 'supported' })
  const title = isReplaced(r)
    ? DECIDE_TITLES.replaced
    : retired
      ? DECIDE_TITLES.ruledOut
      : disagree
        ? DECIDE_TITLES.disagree
        : notTested
          ? DECIDE_TITLES.notTested
          : DECIDE_TITLES.standing

  return (
    <DrawerGroup title={title}>
      {isReplaced(r) && r.superseded_by && <SupersededBy refText={r.superseded_by} onOpen={onOpenRef} />}
      <div className="drawer-actions">
        {retired ? (
          // It comes back with its label worked out from its sources again.
          <Button
            variant="secondary"
            disabled={busy}
            onClick={() => void run(() => calls.update({ status: kind === 'claim' ? 'active' : 'open', verification: 'ai_generated' }))}
          >
            {BRING_BACK_ACTION}
          </Button>
        ) : (
          <>
            {disagree && (
              <Button variant="secondary" disabled={busy || reviewChanged} onClick={() => setConfirming(true)}>
                {STILL_HOLDS_ACTION}
              </Button>
            )}
            {notTested && !disagree && (
              <Button variant="secondary" disabled={busy} onClick={() => void run(turnedOutRight)}>
                {TURNED_OUT_RIGHT_ACTION}
              </Button>
            )}
            <Button variant="secondary" disabled={busy} onClick={() => void run(() => calls.update({ status: kind === 'claim' ? 'rejected' : 'disproven' }))}>
              {RULE_OUT_ACTION}
            </Button>
            <Button variant="secondary" icon={Replace} disabled={busy} onClick={onReplace}>
              {REPLACE_ACTION}
            </Button>
          </>
        )}
      </div>
      {confirming && disagree && !retired && <Banner tone="warning" className="banner-stacked" action={<div className="banner-actions">
        <Button variant="ghost" onClick={() => setConfirming(false)}>Cancel</Button>
        <Button
          variant="secondary"
          disabled={busy || reviewChanged}
          onClick={() =>
            void run(async () => {
              await calls.decideHolds()
              // One the person decided holds is no longer waiting to be tested.
              if (notTested) await turnedOutRight()
            })
          }
        >
          {SETTLE_ACTION}
        </Button>
      </div>}>{SETTLE_COPY}</Banner>}
      {summary && (
        <p className="drawer-summary" role="alert">
          {summary}
        </p>
      )}
    </DrawerGroup>
  )
}

/* Deleting: permanent, confirmed, and refused while another record
   names this one as its replacement. */

export const deleteStatementBody = (r: StatementRecord) =>
  `This finding will be removed from this question.${r.links.length > 0 ? ' Its sources stay.' : ''} Use this for a finding added by mistake. To keep it on the page, rule it out or replace it instead.`

export const replacesBlocker = (replaced: string[]) =>
  replaced.length === 1
    ? 'This finding replaced another finding. Bring that one back or delete it first. It is under Show replaced findings.'
    : `This finding replaced ${replaced.length} other findings. Bring them back or delete them first. They are under Show replaced findings.`

function DeleteStatement({
  kind,
  record: r,
  detail,
  onDeleted,
}: {
  kind: Kind
  record: StatementRecord
  detail: InvestigationDetail
  onDeleted?: () => void
}) {
  const write = useWrite()
  const toast = useToast()
  const [open, setOpen] = useState(false)
  const noun = 'finding'
  const peers: StatementRecord[] = kind === 'claim' ? detail.claims : detail.hypotheses
  const replaced = peers.filter((p) => p.superseded_by === r.ref).map((p) => p.ref)
  const invRef = detail.investigation.ref
  return (
    <div className="drawer-delete">
      <Button variant="ghost" icon={Trash2} onClick={() => setOpen(true)}>
        {`Delete ${noun}...`}
      </Button>
      {open && (
        <DeleteDialog
          title={`Delete this ${noun}?`}
          body={deleteStatementBody(r)}
          blocker={replaced.length > 0 ? replacesBlocker(replaced) : null}
          blockerTitle="This finding replaced another one"
          confirmLabel={`Delete ${noun}`}
          onConfirm={async () => {
            const call = kind === 'claim' ? api.deleteClaim : api.deleteHypothesis
            const inv = detail.investigation
            const backing = (inv.conclusion_supporting_claims ?? []).filter((ref) => ref !== r.ref)
            const against = (inv.conclusion_contradicting_claims ?? []).filter((ref) => ref !== r.ref)
            const inAnswer = backing.length + against.length < (inv.conclusion_supporting_claims ?? []).length + (inv.conclusion_contradicting_claims ?? []).length
            const done = await write(
              async () => {
                // The engine keeps a finding the answer is based on, so the answer lets go of it first.
                if (inAnswer) {
                  await api.updateConclusion(invRef, {
                    conclusion: inv.conclusion,
                    confidence: inv.conclusion_confidence,
                    rationale: inv.conclusion_rationale,
                    supporting_claims: backing,
                    contradicting_claims: against,
                  })
                }
                await call(invRef, r.ref)
                return true
              },
              { rethrowConflict: true },
            )
            if (done) {
              toast.success(`Deleted the ${noun}.`)
              onDeleted?.()
            }
            return Boolean(done)
          }}
          onClose={() => setOpen(false)}
        />
      )}
    </div>
  )
}

/* Group 3: the sources for and against it. */

function EvidenceGroups({
  record: r,
  detail,
  calls,
  onOpenRef,
  onLink,
  linkButtonRef,
}: {
  record: StatementRecord
  detail: InvestigationDetail
  calls: Calls
  onOpenRef: (ref: string) => void
  onLink: () => void
  linkButtonRef: RefObject<HTMLButtonElement | null>
}) {
  const write = useWrite()
  const toast = useToast()
  const [pending, setPending] = useState<string | null>(null)
  const [focusRef, setFocusRef] = useState<string | null>(null)
  const containerRef = useRef<HTMLDivElement>(null)

  const byRef = new Map(detail.evidence.map((e) => [e.ref, e]))
  const linked = (relation: Relation): Evidence[] =>
    r.links
      .filter((l) => l.relation === relation)
      .map((l) => byRef.get(l.evidence_ref))
      .filter((e): e is Evidence => Boolean(e))
  const supporting = linked('supports')
  const contradicting = linked('contradicts')
  const none = r.links.length === 0

  // After a flip the block moves to the other group: keep focus on it.
  useEffect(() => {
    if (!focusRef) return
    const block = containerRef.current?.querySelector<HTMLElement>(`[data-ref="${focusRef}"] .row-hit`)
    if (block) {
      block.focus()
      setFocusRef(null)
    }
  })

  const flip = async (evidenceRef: string, to: Relation) => {
    if (pending) return
    setPending(evidenceRef)
    try {
      const saved = await write(() => calls.link(evidenceRef, to))
      if (saved) setFocusRef(evidenceRef)
    } catch (err) {
      toast.error(errorCopy(toApiError(err)).message)
    } finally {
      setPending(null)
    }
  }

  const unlink = async (evidenceRef: string, was: Relation) => {
    if (pending) return
    setPending(evidenceRef)
    try {
      const saved = await write(() => calls.unlink(evidenceRef))
      if (saved) {
        linkButtonRef.current?.focus()
        toast.success('Removed the source from this finding.', {
          label: 'Undo',
          onClick: () => void write(() => calls.link(evidenceRef, was)).catch(() => {}),
        })
      }
    } catch (err) {
      toast.error(errorCopy(toApiError(err)).message)
    } finally {
      setPending(null)
    }
  }

  const list = (items: Evidence[], relation: Relation) => (
    <ul className="evidence-block-list">
      {items.map((e) => (
        <li key={e.ref}>
          <EvidenceBlock
            evidence={e}
            onOpen={onOpenRef}
            actions={
              <>
                <Button
                  variant="ghost"
                  disabled={pending === e.ref}
                  onClick={() => void flip(e.ref, relation === 'supports' ? 'contradicts' : 'supports')}
                >
                  {relation === 'supports' ? MOVE_TO_AGAINST : MOVE_TO_BACKS}
                </Button>
                <Button variant="ghost" icon={Unlink} disabled={pending === e.ref} onClick={() => void unlink(e.ref, relation)}>
                  Remove from this finding
                </Button>
              </>
            }
          />
        </li>
      ))}
    </ul>
  )

  return (
    <div className="evidence-groups" ref={containerRef}>
      <DrawerGroup title={SUPPORTING_TITLE}>
        {supporting.length > 0 ? (
          list(supporting, 'supports')
        ) : (
          <p className="drawer-empty">{none ? 'This finding has no sources yet.' : 'Nothing backs this yet.'}</p>
        )}
      </DrawerGroup>
      {!none && (
        <DrawerGroup title={CONTRADICTING_TITLE}>
          {contradicting.length > 0 ? (
            list(contradicting, 'contradicts')
          ) : (
            <p className="drawer-empty">Nothing goes against this yet.</p>
          )}
        </DrawerGroup>
      )}
      <div className="drawer-link-actions">
        <Button variant="secondary" icon={LinkIcon} ref={linkButtonRef} onClick={onLink}>
          Add a source to this finding
        </Button>
      </div>
    </div>
  )
}

