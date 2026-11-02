# 0013: One coherent status and verification state for claims and hypotheses

- Status: Accepted
- Date: 2026-10-05

Amends ADR-0006; that file is not edited. Prompted by the adversarial review finding AR-02.

## Context

ADR-0006 gave evidence, claims and hypotheses a verification state (`ai_generated`, `human_verified`, `source_verified`, `disputed`, `superseded`). Claims and hypotheses also carry a status (claims: `active`, `disputed`, `superseded`, `rejected`; hypotheses: `open`, `supported`, `disproven`, `superseded`). Nothing tied the two axes together. A claim could be `{active, superseded}` or `{active, disputed}` and the context pack then printed it under "Established claims" with a Superseded or Disputed label; the desktop could show "Human verified" next to the status "Disputed". Every surface keyed off a different axis.

Two fixes were on the table: remove `disputed` and `superseded` from the verification enum, or keep the enum and enforce one rule in the engine.

## Decision

Keep the enum and enforce one rule in `internal/app`, backed by database triggers.

- Verification is `superseded` exactly when the status is `superseded`. Only superseding (`Supersede*` or a patch with `superseded_by`) writes it. Any other request for verification `superseded` is a `ValidationError` on `verification`.
- For claims, verification is `disputed` exactly when the status is `disputed`. Either half alone sets both: `SetClaimVerification(disputed)` or a patch with only verification `disputed` disputes the claim; a patch with only status `disputed` sets verification `disputed`. A verification of `ai_generated`, `human_verified` or `source_verified` on a disputed claim makes it `active` again (verifying a claim settles the dispute). An explicit contradicting pair is a `ValidationError` on `verification`.
- Leaving `disputed` or `superseded` through a status change without a verification resets verification to the acting actor's default (the existing rule for leaving superseded).
- A verification change alone on a superseded record is `ErrConflict`: it must be reinstated with a status first, which also clears `superseded_by`.
- Hypotheses have no disputed status, so verification `disputed` stays a free verification state on a hypothesis that is not superseded.
- Evidence has no status and keeps the whole enum unchanged.

Migration 0002 repairs existing rows (the status wins for superseded and disputed; a claim with status `active` and verification `disputed` becomes disputed; any other leftover `disputed` or `superseded` verification becomes the author's default verification: `human_verified` for human authors, `ai_generated` otherwise) and adds `BEFORE INSERT` and `BEFORE UPDATE` triggers that refuse a disagreeing pair. Repairs keep `updated_at` and write no activity row: they restate what the record already said on the other axis. Import applies the same repair to files written by an older Casebook, and backup validation checks the rule for files at schema version 2 or later (older files are repaired by the migration during restore).

The context pack also places a claim by both axes, so a snapshot that somehow disagrees still never lists a disputed, superseded or rejected claim under "Established claims".

## Why not remove the values

Removing `disputed` and `superseded` from the enum would have been cleaner for claims but breaks evidence (which has no status and uses both values today), every existing export file, the desktop and MCP enums, and the frontend labels in one step. Enforcing the pairing keeps the wire format and the enum unchanged, so surfaces only adjust which control writes which field.

## Consequences

- Surfaces can read either axis and get the same answer. The desktop's Dispute and Verify controls each produce one consistent shape; the MCP schemas should describe `superseded` as "set by superseding" rather than offering it as a free choice.
- `SetClaimVerification` may change the status; its activity summary says so (`Set verification of C-001 to disputed; status disputed`).
- A request that used to be accepted with a contradicting pair now fails with a message naming the field, which an AI client can correct in one retry.
