# 0015: Per-directory active investigation

- Status: Accepted
- Date: 2026-10-06

## Context

ADR-0008 keeps one global active investigation in the database. People who run Claude Code or Codex in several repositories at once want each session to pick up the case for the repository it runs in, without passing a ref every time and without one session switching the case under another.

The surfaces already know where they run: Claude Code hook payloads carry `cwd`, Claude Code and Codex start the `casebook mcp` process in the project directory, and the CLI has its current directory.

## Decision

- A new table `directory_investigations (dir, investigation_id, updated_at)` (migration 0005) binds a directory to an investigation. Deleting the investigation deletes its bindings (foreign key cascade). It lives in the database, next to the global setting, for the same reason as ADR-0008: every surface sees the same state and there is no config file to drift.
- `casebook investigation activate <ref> --here` binds the project root of the current directory: the nearest ancestor holding a `.git` entry (a repository or worktree), else the directory itself. `--clear --here` removes the binding that applies; `casebook investigation dirs` lists bindings. Paths are absolute, cleaned and symlink-resolved, and stored as spelled (the on-disk case when the directory exists). On Windows, whose file names are case-insensitive, a binding matches any spelling of its directory (Unicode case folding in Go, not SQL); rebinding under another spelling replaces the row, and surfaces show the stored spelling rather than a lowercased path. (Migration 0005's comment still says "lowercased on Windows"; shipped migrations are not edited.)
- `app.WithWorkDir(dir)` gives an App its working directory. Resolution of "the active investigation" (an empty ref, `GetActiveInvestigation`, `is_active`) checks the directory and then each ancestor for a binding, nearest first, and falls back to the global setting. The CLI and `casebook mcp` pass the process's current directory; the hooks pass the payload's `cwd` (translated from WSL on Windows, as the hook scope already does); the desktop app passes nothing and keeps seeing the global setting.
- Setting the active investigation from a directory where a binding applies (CLI `activate` without flags, `casebook_set_active_investigation`, create with make-active) re-points that binding instead of the global setting, so a session never silently switches the case of other repositories. `--global` forces the global setting. The CLI prints a note when a binding still wins after a global change.
- `casebook_get_active_investigation` reports `active_dir` when the case comes from a binding. `casebook status` and `casebook doctor` show `(for <dir>)`.
- Binding changes are logged as `active.bind` (on the investigation) and `active.unbind` (global). Like `active.set`, they are not material changes in context packs.

## Consequences

- Sessions in different repositories can work on different cases at the same time; everything else keeps working exactly as before when no binding exists.
- The MCP server relies on the client starting it in the project directory. Clients that start servers elsewhere (for example Claude Desktop, which has no project) see the global active investigation. MCP roots are not used; they can be added later without changing the storage.
- Bindings are machine-local paths. They are copied by backup and restore with the rest of the database but are never exported, so a shared case file carries no local paths.
- The desktop app does not show bindings yet; it shows the global active investigation as before.
