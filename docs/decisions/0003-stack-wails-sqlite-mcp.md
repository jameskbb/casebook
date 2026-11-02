# 0003: Stack: Wails v2, pure-Go SQLite, official MCP Go SDK

- Status: Accepted
- Date: 2026-10-01

## Context

The brief directs Go, SQLite, Wails, React with TypeScript, and MCP over stdio, using current stable releases and avoiding CGO where a mature pure-Go option exists, to minimize cross-platform packaging pain.

## Decision

- Desktop: Wails v2.14.0 (latest stable; v3 is still beta) with a React, TypeScript, Vite frontend.
- Database: `modernc.org/sqlite` (pure Go, no CGO, FTS5 included).
- MCP: the official `github.com/modelcontextprotocol/go-sdk` over stdio.
- Persistence: plain SQL, no ORM.
- CLI: `github.com/spf13/cobra`, or the standard library if that proves leaner (implementer decides in phase 1).
- Phase 1 pins every Go dependency in `go.mod` so parallel tracks never edit it concurrently.

## Consequences

- The CLI and MCP binary cross-compile without a C toolchain.
- Wails v2 on Windows is CGO-free, so the Windows desktop app can be cross-compiled from Linux/WSL.
- Moving to Wails v3 later is a contained change in `desktop/`.
