// Search snippets (docs/api.md section 8, SearchHit): the engine returns
// each snippet as segments with the matches flagged, so a literal ** in
// stored text is never mistaken for a highlight. hitSegments renders those;
// parseSnippet (splitting the ** text form) is only the fallback for a hit
// without segments. Segments render as React text and <mark> children, so
// stored text (including anything that looks like markup) is never parsed
// as HTML.

export interface SnippetSegment {
  text: string
  mark: boolean
}

/**
 * Splits on ** into alternating plain and marked segments. A marker with no
 * partner leaves the rest of the snippet plain. Empty segments are dropped
 * and neighbouring plain segments are joined.
 */
export function parseSnippet(snippet: string): SnippetSegment[] {
  const parts = snippet.split('**')
  // An even number of parts means one marker has no partner: the last part
  // follows it and stays plain.
  const unmatched = parts.length % 2 === 0
  const out: SnippetSegment[] = []
  parts.forEach((text, i) => {
    const mark = i % 2 === 1 && !(unmatched && i === parts.length - 1)
    if (!text) return
    const last = out[out.length - 1]
    if (last && !last.mark && !mark) last.text += text
    else out.push({ text, mark })
  })
  return out
}

/** The hit's engine segments when present, else its ** snippet parsed. */
export function hitSegments(hit: { snippet: string; segments?: { text: string; match: boolean }[] }): SnippetSegment[] {
  if (hit.segments) {
    return hit.segments.filter((s) => s.text !== '').map((s) => ({ text: s.text, mark: s.match }))
  }
  return parseSnippet(hit.snippet)
}
