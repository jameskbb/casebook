// The quiet "Imported" marker: a record or History row copied from
// an imported file. Its author, label and history are what the file
// said; Casebook did not see them happen, so the marker sits next to the
// verification badge and the actor wherever they are shown.
import { FileInput } from 'lucide-react'
import './ImportedTag.css'

export const IMPORTED_TITLE =
  'This came from a file. Who wrote it, its label and its history are what the file says. Casebook did not see them happen.'

export function ImportedTag() {
  return (
    <span className="imported-tag" title={IMPORTED_TITLE}>
      <FileInput className="imported-tag-icon" size={14} strokeWidth={1.75} aria-hidden="true" />
      From a file
    </span>
  )
}
