Question: INV-0001
Title: September cloud bill up 38%
Status: In progress

The question:
Why did the cloud bill rise 38% in September when traffic was flat?

Answer so far:
Debug logging requested for payments only reached every service and drove most of the increase.

Answer marked: Not checked
Findings behind the answer: none selected.
Findings against the answer: none selected.
Answer review: not reviewed yet.

Why this answer:
The change, the pricing page and the billing export agree; the remaining 29% is still open.

Findings:
C-001 (imported) [Not checked]
A shared platform default moved every service to debug logging on August 28; only payments asked for it.
Why: The chart diff and the head of platform agree.
Sources: E-001 [Not checked], E-004 [Not checked]
C-002 (imported) [Not checked]
Log ingestion, billed per GB, is $96,400 of the $135,800 increase; compute and network are flat.
Why: The pricing model and the billing export agree.
Sources: E-002 [Not checked], E-003 [Not checked]
C-004 [A person checked this]
The payments values file was never changed.
Sources: E-005 [A person checked this]

Findings still being tested:
H-001 (imported) [Not tested yet, Not checked]
The new tracing agent may also be shipping some services' logs twice.

Follow-up questions:
Q-001 (imported)
Can every service except payments go back to info level today?
Q-002
Who owns the chart default now?

Findings ruled out:
H-002 (imported) Data transfer from the region move doubled the network line. [Not checked]
Ruled out by: E-003 [Not checked]

Sources:
E-001 (imported) Service chart default log level set to debug (Platform Helm chart) [Not checked]
E-002 (imported) Logs are billed per GB ingested in the month (Provider pricing page) [Not checked]
E-003 (imported) Log ingestion is 71% of the September increase (Billing export query) [Not checked]
E-004 (imported) Head of platform: debug was requested for payments only (Head of platform, in person) [Not checked]
E-005 Local check: payments values file still lacks the debug override (Local review) [A person checked this]

Recent changes:
- 2026-01-15T09:00:00.000Z human casebook-ui: Asked Q-002: Who owns the chart default now?
- 2026-01-15T09:00:00.000Z human casebook-ui: Added finding C-004 (1 source)
- 2026-01-15T09:00:00.000Z human casebook-ui: Added source E-005 "Local check: payments values file still lacks the debug override"
- 2026-01-15T09:00:00.000Z import casebook-ui: Imported this question
- 2026-01-15T09:00:00.000Z human casebook-ui (imported): Updated the answer
- 2026-01-15T09:00:00.000Z human casebook-ui (imported): Asked Q-001: Can every service except payments go back to info level today?
- 2026-01-15T09:00:00.000Z ai claude-code (imported): Ruled out H-002
- 2026-01-15T09:00:00.000Z ai claude-code (imported): Added finding H-002 (1 source)
- 2026-01-15T09:00:00.000Z ai claude-code (imported): Added finding H-001
- 2026-01-15T09:00:00.000Z ai claude-code (imported): Replaced finding C-003 with C-002
