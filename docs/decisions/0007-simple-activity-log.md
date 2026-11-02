# 0007: Activity log is a simple table

- Status: Accepted
- Date: 2026-10-01

## Context

Provenance matters (which client or person changed what), but the brief forbids event-sourcing frameworks and giant audit systems.

## Decision

Every mutation inserts one row into an `activity` table: actor kind (human, ai, import, system), client name (for example "Casebook UI", "claude-code", "codex"), action, target ref, and a one-line summary. Current state lives in normal tables; the activity table is never replayed to rebuild state.

## Consequences

- The UI can show readable provenance and the context pack can list recent material changes.
- There is no event replay, projection, or snapshot machinery.
