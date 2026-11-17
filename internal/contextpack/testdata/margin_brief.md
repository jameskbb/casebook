# INV-0002: Northeast margin down 6 points in Q3

Leadership brief. Status: Investigating.

## Question
Why did gross margin in the Northeast fall from 41% to 35% in Q3 when revenue was up 4%?

## Answer

White-glove onboarding, given away free on every deal above $250k since July, drove most of the margin drop.

Conclusion verification: Human verified

Supporting claims: C-002 (AI generated); C-003 (Human verified)

Contradicting claims: C-001 (Disputed)

Conclusion reviewed: 2026-10-09 12:55 UTC

Review basis: Checked the implementation-services ledger, July playbook, deal records and the sales-ops explanation; discounting is disputed and travel remains open.

Confidence: High. The playbook text (E-008), the deal desk records (E-009) and the sales-ops lead (E-010) agree; travel on the same accounts (H-004) and $400k of smaller lines are still open.

## Selected findings and what is still in doubt

- **C-001** Deep discounts on the quarter's two biggest deals caused most of the margin drop. (Disputed; Low confidence)

  - Why: The sales VP's reading of the quarter; not yet checked against the deal desk.

  - Supported by E-003: Sales VP: we cut prices hard to win the quarter's two biggest deals (AI generated)

  - Contradicted by E-006: Discounts are flat: 11.6% in Q3 against 11.8% in Q2; the two biggest deals got 12% and 14% (AI generated)

- **C-002** Implementation services are $1.9M of the $2.3M cost rise; three accounts moved to white-glove onboarding in July carry $1.4M of it. (Current; High confidence; AI generated)

  - Why: The cost ledger by line and by account agree.

  - Supported by E-001: Northeast Q3: revenue $27.0M (+4%), cost of delivery $17.6M (+15%), gross margin 35% against 41% (AI generated)

  - Supported by E-005: Implementation services are $1.9M of the $2.3M cost rise; contractor rate flat, hours up 78% (AI generated)

  - Supported by E-007: Three accounts moved to white-glove onboarding in July carry $1.4M of the $1.9M (AI generated)

- **C-003** The July 6 playbook update made white-glove onboarding free on every deal above $250k; seven Northeast deals got it and none was charged. (Current; High confidence; Human verified)

  - Why: The playbook text and the deal desk records agree; the three accounts in E-007 are among the seven.

  - Supported by E-007: Three accounts moved to white-glove onboarding in July carry $1.4M of the $1.9M (AI generated)

  - Supported by E-008: July 6 playbook update: white-glove onboarding included free on every deal above $250k (Source verified)

  - Supported by E-009: Deal desk: seven Northeast deals above $250k since July 6, onboarding fee $0 on every one (AI generated)

  - Supported by E-010: Regional sales-ops lead: the free white-glove clause was written for the two Pinecrest deals only (Human verified)

## Explanations still open or supported

- **H-003** A July change in sales policy, not account-by-account decisions, put the new accounts on white-glove onboarding. (Supported; High confidence; AI generated)

  - Supported by E-007: Three accounts moved to white-glove onboarding in July carry $1.4M of the $1.9M (AI generated)

- **H-004** Travel for on-site white-glove sessions is growing with the same accounts. (Open; Medium confidence; AI generated)

  - Supported by E-005: Implementation services are $1.9M of the $2.3M cost rise; contractor rate flat, hours up 78% (AI generated)

## Ruled out

- **H-001** July contractor hires raised the cost of each onboarding in the Northeast. (Disproven; AI generated)

  - Supported by E-004: Onboarding team added four contractors in July (AI generated)

  - Contradicted by E-005: Implementation services are $1.9M of the $2.3M cost rise; contractor rate flat, hours up 78% (AI generated)

- **H-002** The weaker Canadian dollar inflated delivery cost on the Canadian accounts. (Disproven; AI generated)

  - Contradicted by E-005: Implementation services are $1.9M of the $2.3M cost rise; contractor rate flat, hours up 78% (AI generated)

## Selected questions

- **Q-003** Can onboarding be charged on the Q4 deals already signed under the July playbook? (Open)

- **Q-004** Should the Q3 regional forecast be restated for the onboarding that was delivered and never charged? (Deferred)

  - Deferred: Waiting for the quarter close on October 10.

## Evidence and source caveats

- **E-001** Northeast Q3: revenue $27.0M (+4%), cost of delivery $17.6M (+15%), gross margin 35% against 41% (Q3 regional P&L: finance/pnl/northeast-2026-Q3.xlsx; AI generated)

  - The Q3 regional P&L shows Northeast revenue of $27.0M against $26.0M in Q2 and cost of delivery of $17.6M against $15.3M, taking gross margin from 41.2% to 34.8%; the P&L carries cost of delivery as one line.

- **E-003** Sales VP: we cut prices hard to win the quarter's two biggest deals (Sales VP, in the pipeline call; AI generated)

  - The sales VP said in the Monday pipeline call that the region cut prices hard to win the quarter's two biggest deals in July and expects that to explain the margin drop.

- **E-004** Onboarding team added four contractors in July (Contractor roster: people/contractors/northeast-onboarding-2026.csv; AI generated)

  - The contractor roster shows four implementation contractors starting in the Northeast onboarding team on July 13 and July 20, at $135 to $145 an hour.

- **E-005** Implementation services are $1.9M of the $2.3M cost rise; contractor rate flat, hours up 78% (Cost ledger query: finance/queries/cost_of_delivery_by_line_qoq.sql; AI generated)

  - In the cost ledger, implementation services rose from $2.4M to $4.3M, which is $1.9M of the $2.3M quarter-over-quarter rise; travel and expenses +$210k, customer support +$110k, hosting +$90k, FX revaluation +$40k, everything else -$50k. The blended contractor rate is $141 an hour against $140; hours are 30,400 against 17,100.

- **E-006** Discounts are flat: 11.6% in Q3 against 11.8% in Q2; the two biggest deals got 12% and 14% (Deal desk discount query: salesops/queries/discount_rate_by_quarter.sql; AI generated)

  - Across the 47 Northeast deals booked in Q3 the average discount is 11.6% against 11.8% over Q2's 44 deals; the two biggest deals got 12% and 14%, inside the 8% to 16% range of the last four quarters.

- **E-007** Three accounts moved to white-glove onboarding in July carry $1.4M of the $1.9M (Implementation hours query: finance/queries/implementation_cost_by_account.sql; AI generated)

  - Implementation cost by account: Pinecrest Logistics, Pellston Health and Ridgeway Mutual moved to the white-glove onboarding tier in July and carry $1.4M of the $1.9M rise; three later deals above $250k add $300k together and the standard tier $200k.

- **E-008** July 6 playbook update: white-glove onboarding included free on every deal above $250k (Sales playbook v7: Sales shared drive: Northeast Sales Playbook v7 (2026-07-06), section 4; Source verified)

  - The Northeast sales playbook revised on July 6 (v7) adds "White-glove onboarding is included at no charge on every deal with ACV above $250k" to the standard offer; v6 (April 2) listed white-glove onboarding as a priced add-on at $120k to $180k.

- **E-009** Deal desk: seven Northeast deals above $250k since July 6, onboarding fee $0 on every one (Deal desk approvals export: salesops/exports/northeast-approvals-2026-Q3.csv; AI generated)

  - The deal desk approvals export lists seven Northeast deals above $250k closed from July 6 to September 30, every one on the white-glove tier with an onboarding fee of $0; the four Q2 deals of that size carried fees of $120k to $180k.

- **E-010** Regional sales-ops lead: the free white-glove clause was written for the two Pinecrest deals only (Sales-ops lead, in person; Human verified)

  - The regional sales-ops lead said in person that the July 6 playbook clause was written to close the two Pinecrest deals and was never meant to apply to every deal above $250k; nobody reviewed it after Pinecrest signed.

## Scope and omissions

Labels: Human verified means a person checked this and vouched for it. Source verified means it was marked as checked against its source, usually by an AI. AI generated means it has not been marked as checked by a person in Casebook. Disputed means evidence contradicts it; it stays on record until it is settled. Superseded means it was replaced by a newer statement.

Not included: Q-001, Q-002 and E-002. Activity history is not included. Ask the person who shared this for the complete report.

Disputed findings, contradicting evidence and the claims behind the answer are always included.

_Prepared by Casebook from the saved records of INV-0002. Casebook did not rewrite them or add to them._
