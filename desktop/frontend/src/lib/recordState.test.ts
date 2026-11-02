import { describe, expect, it } from 'vitest'
import { statementMarks } from './recordState'

describe('statementMarks (ADR-0013)', () => {
  it('shows a verified active claim as its badge only', () => {
    expect(statementMarks('claim', { status: 'active', verification: 'human_verified' })).toEqual({
      badge: 'human_verified',
      status: 'active',
      disputed: false,
      superseded: false,
    })
  })

  it('shows a disputed claim once, whichever half says so', () => {
    for (const r of [
      { status: 'disputed', verification: 'disputed' },
      { status: 'disputed', verification: 'human_verified' },
      { status: 'active', verification: 'disputed' },
    ] as const) {
      expect(statementMarks('claim', r)).toMatchObject({ badge: null, status: 'disputed', disputed: true })
    }
  })

  it('lets a superseded status win over any verification and hides a stray superseded verification', () => {
    expect(statementMarks('claim', { status: 'superseded', verification: 'human_verified' })).toMatchObject({
      badge: null,
      status: 'superseded',
      superseded: true,
    })
    expect(statementMarks('claim', { status: 'active', verification: 'superseded' })).toMatchObject({
      badge: null,
      status: 'active',
      superseded: false,
    })
    expect(statementMarks('hypothesis', { status: 'open', verification: 'superseded' }).badge).toBeNull()
  })

  it('keeps disputed as a free verification on a hypothesis', () => {
    expect(statementMarks('hypothesis', { status: 'open', verification: 'disputed' })).toEqual({
      badge: 'disputed',
      status: 'open',
      disputed: true,
      superseded: false,
    })
  })
})
