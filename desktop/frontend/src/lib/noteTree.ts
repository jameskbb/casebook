// The small element tree the summary preview draws (see reportView.ts and
// NotePaper). Nothing is injected as HTML: the tree holds only known
// elements, their text and any inline style kept for them.
export type NoteNode =
  | { type: 'text'; text: string }
  | { type: 'el'; tag: string; style: Record<string, string>; children: NoteNode[] }
