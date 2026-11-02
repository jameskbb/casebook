package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

func TestSanitizeIntegrityLine(t *testing.T) {
	objects := map[string]string{"claims": "table", "claims_superseded_by": "index"}
	cases := map[string]string{
		"row 3 missing from index claims_superseded_by":                           "integrity_check: problem in index claims_superseded_by",
		"NULL value in claims.statement":                                          "integrity_check: problem in table claims",
		"*** in database main ***\nPage 5: btreeInitPage() returns error code 11": "integrity_check: damaged database pages",
		"secret 'value' in unknown_table":                                         "integrity_check: damaged database pages",
	}
	for in, want := range cases {
		if got := sanitizeIntegrityLine(in, objects); got != want {
			t.Errorf("sanitizeIntegrityLine(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestIntegrityProblemsSanitizesRawLines corrupts an index (its definition no
// longer matches its content) and checks the reported problems name the
// index only, never raw integrity_check text.
func TestIntegrityProblemsSanitizesRawLines(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "casebook.db")
	db, err := Open(ctx, path, Config{})
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, db, func(ctx context.Context, tx *sql.Tx) error {
		_, err := insertTestInvestigation(ctx, tx, "Mixed Case Title")
		return err
	})
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	tamperOneConn(t, path,
		`CREATE INDEX damaged_idx ON investigations (lower(title))`,
		`PRAGMA writable_schema = ON`,
		`UPDATE sqlite_schema SET sql = 'CREATE INDEX damaged_idx ON investigations (upper(title))' WHERE name = 'damaged_idx'`,
		`PRAGMA writable_schema = OFF`)

	raw, err := sql.Open("sqlite", fileURI(path))
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	problems, err := IntegrityProblems(ctx, raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) == 0 {
		t.Fatal("IntegrityProblems found nothing in a damaged index")
	}
	for _, p := range problems {
		if p != "integrity_check: problem in index damaged_idx" && p != "integrity_check: problem in table investigations" {
			t.Errorf("problem %q is not sanitized", p)
		}
		if strings.Contains(p, "Mixed") || strings.Contains(p, "row ") || strings.Contains(p, "missing") {
			t.Errorf("problem %q leaks raw text", p)
		}
	}
}
