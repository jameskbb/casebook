import type { SourceType } from '../api/types'

interface CaptureHints {
  reference: string
  detail: string
  exampleReference: string
  exampleDetail: string
}

// Hints and one made-up example per kind of source, shown in the Add source
// dialog. The examples all come from the call center question.
export const captureHints: Partial<Record<SourceType, CaptureHints>> = {
  sql_query: {
    reference: 'The name of the query and the data it reads',
    detail: 'The query and the rows that matter, with what was counted and left out',
    exampleReference: 'Calls by reason, from the call log',
    exampleDetail: 'Counts every call to the sales line in September and October. Calls about a bill: 400 in September, 1,070 in October.',
  },
  file: {
    reference: 'The file and its date. For a spreadsheet, the sheet and the cells',
    detail: 'The rows or lines that matter, with their headings',
    exampleReference: 'Call log October.xlsx, sheet By reason, cells B4 to C6, saved November 2',
    exampleDetail: 'Wanted to buy: 2,000 in September, 2,050 in October. Asked about a bill: 400 in September, 1,070 in October.',
  },
  documentation: {
    reference: 'The document, its date, and the page or section',
    detail: 'The lines that matter, with their page or section',
    exampleReference: 'October sales report, page 2, Sales by month',
    exampleDetail: 'Page 2: sales were $510k in September and $512k in October.',
  },
  external_url: {
    reference: 'The web address, the section, and the date on the page if it has one',
    detail: 'The lines that matter and the day you read them',
    exampleReference: 'https://example.com/billing/notices#october-3',
    exampleDetail: 'Read on November 2: the billing team says 1,900 customers were charged twice on October 3.',
  },
  api_response: {
    reference: 'Which system you asked, what you asked for, and when. Leave out passwords',
    detail: 'The part of the reply that matters, with what was counted',
    exampleReference: 'Phone system, calls for October by reason, asked on November 2',
    exampleDetail: 'The phone system returned 3,120 calls for October: 2,050 to buy and 1,070 about a bill.',
  },
  code: {
    reference: 'The file, its version, and the lines or the name of the part',
    detail: 'The lines that matter and what they do',
    exampleReference: 'billing/send_bills, version of October 2, the part that loads the file',
    exampleDetail: 'The step that loads the file of bills can run twice when it is started again. That would send each bill two times.',
  },
  jira: {
    reference: 'The ticket number or its address, and the comment or attachment',
    detail: 'The lines that matter, who wrote them and when',
    exampleReference: 'BILL-123, comment dated October 4',
    exampleDetail: 'The billing team wrote that bills sent on October 3 went out twice. The comment does not say how many customers were paid back.',
  },
  user_statement: {
    reference: 'Who said it, when and where',
    detail: 'What they said, and how sure they were. Leave out whole conversations',
    exampleReference: 'Sales manager, Monday sales meeting, November 2',
    exampleDetail: 'The sales manager said the team was two people short in October. This is something a person said. Check it against the staff schedule.',
  },
  ai_tool_output: {
    reference: 'Which AI, what it read, and the date',
    detail: 'What it reported and the source underneath. Leave out the chat',
    exampleReference: 'Claude Code read Call log October.xlsx on November 2',
    exampleDetail: 'It counted 1,070 calls about a bill in October from the sheet By reason. Check the count against the sheet.',
  },
  manual_observation: {
    reference: 'Where and when you saw it',
    detail: 'What you saw, and what you could not see',
    exampleReference: 'Call center wall board, November 2',
    exampleDetail: 'The wall board showed 3,120 calls for October. It did not show what the calls were about.',
  },
  casebook_artifact: {
    reference: 'The other question, the source or finding in it, and the date',
    detail: 'The lines that matter, with their label and where they came from',
    exampleReference: 'The help desk question, its source from the ticket report, saved November 2',
    exampleDetail: 'That source says tickets about a bill took nine days to close. It is labelled Not checked, so check it again here.',
  },
}
