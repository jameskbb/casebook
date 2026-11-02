# 0006: Confidence, verification, and explicit evidence links

- Status: Accepted
- Date: 2026-10-01

## Context

The brief wants confidence on individual claims and hypotheses without fake mathematical precision, human verification states so AI output does not become unquestioned truth, and an explicit (not text-flattened) relationship between evidence and the claims or hypotheses it supports or contradicts.

## Decision

- Confidence enum: `low`, `medium`, `high`, `unknown`, always with an optional rationale.
- Verification enum on evidence, claims, and hypotheses: `ai_generated`, `human_verified`, `source_verified`, `disputed`, `superseded`.
- Explicit link tables for evidence to claim and evidence to hypothesis, each with relation `supports` or `contradicts`, constrained to the same investigation.

## Consequences

- The UI and context pack can show "supported by E-001, contradicted by E-003" directly.
- No numeric scores exist anywhere in the model.
- Superseded records are excluded from context packs by default.
