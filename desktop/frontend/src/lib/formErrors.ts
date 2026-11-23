// Splits a write error into per-field messages and a form summary: a
// validation error on a field the form shows goes under that field; any
// other goes into the summary. Both are plain sentences (plainError): the
// engine's field names and wording never reach the screen.
import { ApiError, plainError, toApiError } from '../api/errors'

export interface FormErrors {
  fieldErrors: Record<string, string>
  summary?: string
}

export function splitFieldErrors(e: unknown, knownFields: readonly string[]): FormErrors {
  const err: ApiError = toApiError(e)
  if (err.kind === 'validation' && err.field && knownFields.includes(err.field)) {
    return { fieldErrors: { [err.field]: plainError(err) } }
  }
  return { fieldErrors: {}, summary: plainError(err) }
}
