package store

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/jameskbb/casebook/internal/domain"
)

// TestEvidenceLinks seeds claims, hypotheses and links with raw SQL (the app
// methods that create them arrive in a later plan) and checks GetEvidence and
// ListEvidence report them ordered by ref with relations, and non-nil empty
// slices otherwise.
func TestEvidenceLinks(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	const ts = "2026-01-15T09:00:00.000Z"
	actor := domain.Actor{Kind: domain.ActorHuman, Client: "test"}
	err := db.WithWriteTx(ctx, func(tx *sql.Tx) error {
		inv, err := InsertInvestigation(ctx, tx, NewInvestigation{RefNum: 1, Title: "T", Status: domain.InvestigationStatusNew, CreatedAt: ts, CreatedBy: actor})
		if err != nil {
			return err
		}
		for n := int64(1); n <= 2; n++ {
			if _, err := InsertEvidence(ctx, tx, NewEvidence{InvestigationID: inv, RefNum: n, Title: "E", Summary: "S",
				SourceType: domain.SourceTypeCode, CapturedAt: ts, CreatedAt: ts, CreatedBy: actor,
				Verification: domain.VerificationHumanVerified}); err != nil {
				return err
			}
		}
		for _, stmt := range []string{
			`INSERT INTO claims (id, investigation_id, ref_num, statement, verification, created_at, updated_at, created_by_kind, created_by_client)
			 VALUES (10, 1, 2, 'c2', 'ai_generated', '` + ts + `', '` + ts + `', 'ai', 'x'),
			        (11, 1, 1, 'c1', 'ai_generated', '` + ts + `', '` + ts + `', 'ai', 'x')`,
			`INSERT INTO hypotheses (id, investigation_id, ref_num, statement, verification, created_at, updated_at, created_by_kind, created_by_client)
			 VALUES (20, 1, 1, 'h1', 'ai_generated', '` + ts + `', '` + ts + `', 'ai', 'x')`,
			`INSERT INTO claim_evidence (claim_id, evidence_id, relation, created_at, created_by_kind, created_by_client)
			 VALUES (10, 1, 'contradicts', '` + ts + `', 'ai', 'x'), (11, 1, 'supports', '` + ts + `', 'ai', 'x')`,
			`INSERT INTO hypothesis_evidence (hypothesis_id, evidence_id, relation, created_at, created_by_kind, created_by_client)
			 VALUES (20, 1, 'supports', '` + ts + `', 'ai', 'x')`,
		} {
			if _, err := tx.ExecContext(ctx, stmt); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	err = db.WithReadTx(ctx, func(tx *sql.Tx) error {
		e1, err := GetEvidence(ctx, tx, 1, 1, false)
		if err != nil {
			return err
		}
		want := []domain.RecordLink{{Ref: "C-001", Relation: domain.RelationSupports}, {Ref: "C-002", Relation: domain.RelationContradicts}}
		if len(e1.ClaimLinks) != 2 || e1.ClaimLinks[0] != want[0] || e1.ClaimLinks[1] != want[1] {
			t.Errorf("E-001 claim links = %+v, want %+v", e1.ClaimLinks, want)
		}
		if len(e1.HypothesisLinks) != 1 || e1.HypothesisLinks[0] != (domain.RecordLink{Ref: "H-001", Relation: domain.RelationSupports}) {
			t.Errorf("E-001 hypothesis links = %+v", e1.HypothesisLinks)
		}
		list, err := ListEvidence(ctx, tx, 1)
		if err != nil {
			return err
		}
		if len(list) != 2 || len(list[0].ClaimLinks) != 2 || len(list[0].HypothesisLinks) != 1 {
			t.Errorf("list links = %+v", list)
		}
		if list[1].ClaimLinks == nil || list[1].HypothesisLinks == nil || len(list[1].ClaimLinks) != 0 {
			t.Errorf("E-002 links = %#v, %#v; want non-nil empty", list[1].ClaimLinks, list[1].HypothesisLinks)
		}
		if _, err := GetEvidence(ctx, tx, 1, 9, false); !errors.Is(err, sql.ErrNoRows) {
			t.Errorf("missing evidence err = %v, want sql.ErrNoRows", err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestUpdateEvidenceColumnWhitelist(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	err := db.WithWriteTx(ctx, func(tx *sql.Tx) error {
		return UpdateEvidence(ctx, tx, 1, []Assignment{{Column: "created_by_kind", Value: "ai"}}, "x")
	})
	if err == nil {
		t.Fatal("non-whitelisted column accepted")
	}
}
