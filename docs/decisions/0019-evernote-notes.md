# 0019: Evernote notes: post a labelled case block, capture a note as evidence

- Status: Superseded by [ADR 0024](0024-portable-output-instead-of-evernote.md) as of 2026-10-27; Casebook no longer has an Evernote integration. Before that: superseded in part by [ADR 0020](0020-evernote-through-client-mcp.md): the mechanism (Thrift, OAuth, the token, the network path and who posts) is replaced; the product shape (the labelled block, its delimiter lines, splicing only the block, the hand-edit rule, capture, ENEX and the link table) stands.
- Date: 2026-10-09

> This record describes the v0.2.0 design, which shipped as v0.2.0 and was superseded the same day by 0.3.0. Casebook no longer connects to Evernote or holds any credential; read [ADR 0020](0020-evernote-through-client-mcp.md) for how posting works now.

## Context

People keep the human side of an investigation in a notebook, often Evernote, while the evidence and claims their AI clients collect live in Casebook. The request was "AI insights written back into notes". Taken literally that strains Casebook in three places:

- Casebook has no insights, only records with a verification state. Text dumped into a note without saying who checked it is exactly what Casebook exists to avoid.
- Casebook made no network connections and held no credentials ([Privacy and local data](../privacy.md)).
- Casebook has no schedulers or sync; nothing happens unless a person or an AI client acts.

## Decision

Build "Evernote notes" in a narrow shape: a person posts the current state of a case into the Evernote note they choose, every claim marked by who checked it, in a block Casebook owns and lets nothing else touch. A note can also be captured as one evidence record.

The case block:

- One deterministic renderer, `contextpack.RenderNoteBlock`, builds it from the same pack as the context pack, without a size budget: the heading `Casebook case INV-0002: <title>` and status; Conclusion with confidence and rationale; Established claims; Disputed claims; Open questions; Not yet established (open and supported hypotheses); Ruled out (disproven hypotheses with the evidence that contradicted them). Evidence appears as refs and titles only; raw detail, metadata, recent changes and activity never do.
- Every claim and hypothesis line ends with its verification label (AI generated, Human verified, Source verified, Disputed). The block opens with "Posted by Casebook from INV-0002. Lines marked AI generated have not been checked by a person. Edit this note freely outside the block; Casebook replaces only the block." and ends with "Casebook case INV-0002".
- ENML forbids `id`, `class`, `data` and `on*` attributes, so the block has no markup identity. It is delimited by text: single-line divs `BEGIN CASEBOOK INV-0002` and `END CASEBOOK INV-0002`, visible on purpose. The block uses only `div`, `h2`, `h3`, `ul`, `li`, `b` and `i`, no attributes at all, and XML-escapes every value. A delimiter line is a line whose whole text (normalized whitespace, inline formatting allowed, case-sensitive) is exactly `BEGIN CASEBOOK <ref>` or `END CASEBOOK <ref>`; a stored text that would form such a line on its own gets a hyphen for its first space (`END-CASEBOOK INV-0002`), so it can never be taken for a delimiter. Text that merely contains those words is left alone.
- The block holds no time, so the same records give the same bytes (`NoteBlock.Hash` is their SHA-256). What Casebook remembers after a post is the block's fingerprint (`evernote.Fingerprint`: a SHA-256 of its element names and normalized text), which an Evernote client re-serializing the block, for example adding a style attribute or reflowing whitespace, does not change, while any edit to a word, label or line does.

Write-back safety:

- Fetch the note immediately before writing, find exactly one begin line and its end line, replace that span inclusive, and send every other byte back unchanged, including `en-media` references. Resources are omitted from `updateNote`, which Evernote documents as leaving them unchanged. With no block yet, append at the end, never the top. Zero begins with an end, two begins, or a begin without an end: refuse, name the problem, change nothing.
- Same block already in the note: no update call at all.
- A new table, `evernote_links` (migration 0006: investigation, note GUID, title, last block hash, last update sequence number, last posted time), remembers what was written. A block in the note whose fingerprint differs from the remembered one was edited by hand and is refused unless the person passes `--overwrite-block` (or confirms in the desktop app).
- The note's `updateSequenceNum` is re-read just before `updateNote`; if it moved, one retry from fresh content, then refusal.
- `contentClass` is never set (a client that does not know the class must treat the note as read-only, which would lock the person out of their own note). `sourceApplication` is set to `Casebook` only on notes Casebook creates.

Who acts:

- Only a person posts, through the CLI or the desktop app (`human` actors). `app.RecordNotePosted` refuses any other actor. The MCP server gets one read-only tool, `casebook_get_note_block`, which returns the block as framed data: an AI can show, a person decides ([ADR 0014](0014-only-people-verify.md)); the surface an AI drives prints, it does not write ([ADR 0017](0017-setup-mcp-prints-only.md)). This rests on the CLI and the desktop acting for the person: an AI agent with shell access can run `casebook evernote post` and is then recorded as a person. Casebook does not prevent that; the MCP server simply offers no posting tool.
- Capture uses the existing evidence path with a new source type, `evernote_note`: `source_ref` the note link, `source_label` the title, raw detail the note's text with markup stripped and cut at the existing 64 KiB limit (saying so), metadata the GUID, update sequence number and update time. The person or AI writes the summary; Casebook never summarizes. An AI that already has the text uses `casebook_add_evidence`; Casebook fetches nothing for it.

Network and credentials:

- Off by default: no connection is made until the person connects Evernote, and then only while they post or capture a note.
- All network and Thrift code lives in one package, `internal/evernote`. `TestPackageBoundaries` lets only `internal/cli`, `desktop` and `scripts` import it, so (although `cmd/casebook` links it into the same binary that serves `casebook mcp`) `internal/app`, `internal/mcpserver`, the hooks in `internal/integrations` and `internal/doctor` cannot reach it. The engine stays network-free: it renders the block, records links and activity, and accepts already-fetched text.
- Thrift: a small hand-written binary protocol codec, no new module dependency, with field ids pinned to the published Evernote IDL.
- Auth: Evernote no longer issues developer tokens except for proven necessity, so `connect` uses OAuth 1.0a with a loopback callback on 127.0.0.1. The consumer key and secret come from Evernote, are injected at build time and are never in the source. A build without them says so and accepts `connect --token` with a token the person obtained themselves.
- Token storage: `CASEBOOK_EVERNOTE_TOKEN` wins, for scripting. Otherwise the token lives in `evernote-token.json` next to the database, created owner-only (mode 0600; its folder is 0700 when Casebook creates it, and an existing folder chosen with `CASEBOOK_DB` keeps its mode); on Windows it is also encrypted for the current user with DPAPI. The OS keychain on macOS is a follow-up. The token is never in the database, exports, backups, doctor output, logs or error messages. `disconnect` deletes the file.
- Host: `www.evernote.com` over HTTPS. `CASEBOOK_EVERNOTE_HOST` exists only for the local test stub and is refused unless it names a loopback address; plain `http` is accepted only for loopback.
- Rate limits: on `RATE_LIMIT_REACHED`, show the wait Evernote returns and stop. No retry loop, no queue.
- ENEX, fully offline, ships too: `preview --enex FILE` writes a one-note export the person imports by hand, and `capture --enex FILE` reads a note from an exported file. If Evernote never grants a key, this is the feature and the no-network promise holds without exception.

No schedulers: posting and capturing happen only when a person acts. No watch, webhook, timer, or post on checkpoint or verify. The only "state" shown is "Last posted" from the link table.

Storage: the link is local state. Backups copy the whole database, so they carry it, and a restore brings it back; the stable investigation export ([ADR 0016](0016-stable-export-for-repo-sharing.md)) leaves it out, so an imported copy is never linked to someone else's note. Posting and unlinking log `evernote.post` and `evernote.unlink`, which, like `checkpoint`, are not material changes: they never appear in a context pack's recent changes or a report's timeline.

## Non-goals

Two-way sync or merge; notebook or tag import; editing a person's text; summarizing notes; creating or changing tags, notebooks, reminders or attachments; `contentClass`; the Yinxiang host; other note targets (the renderer is target-neutral, which is enough); background work; any MCP tool that writes to Evernote; more than one block per note or one note per case; a Linux desktop panel.

## Consequences

- Casebook now has one opt-in network path and one credential. [Privacy and local data](../privacy.md) states the exception and where the token lives.
- The feature depends on Evernote granting a consumer key, which can take days or never happen; the ENEX path works without it. Nothing in this repository has been tested against a real Evernote account: every test, demo and screenshot runs against a loopback stub that speaks the same Thrift protocol.
- The API is legacy and Evernote's sandbox is deprecated; it may change under us. The fetch-to-write window that remains after the update sequence number check is milliseconds and accepted.
- A consumer secret shipped in a binary can be extracted; that is Evernote's own client model, and why its rate limits are per key.
- Some people will dislike the visible delimiter lines. They are the honesty mechanism and stay.
