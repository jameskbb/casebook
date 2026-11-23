// Finding drawer body (a claim or a hypothesis):
//   1. the finding in Literata 20 under its labels, with "Edit finding"
//      opening an inline editor;
//   2. why you think so. Confidence is stored and never shown or sent;
//   3. "Check this finding": I checked this, Sources disagree, Rule this out
//      and Replace with a newer finding; a replaced or ruled-out finding
//      shows only Bring this back;
//   4. the sources that back it up and go against it (each opens the source
//      drawer), then Add a source to this finding.
//
// Every write goes through useWrite. Saves show no toast; the group simply
// shows the saved value. Edit-in-place safety: the editors snapshot the
// stored values; a reload never touches unsaved input, and when the stored
// values changed meanwhile the group says so and offers the new version.
// While a group holds unsaved input, Esc discards that input first; the next
// Esc closes the drawers.
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
  Verification,
} from '../../api/types'
import { useChangedElsewhere } from '../../hooks/useChangedElsewhere'
import { LIMITS, nearLimit } from '../../lib/bytes'
import { splitFieldErrors } from '../../lib/formErrors'
import { pushLayer } from '../../lib/layers'
import { statementMarks } from '../../lib/recordState'
import { useDirtyEditor } from '../../state/dirtyGuard'
import { useWrite } from '../../state/useWrite'
import { ImportedTag } from '../record/ImportedTag'
import { RecordId } from '../record/RecordId'
import { RecordStatusLabel } from '../record/StatusLabel'
import { VerificationBadge } from '../record/VerificationBadge'
import { DeleteDialog } from '../dialogs/DeleteDialog'
import { EvidenceDialog } from '../dialogs/EvidenceDialog'
import { LinkEvidenceDialog } from '../dialogs/LinkEvidenceDialog'
import { SupersedeDialog } from '../dialogs/SupersedeDialog'
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

/** The one act a person records: they compared the finding with its sources. */
export const CHECKED_ACTION = 'I checked this'
export const SETTLE_ACTION = 'Yes, I checked it'
export const SETTLE_COPY = 'The sources disagree about this finding. Choose this only if you read them and the finding still holds.'
export const DISAGREE_ACTION = 'Sources disagree'
export const RULE_OUT_ACTION = 'Rule this out'
export const REPLACE_ACTION = 'Replace with a newer finding'
export const BRING_BACK_ACTION = 'Bring this back'
export const SUPPORTING_TITLE = 'Sources that back it up'
export const CONTRADICTING_TITLE = 'Sources that go against it'
export const FINDING_CHANGED_COPY = 'This finding or its sources changed. Read the new version before you check it.'

type Kind = 'claim' | 'hypothesis'
type StatementRecord = Claim | Hypothesis
/** A claim or hypothesis patch; useRecordCalls sends it to the matching call. */
type Patch = Omit<ClaimPatch, 'status'> & Omit<HypothesisPatch, 'status'> & { status?: ClaimStatus | HypothesisStatus }

const RETIRED = new Set(['superseded', 'rejected', 'disproven'])

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
  return {
    update: (patch: Patch): Promise<StatementRecord> =>
      kind === 'claim'
        ? api.updateClaim(invRef, ref, patch as ClaimPatch)
        : api.updateHypothesis(invRef, ref, patch as HypothesisPatch),
    setVerification: async (v: Verification, settleDispute = false): Promise<StatementRecord> => {
      if (v === 'human_verified') {
        if (!basis) throw new ApiError('conflict', 'this finding changed; show the new version')
        await api.reviewStatement(invRef, { kind, ref, basis, verification: v, settle_dispute: settleDispute })
        return kind === 'claim' ? api.getClaim(invRef, ref) : api.getHypothesis(invRef, ref)
      }
      return kind === 'claim'
        ? api.setClaimVerification(invRef, ref, v)
        : api.setHypothesisVerification(invRef, ref, v)
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
  const invRef = detail.investigation.ref
  const latestBasis = detail.review_bases?.find((b) => b.ref === r.ref)?.basis
  const reviewKey = `${invRef}/${r.ref}`
  const [reviewSnapshot, setReviewSnapshot] = useState({ key: reviewKey, basis: latestBasis })
  useEffect(() => {
    setReviewSnapshot({ key: reviewKey, basis: latestBasis })
    // A live refresh must not acknowledge new source material for the person.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [reviewKey])
  const reviewChanged = reviewSnapshot.key !== reviewKey || reviewSnapshot.basis !== latestBasis
  const calls = useRecordCalls(kind, invRef, r.ref, reviewSnapshot.key === reviewKey ? reviewSnapshot.basis : undefined)
  const [dialog, setDialog] = useState<'link' | 'supersede' | 'addEvidence' | null>(null)
  // Evidence added from the link dialog is preselected when it reopens.
  const [linkInitial, setLinkInitial] = useState<string | undefined>()
  const linkButtonRef = useRef<HTMLButtonElement>(null)

  return (
    <>
      <StatementGroup kind={kind} record={r} calls={calls} />
      <WhyGroup kind={kind} record={r} calls={calls} />
      {reviewChanged && <Banner tone="warning" action={<Button variant="secondary" onClick={() => setReviewSnapshot({ key: reviewKey, basis: latestBasis })}>{SHOW_NEW_VERSION}</Button>}>
        {FINDING_CHANGED_COPY}
      </Banner>}
      <StatusGroup
        kind={kind}
        record={r}
        calls={calls}
        onOpenRef={onOpenRef}
        onSupersede={() => setDialog('supersede')}
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
      {dialog === 'supersede' &&
        (props.kind === 'claim' ? (
          <SupersedeDialog
            kind="claim"
            record={props.record}
            records={detail.claims}
            invRef={invRef}
            onClose={() => setDialog(null)}
          />
        ) : (
          <SupersedeDialog
            kind="hypothesis"
            record={props.record}
            records={detail.hypotheses}
            invRef={invRef}
            onClose={() => setDialog(null)}
          />
        ))}
    </>
  )
}

/* Group 1: the statement and its inline editor. */

function StatementGroup({ kind, record: r, calls }: { kind: Kind; record: StatementRecord; calls: Calls }) {
  const write = useWrite()
  const retired = RETIRED.has(r.status)
  const [editing, setEditing] = useState(false)
  const [draft, setDraft] = useState(r.statement)
  const [snapshot, setSnapshot] = useState(r.statement)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | undefined>()
  const [summary, setSummary] = useState<string | undefined>()
  const [focus, setFocus] = useState<'editor' | 'button' | null>(null)
  const editorRef = useRef<HTMLDivElement>(null)
  const buttonRef = useRef<HTMLButtonElement>(null)

  const dirty = editing && draft !== snapshot
  useDirtyEditor(dirty, 'drawer')
  const changedElsewhere =
    useChangedElsewhere(editing ? snapshot : null, r.statement, dirty) && !busy && r.statement !== draft

  // A clean editor follows reloads; a dirty one keeps its input.
  useEffect(() => {
    if (!editing || dirty) return
    setSnapshot(r.statement)
    setDraft(r.statement)
  }, [r.statement, editing, dirty])

  useEffect(() => {
    if (focus === 'editor') editorRef.current?.querySelector('textarea')?.focus()
    if (focus === 'button') buttonRef.current?.focus()
    if (focus) setFocus(null)
  }, [focus])

  const begin = () => {
    setDraft(r.statement)
    setSnapshot(r.statement)
    setError(undefined)
    setSummary(undefined)
    setEditing(true)
    setFocus('editor')
  }

  const finish = useCallback(() => {
    setEditing(false)
    setError(undefined)
    setSummary(undefined)
    setFocus('button')
  }, [])

  useEscapeLayer(editing, finish)

  const save = async () => {
    if (busy) return
    if (draft === r.statement) {
      finish()
      return
    }
    setBusy(true)
    setError(undefined)
    setSummary(undefined)
    try {
      const saved = await write(() => calls.update({ statement: draft }))
      if (saved) finish()
    } catch (e) {
      const split = splitFieldErrors(e, ['statement'])
      setError(split.fieldErrors.statement)
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

  // One coherent state; a superseded record says so in the
  // Status and verification group, with its replacement.
  const marks = statementMarks(kind, r)
  return (
    <div className="drawer-group statement-group">
      <p className="drawer-meta-line">
        {marks.badge && <VerificationBadge verification={marks.badge} />}
        {!marks.superseded &&
          (kind === 'claim' ? (
            <RecordStatusLabel kind="claim" status={marks.status as ClaimStatus} />
          ) : (
            <RecordStatusLabel kind="hypothesis" status={marks.status as HypothesisStatus} />
          ))}
        {r.imported && <ImportedTag />}
      </p>
      {editing ? (
        <div className="statement-editor" ref={editorRef} onKeyDown={onKeyDown}>
          {changedElsewhere && (
            <ChangedElsewhereBanner
              onShowLatest={() => {
                setSnapshot(r.statement)
                setDraft(r.statement)
                setError(undefined)
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
            value={draft}
            onChange={setDraft}
            error={error}
            note={nearLimit(draft, LIMITS.statement)}
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

/* Group 2: why you think so. Only this text is sent, so a save never writes over what an AI stored beside it. */

const whyOf = (kind: Kind, r: StatementRecord): string => (kind === 'claim' ? (r as Claim).confidence_rationale : (r as Hypothesis).rationale)

function WhyGroup({ kind, record: r, calls }: { kind: Kind; record: StatementRecord; calls: Calls }) {
  const write = useWrite()
  const stored = whyOf(kind, r)
  const [draft, setDraft] = useState(stored)
  const [snapshot, setSnapshot] = useState(stored)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | undefined>()
  const [summary, setSummary] = useState<string | undefined>()
  const field = kind === 'claim' ? 'confidence_rationale' : 'rationale'

  const dirty = draft !== snapshot
  useDirtyEditor(dirty, 'drawer')
  const changedElsewhere = useChangedElsewhere(snapshot, stored, dirty) && !busy && stored !== draft
  const changed = draft !== stored

  // A clean editor follows reloads; a dirty one keeps its input.
  useEffect(() => {
    if (dirty) return
    setSnapshot(stored)
    setDraft(stored)
  }, [stored, dirty])

  const reset = useCallback(() => {
    setSnapshot(stored)
    setDraft(stored)
    setError(undefined)
    setSummary(undefined)
  }, [stored])

  useEscapeLayer(dirty, reset)

  const save = async () => {
    if (busy || !changed) return
    setBusy(true)
    setError(undefined)
    setSummary(undefined)
    const sent = draft
    try {
      const saved = await write(() => calls.update({ [field]: sent }))
      if (saved) setSnapshot(sent)
    } catch (e) {
      const split = splitFieldErrors(e, [field])
      setError(split.fieldErrors[field])
      setSummary(split.summary)
    } finally {
      setBusy(false)
    }
  }

  return (
    <div
      className="drawer-group confidence-form"
      onKeyDown={(e) => {
        if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) {
          e.preventDefault()
          void save()
        }
      }}
    >
      {changedElsewhere && <ChangedElsewhereBanner onShowLatest={reset} />}
      {summary && (
        <p className="drawer-summary" role="alert">
          {summary}
        </p>
      )}
      <TextArea
        label="Why you think so (optional)"
        value={draft}
        onChange={setDraft}
        error={error}
        note={nearLimit(draft, LIMITS.rationale)}
        spellCheck
      />
      <div className="drawer-form-actions">
        <Button variant="primary" busy={busy} busyLabel="Saving..." disabled={!changed} onClick={() => void save()}>
          Save
        </Button>
      </div>
    </div>
  )
}

/* Group 3: checking the finding. Four buttons cover what a person decides;
   a replaced or ruled-out finding offers only Bring this back. */

function StatusGroup({
  kind,
  record: r,
  calls,
  onOpenRef,
  onSupersede,
  reviewChanged,
}: {
  kind: Kind
  record: StatementRecord
  calls: Calls
  onOpenRef: (ref: string) => void
  onSupersede: () => void
  reviewChanged: boolean
}) {
  const write = useWrite()
  const [busy, setBusy] = useState(false)
  const [summary, setSummary] = useState<string | undefined>()
  const [settleDispute, setSettleDispute] = useState(false)

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

  const marks = statementMarks(kind, r)
  const retired = RETIRED.has(r.status)
  const disputed = marks.disputed
  const checked = !disputed && r.verification === 'human_verified'

  const check = (settle = false) =>
    run(async () => {
      await calls.setVerification('human_verified', settle)
      // A guess the person checked is no longer waiting to be tested.
      if (kind === 'hypothesis' && r.status === 'open') await calls.update({ status: 'supported' })
    })

  return (
    <DrawerGroup title="Check this finding">
      {r.status === 'superseded' && r.superseded_by && <SupersededBy refText={r.superseded_by} onOpen={onOpenRef} />}
      <div className="drawer-actions">
        {retired ? (
          // It comes back as Not checked: bringing it back is not checking it.
          <Button
            variant="secondary"
            disabled={busy}
            onClick={() => void run(() => calls.update({ status: kind === 'claim' ? 'active' : 'open', verification: 'ai_generated' }))}
          >
            {BRING_BACK_ACTION}
          </Button>
        ) : (
          <>
            {!checked && (
              <Button variant="secondary" disabled={busy || reviewChanged} onClick={() => (disputed ? setSettleDispute(true) : void check())}>
                {CHECKED_ACTION}
              </Button>
            )}
            <Button
              variant="secondary"
              disabled={busy || disputed}
              onClick={() =>
                void run(() => (kind === 'claim' ? calls.update({ status: 'disputed', verification: 'disputed' }) : calls.setVerification('disputed')))
              }
            >
              {DISAGREE_ACTION}
            </Button>
            <Button variant="secondary" disabled={busy} onClick={() => void run(() => calls.update({ status: kind === 'claim' ? 'rejected' : 'disproven' }))}>
              {RULE_OUT_ACTION}
            </Button>
            <Button variant="secondary" icon={Replace} disabled={busy} onClick={onSupersede}>
              {REPLACE_ACTION}
            </Button>
          </>
        )}
      </div>
      {settleDispute && disputed && !retired && <Banner tone="warning" action={<>
        <Button variant="ghost" onClick={() => setSettleDispute(false)}>Cancel</Button>
        <Button variant="secondary" disabled={busy || reviewChanged} onClick={() => void check(true)}>{SETTLE_ACTION}</Button>
      </>}>{SETTLE_COPY}</Banner>}
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

/* Group 4: the sources for and against it. */

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
                  {relation === 'supports' ? 'It goes against this' : 'It backs this'}
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

