// Claim and hypothesis drawer body (D-21; UI-08, UI-09, UI-10, UI-13,
// UI-14, UI-15; UI-SPEC Drawers, groups 1 to 4):
//   1. the statement in Literata 20 under its verification badge and status,
//      with "Edit statement" opening an inline editor;
//   2. "How sure are we": confidence and rationale, saved together;
//   3. "Status and verification": Verify, Dispute, Supersede by..., with the
//      Status and Verification selects behind "More options";
//   4. supporting and contradicting evidence blocks (each opens the evidence
//      drawer) with Mark as ... and Unlink, then Link evidence.
//
// Every write goes through useWrite. Saves show no toast; the group simply
// shows the saved value. Edit-in-place safety: the statement editor and the
// confidence form snapshot the stored values; a reload never touches unsaved
// input, and when the stored values changed elsewhere the group says "This
// changed in another app." with "Show latest". While a group holds unsaved
// input, Esc discards that input first; the next Esc closes the drawers.
import { useCallback, useEffect, useId, useRef, useState, type KeyboardEvent, type ReactNode, type RefObject } from 'react'
import { BadgeCheck, Link as LinkIcon, Pencil, Replace, Trash2, Unlink } from 'lucide-react'
import { api } from '../../api/api'
import { ApiError, errorCopy, toApiError } from '../../api/errors'
import type {
  Claim,
  ClaimPatch,
  ClaimStatus,
  Confidence,
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
import { claimStatusLabels, confidenceLabels, confidenceOptions, hypothesisStatusLabels, verificationLabels } from '../../lib/labels'
import { pushLayer } from '../../lib/layers'
import { STATEMENT_VERIFICATIONS, statementMarks } from '../../lib/recordState'
import { useDirtyEditor } from '../../state/dirtyGuard'
import { useWrite } from '../../state/useWrite'
import { ImportedTag } from '../record/ImportedTag'
import { RecordStatusLabel } from '../record/StatusLabel'
import { VerificationBadge } from '../record/VerificationBadge'
import { DeleteDialog, refList } from '../dialogs/DeleteDialog'
import { EvidenceDialog } from '../dialogs/EvidenceDialog'
import { LinkEvidenceDialog } from '../dialogs/LinkEvidenceDialog'
import { SupersedeDialog } from '../dialogs/SupersedeDialog'
import { Banner } from '../ui/Banner'
import { Button } from '../ui/Button'
import { SegmentedControl } from '../ui/SegmentedControl'
import { Select, type SelectOption } from '../ui/Select'
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

export const MORE_OPTIONS_LABEL = 'More options'
export const FEWER_OPTIONS_LABEL = 'Fewer options'

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

/** "Superseded by C-002" where the ref is a text button that opens the replacement. */
export function SupersededBy({ refText, onOpen }: { refText: string; onOpen: (ref: string) => void }) {
  return (
    <p className="superseded-by">
      Superseded by{' '}
      <button type="button" className="text-button tabular" title={`Open ${refText}`} onClick={() => onOpen(refText)}>
        {refText}
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
        if (!basis) throw new ApiError('conflict', 'Reload this case before verifying the finding.')
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
      <ConfidenceGroup kind={kind} record={r} calls={calls} />
      {reviewChanged && <Banner tone="warning" action={<Button variant="secondary" onClick={() => setReviewSnapshot({ key: reviewKey, basis: latestBasis })}>Use latest for review</Button>}>
        This finding or its evidence changed. Inspect the latest version before verifying it.
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

  // One coherent state (ADR-0013); a superseded record says so in the
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
            <ChangedElsewhere
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
            label="Statement"
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
              Save statement
            </Button>
          </div>
        </div>
      ) : (
        <>
          <p className={['drawer-statement', retired ? 'is-retired' : ''].filter(Boolean).join(' ')}>{r.statement}</p>
          <div>
            <Button variant="ghost" icon={Pencil} className="drawer-edit-button" ref={buttonRef} onClick={begin}>
              Edit statement
            </Button>
          </div>
        </>
      )}
    </div>
  )
}

function ChangedElsewhere({ onShowLatest }: { onShowLatest: () => void }) {
  return (
    <Banner
      className="changed-elsewhere"
      action={
        <Button variant="ghost" onClick={onShowLatest}>
          Show latest
        </Button>
      }
    >
      This changed in another app.
    </Banner>
  )
}

/* Group 2: how sure we are. */

interface ConfidenceDraft {
  confidence: Confidence
  rationale: string
}

const sameConfidence = (a: ConfidenceDraft, b: ConfidenceDraft) =>
  a.confidence === b.confidence && a.rationale === b.rationale

function storedConfidence(kind: Kind, r: StatementRecord): ConfidenceDraft {
  return {
    confidence: confidenceOptions.includes(r.confidence) ? r.confidence : 'unknown',
    rationale: kind === 'claim' ? (r as Claim).confidence_rationale : (r as Hypothesis).rationale,
  }
}

function ConfidenceGroup({ kind, record: r, calls }: { kind: Kind; record: StatementRecord; calls: Calls }) {
  const write = useWrite()
  const stored = storedConfidence(kind, r)
  const [draft, setDraft] = useState<ConfidenceDraft>(stored)
  const [snapshot, setSnapshot] = useState<ConfidenceDraft>(stored)
  const [busy, setBusy] = useState(false)
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [summary, setSummary] = useState<string | undefined>()
  const rationaleField = kind === 'claim' ? 'confidence_rationale' : 'rationale'

  const dirty = !sameConfidence(draft, snapshot)
  useDirtyEditor(dirty, 'drawer')
  const changedElsewhere =
    useChangedElsewhere(snapshot, stored, dirty, sameConfidence) && !busy && !sameConfidence(stored, draft)
  const changed = !sameConfidence(draft, stored)

  const storedKey = JSON.stringify(stored)
  useEffect(() => {
    if (dirty) return
    const latest: ConfidenceDraft = JSON.parse(storedKey)
    setSnapshot((s) => (sameConfidence(s, latest) ? s : latest))
    setDraft((d) => (sameConfidence(d, latest) ? d : latest))
  }, [storedKey, dirty])

  const reset = useCallback(() => {
    const latest: ConfidenceDraft = JSON.parse(storedKey)
    setSnapshot(latest)
    setDraft(latest)
    setErrors({})
    setSummary(undefined)
  }, [storedKey])

  useEscapeLayer(dirty, reset)

  const save = async () => {
    if (busy || !changed) return
    setBusy(true)
    setErrors({})
    setSummary(undefined)
    const sent = draft
    try {
      const patch: Patch =
        kind === 'claim'
          ? { confidence: sent.confidence, confidence_rationale: sent.rationale }
          : { confidence: sent.confidence, rationale: sent.rationale }
      const saved = await write(() => calls.update(patch))
      if (saved) setSnapshot(sent)
    } catch (e) {
      const split = splitFieldErrors(e, ['confidence', rationaleField])
      setErrors(split.fieldErrors)
      setSummary(split.summary)
    } finally {
      setBusy(false)
    }
  }

  return (
    <DrawerGroup title="How sure are we">
      <div
        className="confidence-form"
        onKeyDown={(e) => {
          if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) {
            e.preventDefault()
            void save()
          }
        }}
      >
        {changedElsewhere && <ChangedElsewhere onShowLatest={reset} />}
        {summary && (
          <p className="drawer-summary" role="alert">
            {summary}
          </p>
        )}
        <SegmentedControl<Confidence>
          label="Confidence"
          options={confidenceOptions.map((c) => ({ value: c, label: confidenceLabels[c] }))}
          value={draft.confidence}
          onChange={(confidence) => setDraft((d) => ({ ...d, confidence }))}
          error={errors.confidence}
        />
        <TextArea
          label="Why this confidence (optional)"
          value={draft.rationale}
          onChange={(rationale) => setDraft((d) => ({ ...d, rationale }))}
          error={errors[rationaleField]}
          note={nearLimit(draft.rationale, LIMITS.rationale)}
          spellCheck
        />
        <div className="drawer-form-actions">
          <Button
            variant="primary"
            busy={busy}
            busyLabel="Saving..."
            disabled={!changed}
            onClick={() => void save()}
          >
            Save confidence
          </Button>
        </div>
      </div>
    </DrawerGroup>
  )
}

/* Group 3: status and verification. */

const CLAIM_STATUSES: ClaimStatus[] = ['active', 'disputed', 'rejected']
const HYPOTHESIS_STATUSES: HypothesisStatus[] = ['open', 'supported', 'disproven']

export const SUPERSEDED_HINT =
  'Superseded records keep their verification. To change it, choose a status to bring this one back first.'

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
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [summary, setSummary] = useState<string | undefined>()
  // Verify, Dispute and Supersede sit in the open; the Status and
  // Verification selects, the only way to set Disproven, Supported or
  // Rejected, wait behind More options.
  const [more, setMore] = useState(false)
  const [settleDispute, setSettleDispute] = useState(false)

  const run = async (fn: () => Promise<unknown>) => {
    if (busy) return
    setBusy(true)
    setErrors({})
    setSummary(undefined)
    try {
      // A conflict (for example a change the engine refuses on a superseded
      // record) is shown here in plain words rather than as a toast.
      await write(fn, { rethrowConflict: true })
    } catch (e) {
      const split = splitFieldErrors(e, ['status', 'verification', 'superseded_by'])
      setErrors(split.fieldErrors)
      setSummary(split.summary)
      // A field error shows under its select, so never leave it hidden.
      if (Object.keys(split.fieldErrors).length > 0) setMore(true)
    } finally {
      setBusy(false)
    }
  }

  // ADR-0013: Verify settles a dispute (the claim becomes active), Dispute
  // sets both halves, and nothing but Supersede by... makes a record
  // superseded. A superseded record changes only through its status.
  const marks = statementMarks(kind, r)
  const superseded = marks.superseded
  const verified = !marks.disputed && r.verification === 'human_verified'
  const disputed = marks.disputed

  const statusOptions: SelectOption[] =
    kind === 'claim'
      ? CLAIM_STATUSES.map((s) => ({ value: s, label: claimStatusLabels[s] }))
      : HYPOTHESIS_STATUSES.map((s) => ({ value: s, label: hypothesisStatusLabels[s] }))
  if (r.status === 'superseded') statusOptions.unshift({ value: 'superseded', label: 'Superseded', disabled: true })
  const verificationOptions: SelectOption[] = STATEMENT_VERIFICATIONS.map((v) => ({ value: v, label: verificationLabels[v] }))
  // The selects show the one coherent state: a disputed claim reads Disputed
  // in both, a superseded record Superseded in both.
  const statusValue: string = marks.status
  let verificationValue: Verification = r.verification
  if (superseded) verificationValue = 'superseded'
  else if (kind === 'claim' && disputed) verificationValue = 'disputed'
  else if (!STATEMENT_VERIFICATIONS.includes(verificationValue)) verificationValue = 'ai_generated'
  if (verificationValue === 'superseded') {
    verificationOptions.push({ value: 'superseded', label: verificationLabels.superseded, disabled: true })
  }

  return (
    <DrawerGroup title="Status and verification">
      {r.status === 'superseded' && r.superseded_by && <SupersededBy refText={r.superseded_by} onOpen={onOpenRef} />}
      <div className="drawer-actions">
        {verified ? (
          <Button variant="secondary" icon={BadgeCheck} disabled>
            Verified
          </Button>
        ) : (
          <Button
            variant="secondary"
            disabled={busy || superseded || reviewChanged}
            onClick={() => disputed ? setSettleDispute(true) : void run(() => calls.setVerification('human_verified'))}
          >
            Verify myself
          </Button>
        )}
        <Button
          variant="secondary"
          disabled={busy || disputed || superseded}
          onClick={() =>
            void run(() =>
              kind === 'claim'
                ? calls.update({ status: 'disputed', verification: 'disputed' })
                : calls.setVerification('disputed'),
            )
          }
        >
          Dispute
        </Button>
        <Button variant="secondary" icon={Replace} disabled={busy} onClick={onSupersede}>
          Supersede by...
        </Button>
      </div>
      {settleDispute && disputed && <Banner tone="warning" action={<>
        <Button variant="ghost" onClick={() => setSettleDispute(false)}>Cancel</Button>
        <Button variant="secondary" disabled={busy || reviewChanged} onClick={() => void run(() => calls.setVerification('human_verified', true))}>Verify and settle dispute</Button>
      </>}>Verifying this finding will settle its dispute. Confirm after checking its evidence.</Banner>}
      {summary && (
        <p className="drawer-summary" role="alert">
          {summary}
        </p>
      )}
      <button
        type="button"
        className="text-button drawer-more"
        aria-expanded={more}
        onClick={() => setMore((v) => !v)}
      >
        {more ? FEWER_OPTIONS_LABEL : MORE_OPTIONS_LABEL}
      </button>
      {more && (
        <div className="drawer-selects">
          <Select
            label="Status"
            value={statusValue}
            options={statusOptions}
            disabled={busy}
            error={errors.status ?? errors.superseded_by}
            onChange={(v) => {
              if (v !== statusValue) void run(() => calls.update({ status: v as ClaimStatus | HypothesisStatus }))
            }}
          />
          <Select
            label="Verification"
            value={verificationValue}
            options={verificationOptions}
            disabled={busy || superseded || reviewChanged}
            error={errors.verification}
            onChange={(v) => {
              if (v !== verificationValue) void run(() => calls.setVerification(v as Verification))
            }}
          />
        </div>
      )}
      {superseded && <p className="drawer-hint">{SUPERSEDED_HINT}</p>}
    </DrawerGroup>
  )
}

/* Deleting (AR-10): permanent, confirmed, and refused while another record
   names this one as its replacement. */

export const deleteStatementBody = (kind: Kind, r: StatementRecord) => {
  const noun = kind === 'claim' ? 'claim' : 'hypothesis'
  const links = r.links.length
  const linkText = links === 0 ? '' : ` and its ${links === 1 ? 'evidence link' : `${links} evidence links`}`
  return `${r.ref} will be removed from this case${linkText}. The evidence itself stays. Use this for a ${noun} entered by mistake; to retire one, change its status or supersede it instead.`
}

export const replacesBlocker = (ref: string, replaced: string[]) =>
  `${ref} replaces ${refList(replaced)}. Bring ${replaced.length === 1 ? 'it' : 'them'} back with a status, or delete ${replaced.length === 1 ? 'it' : 'them'}, first.`

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
  const noun = kind === 'claim' ? 'claim' : 'hypothesis'
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
          title={`Delete ${noun} ${r.ref}?`}
          body={deleteStatementBody(kind, r)}
          blocker={replaced.length > 0 ? replacesBlocker(r.ref, replaced) : null}
          confirmLabel={`Delete ${noun}`}
          onConfirm={async () => {
            const call = kind === 'claim' ? api.deleteClaim : api.deleteHypothesis
            const done = await write(() => call(invRef, r.ref).then(() => true), { rethrowConflict: true })
            if (done) {
              toast.success(`Deleted ${noun} ${r.ref}.`)
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

/* Group 4: the evidence for and against it. */

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
    const recordRef = r.ref
    try {
      const saved = await write(() => calls.unlink(evidenceRef))
      if (saved) {
        linkButtonRef.current?.focus()
        toast.success(`Unlinked ${evidenceRef} from ${recordRef}.`, {
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
                  {relation === 'supports' ? 'Mark as contradicting' : 'Mark as supporting'}
                </Button>
                <Button variant="ghost" icon={Unlink} disabled={pending === e.ref} onClick={() => void unlink(e.ref, relation)}>
                  Unlink
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
      <DrawerGroup title="Supporting evidence">
        {supporting.length > 0 ? (
          list(supporting, 'supports')
        ) : (
          <p className="drawer-empty">{none ? 'No evidence is linked yet.' : 'Nothing supports this yet.'}</p>
        )}
      </DrawerGroup>
      {!none && (
        <DrawerGroup title="Contradicting evidence">
          {contradicting.length > 0 ? (
            list(contradicting, 'contradicts')
          ) : (
            <p className="drawer-empty">Nothing contradicts this yet.</p>
          )}
        </DrawerGroup>
      )}
      <div className="drawer-link-actions">
        <Button variant="secondary" icon={LinkIcon} ref={linkButtonRef} onClick={onLink}>
          Link evidence
        </Button>
      </div>
    </div>
  )
}

