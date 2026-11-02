# Changelog

All notable changes to Casebook are recorded here. Versions follow [Semantic Versioning](https://semver.org/); while the version is 0.x, a minor release may change behavior.

## Unreleased

## 0.8.1 (2026-11-01)

### Changed

- The example case reads more plainly. The sales VP's suspicion is now deep discounts on the quarter's two biggest deals, with Pinecrest Logistics as the customer, in the seeded demo case, the README and the brief. Record refs and the rest of the case are unchanged.
- The README and the install guide list the install script and `go install`; Homebrew is no longer listed, since the tap is not published.
- Install and update docs no longer describe the repository as private.

## 0.8.0 (2026-10-27)

Casebook now reads the same way in the app, the shared brief, the command line and what your AI is told. Machine names are unchanged: MCP tool and argument names, CLI commands and flags, JSON fields and enum values, refs and the export format stay as they were. No schema change.

### Added

- The leadership brief can carry the explanations still being tested: **Share summary** has a fourth group, **Explanations still open or supported**, and the brief prints them under a section of that name. The default brief from `casebook report --brief` and `casebook_get_report` includes up to two of them. Ruled-out explanations and rejected claims print under their own **Ruled out** section instead of among the findings still in doubt.
- The brief ends its Scope and omissions with a short paragraph saying what Human verified, Source verified, AI generated, Disputed and Superseded mean, for a reader without the app.
- The case's **Still open** section lists open explanations above the questions, and deferred questions show with their reason.
- A new case with no records shows **Get your first finding**, with **Connect AI clients...** and **Add evidence myself**.
- When an AI's edit resets a statement a person checked, the MCP result names the record, says it was reset to AI generated and asks the AI to tell you; the structured content carries `verification_reset` and `previous_verification` (`verification_resets` from a checkpoint).

### Changed

- Brief and report wording: a standing claim is **Current**, never Active; Disputed prints once; the review time reads `2026-10-09 12:55 UTC`; the conclusion's basis reads `Supporting claims: C-002 (AI generated); C-003 (Human verified)`. In the brief, evidence under a finding shows its title and label (the source prints once under Evidence), ruled-out explanations show no confidence, the omission notice names what was left out (`Not included: Q-001, Q-002 and E-002.`) and tells the reader to ask for the complete report, the closing says disputed findings, contradicting evidence and the claims behind the answer are always included, and the footer reads "Prepared by Casebook from the saved records of INV-0002. Casebook did not rewrite them or add to them."
- Context packs keep square brackets but drop Active and the repeated Disputed and print the readable review time.
- A ruled-out explanation in the case reads "Ruled out by E-005: ..." with the evidence title in full, instead of a confidence meter.
- First run: the empty screen leads with **Connect your AI**, then **New case**; the new case title hint uses the margin example.
- **Review findings** filters name the label they select ("AI generated, not yet checked by you", "Source verified, not yet checked by you"), Support needs attention has a one-line explanation, and a case with nothing to check says "Nothing to review yet".
- **What the labels mean** explains confidence, Not assessed and Current, and says Source verified is usually marked by your AI and that only Human verified means a person vouched for it.
- History's second lines are plain sentences ("Marked C-003 Human verified", "Made INV-0001 your AI's current case", "Recorded a review of the conclusion"), including a reset after an AI edit. The sidebar marker reads **Your AI's current case**.
- The CLI prints Not assessed instead of Unknown in `case list` and `case show`, and `case --help` and `status` tie "active" to your AI's current case.
- The MCP server instructions, the session start hint, the skill description, `casebook setup instructions` and the `casebook_get_report` description lead with business questions (why a number moved, which explanation holds) and keep the engineering ones.
- The README explains every label, including Not assessed, and says the agent evaluation measures engineering scenarios. [Sharing a case](docs/sharing.md) describes the brief and treats an export as moving your own case or handing someone a copy.

## 0.7.0 (2026-10-27)

Casebook removes its Evernote-specific integration and makes its summaries the portable way to take a case into other apps: copy or save them, or ask your AI client to save them to an app it is already connected to ([ADR 0024](docs/decisions/0024-portable-output-instead-of-evernote.md), [Using a case summary in other apps](docs/sharing.md#using-a-case-summary-in-other-apps)).

### Removed

- Evernote notes: the desktop app's **Evernote block** panel with its **Setup steps**, **Notebook for new notes** field and "Last posted" line; the `casebook evernote` commands (`setup`, `preview`, `status`, `notebook` and the others); the MCP tools `casebook_get_note_block`, `casebook_prepare_note_update` and `casebook_confirm_note_posted`; ENEX import and export and ENML note bodies; the Evernote guidance in the MCP server instructions and the generated `/casebook` skill. Why: the feature served one vendor's paid plans, depended on a note body format Evernote does not document, was never tested against Evernote's server, and made Casebook keep a record of remote notes it could not verify. Casebook's job is the content; the person's AI client already does the writing.
- Moving on: use **Share summary** (Copy or Save as file...), `casebook report` (with `--brief` and `-o FILE`) or `casebook_get_report` (with `mode`), and ask your AI client to save the result to a connected notes app. `casebook evernote` now prints a short pointer to these and exits non-zero. AI clients no longer see the three note tools. To record a note as evidence, ask your AI client to add its text with an accurate source reference; Casebook does not fetch notes.
- Kept for compatibility: the `evernote_note` source type and its "Evernote note" label (existing evidence is never relabelled); migrations 0006 and 0007 and the now unused `evernote_links` table, which backup and restore still carry; existing settings rows; old History entries about posts, which still render. The case export never included Evernote links, so its format version is unchanged. ENEX files you saved stay on disk; Casebook no longer reads them.

### Added

- `casebook report --brief` prints the leadership brief with Casebook's default selection; `-o FILE` saves it.
- `casebook_get_report` takes `mode`: `report` (the complete report, the default) or `brief` (the leadership brief).
- **Share summary** gains **Use this summary in another app**, a short note under the preview on copying or saving the summary or asking your AI client to save it to an app it is connected to, with an example request built from the open case and a **Copy example request** button for pasting it into your AI client.

### Changed

- Docs: [Share a case](docs/sharing.md) now covers using a case summary in other apps, directly or through your AI client, and what to do if you used Evernote notes. [Evernote notes](docs/evernote.md) is a short retirement note. The README, the MCP, Claude Code, Codex, privacy, architecture and engine API guides no longer describe the Evernote feature.

## 0.6.0 (2026-10-23)

### Added

- A small "Made with ❤️ by jameskbb" credit beneath the version in the desktop app's Data menu.

### Changed

- Clearer introductions, examples, onboarding and CLI help, with shorter explanations and a contributor voice guide.
- Verification help describes recorded checks without assuming who wrote or checked a record. Privacy, deletion, export and Evernote guidance states the limits of each guarantee.

## 0.5.0 (2026-10-23)

### Added

- Focused **Review findings**: inspect statements beside supporting and contradicting evidence, filter findings needing attention, personally verify source-checked findings, and explicitly settle disputes. Snapshot checks refuse verification of material changed by another client; live drawers require acknowledging refreshed source material.
- Conclusion verification, selected supporting and contradicting claims, and explicit human review with a timestamp and concise basis. Changes to selected material raise a review-needed notice while preserving the historical statement label. Schema 8 upgrades existing conclusions without inventing a review ([ADR 0023](docs/decisions/0023-conclusion-accountability.md)).
- A selected leadership brief alongside the complete report. Current caveats, disputed sources and the answer's claim basis stay included; omitted records are counted. Copy and Save preserve the exact reviewed preview across later case edits.
- A fictional, read-only margin example and direct AI connection guidance on the first screen. Setup shows one selected client's instructions, with alternative shells and diagnostics disclosed separately.
- Source-sensitive capture hints and an [evidence guide](docs/evidence.md) for reporting periods, filters, definitions, units and exact source locations. Illustrative examples never populate evidence fields.
- Check for updates: **Check for updates...** in the desktop app's **Data** menu and `casebook update` (with `--json`) ask GitHub whether a newer release exists and show its release page and the download for your system. It runs only when you choose it, never at startup or on a timer, and never downloads or installs anything. It sends one request to GitHub's public releases API with no version, identifier or data, and it is the one network request Casebook makes ([ADR 0022](docs/decisions/0022-check-for-updates.md)); the engine, `casebook mcp` and the hooks still have no network code path. While the repository was private, the check reported that GitHub lists no release.

### Changed

- Evidence verification travels with references in full and compact context packs, note blocks and reports. Human-facing headings say **Current claims**. Size-limited packs retain verification and explicit notices about omitted disputes and evidence outside the included findings.
- Investigation exports write format version 2 for conclusion accountability. Import still reads version 1 and clears untrusted conclusion reviews; local backups retain their original reviews.
- `AGENTS.md` and `CLAUDE.md` now carry identical repository guidance, enforced by the documentation check.
- The release workflow installs NSIS from the official zip, checked against a pinned SHA-256, when the Chocolatey feed is down; the first 0.4.0 build lost both Windows jobs to such an outage and was re-run once the feed returned.
- The release workflow keeps its artifacts briefly instead of GitHub's 90-day default: the per-job builds for 3 days and the `release-bundle` for 7. Each run uploads about 300 MB, and the old runs had filled the account's Actions storage.

## 0.4.0 (2026-10-13)

The desktop app and the docs now speak to the person who downloads the app rather than to the engineer who built it ([ADR 0021](docs/decisions/0021-case-is-the-word.md)). No schema change; a 0.3.0 database opens unchanged.

### Added

- A release-commit checklist in [Building releases](docs/building-releases.md), and `scripts/check-docs.sh --release` now fails a stable tag when `CHANGELOG.md`, `desktop/wails.json` or the frontend package does not carry that version.
- A Publish release workflow (`gh workflow run publish-release.yml -f tag=v1.2.3`) that checks a draft release's files and checksums, publishes it, marks every earlier published release as superseded with a notice at the top of its notes, and updates the Homebrew tap. `supersede_only=true` writes the notices for a release published by hand.

### Changed

- The desktop app's primary action is now **Share summary**: a readable write-up of the case with Copy and Save as file, its timeline in plain words ("Marked C-001 Human verified", "Ruled out H-001"). Copy report and Copy as plain Markdown left the More actions menu; Share summary's Copy and **Copy case as text** replace them. Copy context is now **Copy for your AI**; the AI toggle reads **Use with my AI** and the sidebar marks that case **Your AI's case**; evidence chips name the evidence with the ref second; every verification label has a one-line explanation and a **What the labels mean** dialog; a standing claim is **Current**.
- The README leads with downloading the app and connecting your AI; the command line section moved to the end.
- Everywhere a person reads, an investigation is now a **case**: New case, Search cases, Archive case; `casebook case` is an alias for `casebook investigation`. Refs, the API and the MCP tools are unchanged (ADR 0021).
- The demo, README and screenshots now feature the Northeast margin case; the flaky test and a checkout outage are sidebar cases.

## 0.3.0 (2026-10-09)

Evernote notes keeps its shape (a labelled case block that Casebook alone replaces, between visible `BEGIN CASEBOOK` and `END CASEBOOK` lines) and changes how it reaches Evernote: Casebook no longer connects to Evernote at all ([ADR 0020](docs/decisions/0020-evernote-through-client-mcp.md), [Evernote notes](docs/evernote.md)).

### Added

- Posting through your AI client: connected to Evernote's own MCP server, the client fetches the note with `get_note`, Casebook's new `casebook_prepare_note_update` tool returns the note body with only Casebook's block replaced or appended, the client writes it with `edit_note` after you approve the write, and the new `casebook_confirm_note_posted` tool records the post only after checking the note read back holds the block exactly once. The history names the AI client that posted.
- Note bodies in Markdown, HTML or ENML: Casebook detects the format, splices only its block and keeps every other byte, because Evernote does not document which format its MCP tools use.
- `casebook_get_note_block` takes a `format` (`markdown`, the default, `html`, `enml` or `text`) and returns the Last posted line, the notebook for new notes and instructions for the agent. The MCP server instructions and the `/casebook` skill describe the posting flow.
- `casebook evernote setup` prints the steps to connect Claude Code, Claude Desktop, Codex and other clients to Evernote's MCP server; the desktop app shows the same text under **Setup steps**.
- A notebook for new notes: `casebook evernote notebook [NAME | --clear]` and the desktop field **Notebook for new notes**. The note tools pass it to the agent as a hint for `create_note`; no MCP tool can change it.
- The desktop app's **Evernote block** panel (More actions): the block preview, the Last posted line, **Copy block**, **Save as ENEX file...** and **Setup steps**.
- Each release now opens with a "Which file do I need?" table listing every download, written by the release workflow from the same file list as `SHA256SUMS.txt`; [Installing Casebook](docs/install.md#which-file-do-i-need) carries the same table.

### Changed

- Windows portable zips are now named `-portable.zip` (`Casebook-<version>-windows-<arch>-portable.zip`); the installer keeps `-installer.exe`, and the `casebook-cli-*` archives keep their names.
- Privacy: Casebook makes no network connections and holds no credentials, with no exception. Evernote data moves through your AI client like any other MCP data ([Privacy and local data](docs/privacy.md)).
- A post can now be recorded for an AI actor, only through `casebook_confirm_note_posted`; the activity entry keeps the actor kind and client name.
- Database schema 7 (migration 0007 lets the stored note id be any id of 1 to 256 characters that Evernote's MCP tools return). Casebook backs up the database before upgrading it; 0.2.0 refuses to open a schema 7 database.

### Removed

- Casebook's own connection to Evernote: the Thrift client, OAuth sign-in, the token file, `CASEBOOK_EVERNOTE_TOKEN` and `CASEBOOK_EVERNOTE_HOST`, the local stub server, `casebook evernote connect`, `disconnect`, `post` and network `capture`, doctor's Evernote line, and the desktop app's Connect, Disconnect and Post actions. ENEX export and capture stay.
- If you ran a 0.2.0 build, delete `evernote-token.json` from the data folder and remove Casebook from the applications that can access your Evernote account ([Privacy and local data](docs/privacy.md#evernote-notes)).

### Known limits

- Not yet tested against Evernote's server: nobody has posted through Evernote's MCP server with a real account, and the note body format is undocumented. Evernote's MCP server is in beta and needs a paid Evernote plan (personal, Business or Teams). Through MCP a note can hold about 28 KiB of text, because the body travels twice in one 64 KiB tool response; longer notes take **Copy block**.

## 0.2.0 (2026-10-09)

Published on 2026-10-09 and superseded the same day by 0.3.0, which removed its direct Evernote connection. See the 0.3.0 notes if you ran it.

### Added

- Evernote notes ([ADR 0019](docs/decisions/0019-evernote-notes.md), [Evernote notes](docs/evernote.md)): post the current state of a case into an Evernote note you choose, between `BEGIN CASEBOOK` and `END CASEBOOK` lines that Casebook alone replaces, with every claim and hypothesis labelled Human verified, Source verified, AI generated or Disputed. The rest of the note is sent back unchanged; a hand-edited block is refused unless you pass `--overwrite-block` or confirm in the desktop app.
- Capture an Evernote note as evidence (new source type `evernote_note`), and ENEX files for both directions with no network at all.
- `casebook evernote connect`, `disconnect`, `status`, `preview`, `post` and `capture`; the desktop app's **Post to Evernote...** panel, **Connect Evernote...** and **Capture Evernote note...**.
- The `casebook_get_note_block` MCP tool returns the block for an AI client to preview; no MCP tool can post.
- `casebook doctor` reports whether Evernote is connected, never the token.

### Changed

- Privacy: Casebook still makes no network connections unless you connect Evernote; after that, only the `casebook evernote` commands and the desktop's Evernote panels connect to evernote.com, only while you post or capture. The MCP server, the hooks and `casebook doctor` never connect ([Privacy and local data](docs/privacy.md)).
- Database schema 6 (migration 0006 adds the note link table and the `evernote_note` source type). Casebook backs up the database before upgrading it; 0.1.2 refuses to open a schema 6 database.
- The JSON context pack lists `verification` on disproven hypotheses.

### Known limits

- Not yet tested against a real Evernote account. Signing in through Evernote needs a consumer key that Evernote must grant to Casebook; until then `connect --token` accepts a token you obtained yourself, and ENEX works offline.

## 0.1.2 (2026-10-08)

The first published release. The tags `v0.1.0` and `v0.1.1` were internal builds and were never published.

### Added

- One local engine (`internal/app`) over a SQLite database with full-text search, shared by the desktop app, the `casebook` CLI and the MCP server.
- Investigations with evidence (and where it came from), claims, hypotheses, questions and a conclusion, each with confidence, rationale and verification; evidence links as supports or contradicts.
- Deterministic context packs in Markdown, compact Markdown and JSON (`casebook context`, `casebook_get_context`, **Copy context**), framed as data for the AI. Disproven hypotheses list the evidence that contradicted them (`Contradicted by: E-003`), and that evidence stays in the index.
- MCP server (`casebook mcp`) with tools to read and write cases and save a batch of findings with `casebook_checkpoint`; no delete tool.
- Only a person can mark a record human verified; an AI edit to a verified record resets it to AI generated ([ADR 0014](docs/decisions/0014-only-people-verify.md)).
- Activity history with the actor kind and client of every change.
- `casebook setup claude` (`--hooks`, `--scope marked`, `--skill`) and `casebook setup codex` (`--skill`), each backing up the file it changes; the skill adds `/casebook` to Claude Code and teaches both clients when to offer a case and to ask before opening one.
- The session start hook prints a one-line hint when no case is active; `casebook setup claude --hooks --quiet-when-inactive` turns it off ([ADR 0018](docs/decisions/0018-session-start-hint.md)).
- `casebook setup mcp` prints the configuration for any MCP client (`--client`, `--list`, `--wsl`) and never writes files ([ADR 0017](docs/decisions/0017-setup-mcp-prints-only.md)).
- `casebook setup instructions` prints a section for a project's CLAUDE.md or AGENTS.md.
- Per-repository active investigation: `casebook investigation activate <ref> --here` and `casebook investigation dirs` ([ADR 0015](docs/decisions/0015-per-directory-active-investigation.md)).
- `casebook report` (`-o`, `--timeline`, `--json`), the `casebook_get_report` tool and **Copy report** in the desktop app: a Markdown write-up with the conclusion, claims, what was ruled out and a timeline.
- Stable export for sharing a case through a repository (`casebook export <ref> --out .casebook/<ref>.json`); import always adds a separate copy ([ADR 0016](docs/decisions/0016-stable-export-for-repo-sharing.md), [Sharing a case](docs/sharing.md)).
- `casebook backup` and `casebook restore`, and a pre-upgrade backup before every schema migration.
- `casebook doctor` diagnostics and `casebook status`.
- Desktop app for Windows (portable zip and installer) and macOS (universal DMG), with light and dark themes.
- CLI-only release archives for Linux, macOS and Windows; install with `scripts/install.sh`, Homebrew (`brew install jameskbb/tap/casebook`) or `go install`. Release assets include `THIRD_PARTY_NOTICES`.
- An agent eval harness (`scripts/agent-eval`) that scores how Claude Code and Codex use Casebook in headless sessions.
- MIT license.
