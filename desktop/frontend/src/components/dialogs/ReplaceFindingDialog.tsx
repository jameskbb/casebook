// Replace a finding: one field for the newer finding's words. Saving adds
// the new finding, with the same sources as the old one, and replaces the
// old one with it. The old one stays under Show replaced findings.
import { useRef, useState } from 'react'
import { api } from '../../api/api'
import type { EvidenceLinkInput } from '../../api/types'
import { LIMITS, nearLimit } from '../../lib/bytes'
import { splitFieldErrors } from '../../lib/formErrors'
import type { Finding } from '../../lib/records'
import { useWrite } from '../../state/useWrite'
import { Dialog } from '../ui/Dialog'
import { TextArea } from '../ui/TextArea'

export const REPLACE_TITLE = 'Replace with a newer finding'
export const REPLACE_LEDE = 'Write the finding as it should read now. It keeps the same sources. The old one stays, crossed out.'

/**
 * Adds the newer finding, as Not checked (nobody has decided it holds), and
 * replaces the old one with it. Two calls: when
 * the second is refused the new finding is already there, and the error says
 * what happened.
 */
export async function replaceWithNew(invRef: string, old: Finding, statement: string): Promise<{ ref: string }> {
  const links: EvidenceLinkInput[] = old.record.links.map((l) => ({ evidence_ref: l.evidence_ref, relation: l.relation }))
  if (old.kind === 'claim') {
    const added = await api.addClaim(invRef, { statement, confidence: 'unknown', confidence_rationale: '', verification: 'ai_generated', links })
    await api.supersedeClaim(invRef, old.record.ref, added.ref)
    return added
  }
  // One that had turned out right stays out of Still to find out.
  const status = old.record.status === 'supported' ? 'supported' : ''
  const added = await api.addHypothesis(invRef, { statement, confidence: 'unknown', rationale: '', status, verification: 'ai_generated', links })
  await api.supersedeHypothesis(invRef, old.record.ref, added.ref)
  return added
}

export function ReplaceFindingDialog({ finding, invRef, onClose, onReplaced }: { finding: Finding; invRef: string; onClose: () => void; onReplaced?: (ref: string) => void }) {
  const write = useWrite()
  const [text, setText] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | undefined>()
  const [summary, setSummary] = useState<string | undefined>()

  // Try again from a message sends what the form holds now.
  const submitRef = useRef<() => Promise<void>>(async () => {})
  const submit = async () => {
    if (busy) return
    setBusy(true)
    setError(undefined)
    setSummary(undefined)
    try {
      const saved = await write(() => replaceWithNew(invRef, finding, text), { rethrowConflict: true, onRetry: () => void submitRef.current() })
      if (saved) {
        onClose()
        onReplaced?.(saved.ref)
      }
    } catch (e) {
      const split = splitFieldErrors(e, ['statement'])
      setError(split.fieldErrors.statement)
      setSummary(split.summary)
    } finally {
      setBusy(false)
    }
  }
  submitRef.current = submit

  return (
    <Dialog
      title={REPLACE_TITLE}
      size="confirm"
      onClose={onClose}
      onSubmit={() => void submit()}
      submitLabel="Replace finding"
      busyLabel="Replacing..."
      busy={busy}
      dirty={text !== ''}
      summary={
        summary ? (
          <p className="dialog-summary" role="alert">
            {summary}
          </p>
        ) : undefined
      }
    >
      <p className="dialog-text dialog-quote">{finding.record.statement}</p>
      <p className="dialog-text">{REPLACE_LEDE}</p>
      <TextArea label="The newer finding" value={text} onChange={setText} error={error} note={nearLimit(text, LIMITS.statement)} spellCheck />
    </Dialog>
  )
}
