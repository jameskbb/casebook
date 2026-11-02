-- Casebook schema version 7: note ids from an AI client's Evernote
-- connection (ADR-0020).
--
-- Casebook no longer talks to Evernote itself. The note id it records after
-- a verified post is whatever id the person's AI client got from Evernote's
-- MCP tools, which is not documented to be a 36-character GUID, so
-- evernote_links.note_guid now holds 1 to 256 characters (the engine caps it
-- at 256 bytes). evernote_links is a leaf table (nothing references it), so
-- it is rebuilt: create the new table, copy every row, drop the old one and
-- rename. note_usn stays (always 0 from now on).
CREATE TABLE evernote_links_0007 (
    investigation_id INTEGER PRIMARY KEY REFERENCES investigations (id) ON DELETE CASCADE,
    note_guid        TEXT NOT NULL CHECK (length(note_guid) BETWEEN 1 AND 256),
    note_title       TEXT NOT NULL DEFAULT '' CHECK (length(note_title) <= 1024),
    block_hash       TEXT NOT NULL CHECK (length(block_hash) = 64),
    note_usn         INTEGER NOT NULL DEFAULT 0,
    posted_at        TEXT NOT NULL
) STRICT;

INSERT INTO evernote_links_0007 (investigation_id, note_guid, note_title, block_hash, note_usn, posted_at)
SELECT investigation_id, note_guid, note_title, block_hash, note_usn, posted_at FROM evernote_links;

DROP TABLE evernote_links;

ALTER TABLE evernote_links_0007 RENAME TO evernote_links;
