# Casebook

## Purpose

Casebook gives AI investigations durable memory. Evidence, claims, hypotheses, questions, and conclusions live locally and can be shared across AI clients without relying on chat history. A desktop app, a CLI, and an MCP server all read and write the same local SQLite database through one Go application layer.

## Status

Released. See `CHANGELOG.md` for release history and the changes in each version. The product brief is `docs/BRIEF.md`; architecture decisions are in `docs/decisions/`; the acceptance run is `docs/acceptance.md`; open non-blocking items are in `docs/follow-ups.md`.

## Layout

- `cmd/casebook/`: CLI binary; also serves `casebook mcp` over stdio
- `desktop/`: Wails v2 desktop app (`main.go`, `wails.json`, `frontend/` with React, TypeScript, Vite)
- `internal/domain/`: types, enums, validation
- `internal/store/`: SQLite, embedded migrations, FTS5
- `internal/app/`: application service; the only API every surface calls
- `internal/contextpack/`: deterministic Markdown and JSON context packs
- `internal/mcpserver/`: MCP tools
- `internal/cli/`: CLI commands
- `internal/integrations/`: Claude Code and Codex setup, hooks
- `internal/update/`: the check for updates, the one network request (ADR 0022); only `internal/cli` and `desktop` import it
- `internal/paths/`: per-user data directory resolution
- `internal/doctor/`: diagnostics
- `docs/`: brief, decisions (ADRs), guides

## Working context

Read `docs/BRIEF.md` for the original product intent, then `docs/architecture.md` and the relevant ADRs in `docs/decisions/` before proposing or implementing product behavior. Together they set the audience, the verification guarantees and the feature boundaries; where a later ADR differs from the brief, the ADR records the current decision.

## How to test

Use Go 1.27 or later and Node 24. See `docs/development.md` for platform prerequisites. Before Go checks in a fresh checkout, run `bash scripts/stub-frontend-dist.sh` if frontend assets are absent. If npm dependencies were installed with `--ignore-scripts`, run `node desktop/frontend/scripts/fence-node-modules.mjs` to keep Go from scanning dependency fixtures.

- Go: `go build ./...`, `go vet ./...`, `go test ./...` (add `-race` before pushing concurrency changes)
- Frontend: `npm --prefix desktop/frontend test -- --run` and `npm --prefix desktop/frontend run build`
- Scripts and docs: `bash scripts/test-scripts.sh`, `bash scripts/check-docs.sh`

Tests and visual checks use a temporary, non-default database. Demo cases live in `internal/demo`; never seed fictional findings into a person's real database. Report what was actually tested, and distinguish source inspection from a live desktop check.

## Shared contracts

- Verification and confidence are separate. Only human actors can set `human_verified`; AI edits to checked content reset verification unless the AI explicitly rechecks it as `source_verified`. Enforce these rules in `internal/app`, including checkpoint paths. See ADR 0014.
- Explicit human review must compare the displayed statement and source snapshot inside the write transaction. Do not automatically acknowledge changed material or retry a stale verification against fresh data.
- Copy and Save from a reviewed preview use its exact bytes. Refresh is explicit; never silently render a newer case when sharing an older preview.
- Imported verification is untrusted. Keep old supported import formats readable when an export format changes, and version incompatible format changes explicitly.
- Context packs, reports and leadership briefs are deterministic. Preserve verification labels and truthful omission notices in every format. Review changed goldens rather than regenerating them blindly (`go test ./internal/contextpack -update` updates them).
- After changing exported `Desktop` methods, run `desktop/scripts/gen-bindings.sh` and update `desktop/frontend/src/api/api.ts`, its mirrored types, test mocks and affected fixtures together. The frontend calls the API wrapper, never Wails bindings directly.
- Package boundaries are enforced by `internal/app/boundary_test.go`. Surfaces use `internal/app` aliases instead of importing `internal/domain` or `internal/contextpack` directly.
- Casebook produces portable content (briefs, reports, exports) and never connects to another app; the person copies or saves it, or their AI client writes it with that app's own tools (ADR 0024). Casebook records nothing about external destinations and never claims an external write happened. Keep offline file formats usable without a connected service.

## Branch model

`main` is the only long-lived branch. The maintainer commits directly to `main`; contributors open pull requests against it.

## Releases

When the maintainer asks for a release, make it without asking again: follow "Making a release" in `docs/building-releases.md` end to end. Prepare the release commit, push it, wait for CI, create and push the `vMAJOR.MINOR.PATCH` tag, watch the Release workflow, then publish the draft with `gh workflow run publish-release.yml -f tag=vX.Y.Z` and confirm the Homebrew tap run. The request is the authorization for the tag push and the publish; only a failed check or a red workflow stops the release.

## Rules

- Keep `CLAUDE.md` and `AGENTS.md` identical. Update both files together whenever either file changes.
- No em dashes anywhere (code, comments, docs, UI text, commit messages). Use a colon, semicolon, comma, or parentheses instead.
- No orchestration, workflow engines, DAGs, schedulers, task runners, event sourcing, or vector/embedding infrastructure in Casebook.
- Every surface (desktop, CLI, MCP, hooks) goes through `internal/app`. Never import `internal/store` from a surface, and never duplicate business logic.
- Never write secrets, tokens, or credentials into code, logs, fixtures, or docs.
- Never record transcripts or chain-of-thought; store concise evidence only.
- UI changes update the README screenshots in the same change, and rewrite any README sentence or caption that describes them, so the README never shows or describes a screen the app no longer has.
- Keep the MCP process's stdout clean: protocol traffic only, logs go to stderr.
- Schema changes are new forward-only migrations with tests; never edit or drop shipped migrations.
