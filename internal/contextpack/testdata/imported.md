Investigation: INV-0001
Title: September cloud bill up 38%
Status: Investigating

Primary question:
Why did the cloud bill rise 38% in September when traffic was flat?

Current conclusion:
Debug logging requested for payments only reached every service and drove most of the increase.

Conclusion confidence:
High

Conclusion verification: AI generated
Supporting claims: none selected.
Contradicting claims: none selected.
Conclusion review: not reviewed; needs review.

Conclusion rationale:
The change, the pricing page and the billing export agree; the remaining 29% is still open.

Current claims:
C-001 (imported) [High, AI generated]
A shared platform default moved every service to debug logging on August 28; only payments asked for it.
Why: The chart diff and the head of platform agree.
Evidence: E-001 [AI generated], E-004 [AI generated]
C-002 (imported) [High, AI generated]
Log ingestion, billed per GB, is $96,400 of the $135,800 increase; compute and network are flat.
Why: The pricing model and the billing export agree.
Evidence: E-002 [AI generated], E-003 [AI generated]
C-004 [Medium, Human verified]
The payments values file was never changed.
Evidence: E-005 [Human verified]

Hypotheses:
H-001 (imported) [Medium, Open, AI generated]
The new tracing agent may also be shipping some services' logs twice.

Open questions:
Q-001 (imported)
Can every service except payments go back to info level today?
Q-002
Who owns the chart default now?

Known disproven hypotheses:
H-002 (imported) Data transfer from the region move doubled the network line. [AI generated]
Contradicted by: E-003 [AI generated]

Evidence index:
E-001 (imported) Service chart default log level set to debug (Platform Helm chart) [AI generated]
E-002 (imported) Logs are billed per GB ingested in the month (Provider pricing page) [AI generated]
E-003 (imported) Log ingestion is 71% of the September increase (Billing export query) [AI generated]
E-004 (imported) Head of platform: debug was requested for payments only (Head of platform, in person) [AI generated]
E-005 Local check: payments values file still lacks the debug override (Local review) [Human verified]

Recent material changes:
- 2026-01-15T09:00:00.000Z human casebook-ui: Added question Q-002: Who owns the chart default now?
- 2026-01-15T09:00:00.000Z human casebook-ui: Added claim C-004 [medium] with 1 evidence link
- 2026-01-15T09:00:00.000Z human casebook-ui: Added evidence E-005 "Local check: payments values file still lacks the debug override"
- 2026-01-15T09:00:00.000Z import casebook-ui: Imported INV-0001 (4 evidence, 3 claims, 2 hypotheses, 1 questions); 3 verifications reset to ai_generated
- 2026-01-15T09:00:00.000Z human casebook-ui (imported): Updated conclusion of INV-0001 (confidence high)
- 2026-01-15T09:00:00.000Z human casebook-ui (imported): Added question Q-001: Can every service except payments go back to info level today?
- 2026-01-15T09:00:00.000Z ai claude-code (imported): Updated hypothesis H-002: status disproven
- 2026-01-15T09:00:00.000Z ai claude-code (imported): Added hypothesis H-002 [low] with 1 evidence link
- 2026-01-15T09:00:00.000Z ai claude-code (imported): Added hypothesis H-001 [medium]
- 2026-01-15T09:00:00.000Z ai claude-code (imported): C-003 superseded by C-002
