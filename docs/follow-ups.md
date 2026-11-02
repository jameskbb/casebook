# Follow-ups

Non-blocking items from the readiness review of the first build (2026-10-05) and from the security audit and its follow-up pass (2026-10-06). None of them blocks a first release candidate.

## Packaging and release

- The release workflow bundles `THIRD_PARTY_NOTICES.txt` (with `LICENSE`) in the portable zips, the CLI archives and `Casebook.app` (`Contents/Resources`), but the Windows NSIS installer (`packaging/windows/project.nsi`) still installs only `Casebook.exe` and `bin\casebook.exe`; add both files to the installer ([ADR 0011](decisions/0011-license.md)).
- A signed, notarized macOS build is only provable in CI or on a Mac; confirm on the first tagged release. (The race detector now runs and passes in CI.)

## CLI and hooks

- `casebook backup` (like the other data commands) creates an empty database when none exists, so it can write a backup of nothing; it should report that there is no database to back up instead.
- On Windows, `casebook setup claude --hooks` writes exec-form hook entries, which need Claude Code 2.1.139 or later; an older Claude Code ignores `args` and runs bare `casebook.exe`. Drop this note once that version is well behind most users.
- The Stop hook's per-session checkpoint gate relies on `CLAUDE_CODE_SESSION_ID` in the MCP server's environment (Claude Code 2.1.154 or later, and listed in `WSLENV` for a WSL client running the Windows `casebook.exe`). Without it, a session that did checkpoint may still get one reminder.

## Agent behavior (from `scripts/agent-eval`, 2026-10-06)

- The Stop reminder waits 10 minutes from session start. Every eval session finished in under 6 minutes, so the reminder never fired; agents that checkpointed did so because the MCP instructions or the skill asked. Consider a shorter gate for sessions that already wrote to the active case.
- Continuation sessions still re-check settled points now and then (one Opus and one Sonnet run re-derived the hour-of-day spread that had disproven the midnight theory). The compact pack now prints a "Contradicted by" line with the evidence refs under each disproven hypothesis; rerun the eval to see whether that stops the re-checks.
- Haiku opened a case without asking in 1 of 1 runs, despite the instructions; Sonnet, Opus and Codex asked first.

## Portable output ([ADR 0024](decisions/0024-portable-output-instead-of-evernote.md), 2026-10-27)

The Evernote notes items listed here before (the real body format, rate limits, setup text, fingerprint tolerance, ENEX DTD copies, "Last posted" in status) were closed as no longer applicable when the feature was removed.

- Nobody has yet asked an AI client to save a brief into a real connected notes or document app with the example request from **Share summary**. Try it by hand with at least one client and one app, never in CI, and note in [Using a case summary in other apps](sharing.md#using-a-case-summary-in-other-apps) anything that reads differently in practice (for example a client that summarizes the brief instead of saving it verbatim).
- The dormant `evernote_links` table and the `evernote_notebook` setting stay in databases that used the retired feature. Decide whether a later forward-only migration should drop them, or whether they stay for good as harmless history.

## Plain reader, round 1 (2026-10-13; [ADR 0021](decisions/0021-case-is-the-word.md))

- Error texts from the engine that reach a person (the CLI's no-active-case error, the desktop's conflict messages) still say investigation; they are shared with MCP clients, so changing them is a wording decision for every surface at once.
- The MCP tools' `Title` fields still say investigation ("Get the active investigation"); some clients show titles in approval prompts. ADR 0021 keeps program-facing names, so decide whether titles count.
- The Share summary timeline names actors as the engine stores them ("ai codex", "human casebook-ui"), while History says "Codex (AI)"; map the actor the same way.
- Ruling a hypothesis out now sits behind More options in the claim drawer; consider a visible Rule out button.
- The frontend copy audit (`desktop/frontend/src/copy.test.ts`) checks string literals only: JSX text between tags and multi-line template literals escape it.

## Editorial review pass (2026-10-26, merged 2026-10-27)

Deferred from the 2026-10-26 review pass:

- Name the changed claim in the conclusion's "changed since this review" notice. The review snapshot is one SHA-256 of the whole basis (migration 0008), so this needs a new forward-only migration.
- A business scenario (a margin or churn question) in `scripts/agent-eval`; today it measures engineering scenarios only.
- Verifier identity or date on each record and on the conclusion; touches the single-user model and the schema.
- `case` as the primary CLI command name with `investigation` as the alias (a change to the wording of [ADR 0021](decisions/0021-case-is-the-word.md)).
- Friendly actor names in report and brief timelines; MCP tool Titles that say case.
- A per-record "changed since you checked it" marker in the reading column.
- The complete report still heads its open hypotheses "Hypotheses still open or supported" while the brief says "Explanations still open or supported"; renaming the report heading changes its golden.
- Small layout items from the acceptance pass: the header title collapses to "IN" at 960 px wide; the sidebar card title is squeezed beside the "Your AI's current case" badge; widen the labels dialog; Connect dialog sticky subheader spacing; the Connect status wording "Uses a different command-line tool"; the Review findings position count and the "Current" chip in review; the conclusion changed-notice wording after a verification raise.
- MCP handles tool calls concurrently and a result's `changed` list comes from a before-read outside the transaction, so under a concurrent checkpoint a result can list `verification` without the reset sentence. The `verification_reset` flag itself is computed inside the transaction and is correct.
