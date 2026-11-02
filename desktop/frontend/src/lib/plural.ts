// Count-aware copy (UI-SPEC zero-one-many): the singular form for exactly 1,
// the plural form otherwise (including 0).

export function plural(n: number, one: string, many: string): string {
  return n === 1 ? one : many
}

/** "1 claim", "3 claims", "0 claims". */
export function countOf(n: number, one: string, many: string): string {
  return `${n} ${plural(n, one, many)}`
}
