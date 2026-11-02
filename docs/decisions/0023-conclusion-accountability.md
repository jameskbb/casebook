# 0023: Label the conclusion and retain the basis of a person's review

- Status: Accepted
- Date: 2026-10-23

Builds on ADR 0014 (verification authority), ADR 0016 (portable exports) and the one application layer in ADR 0001.

## Context

The conclusion is the answer a reader is most likely to act on, but it previously had confidence and prose alone. A person could verify its claims, then a later change could dispute a supporting claim or revise its evidence without the conclusion drawing attention to that change. Confidence does not say who checked an answer, and a statement a person checked must not silently extend that person's endorsement to new material.

## Decision

A conclusion persists its own verification, explicitly selected supporting and contradicting claim refs, the timestamp and concise basis of a person's review, and a private fingerprint of the selected material at that review. Existing conclusions start `ai_generated`, without an attributable review. New conclusion verification defaults by actor kind. `human_verified` is refused from AI, import and system actors on every application write path, including checkpoint batches. MCP schemas offer only `ai_generated`, `source_verified` and `disputed`.

The statement label and the current basis are separate facts. A material change to a selected claim or its linked evidence raises `conclusion_needs_review` while retaining the historical statement label, review timestamp and review basis. It does not automatically declare the conclusion false or revoke what a person previously said. The selected claim's statement, confidence, rationale, status, verification, replacement, evidence links and link relations count as material, as do linked evidence observation and source fields, metadata and verification. Unrelated claims, administrative timestamps, authors and navigation do not.

This is a synchronous comparison, on the caller's read transaction, of the selected claims and their directly linked evidence. It never follows another conclusion and creates no background work, dependency graph, propagation engine or scheduler. The fingerprint is a comparison aid, not a transcript, event log or independent source of truth.

A person records a review explicitly, with a nonempty basis and the review token of the records they saw. The token covers the current conclusion and selected material. The application compares it inside the write transaction and refuses a stale preview. Selected refs must be saved before review, so the token covers the material being reviewed. A review does not turn the selected claims or evidence into verified records. A selected claim cannot be deleted until it is removed from the conclusion.

Changing the conclusion statement, confidence, rationale or selected refs clears its recorded review. An AI, import or system edit to a checked conclusion statement also resets its verification to `ai_generated`, unless the same request explicitly sets another permitted verification, following ADR 0014. Confidence and rationale changes clear the review but keep the statement label. A person's statement edit preserves its verification. Clearing a conclusion clears its selected refs and review.

The desktop shows verification separately from confidence, selected claims with their own labels, the recorded review and a visible review-needed notice. CLI, MCP, context packs, notes and reports carry the same facts through `internal/app`. A disputed conclusion keeps an independent disputed warning even when its selected material matches its recorded fingerprint. Context budgets may omit other sections but retain the conclusion's verification and review-needed facts.

## Portability

Investigation exports now write version 2, preserving the conclusion label, selected refs, review timestamp and basis for inspection. The strict reader accepts versions 1 and 2 and refuses newer versions before decoding unfamiliar fields. Version 1 remains importable and has no attributable conclusion review. Every import treats file assertions as untrusted: checked conclusion verification is lowered to `ai_generated`, and its review timestamp, review basis and fingerprint are cleared. Selected refs remain after validation. A genuine database backup and restore preserves its local review.

## Consequences

- A person can distinguish a checked statement from a changed basis without losing the original accountable act.
- Review is deliberate and local. An AI can surface doubt and update the answer, but cannot manufacture a person's review.
- New clients must handle export version 2. Older clients receive an unsupported-version error instead of silently discarding accountability.
- Ref selections are bounded to 100 supporting and 100 contradicting claims. Attention evaluates only those selections and their linked evidence, not every potentially relevant fact in the case.
