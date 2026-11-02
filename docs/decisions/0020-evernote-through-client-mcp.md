# 0020: Evernote through the client's MCP connection

- Status: Superseded by [ADR 0024](0024-portable-output-instead-of-evernote.md) as of 2026-10-27; Casebook no longer has an Evernote integration. Its compatibility facts (schema versions 6 and 7, the `evernote_note` source type) stand.
- Date: 2026-10-09
- Supersedes: the mechanism of [ADR 0019](0019-evernote-notes.md) (its network, credential and posting sections). The product shape of 0019 stands.

## Context

[ADR 0019](0019-evernote-notes.md) built Evernote notes on Evernote's classic API: a Thrift client inside Casebook, OAuth 1.0a with a consumer key Evernote had to grant, a token file next to the database, and the one exception to "Casebook makes no network connections". It shipped as v0.2.0 and was superseded the same day.

Evernote's developer site (dev.evernote.com, read on 2026-10-09) changed the ground under that design:

- The classic Evernote Developer API (EDAM) and its SDKs are marked deprecated and no longer actively developed.
- Evernote ships an official remote MCP server at `https://mcp.evernote.com/mcp`. It signs in with OAuth and Dynamic Client Registration: there are no keys or tokens to manage, and nobody creates an Evernote app or pastes credentials.
- The MCP server is in beta ("Tools and supported clients may change"), is available on paid personal, Business and Teams plans only (Free, or Basic, accounts are not included), and rate-limits tool calls per minute, per tool and across all tools.
- Its tools include `get_note` ("Fetch a note's full content, tags, and attachments"), `edit_note` ("Update a note's title or body"), `create_note` (optionally in a notebook), `search_notes` and `search_notebooks`. No page says in which format `get_note` returns a note's body or `edit_note` accepts one.
- Evernote advises: "Treat write actions (create, edit, delete) as you would any automation: review before confirming."

The people who would post a case block into a note already work through an AI client that speaks MCP. That client can connect to Evernote's server itself.

## Decision

Casebook never connects to Evernote. The person's AI client, connected to Evernote's MCP server, carries the note in and out. Casebook does the part that needs its records: it builds the labelled block, splices it into the note's body, and verifies the result before recording a post. Casebook makes no network connections again, with no exception, and holds no credential of any kind.

The posting flow an agent follows, in order:

1. `get_note` (Evernote) for the note the person named. For a new note, `create_note` with the block from `casebook_get_note_block`, in the person's preferred notebook when one is set.
2. `casebook_prepare_note_update` (Casebook) with the note's id, title and body exactly as returned. Casebook finds its block for the case (or appends one at the end) and returns the complete `new_body`, or refuses: a hand-edited block, broken or doubled `BEGIN CASEBOOK` and `END CASEBOOK` lines, or a body over the size cap. It writes nothing.
3. `edit_note` (Evernote) with `new_body` passed verbatim as the whole body. `edit_note` replaces the whole body, so anything else risks the person's own text.
4. `get_note` again, then `casebook_confirm_note_posted` with the body read back. Casebook checks that its block for the case appears exactly once and matches the current block's fingerprint, and only then records the link (the "Last posted" line) and one activity entry.

The tool descriptions, the MCP server instructions and the `/casebook` skill each carry these steps and the rules that go with them: pass `new_body` verbatim; never edit a person's note except with `new_body` from prepare; never say a post is done before confirm succeeds; before the first post in a session, tell the person which note will change and that their client will ask them to approve the write. The outputs of the three note tools are not wrapped in Casebook's `BEGIN CASEBOOK DATA` frame, because their bodies are passed verbatim to another tool and a frame would be copied into the note.

The block, the delimiter lines, the hand-edit rule and "Casebook replaces only its block" are unchanged from 0019. Because the body format is unknown, `internal/noteblock` (a pure package with no network, store or engine imports) detects and splices Markdown, HTML and ENML, keeps every byte outside the block, and computes a fingerprint per format that ignores presentation but counts any change to a word, line, structure or checkbox. Golden files and fuzz tests cover each format. An explicit `format` argument wins over detection.

Two additions ship with the mechanism:

- `casebook evernote setup` prints the steps for Claude Code, Claude Desktop, Codex and other clients, the paid-plan requirement, the beta status and the notebook setting. It prints only, like `casebook setup mcp` ([ADR 0017](0017-setup-mcp-prints-only.md)). One function in `internal/app` produces that text for the CLI, the desktop app's **Setup steps** dialog and [Evernote notes](../evernote.md); a test checks that the guide contains it verbatim.
- A preferred notebook for new notes: the setting `evernote_notebook`, set by a person with `casebook evernote notebook` or the desktop field "Notebook for new notes". The two note tools return it as a hint for `create_note`. No MCP tool sets it, it is not recorded in activity and it is not exported.

## Trust boundary

- The person's approval of `edit_note` in their own AI client is the act of posting. Evernote tells people to review writes before confirming; Casebook relies on the same gate rather than adding a second one.
- Recording requires verification. `ConfirmNotePosted` is the only path to `RecordNotePosted`, and it records nothing unless the block for the case appears exactly once in the body read back and its fingerprint matches. There is no bare "record posted" tool over MCP.
- AI actors may now record a post, through confirm only. The activity entry keeps the actor kind and the client name, so history shows, for example, that Claude Code (AI) posted INV-0002 to the note 'Checkout flake'. This does not weaken [ADR 0014](0014-only-people-verify.md): a verified splice says where the block went, not that any claim in it is true, and every claim keeps its own verification label.
- An agent can skip prepare and edit a note freehand. Casebook cannot prevent that. It refuses to call such an edit a post: confirm fails when the block does not match, and nothing is recorded. The docs say so plainly.
- Confirm refuses a missing, doubled or different block alike for a person and for an AI.

## What was removed, and why there is no flag

Removed outright: the Thrift codec (`internal/evernote/edam`), the OAuth flow, the token file and its storage code, the network post and capture, note-link parsing, the loopback stub server and `scripts/evernote-stub`, every `CASEBOOK_EVERNOTE_*` variable, doctor's Evernote line, the CLI commands `casebook evernote connect`, `disconnect`, `post` and network `capture`, and the desktop app's Connect, Disconnect and Post actions. `internal/evernote` keeps only the offline ENEX code. Outside tests, no Go file imports a network package other than `net/url`, and the boundary test enforces that across the module.

No feature flag keeps the old path. A flag would keep network and credential code in the binary that serves `casebook mcp` and keep the privacy promise conditional; git history keeps the code at no cost.

## Alternatives rejected

- **Casebook as its own MCP client of Evernote's server.** It would put a network path, a token store and a beta sign-in flow back into the binary that serves `casebook mcp`, duplicate what the person's client already does, and make people sign in to Evernote twice.
- **Keep the Thrift API.** It is deprecated, no consumer key was ever granted, and its credential model is one Evernote has abandoned.
- **Both.** Twice the surface for half the testing each.

## Consequences

- "No network, no credentials" is unconditional again ([Privacy and local data](../privacy.md)). Evernote data moves through the person's AI client, under that client's and Evernote's terms, like any other MCP data.
- Posting needs an AI client connected to Evernote's MCP server and a paid Evernote plan. People who use only the desktop app or the CLI copy the block (**Copy block**) or save it as an ENEX file and paste or import it themselves.
- The body format is not documented, and nothing has been tested against Evernote's server: no test, demo or screenshot uses a real account. The splice ships with three formats and goldens, and the docs say plainly that real-server behavior is unconfirmed. Beta changes to tool names or behavior land on the agent instructions and the splice layer alone.
- Rate limits belong to the client. Casebook adds no retry or queue.
- `casebook setup claude --evernote` was considered and refused: [ADR 0009](0009-conservative-ai-client-setup.md) lets setup merge only Casebook's own entries, an `evernote` entry is not Casebook's, and the saving is one paste.
- v0.2.0 stays tagged and published as history; this ships as 0.3.0 the same day. Anyone who ran a 0.2.0 build removes `evernote-token.json` and revokes Casebook's access in their Evernote account.
- The `evernote_links` table (migration 0006) stays. Its note id column now holds whatever id Evernote's `get_note` returns.
