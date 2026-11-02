package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestMigration0008ConclusionAccountability(t *testing.T) {
	ctx := context.Background()
	w := rawDB(t, filepath.Join(t.TempDir(), "casebook.db"))
	if v, err := Migrate(ctx, w, migrationsUpTo(t, 7), nil); err != nil || v != 7 {
		t.Fatalf("schema7: %d %v", v, err)
	}
	const now = "2026-10-01T00:00:00.000Z"
	if _, err := w.ExecContext(ctx, `INSERT INTO investigations (ref_num,title,conclusion,conclusion_confidence,conclusion_rationale,created_at,updated_at,created_by_kind,created_by_client) VALUES (1,'Margin','Costs rose.','high','Billing export.',?,?,'human','desktop')`, now, now); err != nil {
		t.Fatal(err)
	}
	if v, err := Migrate(ctx, w, migrationsUpTo(t, 8), nil); err != nil || v != 8 {
		t.Fatalf("schema8: %d %v", v, err)
	}
	inv, err := GetInvestigationByNum(ctx, w, 1)
	if err != nil {
		t.Fatal(err)
	}
	if inv.Conclusion != "Costs rose." || inv.ConclusionRationale != "Billing export." || inv.ConclusionConfidence != "high" || inv.UpdatedAt != now || inv.CreatedAt != now {
		t.Fatalf("legacy text changed: %+v", inv)
	}
	if inv.ConclusionVerification != "ai_generated" || inv.ConclusionReviewedAt != "" || inv.ConclusionReviewBasis != "" || len(inv.ConclusionSupportingClaims) != 0 || len(inv.ConclusionContradictingClaims) != 0 {
		t.Fatalf("inferred an unrecorded review: %+v", inv)
	}
	hash, err := ConclusionReviewSnapshot(ctx, w, inv.ID)
	if err != nil || hash != "" {
		t.Fatalf("snapshot: %q %v", hash, err)
	}
	for _, verification := range []string{"ai_generated", "human_verified", "source_verified", "disputed"} {
		if _, err := w.ExecContext(ctx, `UPDATE investigations SET conclusion_verification=?`, verification); err != nil {
			t.Fatalf("verification %s: %v", verification, err)
		}
	}
	for _, query := range []string{`UPDATE investigations SET conclusion_verification='superseded'`, `UPDATE investigations SET conclusion_supporting_claims='{}'`, `UPDATE investigations SET conclusion_contradicting_claims='not-json'`} {
		if _, err := w.ExecContext(ctx, query); err == nil || !isConstraintErr(err) {
			t.Fatalf("invalid schema8 data accepted: %v", err)
		}
	}
	if v, err := Migrate(ctx, w, migrationsUpTo(t, 8), nil); err != nil || v != 8 {
		t.Fatalf("repeat schema8: %d %v", v, err)
	}
	inv, err = GetInvestigationByNum(ctx, w, 1)
	if err != nil || inv.ConclusionVerification != "disputed" {
		t.Fatalf("migration ran twice: %+v %v", inv, err)
	}
}
