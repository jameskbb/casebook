# 0024: Portable output instead of an Evernote integration

- Status: Accepted
- Date: 2026-10-27
- Supersedes: [ADR 0019](0019-evernote-notes.md) and [ADR 0020](0020-evernote-through-client-mcp.md), except their compatibility facts: schema versions 6 and 7 (the `evernote_links` table and its widened note id) and the `evernote_note` evidence source type
- Unchanged: [ADR 0022](0022-check-for-updates.md) (the check for updates a person starts remains Casebook's one network request)

## Context

Casebook put a case into an Evernote note twice. ADR 0019 connected to Evernote's classic API with an OAuth token and was withdrawn the same day. ADR 0020 moved the network side to the person's AI client, connected to Evernote's MCP server, while Casebook kept the Evernote-specific parts: a labelled block with delimiter lines, a splicer for Markdown, HTML and ENML bodies, a fingerprint, a prepare and confirm pair of MCP tools, a publication ledger (`evernote_links` and "Last posted"), a notebook preference, ENEX import and export, a setup guide, a desktop panel and `casebook evernote` commands.

That second shape kept Casebook off the network but not out of the integration business:

- It was never tested against Evernote's server. Evernote does not document the body format its MCP tools use, so the splice and the fingerprint were built for three guessed formats.
- The ledger made Casebook claim knowledge of remote state ("Last posted") that it could only infer from a body an AI client handed it.
- It served one vendor, and only paid plans of that vendor, while the people Casebook is for keep notes in many apps. Every other app would have asked for the same machinery again.
- Meanwhile **Share summary** and `casebook report` already produce deterministic, labelled text that reads correctly anywhere, and the person's AI client can already reach whatever apps they have connected to it.

## Decision

Casebook produces trustworthy, portable content and stops there. It removes its Evernote-specific integration and relies on two paths that already exist:

1. **The person moves the text.** The **Share summary** dialog (leadership brief or complete report; Copy; Save as file), `casebook report` (Markdown, `-o FILE` to save) and the new `casebook report --brief` (the leadership brief with Casebook's default selection).
2. **The person's AI client moves the text.** The MCP tool `casebook_get_report` gains an input `mode` (`report`, the default, or `brief`). A person asks their AI client (Claude Code, Codex, Claude Desktop, Cursor or any MCP client) to save the brief or report to an app that client is already connected to.

Removed: the desktop Evernote block panel, its setup steps and notebook field, the "Last posted" line; the `casebook evernote` commands; the MCP tools `casebook_get_note_block`, `casebook_prepare_note_update` and `casebook_confirm_note_posted`; ENEX import and export and ENML support; the packages `internal/evernote` and `internal/noteblock`; the Evernote guidance in the MCP server instructions and the generated skill; the screenshots and guide for the feature. The hidden `casebook evernote` command (any subcommand) now prints one paragraph pointing to the replacements and exits non-zero, so a script that used it fails loudly instead of silently doing nothing. The MCP tools are removed without stubs.

The desktop **Share summary** dialog adds a short paragraph ("Use this summary in the tools you already use. Copy it into a note or document, save it as a file, or ask your AI to save it to an app it is already connected to."), an example request built from the open case, and a **Copy example request** button that copies it. The example reads, for the featured case: Save the leadership brief for case INV-0002 in my connected notes app as a new note titled "Northeast margin review". Keep its verification labels, evidence references and caveats.

## Responsibilities

- **Casebook** renders the brief and the report deterministically, with verification labels, evidence references and omission notices intact, and hands the text to the person (copy, save) or to an AI client (`casebook_get_report`). It connects to no app, holds no credential for one, tracks no remote state, keeps no record of where text went and never says an external note was created or updated.
- **The AI client** discovers and invokes the tools actually present in its session. It fetches the text from Casebook and writes it with the destination app's tools, under its own permission prompts. It reports the actual result of the app's tool to the person. A successful write call and a readback of the destination are different checks; the client says which one it did. To update an existing note it reads the destination first, keeps unrelated content and uses the destination's real update capability. If no writable tool for that app is in the session, it says so, and copy or save is the path.
- **The destination app** creates or updates the note or document through its own tools, under the person's sign-in and the app's terms.

A brief or report an AI client fetches is its own snapshot, and a brief fetched this way uses the default selection of findings. It matches a preview the person reviewed in the desktop app only if the case has not changed in between and the person kept that selection. Copy and Save in the dialog keep using the exact reviewed bytes, as before ([Reviewed sharing snapshots](../api.md#reviewed-sharing-snapshots)).

## What is not built

No per-app adapters, no connector registry, no OAuth, keys or tokens, no Casebook acting as an MCP client of another server, no background publishing, no retry queues, no remote-state tracking or publication ledger, no approval system of Casebook's own and no automatic writes. A request to add any of these for a specific app is answered by this record: the person's AI client already does that job, under the person's own permissions.

## Compatibility

- The `evernote_note` source type and its "Evernote note" label stay valid and readable. Existing evidence is never relabelled. The value stays in the MCP schema's list of valid source types; descriptions no longer suggest it. Evidence capture no longer strips Casebook blocks from note text.
- Migrations 0006 and 0007 stay, forward-only. The dormant `evernote_links` table stays in the schema; backup and restore still carry it, as they carry every table. Nothing reads or writes it.
- The `evernote_notebook` setting row stays where it was written; nothing reads it.
- Old History rows for `evernote.post` and `evernote.unlink` still render. Nothing writes new ones.
- The stable case export ([ADR 0016](0016-stable-export-for-repo-sharing.md)) never included Evernote links, so the export format and its version are unchanged.
- ENEX files a person saved earlier stay on disk, untouched; Casebook no longer reads them.
- The 0.2.0 advice in [Privacy and local data](../privacy.md) about deleting a leftover `evernote-token.json` stays.
- [Evernote notes](../evernote.md) remains as a short retirement note so existing links resolve.

## Consequences

- Casebook works with any note-taking or document app the person or their AI client can reach, without code for any of them, and the privacy statement gets simpler: no connector, no ledger, no remote state.
- Casebook can no longer show that a case was posted, or detect a hand-edited block in a note. Those claims were inferred from bodies an AI client supplied and were never verified against a real Evernote account. The person relies on their AI client's report of the tool result, and on reading the destination.
- Whether a particular client saves the brief verbatim depends on that client and the app's tools. No Evernote account, developer approval or live external write was used to validate this change; local tests and fixtures do not establish compatibility with any particular app.
- People who used `casebook evernote` or the note tools move to `casebook report` (with `--brief` or `-o`), `casebook_get_report` with `mode`, and **Share summary**. [Share a case](../sharing.md#if-you-used-evernote-notes) has the steps.
- "Casebook is never the thing that acts in the world" holds without an Evernote-shaped exception.
