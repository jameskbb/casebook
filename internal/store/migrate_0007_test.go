package store

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

// TestMigration0007NoteIDs migrates a schema 6 database holding a note link
// to schema 7, then checks the link survives byte for byte, note ids of up
// to 256 characters are accepted while empty and longer ones are refused,
// the other checks still hold, and the link still goes away with its
// investigation.
func TestMigration0007NoteIDs(t *testing.T) {
	ctx := context.Background()
	w := rawDB(t, filepath.Join(t.TempDir(), "casebook.db"))
	if v, err := Migrate(ctx, w, migrationsUpTo(t, 6), nil); err != nil || v != 6 {
		t.Fatalf("migrate to 6 = %d, %v", v, err)
	}
	const now = "2026-10-09T09:00:00.000Z"
	for _, q := range []string{
		`INSERT INTO investigations (id, ref_num, title, created_at, updated_at, created_by_kind, created_by_client)
VALUES (7, 1, 'T', '` + now + `', '` + now + `', 'human', 'cli')`,
		`INSERT INTO investigations (id, ref_num, title, created_at, updated_at, created_by_kind, created_by_client)
VALUES (8, 2, 'U', '` + now + `', '` + now + `', 'human', 'cli')`,
	} {
		if _, err := w.ExecContext(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	hash := strings.Repeat("ab", 32)
	old := EvernoteLink{InvestigationID: 7, NoteGUID: "5e0f2a64-8c1d-4b7a-9f3e-2d6c8b1a4e90", NoteTitle: "Checkout flake",
		BlockHash: hash, NoteUSN: 41, PostedAt: now}
	if err := SetEvernoteLink(ctx, w, old); err != nil {
		t.Fatal(err)
	}
	long := old
	long.InvestigationID, long.NoteGUID = 8, strings.Repeat("n", 65)
	if err := SetEvernoteLink(ctx, w, long); err == nil || !isConstraintErr(err) {
		t.Fatalf("65-character id under schema 6: err = %v, want a constraint error", err)
	}

	if v, err := Migrate(ctx, w, migrationsUpTo(t, 7), nil); err != nil || v != 7 {
		t.Fatalf("migrate to 7 = %d, %v", v, err)
	}
	if got, found, err := GetEvernoteLink(ctx, w, 7); err != nil || !found || got != old {
		t.Fatalf("link after 0007 = %+v, %v, %v; want %+v", got, found, err, old)
	}
	long.NoteGUID = strings.Repeat("n", 256)
	if err := SetEvernoteLink(ctx, w, long); err != nil {
		t.Fatalf("256-character id after 0007: %v", err)
	}
	for name, bad := range map[string]EvernoteLink{
		"257 characters": {InvestigationID: 8, NoteGUID: strings.Repeat("n", 257), BlockHash: hash, PostedAt: now},
		"empty id":       {InvestigationID: 8, NoteGUID: "", BlockHash: hash, PostedAt: now},
		"short hash":     {InvestigationID: 8, NoteGUID: "g", BlockHash: "short", PostedAt: now},
		"no such case":   {InvestigationID: 99, NoteGUID: "g", BlockHash: hash, PostedAt: now},
	} {
		if err := SetEvernoteLink(ctx, w, bad); err == nil || !isConstraintErr(err) {
			t.Errorf("%s: err = %v, want a constraint error", name, err)
		}
	}
	var ok string
	if err := w.QueryRowContext(ctx, `PRAGMA integrity_check`).Scan(&ok); err != nil || ok != "ok" {
		t.Fatalf("integrity_check = %q, %v", ok, err)
	}
	if err := w.QueryRowContext(ctx, `PRAGMA foreign_key_check`).Scan(&ok); err == nil {
		t.Fatalf("foreign_key_check found a violation: %s", ok)
	}
	if _, err := w.ExecContext(ctx, `DELETE FROM investigations WHERE id = 7`); err != nil {
		t.Fatal(err)
	}
	if _, found, err := GetEvernoteLink(ctx, w, 7); err != nil || found {
		t.Fatalf("link after deleting the investigation: found = %v, %v", found, err)
	}
	var tables int
	if err := w.QueryRowContext(ctx, `SELECT count(*) FROM sqlite_schema WHERE name LIKE 'evernote_links%'`).Scan(&tables); err != nil || tables != 1 {
		t.Fatalf("evernote_links tables = %d, %v", tables, err)
	}
}
