package app_test

import (
	"context"
	"testing"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/app/apptest"
)

// TestClaimStatusVerificationInvariant pins ADR-0013: a claim's status and
// verification never disagree about disputed or superseded, whichever
// method writes them.
func TestClaimStatusVerificationInvariant(t *testing.T) {
	ctx := context.Background()
	a, _ := claimEnv(t)
	mustAddClaim(t, a, apptest.AI, "", app.ClaimInput{Statement: "One"})
	mustAddClaim(t, a, apptest.AI, "", app.ClaimInput{Statement: "Two"})

	want := func(c app.Claim, err error, status app.ClaimStatus, v app.Verification) {
		t.Helper()
		if err != nil || c.Status != status || c.Verification != v {
			t.Fatalf("claim = %s/%s, %v; want %s/%s", c.Status, c.Verification, err, status, v)
		}
	}

	// Verification disputed alone disputes the claim.
	c, err := a.SetClaimVerification(ctx, apptest.Human, "", "C-001", app.VerificationDisputed)
	want(c, err, app.ClaimStatusDisputed, app.VerificationDisputed)
	// Verifying a disputed claim makes it active again.
	c, err = a.SetClaimVerification(ctx, apptest.Human, "", "C-001", app.VerificationHumanVerified)
	want(c, err, app.ClaimStatusActive, app.VerificationHumanVerified)
	// Status disputed alone sets verification disputed.
	c, err = a.UpdateClaim(ctx, apptest.Human, "", "C-001", app.ClaimPatch{Status: ptr(app.ClaimStatusDisputed)})
	want(c, err, app.ClaimStatusDisputed, app.VerificationDisputed)
	// Leaving disputed without a verification resets to the actor default.
	c, err = a.UpdateClaim(ctx, apptest.AI, "", "C-001", app.ClaimPatch{Status: ptr(app.ClaimStatusActive)})
	want(c, err, app.ClaimStatusActive, app.VerificationAIGenerated)
	// Verification disputed through a patch disputes too.
	c, err = a.UpdateClaim(ctx, apptest.AI, "", "C-001", app.ClaimPatch{Verification: ptr(app.VerificationDisputed)})
	want(c, err, app.ClaimStatusDisputed, app.VerificationDisputed)
	c, err = a.UpdateClaim(ctx, apptest.Human, "", "C-001", app.ClaimPatch{Verification: ptr(app.VerificationSourceVerified)})
	want(c, err, app.ClaimStatusActive, app.VerificationSourceVerified)

	// Contradictory pairs are refused on verification.
	for _, p := range []app.ClaimPatch{
		{Status: ptr(app.ClaimStatusActive), Verification: ptr(app.VerificationDisputed)},
		{Status: ptr(app.ClaimStatusDisputed), Verification: ptr(app.VerificationHumanVerified)},
		{Status: ptr(app.ClaimStatusRejected), Verification: ptr(app.VerificationDisputed)},
		{Verification: ptr(app.VerificationSuperseded)},
		{SupersededBy: ptr("C-002"), Verification: ptr(app.VerificationHumanVerified)},
	} {
		_, err := a.UpdateClaim(ctx, apptest.Human, "", "C-001", p)
		wantValidation(t, err, "verification")
	}
	_, err = a.SetClaimVerification(ctx, apptest.Human, "", "C-001", app.VerificationSuperseded)
	wantValidation(t, err, "verification")
	for _, in := range []app.ClaimInput{
		{Statement: "x", Verification: app.VerificationSuperseded},
		{Statement: "x", Status: app.ClaimStatusActive, Verification: app.VerificationDisputed},
		{Statement: "x", Status: app.ClaimStatusDisputed, Verification: app.VerificationHumanVerified},
	} {
		_, err := a.AddClaim(ctx, apptest.AI, "", in)
		wantValidation(t, err, "verification")
	}
	// Add infers the other half.
	c, err = a.AddClaim(ctx, apptest.AI, "", app.ClaimInput{Statement: "Three", Verification: app.VerificationDisputed})
	want(c, err, app.ClaimStatusDisputed, app.VerificationDisputed)
	c, err = a.AddClaim(ctx, apptest.Human, "", app.ClaimInput{Statement: "Four", Status: app.ClaimStatusDisputed})
	want(c, err, app.ClaimStatusDisputed, app.VerificationDisputed)

	// Superseding sets both; a verification alone on a superseded claim is a
	// conflict; reinstating with a status resets verification.
	c, err = a.SupersedeClaim(ctx, apptest.Human, "", "C-001", "C-002")
	want(c, err, app.ClaimStatusSuperseded, app.VerificationSuperseded)
	_, err = a.SetClaimVerification(ctx, apptest.Human, "", "C-001", app.VerificationHumanVerified)
	wantErrIs(t, err, app.ErrConflict)
	_, err = a.UpdateClaim(ctx, apptest.Human, "", "C-001", app.ClaimPatch{Verification: ptr(app.VerificationDisputed)})
	wantErrIs(t, err, app.ErrConflict)
	c, err = a.UpdateClaim(ctx, apptest.Human, "", "C-001", app.ClaimPatch{Status: ptr(app.ClaimStatusDisputed)})
	want(c, err, app.ClaimStatusDisputed, app.VerificationDisputed)
	if c.SupersededBy != "" {
		t.Fatalf("superseded_by kept after reinstating: %+v", c)
	}
}

// TestHypothesisStatusVerificationInvariant: hypotheses pair only
// superseded; disputed is a free verification state.
func TestHypothesisStatusVerificationInvariant(t *testing.T) {
	ctx := context.Background()
	a, _ := claimEnv(t)
	for _, s := range []string{"One", "Two"} {
		if _, err := a.AddHypothesis(ctx, apptest.AI, "", app.HypothesisInput{Statement: s}); err != nil {
			t.Fatal(err)
		}
	}
	h, err := a.SetHypothesisVerification(ctx, apptest.Human, "", "H-001", app.VerificationDisputed)
	if err != nil || h.Status != app.HypothesisStatusOpen || h.Verification != app.VerificationDisputed {
		t.Fatalf("disputed hypothesis = %+v, %v", h, err)
	}
	_, err = a.SetHypothesisVerification(ctx, apptest.Human, "", "H-001", app.VerificationSuperseded)
	wantValidation(t, err, "verification")
	_, err = a.AddHypothesis(ctx, apptest.AI, "", app.HypothesisInput{Statement: "x", Verification: app.VerificationSuperseded})
	wantValidation(t, err, "verification")
	h, err = a.SupersedeHypothesis(ctx, apptest.Human, "", "H-001", "H-002")
	if err != nil || h.Verification != app.VerificationSuperseded {
		t.Fatalf("superseded hypothesis = %+v, %v", h, err)
	}
	_, err = a.SetHypothesisVerification(ctx, apptest.Human, "", "H-001", app.VerificationHumanVerified)
	wantErrIs(t, err, app.ErrConflict)
	h, err = a.UpdateHypothesis(ctx, apptest.Human, "", "H-001", app.HypothesisPatch{Status: ptr(app.HypothesisStatusDisproven)})
	if err != nil || h.Verification != app.VerificationHumanVerified || h.SupersededBy != "" {
		t.Fatalf("reinstated hypothesis = %+v, %v", h, err)
	}
}

// TestImportRepairsStatusVerification: an export written before ADR-0013
// may hold a disagreeing pair; import repairs it like migration 0002.
func TestImportRepairsStatusVerification(t *testing.T) {
	ctx := context.Background()
	src := apptest.Open(t)
	apptest.SeedCloudSpend(t, src)
	doc, _ := exportDoc(t, src, "")
	doc.Claims[0].Status, doc.Claims[0].Verification = app.ClaimStatusActive, app.VerificationDisputed
	doc.Claims[1].Status, doc.Claims[1].Verification = app.ClaimStatusActive, app.VerificationSuperseded
	doc.Hypotheses[0].Verification = app.VerificationSuperseded

	dst := apptest.Open(t)
	if _, err := dst.ImportInvestigation(ctx, apptest.Human, marshalDoc(t, doc)); err != nil {
		t.Fatal(err)
	}
	c1, _ := dst.GetClaim(ctx, "INV-0001", "C-001")
	c2, _ := dst.GetClaim(ctx, "INV-0001", "C-002")
	h1, _ := dst.GetHypothesis(ctx, "INV-0001", "H-001")
	if c1.Status != app.ClaimStatusDisputed || c1.Verification != app.VerificationDisputed {
		t.Fatalf("C-001 = %s/%s", c1.Status, c1.Verification)
	}
	if c2.Status != app.ClaimStatusActive || c2.Verification == app.VerificationSuperseded {
		t.Fatalf("C-002 = %s/%s", c2.Status, c2.Verification)
	}
	if h1.Verification == app.VerificationSuperseded {
		t.Fatalf("H-001 = %s/%s", h1.Status, h1.Verification)
	}
}
