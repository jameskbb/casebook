# Capture evidence someone can check

Capture the finding and enough of its source for another person or AI client to
check it later. Use the fields that help with the particular source. A manual
observation or a person's statement is legitimate evidence without a spreadsheet,
query or numerical comparison. Say what was observed or said and acknowledge its
limits.

Casebook stores the text you provide. It does not fetch a source, run a query,
calculate a metric or establish that a source is correct. In the desktop **Add
evidence** and **Edit evidence** dialogs, **Capture details guide** shows optional
guidance and an illustrative example for the selected source type. Examples stay
in the guide and are never inserted into your evidence.

## Put each detail where someone will find it

| Field | What to capture |
| --- | --- |
| Title | A short description of the evidence. |
| Summary | What the evidence shows and any material limitation, in one or two sentences. |
| Source type | The kind of source you actually inspected. A spreadsheet uses `file`; an original observation can use `manual_observation`. |
| Source | A readable name for the source, such as a finance workbook or reporting dataset. |
| Reference | The exact locator, including a version or date where available. For a workbook, include its sheet and range; for a document, its page or section; for SQL, the query path or name, version and database or dataset. |
| Captured | When you captured the evidence. This is separate from the reporting period. |
| Raw detail | A concise source excerpt, relevant rows with headings, or the exact query and a concise result excerpt. Include context needed to interpret it. |
| Metadata | An optional JSON object for details you want to keep under explicit keys. Existing arbitrary keys remain supported. |

For URLs, keep the exact page and section plus the source date or access date.
For code, keep the repository path, commit and relevant lines or symbol. For an
API response, keep the endpoint, non-secret parameters and response time. For a
statement, identify the speaker, date and context without copying a transcript.
For AI tool output, identify the underlying source as well as the tool: the
output alone does not establish that its result was checked.

## Make business comparisons reproducible

For a numerical claim, keep these details when the source provides them:

- **Period and comparison:** the date range, timezone or fiscal calendar if
  relevant, and the baseline period. A captured timestamp does not replace these.
- **Scope and filters:** region, segment, product, population, inclusion rules
  and exclusions. Say when the comparison uses different populations.
- **Metric definition and denominator:** what was measured, how it was defined
  and what it was divided by. For a rate, distinguish a ratio of totals from an
  average of individual rates when relevant.
- **Units:** currency, scale and whether the change is a percent or percentage
  points. A move from 28% to 22% is a decline of 6 percentage points.
- **Exact locator and excerpt:** enough source material to check the particular
  result, including headings and any caveat. Do not keep an entire chat or report
  when a concise excerpt will do.

Do not invent missing context. If a dashboard hides its filters or a workbook has
no metric definition, record that limitation and leave an open question. Checking
provenance is separate from confidence in a conclusion and from the evidence's
verification label.

## Optional metadata convention

These metadata fields are optional. Use them to keep reporting periods, filters
and metric definitions alongside a finding. Metadata must be a JSON object; adding
these fields does not calculate a result or verify the evidence. Put a checkable
summary in the summary field too: context packs omit raw detail and metadata.

The following is a fictional example. Use your own source details:

```json
{
  "period": "2026-Q3",
  "comparison_period": "2026-Q2",
  "scope": "Northeast region",
  "filters": ["closed orders", "exclude cancelled orders"],
  "metric_definition": "gross profit divided by net revenue",
  "units": "percent; change in percentage points",
  "denominator": "net revenue in USD"
}
```

Keep the precise source locator in `source_ref` and the relevant excerpt in
`raw_detail`. When adding metadata to an existing record, read the current JSON
object first and preserve unrelated keys. An evidence edit replaces the metadata
field if you send it, so send only fields you intend to change. Repair invalid
JSON deliberately before replacing it; do not erase an unreadable object to make
room for these optional keys.

Keep secrets, credentials, transcripts and reasoning traces out of every field.
The person's AI client can inspect external sources through its own tools and
record concise evidence through the existing Casebook APIs. Casebook itself
makes no source fetches or background captures.

For field limits and API input shapes, see the [engine API reference](api.md).
For how verification works, see [the verification rules](decisions/0014-only-people-verify.md).
