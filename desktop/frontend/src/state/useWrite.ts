// The one way UI code writes (Plan 01 contract): run the call, refresh the
// list and detail on success, and on failure show the Errors table toast.
// Validation errors (and conflicts when asked) are rethrown so forms can show
// them next to their fields.
//
// Retry toasts belong to the component that showed them: a later successful
// write from the same component dismisses them (the failed change has been
// made another way, and retrying would duplicate it, leaving a copy to
// delete by hand), and so does unmounting (a closed form must never submit
// again). A Retry clicked after unmount does nothing.
import { useCallback, useEffect, useRef } from 'react'
import { errorCopy, toApiError } from '../api/errors'
import { useToast, type ToastAction } from '../components/ui/Toaster'
import { useAppData } from './appData'

export interface WriteOptions {
  /** Offer Retry on busy and internal errors (default true). */
  retry?: boolean
  /** Rethrow conflict errors instead of showing a toast. */
  rethrowConflict?: boolean
  /**
   * Replaces the default Retry action (repeat fn). Forms pass their latest
   * submit (through a ref, so the retry sends what the form holds now) so a
   * successful retry also closes the form.
   */
  onRetry?: () => void
}

export type Write = <T>(fn: () => Promise<T>, opts?: WriteOptions) => Promise<T | undefined>

export function useWrite(): Write {
  const { refresh } = useAppData()
  const toast = useToast()
  const alive = useRef(true)
  const retryToasts = useRef(new Set<number>())

  const dismissRetries = useCallback(() => {
    for (const id of retryToasts.current) toast.dismiss(id)
    retryToasts.current.clear()
  }, [toast])

  useEffect(() => {
    alive.current = true
    return () => {
      alive.current = false
      dismissRetries()
    }
  }, [dismissRetries])

  const write: Write = useCallback(
    async <T,>(fn: () => Promise<T>, opts: WriteOptions = {}): Promise<T | undefined> => {
      let value: T
      try {
        value = await fn()
      } catch (e) {
        const err = toApiError(e)
        if (err.kind === 'validation') throw err
        if (err.kind === 'conflict' && opts.rethrowConflict) throw err
        const copy = errorCopy(err)
        let action: ToastAction | undefined
        let id = 0
        if (copy.action === 'retry' && opts.retry !== false) {
          const retry = opts.onRetry ?? (() => void write(fn, opts).catch(() => {}))
          action = {
            label: 'Retry',
            onClick: () => {
              retryToasts.current.delete(id)
              if (alive.current) retry()
            },
          }
        } else if (copy.action === 'refresh') {
          action = { label: 'Refresh', onClick: () => void refresh() }
        }
        id = toast.error(copy.message, action)
        if (action?.label === 'Retry') {
          if (alive.current) retryToasts.current.add(id)
          else toast.dismiss(id)
        }
        if (err.kind === 'not_found') void refresh()
        return undefined
      }
      dismissRetries()
      await refresh()
      return value
    },
    [refresh, toast, dismissRetries],
  ) as Write

  return write
}
