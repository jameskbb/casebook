# 0001: Modular monolith in one Go module

- Status: Accepted
- Date: 2026-10-01

## Context

The brief requires the desktop app, MCP server, CLI, and hooks to share one domain and application layer and one database, with no duplicated persistence or business logic. It also forbids microservices, distributed systems, and orchestration infrastructure.

## Decision

Casebook is a single Go module, `github.com/jameskbb/casebook`, targeting Go 1.27, organized as a modular monolith:

- `internal/domain`: types, enums, validation
- `internal/store`: SQLite access, embedded migrations, FTS5
- `internal/app`: the application service; the ONLY API every surface calls
- `internal/contextpack`: deterministic Markdown and JSON context pack generator
- `internal/mcpserver`: MCP tool definitions and handlers
- `internal/cli`: CLI commands
- `internal/integrations`: Claude Code and Codex setup, hook output
- `internal/paths`: per-user data dir resolution (os.UserConfigDir and LocalAppData semantics)
- `internal/doctor`: diagnostics
- `cmd/casebook`: the CLI binary, which also serves `casebook mcp` over stdio
- `desktop/`: the Wails v2 app (`desktop/main.go`, `desktop/wails.json`, `desktop/frontend` with React, TypeScript, Vite) importing `internal/app`

## Consequences

- Surfaces stay thin adapters; business rules live once in `internal/domain` and `internal/app`.
- No surface may import `internal/store` directly; review enforces this.
- One `go.mod` for everything means dependency changes are serialized (phase 1 pins all of them).
