# Contributing to Casebook

Thank you for helping. Casebook is a small, local-first tool, and careful contributions of every size are welcome:

- bug reports with clear steps to reproduce
- fixes for bugs and rough edges
- documentation corrections and clearer wording
- tests that cover behavior nobody has pinned down yet
- improvements to the build scripts, packaging and CI

For a larger change (a new feature, a new command or MCP tool, a schema change, or anything that changes how the pieces fit together), please open an issue first and describe what you want to do and why. Agreeing on the approach before you write the code saves everyone time.

## Setup and testing

[Development](docs/development.md) covers everything: prerequisites (Go, Node 24 and the Wails CLI), running the desktop app and the CLI during development, building for every target (including the Windows cross-compile from WSL) and what CI checks.

Before you open a pull request, run the [quick checks](docs/development.md#quick-checks) from the repository root. They build, vet and test the Go code, run the script self-tests, lint the scripts and workflows, and run the documentation gate. CI runs the same checks, plus the race detector and the frontend tests, on every push and every pull request.

To understand the code first, start with the [architecture overview](docs/architecture.md) and the [engine API reference](docs/api.md).

## Project rules

These come from the project's agent guide, [CLAUDE.md](CLAUDE.md), and apply to every change:

- **No em dashes anywhere:** code, comments, docs, UI text and commit messages. Use a colon, semicolon, comma or parentheses instead. `scripts/check-docs.sh` fails on any em dash character in the repository.
- **Every surface goes through `internal/app`.** The desktop app, the CLI, the MCP server and the hooks call the application service and nothing else. Never import `internal/store` from a surface, and never duplicate business logic. The boundary test in `internal/app/boundary_test.go` enforces the import rules.
- **No orchestration.** Casebook has no workflow engines, DAGs, schedulers, task runners, event sourcing, or vector or embedding infrastructure, and pull requests that add them will not be accepted. The [architecture overview](docs/architecture.md#why-there-is-no-orchestration) explains why.
- **Never write secrets, tokens or credentials** into code, logs, test fixtures or docs.
- **Never record transcripts** or chain-of-thought. Casebook stores concise evidence and findings only.
- **Keep the MCP server's stdout clean.** `casebook mcp` writes protocol traffic only to stdout; every log line goes to stderr.
- **Schema changes are new, forward-only migrations with tests.** Add a new numbered file under `internal/store/migrations/`; never edit or drop a migration that has shipped.
- **New `App` methods need a contract and documentation.** Add the method to the contract test in `internal/app/contract_test.go` and describe it in [docs/api.md](docs/api.md) in the same change.
- **UI changes update the README images** in the same change (`docs/screenshots/` and `docs/images/`), with any README sentence or caption that describes them.

## Writing for readers

Write for a busy manager or analyst investigating a real question with AI. They
know their work, but may be new to Casebook.

- Start with their question or next action. Introduce record types and refs when
  the reader needs them.
- Give each passage one job. Explain value in introductions, actions in
  instructions, and recovery in errors.
- Use concrete situations, ordinary verbs and direct sentences. Delete generic
  claims and repeated explanations rather than polishing them.
- Keep examples focused on what was learned, its sources and what remains
  uncertain. Preserve the facts without demonstrating every feature at once.
- Name who does what: Casebook, the AI client or the person. Describe guarantees
  precisely and keep them near the decision they affect.
- Keep verification labels, confidence, source limitations, commands and control
  names accurate. Use the same term for the same thing across surfaces.

## Commit messages

Write the subject line in plain, present-tense words that say what the change does, as the history does: `Keep tables Codex appends inside the Casebook block in config.toml` says more than `Fix Codex setup`. A conventional `type(scope):` prefix (`fix(store): reject a backup path inside the data folder`) is welcome but not required. No em dashes, here or anywhere else.

Keep each commit focused on one change, and say why in the body when the reason is not obvious from the diff.

## Pull request checklist

- [ ] The [quick checks](docs/development.md#quick-checks) pass locally.
- [ ] New behavior has tests, and fixed bugs have a test that fails without the fix.
- [ ] Docs are updated: the guides for user-visible behavior, [docs/api.md](docs/api.md) for `internal/app` changes, and the README screenshots for UI changes.
- [ ] `scripts/check-docs.sh` passes (no em dashes, no broken links).
- [ ] No secrets, credentials or personal data in code, fixtures, logs or docs.

## Security issues

Do not report security vulnerabilities in public issues or pull requests. Follow [SECURITY.md](SECURITY.md) to report them privately.

## License

Casebook is released under the [MIT License](LICENSE). By contributing, you agree that your contributions are licensed under the same terms.
