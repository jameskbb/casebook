import { describe, expect, it } from 'vitest'
import {
  ApiError,
  BUSY_COPY,
  CHANGED_COPY,
  CLIPBOARD_COPY,
  DIALOG_COPY,
  FILE_COPY,
  GONE_COPY,
  INTERNAL_COPY,
  NOT_SAVED_COPY,
  NO_ACTIVE_COPY,
  REQUIRED_COPY,
  TOO_LONG_COPY,
  WENT_WRONG_COPY,
  errorCopy,
  plainError,
  toApiError,
} from './errors'

describe('toApiError', () => {
  it('decodes the JSON error string', () => {
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

describe('plainError', () => {
  it('maps the engine messages a person can run into to their plain sentence', () => {
    expect(plainError(new ApiError('validation', 'is required', 'title'))).toBe(REQUIRED_COPY)
    expect(REQUIRED_COPY).toBe('Write something here first.')
    expect(plainError(new ApiError('validation', 'must be at most 200 bytes', 'title'))).toBe(TOO_LONG_COPY)
    expect(plainError(new ApiError('validation', 'invalid character at byte offset 12', 'file'))).toBe(FILE_COPY)
    expect(plainError(new ApiError('validation', 'unsupported version 2', 'version'))).toBe(FILE_COPY)
    expect(plainError(new ApiError('conflict', 'conflict: this finding or its evidence changed; show the latest version before reviewing'))).toBe(CHANGED_COPY)
    expect(plainError(new ApiError('conflict', 'conflict: the answer, its findings or their sources changed; reload and review the latest version'))).toBe(CHANGED_COPY)
  })

  it('falls back to one plain sentence for anything it does not know', () => {
    expect(plainError(new ApiError('validation', 'is invalid', 'links[1].relation'))).toBe(NOT_SAVED_COPY)
    expect(plainError(new ApiError('conflict', 'conflict: C-002 is already superseded'))).toBe(NOT_SAVED_COPY)
    expect(plainError(new ApiError('other', 'open C:\\x.db: access is denied'))).toBe(WENT_WRONG_COPY)
    expect(plainError(new ApiError('not_found', 'not found: claim C-009 in INV-0001'))).toBe(GONE_COPY)
  })

  it('never returns the engine message or a field name', () => {
    const errors = [
      new ApiError('validation', 'is required', 'problem_statement'),
      new ApiError('validation', 'must be at most 200 bytes', 'source_label'),
      new ApiError('validation', 'is invalid', 'links[1].relation'),
      new ApiError('validation', 'unsupported version 2', 'format_version'),
      new ApiError('conflict', 'conflict: C-002 is already superseded'),
      new ApiError('conflict', 'conflict: this finding or its evidence changed; show the latest version before reviewing'),
      new ApiError('conflict', 'conflict: E-009 is linked by 1 claim link and 0 hypothesis links; unlink it first'),
      new ApiError('not_found', 'not found: claim C-009 in INV-0001'),
      new ApiError('other', 'open C:\\x.db: access is denied'),
      new ApiError('other', 'sql: no rows in result set', 'raw_detail'),
    ]
    for (const err of errors) {
      const text = plainError(err)
      expect(text, err.message).toMatch(/^[A-Z].*\.$/)
      expect(text, err.message).not.toContain(err.message)
      expect(text, err.message).not.toContain(err.message.replace(/^(conflict|not found): /, ''))
      if (err.field) expect(text, err.message).not.toContain(err.field)
      expect(text, err.message).not.toMatch(/\b[CEHQ]-\d+\b|INV-\d+|claim|evidence|hypothesis|superseded|bytes|_/)
    }
  })
})

describe('errorCopy', () => {
  it('maps every kind to its toast copy and action', () => {
    expect(errorCopy(new ApiError('busy', 'Casebook database is busy; try again'))).toEqual({
      message: BUSY_COPY,
      action: 'retry',
    })
    expect(errorCopy(new ApiError('internal', 'casebook: internal storage error'))).toEqual({
      message: INTERNAL_COPY,
      action: 'retry',
    })
    expect(errorCopy(new ApiError('not_found', 'not found: claim C-009 in INV-0001'))).toEqual({
      message: GONE_COPY,
      action: 'reload',
    })
    expect(errorCopy(new ApiError('conflict', 'conflict: C-002 is already superseded'))).toEqual({ message: NOT_SAVED_COPY })
    expect(errorCopy(new ApiError('no_active', 'no active investigation is set'))).toEqual({ message: NO_ACTIVE_COPY })
    expect(errorCopy(new ApiError('validation', 'is required', 'title'))).toEqual({ message: REQUIRED_COPY })
    expect(errorCopy(new ApiError('other', 'open C:\\x.db: access is denied'))).toEqual({ message: WENT_WRONG_COPY })
  })

  it('gives the native failure copy for clipboard and dialog errors', () => {
    expect(errorCopy(new ApiError('other', 'locked', 'clipboard'))).toEqual({ message: CLIPBOARD_COPY, action: 'retry' })
    expect(errorCopy(new ApiError('other', 'no window', 'dialog'))).toEqual({ message: DIALOG_COPY })
  })

  it('never contains an em dash', () => {
    for (const c of [BUSY_COPY, INTERNAL_COPY, NO_ACTIVE_COPY, CLIPBOARD_COPY, DIALOG_COPY, CHANGED_COPY, FILE_COPY, GONE_COPY, NOT_SAVED_COPY, REQUIRED_COPY, TOO_LONG_COPY, WENT_WRONG_COPY]) {
      expect(c).not.toContain('\u2014')
    }
  })
})
