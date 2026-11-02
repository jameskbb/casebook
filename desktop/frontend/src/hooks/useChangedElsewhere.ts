// Edit-in-place safety (UI-SPEC Component Contract; D-10 interplay): an
// editor snapshots the stored values when editing starts. While it holds
// unsaved input, a reload that changed those stored values elsewhere must
// not overwrite the input; instead the editor shows "This changed in another
// app." with "Show latest". This hook answers the question: true when dirty
// and the current stored values differ from the snapshot.
import { useMemo } from 'react'

export function sameValues(a: unknown, b: unknown): boolean {
  return JSON.stringify(a) === JSON.stringify(b)
}

/**
 * @param baseline the stored values when editing started (null when not editing)
 * @param current the stored values now (after any reload)
 * @param dirty whether the editor holds unsaved input
 */
export function useChangedElsewhere<T>(
  baseline: T | null,
  current: T,
  dirty: boolean,
  equals: (a: T, b: T) => boolean = sameValues,
): boolean {
  return useMemo(
    () => dirty && baseline !== null && !equals(baseline, current),
    [dirty, baseline, current, equals],
  )
}
