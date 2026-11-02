// Splits a write error into per-field messages and a form summary (UI-SPEC
// Errors table: a validation error on a field the form shows goes under that
// field; any other goes into "Check this entry: {field}: {message}").
import { ApiError, cleanMessage, toApiError } from '../api/errors'

export interface FormErrors {
  fieldErrors: Record<string, string>
  summary?: string
}

export function splitFieldErrors(e: unknown, knownFields: readonly string[]): FormErrors {
  const err: ApiError = toApiError(e)
  if (err.kind === 'validation' && err.field && knownFields.includes(err.field)) {
    return { fieldErrors: { [err.field]: err.message } }
  }
  if (err.kind === 'validation' && err.field) {
    return { fieldErrors: {}, summary: `Check this entry: ${err.field}: ${err.message}` }
  }
  return { fieldErrors: {}, summary: cleanMessage(err.message) }
}
