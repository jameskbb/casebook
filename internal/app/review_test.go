package app_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/app/apptest"
)

func reviewRow(t *testing.T, a *app.App, invRef, ref string) app.ReviewFinding {
	t.Helper()
	rows, err := a.GetReviewFindings(context.Background(), invRef)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if r.Ref == ref {
			return r
		}
	}
	t.Fatalf("no review finding %s", ref)
	return app.ReviewFinding{}
}

func TestReviewStatementRejectsStaleMaterial(t *testing.T) {
	ctx := context.Background()
	changes := map[string]func(*app.App, string, string) error{
		"statement": func(a *app.App, inv, ref string) error {
			value := "Changed by another AI"
			_, err := a.UpdateClaim(ctx, apptest.AI, inv, ref, app.ClaimPatch{Statement: &value})
			return err
		},
		"source summary": func(a *app.App, inv, ref string) error {
			value := "Changed observation"
			_, err := a.UpdateEvidence(ctx, apptest.AI, inv, "E-001", app.EvidencePatch{Summary: &value})
			return err
		},
		"source raw": func(a *app.App, inv, ref string) error {
			value := "Changed raw material"
			_, err := a.UpdateEvidence(ctx, apptest.AI, inv, "E-001", app.EvidencePatch{RawDetail: &value})
			return err
		},
		"source verification": func(a *app.App, inv, ref string) error {
			_, err := a.SetEvidenceVerification(ctx, apptest.AI, inv, "E-001", app.VerificationDisputed)
			return err
		},
		"link relation": func(a *app.App, inv, ref string) error {
			_, err := a.LinkEvidenceToClaim(ctx, apptest.AI, inv, ref, "E-001", app.RelationContradicts)
			return err
		},
		"unlink": func(a *app.App, inv, ref string) error {
			_, err := a.UnlinkEvidenceFromClaim(ctx, apptest.AI, inv, ref, "E-001")
			return err
		},
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			a, inv := claimEnv(t)
			c := mustAddClaim(t, a, apptest.AI, inv.Ref, app.ClaimInput{Statement: "Original finding", Links: []app.EvidenceLinkInput{{EvidenceRef: "E-001", Relation: app.RelationSupports}}})
			r := reviewRow(t, a, inv.Ref, c.Ref)
			// A separate handle with the same fixed clock proves content, not clock precision, guards the write.
			other := apptest.Reopen(t, a.DBPath())
			if err := change(other, inv.Ref, c.Ref); err != nil {
				t.Fatal(err)
			}
			before := activityCount(t, a, inv.Ref)
			_, err := a.ReviewStatement(ctx, apptest.Human, inv.Ref, app.ReviewStatementInput{Kind: app.RecordClaim, Ref: c.Ref, Basis: r.Basis, Verification: app.VerificationHumanVerified})
			if !errors.Is(err, app.ErrConflict) {
				t.Fatalf("stale review: %v", err)
			}
			if activityCount(t, a, inv.Ref) != before {
				t.Fatal("stale review wrote activity")
			}
			current, _ := a.GetClaim(ctx, inv.Ref, c.Ref)
			if current.Verification == app.VerificationHumanVerified {
				t.Fatal("stale finding was verified")
			}
		})
	}
}

func TestReviewFiltersAndExplicitDisputeSettlement(t *testing.T) {
	ctx := context.Background()
	a, inv := claimEnv(t)
	c := mustAddClaim(t, a, apptest.AI, inv.Ref, app.ClaimInput{Statement: "Source checked", Verification: app.VerificationSourceVerified, Links: []app.EvidenceLinkInput{{EvidenceRef: "E-001", Relation: app.RelationSupports}}})
	_, err := a.SetEvidenceVerification(ctx, apptest.AI, inv.Ref, "E-001", app.VerificationDisputed)
	if err != nil {
		t.Fatal(err)
	}
	r := reviewRow(t, a, inv.Ref, c.Ref)
	if !slices.Contains(r.Filters, "source_checked") || !slices.Contains(r.Filters, "support_attention") {
		t.Fatalf("filters: %v", r.Filters)
	}
	detail, err := a.GetInvestigationDetail(ctx, inv.Ref, app.DetailOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.ReviewBases) != 1 || detail.ReviewBases[0].Basis != r.Basis {
		t.Fatalf("detail guard does not match review: %+v", detail.ReviewBases)
	}
	in := app.ReviewStatementInput{Kind: app.RecordClaim, Ref: c.Ref, Basis: r.Basis, Verification: app.VerificationHumanVerified}
	if _, err = a.ReviewStatement(ctx, apptest.AI, inv.Ref, in); err == nil {
		t.Fatal("AI reviewed as a person")
	}
	verified, err := a.ReviewStatement(ctx, apptest.Human, inv.Ref, in)
	if err != nil || verified.Verification != app.VerificationHumanVerified {
		t.Fatalf("source check to person check: %+v %v", verified, err)
	}
	// Disputed support stays disputed and the historical statement check stays truthful.
	e, _ := a.GetEvidence(ctx, inv.Ref, "E-001", app.GetEvidenceOptions{})
	if e.Verification != app.VerificationDisputed {
		t.Fatal("review settled evidence dispute")
	}
	in.Basis = verified.Basis
	in.Verification = app.VerificationDisputed
	disputed, err := a.ReviewStatement(ctx, apptest.Human, inv.Ref, in)
	if err != nil {
		t.Fatal(err)
	}
	in.Basis = disputed.Basis
	in.Verification = app.VerificationHumanVerified
	if _, err = a.ReviewStatement(ctx, apptest.Human, inv.Ref, in); err == nil {
		t.Fatal("dispute settled without confirmation")
	}
	in.SettleDispute = true
	settled, err := a.ReviewStatement(ctx, apptest.Human, inv.Ref, in)
	if err != nil || settled.Status != "active" {
		t.Fatalf("explicit settlement: %+v %v", settled, err)
	}
}

func TestReviewExcludesRetiredPreservesArchived(t *testing.T) {
	ctx := context.Background()
	a, inv := claimEnv(t)
	c := mustAddClaim(t, a, apptest.AI, inv.Ref, app.ClaimInput{Statement: "Rejected", Status: app.ClaimStatusRejected})
	_, err := a.AddHypothesis(ctx, apptest.AI, inv.Ref, app.HypothesisInput{Statement: "Disproven", Status: app.HypothesisStatusDisproven})
	if err != nil {
		t.Fatal(err)
	}
	current := mustAddClaim(t, a, apptest.AI, inv.Ref, app.ClaimInput{Statement: "Current"})
	old := mustAddClaim(t, a, apptest.AI, inv.Ref, app.ClaimInput{Statement: "Old"})
	if _, err = a.SupersedeClaim(ctx, apptest.Human, inv.Ref, old.Ref, current.Ref); err != nil {
		t.Fatal(err)
	}
	if _, err = a.ArchiveInvestigation(ctx, apptest.Human, inv.Ref); err != nil {
		t.Fatal(err)
	}
	rows, err := a.GetReviewFindings(ctx, inv.Ref)
	if err != nil || len(rows) != 1 || rows[0].Ref != current.Ref {
		t.Fatalf("current queue: %+v %v", rows, err)
	}
	_, err = a.ReviewStatement(ctx, apptest.Human, inv.Ref, app.ReviewStatementInput{Kind: app.RecordClaim, Ref: c.Ref, Basis: "anything", Verification: app.VerificationHumanVerified})
	if !errors.Is(err, app.ErrConflict) {
		t.Fatalf("retired review: %v", err)
	}
	if _, err = a.ReviewStatement(ctx, apptest.Human, inv.Ref, app.ReviewStatementInput{Kind: app.RecordClaim, Ref: current.Ref, Basis: rows[0].Basis, Verification: app.VerificationHumanVerified}); err != nil {
		t.Fatal(err)
	}
	after, _ := a.GetInvestigation(ctx, inv.Ref)
	if !after.Archived {
		t.Fatal("review unarchived the case")
	}
}
