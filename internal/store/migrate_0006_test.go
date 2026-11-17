package store

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

// TestMigration0006EvernoteLinks migrates a schema 5 database holding an
// investigation with evidence linked to a claim to schema 6, then checks
// the existing rows and links survive untouched, evernote_note evidence is
// accepted while unknown source types are still refused, and a note link
// can be written, replaced, and goes away with its investigation.
func TestMigration0006EvernoteLinks(t *testing.T) {
	ctx := context.Background()
	w := rawDB(t, filepath.Join(t.TempDir(), "casebook.db"))
	if v, err := Migrate(ctx, w, migrationsUpTo(t, 5), nil); err != nil || v != 5 {
		t.Fatalf("migrate to 5 = %d, %v", v, err)
	}
	const now = "2026-10-09T09:00:00.000Z"
	for _, q := range []string{
		`INSERT INTO investigations (id, ref_num, title, created_at, updated_at, created_by_kind, created_by_client)
VALUES (7, 1, 'T', '` + now + `', '` + now + `', 'human', 'cli')`,
		`INSERT INTO evidence (id, investigation_id, ref_num, title, summary, source_type, captured_at, created_at,
updated_at, created_by_kind, created_by_client, verification)
VALUES (3, 7, 1, 'log line', 'the flake', 'code', '` + now + `', '` + now + `', '` + now + `', 'ai', 'mcp', 'ai_generated')`,
		`INSERT INTO claims (id, investigation_id, ref_num, statement, verification, created_at, updated_at,
created_by_kind, created_by_client) VALUES (4, 7, 1, 'c', 'ai_generated', '` + now + `', '` + now + `', 'ai', 'mcp')`,
		`INSERT INTO claim_evidence (claim_id, evidence_id, relation, created_at, created_by_kind, created_by_client)
VALUES (4, 3, 'supports', '` + now + `', 'ai', 'mcp')`,
	} {
		if _, err := w.ExecContext(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := w.ExecContext(ctx, `UPDATE evidence SET source_type = 'evernote_note' WHERE id = 3`); err == nil ||
		!isConstraintErr(err) {
		t.Fatalf("evernote_note under schema 5: err = %v, want a constraint error", err)
	}

	if v, err := Migrate(ctx, w, migrationsUpTo(t, 6), nil); err != nil || v != 6 {
		t.Fatalf("migrate to 6 = %d, %v", v, err)
	}

	var title, links, hits int
	if err := w.QueryRowContext(ctx, `SELECT count(*) FROM evidence WHERE id = 3 AND title = 'log line'`).Scan(&title); err != nil || title != 1 {
		t.Fatalf("evidence after 0006 = %d, %v", title, err)
	}
	if err := w.QueryRowContext(ctx, `SELECT count(*) FROM claim_evidence`).Scan(&links); err != nil || links != 1 {
		t.Fatalf("claim_evidence after 0006 = %d, %v", links, err)
	}
	if err := w.QueryRowContext(ctx, `SELECT count(*) FROM search_index WHERE search_index MATCH 'flake'`).Scan(&hits); err != nil || hits != 1 {
		t.Fatalf("search hits after 0006 = %d, %v", hits, err)
	}
	if _, err := w.ExecContext(ctx, `UPDATE evidence SET source_type = 'evernote_note', summary = 'the note' WHERE id = 3`); err != nil {
		t.Fatalf("evernote_note after 0006: %v", err)
	}
	if err := w.QueryRowContext(ctx, `SELECT count(*) FROM search_index WHERE search_index MATCH 'note'`).Scan(&hits); err != nil || hits != 1 {
		t.Fatalf("search triggers after 0006: hits = %d, %v", hits, err)
	}
	if _, err := w.ExecContext(ctx, `UPDATE evidence SET source_type = 'onenote' WHERE id = 3`); err == nil || !isConstraintErr(err) {
		t.Fatalf("unknown source type after 0006: err = %v, want a constraint error", err)
	}
	var ok string
	if err := w.QueryRowContext(ctx, `PRAGMA integrity_check`).Scan(&ok); err != nil || ok != "ok" {
		t.Fatalf("integrity_check = %q, %v", ok, err)
	}

	hash := strings.Repeat("ab", 32)
	l := EvernoteLink{InvestigationID: 7, NoteGUID: "g-1", NoteTitle: "Checkout flake", BlockHash: hash, NoteUSN: 41, PostedAt: now}
	if err := SetEvernoteLink(ctx, w, l); err != nil {
		t.Fatal(err)
	}
	l.NoteUSN, l.NoteTitle = 42, "Checkout flake (renamed)"
	if err := SetEvernoteLink(ctx, w, l); err != nil {
		t.Fatal(err)
	}
	got, found, err := GetEvernoteLink(ctx, w, 7)
	if err != nil || !found || got != l {
		t.Fatalf("GetEvernoteLink = %+v, %v, %v; want %+v", got, found, err, l)
	}
	bad := l
	bad.InvestigationID = 99
	if err := SetEvernoteLink(ctx, w, bad); err == nil || !isConstraintErr(err) {
		t.Fatalf("link to a missing investigation: err = %v, want a constraint error", err)
	}
	bad = l
	bad.BlockHash = "short"
	if err := SetEvernoteLink(ctx, w, bad); err == nil || !isConstraintErr(err) {
		t.Fatalf("link with a bad hash: err = %v, want a constraint error", err)
	}
	if _, err := w.ExecContext(ctx, `DELETE FROM investigations WHERE id = 7`); err != nil {
		t.Fatal(err)
	}
	if _, found, err := GetEvernoteLink(ctx, w, 7); err != nil || found {
		t.Fatalf("link after deleting the investigation: found = %v, %v", found, err)
	}
}

// TestDeleteEvernoteLink removes a link and reports whether one existed.
func TestDeleteEvernoteLink(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	const now = "2026-10-09T09:00:00.000Z"
	if _, err := db.writer.ExecContext(ctx, `INSERT INTO investigations (id, ref_num, title, created_at, updated_at,
created_by_kind, created_by_client) VALUES (1, 1, 'T', ?, ?, 'human', 'cli')`, now, now); err != nil {
		t.Fatal(err)
	}
	if err := SetEvernoteLink(ctx, db.writer, EvernoteLink{InvestigationID: 1, NoteGUID: "g", BlockHash: strings.Repeat("0", 64), PostedAt: now}); err != nil {
		t.Fatal(err)
	}
	for i, want := range []bool{true, false} {
		had, err := DeleteEvernoteLink(ctx, db.writer, 1)
		if err != nil || had != want {
			t.Fatalf("delete %d = %v, %v; want %v", i, had, err, want)
		}
	}
}
