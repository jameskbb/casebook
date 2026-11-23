// Copy with headings and lists: the same content goes on the clipboard as
// rich text and as plain text, so an email or a document shows it laid out
// and a plain box shows it without marks.

/** Resolves false when this window cannot put rich text on the clipboard; the caller then copies plain text. */
export async function copyRich(html: string, text: string): Promise<boolean> {
  try {
    if (typeof ClipboardItem === 'undefined' || !navigator.clipboard?.write) return false
    await navigator.clipboard.write([
      new ClipboardItem({
        'text/html': new Blob([html], { type: 'text/html' }),
        'text/plain': new Blob([text], { type: 'text/plain' }),
      }),
    ])
    return true
  } catch {
    return false
  }
}
