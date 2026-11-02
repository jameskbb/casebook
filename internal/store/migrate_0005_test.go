package store

import (
	"context"
	"path/filepath"
	"testing"
)

// TestMigration0005DirectoryBindings migrates a schema 4 database holding an
// investigation and the global setting to schema 5, then checks bindings
// can be written, keep the existing data, and go away with their
// investigation.
func TestMigration0005DirectoryBindings(t *testing.T) {
	ctx := context.Background()
	w := rawDB(t, filepath.Join(t.TempDir(), "casebook.db"))
	if v, err := Migrate(ctx, w, migrationsUpTo(t, 4), nil); err != nil || v != 4 {
		t.Fatalf("migrate to 4 = %d, %v", v, err)
	}
	const now = "2026-10-06T09:00:00.000Z"
	if _, err := w.ExecContext(ctx, `INSERT INTO investigations (id, ref_num, title, problem_statement, status,
conclusion, conclusion_confidence, conclusion_rationale, created_at, updated_at, created_by_kind, created_by_client)
VALUES (7, 1, 'T', '', 'new', '', 'unknown', '', ?, ?, 'human', 'cli')`, now, now); err != nil {
		t.Fatal(err)
	}
	if err := SetActiveInvestigationID(ctx, w, 7, now); err != nil {
		t.Fatal(err)
	}
	if v, err := Migrate(ctx, w, migrationsUpTo(t, 5), nil); err != nil || v != 5 {
		t.Fatalf("migrate to 5 = %d, %v", v, err)
	}
	if id, ok, err := GetActiveInvestigationID(ctx, w); err != nil || !ok || id != 7 {
		t.Fatalf("global active after 0005 = %d, %v, %v", id, ok, err)
	}
	if err := SetDirectoryBinding(ctx, w, "/src/shop", 7, now); err != nil {
		t.Fatal(err)
	}
	b, found, err := NearestDirectoryBinding(ctx, w, []string{"/src/shop/internal", "/src/shop", "/src", "/"})
	if err != nil || !found || b.Dir != "/src/shop" || b.InvestigationID != 7 {
		t.Fatalf("NearestDirectoryBinding = %+v, %v, %v", b, found, err)
	}
	if err := SetDirectoryBinding(ctx, w, "/src/missing", 99, now); err == nil || !isConstraintErr(err) {
		t.Fatalf("binding to a missing investigation: err = %v, want a constraint error", err)
	}
	if _, err := w.ExecContext(ctx, `DELETE FROM investigations WHERE id = 7`); err != nil {
		t.Fatal(err)
	}
	list, err := ListDirectoryBindings(ctx, w)
	if err != nil || len(list) != 0 {
		t.Fatalf("bindings after deleting the investigation = %+v, %v", list, err)
	}
}
