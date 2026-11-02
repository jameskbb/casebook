import { describe, expect, it } from 'vitest'
import {
  ApiError,
  BUSY_COPY,
  CLIPBOARD_COPY,
  DIALOG_COPY,
  INTERNAL_COPY,
  NO_ACTIVE_COPY,
  cleanMessage,
  errorCopy,
  toApiError,
} from './errors'

describe('toApiError', () => {
  it('decodes the D-05 JSON string', () => {
    const e = toApiError('{"kind":"not_found","message":"not found: claim C-009 in INV-0001"}')
    expect(e).toBeInstanceOf(ApiError)
    expect(e.kind).toBe('not_found')
    expect(e.field).toBeUndefined()
    expect(e.message).toBe('not found: claim C-009 in INV-0001')
  })

  it('keeps the field of a validation error', () => {
    expect(toApiError('{"kind":"validation","field":"links[1].relation","message":"is invalid"}')).toMatchObject({
      kind: 'validation',
      field: 'links[1].relation',
    })
  })

  it('treats anything else as kind other', () => {
    expect(toApiError('plain text')).toMatchObject({ kind: 'other', message: 'plain text' })
    expect(toApiError('{"kind":"weird","message":"x"}')).toMatchObject({ kind: 'other' })
    expect(toApiError(new Error('boom'))).toMatchObject({ kind: 'other', message: 'boom' })
    expect(toApiError(42)).toMatchObject({ kind: 'other', message: '42' })
    expect(toApiError(undefined)).toMatchObject({ kind: 'other' })
  })

  it('returns an ApiError unchanged', () => {
    const e = new ApiError('busy', 'x')
    expect(toApiError(e)).toBe(e)
  })
})

describe('cleanMessage', () => {
  it('strips the engine prefixes and capitalizes', () => {
    expect(cleanMessage('not found: claim C-009 in INV-0001')).toBe('Claim C-009 in INV-0001')
    expect(cleanMessage('conflict: a file already exists at the destination')).toBe(
      'A file already exists at the destination',
    )
    expect(cleanMessage('already fine')).toBe('Already fine')
  })
})

describe('errorCopy', () => {
  it('maps every kind to the UI-SPEC copy', () => {
    expect(errorCopy(new ApiError('busy', 'Casebook database is busy; try again'))).toEqual({
      message: BUSY_COPY,
      action: 'retry',
    })
    expect(errorCopy(new ApiError('internal', 'casebook: internal storage error'))).toEqual({
      message: INTERNAL_COPY,
      action: 'retry',
    })
    expect(errorCopy(new ApiError('not_found', 'not found: claim C-009 in INV-0001'))).toEqual({
      message: 'Claim C-009 in INV-0001. It may have changed in another app.',
      action: 'refresh',
    })
    expect(errorCopy(new ApiError('conflict', 'conflict: C-002 is already superseded'))).toEqual({
      message: 'C-002 is already superseded',
    })
    expect(errorCopy(new ApiError('no_active', 'no active investigation is set'))).toEqual({ message: NO_ACTIVE_COPY })
    expect(errorCopy(new ApiError('validation', 'is required', 'title'))).toEqual({
      message: 'Check this entry: title: is required',
    })
    expect(errorCopy(new ApiError('other', 'open C:\\x.db: access is denied'))).toEqual({
      message: 'Open C:\\x.db: access is denied',
    })
  })

  it('gives the native failure copy for clipboard and dialog errors', () => {
    expect(errorCopy(new ApiError('other', 'locked', 'clipboard'))).toEqual({ message: CLIPBOARD_COPY, action: 'retry' })
    expect(errorCopy(new ApiError('other', 'no window', 'dialog'))).toEqual({ message: DIALOG_COPY })
  })

  it('never contains an em dash', () => {
    for (const c of [BUSY_COPY, INTERNAL_COPY, NO_ACTIVE_COPY, CLIPBOARD_COPY, DIALOG_COPY]) {
      expect(c).not.toContain('\u2014')
    }
  })
})
