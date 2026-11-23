// Source drawer body: the name, its check label and what it shows; where to
// find it; the exact words or numbers, fetched on demand through GetEvidence
// with include_raw_detail; the stored kind and the extra details an AI
// stored, shown only when codes are on; and the findings that use it, each
// opening its drawer.
//
// The exact words and the extra details are text written by people, AI tools
// and files, so they render as text inside a pre block (never markup), and
// where to find it is plain selectable text (never a link).
//
// Actions: Edit opens the source dialog over the drawer; I checked this, or
// Undo my check once the person has; the More menu holds A newer source
// replaces this (after a confirmation) and Delete source... A replaced
// source offers Bring this back.
import { useCallback, useEffect, useRef, useState } from 'react'
import { ChevronDown, Ellipsis, Pencil, Replace, Trash2 } from 'lucide-react'
import { api } from '../../api/api'
import { errorCopy, toApiError } from '../../api/errors'
import type { Evidence, InvestigationDetail, Verification } from '../../api/types'
import { useDelayedFlag } from '../../hooks/useDelayedFlag'
import { relationTags, REPLACED, RULED_OUT, sourceKindWord } from '../../lib/labels'
import { prettyMetadata } from '../../lib/metadata'
import { isReplaced, isRuledOut, usedBy } from '../../lib/records'
import { ImportedTag } from '../record/ImportedTag'
import { kindRuleClass } from '../record/KindRule'
import { RecordId } from '../record/RecordId'
import { VerificationBadge } from '../record/VerificationBadge'
import { useWrite } from '../../state/useWrite'
import { DeleteDialog } from '../dialogs/DeleteDialog'
import { EvidenceDialog } from '../dialogs/EvidenceDialog'
import { BreakablePath } from '../ui/BreakablePath'
import { Button } from '../ui/Button'
import { ConfirmDialog } from '../ui/ConfirmDialog'
import { IconButton } from '../ui/IconButton'
import { Menu } from '../ui/Menu'
import { useToast } from '../ui/Toaster'
import { BRING_BACK_ACTION, DrawerGroup } from './StatementDrawer'
import './EvidenceDrawer.css'

export const CHECKED_ACTION = 'I checked this'
export const UNDO_CHECK_ACTION = 'Undo my check'

export interface EvidenceDrawerProps {
  evidence: Evidence
  detail: InvestigationDetail
  /** Pushes the drawer of a claim or hypothesis that uses this evidence. */
  onOpenRef: (ref: string) => void
  /** Called after the evidence was deleted (the drawer stack closes). */
  onDeleted?: () => void
}

export function EvidenceDrawer({ evidence: e, detail, onOpenRef, onDeleted }: EvidenceDrawerProps) {
  const retired = e.verification === 'superseded'
  const metadata = prettyMetadata(e.metadata)
  const users = usedBy(e, detail)
  const records = new Map([...detail.claims, ...detail.hypotheses].map((r) => [r.ref, r] as const))

  return (
    <>
      <div className="drawer-group evidence-head">
        <p className="drawer-meta-line">
          <VerificationBadge verification={e.verification} />
          {e.imported && <ImportedTag />}
        </p>
        <p className={['evidence-drawer-title', retired ? 'is-retired' : ''].filter(Boolean).join(' ')}>{e.title}</p>
        {e.summary && <p className="evidence-drawer-summary">{e.summary}</p>}
        <EvidenceActions
          evidence={e}
          invRef={detail.investigation.ref}
          linkedBy={users.map((u) => u.ref)}
          onDeleted={onDeleted}
        />
      </div>

      {e.source_ref && (
        <DrawerGroup title="Where to find it">
          <p className="source-reference">
            <BreakablePath text={e.source_ref} />
          </p>
        </DrawerGroup>
      )}

      {e.has_raw_detail && <RawDetail evidence={e} invRef={detail.investigation.ref} />}

      <DrawerGroup title="Kind" className="codes-only">
        <p>{sourceKindWord[e.source_type] ?? e.source_type}</p>
      </DrawerGroup>

      {metadata !== '' && (
        <DrawerGroup title="Extra details" className="codes-only">
          <pre className="raw-detail">{metadata}</pre>
        </DrawerGroup>
      )}

      <DrawerGroup title="Findings that use this source">
        {users.length === 0 ? (
          <p className="drawer-empty">No finding uses this source yet.</p>
        ) : (
          <ul className="used-by-list">
            {users.map((u) => {
              const rec = records.get(u.ref)
              const retired = rec ? isReplaced(rec) || isRuledOut(rec) : false
              return (
                <li key={u.ref}>
                  <button
                    type="button"
                    className={['used-by-item', kindRuleClass(u.kind, rec ?? {})].join(' ')}
                    onClick={() => onOpenRef(u.ref)}
                  >
                    <span className="used-by-line">
                      <span className="used-by-ref tabular">{u.ref}</span>
                      {/* One label a row: a finding that is no longer counted says only that. */}
                      {!retired && (
                        <span className={`used-by-relation used-by-${u.relation}`}>
                          {relationTags[u.relation]}
                        </span>
                      )}
                      {retired && rec && (
                        <span className="used-by-status">
                          {rec.status === 'superseded' ? REPLACED : RULED_OUT}
                          {rec.status === 'superseded' && rec.superseded_by && (
                            <>
                              {' '}
                              <RecordId>by {rec.superseded_by}</RecordId>
                            </>
                          )}
                        </span>
                      )}
                    </span>
                    <span className={['used-by-statement', retired ? 'is-retired' : ''].filter(Boolean).join(' ')}>
                      {u.statement}
                    </span>
                  </button>
                </li>
              )
            })}
          </ul>
        )}
      </DrawerGroup>
    </>
  )
}

export const REPLACED_SOURCE_LABEL = 'A newer source replaces this'
export const supersedeEvidenceBody = `This source stays here, crossed out, and is no longer counted. Add the newer source. To use this one again, choose "${BRING_BACK_ACTION}".`

export const deleteEvidenceBody = (e: Evidence) =>
  `"${e.title}" will be removed from this question with everything saved about it.`
export const evidenceLinkedBlocker = (linkedBy: string[]) =>
  `To delete this source, first open ${linkedBy.length === 1 ? 'the finding' : `each of the ${linkedBy.length} findings`} listed under Findings that use this source and choose Remove from this finding.`

function EvidenceActions({
  evidence: e,
  invRef,
  linkedBy,
  onDeleted,
}: {
  evidence: Evidence
  invRef: string
  linkedBy: string[]
  onDeleted?: () => void
}) {
  const write = useWrite()
  const toast = useToast()
  const [busy, setBusy] = useState(false)
  const [editing, setEditing] = useState(false)
  const [deleting, setDeleting] = useState(false)
  const [superseding, setSuperseding] = useState(false)
  const checked = e.verification === 'human_verified'
  const replaced = e.verification === 'superseded'

  const setVerification = async (v: Verification) => {
    if (busy) return
    setBusy(true)
    try {
      await write(() => api.setEvidenceVerification(invRef, e.ref, v))
    } catch (err) {
      toast.error(errorCopy(toApiError(err)).message)
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="evidence-actions">
      <Button variant="secondary" icon={Pencil} onClick={() => setEditing(true)}>
        Edit
      </Button>
      {replaced ? (
        // It comes back as Not checked: bringing it back is not checking it.
        <Button variant="secondary" disabled={busy} onClick={() => void setVerification('ai_generated')}>
          {BRING_BACK_ACTION}
        </Button>
      ) : checked ? (
        <Button variant="secondary" disabled={busy} onClick={() => void setVerification('ai_generated')}>
          {UNDO_CHECK_ACTION}
        </Button>
      ) : (
        <Button variant="primary" disabled={busy} onClick={() => void setVerification('human_verified')}>
          {CHECKED_ACTION}
        </Button>
      )}
      <Menu
        label="More"
        align="end"
        items={[
          {
            label: REPLACED_SOURCE_LABEL,
            icon: Replace,
            disabled: busy || replaced,
            onSelect: () => setSuperseding(true),
          },
          {
            label: 'Delete source...',
            icon: Trash2,
            separatorBefore: true,
            onSelect: () => setDeleting(true),
          },
        ]}
        trigger={(p) => <IconButton {...p} icon={Ellipsis} label="More" />}
      />
      {editing && <EvidenceDialog mode="edit" evidence={e} invRef={invRef} onClose={() => setEditing(false)} />}
      {superseding && (
        <ConfirmDialog
          title="Did a newer source replace this one?"
          body={supersedeEvidenceBody}
          confirmLabel="Yes, it was replaced"
          confirmVariant="primary"
          onConfirm={() => {
            setSuperseding(false)
            void setVerification('superseded')
          }}
          onCancel={() => setSuperseding(false)}
        />
      )}
      {deleting && (
        <DeleteDialog
          title="Delete this source?"
          body={deleteEvidenceBody(e)}
          blocker={linkedBy.length > 0 ? evidenceLinkedBlocker(linkedBy) : null}
          blockerTitle={linkedBy.length === 1 ? 'This source is used by a finding' : 'This source is used by findings'}
          confirmLabel="Delete source"
          onConfirm={async () => {
            const done = await write(() => api.deleteEvidence(invRef, e.ref).then(() => true), { rethrowConflict: true })
            if (done) {
              toast.success('Deleted the source.')
              onDeleted?.()
            }
            return Boolean(done)
          }}
          onClose={() => setDeleting(false)}
        />
      )}
    </div>
  )
}

/**
 * The exact words on demand: nothing is fetched until "Show the exact words".
 * While the fetch runs nothing changes for 300 ms, then a quiet loading line
 * appears. When the source changes while the text is shown (the detail's
 * updated_at moves), it is fetched again and the previous text stays until
 * the new one arrives. The cache is keyed on the detail's updated_at at
 * request time, not on GetEvidence's: when the source changed between the
 * detail load and GetEvidence the two differ, and comparing them would
 * refetch in a loop until the detail reloads.
 */
function RawDetail({ evidence: e, invRef }: { evidence: Evidence; invRef: string }) {
  const toast = useToast()
  const [shown, setShown] = useState(false)
  const [loading, setLoading] = useState(false)
  const [raw, setRaw] = useState<{ text: string; requestedFor: string } | null>(null)
  const slow = useDelayedFlag(loading)
  const seq = useRef(0)
  const version = e.updated_at

  const load = useCallback(async () => {
    const n = ++seq.current
    setLoading(true)
    try {
      const full = await api.getEvidence(invRef, e.ref, { include_raw_detail: true })
      if (n !== seq.current) return
      setRaw({ text: full.raw_detail ?? '', requestedFor: version })
    } catch (err) {
      if (n !== seq.current) return
      toast.error(errorCopy(toApiError(err)).message)
      setShown(false)
    } finally {
      if (n === seq.current) setLoading(false)
    }
  }, [invRef, e.ref, version, toast])

  useEffect(() => {
    if (shown && raw && raw.requestedFor !== version && !loading) void load()
  }, [shown, raw, version, loading, load])

  const toggle = () => {
    if (shown) {
      setShown(false)
      return
    }
    setShown(true)
    if (!raw || raw.requestedFor !== version) void load()
  }

  return (
    <DrawerGroup
      title="Exact words or numbers"
      action={
        <Button variant="ghost" className="disclosure-button" aria-expanded={shown} onClick={toggle}>
          <ChevronDown
            className={['disclosure-icon', shown ? 'is-open' : ''].filter(Boolean).join(' ')}
            size={16}
            strokeWidth={1.75}
            aria-hidden="true"
          />
          {shown ? 'Hide the exact words' : 'Show the exact words'}
        </Button>
      }
    >
      {shown && slow && !raw && <p className="drawer-empty">Loading the exact words...</p>}
      {shown && raw && <pre className="raw-detail">{raw.text}</pre>}
    </DrawerGroup>
  )
}
