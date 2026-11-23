import type { SourceType } from '../api/types'

interface CaptureHints {
  /** Under Where to find it. */
  reference: string
  /** Under Exact words or numbers. */
  detail: string
}

// One hint per field for each kind of source a person can choose in the Add
// source dialog. Any other kind shows no hint.
export const captureHints: Partial<Record<SourceType, CaptureHints>> = {
  file: {
    reference: 'The file and its date. For a spreadsheet, the sheet and the cells',
    detail: 'The rows or lines that matter, with their headings',
  },
  documentation: {
    reference: 'The document or email, its date, and the page or section',
    detail: 'The lines that matter, with their page or section',
  },
  external_url: {
    reference: 'The web address, the section, and the date on the page if it has one',
    detail: 'The lines that matter and the day you read them',
  },
  user_statement: {
    reference: 'Who said it, when and where',
    detail: 'What they said, and how sure they were',
  },
  manual_observation: {
    reference: 'Where and when you saw it',
    detail: 'What you saw, and what you could not see',
  },
}
