package cli

import (
	"github.com/jameskbb/casebook/internal/app"
	"testing"
)

func TestConclusionCLIReviewToken(t *testing.T) {
	_, db, _ := seededApp(t)
	r := runCLI(t, "", "--db", db, "--json", "investigation", "conclude", "--text", "Costs drove the increase.", "--supporting-claims", "C-001", "--contradicting-claims", "C-003")
	wantCode(t, r, 0)
	var saved app.Investigation
	decodeJSON(t, r.Stdout, &saved)
	if saved.ConclusionVerification != app.VerificationHumanVerified || !saved.ConclusionNeedsReview || saved.ConclusionReviewToken == "" {
		t.Fatalf("save: %+v", saved)
	}
	r = runCLI(t, "", "--db", db, "investigation", "conclude", "--review", "--review-basis", "Checked the billing export.", "--expected-review-token", "stale")
	wantCode(t, r, 1)
	r = runCLI(t, "", "--db", db, "--json", "investigation", "conclude", "--review", "--review-basis", "Checked the billing export.", "--expected-review-token", saved.ConclusionReviewToken)
	wantCode(t, r, 0)
	var reviewed app.Investigation
	decodeJSON(t, r.Stdout, &reviewed)
	if reviewed.ConclusionNeedsReview || reviewed.ConclusionReviewedAt == "" || reviewed.ConclusionReviewBasis != "Checked the billing export." {
		t.Fatalf("review: %+v", reviewed)
	}
	r = runCLI(t, "", "--db", db, "investigation", "show")
	wantCode(t, r, 0)
	wantContains(t, r.Stdout, "Human verified", "C-001", "C-003", "Checked the billing export.")
}
