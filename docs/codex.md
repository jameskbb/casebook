# Codex integration

Connect Codex to read and save your Casebook cases. For the available tools, see the [MCP guide](mcp.md).

You need Casebook installed (see [Installing Casebook](install.md)), Codex installed with the `codex` command available, and the path of the `casebook` CLI:

- Windows: `<install folder>\bin\casebook.exe`
- macOS: `/Applications/Casebook.app/Contents/MacOS/bin/casebook`

The desktop app's Connect AI clients panel shows the exact path. If `casebook` is not on your PATH, use the full path wherever this guide says `casebook`.

On Windows, quote a path that contains spaces, and note that the quoting differs by shell: in PowerShell, `& 'C:\Program Files\Casebook\bin\casebook.exe' setup codex`; in Command Prompt, `"C:\Program Files\Casebook\bin\casebook.exe" setup codex`.

## Quick setup

```sh
casebook setup codex            # add Casebook to your Codex configuration
casebook setup codex --skill    # recommended: also add the Casebook skill
casebook setup codex --print    # show exactly what would be written; change nothing
casebook setup instructions >> AGENTS.md   # optional: a short AGENTS.md section, written by you
```

`casebook setup codex` adds a `[mcp_servers.casebook]` entry to `~/.codex/config.toml` (or `$CODEX_HOME/config.toml` when you set `CODEX_HOME`) when it can do so safely:

- It backs up the file first and tells you where the backup is.
- It changes only the Casebook entry and keeps the rest of your configuration as it is.
- Running it again changes nothing when the entry is already in place.
- When it cannot merge safely (the file is not valid TOML, or it already has a `[mcp_servers.casebook]` table outside Casebook's managed block), it changes nothing, prints the block for you to add by hand, and exits with status 1 after one line such as `casebook: refusing to change /Users/you/.codex/config.toml: not valid TOML (line 1, column 10); nothing was written`.

The backup sits next to the file as `config.toml.casebook-backup-<UTC timestamp>` (with `-2` and so on when one already exists for that second), readable only by you; Casebook keeps the newest 5 backups of the file and removes older ones after a successful change (see [Privacy and local data](privacy.md#deleting-your-data)). A successful run prints:

```text
/Users/you/.codex/config.toml: added mcp_servers.casebook
  backup: /Users/you/.codex/config.toml.casebook-backup-20261002T144606Z
Check with: codex mcp list (or codex mcp get casebook --json)
```

Setup writes the entry inside a managed block, which is exactly what `casebook setup codex --print` shows:

```toml
# BEGIN casebook (managed by "casebook setup codex"; remove with "casebook setup codex --remove")
# Keep root-level keys above this block: TOML reads keys below a table header as part of that table.
[mcp_servers.casebook]
command = '/Applications/Casebook.app/Contents/MacOS/bin/casebook'
args = ['mcp']
# END casebook
```

Codex itself appends tables (such as a `[projects."<path>"]` trust entry) at the end of `config.toml`, which puts them inside the block when the block ends the file. Rerunning `casebook setup codex` moves such tables out of the block, right after it, and `--remove` leaves them where the block was; `casebook doctor` warns while one sits inside the block.

When `CASEBOOK_DB` or `CASEBOOK_HOME` is set in your shell, setup notes that Codex may start `casebook` without it; pass `--db <file>` (`casebook --db <file> setup codex`) to put that database into `args`.

Codex starts MCP servers when a session starts, so start a new Codex session after setup.

## Getting Codex to use Casebook

Codex loads MCP tools on demand and, in our tests with Codex 0.156, does not show the model the Casebook server's instructions. With only the MCP entry, Codex never reached for Casebook on its own: in the agent eval (`scripts/agent-eval`) it debugged a flaky test across two sessions with zero Casebook calls, and the second session redid the first one's work from scratch. Codex needs one of these to know when Casebook applies:

- **The skill** (`casebook setup codex --skill`). Setup writes `~/.codex/skills/casebook/SKILL.md` (`$CODEX_HOME/skills/casebook/SKILL.md` when that is set). Codex lists every skill's one-line description to the model, so it loads this one when you mention earlier investigation work or agree to open a case, and it then follows the routine: find the case, read its context, record evidence, hypotheses and questions as it goes, checkpoint before its final reply. The file carries a `casebook-managed` line: a rerun replaces it (with a backup), `--remove` deletes it, and a `SKILL.md` without that line is yours and is never touched.
- **An AGENTS.md section.** `casebook setup instructions` prints a short section that tells Codex to offer a Casebook case when a question needs more than one look (why a number moved, which explanation holds, a failure whose cause is not obvious) or will continue later, to read the case before resuming, and to record findings as it goes. Setup never edits `AGENTS.md`; append it yourself (`casebook setup instructions >> AGENTS.md`) or paste it into `~/.codex/AGENTS.md` for every project.

To have Codex save a case summary in a notes or document app, connect Codex to that app as well, then ask it to save the brief or the report there; [Using a case summary in other apps](sharing.md#using-a-case-summary-in-other-apps) explains what to ask for and what to check.

Codex asks before running a tool that writes unless you approve the server's tools up front, which the smoke test below does with `-c 'mcp_servers.casebook.default_tools_approval_mode="approve"'`.

## Example configuration

The entry Casebook needs in `~/.codex/config.toml`:

```toml
[mcp_servers.casebook]
command = "/Applications/Casebook.app/Contents/MacOS/bin/casebook"
args = ["mcp"]
```

On Windows, write the path as a single-quoted TOML literal string, so backslashes need no escaping:

```toml
[mcp_servers.casebook]
command = '<install folder>\bin\casebook.exe'
args = ["mcp"]
```

## Alternative: Codex's own command

Codex can write the same entry for you:

```sh
codex mcp add casebook -- <path to casebook> mcp
```

For example on macOS:

```sh
codex mcp add casebook -- /Applications/Casebook.app/Contents/MacOS/bin/casebook mcp
```

## Codex inside WSL

On Windows, `casebook setup codex` writes the Windows-side Codex settings (`C:\Users\<you>\.codex\config.toml`). A Codex installed inside WSL reads `~/.codex/config.toml` under your Linux home folder instead. Register the Windows `casebook.exe` from a WSL shell, through the path WSL mounts it at (`C:\` becomes `/mnt/c/`, backslashes become slashes), in single quotes when it contains spaces:

```sh
codex mcp add casebook -- '/mnt/c/Users/Jo Doe/AppData/Local/Programs/Casebook/bin/casebook.exe' mcp
```

The **Connect AI clients...** panel in the desktop app shows this command for your computer under **Or in WSL (bash)**, with a Copy button. WSL runs `casebook.exe` as a Windows program, so it opens the same per-user database as the desktop app. If you pass `--db`, put it after `mcp` and keep the database path in its Windows form. The commands assume WSL's default `/mnt` mount root.

## Verify

```sh
codex mcp list
codex mcp get casebook
```

`codex mcp list` shows a table with a `casebook` row: your CLI path under Command, `mcp` under Args, and `enabled` under Status. For example:

```text
Name      Command                                                 Args  Env  Cwd  Status   Auth
casebook  /Applications/Casebook.app/Contents/MacOS/bin/casebook  mcp   -    -    enabled  Unsupported
```

The Auth column reads `Unsupported` because a local stdio server like Casebook needs no sign-in. `codex mcp get casebook` shows the details: `enabled: true`, `transport: stdio`, the command and `args: mcp`.

## Smoke test

1. Check that Casebook's MCP server starts:

   ```sh
   casebook doctor
   ```

   Among its checks, `casebook doctor` starts `casebook mcp` and completes an MCP initialize handshake. No check should fail; a warning that the desktop app was not found next to the CLI is expected when you run a CLI you built yourself.

2. Ask Codex to call a Casebook tool without opening an interactive session:

   ```sh
   codex exec "Call the casebook_get_active_investigation tool. Reply with the case ref and title, or say that no case is active."
   ```

   Expected result: Codex calls the tool and replies with the active case, for example `INV-0001 September cloud bill up 38%`, or says that none is set. Each case has a ref such as INV-0001 (every case ref starts with INV). Either answer proves Codex can see and use Casebook. If Codex says it has no such tool, check `codex mcp list` and the path in your configuration.

   `codex exec` has nobody to approve tool calls. If Codex reports that the call was declined or needs approval, pre-approve only Casebook's tools for that run, the way Casebook's own opt-in test does (it never uses Codex's bypass flag):

   ```sh
   codex exec --skip-git-repo-check -s read-only -c 'mcp_servers.casebook.default_tools_approval_mode="approve"' "Call the casebook_get_active_investigation tool. Reply with the case ref and title, or say that no case is active."
   ```

Casebook's automated tests also check that a real `codex mcp list` and `codex mcp get casebook --json` read the configuration `casebook setup codex` writes, in a throwaway `CODEX_HOME`.

## Removing

```sh
casebook setup codex --remove
```

This removes only the `[mcp_servers.casebook]` entry, after backing up the file, and the Casebook skill when its file still carries the `casebook-managed` line. Or use Codex's own command:

```sh
codex mcp remove casebook
```

Removing the integration does not delete your Casebook data; see [Privacy and local data](privacy.md#deleting-your-data).
