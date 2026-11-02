# Share a case

There are two ways to share a case. **Share summary** writes the answer up for someone who needs to read it; they do not need Casebook. Copy or save it, or ask your AI client to save it in a notes or document app you already use ([Using a case summary in other apps](#using-a-case-summary-in-other-apps), at the end of this page). An export file moves the case itself: to your own other computer, or as a copy for someone who wants to open it in their Casebook. Casebook is for one person, so a copy never stays linked to yours.

## Share the answer with Share summary

Open the case and choose **Share summary**. Pick **Leadership brief** for the answer and the findings behind it, or **Complete report** for everything in the case, including its history. The same complete report comes from `casebook report` and from the `casebook_get_report` tool your AI can call; `casebook report --brief` and `casebook_get_report` with `mode` `brief` give the leadership brief with Casebook's default selection. Choosing the findings yourself is done in the desktop app.

For a brief, open **Choose findings and questions** to pick what goes in from four groups: Claims, Explanations still open or supported, Questions and Ruled out. Casebook starts with up to three current claims, two open or supported explanations, two unanswered questions and two ruled-out findings, and you can change the selection. Disputed findings, contradicting evidence and the claims behind the answer are always included and cannot be unticked. After changing the selection, choose **Update preview**.

The brief has these parts:

- **Question** and **Answer**: the case's question, the current conclusion, its verification, the claims supporting and contradicting it with their labels (`Supporting claims: C-002 (AI generated); C-003 (Human verified)`), when you reviewed it and on what basis, and the confidence with its reason.
- **Selected findings and what is still in doubt**: the chosen claims, each with its status, confidence and verification label (`(Current; High confidence; AI generated)`, `(Disputed; Low confidence)`), and the evidence for and against it by title and label.
- **Explanations still open or supported**: the explanations still being tested, so the reader sees what the answer does not yet settle.
- **Ruled out**: the chosen ruled-out explanations and rejected claims, with the evidence that ruled them out. They show no confidence.
- **Selected questions**: open and deferred questions, with the reason a question was deferred.
- **Evidence and source caveats**: each cited evidence item once, with its source and the saved summary.
- **Scope and omissions**: a short paragraph saying what Human verified, Source verified, AI generated, Disputed and Superseded mean, then a list of what was left out by ref (`Not included: Q-001, Q-002 and E-002.`), a note that activity history is not included, and a line telling the reader to ask you for the complete report.

It ends with `_Prepared by Casebook from the saved records of INV-0002. Casebook did not rewrite them or add to them._` Casebook builds the brief from your saved records without asking a model to write anything, so the same records and selection give the same text.

The preview does not change while the dialog is open, even if your AI changes the case. **Copy** and **Save as file...** use exactly the preview you reviewed; choose **Refresh preview** to bring in newer changes first.

## Move a case or hand over a copy

To open the case in another Casebook, export it to a file and import that file on the other computer. You might move your own case to a new laptop, keep it beside a project in a Git repository, or give a colleague a copy to read and hand to their own AI. The file format is described in [ADR-0016](decisions/0016-stable-export-for-repo-sharing.md).

## Before you hand over a file

An export holds everything in the case: titles, summaries, raw evidence detail, metadata and the activity log. Read it before you hand it over or commit it, and never share a case whose evidence holds secrets, tokens, customer data or anything else the recipient should not see. Anyone who can read the file, or the repository it is committed to, can read the case. See [Privacy and local data](privacy.md#backups-export-and-restore).

## Export the case

To write the file into a Git repository, run this from the repository root (any other folder works the same way without the `git` lines):

```sh
mkdir -p .casebook
casebook export INV-0002 --out .casebook/INV-0002.json
git add .casebook/INV-0002.json
git commit -m "Add the Casebook case for the Northeast margin"
```

`--out` (or `-o`) never overwrites an existing file. To refresh the file after more work, add `--force`:

```sh
casebook export INV-0002 --out .casebook/INV-0002.json --force
```

The file is written to diff well:

- It is indented JSON with a fixed key order. Records are sorted by ref (`E-001`, `E-002`, ...), links by record and then evidence ref, and activity in the order it happened.
- Exporting an unchanged case gives the same bytes, so `git status` stays clean until something real changes. `exported_at` is the time of the newest change in the file, not the time you ran the command.
- Which case is active on your machine is left out (`active.*` activity), and so are per-repository bindings, but paths saved in evidence or other case text remain in the export.
- New work shows up as added lines: new records, changed fields and new activity entries.

A `.casebook` folder at the repository root also marks the project for the Claude Code hooks' marked scope ([Claude Code guide](claude-code.md#which-sessions-get-the-context)).

## Import it on the other computer

On the other computer, copy the file over (or pull the repository) and run:

```sh
casebook import .casebook/INV-0002.json
```

Current exports use format version 2 to carry conclusion verification and review details. This Casebook also reads legacy version 1 files. Update an older Casebook before importing a version 2 export.

What import does:

- It checks the whole file before writing anything and adds the case in one step. It keeps the record refs; the case keeps its ref (`INV-0002`; every case ref starts with INV) when that ref is free on that machine and otherwise gets the next free one (the output says `Imported INV-0002 as INV-0005`).
- Everything it adds is marked as imported, and context packs show `(imported)` next to those refs.
- Verification is not trusted across machines: records marked `human_verified` or `source_verified` in the file come in as `ai_generated`. Conclusion review timestamps and review bases are also cleared on import. Only a person can mark a record Human verified in the receiving database ([ADR-0014](decisions/0014-only-people-verify.md)).
- It does not change the active case. To work on the case in this repository, run `casebook case activate INV-0002 --here` (with the ref the import printed), so Claude Code and Codex sessions started in the repository pick it up ([ADR-0015](decisions/0015-per-directory-active-investigation.md)).

## When the file changes

Importing a file always adds a separate copy; it never updates, replaces or merges a case that is already there. When the same case is already in the database (the original on the author's machine, or an earlier import of the file), import says so:

```text
Imported INV-0002 as INV-0006 (6 evidence, 1 claims, 2 hypotheses, 1 questions).
The same case is already here as INV-0005; INV-0006 is a separate copy. To keep one, archive and delete the other in the desktop app (Casebook never merges copies).
```

"The same case" means a case created at the same moment by the same author, which is what an export carries. With `--json` the refs are in `same_case_refs`.

Each copy is its own case from then on:

- Moving your own case: import it on the new computer, check it is all there, then archive and delete the old copy on the old computer if you no longer need it. Check again with **Verify myself** anything you rely on, because verification does not travel.
- Handing someone a copy: what they add stays in their copy and never comes back to yours. If they find something, they tell you and you record it in your case. To give them a newer version, export again; they archive and delete their old copy and import the new file.

## Troubleshooting

- `already exists; pass --force to overwrite`: the file is there from an earlier export. Add `--force` to replace it.
- A diff shows changes you did not expect: check the activity entries at the end of the file. Every change to the case, by you or by an AI client, is recorded there with the actor and time.
- `unsupported export version`: the file came from a newer Casebook. Update Casebook ([Updating Casebook](updating.md)).

## Using a case summary in other apps

A case summary is ordinary text that reads correctly anywhere: every finding keeps its verification label (Human verified, Source verified, AI generated, Disputed), its evidence references and the caveats about what was left out. Casebook writes the summary. It does not connect to your notes or document app, and it does not need to: you, or your AI client, put the text there. The design is in [ADR 0024](decisions/0024-portable-output-instead-of-evernote.md).

### Copy or save it yourself

In the desktop app, choose **Share summary** and pick the format and findings as described in [Share the answer with Share summary](#share-the-answer-with-share-summary). **Copy** puts the summary on the clipboard to paste into a note, a document, an email or a chat; **Save as file...** writes it as a Markdown file you can attach, import or keep. Both use exactly the preview you reviewed.

From the command line:

```sh
casebook report INV-0002                                       # the complete report
casebook report INV-0002 --brief                               # the leadership brief
casebook report INV-0002 --brief -o northeast-margin-brief.md  # the brief, saved as a file
```

`--brief` chooses findings the way the dialog does before you change its selection.

### Ask your AI client to save it

If your AI client (Claude Code, Codex, Claude Desktop, Cursor or another client) is connected to both Casebook and a notes or document app, you can ask it to do the saving. **Share summary** shows an example request for the open case, and **Copy example request** copies it so you can paste it into your AI client. For the featured case it reads:

> Save the leadership brief for case INV-0002 in my connected notes app as a new note titled "Northeast margin review". Keep its verification labels, evidence references and caveats.

Change the app, the title, or "leadership brief" to "complete report", as you need.

What this needs:

- Your AI client must have tools for that app in its current session: the app's own connector, plugin or server, set up in the client. Casebook does not provide or configure them.
- The app's tools must be able to create a note or document (and to update one, if you ask for that) through your AI client.
- Your client's permission prompts apply. Read what it is about to write before you approve it.

Who does what:

- **Casebook** provides the summary text, and nothing else. (For contributors: the client fetches it with the MCP tool `casebook_get_report`, with `mode` set to `brief` or `report`.)
- **Your AI client** finds the app's tools in its session, fetches the summary from Casebook and calls the app's tool to write it, asking your permission the way it always does.
- **The app** creates or updates the note under your own sign-in and its own terms.

**Updating an existing note.** Ask the client to read the note first, keep everything in it that is not part of the summary, and use the app's real update tool. For example: "Replace the Casebook summary in my note 'Northeast margin review' with the current leadership brief for INV-0002, and leave the rest of the note as it is." If the app's tools can only create notes, the client should say so rather than quietly make a second copy.

**When nothing writable is connected.** If your client has no tool that can write to that app, it should tell you. Copy or save the summary yourself instead.

**What "done" means.** Your AI client reports what the app's tool actually returned. A successful write and reading the note back afterwards are different checks; ask the client to read the note back if you want the second one. Casebook records nothing about notes in other apps: it does not know where the summary went, and it never says a note was created or updated.

**Snapshots.** A summary your AI client fetches is taken when it asks, with Casebook's default selection of findings. It matches the preview you reviewed in **Share summary** only if the case has not changed since and you kept that selection. To share exactly what you reviewed, use Copy or Save in the dialog.

### If you used Evernote notes

Casebook 0.6.0 and earlier had an Evernote-specific feature. It was removed on 2026-10-27:

- Removed: the **Evernote block** panel with its setup steps, notebook field and "Last posted" line; the `casebook evernote` commands; the Evernote note tools for AI clients; ENEX import and export.
- Instead: copy or save the summary, use `casebook report --brief`, or, if your AI client is connected to Evernote, ask it to save the brief there as described above. Running `casebook evernote` now prints a short pointer to these and exits with an error.
- To record an Evernote note as evidence, ask your AI client to add the note's text as evidence with an accurate source reference (the note's title and link). Casebook does not fetch notes.
- Existing evidence keeps its "Evernote note" label, old History entries about posts still show, and notes you posted earlier stay in your Evernote account unchanged. ENEX files you saved stay on disk; Casebook no longer reads them.
