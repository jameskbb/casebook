# Share a question

There are two ways to share a question. **Share the answer** writes the answer up for someone who needs to read it; they do not need Casebook. Copy or save it, or ask your AI client to save it in a notes or document app you already use ([Using a question summary in other apps](#using-a-question-summary-in-other-apps), at the end of this page). An export file moves the question itself: to your own other computer, or as a copy for someone who wants to open it in their Casebook. Casebook is for one person, so a copy never stays linked to yours.

## Write the answer up with Share the answer

Open the question and choose **Share the answer**. Pick **Leadership brief** for the answer and the findings behind it, or **Complete report** for everything in the question, including its history. The same complete report comes from `casebook report` and from the `casebook_get_report` tool your AI can call; `casebook report --brief` and `casebook_get_report` with `mode` `brief` give the leadership brief with Casebook's default selection. Choosing the findings yourself is done in the desktop app.

For a brief, open **Choose findings and questions** to pick what goes in from four groups: Findings, Explanations still open or supported, Questions and Ruled out. Casebook starts with up to three current findings, two open or supported explanations, two unanswered questions and two ruled-out findings, and you can change the selection. Findings whose sources disagree, sources that go against a finding, and the findings behind the answer are always included and cannot be unticked. After changing the selection, choose **Update preview**.

The brief has these parts:

- **Question** and **Answer**: the question, the current answer and who checked it, the findings for and against it with their labels, when you checked it and what you checked, and why the answer holds.
- **Selected findings and what is still in doubt**: the chosen findings, each with its status and the label saying who checked it, and the sources for and against it by title and label.
- **Explanations still open or supported**: the explanations still being tested, so the reader sees what the answer does not yet settle.
- **Ruled out**: the chosen findings that were ruled out, with the sources that ruled them out.
- **Selected questions**: open and deferred questions, with the reason a question was deferred.
- **Sources and source caveats**: each cited source once, with where it came from and the saved summary.
- **Scope and omissions**: a short paragraph saying what You checked this, AI checked the source, Not checked, Sources disagree and Replaced mean, then a list of what was left out by ref (`Not included: Q-001, Q-002 and E-002.`), a note that activity history is not included, and a line telling the reader to ask you for the complete report.

It ends with `_Prepared by Casebook from the saved records of INV-0002. Casebook did not rewrite them or add to them._` Casebook builds the brief from your saved records without asking a model to write anything, so the same records and selection give the same text.

The preview does not change while the dialog is open, even if your AI changes the question. **Copy** and **Save as file...** use exactly the preview you reviewed; choose **Refresh preview** to bring in newer changes first.

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
- Checks are not trusted across machines: records marked `human_verified` or `source_verified` in the file come in as `ai_generated`. When the answer was checked, and what was checked, are also cleared on import. Only a person can mark a record You checked this in the receiving database.
- It does not change the active question. To work on the question in this repository, run `casebook inv activate INV-0002 --here` (with the ref the import printed), so Claude Code and Codex sessions started in the repository pick it up.

## When the file changes

Importing a file always adds a separate copy; it never updates, replaces or merges a question that is already there. When the same question is already in the database (the original on the author's machine, or an earlier import of the file), import says so:

```text
Imported INV-0002 as INV-0006 (6 sources, 1 findings, 2 findings being tested, 1 follow-up questions).
The same question is already here as INV-0005; INV-0006 is a separate copy. To keep one, archive and delete the other in the desktop app (Casebook never merges copies).
```

"The same question" means a question created at the same moment by the same author, which is what an export carries. With `--json` the refs are in `same_case_refs`.

Each copy is its own question from then on:

- Moving your own question: import it on the new computer, check it is all there, then archive and delete the old copy on the old computer if you no longer need it. Check anything you rely on again and mark it with **I checked this**, because checks do not travel.
- Handing someone a copy: what they add stays in their copy and never comes back to yours. If they find something, they tell you and you record it in your question. To give them a newer version, export again; they archive and delete their old copy and import the new file.

## Troubleshooting

- `already exists; pass --force to overwrite`: the file is there from an earlier export. Add `--force` to replace it.
- A diff shows changes you did not expect: check the activity entries at the end of the file. Every change to the question, by you or by an AI client, is recorded there with the actor and time.
- `unsupported export version`: the file came from a newer Casebook. Update Casebook ([Updating Casebook](updating.md)).

## Using a question summary in other apps

A question summary is ordinary text that reads correctly anywhere: every finding keeps the label saying who checked it (You checked this, AI checked the source, Not checked, Sources disagree), its source references and the caveats about what was left out. Casebook writes the summary. It does not connect to your notes or document app, and it does not need to: you, or your AI client, put the text there.

### Copy or save it yourself

In the desktop app, choose **Share the answer** and pick the format and findings as described in [Write the answer up with Share the answer](#write-the-answer-up-with-share-the-answer). **Copy** puts the summary on the clipboard to paste into a note, a document, an email or a chat; **Save as file...** writes it as a Markdown file you can attach, import or keep. Both use exactly the preview you reviewed.

From the command line:

```sh
casebook report INV-0002                                       # the complete report
casebook report INV-0002 --brief                               # the leadership brief
casebook report INV-0002 --brief -o october-calls-brief.md  # the brief, saved as a file
```

`--brief` chooses findings the way the dialog does before you change its selection.

### Ask your AI client to save it

If your AI client (Claude Code, Codex, Claude Desktop, Cursor or another client) is connected to both Casebook and a notes or document app, you can ask it to do the saving. **Share the answer** shows an example request for the open question, and **Copy example request** copies it so you can paste it into your AI client. For the featured question it reads:

> Save the leadership brief for question INV-0002 in my connected notes app as a new note titled "Why calls went up in October". Keep its labels, source references and caveats.

Change the app, the title, or "leadership brief" to "complete report", as you need.

What this needs:

- Your AI client must have tools for that app in its current session: the app's own connector, plugin or server, set up in the client. Casebook does not provide or configure them.
- The app's tools must be able to create a note or document (and to update one, if you ask for that) through your AI client.
- Your client's permission prompts apply. Read what it is about to write before you approve it.

Who does what:

- **Casebook** provides the summary text, and nothing else. (For developers: the client fetches it with the MCP tool `casebook_get_report`, with `mode` set to `brief` or `report`.)
- **Your AI client** finds the app's tools in its session, fetches the summary from Casebook and calls the app's tool to write it, asking your permission the way it always does.
- **The app** creates or updates the note under your own sign-in and its own terms.

**Updating an existing note.** Ask the client to read the note first, keep everything in it that is not part of the summary, and use the app's real update tool. For example: "Replace the Casebook summary in my note 'Why calls went up in October' with the current leadership brief for INV-0002, and leave the rest of the note as it is." If the app's tools can only create notes, the client should say so rather than quietly make a second copy.

**When nothing writable is connected.** If your client has no tool that can write to that app, it should tell you. Copy or save the summary yourself instead.

**What "done" means.** Your AI client reports what the app's tool actually returned. A successful write and reading the note back afterwards are different checks; ask the client to read the note back if you want the second one. Casebook records nothing about notes in other apps: it does not know where the summary went, and it never says a note was created or updated.

**Snapshots.** A summary your AI client fetches is taken when it asks, with Casebook's default selection of findings. It matches the preview you reviewed in **Share the answer** only if the question has not changed since and you kept that selection. To share exactly what you reviewed, use Copy or Save in the dialog.

### If you used Evernote notes

Casebook 0.6.0 and earlier had an Evernote-specific feature. It was removed on 2026-10-27:

- Removed: the **Evernote block** panel with its setup steps, notebook field and "Last posted" line; the `casebook evernote` commands; the Evernote note tools for AI clients; ENEX import and export.
- Instead: copy or save the summary, use `casebook report --brief`, or, if your AI client is connected to Evernote, ask it to save the brief there as described above. Running `casebook evernote` now prints a short pointer to these and exits with an error.
- To record an Evernote note as a source, ask your AI client to add the note's text as a source with an accurate source reference (the note's title and link). Casebook does not fetch notes.
- An existing source keeps its "Evernote note" label, old History entries about posts still show, and notes you posted earlier stay in your Evernote account unchanged. ENEX files you saved stay on disk; Casebook no longer reads them.
