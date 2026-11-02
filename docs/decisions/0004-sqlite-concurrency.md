# 0004: SQLite concurrency across processes

- Status: Accepted
- Date: 2026-10-01

## Context

The GUI process and one or more MCP server processes may open the same database at the same time. The brief forbids a separate synchronization service and requires graceful lock handling and tests for contention.

## Decision

- Journal mode WAL, a busy_timeout, and foreign_keys on for every connection.
- Writes use short transactions started with BEGIN IMMEDIATE; contention is retried and surfaced as a friendly error.
- All processes share the database file directly; there is no sync service.
- The GUI detects external writes with a cheap poll of `PRAGMA data_version` plus the latest `updated_at`, and reloads only when it changes.

## Consequences

- MCP writes appear in the open desktop app within a few seconds.
- Long-running write transactions are not allowed anywhere.
- Concurrency is covered by tests with multiple connections and a separate OS process.
