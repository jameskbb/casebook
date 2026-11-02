// Byte counting for the engine's byte limits (docs/api.md section 11).

const encoder = new TextEncoder()

/** UTF-8 length of s in bytes, as the engine counts it. */
export function byteLength(s: string): number {
  return encoder.encode(s).length
}

const fmt = new Intl.NumberFormat('en-US')

/**
 * The near-limit counter for a field (UI-SPEC TextField near limit state):
 * "Close to the limit (1,850 of 2,000 bytes)" above 90 percent of max,
 * "Over the limit (2,050 of 2,000 bytes)" past it, otherwise undefined.
 */
export function nearLimit(value: string, max: number): string | undefined {
  const n = byteLength(value)
  if (n <= max * 0.9) return undefined
  const counts = `(${fmt.format(n)} of ${fmt.format(max)} bytes)`
  return n > max ? `Over the limit ${counts}` : `Close to the limit ${counts}`
}

/** Engine byte limits the forms show counters for. */
export const LIMITS = {
  title: 200,
  problemStatement: 4000,
  conclusion: 4000,
  conclusionRationale: 2000,
  evidenceSummary: 4000,
  rawDetail: 65536,
  sourceRef: 2000,
  sourceLabel: 200,
  metadata: 16384,
  statement: 2000,
  rationale: 2000,
  question: 2000,
  resolution: 4000,
  deferNote: 2000,
  searchQuery: 500,
} as const
