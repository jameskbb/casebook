// Decoding of the error JSON the Go bindings reject with, plus the plain
// sentence each error becomes. The engine's own wording and field names never
// reach the screen: plainError maps the messages a person can run into and
// gives everything else one generic sentence.

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

export interface ErrorCopy {
  message: string
  /** Retry repeats the failed call; reload fetches the list and the open question again. */
  action?: 'retry' | 'reload'
}

export const BUSY_COPY = 'Casebook is busy with another change. Nothing was saved.'
export const INTERNAL_COPY =
  'Something went wrong and nothing was saved. Try again. If it keeps happening, save a backup and restart Casebook.'
export const NO_ACTIVE_COPY = 'No question is open. Choose one from the list.'
export const CLIPBOARD_COPY = 'Could not copy. Try again.'
export const DIALOG_COPY = 'Could not open the window for choosing a file. Try again.'
export const GONE_COPY = 'Casebook could not find that. Your AI may have changed it.'
export const REQUIRED_COPY = 'Write something here first.'
export const TOO_LONG_COPY = 'This is too long. Shorten it.'
export const FILE_COPY = 'Casebook cannot read this file. It may not be a Casebook file, or it was saved by a newer version.'
export const NEWER_FILE_COPY = 'This file was saved by a newer Casebook. Update Casebook to open it.'
export const CHANGED_COPY = 'This changed while you had it open. Your AI may have updated it. Look at the new version, then try again.'
export const NOT_SAVED_COPY = 'That did not save. Try again.'
export const WENT_WRONG_COPY = 'Something went wrong. Try again.'

// The engine messages a person can run into, each with its plain sentence.
// First match wins; the engine text itself is never shown.
const RULES: [RegExp, string][] = [
  [/Choose a backup file instead/i, 'That is the file Casebook could not open. Choose a backup instead.'],
  [/saved by a newer Casebook/i, NEWER_FILE_COPY],
  [/cannot read this file|not a Casebook question file/i, FILE_COPY],
  [/byte offset|not valid JSON|unsupported (version|record)|unknown field|not a valid Casebook|not in this export|not in the file|duplicate key|wrong JSON type|not a usable file|regular file/i, FILE_COPY],
  [/\bis required\b|explain what you checked|must not be empty/i, REQUIRED_COPY],
  [/at most [\d,]+ (bytes|characters)|too long/i, TOO_LONG_COPY],
  [/changed; (show|reload)/i, CHANGED_COPY],
  [/was (itself )?replaced/i, 'This finding was replaced or ruled out. Bring it back first.'],
  [/bring back or delete/i, 'This finding replaced another one. Bring that one back or delete it first.'],
  [/part of the answer/i, 'The answer is based on this finding, so it was not deleted. Try again.'],
  [/unlink it first/i, 'A finding uses this source. Remove it from that finding first.'],
  [/is not archived/i, 'Archive this question before you delete it.'],
  [/cannot replace itself|not itself/i, 'A finding cannot replace itself. Choose another one.'],
  [/already exists/i, 'A file with that name already exists. Choose a new name.'],
  [/even though its sources disagree/i, 'The sources disagree about this finding. Say that you checked it anyway.'],
  [/only a person/i, 'Only a person can check this.'],
  [/valid JSON object/i, NOT_SAVED_COPY],
]

/** The one plain sentence for an error. Never the engine's own text. */
export function plainError(err: ApiError): string {
  switch (err.kind) {
    case 'busy':
      return BUSY_COPY
    case 'internal':
      return INTERNAL_COPY
    case 'not_found':
      return GONE_COPY
    case 'no_active':
      return NO_ACTIVE_COPY
    case 'other':
      if (err.field === 'clipboard') return CLIPBOARD_COPY
      if (err.field === 'dialog') return DIALOG_COPY
  }
  for (const [re, copy] of RULES) if (re.test(err.message)) return copy
  return err.kind === 'other' ? WENT_WRONG_COPY : NOT_SAVED_COPY
}

/** The toast copy for an error, with the action it offers. */
export function errorCopy(err: ApiError): ErrorCopy {
  const message = plainError(err)
  if (err.kind === 'busy' || err.kind === 'internal' || (err.kind === 'other' && err.field === 'clipboard')) return { message, action: 'retry' }
  if (err.kind === 'not_found') return { message, action: 'reload' }
  return { message }
}
