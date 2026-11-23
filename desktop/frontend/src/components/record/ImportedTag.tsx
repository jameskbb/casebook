// The quiet "From a file" marker: a finding, source or history row that
// came in with a question someone sent. Casebook shows what the file says
// and did not see it happen, so the marker sits next to the check label and
// the name wherever they are shown.
import { FileInput } from 'lucide-react'
import './ImportedTag.css'

export const IMPORTED_TITLE =
  'This came from a file someone sent. Casebook shows what the file says.'

export function ImportedTag() {
  return (
    <span className="imported-tag" title={IMPORTED_TITLE}>
      <FileInput className="imported-tag-icon" size={14} strokeWidth={1.75} aria-hidden="true" />
      From a file
    </span>
  )
}
