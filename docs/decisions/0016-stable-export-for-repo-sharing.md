# 0016: Stable exports for sharing a case through a repository

- Status: Accepted
- Date: 2026-10-06

## Context

Teams want to share an investigation without a server. A repository they already share is the natural place: commit the case file next to the code it is about. For that, an export must diff well and must not churn when nothing changed, and import must have clear semantics when a newer version of a file arrives. Casebook must not grow sync, merge or a server (CLAUDE.md rules).

Before this decision an export already had a fixed key order, indented JSON and records sorted by ref, but `exported_at` was the wall clock (every export differed) and the activity included `active.set` rows, which record machine-local choices.

## Decision

- Export stays `casebook.investigation` version 1; no field is added or removed, so every existing file still imports.
- `exported_at` is the newest timestamp in the document (investigation, records, links, activity) instead of the wall clock. Exporting an unchanged investigation therefore gives identical bytes.
- Activity rows whose action starts with `active.` (`active.set`, `active.bind`; ADR-0015) are not exported: which investigation was active on one machine is not part of the case, and per-directory bindings would leak local paths.
- `casebook export` accepts `--out` as a synonym of `-o/--output`.
- Import keeps its rules (AR-09): everything untrusted and validated, imported rows marked, `human_verified` and `source_verified` lowered to `ai_generated`, active investigation unchanged.
- Re-import is a copy, never a merge. Importing always adds a new investigation. When the database already holds the same case (same `created_at` and creator, which import preserves), the result lists those refs in `same_case_refs` and the CLI says so and suggests archiving and deleting the copy that is not needed. We considered refusing duplicates and a `--replace` flag; refusing blocks the normal "a newer file arrived" flow, and replacing could silently drop work recorded on the importing machine. Naming the existing copy keeps every step explicit.
- The documented workflow is one owner per case file; teammates import read copies ([Share a case through your repo](../sharing.md)).

## Consequences

- `git diff` of a re-exported case shows only real changes: new records, changed fields and new activity rows.
- Two machines editing the same case diverge; Casebook does not reconcile them. The guide says to keep separate cases in that situation.
- "Same case" detection relies on the creation timestamp (millisecond precision) and creator. Two unrelated investigations created in the same millisecond by the same client would be reported as the same case; the import still succeeds, so the cost is a misleading note.
- Committed exports contain raw evidence detail; the guide warns to review files before committing them.
