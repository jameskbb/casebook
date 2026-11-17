import type { SourceType } from '../api/types'

interface CaptureHints {
  reference: string
  detail: string
  exampleReference: string
  exampleDetail: string
}

// Read-only guidance, never default evidence values or generated provenance.
// A source type kept only for existing records (evernote_note) has no hints.
export const captureHints: Partial<Record<SourceType, CaptureHints>> = {
  sql_query: {
    reference: 'Query path or name, version, and database or dataset',
    detail: 'Exact query and a concise result excerpt, including filters and units',
    exampleReference: 'finance/quarterly_margin.sql, revision 4, finance_reporting',
    exampleDetail: 'Query filters: region = Northeast, Q3 versus Q2 2026. Result excerpt: margin_pct 22.0 versus 28.0. Definition: gross profit / net revenue, percent.',
  },
  file: {
    reference: 'File and version; for a spreadsheet, include sheet and cell range',
    detail: 'Relevant rows or excerpt, with headings, filters and units',
    exampleReference: 'margin-2026-Q3.xlsx, Northeast!B4:F8, saved 2026-10-02',
    exampleDetail: 'Rows: Q2 margin 28%; Q3 margin 22%. Filter: Northeast, closed orders. Margin = gross profit / net revenue. Revenue in USD; margin in percent.',
  },
  documentation: {
    reference: 'Document path or URL, version, and page or section',
    detail: 'A short source excerpt with its page or section',
    exampleReference: 'finance/metric-definitions.pdf, revision 3, page 12, Gross margin',
    exampleDetail: 'Page 12: Gross margin is gross profit divided by net revenue. Excludes returns booked after the reporting close.',
  },
  external_url: {
    reference: 'Exact URL, page or section, and source date when available',
    detail: 'A short source excerpt and when you read it',
    exampleReference: 'https://example.com/reports/q3-2026#regional-margin',
    exampleDetail: 'Regional margin table, read 2026-10-02: Northeast Q3 margin 22%, Q2 28%. The report defines margin as gross profit / net revenue.',
  },
  api_response: {
    reference: 'Endpoint, request parameters and response time; omit credentials',
    detail: 'Relevant response fields, units, filters and metric definition',
    exampleReference: 'GET /reports/margin?region=northeast&period=2026-Q3, 2026-10-02T09:00Z',
    exampleDetail: 'Response excerpt: gross_profit_usd 220000; net_revenue_usd 1000000. Scope: closed Northeast orders. Margin = gross profit / net revenue.',
  },
  code: {
    reference: 'Repository path, commit or version, and lines or symbol',
    detail: 'The relevant code excerpt and the behavior it establishes',
    exampleReference: 'reporting/margin.go, commit 4e12abc, GrossMargin',
    exampleDetail: 'GrossMargin returns gross profit / net revenue. The caller excludes cancelled orders before aggregation.',
  },
  jira: {
    reference: 'Issue key or URL and the relevant comment or attachment',
    detail: 'A concise excerpt with its author, date and limitations',
    exampleReference: 'FIN-123, comment dated 2026-10-02, margin-reconciliation.csv',
    exampleDetail: 'Finance reviewer reported that Q3 Northeast figures exclude open orders. The comment does not establish whether returns are included.',
  },
  user_statement: {
    reference: 'Who said it, when, and the relevant context',
    detail: 'The concise statement and any stated uncertainty; omit transcripts',
    exampleReference: 'Regional finance lead, Q3 review, 2026-10-02',
    exampleDetail: 'The finance lead said returns may explain part of the margin decline. This is a statement to check against the ledger, not a measured result.',
  },
  ai_tool_output: {
    reference: 'Client, tool, source inputs and run date',
    detail: 'Relevant output and its underlying source; omit chat history',
    exampleReference: 'Analysis client, workbook reader, margin-2026-Q3.xlsx, 2026-10-02',
    exampleDetail: 'Tool reported margin 22% from Northeast!B4:F8. Preserve the exact cells and definition before checking the result against the workbook.',
  },
  manual_observation: {
    reference: 'Where and when you observed it, if useful',
    detail: 'What you observed, under which conditions, and any limits',
    exampleReference: 'Quarterly margin dashboard, Northeast view, 2026-10-02',
    exampleDetail: 'The dashboard displayed a lower margin for Q3 than Q2. The filters and denominator were not visible, so the size of the change is unconfirmed.',
  },
  casebook_artifact: {
    reference: 'Case reference, record reference and artifact version or date',
    detail: 'A concise excerpt with its verification label and source references',
    exampleReference: 'INV-0002, E-004, export dated 2026-10-02',
    exampleDetail: 'The source record states margin is gross profit / net revenue and is labelled AI generated. The imported excerpt still needs checking in this case.',
  },
}
