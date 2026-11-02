# Casebook MCP guide

Casebook includes a Model Context Protocol (MCP) server, so any AI client that speaks MCP can read and write your cases. This guide covers any client: [Print the configuration for your client](#print-the-configuration-for-your-client) has the steps for Cursor, VS Code, Windsurf, Gemini CLI and Claude Desktop. For step-by-step setup in Claude Code and Codex, see the [Claude Code guide](claude-code.md) and the [Codex guide](codex.md).

## What it is

`casebook mcp` runs a local MCP server over standard input and output (stdio). Your AI client starts it as a child process when a session begins and stops it when the session ends.

- Standard output (stdout) carries only MCP protocol messages. Logs go to standard error (stderr).
- It uses the same database as the desktop app and the CLI, so changes appear everywhere within a few seconds.
- The MCP server sends nothing over the network and opens no network connection; the one request Casebook makes, the check for updates a person starts ([ADR 0022](decisions/0022-check-for-updates.md)), lives outside it. The AI client sends what it reads from Casebook to its own model provider, and carries data between Casebook and any other app or MCP server it is connected to (for example when you ask it to save a case summary in a notes app, as [Using a case summary in other apps](sharing.md#using-a-case-summary-in-other-apps) describes); see [Privacy and local data](privacy.md#what-leaves-your-machine).

## Server command

The command is the absolute path of the `casebook` CLI followed by `mcp`:

```text
<path to casebook> mcp
```

Where the CLI is installed:

- Windows: `<install folder>\bin\casebook.exe`
- macOS: `/Applications/Casebook.app/Contents/MacOS/bin/casebook`
- Built from source: wherever you put the binary (see the [development guide](development.md)).

The desktop app's Connect AI clients panel shows the exact path on your computer. Always configure clients with the absolute path: AI clients often start servers without your shell's PATH, so a bare `casebook` may not be found.

## Generic client configuration

Most MCP clients accept a JSON configuration with an `mcpServers` object. For Casebook, set `command` to the CLI path and `args` to `["mcp"]`:

```json
{
  "mcpServers": {
    "casebook": {
      "command": "/Applications/Casebook.app/Contents/MacOS/bin/casebook",
      "args": ["mcp"]
    }
  }
}
```

On Windows, JSON needs every backslash doubled:

```json
{
  "mcpServers": {
    "casebook": {
      "command": "<install folder>\\bin\\casebook.exe",
      "args": ["mcp"]
    }
  }
}
```

To point a client at a different database (for example a test database), add an `env` object with `CASEBOOK_HOME` (a folder that holds `casebook.db`) or `CASEBOOK_DB` (a database file path):

```json
{
  "mcpServers": {
    "casebook-test": {
      "command": "/Applications/Casebook.app/Contents/MacOS/bin/casebook",
      "args": ["mcp"],
      "env": {
        "CASEBOOK_HOME": "/Users/you/casebook-test"
      }
    }
  }
}
```

[Privacy and local data](privacy.md#where-it-lives) explains how these variables choose the database.

## Print the configuration for your client

`casebook setup mcp` prints the configuration for a client with the absolute path of the `casebook` you ran it with, so you never have to look the path up:

```sh
casebook setup mcp --client cursor
```

It writes nothing. Standard output is only the snippet (JSON, TOML or a command), so you can paste it or redirect it; standard error says where it goes. `--list` shows every client it knows, `--json` returns the snippet, its format and its location as one JSON object, and `--db <path>` adds `--db` to the server command. Merge the snippet into the client's file by hand: if the file already has an `mcpServers` (or `servers`) object, add the `casebook` entry inside it rather than replacing the file.

On Windows, `--wsl` prints the configuration for a client running inside WSL: it starts the Windows `casebook.exe` through its `/mnt/<drive>` path, so both sides share the desktop app's database.

The server uses the active case of the folder the client starts it in (see [How the tools behave](#how-the-tools-behave)), so clients that start it in your project pick up that project's case.

### Cursor

```sh
casebook setup mcp --client cursor
```

Add the `casebook` entry to the `mcpServers` object of `~/.cursor/mcp.json` (every project) or `.cursor/mcp.json` in one project. Cursor's settings list the server and show whether it started.

### Visual Studio Code

```sh
casebook setup mcp --client vscode
```

VS Code uses a `servers` object with `"type": "stdio"`. Put it in `.vscode/mcp.json` in a workspace (`casebook setup mcp --client vscode > .vscode/mcp.json` when the file does not exist yet), or in your user configuration (command palette: MCP: Open User Configuration). Casebook's tools appear in Copilot Chat's agent mode.

### Windsurf

```sh
casebook setup mcp --client windsurf
```

Add the entry to the `mcpServers` object of `~/.codeium/windsurf/mcp_config.json`, then refresh the MCP servers in Windsurf.

### Gemini CLI

```sh
casebook setup mcp --client gemini
```

Add the entry to the `mcpServers` object of `~/.gemini/settings.json` (every project) or `.gemini/settings.json` in one project. In Gemini CLI, `/mcp` lists the server and its tools.

### Claude Desktop

```sh
casebook setup mcp --client claude-desktop
```

Add the entry to the `mcpServers` object of `claude_desktop_config.json`: `~/Library/Application Support/Claude/` on macOS, `%APPDATA%\Claude\` on Windows (Settings, Developer, Edit Config opens it). Restart Claude Desktop. Claude Desktop starts servers outside any project, so it always uses the global active case.

### Claude Code and Codex

`casebook setup claude` and `casebook setup codex` write the configuration for you, with a backup and an undo ([Claude Code guide](claude-code.md), [Codex guide](codex.md)). To do it by hand instead, `casebook setup mcp --client claude-code` prints the `claude mcp add` command and `casebook setup mcp --client codex` prints the `config.toml` block.

### Any other client

```sh
casebook setup mcp
```

prints the common `mcpServers` JSON shown above. Clients that use another shape still need the same two things: the command (the absolute path of `casebook`) and the arguments (`mcp`), over stdio.

## Tools

Casebook offers 21 tools, all named `casebook_*`. Here they are by category, in plain words.

### Cases

A case is one question and everything learned about it. Each case has a ref such as `INV-0002` (every case ref starts with INV). The tools keep the engine's name for it, investigation ([ADR 0021](decisions/0021-case-is-the-word.md)).

- `casebook_list_investigations`: list cases compactly, with ref, title, status, confidence and when each was last updated.
- `casebook_search_investigations`: search all cases and return ranked matches with the record type and ref that matched.
- `casebook_get_investigation`: get one case in detail (bounded in size), by default the active one.
- `casebook_create_investigation`: create a case with a title and primary question, and optionally make it active.
- `casebook_update_investigation`: change a case's title, problem statement or status (for example to mark it resolved or blocked).

### Active case

The active case is the one the desktop app marks as your AI's current case.

- `casebook_get_active_investigation`: get a summary of the active case, or a clear "none set" result.
- `casebook_set_active_investigation`: make a case active by its ref.

### Evidence

- `casebook_add_evidence`: add evidence with where it came from (source type, reference and label) and get back its ref, such as `E-004`.
- `casebook_get_evidence`: get one piece of evidence, including its raw detail when asked.

### Claims

- `casebook_add_claim`: add a claim, optionally linking evidence that supports or contradicts it in the same call.
- `casebook_update_claim`: change a claim's statement, confidence, rationale, status or verification, or supersede it with another claim. Status and verification stay in step ([ADR-0013](decisions/0013-status-verification-invariant.md)): disputing a claim sets both to disputed, verifying a disputed claim makes it active again, and `superseded` is set only by superseding (passing `superseded_by`).
- `casebook_link_evidence_to_claim`: link evidence to a claim as supporting or contradicting, or remove that link.

### Hypotheses

- `casebook_add_hypothesis`: add a hypothesis that is still being tested, optionally with evidence links.
- `casebook_update_hypothesis`: change a hypothesis's statement, confidence, rationale, status (supported, disproven or superseded) or verification.
- `casebook_link_evidence_to_hypothesis`: link evidence to a hypothesis as supporting or contradicting, or remove that link.

### Questions

- `casebook_add_question`: add an open question.
- `casebook_resolve_question`: resolve a question with its answer, or defer it with a note.

### Conclusion

- `casebook_update_conclusion`: set the case's current conclusion, its confidence and the rationale.

### Context and checkpoints

- `casebook_get_context`: get the context pack (a short, structured summary of the case) as Markdown or JSON.
- `casebook_get_report`: get a Markdown write-up of the case to share (a regional review, a status note, a pull request description or a postmortem), the same complete report `casebook report` prints. `mode` chooses `report` (the complete report, the default) or `brief` (the leadership brief with Casebook's default selection, the same text as `casebook report --brief`).
- `casebook_checkpoint`: record a short checkpoint note, optionally with a batch of new and changed evidence, claims, hypotheses, questions and the conclusion, and get back the refs it created and updated, what changed since the previous checkpoint, and open items that may need updating. The batch is all or nothing: if any item fails, nothing is written and the error names the item, so the AI can fix it and resend the whole batch.

### Saving a summary in another app

Casebook connects to no other app. When a person asks their AI client to save a case summary in a notes or document app, the client fetches the text with `casebook_get_report` (with `mode` set to `brief` for the leadership brief) and writes it with the tools that app provides in the same session, under the client's own permission prompts. Casebook records nothing about the destination and never reports that a note was created or updated; the client reports the actual result of the app's tool. [Using a case summary in other apps](sharing.md#using-a-case-summary-in-other-apps) explains the flow for people.

## How the tools behave

- Active case by default. Tools that work on one case use the active case when no ref is given. When none is set, the tool returns an error telling the AI to pass a ref or ask you to pick one.
- Per-project active case. `casebook case activate <ref> --here` makes a case active for one repository only (its root is the nearest folder holding `.git`). The MCP server resolves the active case from the folder the client starts it in, so Claude Code or Codex sessions in different repositories each get their own case; `casebook_get_active_investigation` then reports `active_dir`. Clients that start servers outside your project (such as Claude Desktop) use the global active case. See [ADR-0015](decisions/0015-per-directory-active-investigation.md).
- Refs. Cases are `INV-0001`; records inside a case are evidence `E-001`, claims `C-001`, hypotheses `H-001` and questions `Q-001`. Refs never change and are never reused.
- Compact results. Tools that change data return the refs and the fields that changed, not the whole case.
- Helpful errors. Invalid input returns a tool error that names the field and what is wrong (for example a length limit). The server keeps running.
- Bounded responses. Every response stays within a documented size budget, so a large case cannot flood the AI's context.
- Provenance. Every change is recorded in the activity log with actor kind `ai` and the client name your AI client reports when it connects (for example `claude-code`), so you can always see which client changed what.
- Checkpoints store a short note, never a transcript. A checkpoint batch is applied in one transaction: all of it or none of it.
- Imported records are marked `imported`, and verification copied from an imported file is not trusted until someone verifies the record in this database. Context packs mark them too: `(imported)` after the ref in Markdown, `"imported": true` in JSON.
- Stored text is data. Everything in a case was written by you or by an AI client, possibly a different one, or came from an imported file, so the server's instructions tell the AI to treat record text as data, not as instructions. `casebook_get_context` and the context resources go further: they hand over the pack after one line saying so, fenced between `BEGIN CASEBOOK DATA <id>` and `END CASEBOOK DATA <id>` lines whose id is derived from the pack, so text inside a record cannot fake the end of the block.
- Only you can mark a record `human_verified` ([ADR-0014](decisions/0014-only-people-verify.md)): no tool offers that value, so an AI can record evidence, claims and hypotheses as `ai_generated`, mark them `source_verified` when it checked them against the source, or dispute them (even ones you verified), and you confirm them in the desktop app. If an AI rewrites checked content, it goes back to `ai_generated` unless the AI explicitly rechecks it as `source_verified`; the tool result then says which record was reset (with `verification_reset` and `previous_verification` in its structured content, or a `verification_resets` list from a checkpoint) and asks the AI to tell you so you can review it again.

## Exact tool schemas

The exact input schema of every tool, including enums, required fields, limits and the response size budget, is in [the MCP tool reference](mcp-tools.md), generated from the MCP server's own tool definitions.

The server also offers two read-only resources with the compact Markdown context pack, framed as stored data like `casebook_get_context`: `casebook://active/context` (the active case) and `casebook://investigation/{ref}/context` (any case by ref). It offers no prompts.

## A typical session

1. Start with `casebook_get_active_investigation` or `casebook_get_context` to load what is already known.
2. As you find things, add evidence with `casebook_add_evidence`, always with its source.
3. Add or update claims with `casebook_add_claim` and `casebook_update_claim`, linking the evidence that supports or contradicts them.
4. Record hypotheses and open questions as they come up; mark hypotheses supported or disproven and resolve questions as the evidence settles them.
5. Update the conclusion with `casebook_update_conclusion` only when the evidence justifies it.
6. End with `casebook_checkpoint` and a one or two sentence note about what this session established.

You can ask your AI client to follow this pattern in plain words, for example: "Use Casebook to record what we find in this case, and checkpoint before we stop."

## Troubleshooting

- Run `casebook doctor`. It never prints evidence content and changes nothing except the database migrations an ordinary open applies (with the pre-upgrade backup that comes first; see [Updating Casebook](updating.md)). Add `--json` for machine-readable output. Each line is `ok`, `warn`, `fail` or `skip`:
  - `data_dir`, `database`, `integrity`, `migrations` and `write_access`: the data folder is writable, the database exists, passes SQLite's integrity check, is at the current schema version and accepts writes.
  - `mcp_startup`: it starts `casebook mcp` and completes an MCP initialize and `tools/list` (for example `initialize and tools/list completed: casebook v0.1.0, 21 tools`).
  - `cli_path`: where this `casebook` lives and whether the desktop app sits next to it.
  - `bundle_version`: whether that desktop app is the same release as this CLI. A warning means a folder was only partly replaced during an update; `skip` for builds from source or when no desktop app sits next to the CLI.
  - `claude_config` and `codex_config`: whether `~/.claude.json` and `~/.codex/config.toml` register Casebook, and whether the registered command is this CLI. A missing file is a warning that names the setup command to run.
  - `claude_cli` and `codex_cli`: whether the `claude` and `codex` commands are on PATH.
  - It ends with `All checks passed` (with the warning and skip counts) or a count of failed checks, and exits 1 when any check fails.
- Use the absolute path to the CLI in client configuration. A client that cannot find the command usually reports that the server failed to start.
- Windows paths: in JSON, double every backslash (`C:\\...`); in TOML, use a single-quoted literal string (`'C:\...'`). Paths with spaces need no extra quoting inside `command`, because `args` is a separate list.
- If the client connects but every tool says no investigation is active, set one in the desktop app, with `casebook case activate <ref>`, or with `casebook_set_active_investigation`.
- If you moved or reinstalled Casebook in a different folder, update the path in your client configuration (or rerun `casebook setup claude` or `casebook setup codex`).
