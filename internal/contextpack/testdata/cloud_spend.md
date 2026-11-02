Investigation: INV-0001
Title: September cloud bill up 38%
Status: Investigating

Primary question:
Why did the cloud bill rise 38% in September when traffic was flat?

Current conclusion:
Debug logging requested for payments only reached every service and drove most of the increase.

Conclusion confidence:
High

Conclusion verification: Human verified
Supporting claims: none selected.
Contradicting claims: none selected.
Conclusion review: not reviewed; needs review.

Conclusion rationale:
The change, the pricing page and the billing export agree; the remaining 29% is still open.

Current claims:
C-001 [High, Human verified]
A shared platform default moved every service to debug logging on August 28; only payments asked for it.
Why: The chart diff and the head of platform agree.
Evidence: E-001 [AI generated], E-004 [Human verified]
C-002 [High, AI generated]
Log ingestion, billed per GB, is $96,400 of the $135,800 increase; compute and network are flat.
Why: The pricing model and the billing export agree.
Evidence: E-002 [AI generated], E-003 [AI generated]

Hypotheses:
H-001 [Medium, Open, AI generated]
The new tracing agent may also be shipping some services' logs twice.

Open questions:
Q-001
Can every service except payments go back to info level today?

Known disproven hypotheses:
H-002 Data transfer from the region move doubled the network line. [AI generated]
Contradicted by: E-003 [AI generated]

Evidence index:
E-001 Service chart default log level set to debug (Platform Helm chart) [AI generated]
E-002 Logs are billed per GB ingested in the month (Provider pricing page) [AI generated]
E-003 Log ingestion is 71% of the September increase (Billing export query) [AI generated]
E-004 Head of platform: debug was requested for payments only (Head of platform, in person) [Human verified]

Recent material changes:
- 2026-01-15T09:00:00.000Z human casebook-ui: Updated conclusion of INV-0001 (confidence high)
- 2026-01-15T09:00:00.000Z human casebook-ui: Added question Q-001: Can every service except payments go back to info level today?
- 2026-01-15T09:00:00.000Z ai claude-code: Updated hypothesis H-002: status disproven
- 2026-01-15T09:00:00.000Z ai claude-code: Added hypothesis H-002 [low] with 1 evidence link
- 2026-01-15T09:00:00.000Z ai claude-code: Added hypothesis H-001 [medium]
- 2026-01-15T09:00:00.000Z ai claude-code: C-003 superseded by C-002
- 2026-01-15T09:00:00.000Z ai claude-code: Added claim C-003 [low] with 1 evidence link
- 2026-01-15T09:00:00.000Z ai claude-code: Added claim C-002 [high] with 2 evidence links
- 2026-01-15T09:00:00.000Z human casebook-ui: Added claim C-001 [high] with 2 evidence links
- 2026-01-15T09:00:00.000Z human casebook-ui: Added evidence E-004 "Head of platform: debug was requested for payments only"
