package store

import (
	"context"
	"database/sql"
	"testing"
)

func ftsCount(t *testing.T, db *DB, match string) int {
	t.Helper()
	var n int
	if err := db.reader.QueryRow(`SELECT count(*) FROM search_index WHERE search_index MATCH ?`, match).Scan(&n); err != nil {
		t.Fatalf("MATCH %q: %v", match, err)
	}
	return n
}

func mustWrite(t *testing.T, db *DB, fn func(ctx context.Context, tx *sql.Tx) error) {
	t.Helper()
	ctx := context.Background()
	if err := db.WithWriteTx(ctx, func(tx *sql.Tx) error { return fn(ctx, tx) }); err != nil {
		t.Fatal(err)
	}
}

func TestSearchIndexInvestigationTriggers(t *testing.T) {
	db := openTestDB(t)
	var id int64
	mustWrite(t, db, func(ctx context.Context, tx *sql.Tx) error {
		var err error
		id, err = insertTestInvestigation(ctx, tx, "Zebra spike")
		return err
	})
	if ftsCount(t, db, "zebra") != 1 {
		t.Fatal("inserted title does not match")
	}

	mustWrite(t, db, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `UPDATE investigations SET title = 'Giraffe spike', problem_statement = 'okapi' WHERE id = ?`, id)
		return err
	})
	if ftsCount(t, db, "zebra") != 0 || ftsCount(t, db, "giraffe") != 1 || ftsCount(t, db, "okapi") != 1 {
		t.Fatal("title update did not move the FTS row")
	}

	// Plant a canary in the FTS row; touching updated_at or bumping a record
	// counter must not rewrite it (UPDATE OF lists only searchable columns).
	mustWrite(t, db, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `UPDATE search_index SET body = 'canaryword' WHERE rowid = ?`, id*8+1)
		return err
	})
	mustWrite(t, db, func(ctx context.Context, tx *sql.Tx) error {
		if err := TouchInvestigation(ctx, tx, id, "2026-10-02T00:00:00.000Z"); err != nil {
			return err
		}
		_, err := AllocateRecordNum(ctx, tx, id, "claim")
		return err
	})
	if ftsCount(t, db, "canaryword") != 1 {
		t.Fatal("touching updated_at or a counter rewrote the FTS row")
	}

	mustWrite(t, db, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `DELETE FROM investigations WHERE id = ?`, id)
		return err
	})
	if ftsCount(t, db, "giraffe") != 0 || ftsCount(t, db, "canaryword") != 0 {
		t.Fatal("deleted investigation is still indexed")
	}
}

func TestSearchIndexRecordTriggers(t *testing.T) {
	db := openTestDB(t)
	mustWrite(t, db, func(ctx context.Context, tx *sql.Tx) error {
		if _, err := insertTestInvestigation(ctx, tx, "parent"); err != nil {
			return err
		}
		ev := baseRow("evidence", 1)
		ev["title"], ev["summary"], ev["raw_detail"] = "Billing query", "lynx summary", "secretrawdetail"
		cl := baseRow("claims", 1)
		cl["statement"] = "ocelot claim"
		hy := baseRow("hypotheses", 1)
		hy["statement"] = "puma hypothesis"
		qu := baseRow("questions", 1)
		qu["question"] = "cougar question"
		for table, row := range map[string]map[string]any{"evidence": ev, "claims": cl, "hypotheses": hy, "questions": qu} {
			if err := insertRow(ctx, tx, table, row); err != nil {
				return err
			}
		}
		return nil
	})
	for _, term := range []string{"billing", "lynx", "ocelot", "puma", "cougar"} {
		if ftsCount(t, db, term) != 1 {
			t.Errorf("%q not indexed", term)
		}
	}
	if ftsCount(t, db, "secretrawdetail") != 0 {
		t.Error("evidence raw_detail must never be indexed")
	}

	mustWrite(t, db, func(ctx context.Context, tx *sql.Tx) error {
		for _, q := range []string{
			`UPDATE evidence SET summary = 'bobcat summary'`,
			`UPDATE claims SET statement = 'jaguar claim'`,
			`UPDATE hypotheses SET statement = 'serval hypothesis'`,
			`UPDATE questions SET status = 'resolved', resolution = 'caracal answer'`,
		} {
			if _, err := tx.ExecContext(ctx, q); err != nil {
				return err
			}
		}
		return nil
	})
	for term, want := range map[string]int{"lynx": 0, "bobcat": 1, "ocelot": 0, "jaguar": 1, "puma": 0, "serval": 1, "cougar": 1, "caracal": 1} {
		if got := ftsCount(t, db, term); got != want {
			t.Errorf("after update %q matches %d, want %d", term, got, want)
		}
	}

	// Deleting the investigation cascades to its records and their FTS rows.
	mustWrite(t, db, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `DELETE FROM investigations`)
		return err
	})
	var n int
	if err := db.reader.QueryRow(`SELECT count(*) FROM search_index`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("search_index rows after cascade delete = %d, %v", n, err)
	}
}
