# 0017: `casebook setup mcp` prints configuration for any MCP client

- Status: Accepted
- Date: 2026-10-06

## Context

Casebook's MCP server works with any client that speaks MCP over stdio, but only Claude Code and Codex had setup commands. Users of Cursor, VS Code, Windsurf, Gemini CLI and Claude Desktop had to find the CLI's absolute path and hand-write JSON, escaping Windows backslashes themselves. ADR-0009 requires that Casebook only writes client config it can merge conservatively: read first, merge only Casebook-owned keys, back up, stay idempotent, offer removal.

Each of these clients keeps its config in a different file, some (VS Code) in a different shape, several allow comments or trailing commas that a strict JSON rewrite would destroy, and their locations and formats change between releases.

## Decision

- `casebook setup mcp [--client <name>] [--wsl] [--list]` prints the configuration and writes nothing. Clients: `generic` (default, the common `mcpServers` JSON), `claude-desktop`, `cursor`, `vscode` (`servers` with `"type": "stdio"`), `windsurf`, `gemini`, `claude-code` (the `claude mcp add` command) and `codex` (the managed `config.toml` block).
- The command is the running CLI's absolute path, like the other setup commands, with `--db <path>` passed through. The JSON is produced by an encoder, so Windows backslashes are escaped correctly.
- Standard output is only the snippet, so it can be pasted or redirected; the location and any note go to standard error. `--json` returns client, format, location, command, args and snippet in one object.
- On Windows, `--wsl` targets a client running inside WSL: the command becomes the `/mnt/<drive>` path of `casebook.exe` (or, for Claude Code and Codex, the same `mcp add` command `integrations.WSLSetupCommand` already produces).
- `casebook setup claude` and `casebook setup codex` stay the only commands that write client config.

## Consequences

- Every MCP client gets a correct, copy-ready configuration in one command, with no risk to the user's existing settings.
- The user merges the snippet by hand. When a client changes its config format, Casebook prints a stale snippet but never damages a file.
- Adding a client is one table entry and a docs section in `docs/mcp.md`; writing to a new client's file would need its own ADR under ADR-0009's rules.
