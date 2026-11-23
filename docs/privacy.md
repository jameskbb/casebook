# Privacy and local data

## Summary

Casebook stores your questions in a database on your computer. When you connect an AI client, that client can read your questions and send the content to its model provider under that provider's terms.

- No telemetry: Casebook does not collect usage data or crash reports.
- No hosted accounts or automatic uploads by Casebook.
- No passwords, tokens or keys held by Casebook.
- One optional network request: **Look for updates...** or `casebook update` asks GitHub for the latest release. It sends no question content, settings, installed version or identifier, and downloads no update.

[What leaves your machine](#what-leaves-your-machine) explains the update request and your AI client's separate connection. [Using a question summary in other apps](#using-a-question-summary-in-other-apps) uses your AI client's connection too.

## What is stored

Casebook stores questions and what was learned in them:

- Questions, each with a ref (`INV-0002`; every question ref starts with INV), the question in your words, a status, and the current answer with why it holds (plus a confidence rating that is stored but not shown in the app).
- Sources, such as `E-001`: a title, a summary, where it came from (source type, source reference and label), optional raw detail up to 64 KiB, and optional metadata.
- Findings (`C-001`) and findings being tested (`H-001`): a statement, why it holds, a status, who checked it (you, your AI, or nobody yet), a confidence rating that is stored but not shown in the app, and links to the sources that support or contradict it.
- Questions (`Q-001`): open, resolved with a resolution, or deferred with a note.
- An activity log: who changed what and when. Each entry names the kind of actor (a person, an AI, an import or the system) and the client, for example `casebook-ui` or the name an AI client reports, such as `claude-code`.
- Settings: which question is active.
- Nothing about where you save or paste a summary. Casebook does not know or record which app, note or document received it.
- Left from earlier versions only: if you used the retired Evernote notes feature, the database still holds the notebook setting and, for each question you posted, the note's id and title, a fingerprint (SHA-256) of the block and when it was confirmed. Casebook no longer reads, shows or updates these rows; backups carry them, as they carry every table. See [If you used Evernote notes](sharing.md#if-you-used-evernote-notes).

Sources can hold raw detail that you or an AI client add, such as a query result, a log excerpt or a code snippet. Treat the database like any other file that contains your work notes.

What Casebook never stores:

- Chat transcripts, conversations, or an AI's hidden reasoning. Casebook keeps concise findings only.
- Credentials. Casebook needs no passwords, tokens or API keys and stores none, in the database or anywhere else.

## Where it lives

By default the database is a file named `casebook.db` in a per-user folder:

| System | Database file |
|---|---|
| Windows | `%LocalAppData%\Casebook\casebook.db` |
| macOS | `~/Library/Application Support/Casebook/casebook.db` |
| Linux | `$XDG_DATA_HOME/casebook/casebook.db`, or `~/.local/share/casebook/casebook.db` when `XDG_DATA_HOME` is not set or is not an absolute path |

Next to it you may see `casebook.db-wal` and `casebook.db-shm` (SQLite working files that belong to the database), `hook-state.json` when you use the Claude Code hooks, and a `backups` folder with the copies Casebook makes before a restore or an upgrade (see [Backups, export and restore](#backups-export-and-restore)). The program files (the desktop app and `casebook`) never hold data, so replacing them during an update ([Updating Casebook](updating.md)) leaves your data alone.

Two environment variables change the location:

- `CASEBOOK_DB` set to a file path uses exactly that database file. It wins over everything else.
- `CASEBOOK_HOME` set to a folder uses `casebook.db` inside that folder.

Keep the database on a local disk. SQLite's write-ahead log (the `-wal` and `-shm` files) needs shared memory that network shares (such as `\\server\share` paths or mounted SMB and NFS folders) and file sync folders (Dropbox, OneDrive, iCloud Drive, Google Drive) do not provide reliably, so pointing `CASEBOOK_DB` or `CASEBOOK_HOME` there can lose or corrupt data. To move data between computers, use backups or exports instead.

Relative values are resolved against the current folder. Every Casebook program that sees the same values uses the same database, so set them the same way for the desktop app, the CLI and your AI clients if you use them.

To see which file is in use, run `casebook status` (its `Database:` line) or `casebook doctor` (its `database` line), or in the desktop app open the **Settings** menu at the bottom of the sidebar, then **Technical...**, which shows where the data file is.

## File permissions

On macOS and Linux, the folders Casebook creates are readable only by you (mode 0700), and the database, its `-wal` and `-shm` files and every backup are readable and writable only by you (mode 0600).

On Windows these modes do not apply. The files live in your own `%LocalAppData%` folder, whose access rules already limit them to your user account.

## What leaves your machine

Casebook does not upload your questions. Your connected AI client can read and send question content, and Casebook itself can contact GitHub for an update check you start.

The one request Casebook makes is the check for updates, and only when you start it: **Look for updates...** in the desktop app's **Settings** menu, or `casebook update`. It asks GitHub's public releases API (`api.github.com`) for the latest Casebook release, sending only a fixed program name (`Casebook-update-check`) that GitHub requires. It sends no version, no identifier, nothing from your questions and no settings; your computer compares the versions after the answer arrives. GitHub sees your IP address, as for any web page you visit. Nothing is downloaded or installed, and nothing checks on a timer or at startup ([Updating Casebook](updating.md#checking-for-a-newer-version)).

AI clients are different. When you connect Claude Code, Codex or another AI client to Casebook through MCP or hooks, that client reads Casebook data (context packs, sources, findings and so on) and sends it to its model provider as part of the conversation, under that provider's terms. You decide which clients to connect, and you can remove them at any time (see [Deleting your data](#deleting-your-data)). The same applies when you copy a context pack into an AI chat yourself, and to anything an AI client carries between Casebook and another app it is connected to, such as a notes app you ask it to save a summary in (see [Using a question summary in other apps](#using-a-question-summary-in-other-apps)).

The Windows installer may download Microsoft's WebView2 runtime when it is missing. That download comes from Microsoft and is part of installing, not of using Casebook.

How this was checked (an audit on 2026-10-02, and a rerun for 0.3.0 on 2026-10-09):

- At those audits no Casebook source file made a network call: the only `net` imports were `net/url`, used to build the SQLite file URI and to decode refs in MCP resource URIs, and `TestPackageBoundaries` failed the build on any other network import outside tests. Through release 0.4.0, no HTTP client, telemetry, analytics or update check existed in the Go code or the desktop page, and the audits above found none. Since then the check for updates is the one HTTP client, in `internal/update`, which only the command line tool and the desktop app may import; `TestPackageBoundaries` now enforces that shape instead of a blanket ban, so the engine, `casebook mcp`, the hooks and doctor still have no network code path.
- `net/http` is linked into both executables only because the MCP Go SDK carries optional HTTP transports and OAuth helpers. Casebook uses its stdio transport only and never starts an HTTP server.
- The desktop page is locked by a Content-Security-Policy to its own origin (`connect-src 'self'`), its fonts are bundled, and Wails serves it from inside the process rather than from a network port.
- Run in a Linux network namespace with no network at all (`unshare -rn`), `casebook mcp` answered `initialize` and wrote a source, and `casebook doctor` passed. The 0.3.0 rerun drove `casebook mcp` through the three Evernote note tools that release had (since removed), then ran the session start hook and `casebook doctor`; everything completed with no network available.
- On a Windows 11 host, the running `Casebook.exe` process held no TCP connections. The Microsoft Edge WebView2 runtime process that draws the window did hold two HTTPS connections to a Microsoft address. That traffic belongs to the WebView2 runtime shared with other Windows apps, not to Casebook, and Casebook gives it no data: the page cannot connect anywhere but itself.

## Using a question summary in other apps

Saving a summary in a notes or document app ([how](sharing.md#using-a-shared-answer-in-other-apps)) adds no network path and no credential to Casebook.

- Casebook connects to no other app. When you copy or save a summary, the text goes where you put it. When you ask your AI client to save it in an app, the client fetches the summary from Casebook and writes it with that app's own tools, under your sign-in to that app and the client's permission prompts. That text moves through your AI client like any other data, under the client's terms and the app's.
- What a shared answer holds is what you see in **Share the answer**: the short version (the findings you choose, with the labels saying who checked them, their sources and a note of what is left out) or everything (every finding and the history of changes, never the exact words stored with a source).
- Casebook holds no sign-in for any other app and stores nothing about the destination: no note ids, no links, no record of a save.
- What the other app does with the text is governed by that app's terms, as for anything you write there.

If you ran a 0.2.0 build: that version (published on 2026-10-09 and replaced the same day) connected to Evernote itself and could store an Evernote access token. Delete `evernote-token.json` from the data folder ([Where it lives](#where-it-lives)) if it is there, unset `CASEBOOK_EVERNOTE_TOKEN` if you set it, and remove Casebook from the applications that can access your Evernote account. Later versions never read that file.

## Logs

Casebook's logs carry operation names and refs (such as `E-003`) only, never source text, raw detail or metadata, and never secrets. Casebook never writes logs to standard output. The MCP server writes its logs to standard error, which the AI client may show or save in its own logs.

## Backups, export and restore

- Do not copy `casebook.db` by hand while the desktop app or an AI client is running: recent changes may still sit in `casebook.db-wal`, and a copy of the main file alone silently misses them. Use **Save a backup...** in the **Settings** menu or `casebook backup <file>`, which are safe at any time.
- A backup is a full copy of the database. Casebook makes it with SQLite's `VACUUM INTO`, so it is consistent even while other programs are writing. It never overwrites an existing file, it is checked before Casebook reports success, and on macOS and Linux the file is readable only by you.
- Restoring checks the backup first. Before replacing anything, Casebook saves a safety copy of the current data as `<data folder>/backups/casebook-pre-restore-<UTC timestamp>.db`. It then replaces the contents in one step, so other open Casebook programs see either the old data or the new data, never a mix.
- When a newer Casebook has to upgrade the database format, the first program that opens the data saves a copy first, as `<data folder>/backups/casebook-pre-upgrade-schema<old version>-<UTC timestamp>.db`, and refuses to upgrade if it cannot. An older Casebook can restore that copy ([Updating Casebook](updating.md#going-back-to-an-older-version)).
- Casebook never deletes anything in the `backups` folder. The copies are full copies of your data, so delete old ones yourself when you no longer need them.
- Export writes one question to a JSON file (`casebook.investigation`, version 2; version 1 still imports). Exports include raw detail and the activity log, so store and share them as carefully as the database itself. The file is stable, so you can commit it to a repository and share it with your team: see [Share a question](sharing.md).
- Import treats every file as untrusted and checks every field before adding anything. It never changes which question is active. Everything it adds is marked as imported, including the file's activity history, so the app and context packs show those entries as coming from the file rather than from someone using this computer. A file's "You checked this" or "AI checked the source" is not trusted: those records start as Not checked until someone checks them here.

## Deleting your data

To remove a single mistake rather than everything, the Casebook engine can permanently delete a record or a whole question, and every place that offers Delete asks you to confirm first. A question must be archived first, and deleting it removes every record, link and activity entry in it. A record that something still depends on cannot be deleted until that dependency is removed: a source that a finding links to, or a finding that another one names as its replacement. Deleted records are gone from search, context packs and later exports, but not from backups or exports you made earlier. Their refs are never reused.

To delete all Casebook data:

1. Quit the Casebook desktop app and any AI client sessions that use Casebook, so no program has the database open.
2. Delete the data folder shown in [Where it lives](#where-it-lives), including its `backups` folder. If you set `CASEBOOK_DB` or `CASEBOOK_HOME`, delete that file or folder instead.
3. Delete any exports or backups you saved elsewhere.
4. Summaries you saved or pasted into other apps, or asked your AI client to save there, stay in those apps; edit or delete them there.

Uninstalling Casebook keeps your data on purpose, so a reinstall or an upgrade never loses it. The desktop app also has a WebView2 cache folder on Windows, `%AppData%\Casebook.exe` (a folder, despite its name), which holds no Casebook records. The uninstaller removes that folder only for the Windows account that runs it. If you approved the uninstall with a different administrator account, or other accounts on the computer used Casebook, delete `%AppData%\Casebook.exe` by hand while signed in as each of those users.

To disconnect AI clients, choose **Disconnect** for the tool under **Settings**, **Connect your AI...** in the desktop app, or run `casebook setup claude --remove` and `casebook setup codex --remove`. The app runs the same code as those commands. They remove only the entries Casebook added (the `casebook` MCP server entry and any Casebook hook handlers), back up each file they change first, and print one line per file with its backup, for example:

```text
/Users/you/.claude.json: removed mcpServers.casebook
  backup: /Users/you/.claude.json.casebook-backup-20261002T144606Z
Restart Claude Code to unload Casebook.
```

Backups are named `<file>.casebook-backup-<UTC timestamp>` next to the original (with `-2`, `-3` and so on when one already exists for that second) and are readable only by you. A file that would not change is not touched and gets no backup. These backups are full copies of the client's settings file (`~/.claude.json` holds account details and per-project state), so Casebook keeps only the newest 5 for each file: after a successful change it removes older regular files named exactly `<file>.casebook-backup-<UTC timestamp>[-N]` in the same folder and never touches any other file. The [README](../README.md#connect-your-ai) links the guides with the manual steps.

## Hooks and transcripts

The optional Claude Code hooks never read or store transcripts. Claude Code passes each hook the path of the session transcript; Casebook does not open it. The session start hook only reads the active question's context pack (and prints nothing in sessions where no question is active, unless you turn on its opt-in hint), and the checkpoint hook only prints a reminder to record new findings through the Casebook MCP tools.

The active question is one setting shared by every surface, and the hooks are installed for your user, so by default its context pack reaches every Claude Code session you start, in any directory, until you clear it. That default (the `all` scope) is by design and stays the default: a question often spans several projects, and loading it everywhere needs no per-project setup. To confine it to projects you mark with a `.casebook` file, install the hooks with `casebook setup claude --hooks --scope marked`, or set `CASEBOOK_HOOK_SCOPE=marked`; the [Claude Code guide](claude-code.md#which-sessions-get-the-context) explains both settings. The hooks read the session's working directory from the hook payload only to apply that scope and never store it.

The checkpoint hook keeps a small `hook-state.json` file next to the database. For each recent Claude Code session id it holds only times and question refs: when the session started, whether it was already reminded, and when it last called `casebook_checkpoint` for which question (the MCP server adds that from the session id Claude Code gives it, and only when the hooks have created the file). It holds no record text, payload or transcript, keeps at most 200 sessions, and can be deleted at any time.
