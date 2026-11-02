package store

import (
	"context"
	"io/fs"
	"path/filepath"
	"testing"
	"testing/fstest"
)

// migrationsUpTo returns the embedded migrations numbered at most n.
func migrationsUpTo(t *testing.T, n int) fstest.MapFS {
	t.Helper()
	sub, err := fs.Sub(Migrations, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	ms, err := listMigrations(sub)
	if err != nil {
		t.Fatal(err)
	}
	out := fstest.MapFS{}
	for _, m := range ms {
		if m.version > n {
			break
		}
		b, err := fs.ReadFile(sub, m.name)
		if err != nil {
			t.Fatal(err)
		}
		out[m.name] = &fstest.MapFile{Data: b}
	}
	return out
}

// TestMigration0002RepairsStatusVerification seeds every disagreeing status
// and verification pair under schema 1 and checks migration 0002 repairs
// each one (ADR-0013) and then refuses new disagreeing writes.
func TestMigration0002RepairsStatusVerification(t *testing.T) {
	ctx := context.Background()
	w := rawDB(t, filepath.Join(t.TempDir(), "casebook.db"))
	if v, err := Migrate(ctx, w, migrationsUpTo(t, 1), nil); err != nil || v != 1 {
		t.Fatalf("migrate to 1 = %d, %v", v, err)
	}
	type pair struct{ status, verification, kind string }
	claims := []struct{ in, want pair }{
		{pair{"active", "superseded", "human"}, pair{"active", "human_verified", ""}},
		{pair{"active", "superseded", "ai"}, pair{"active", "ai_generated", ""}},
		{pair{"active", "disputed", "ai"}, pair{"disputed", "disputed", ""}},
		{pair{"disputed", "human_verified", "human"}, pair{"disputed", "disputed", ""}},
		{pair{"disputed", "superseded", "ai"}, pair{"disputed", "disputed", ""}},
		{pair{"superseded", "ai_generated", "ai"}, pair{"superseded", "superseded", ""}},
		{pair{"rejected", "disputed", "human"}, pair{"rejected", "human_verified", ""}},
		{pair{"rejected", "source_verified", "ai"}, pair{"rejected", "source_verified", ""}},
		{pair{"active", "source_verified", "ai"}, pair{"active", "source_verified", ""}},
	}
	hyps := []struct{ in, want pair }{
		{pair{"open", "superseded", "import"}, pair{"open", "ai_generated", ""}},
		{pair{"superseded", "disputed", "ai"}, pair{"superseded", "superseded", ""}},
		{pair{"open", "disputed", "ai"}, pair{"open", "disputed", ""}},
		{pair{"disproven", "human_verified", "human"}, pair{"disproven", "human_verified", ""}},
	}
	tx, err := w.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := seedParents(ctx, tx); err != nil {
		t.Fatal(err)
	}
	for i, c := range claims {
		row := baseRow("claims", 10+i)
		row["status"], row["verification"], row["created_by_kind"] = c.in.status, c.in.verification, c.in.kind
		if err := insertRow(ctx, tx, "claims", row); err != nil {
			t.Fatal(err)
		}
	}
	for i, h := range hyps {
		row := baseRow("hypotheses", 10+i)
		row["status"], row["verification"], row["created_by_kind"] = h.in.status, h.in.verification, h.in.kind
		if err := insertRow(ctx, tx, "hypotheses", row); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	if v, err := Migrate(ctx, w, migrationsUpTo(t, 2), nil); err != nil || v != 2 {
		t.Fatalf("migrate to 2 = %d, %v", v, err)
	}
	check := func(table string, cases []struct{ in, want pair }) {
		t.Helper()
		for i, c := range cases {
			var got pair
			if err := w.QueryRowContext(ctx, `SELECT status, verification FROM `+table+` WHERE ref_num = ?`,
				110+i).Scan(&got.status, &got.verification); err != nil {
				t.Fatal(err)
			}
			if got != c.want {
				t.Errorf("%s %v repaired to %v, want %v", table, c.in, got, c.want)
			}
		}
	}
	check("claims", claims)
	check("hypotheses", hyps)

	for _, q := range []string{
		`UPDATE claims SET verification = 'disputed' WHERE ref_num = 110`,
		`UPDATE claims SET status = 'active' WHERE ref_num = 112`,
		`UPDATE hypotheses SET verification = 'superseded' WHERE ref_num = 110`,
	} {
		if _, err := w.ExecContext(ctx, q); err == nil || !isConstraintErr(err) {
			t.Errorf("%s: want constraint error, got %v", q, err)
		}
	}
}

// TestMigration0003AddsImported: existing rows read as not imported and the
// column only holds 0 or 1.
func TestMigration0003AddsImported(t *testing.T) {
	ctx := context.Background()
	w := rawDB(t, filepath.Join(t.TempDir(), "casebook.db"))
	if v, err := Migrate(ctx, w, migrationsUpTo(t, 2), nil); err != nil || v != 2 {
		t.Fatalf("migrate to 2 = %d, %v", v, err)
	}
	tx, err := w.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := seedParents(ctx, tx); err != nil {
		t.Fatal(err)
	}
	if err := insertRow(ctx, tx, "activity", baseRow("activity", 1)); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if v, err := Migrate(ctx, w, migrationsUpTo(t, 3), nil); err != nil || v != 3 {
		t.Fatalf("migrate to 3 = %d, %v", v, err)
	}
	for _, table := range []string{"evidence", "claims", "hypotheses", "activity"} {
		var n int
		if err := w.QueryRowContext(ctx, `SELECT count(*) FROM `+table+` WHERE imported != 0`).Scan(&n); err != nil || n != 0 {
			t.Fatalf("%s imported rows = %d, %v", table, n, err)
		}
		if _, err := w.ExecContext(ctx, `UPDATE `+table+` SET imported = 2`); err == nil || !isConstraintErr(err) {
			t.Fatalf("%s imported = 2: want constraint error, got %v", table, err)
		}
	}
}
