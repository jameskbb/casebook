// Decoding of the D-05 error JSON the Go bindings reject with, plus the
// message cleanup the UI-SPEC Errors table asks for.

export type ErrorKind =
  | 'validation'
  | 'not_found'
  | 'conflict'
  | 'no_active'
  | 'busy'
  | 'internal'
  | 'other'

const KINDS: readonly ErrorKind[] = [
  'validation',
  'not_found',
  'conflict',
  'no_active',
  'busy',
  'internal',
  'other',
]

/** An error from a Desktop binding: kind, optional field and message. */
export class ApiError extends Error {
  readonly kind: ErrorKind
  readonly field?: string

  constructor(kind: ErrorKind, message: string, field?: string) {
    super(message)
    this.name = 'ApiError'
    this.kind = kind
    if (field) this.field = field
  }
}

function isKind(v: unknown): v is ErrorKind {
  return typeof v === 'string' && (KINDS as readonly string[]).includes(v)
}

/**
 * Turns whatever a binding rejected with into an ApiError. The Go side
 * rejects with the JSON string {kind, field?, message}; anything else (a
 * runtime failure, a non-JSON string) becomes kind "other".
 */
export function toApiError(e: unknown): ApiError {
  if (e instanceof ApiError) return e
  const text = typeof e === 'string' ? e : e instanceof Error ? e.message : String(e)
  try {
    const parsed: unknown = JSON.parse(text)
    if (parsed && typeof parsed === 'object') {
      const o = parsed as { kind?: unknown; field?: unknown; message?: unknown }
      if (isKind(o.kind) && typeof o.message === 'string') {
        return new ApiError(o.kind, o.message, typeof o.field === 'string' && o.field ? o.field : undefined)
      }
    }
  } catch {
    // Not JSON: fall through.
  }
  return new ApiError('other', text)
}

/**
 * Removes a leading "not found: " or "conflict: " from an engine message and
 * capitalizes the first letter ("not found: claim C-009" becomes
 * "Claim C-009").
 */
export function cleanMessage(msg: string): string {
  let m = msg.trim()
  for (const prefix of ['not found: ', 'conflict: ']) {
    if (m.toLowerCase().startsWith(prefix)) {
      m = m.slice(prefix.length)
      break
    }
  }
  return m.charAt(0).toUpperCase() + m.slice(1)
}

export interface ErrorCopy {
  message: string
  /** Retry repeats the failed call; Refresh reloads the list and detail. */
  action?: 'retry' | 'refresh'
}

export const BUSY_COPY = 'Casebook is busy with another change. Nothing was saved.'
export const INTERNAL_COPY =
  'Something went wrong and nothing was saved. Try again. If it keeps happening, back up your data and restart Casebook.'
export const NO_ACTIVE_COPY = 'No case is selected. Choose one from the list.'
export const CLIPBOARD_COPY = 'Could not copy to the clipboard. Try again.'
export const DIALOG_COPY = 'Could not open the file window. Try again.'

/** cleanMessage plus a closing period when the message has none. */
export function asSentence(msg: string): string {
  const m = cleanMessage(msg)
  return /[.!?]$/.test(m) ? m : `${m}.`
}

/**
 * The toast copy for an error (UI-SPEC Errors table). Validation errors are
 * normally shown next to their field; errorCopy gives the summary form for
 * when no field can show them.
 */
export function errorCopy(err: ApiError): ErrorCopy {
  switch (err.kind) {
    case 'busy':
      return { message: BUSY_COPY, action: 'retry' }
    case 'internal':
      return { message: INTERNAL_COPY, action: 'retry' }
    case 'not_found':
      return { message: `${asSentence(err.message)} It may have changed in another app.`, action: 'refresh' }
    case 'no_active':
      return { message: NO_ACTIVE_COPY }
    case 'conflict':
      return { message: cleanMessage(err.message) }
    case 'validation':
      return { message: err.field ? `Check this entry: ${err.field}: ${err.message}` : cleanMessage(err.message) }
    case 'other':
    default:
      if (err.field === 'clipboard') return { message: CLIPBOARD_COPY, action: 'retry' }
      if (err.field === 'dialog') return { message: DIALOG_COPY }
      return { message: cleanMessage(err.message) }
  }
}
