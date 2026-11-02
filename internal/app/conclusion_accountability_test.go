package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/app/apptest"
)

func conclusionFixture(t *testing.T) (*app.App, app.Investigation) {
	t.Helper()
	a := apptest.Open(t)
	inv := newInv(t, a, "Why did margin fall?")
	e, err := a.AddEvidence(context.Background(), apptest.AI, inv.Ref, app.EvidenceInput{Title: "Billing export", Summary: "Costs rose.", SourceType: app.SourceTypeSQLQuery})
	if err != nil {
		t.Fatal(err)
	}
	_, err = a.AddClaim(context.Background(), apptest.AI, inv.Ref, app.ClaimInput{Statement: "Costs drove the decline.", Links: []app.EvidenceLinkInput{{EvidenceRef: e.Ref, Relation: app.RelationSupports}}})
	if err != nil {
		t.Fatal(err)
	}
	inv, err = a.UpdateConclusion(context.Background(), apptest.Human, inv.Ref, app.ConclusionInput{Conclusion: "Supplier costs explain the decline.", Confidence: app.ConfidenceHigh, Rationale: "The export reconciles.", SupportingClaims: []string{"c-1", "C-001"}})
	if err != nil {
		t.Fatal(err)
	}
	return a, inv
}

func reviewConclusion(t *testing.T, a *app.App, inv app.Investigation) app.Investigation {
	t.Helper()
	got, err := a.UpdateConclusion(context.Background(), apptest.Human, inv.Ref, app.ConclusionInput{Conclusion: inv.Conclusion, Confidence: inv.ConclusionConfidence, Rationale: inv.ConclusionRationale, Verification: app.VerificationHumanVerified, Review: true, ReviewBasis: "Checked C-001 against the original billing export.", ExpectedReviewToken: inv.ConclusionReviewToken})
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestConclusionReviewTracksMaterialBasisWithoutRevokingStatement(t *testing.T) {
	ctx := context.Background()
	a, inv := conclusionFixture(t)
	if inv.ConclusionVerification != app.VerificationHumanVerified || !inv.ConclusionNeedsReview || inv.ConclusionReviewToken == "" || !reflect.DeepEqual(inv.ConclusionSupportingClaims, []string{"C-001"}) {
		t.Fatalf("unreviewed: %+v", inv)
	}
	inv = reviewConclusion(t, a, inv)
	if inv.ConclusionNeedsReview || inv.ConclusionReviewedAt == "" || inv.ConclusionReviewBasis == "" {
		t.Fatalf("reviewed: %+v", inv)
	}
	// An unrelated finding does not alter the selected review basis.
	if _, err := a.AddClaim(ctx, apptest.AI, inv.Ref, app.ClaimInput{Statement: "An unrelated market changed."}); err != nil {
		t.Fatal(err)
	}
	unchanged, err := a.GetInvestigation(ctx, inv.Ref)
	if err != nil {
		t.Fatal(err)
	}
	if unchanged.ConclusionNeedsReview || unchanged.ConclusionReviewToken != inv.ConclusionReviewToken {
		t.Fatal("unrelated claim invalidated selected review")
	}
	if _, err := a.UpdateClaim(ctx, apptest.AI, inv.Ref, "C-001", app.ClaimPatch{Status: ptr(app.ClaimStatusRejected)}); err != nil {
		t.Fatal(err)
	}
	changed, err := a.GetInvestigation(ctx, inv.Ref)
	if err != nil {
		t.Fatal(err)
	}
	if !changed.ConclusionNeedsReview || changed.ConclusionVerification != app.VerificationHumanVerified || changed.ConclusionReviewedAt != inv.ConclusionReviewedAt || changed.ConclusionReviewBasis != inv.ConclusionReviewBasis {
		t.Fatalf("support change erased historical review or trust: %+v", changed)
	}
	if _, err := a.UpdateConclusion(ctx, apptest.Human, inv.Ref, app.ConclusionInput{Conclusion: inv.Conclusion, Confidence: inv.ConclusionConfidence, Rationale: inv.ConclusionRationale, Review: true, ReviewBasis: "Old preview", ExpectedReviewToken: inv.ConclusionReviewToken}); !errors.Is(err, app.ErrConflict) {
		t.Fatalf("stale review: %v", err)
	}
	refreshed := reviewConclusion(t, a, changed)
	if refreshed.ConclusionNeedsReview {
		t.Fatal("fresh explicit review did not acknowledge current basis")
	}
	if _, err := a.UpdateEvidence(ctx, apptest.AI, inv.Ref, "E-001", app.EvidencePatch{Summary: ptr("Costs were restated.")}); err != nil {
		t.Fatal(err)
	}
	changed, err = a.GetInvestigation(ctx, inv.Ref)
	if err != nil {
		t.Fatal(err)
	}
	if !changed.ConclusionNeedsReview || changed.ConclusionVerification != app.VerificationHumanVerified {
		t.Fatalf("linked observation edit: %+v", changed)
	}
}

func TestConclusionVerifierAndSubstantiveEdit(t *testing.T) {
	ctx := context.Background()
	a, inv := conclusionFixture(t)
	inv = reviewConclusion(t, a, inv)
	before, _ := a.ListActivity(ctx, app.ActivityFilter{InvestigationRef: inv.Ref})
	for _, actor := range []app.Actor{apptest.AI, {Kind: app.ActorImport, Client: "test"}, {Kind: app.ActorSystem, Client: "test"}} {
		_, err := a.UpdateConclusion(ctx, actor, inv.Ref, app.ConclusionInput{Conclusion: inv.Conclusion, Verification: app.VerificationHumanVerified})
		wantValidation(t, err, "verification")
		_, err = a.UpdateConclusion(ctx, actor, inv.Ref, app.ConclusionInput{Conclusion: inv.Conclusion, Review: true, ReviewBasis: "checked", ExpectedReviewToken: inv.ConclusionReviewToken})
		wantValidation(t, err, "review")
	}
	after, _ := a.ListActivity(ctx, app.ActivityFilter{InvestigationRef: inv.Ref})
	if len(after) != len(before) {
		t.Fatal("refused writes recorded activity")
	}
	got, err := a.UpdateConclusion(ctx, apptest.AI, inv.Ref, app.ConclusionInput{Conclusion: "A materially different answer.", Confidence: app.ConfidenceLow})
	if err != nil {
		t.Fatal(err)
	}
	if got.ConclusionVerification != app.VerificationAIGenerated || got.ConclusionReviewedAt != "" || got.ConclusionReviewBasis != "" || !got.ConclusionNeedsReview {
		t.Fatalf("AI edit retained old review: %+v", got)
	}
	acts, _ := a.ListActivity(ctx, app.ActivityFilter{InvestigationRef: inv.Ref, Limit: 1})
	if len(acts) != 1 || acts[0].Summary == "" {
		t.Fatal("missing edit activity")
	}
	got, err = a.UpdateConclusion(ctx, apptest.AI, inv.Ref, app.ConclusionInput{Conclusion: "Rechecked answer.", Verification: app.VerificationSourceVerified})
	if err != nil || got.ConclusionVerification != app.VerificationSourceVerified {
		t.Fatalf("explicit source check: %+v %v", got, err)
	}
}

func TestConclusionSelectionValidationAndDeleteGuard(t *testing.T) {
	ctx := context.Background()
	a, inv := conclusionFixture(t)
	for _, tc := range []struct {
		supporting, contradicting []string
		field                     string
	}{{[]string{"INV-0002/C-001"}, nil, "supporting_claims"}, {[]string{"E-001"}, nil, "supporting_claims[0]"}, {[]string{"C-999"}, nil, "supporting_claims"}, {[]string{"C-001"}, []string{"c-1"}, "contradicting_claims"}} {
		_, err := a.UpdateConclusion(ctx, apptest.Human, inv.Ref, app.ConclusionInput{Conclusion: inv.Conclusion, SupportingClaims: tc.supporting, ContradictingClaims: tc.contradicting})
		wantValidation(t, err, tc.field)
	}
	if err := a.DeleteClaim(ctx, apptest.Human, inv.Ref, "C-001"); !errors.Is(err, app.ErrConflict) {
		t.Fatalf("selected claim delete: %v", err)
	}
	inv = reviewConclusion(t, a, inv)
	_, err := a.UpdateConclusion(ctx, apptest.Human, inv.Ref, app.ConclusionInput{Conclusion: inv.Conclusion, Confidence: inv.ConclusionConfidence, Rationale: inv.ConclusionRationale, SupportingClaims: []string{}, Review: true, ReviewBasis: "unchecked selection", ExpectedReviewToken: inv.ConclusionReviewToken})
	wantValidation(t, err, "review")
	got, err := a.UpdateConclusion(ctx, apptest.Human, inv.Ref, app.ConclusionInput{Conclusion: inv.Conclusion, SupportingClaims: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.ConclusionSupportingClaims) != 0 || got.ConclusionReviewedAt != "" {
		t.Fatal("selection change retained old review")
	}
	if err := a.DeleteClaim(ctx, apptest.Human, inv.Ref, "C-001"); err != nil {
		t.Fatal(err)
	}
}

func TestConclusionCheckpointRefusalIsAtomic(t *testing.T) {
	ctx := context.Background()
	a, inv := conclusionFixture(t)
	_, err := a.CheckpointBatch(ctx, apptest.AI, inv.Ref, app.CheckpointInput{Note: "batch", Claims: []app.ClaimInput{{Statement: "Should not persist."}}, Conclusion: &app.ConclusionInput{Conclusion: "AI cannot vouch.", Verification: app.VerificationHumanVerified}})
	if err == nil {
		t.Fatal("AI checkpoint human verification accepted")
	}
	claims, err := a.ListClaims(ctx, inv.Ref)
	if err != nil || len(claims) != 1 {
		t.Fatalf("non-atomic refusal: %v %v", claims, err)
	}
}

func TestConclusionExportImportLowersTrustAndAcceptsV1(t *testing.T) {
	ctx := context.Background()
	a, inv := conclusionFixture(t)
	inv = reviewConclusion(t, a, inv)
	data, err := a.ExportInvestigation(ctx, inv.Ref)
	if err != nil {
		t.Fatal(err)
	}
	var doc app.ExportDocument
	if err = json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Version != 2 || doc.Investigation.ConclusionReviewedAt == "" || doc.Investigation.ConclusionVerification != app.VerificationHumanVerified {
		t.Fatalf("v2 export: %+v", doc.Investigation)
	}
	dst := apptest.Open(t)
	result, err := dst.ImportInvestigation(ctx, apptest.Human, data)
	if err != nil {
		t.Fatal(err)
	}
	got, err := dst.GetInvestigation(ctx, result.InvestigationRef)
	if err != nil {
		t.Fatal(err)
	}
	if got.ConclusionVerification != app.VerificationAIGenerated || got.ConclusionReviewedAt != "" || got.ConclusionReviewBasis != "" || !got.ConclusionNeedsReview || !reflect.DeepEqual(got.ConclusionSupportingClaims, inv.ConclusionSupportingClaims) {
		t.Fatalf("trusted imported review: %+v", got)
	}
	// A legacy file lacks all accountability fields and still imports.
	var legacy map[string]any
	json.Unmarshal(data, &legacy)
	legacy["version"] = 1
	header := legacy["investigation"].(map[string]any)
	for _, key := range []string{"conclusion_verification", "conclusion_supporting_claims", "conclusion_contradicting_claims", "conclusion_reviewed_at", "conclusion_review_basis", "conclusion_needs_review"} {
		delete(header, key)
	}
	old, _ := json.Marshal(legacy)
	if _, err := dst.ImportInvestigation(ctx, apptest.Human, old); err != nil {
		t.Fatalf("v1 import: %v", err)
	}
}

func TestConclusionBackupRestoresReview(t *testing.T) {
	ctx := context.Background()
	a, inv := conclusionFixture(t)
	inv = reviewConclusion(t, a, inv)
	backup := filepath.Join(t.TempDir(), "casebook.db")
	if _, err := a.Backup(ctx, backup); err != nil {
		t.Fatal(err)
	}
	restored, err := app.Open(backup)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	got, err := restored.GetInvestigation(ctx, inv.Ref)
	if err != nil {
		t.Fatal(err)
	}
	if got.ConclusionNeedsReview || got.ConclusionReviewedAt != inv.ConclusionReviewedAt || got.ConclusionReviewBasis != inv.ConclusionReviewBasis || got.ConclusionReviewToken != inv.ConclusionReviewToken {
		t.Fatalf("backup lost review: %+v", got)
	}
}
