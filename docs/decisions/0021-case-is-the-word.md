# 0021: "Case" is the word people see

- Status: Accepted
- Date: 2026-10-12

## Context

Every screen, guide and README sentence calls the unit of work an "investigation": New investigation, Search investigations, Archive investigation, `casebook investigation create`. The word was chosen with the brief's first example in mind (a flaky test, an outage) and it boxes the product in: an investigation sounds like a bug hunt or an incident, something an engineer does. The audience Casebook is built for (anyone using AI to understand their own work or business, most of them not engineers) asks why margin dropped, which vendor to keep, whether a finding is true, what an audit showed. Those are not investigations to the people asking them.

The owner's direction, 2026-10-12: move away from "New investigation" toward a word that does not read as a bug hunt. "Project" was the first candidate; it is familiar but reads as a workspace with tasks and people in it, which pulls toward a team tool, and Casebook is for one person on one machine.

## Decision

The people-facing word is **case**. The product is Casebook; a case is one question and everything learned about it. New case, Cases, Open case, Archive case, Search cases, "a case you can show someone".

The rename is people-facing only:

- The desktop app's visible copy, the README, the guides, the title card, the screenshots and the setup guide text say case.
- Refs stay `INV-0001`. Changing them would rewrite every stored record, export file and shared link for a word.
- The engine, the JSON API, the MCP tool names and their argument names stay `investigation`. They are addressed to programs, and renaming them would break every connected client on v0.3.0 for no gain to the person reading the app.
- The CLI keeps `casebook investigation` and gains `case` as an alias beside `inv`; help text says case. The MCP tool descriptions, which an AI reads, say "case (an investigation record)" once so the two words are tied together for it.
- Identifiers in code (`Investigation`, `investigation_ref`, `NewInvestigationDialog`) are not renamed in this decision; a later cleanup may rename frontend component files when it touches them anyway.

"Project" and "question" were considered. Project implies a workspace and tasks; question collides with the open questions a case already holds.

## Consequences

- One word for the thing everywhere a person reads, and it is the word already in the product's name.
- A small, permanent translation at the edge: refs that say INV and a CLI noun that has a friendlier alias. Guides explain the ref prefix once.
- A later decision to rename the engine's nouns stays open, with aliases, if the repo's public life ever makes it worth the break.
