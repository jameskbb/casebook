// Display helper for evidence metadata. The engine stores metadata as compact
// JSON object text; the drawer shows it indented. Text that does not parse
// (it should not happen, the engine validates it) is shown as it is. The
// result is always rendered as text, never as markup.

export function prettyMetadata(text: string): string {
  const trimmed = text.trim()
  if (trimmed === '') return ''
  try {
    return JSON.stringify(JSON.parse(trimmed), null, 2)
  } catch {
    return text
  }
}
