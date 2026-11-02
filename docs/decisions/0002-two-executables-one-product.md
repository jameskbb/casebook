# 0002: Two executables, one product

- Status: Accepted
- Date: 2026-10-01

## Context

A Windows GUI-subsystem executable cannot reliably read and write stdio or attach to a terminal, but MCP over stdio and the CLI need exactly that. The brief allows a small companion CLI/MCP executable if both use the same packages and database and installation still feels like one product.

## Decision

Ship two executables built from the same packages:

- `Casebook` (GUI, Wails)
- `casebook` (CLI, and the MCP server via `casebook mcp`)

Both open the same database through `internal/app`. On macOS the CLI ships inside `Casebook.app/Contents/MacOS`; on Windows it ships next to `Casebook.exe`. The desktop app offers a "Connect AI clients" panel that shows and copies the CLI path and ready-made client configuration.

## Consequences

- No business logic is duplicated; only the entry points differ.
- AI clients are configured with the absolute path of the bundled CLI.
- Packaging must place both binaries together and stamp them with the same version.
