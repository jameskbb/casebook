# 0014: Only a person can mark a record human_verified

- Status: Accepted
- Date: 2026-10-06

Builds on ADR-0006 (verification states) and ADR-0013 (status and verification pairing); neither file is edited.

## Context

The brief's principle 6 says human verification must be possible: AI-created knowledge should not automatically become unquestioned truth. `human_verified` is the state that says a person vouched for a record. Until now the engine accepted it from any actor, and the MCP schemas offered it: an AI client could add evidence, a claim or a hypothesis as `human_verified`, patch a record to it, call `Set*Verification` with it, or slip it into a `casebook_checkpoint` batch. The label then meant nothing.

## Decision

`internal/app` refuses verification `human_verified` from every actor whose kind is not `human`, on every write path: `AddEvidence`, `UpdateEvidence`, `SetEvidenceVerification`, `AddClaim`, `UpdateClaim`, `SetClaimVerification`, `AddHypothesis`, `UpdateHypothesis`, `SetHypothesisVerification`, and each item of `CheckpointBatch`. The refusal is a `ValidationError` on `verification` (in a batch, the item path such as `claims[1].verification`, and the whole batch writes nothing) whose message tells the client what to do instead: "only a person can mark a record human_verified; use source_verified when you checked it against the source, or leave it for a person to verify". One check (`checkVerifier`) runs where each path validates its verification, so surfaces do not repeat the rule.

- The check looks at the request, not the outcome: naming `human_verified` is refused even when the record already is `human_verified`. A silent no-op would hide that the client tried.
- `import` and `system` actors are refused too. Import already lowers a file's `human_verified` to `ai_generated` before writing (`untrustedVerification`, AR-09), and no system actor writes records. The one repair that can produce `human_verified` (migration 0002, ADR-0013) runs in SQL, outside the actor model, and only gives a record a person authored that person's default, so it is not an AI vouching for anything. No legitimate non-human path needs the value.
- `source_verified` stays open to AI clients: it means the record was checked against its source, which an AI can do and the record's source makes checkable.
- The MCP input schemas no longer list `human_verified` (evidence keeps `superseded`; claims and hypotheses offer `ai_generated`, `source_verified`, `disputed`), and the field descriptions say only a person can set it. The engine rule still holds for any client that bypasses the schema.
- Defaults are unchanged: a `human` actor's new records default to `human_verified`, everyone else's to `ai_generated`.

## Moving away from human_verified

An AI client may move a record away from `human_verified`: it may dispute it (claims: status or verification `disputed`, which ADR-0013 pairs; hypotheses and evidence: verification `disputed`), supersede it, or set `source_verified` or `ai_generated`.

Disputing has to stay open. A person's verification is a snapshot; when an AI later finds evidence that contradicts a verified claim, refusing the dispute would force it to either drop the evidence or record it next to a claim that still reads as settled, which is the opposite of what principle 6 protects. Lowering trust never makes AI output look more certain than it is, every change is in the activity log with actor kind `ai` and the client name, and a person can restore `human_verified` in one step. The asymmetry is deliberate: an AI can raise doubt but cannot settle it. When an AI reactivates a disputed claim with a status change, verification resets to the AI default (`ai_generated`, ADR-0013), never back to `human_verified`.

## Edits after a check

A verification covers the content that was checked. When an `ai`, `import` or `system` actor changes that content on a record that is `human_verified` or `source_verified`, the same write sets verification `ai_generated`, and the activity summary ends "; verification reset to ai_generated after AI edit" (or "after import edit", "after system edit"). Otherwise a person's verification would end up covering text they never saw. This applies to `UpdateClaim`, `UpdateHypothesis`, `UpdateEvidence` and checkpoint claim and hypothesis updates.

- Substance is what a check vouches for: the statement of a claim or hypothesis, and every evidence field except verification (title, summary, raw detail, source type, source ref, source label, capture time, metadata). For evidence all of these describe the observation or where it came from, so none is cosmetic.
- Confidence, rationale, status and links do not reset it. Verification says the statement is checked, not how sure anyone is: confidence and its rationale are a separate axis that the client is expected to revise as evidence arrives, and status changes and links are how new evidence is recorded against a verified record (the same reason disputing stays open). A rationale is the client's reasoning about confidence, not the claim itself, so rewording it does not change what was verified.
- A verification named in the same patch wins: an AI that reworded a claim and rechecked it against the source passes `source_verified`; it cannot pass `human_verified` (see above).
- A person's edit keeps the verification; a person editing a record they can verify is vouching for the new text.
- Supersede and dispute writes are unaffected: they already set `superseded` or `disputed`.

## Consequences

- The CLI and hooks act as `human` (`casebook-cli`), so they can still write `human_verified`; today no CLI command sets verification. An AI that drives the CLI through a shell acts as the person who granted it that shell; that boundary belongs to the person's tool permissions, not to Casebook.
- Clients that sent `human_verified` now get a tool error naming `verification` and can retry with `source_verified` or no verification.
