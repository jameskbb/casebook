// Evidence chip (UI-SPEC Component Contract, D-21): a pill naming a piece of
// evidence by its first words, with the ref after it in muted small caps.
// Outline style follows the link's record kind (claim solid, hypothesis
// dashed); outline color follows the relation (supports in the kind hue,
// contradicts in brick with a Minus mark at the start). It is a button only
// when onOpen is passed, so no control renders without an action.
import { Minus } from 'lucide-react'
import type { Relation } from '../../api/types'
import './EvidenceChip.css'

export interface EvidenceChipProps {
  /** The ref shown on the chip and passed to onOpen. */
  target: string
  relation: Relation
  /** The record kind the link belongs to: claim (solid) or hypothesis (dashed). */
  linkKind: 'claim' | 'hypothesis'
  /**
   * "record": the chip sits on a claim or hypothesis row and names evidence.
   * "evidence": it sits on an evidence row and names a claim or hypothesis.
   */
  placement?: 'record' | 'evidence'
  /** Evidence title (record placement) or statement (evidence placement). */
  text: string
  onOpen?: (ref: string) => void
}

/** "E-001 supports this: <title>" or "Contradicts H-002: <statement>". */
export function chipLabel(p: Pick<EvidenceChipProps, 'target' | 'relation' | 'placement' | 'text'>): string {
  if (p.placement === 'evidence') {
    return `${p.relation === 'supports' ? 'Supports' : 'Contradicts'} ${p.target}: ${p.text}`
  }
  return `${p.target} ${p.relation} this: ${p.text}`
}

/**
 * The first words of a title that fit in max characters, ending in three
 * periods when cut. A single word longer than max is cut hard.
 */
export function shortLabel(text: string, max = 28): string {
  const t = text.trim().replace(/\s+/g, ' ')
  if (t.length <= max) return t
  const cut = t.slice(0, max - 3)
  const space = cut.lastIndexOf(' ')
  return (space > 0 ? cut.slice(0, space) : cut) + '...'
}

export function EvidenceChip({ target, relation, linkKind, placement = 'record', text, onOpen }: EvidenceChipProps) {
  const label = chipLabel({ target, relation, placement, text })
  const className = ['chip', `chip-${relation}`, `chip-${linkKind}`, onOpen ? 'chip-button' : ''].filter(Boolean).join(' ')
  const content = (
    <>
      {relation === 'contradicts' && <Minus className="chip-minus" size={12} strokeWidth={1.75} aria-hidden="true" />}
      {placement === 'record' && <span className="chip-text">{shortLabel(text)}</span>}
      <span className="chip-ref tabular">{target}</span>
    </>
  )
  if (onOpen) {
    return (
      <button type="button" className={className} aria-label={label} title={label} onClick={() => onOpen(target)}>
        {content}
      </button>
    )
  }
  return (
    <span className={className} aria-label={label} title={label}>
      {content}
    </span>
  )
}
