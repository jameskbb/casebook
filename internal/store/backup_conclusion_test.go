package store

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateSnapshotChecksConclusionAccountability(t *testing.T) {
	good, err := os.ReadFile(snapshotOf(t, seededDB(t)))
	if err != nil {
		t.Fatal(err)
	}
	for name, query := range map[string]string{
		"oversized review basis":  `UPDATE investigations SET conclusion_review_basis = '` + strings.Repeat("x", 2001) + `'`,
		"NUL in review basis":     `UPDATE investigations SET conclusion_review_basis = 'a' || char(0) || 'b'`,
		"missing selected claim":  `UPDATE investigations SET conclusion = 'Answer', conclusion_supporting_claims = '["C-001"]'`,
		"non-text selected claim": `UPDATE investigations SET conclusion = 'Answer', conclusion_supporting_claims = '[1]'`,
		"unattributed review":     `UPDATE investigations SET conclusion = 'Answer', conclusion_reviewed_at = '2026-10-23T12:00:00.000Z'`,
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "bad.db")
			if err := os.WriteFile(path, good, 0o600); err != nil {
				t.Fatal(err)
			}
			tamper(t, path, query)
			if _, err := validateFile(t, path); !errors.Is(err, ErrInvalidSnapshot) {
				t.Fatalf("invalid conclusion snapshot accepted: %v", err)
			}
		})
	}
}

func TestValidateSnapshotStillAcceptsSchema7(t *testing.T) {
	ctx := context.Background()
	w := rawDB(t, filepath.Join(t.TempDir(), "old.db"))
	if v, err := Migrate(ctx, w, migrationsUpTo(t, 7), nil); err != nil || v != 7 {
		t.Fatalf("migrate to 7: %d, %v", v, err)
	}
	path := filepath.Join(t.TempDir(), "schema7.db")
	if _, err := w.ExecContext(ctx, "VACUUM INTO ?", path); err != nil {
		t.Fatal(err)
	}
	if v, err := validateFile(t, path); err != nil || v != 7 {
		t.Fatalf("schema 7 validation: %d, %v", v, err)
	}
}
