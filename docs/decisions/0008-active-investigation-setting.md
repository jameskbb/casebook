# 0008: Active investigation lives in the database

- Status: Accepted
- Date: 2026-10-01

## Context

The CLI, MCP server, and hooks need to find "the investigation I am working on" without the user repeating an ID, and every surface must agree on it.

## Decision

Store the active investigation in a `settings` key/value table in the same SQLite database. Investigation-scoped MCP tools and CLI commands default to it when no ref is given.

## Consequences

- Setting it in the desktop app is immediately visible to MCP and hooks, and vice versa.
- There is no separate config file to drift out of sync.
