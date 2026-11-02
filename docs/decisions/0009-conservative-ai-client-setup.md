# 0009: Conservative AI client setup and hooks

- Status: Accepted
- Date: 2026-10-01

## Context

Power users configure Claude Code and Codex by hand and must not lose settings. The brief requires reading existing configuration, merging only Casebook-owned values, backing up first, idempotency, and an uninstall path. MCP must remain the primary way models write structured Casebook state.

## Decision

- `casebook setup claude` merges only Casebook-owned keys into Claude Code configuration, backs up first, is idempotent, and supports `--remove` and `--print` (dry run). Hooks are opt-in.
- `casebook setup codex` applies the same rules to `~/.codex/config.toml` when it can be merged safely, otherwise prints the exact configuration for manual install.
- `casebook hook session-start` emits a compact context pack for the active investigation as `additionalContext`.
- `casebook hook checkpoint` (Stop and PreCompact) emits a short reminder to persist discoveries via MCP. It never reads or stores transcripts.

## Consequences

- Malformed or unrecognized configuration files are refused rather than rewritten.
- Hooks fail open: an error never blocks the user's session.
