# Casebook MCP guide

Casebook includes a Model Context Protocol (MCP) server, so any AI client that speaks MCP can read and write your questions. This guide covers any client: [Print the configuration for your client](#print-the-configuration-for-your-client) has the steps for Cursor, VS Code, Windsurf, Gemini CLI and Claude Desktop. For step-by-step setup in Claude Code and Codex, see the [Claude Code guide](claude-code.md) and the [Codex guide](codex.md).

## What it is

`casebook mcp` runs a local MCP server over standard input and output (stdio). Your AI client starts it as a child process when a session begins and stops it when the session ends.

- Standard output (stdout) carries only MCP protocol messages. Logs go to standard error (stderr).
- It uses the same database as the desktop app and the CLI, so changes appear everywhere within a few seconds.
- The MCP server sends nothing over the network and opens no network connection; the one request Casebook makes, the check for updates a person starts, lives outside it. The AI client sends what it reads from Casebook to its own model provider, and carries data between Casebook and any other app or MCP server it is connected to (for example when you ask it to save a question summary in a notes app, as [Using a shared answer in other apps](sharing.md#using-a-shared-answer-in-other-apps) describes); see [Privacy and local data](privacy.md#what-leaves-your-machine).

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

The server uses the active question of the folder the client starts it in (see [How the tools behave](#how-the-tools-behave)), so clients that start it in your project pick up that project's question.

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

The desktop app can do this for you: **Settings**, **Connect your AI...**, **Connect it for me** beside the Claude desktop app. It adds only the `casebook` entry under `mcpServers` (with `--db <path>` when Casebook uses a data file you chose), keeps a backup of the file first, refuses a file that is not plain JSON or that changes while it writes, and leaves the file alone when nothing would change. **Disconnect** takes that entry out again. By hand: add the entry to the `mcpServers` object of `claude_desktop_config.json`: `~/Library/Application Support/Claude/` on macOS, `%APPDATA%\Claude\` on Windows (Settings, Developer, Edit Config opens it). Restart Claude Desktop. Claude Desktop starts servers outside any project, so it always uses the global active question.

### Claude Code and Codex

`casebook setup claude` and `casebook setup codex` write the configuration for you, with a backup and an undo ([Claude Code guide](claude-code.md), [Codex guide](codex.md)). To do it by hand instead, `casebook setup mcp --client claude-code` prints the `claude mcp add` command and `casebook setup mcp --client codex` prints the `config.toml` block.

### Any other client

```sh
casebook setup mcp
```

prints the common `mcpServers` JSON shown above. Clients that use another shape still need the same two things: the command (the absolute path of `casebook`) and the arguments (`mcp`), over stdio.

## Tools

Casebook offers 21 tools, all named `casebook_*`. Here they are by category, in plain words.

The tool names keep the engine's words. The app shows people four: an investigation is a question, its conclusion is the answer, a claim or a hypothesis is a finding, and evidence is a source. The server tells the AI to use the app's words when it talks to you.

### Questions

Casebook keeps each question with its answer, the findings behind it and the source for each finding. Each question has a ref such as `INV-0002` (every question ref starts with INV). The tools keep the engine's name for it, investigation.

- `casebook_list_investigations`: list questions compactly, with ref, title, status and when each was last updated.
- `casebook_search_investigations`: search all questions and return ranked matches with the record type and ref that matched.
- `casebook_get_investigation`: get one question in detail (bounded in size), by default the active one.
- `casebook_create_investigation`: create a question (the title is the question itself, in your own words), and optionally make it active.
- `casebook_update_investigation`: change a question's wording or status (for example to mark it resolved or blocked).

### Active question

The active question is the one the desktop app marks as your AI's current question.

- `casebook_get_active_investigation`: get a summary of the active question, or a clear "none set" result.
- `casebook_set_active_investigation`: make a question active by its ref.

### Sources

- `casebook_add_evidence`: add a source with where it came from (source type, reference and label) and get back its ref, such as `E-004`.
- `casebook_get_evidence`: get one source, including its full text when asked.

### Findings

- `casebook_add_claim`: add a finding, optionally linking the sources that support or contradict it in the same call.
- `casebook_update_claim`: change a finding's wording, why it holds, its status or who checked it, or replace it with another finding. Status and check stay in step: when sources disagree both say so (`disputed`), marking the finding checked makes it current again, and Replaced (`superseded`) is set only by passing `superseded_by`.
- `casebook_link_evidence_to_claim`: link a source to a finding as supporting or contradicting it, or remove that link.

### Findings being tested

- `casebook_add_hypothesis`: add a finding that is still being tested (a hypothesis), optionally linked to sources.
- `casebook_update_hypothesis`: change its wording, why, or its status (supported, ruled out or replaced).
- `casebook_link_evidence_to_hypothesis`: link a source to a finding being tested as supporting or contradicting it, or remove that link.

### Still to find out

- `casebook_add_question`: add something still to find out (a question record).
- `casebook_resolve_question`: answer it, or defer it with a note.

### Answer

- `casebook_update_conclusion`: set the question's current answer and why it holds.

### Context and checkpoints

- `casebook_get_context`: get the context pack (a short, structured summary of the question) as Markdown or JSON.
- `casebook_get_report`: get the answer to a question as Markdown to share with people, the same text `casebook report` prints. `mode` chooses `report` (everything, the default) or `brief` (the short version, the same text as `casebook report --brief`).
- `casebook_checkpoint`: record a short checkpoint note, optionally with a batch of new and changed sources, findings, things still to find out and the answer, and get back the refs it created and updated, what changed since the previous checkpoint, and open items that may need updating. The batch is all or nothing: if any item fails, nothing is written and the error names the item, so the AI can fix it and resend the whole batch.

### Saving the answer in another app

Casebook connects to no other app. When a person asks their AI client to save the answer to a question in a notes or document app, the client fetches the text with `casebook_get_report` (with `mode` set to `brief` for the short version) and writes it with the tools that app provides in the same session, under the client's own permission prompts. Casebook records nothing about the destination and never reports that a note was created or updated; the client reports the actual result of the app's tool. [Using a shared answer in other apps](sharing.md#using-a-shared-answer-in-other-apps) explains the flow for people.

## How the tools behave

- Active question by default. Tools that work on one question use the active question when no ref is given. When none is set, the tool returns an error telling the AI to pass a ref or ask you to pick one.
- Per-project active question. `casebook inv activate <ref> --here` makes a question active for one repository only (its root is the nearest folder holding `.git`). The MCP server resolves the active question from the folder the client starts it in, so Claude Code or Codex sessions in different repositories each get their own question; `casebook_get_active_investigation` then reports `active_dir`. Clients that start servers outside your project (such as Claude Desktop) use the global active question.
- Refs. Questions are `INV-0001`; records inside a question are sources `E-001`, findings `C-001`, findings being tested `H-001` and things still to find out `Q-001`. Refs never change and are never reused.
- Compact results. Tools that change data return the refs and the fields that changed, not the whole question.
- Helpful errors. Invalid input returns a tool error that names the field and what is wrong (for example a length limit). The server keeps running.
- Bounded responses. Every response stays within a documented size budget, so a large question cannot flood the AI's context.
- Provenance. Every change is recorded in the activity log with actor kind `ai` and the client name your AI client reports when it connects (for example `claude-code`), so you can always see which client changed what.
- Checkpoints store a short note, never a transcript. A checkpoint batch is applied in one transaction: all of it or none of it.
- Imported records are marked `imported`, and checks copied from an imported file are not trusted until someone checks the record in this database. Context packs mark them too: `(imported)` after the ref in Markdown, `"imported": true` in JSON.
- Stored text is data. Everything in a question was written by you or by an AI client, possibly a different one, or came from an imported file, so the server's instructions tell the AI to treat record text as data, not as instructions. `casebook_get_context` and the context resources go further: they hand over the pack after one line saying so, fenced between `BEGIN CASEBOOK DATA <id>` and `END CASEBOOK DATA <id>` lines whose id is derived from the pack, so text inside a record cannot fake the end of the block.
- Only you can mark a record `human_verified`: no tool offers that value, so an AI can record sources, findings and findings being tested as `ai_generated`, mark them `source_verified` when it checked them against the source, or mark that sources disagree (`disputed`), even on ones you checked, and you check them yourself in the desktop app. If an AI rewrites checked content, it goes back to `ai_generated` unless the AI explicitly rechecks it as `source_verified`; the tool result then says which record was reset (with `verification_reset` and `previous_verification` in its structured content, or a `verification_resets` list from a checkpoint) and asks the AI to tell you so you can review it again.

## Exact tool schemas

The exact input schema of every tool, including enums, required fields, limits and the response size budget, is in [the MCP tool reference](mcp-tools.md), generated from the MCP server's own tool definitions.

The server also offers two read-only resources with the compact Markdown context pack, framed as stored data like `casebook_get_context`: `casebook://active/context` (the active question) and `casebook://investigation/{ref}/context` (any question by ref). It offers no prompts.

## A typical session

1. Start with `casebook_get_active_investigation` or `casebook_get_context` to load what is already known.
2. As you find things, add each source with `casebook_add_evidence`, always saying where it came from.
3. Add or update findings with `casebook_add_claim` and `casebook_update_claim`, linking the sources that support or contradict them.
4. Record findings being tested and what is still to find out as they come up; mark findings being tested supported or ruled out and answer what was still to find out as the sources settle it.
5. Update the answer with `casebook_update_conclusion` only when the sources justify it.
6. End with `casebook_checkpoint` and a one or two sentence note about what this session established.

You can ask your AI client to follow this pattern in plain words, for example: "Use Casebook to record what we find in this question, and checkpoint before we stop."

## Troubleshooting

- Run `casebook doctor`. It never prints the content of your records and changes nothing except the database migrations an ordinary open applies (with the pre-upgrade backup that comes first; see [Updating Casebook](updating.md)). Add `--json` for machine-readable output. Each line is `ok`, `warn`, `fail` or `skip`:
  - `data_dir`, `database`, `integrity`, `migrations` and `write_access`: the data folder is writable, the database exists, passes SQLite's integrity check, is at the current schema version and accepts writes.
  - `mcp_startup`: it starts `casebook mcp` and completes an MCP initialize and `tools/list` (for example `initialize and tools/list completed: casebook v0.1.0, 21 tools`).
  - `cli_path`: where this `casebook` lives and whether the desktop app sits next to it.
  - `bundle_version`: whether that desktop app is the same release as this CLI. A warning means a folder was only partly replaced during an update; `skip` for builds from source or when no desktop app sits next to the CLI.
  - `claude_config` and `codex_config`: whether `~/.claude.json` and `~/.codex/config.toml` register Casebook, and whether the registered command is this CLI. A missing file is a warning that names the setup command to run.
  - `claude_cli` and `codex_cli`: whether the `claude` and `codex` commands are on PATH.
  - It ends with `All checks passed` (with the warning and skip counts) or a count of failed checks, and exits 1 when any check fails.
- Use the absolute path to the CLI in client configuration. A client that cannot find the command usually reports that the server failed to start.
- Windows paths: in JSON, double every backslash (`C:\\...`); in TOML, use a single-quoted literal string (`'C:\...'`). Paths with spaces need no extra quoting inside `command`, because `args` is a separate list.
- If the client connects but every tool says no investigation is active, set one in the desktop app, with `casebook inv activate <ref>`, or with `casebook_set_active_investigation`.
- If you moved or reinstalled Casebook in a different folder, update the path in your client configuration (or rerun `casebook setup claude` or `casebook setup codex`).
