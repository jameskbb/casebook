# 0005: Human-friendly display refs

- Status: Accepted
- Date: 2026-10-01

## Context

Humans and AI clients cite records in conversation and in context packs ("C-001 is supported by E-004"). Opaque UUIDs are hard to read and easy for models to garble.

## Decision

- Investigations get a global sequential ref: `INV-0001`.
- Records within an investigation get per-investigation sequential refs: `E-001` (evidence), `C-001` (claims), `H-001` (hypotheses), `Q-001` (questions).
- Internal integer primary keys are used for joins and are never shown to users.
- Refs are allocated inside the write transaction and are never reused.

## Consequences

- Refs are stable and short enough for prompts and UI.
- Import assigns a new `INV` ref on collision while keeping record refs within the imported investigation.
