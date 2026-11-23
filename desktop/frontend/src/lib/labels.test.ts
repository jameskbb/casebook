import { describe, expect, it } from 'vitest'
import type { Verification } from '../api/types'
import { claims, evidence, hypotheses } from '../test/fixtures'
import {
  AI_WORKING,
  ANSWERED,
  ARCHIVED,
  DECIDED_HOLDS,
  FROM_A_FILE,
  LATER,
  LEGEND,
  NO_ANSWER_YET,
  NO_SOURCE_YET,
  NOT_TESTED,
  relationLabels,
  relationTags,
  REPLACED,
  RULED_OUT,
  sourceLabel,
  SOURCES_DISAGREE,
} from './labels'
import { evidenceByRef, findingLabel, type Finding } from './records'

const VERIFICATIONS: Verification[] = ['ai_generated', 'human_verified', 'source_verified', 'disputed', 'superseded']
const rows = LEGEND.flatMap((g) => g.rows)
/** "1 of 4 sources checked by you" is the legend's "2 of 3 sources checked by you". */
const legendKey = (label: string) => label.replace(/^\d+ of \d+ sources? checked by you$/, '2 of 3 sources checked by you')

describe('labels', () => {
  it('says a source backs a finding or goes against it', () => {
    expect(relationLabels).toEqual({ supports: 'Backs it up', contradicts: 'Goes against it' })
    expect(relationTags).toEqual({ supports: 'Backs it', contradicts: 'Goes against it' })
  })

  it('gives a source one label, and never Sources disagree', () => {
    expect(VERIFICATIONS.map(sourceLabel)).toEqual(['Not checked', 'You checked this', 'AI checked the source', 'Not checked', 'Replaced'])
  })

  it('explains every label in one sentence, once', () => {
    for (const row of rows) {
      expect(row.meaning, row.label).toMatch(/^[A-Z].*\.$/)
      expect(row.meaning.length, row.label).toBeLessThan(120)
    }
    expect(new Set(rows.map((r) => r.label)).size).toBe(rows.length)
  })

  // Every label a row or the header can show, from every state a finding and a source can be in.
  it('lists every label that can appear on the page', () => {
    const sources = evidenceByRef({ evidence })
    const findings: Finding[] = []
    for (const record of claims) {
      for (const status of ['active', 'disputed', 'superseded', 'rejected'] as const) {
        for (const verification of VERIFICATIONS) findings.push({ kind: 'claim', record: { ...record, status, verification } }, { kind: 'claim', record: { ...record, status, verification, links: [] } })
      }
    }
    for (const record of hypotheses) {
      for (const status of ['open', 'supported', 'disproven', 'superseded'] as const) {
        for (const verification of VERIFICATIONS) findings.push({ kind: 'hypothesis', record: { ...record, status, verification } })
      }
    }
    const shown = new Set([
      ...findings.map((f) => legendKey(findingLabel(f, sources))),
      ...VERIFICATIONS.map(sourceLabel),
      ...Object.values(relationTags),
      ANSWERED,
      NO_ANSWER_YET,
      ARCHIVED,
      LATER,
      AI_WORKING,
      FROM_A_FILE,
    ])
    // The walk reached every finding label there is.
    for (const label of [SOURCES_DISAGREE, DECIDED_HOLDS, NO_SOURCE_YET, NOT_TESTED, RULED_OUT, REPLACED]) expect(shown).toContain(label)
    expect([...shown].sort()).toEqual(rows.map((r) => r.label).sort())
  })
})
