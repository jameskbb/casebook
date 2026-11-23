# Share a question

There are two ways to share a question. **Share the answer** writes the answer up for someone who needs to read it; they do not need Casebook. Copy or save it, or ask your AI client to save it in a notes or document app you already use ([Using a shared answer in other apps](#using-a-shared-answer-in-other-apps), at the end of this page). An export file moves the question itself: to your own other computer, or as a copy for someone who wants to open it in their Casebook. Casebook is for one person, so a copy never stays linked to yours.

## Write the answer up with Share the answer

Open a question that has an answer and choose **Share the answer**. Under **How much to share**, pick **Short version** for the answer and the findings behind it, or **Everything** for all of the question, including its history. The same text comes from `casebook report` (everything) and from the `casebook_get_report` tool your AI can call; `casebook report --brief` and `casebook_get_report` with `mode` `brief` give the short version.

The short version always carries the findings where sources disagree, the sources that go against a finding and the findings behind the answer, and a few of the rest.

The reader of a shared answer is not you, so it never says "You". A finding is labelled from its sources: `2 of 2 sources checked by a person`, `No source yet`, or `A person decided this holds` when a source goes against it and you decided it still holds. A source you checked reads `A person checked this`, and the count under the answer reads, for example, `2 of 9 sources checked by a person. 1 checked by AI, 6 not checked.` The answer is followed by `A person checked this answer on November 1, 2026.` or `Nobody has checked this answer yet.`

A shared answer has no record codes: the title is the question, and findings and sources are named by their own text. Dates are written out (`October 30, 2026`). A section with nothing in it is left out, and each finding appears once. The sections, in this order:

- The question as the title, then when it was asked and last changed.
- `## Background`: what you wrote under the question.
- `## Answer`: the answer, the count of sources a person checked, who checked the answer and what they checked, and `How we know:`. `No answer yet.` when there is none.
- `## Findings`: each finding with its label, `Why we think so:`, and its sources by name: `Backs it: ...` and `Goes against it: ...`, each with its own label.
- `## Where sources disagree`: findings a source goes against, until someone decides, and findings your AI marked that way.
- `## Still to find out`: findings not tested yet, then each thing to find out marked `not answered yet`, `answered` or `left for later`, with the answer or why it can wait.
- `## Ruled out`: findings that were ruled out, with the sources that go against them.
- `## Replaced`: findings a newer finding took the place of.
- `## Sources`: each source once, with its label, its name, where to find it and what it shows. Something you saw yourself reads `Seen by the person who shared this`. Everything also gives the date of each source, once.
- `## How we got here` (Everything only): the history of changes, oldest first, one heading per day.

The short version ends by saying what it leaves out, for example `This short version leaves out 1 finding, 2 things still to find out and the history. Ask the person who shared this for the version with everything.` It keeps every thing to find out that has an answer, with the answer. Both versions end with `Made with Casebook. These are the saved findings and sources, word for word.` Casebook builds the text from what is saved without asking a model to write anything, so the same question gives the same text.

**Copy** and **Save** use what you see in the window. **Copy** puts the answer on the clipboard twice over: as formatted text, so an email or a document shows the headings and lists, and as plain text with no `#` or `_` marks, for a chat box or any plain field. **Save** writes a web page (`.html`) that opens in any browser when the reader double-clicks it. The file is named after the question, for example `Why did calls go up 30 percent in October when sales stayed the same.html`, with ` (everything)` added for the full version.

## Move a question or hand over a copy

To open the question in another Casebook, export it to a file and import that file on the other computer. You might move your own question to a new laptop, keep it beside a project in a Git repository, or give a colleague a copy to read and hand to their own AI. The file format is described in the [engine API reference](api.md#15-export-format-v1-and-import).

## Before you hand over a file

An export holds everything in the question: titles, summaries, the full source text, metadata and the activity log. Read it before you hand it over or commit it, and never share a question whose sources hold secrets, tokens, customer data or anything else the recipient should not see. Anyone who can read the file, or the repository it is committed to, can read the question. See [Privacy and local data](privacy.md#backups-export-and-restore).

## Export the question

To write the file into a Git repository, run this from the repository root (any other folder works the same way without the `git` lines):

```sh
mkdir -p .casebook
casebook export INV-0002 --out .casebook/INV-0002.json
git add .casebook/INV-0002.json
git commit -m "Add the Casebook question on October calls"
```

`--out` (or `-o`) never overwrites an existing file. To refresh the file after more work, add `--force`:

```sh
casebook export INV-0002 --out .casebook/INV-0002.json --force
```

The file is written to diff well:

- It is indented JSON with a fixed key order. Records are sorted by ref (`E-001`, `E-002`, ...), links by record and then source ref, and activity in the order it happened.
- Exporting an unchanged question gives the same bytes, so `git status` stays clean until something real changes. `exported_at` is the time of the newest change in the file, not the time you ran the command.
- Which question is active on your machine is left out (`active.*` activity), and so are per-repository bindings, but paths saved in sources or other question text remain in the export.
- New work shows up as added lines: new records, changed fields and new activity entries.

A `.casebook` folder at the repository root also marks the project for the Claude Code hooks' marked scope ([Claude Code guide](claude-code.md#which-sessions-get-the-context)).

## Import it on the other computer

On the other computer, copy the file over (or pull the repository) and run:

```sh
casebook import .casebook/INV-0002.json
```

Current exports use format version 2 to carry who checked the answer and when. This Casebook also reads legacy version 1 files. Update an older Casebook before importing a version 2 export.

What import does:

- It checks the whole file before writing anything and adds the question in one step. It keeps the record refs; the question keeps its ref (`INV-0002`; every question ref starts with INV) when that ref is free on that machine and otherwise gets the next free one (the output says `Imported INV-0002 as INV-0005`).
- Everything it adds is marked as imported, and context packs show `(imported)` next to those refs.
- Checks are not trusted across machines: records marked `human_verified` or `source_verified` in the file come in as `ai_generated`. When the answer was checked, and what was checked, are also cleared on import. Only a person can check a record in the receiving database.
- It does not change the active question. To work on the question in this repository, run `casebook inv activate INV-0002 --here` (with the ref the import printed), so Claude Code and Codex sessions started in the repository pick it up.

## When the file changes

Importing a file always adds a separate copy; it never updates, replaces or merges a question that is already there. When the same question is already in the database (the original on the author's machine, or an earlier import of the file), import says so:

```text
Imported INV-0002 as INV-0006 (6 sources, 1 findings, 2 findings being tested, 1 things to find out).
The same question is already here as INV-0005; INV-0006 is a separate copy. To keep one, archive and delete the other in the desktop app (Casebook never merges copies).
```

"The same question" means a question created at the same moment by the same author, which is what an export carries. With `--json` the refs are in `same_case_refs`.

Each copy is its own question from then on:

- Moving your own question: import it on the new computer, check it is all there, then archive and delete the old copy on the old computer if you no longer need it. Check anything you rely on again and mark it with **I checked this**, because checks do not travel.
- Handing someone a copy: what they add stays in their copy and never comes back to yours. If they find something, they tell you and you record it in your question. To give them a newer version, export again; they archive and delete their old copy and import the new file.

## Troubleshooting

- `already exists; pass --force to overwrite`: the file is there from an earlier export. Add `--force` to replace it.
- A diff shows changes you did not expect: check the activity entries at the end of the file. Every change to the question, by you or by an AI client, is recorded there with the actor and time.
- `this file was saved by a newer Casebook`: update Casebook ([Updating Casebook](updating.md)).

## Using a shared answer in other apps

A shared answer is ordinary text that reads correctly anywhere: every finding keeps its label (how many of its sources a person checked, or Sources disagree), its sources with theirs and the note of what was left out. Casebook writes the text. It does not connect to your notes or document app, and it does not need to: you, or your AI client, put the text there.

### Copy or save it yourself

In the desktop app, choose **Share the answer** and pick how much to share as described in [Write the answer up with Share the answer](#write-the-answer-up-with-share-the-answer). **Copy** puts the text on the clipboard to paste into a note, a document, an email or a chat; **Save** writes a web page you can attach or keep. Both use what you see in the window.

From the command line:

```sh
casebook report INV-0002                                       # everything
casebook report INV-0002 --brief                               # the short version
casebook report INV-0002 --brief -o answer.md               # the short version, saved as a Markdown file
```

`--brief` gives the same short version as the app.

### Ask your AI client to save it

If your AI client (Claude Code, Codex, Claude Desktop, Cursor or another client) is connected to both Casebook and a notes or document app, you can ask it to do the saving. For example:

> Save the short version of the answer to "Why did calls go up 30% in October when sales stayed the same?" in my connected notes app as a new note. Keep its labels, its sources and its note of what is left out.

Change the app, or ask for everything instead of the short version, as you need.

What this needs:

- Your AI client must have tools for that app in its current session: the app's own connector, plugin or server, set up in the client. Casebook does not provide or configure them.
- The app's tools must be able to create a note or document (and to update one, if you ask for that) through your AI client.
- Your client's permission prompts apply. Read what it is about to write before you approve it.

Who does what:

- **Casebook** provides the text of the answer, and nothing else. (For developers: the client fetches it with the MCP tool `casebook_get_report`, with `mode` set to `brief` or `report`.)
- **Your AI client** finds the app's tools in its session, fetches the text from Casebook and calls the app's tool to write it, asking your permission the way it always does.
- **The app** creates or updates the note under your own sign-in and its own terms.

**Updating an existing note.** Ask the client to read the note first, keep everything in it that is not part of the shared answer, and use the app's real update tool. For example: "Replace the Casebook answer in my note 'Why calls went up in October' with the current short version of that answer, and leave the rest of the note as it is." If the app's tools can only create notes, the client should say so rather than quietly make a second copy.

**When nothing writable is connected.** If your client has no tool that can write to that app, it should tell you. Copy or save the answer yourself instead.

**What "done" means.** Your AI client reports what the app's tool actually returned. A successful write and reading the note back afterwards are different checks; ask the client to read the note back if you want the second one. Casebook records nothing about notes in other apps: it does not know where the text went, and it never says a note was created or updated.

**What your AI fetches.** The text your AI client fetches is taken when it asks. It matches what you saw in **Share the answer** only if the question has not changed since. To share exactly what you read, use Copy or Save in the app.

### If you used Evernote notes

Casebook 0.6.0 and earlier had an Evernote-specific feature. It was removed on 2026-10-27:

- Removed: the **Evernote block** panel with its setup steps, notebook field and "Last posted" line; the `casebook evernote` commands; the Evernote note tools for AI clients; ENEX import and export.
- Instead: copy or save the answer, use `casebook report --brief`, or, if your AI client is connected to Evernote, ask it to save the short version there as described above. Running `casebook evernote` now prints a short pointer to these and exits with an error.
- To record an Evernote note as a source, ask your AI client to add the note's text as a source with an accurate source reference (the note's title and link). Casebook does not fetch notes.
- An existing source from Evernote reads "Document or email", old History entries about posts still show, and notes you posted earlier stay in your Evernote account unchanged. ENEX files you saved stay on disk; Casebook no longer reads them.
