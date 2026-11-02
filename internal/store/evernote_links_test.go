package store

import (
	"context"
	"database/sql"
	"errors"
)

// Test-only helpers for the dormant evernote_links table. The Evernote
// integration is gone, so no production code reads or writes the table, but
// the shipped migrations 0006 and 0007 and their unchanged tests still need
// these to prove the table's constraints.

// EvernoteLink is an investigation's evernote_links row (ADR-0019): the
// note the case block was last posted to, the sha256 hex of that block, the
// note's update sequence number after the write and when it happened.
type EvernoteLink struct {
	InvestigationID int64
	NoteGUID        string
	NoteTitle       string
	BlockHash       string
	NoteUSN         int64
	PostedAt        string
}

// GetEvernoteLink returns investigation id's link and whether it has one.
func GetEvernoteLink(ctx context.Context, q Querier, id int64) (EvernoteLink, bool, error) {
	var l EvernoteLink
	err := q.QueryRowContext(ctx, `SELECT investigation_id, note_guid, note_title, block_hash, note_usn, posted_at
FROM evernote_links WHERE investigation_id = ?`, id).
		Scan(&l.InvestigationID, &l.NoteGUID, &l.NoteTitle, &l.BlockHash, &l.NoteUSN, &l.PostedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return EvernoteLink{}, false, nil
	}
	if err != nil {
		return EvernoteLink{}, false, err
	}
	return l, true, nil
}

// SetEvernoteLink stores l, replacing the investigation's previous link.
func SetEvernoteLink(ctx context.Context, q Querier, l EvernoteLink) error {
	_, err := q.ExecContext(ctx, `INSERT INTO evernote_links
(investigation_id, note_guid, note_title, block_hash, note_usn, posted_at) VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT (investigation_id) DO UPDATE SET note_guid = excluded.note_guid, note_title = excluded.note_title,
block_hash = excluded.block_hash, note_usn = excluded.note_usn, posted_at = excluded.posted_at`,
		l.InvestigationID, l.NoteGUID, l.NoteTitle, l.BlockHash, l.NoteUSN, l.PostedAt)
	return err
}

// DeleteEvernoteLink removes investigation id's link and reports whether it
// had one.
func DeleteEvernoteLink(ctx context.Context, q Querier, id int64) (bool, error) {
	res, err := q.ExecContext(ctx, `DELETE FROM evernote_links WHERE investigation_id = ?`, id)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}
