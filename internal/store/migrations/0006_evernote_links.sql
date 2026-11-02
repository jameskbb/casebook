-- Casebook schema version 6: Evernote notes (ADR-0019).
--
-- evidence.source_type gains 'evernote_note'. SQLite cannot alter a CHECK
-- constraint, so the Go step widenSourceTypeCheck (migrate_0006.go), run in
-- this file's transaction, rewrites the evidence CREATE statement with
-- SQLite's documented writable_schema procedure: the new list only adds a
-- value, so every existing row still satisfies it and no row is touched.
--
-- evernote_links remembers, per investigation, the one Evernote note the
-- case block was last posted to: the note's GUID and title, the sha256 hex
-- of the block Casebook wrote (so a hand-edited block is detected), the
-- note's update sequence number after that write, and when. It is local
-- state: backups and restores copy it, the stable export leaves it out.
-- Deleting the investigation deletes its link. No credential is ever stored
-- here.
CREATE TABLE evernote_links (
    investigation_id INTEGER PRIMARY KEY REFERENCES investigations (id) ON DELETE CASCADE,
    note_guid        TEXT NOT NULL CHECK (length(note_guid) BETWEEN 1 AND 64),
    note_title       TEXT NOT NULL DEFAULT '' CHECK (length(note_title) <= 1024),
    block_hash       TEXT NOT NULL CHECK (length(block_hash) = 64),
    note_usn         INTEGER NOT NULL DEFAULT 0,
    posted_at        TEXT NOT NULL
) STRICT;
