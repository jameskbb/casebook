// Evidence drawer body (D-21, UI-15; UI-SPEC Drawers, Evidence drawer): the
// title, verification and full summary; the source as a definition list
// that omits missing fields; raw detail fetched on demand through GetEvidence
// with include_raw_detail; metadata behind a disclosure; and the claims and
// hypotheses that use it, each opening its drawer.
//
// Raw detail and metadata are engine text written by people, AI clients and
// imports, so they render as text inside a pre block (never markup), and
// references are plain selectable text (never links).
//
// Actions (UI-07, UI-14): Edit evidence opens the evidence dialog over the
// drawer; Verify, Dispute and the More menu (Mark superseded, after a
// confirmation, and Mark source verified) set the verification, and Delete
// evidence... removes it (AR-10).
import { useCallback, useEffect, useRef, useState } from 'react'
import { BadgeCheck, ChevronDown, Ellipsis, Minus, Pencil, Replace, Trash2 } from 'lucide-react'
import { api } from '../../api/api'
import { errorCopy, toApiError } from '../../api/errors'
import type { Claim, Evidence, Hypothesis, InvestigationDetail, Verification } from '../../api/types'
import { useDelayedFlag } from '../../hooks/useDelayedFlag'
import { sourceTypeLabels } from '../../lib/labels'
import { prettyMetadata } from '../../lib/metadata'
import { usedBy } from '../../lib/records'
import { formatAbsolute } from '../../lib/relativeTime'
import { ImportedTag } from '../record/ImportedTag'
import { kindRuleClass } from '../record/KindRule'
import { VerificationBadge } from '../record/VerificationBadge'
import { useWrite } from '../../state/useWrite'
import { DeleteDialog, refList } from '../dialogs/DeleteDialog'
import { EvidenceDialog } from '../dialogs/EvidenceDialog'
import { BreakablePath } from '../ui/BreakablePath'
import { Button } from '../ui/Button'
import { ConfirmDialog } from '../ui/ConfirmDialog'
import { IconButton } from '../ui/IconButton'
import { Menu } from '../ui/Menu'
import { useToast } from '../ui/Toaster'
import { DrawerGroup } from './StatementDrawer'
import './EvidenceDrawer.css'

const RETIRED = new Set(['superseded', 'rejected', 'disproven'])

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
  const captured = formatAbsolute(e.captured_at)
  const metadata = prettyMetadata(e.metadata)
  const users = usedBy(e, detail)
  const records = new Map<string, Claim | Hypothesis>([
    ...detail.claims.map((c) => [c.ref, c] as const),
    ...detail.hypotheses.map((h) => [h.ref, h] as const),
  ])
  const [showMetadata, setShowMetadata] = useState(false)

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

      <DrawerGroup title="Source">
        <dl className="source-list">
          <div className="source-row">
            <dt>Source type</dt>
            <dd>{sourceTypeLabels[e.source_type] ?? e.source_type}</dd>
          </div>
          {e.source_label && (
            <div className="source-row">
              <dt>Source</dt>
              <dd>{e.source_label}</dd>
            </div>
          )}
          {e.source_ref && (
            <div className="source-row">
              <dt>Reference</dt>
              <dd className="source-reference">
                <BreakablePath text={e.source_ref} />
              </dd>
            </div>
          )}
          {captured && (
            <div className="source-row">
              <dt>Captured</dt>
              <dd className="tabular">{captured}</dd>
            </div>
          )}
        </dl>
      </DrawerGroup>

      {e.has_raw_detail && <RawDetail evidence={e} invRef={detail.investigation.ref} />}

      {metadata !== '' && (
        <DrawerGroup
          title="Metadata"
          action={
            <Button
              variant="ghost"
              className="disclosure-button"
              aria-expanded={showMetadata}
              onClick={() => setShowMetadata((v) => !v)}
            >
              <ChevronDown
                className={['disclosure-icon', showMetadata ? 'is-open' : ''].filter(Boolean).join(' ')}
                size={16}
                strokeWidth={1.75}
                aria-hidden="true"
              />
              {showMetadata ? 'Hide metadata' : 'Show metadata'}
            </Button>
          }
        >
          {showMetadata && <pre className="raw-detail">{metadata}</pre>}
        </DrawerGroup>
      )}

      <DrawerGroup title="Used by">
        {users.length === 0 ? (
          <p className="drawer-empty">No claim or hypothesis uses this evidence yet.</p>
        ) : (
          <ul className="used-by-list">
            {users.map((u) => {
              const rec = records.get(u.ref)
              const retired = rec ? RETIRED.has(rec.status) : false
              return (
                <li key={u.ref}>
                  <button
                    type="button"
                    className={['used-by-item', kindRuleClass(u.kind, rec ?? {})].join(' ')}
                    onClick={() => onOpenRef(u.ref)}
                  >
                    <span className="used-by-line">
                      <span className="used-by-ref tabular">{u.ref}</span>
                      <span className={`used-by-relation used-by-${u.relation}`}>
                        {u.relation === 'contradicts' && (
                          <Minus className="used-by-minus" size={12} strokeWidth={1.75} aria-hidden="true" />
                        )}
                        {u.relation === 'supports' ? 'Supports' : 'Contradicts'}
                      </span>
                      {retired && rec && (
                        <span className="used-by-status">
                          {rec.status === 'superseded'
                            ? rec.superseded_by
                              ? `Superseded by ${rec.superseded_by}`
                              : 'Superseded'
                            : rec.status === 'rejected'
                              ? 'Rejected'
                              : 'Disproven'}
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

export const supersedeEvidenceBody = (ref: string) =>
  `${ref} stays in this case, struck through, and AI context packs list it as superseded. Evidence has no replacement link: say what replaced it in a newer piece of evidence. Verify brings it back.`

export const deleteEvidenceBody = (e: Evidence) =>
  `${e.ref} "${e.title}" will be removed from this case with its summary, source, raw detail and metadata.`
export const evidenceLinkedBlocker = (ref: string, linkedBy: string[]) =>
  `${ref} is linked to ${refList(linkedBy)}. Unlink it there first (open each one and choose Unlink), then delete it.`

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
  const verified = e.verification === 'human_verified' || e.verification === 'source_verified'

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
        Edit evidence
      </Button>
      {verified ? (
        <Button variant="secondary" icon={BadgeCheck} disabled>
          Verified
        </Button>
      ) : (
        <Button variant="secondary" disabled={busy} onClick={() => void setVerification('human_verified')}>
          Verify
        </Button>
      )}
      <Button
        variant="secondary"
        disabled={busy || e.verification === 'disputed'}
        onClick={() => void setVerification('disputed')}
      >
        Dispute
      </Button>
      <Menu
        label="More actions"
        align="end"
        items={[
          {
            label: 'Mark superseded',
            icon: Replace,
            disabled: busy || e.verification === 'superseded',
            onSelect: () => setSuperseding(true),
          },
          {
            label: 'Mark source verified',
            icon: BadgeCheck,
            disabled: busy || e.verification === 'source_verified',
            onSelect: () => void setVerification('source_verified'),
          },
          {
            label: 'Delete evidence...',
            icon: Trash2,
            separatorBefore: true,
            onSelect: () => setDeleting(true),
          },
        ]}
        trigger={(p) => <IconButton {...p} icon={Ellipsis} label="More actions" />}
      />
      {editing && <EvidenceDialog mode="edit" evidence={e} invRef={invRef} onClose={() => setEditing(false)} />}
      {superseding && (
        <ConfirmDialog
          title={`Mark ${e.ref} superseded?`}
          body={supersedeEvidenceBody(e.ref)}
          confirmLabel="Mark superseded"
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
          title={`Delete evidence ${e.ref}?`}
          body={deleteEvidenceBody(e)}
          blocker={linkedBy.length > 0 ? evidenceLinkedBlocker(e.ref, linkedBy) : null}
          confirmLabel="Delete evidence"
          onConfirm={async () => {
            const done = await write(() => api.deleteEvidence(invRef, e.ref).then(() => true), { rethrowConflict: true })
            if (done) {
              toast.success(`Deleted evidence ${e.ref}.`)
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
 * Raw detail on demand: nothing is fetched until "Show raw detail". While the
 * fetch runs nothing changes for 300 ms, then a quiet loading line appears.
 * When the record changes elsewhere while the text is shown (the detail's
 * updated_at moves), it is fetched again and the previous text stays until
 * the new one arrives. The cache is keyed on the detail's updated_at at
 * request time, not on GetEvidence's: when the record changed between the
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
      title="Raw detail"
      action={
        <Button variant="ghost" className="disclosure-button" aria-expanded={shown} onClick={toggle}>
          <ChevronDown
            className={['disclosure-icon', shown ? 'is-open' : ''].filter(Boolean).join(' ')}
            size={16}
            strokeWidth={1.75}
            aria-hidden="true"
          />
          {shown ? 'Hide raw detail' : 'Show raw detail'}
        </Button>
      }
    >
      {shown && slow && !raw && <p className="drawer-empty">Loading raw detail...</p>}
      {shown && raw && <pre className="raw-detail">{raw.text}</pre>}
    </DrawerGroup>
  )
}
