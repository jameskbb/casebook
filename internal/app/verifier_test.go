package app_test

import (
	"context"
	"strings"
	"testing"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/app/apptest"
)

// nonHumans are every actor kind that may not mark a record human_verified
// (ADR-0014).
var nonHumans = []app.Actor{
	apptest.AI,
	{Kind: app.ActorImport, Client: "import"},
	{Kind: app.ActorSystem, Client: "casebook"},
}

// wantOnlyPeople asserts the ADR-0014 refusal: a ValidationError on field
// (verification, or an item path ending in it) that tells the caller to
// leave human_verified to a person.
func wantOnlyPeople(t *testing.T, err error, field string) {
	t.Helper()
	wantValidation(t, err, field)
	if !strings.Contains(err.Error(), "only a person can mark a record human_verified") ||
		!strings.Contains(err.Error(), "source_verified") {
		t.Fatalf("error %q does not explain the rule", err)
	}
}

// verifyEnv is claimEnv plus one claim (C-001) and one hypothesis (H-001)
// added by an AI client.
func verifyEnv(t *testing.T) *app.App {
	t.Helper()
	a, _ := claimEnv(t)
	mustAddClaim(t, a, apptest.AI, "", app.ClaimInput{Statement: "One"})
	if _, err := a.AddHypothesis(context.Background(), apptest.AI, "", app.HypothesisInput{Statement: "H"}); err != nil {
		t.Fatal(err)
	}
	return a
}

// TestOnlyPeopleVerifyRejectsAI covers every engine path that writes a
// verification: none accepts human_verified from a non-human actor, and a
// refused write changes nothing.
func TestOnlyPeopleVerifyRejectsAI(t *testing.T) {
	ctx := context.Background()
	a := verifyEnv(t)
	hv := app.VerificationHumanVerified
	before, err := a.ChangeToken(ctx)
	if err != nil {
		t.Fatal(err)
	}
	n := activityCount(t, a, "")

	for _, actor := range nonHumans {
		t.Run(string(actor.Kind), func(t *testing.T) {
			_, err := a.AddEvidence(ctx, actor, "", app.EvidenceInput{Title: "t", Summary: "s", SourceType: app.SourceTypeFile, Verification: hv})
			wantOnlyPeople(t, err, "verification")
			_, err = a.UpdateEvidence(ctx, actor, "", "E-001", app.EvidencePatch{Verification: &hv})
			wantOnlyPeople(t, err, "verification")
			_, err = a.SetEvidenceVerification(ctx, actor, "", "E-001", hv)
			wantOnlyPeople(t, err, "verification")

			_, err = a.AddClaim(ctx, actor, "", app.ClaimInput{Statement: "x", Verification: hv})
			wantOnlyPeople(t, err, "verification")
			_, err = a.UpdateClaim(ctx, actor, "", "C-001", app.ClaimPatch{Verification: &hv})
			wantOnlyPeople(t, err, "verification")
			_, err = a.UpdateClaim(ctx, actor, "", "C-001", app.ClaimPatch{Statement: ptr("y"), Verification: &hv})
			wantOnlyPeople(t, err, "verification")
			_, err = a.SetClaimVerification(ctx, actor, "", "C-001", hv)
			wantOnlyPeople(t, err, "verification")

			_, err = a.AddHypothesis(ctx, actor, "", app.HypothesisInput{Statement: "x", Verification: hv})
			wantOnlyPeople(t, err, "verification")
			_, err = a.UpdateHypothesis(ctx, actor, "", "H-001", app.HypothesisPatch{Verification: &hv})
			wantOnlyPeople(t, err, "verification")
			_, err = a.SetHypothesisVerification(ctx, actor, "", "H-001", hv)
			wantOnlyPeople(t, err, "verification")
		})
	}

	after, _ := a.ChangeToken(ctx)
	if after != before || activityCount(t, a, "") != n {
		t.Fatal("a refused write changed something")
	}
	c, err := a.GetClaim(ctx, "", "C-001")
	if err != nil || c.Verification != app.VerificationAIGenerated || c.Statement != "One" {
		t.Fatalf("C-001 = %s %q, %v", c.Verification, c.Statement, err)
	}
}

// TestOnlyPeopleVerifyAlreadyVerified refuses an AI that names
// human_verified even when the record already is: the request itself is
// the claim the rule forbids, and a no-op would hide that.
func TestOnlyPeopleVerifyAlreadyVerified(t *testing.T) {
	ctx := context.Background()
	a := verifyEnv(t)
	if _, err := a.SetClaimVerification(ctx, apptest.Human, "", "C-001", app.VerificationHumanVerified); err != nil {
		t.Fatal(err)
	}
	_, err := a.SetClaimVerification(ctx, apptest.AI, "", "C-001", app.VerificationHumanVerified)
	wantOnlyPeople(t, err, "verification")
}

// TestOnlyPeopleVerifyAllowsHuman keeps every path open to a person, and
// keeps a person's default verification.
func TestOnlyPeopleVerifyAllowsHuman(t *testing.T) {
	ctx := context.Background()
	a := verifyEnv(t)
	hv := app.VerificationHumanVerified
	h := apptest.Human

	e, err := a.AddEvidence(ctx, h, "", app.EvidenceInput{Title: "t", Summary: "s", SourceType: app.SourceTypeFile})
	if err != nil || e.Verification != hv {
		t.Fatalf("human evidence default = %s, %v", e.Verification, err)
	}
	if e, err = a.SetEvidenceVerification(ctx, h, "", "E-001", hv); err != nil || e.Verification != hv {
		t.Fatalf("SetEvidenceVerification = %s, %v", e.Verification, err)
	}
	if e, err = a.UpdateEvidence(ctx, h, "", "E-002", app.EvidencePatch{Verification: &hv}); err != nil || e.Verification != hv {
		t.Fatalf("UpdateEvidence = %s, %v", e.Verification, err)
	}
	c, err := a.AddClaim(ctx, h, "", app.ClaimInput{Statement: "x", Verification: hv})
	if err != nil || c.Verification != hv {
		t.Fatalf("AddClaim = %s, %v", c.Verification, err)
	}
	if c, err = a.UpdateClaim(ctx, h, "", "C-001", app.ClaimPatch{Verification: &hv}); err != nil || c.Verification != hv {
		t.Fatalf("UpdateClaim = %s, %v", c.Verification, err)
	}
	hy, err := a.SetHypothesisVerification(ctx, h, "", "H-001", hv)
	if err != nil || hy.Verification != hv {
		t.Fatalf("SetHypothesisVerification = %s, %v", hy.Verification, err)
	}
	_, err = a.CheckpointBatch(ctx, h, "", app.CheckpointInput{
		Note:     "Checked by hand.",
		Evidence: []app.CheckpointEvidence{{Evidence: app.EvidenceInput{Title: "t", Summary: "s", SourceType: app.SourceTypeFile, Verification: hv}}},
		HypothesisUpdates: []app.HypothesisUpdate{
			{Ref: "H-001", Patch: app.HypothesisPatch{Verification: ptr(app.VerificationDisputed)}},
		},
		ClaimUpdates: []app.ClaimUpdate{{Ref: "C-002", Patch: app.ClaimPatch{Verification: &hv}}},
	})
	if err != nil {
		t.Fatalf("human checkpoint: %v", err)
	}
}

// TestOnlyPeopleVerifyAllowsAISourceVerified: source_verified means the AI
// checked the record against its source, which it can do.
func TestOnlyPeopleVerifyAllowsAISourceVerified(t *testing.T) {
	ctx := context.Background()
	a := verifyEnv(t)
	sv := app.VerificationSourceVerified
	ai := apptest.AI

	e, err := a.AddEvidence(ctx, ai, "", app.EvidenceInput{Title: "t", Summary: "s", SourceType: app.SourceTypeFile, Verification: sv})
	if err != nil || e.Verification != sv {
		t.Fatalf("AddEvidence = %s, %v", e.Verification, err)
	}
	if e, err = a.SetEvidenceVerification(ctx, ai, "", "E-001", sv); err != nil || e.Verification != sv {
		t.Fatalf("SetEvidenceVerification = %s, %v", e.Verification, err)
	}
	c, err := a.AddClaim(ctx, ai, "", app.ClaimInput{Statement: "x", Verification: sv})
	if err != nil || c.Verification != sv {
		t.Fatalf("AddClaim = %s, %v", c.Verification, err)
	}
	if c, err = a.SetClaimVerification(ctx, ai, "", "C-001", sv); err != nil || c.Verification != sv {
		t.Fatalf("SetClaimVerification = %s, %v", c.Verification, err)
	}
	hy, err := a.UpdateHypothesis(ctx, ai, "", "H-001", app.HypothesisPatch{Verification: &sv})
	if err != nil || hy.Verification != sv {
		t.Fatalf("UpdateHypothesis = %s, %v", hy.Verification, err)
	}
	if _, err = a.CheckpointBatch(ctx, ai, "", app.CheckpointInput{
		Note:       "Checked against the source.",
		Hypotheses: []app.HypothesisInput{{Statement: "y", Verification: sv}},
	}); err != nil {
		t.Fatalf("checkpoint: %v", err)
	}
}

// TestOnlyPeopleVerifyAllowsAIDispute: an AI may move a record away from
// human_verified by disputing it, so new contradicting evidence can always
// be recorded against a record a person verified (ADR-0014).
func TestOnlyPeopleVerifyAllowsAIDispute(t *testing.T) {
	ctx := context.Background()
	a := verifyEnv(t)
	h, ai := apptest.Human, apptest.AI
	hv := app.VerificationHumanVerified
	if _, err := a.SetClaimVerification(ctx, h, "", "C-001", hv); err != nil {
		t.Fatal(err)
	}
	if _, err := a.SetHypothesisVerification(ctx, h, "", "H-001", hv); err != nil {
		t.Fatal(err)
	}
	if _, err := a.SetEvidenceVerification(ctx, h, "", "E-001", hv); err != nil {
		t.Fatal(err)
	}

	c, err := a.UpdateClaim(ctx, ai, "", "C-001", app.ClaimPatch{Status: ptr(app.ClaimStatusDisputed)})
	if err != nil || c.Status != app.ClaimStatusDisputed || c.Verification != app.VerificationDisputed {
		t.Fatalf("AI dispute of C-001 = %s/%s, %v", c.Status, c.Verification, err)
	}
	hy, err := a.SetHypothesisVerification(ctx, ai, "", "H-001", app.VerificationDisputed)
	if err != nil || hy.Verification != app.VerificationDisputed {
		t.Fatalf("AI dispute of H-001 = %s, %v", hy.Verification, err)
	}
	e, err := a.SetEvidenceVerification(ctx, ai, "", "E-001", app.VerificationDisputed)
	if err != nil || e.Verification != app.VerificationDisputed {
		t.Fatalf("AI dispute of E-001 = %s, %v", e.Verification, err)
	}
	// The AI cannot settle the dispute back to human_verified; only a
	// person can, and when the AI reactivates the claim it gets the AI
	// default.
	_, err = a.SetClaimVerification(ctx, ai, "", "C-001", hv)
	wantOnlyPeople(t, err, "verification")
	c, err = a.UpdateClaim(ctx, ai, "", "C-001", app.ClaimPatch{Status: ptr(app.ClaimStatusActive)})
	if err != nil || c.Verification != app.VerificationAIGenerated {
		t.Fatalf("AI reactivation of C-001 = %s/%s, %v", c.Status, c.Verification, err)
	}
	if c, err = a.SetClaimVerification(ctx, h, "", "C-001", hv); err != nil || c.Verification != hv {
		t.Fatalf("human re-verify = %s, %v", c.Verification, err)
	}
}

// TestOnlyPeopleVerifyCheckpoint refuses human_verified in any checkpoint
// item from an AI, names the item path, and writes nothing.
func TestOnlyPeopleVerifyCheckpoint(t *testing.T) {
	ctx := context.Background()
	a := verifyEnv(t)
	hv := app.VerificationHumanVerified
	ev := app.EvidenceInput{Title: "t", Summary: "s", SourceType: app.SourceTypeFile}
	evHV := ev
	evHV.Verification = hv
	// Every case carries valid items before the bad one, so a partial
	// write would show.
	good := func(in app.CheckpointInput) app.CheckpointInput {
		in.Note = "n"
		in.Evidence = append([]app.CheckpointEvidence{{Key: "k", Evidence: ev}}, in.Evidence...)
		in.Questions = []app.QuestionInput{{Question: "q?"}}
		return in
	}
	before, err := a.ChangeToken(ctx)
	if err != nil {
		t.Fatal(err)
	}
	n := activityCount(t, a, "")
	for _, tc := range []struct {
		in    app.CheckpointInput
		field string
	}{
		{good(app.CheckpointInput{Evidence: []app.CheckpointEvidence{{Evidence: evHV}}}), "evidence[1].verification"},
		{good(app.CheckpointInput{Claims: []app.ClaimInput{{Statement: "ok"}, {Statement: "x", Verification: hv}}}), "claims[1].verification"},
		{good(app.CheckpointInput{Hypotheses: []app.HypothesisInput{{Statement: "x", Verification: hv}}}), "hypotheses[0].verification"},
		{good(app.CheckpointInput{ClaimUpdates: []app.ClaimUpdate{{Ref: "C-001", Patch: app.ClaimPatch{Verification: &hv}}}}), "claim_updates[0].patch.verification"},
		{good(app.CheckpointInput{HypothesisUpdates: []app.HypothesisUpdate{{Ref: "H-001", Patch: app.HypothesisPatch{Verification: &hv}}}}), "hypothesis_updates[0].patch.verification"},
	} {
		_, err := a.CheckpointBatch(ctx, apptest.AI, "", tc.in)
		wantOnlyPeople(t, err, tc.field)
	}
	after, _ := a.ChangeToken(ctx)
	if after != before || activityCount(t, a, "") != n {
		t.Fatal("a refused checkpoint wrote something")
	}
	if evs, _ := a.ListEvidence(ctx, ""); len(evs) != 3 {
		t.Fatalf("evidence = %d, want 3", len(evs))
	}
}

// TestAIEditResetsVouchedVerification pins the ADR-0014 reset: when a
// non-human actor changes what a check vouched for (a statement, or any
// evidence field), a human_verified or source_verified record drops to
// ai_generated in the same write, and the activity summary says so.
// Confidence, rationale, status and links do not reset it, a person's edit
// keeps it, and a verification named in the same patch wins.
func TestAIEditResetsVouchedVerification(t *testing.T) {
	ctx := context.Background()
	a := verifyEnv(t)
	h, ai := apptest.Human, apptest.AI
	hv, sv, ag := app.VerificationHumanVerified, app.VerificationSourceVerified, app.VerificationAIGenerated

	t.Run("claim", func(t *testing.T) {
		if _, err := a.SetClaimVerification(ctx, h, "", "C-001", hv); err != nil {
			t.Fatal(err)
		}
		// Confidence, rationale and status changes keep it.
		c, err := a.UpdateClaim(ctx, ai, "", "C-001", app.ClaimPatch{
			Confidence: ptr(app.ConfidenceHigh), ConfidenceRationale: ptr("More rows agree."), Status: ptr(app.ClaimStatusRejected)})
		if err != nil || c.Verification != hv {
			t.Fatalf("non-substance edit = %s, %v", c.Verification, err)
		}
		if _, err := a.LinkEvidenceToClaim(ctx, ai, "", "C-001", "E-001", app.RelationSupports); err != nil {
			t.Fatal(err)
		}
		if c, _ = a.GetClaim(ctx, "", "C-001"); c.Verification != hv {
			t.Fatalf("after link = %s", c.Verification)
		}
		// A person's statement edit keeps it.
		if c, err = a.UpdateClaim(ctx, h, "", "C-001", app.ClaimPatch{Statement: ptr("Person wording.")}); err != nil || c.Verification != hv {
			t.Fatalf("human edit = %s, %v", c.Verification, err)
		}
		// An AI statement edit resets it.
		if c, err = a.UpdateClaim(ctx, ai, "", "C-001", app.ClaimPatch{Statement: ptr("AI wording.")}); err != nil || c.Verification != ag {
			t.Fatalf("AI edit = %s, %v", c.Verification, err)
		}
		wantLast(t, a, "", "claim.update", "C-001",
			"Updated claim C-001: statement, verification; verification reset to ai_generated after AI edit")
		// An AI that rechecked the source says so in the same patch.
		if _, err := a.SetClaimVerification(ctx, ai, "", "C-001", sv); err != nil {
			t.Fatal(err)
		}
		if c, err = a.UpdateClaim(ctx, ai, "", "C-001", app.ClaimPatch{Statement: ptr("Rechecked."), Verification: &sv}); err != nil || c.Verification != sv {
			t.Fatalf("AI edit with source_verified = %s, %v", c.Verification, err)
		}
		// source_verified resets like human_verified.
		if c, err = a.UpdateClaim(ctx, ai, "", "C-001", app.ClaimPatch{Statement: ptr("Again.")}); err != nil || c.Verification != ag {
			t.Fatalf("AI edit of source_verified = %s, %v", c.Verification, err)
		}
	})

	t.Run("hypothesis", func(t *testing.T) {
		if _, err := a.SetHypothesisVerification(ctx, h, "", "H-001", hv); err != nil {
			t.Fatal(err)
		}
		hy, err := a.UpdateHypothesis(ctx, ai, "", "H-001", app.HypothesisPatch{Status: ptr(app.HypothesisStatusSupported), Rationale: ptr("r")})
		if err != nil || hy.Verification != hv {
			t.Fatalf("non-substance edit = %s, %v", hy.Verification, err)
		}
		if hy, err = a.UpdateHypothesis(ctx, ai, "", "H-001", app.HypothesisPatch{Statement: ptr("New H.")}); err != nil || hy.Verification != ag {
			t.Fatalf("AI edit = %s, %v", hy.Verification, err)
		}
		wantLast(t, a, "", "hypothesis.update", "H-001",
			"Updated hypothesis H-001: statement, verification; verification reset to ai_generated after AI edit")
	})

	t.Run("evidence", func(t *testing.T) {
		for _, p := range []app.EvidencePatch{
			{Title: ptr("New title")}, {Summary: ptr("New summary")}, {RawDetail: ptr("raw")},
			{SourceRef: ptr("q.sql")}, {SourceLabel: ptr("label")}, {SourceType: ptr(app.SourceTypeCode)},
			{CapturedAt: ptr("2026-01-01T00:00:00Z")}, {Metadata: ptr(`{"rows":1}`)},
		} {
			if _, err := a.SetEvidenceVerification(ctx, h, "", "E-002", hv); err != nil {
				t.Fatal(err)
			}
			e, err := a.UpdateEvidence(ctx, ai, "", "E-002", p)
			if err != nil || e.Verification != ag {
				t.Fatalf("AI edit %+v = %s, %v", p, e.Verification, err)
			}
		}
		wantLast(t, a, "", "evidence.update", "E-002",
			"Updated evidence E-002: metadata, verification; verification reset to ai_generated after AI edit")
		// A person's edit, a named verification and an import actor.
		if _, err := a.SetEvidenceVerification(ctx, h, "", "E-002", sv); err != nil {
			t.Fatal(err)
		}
		if e, err := a.UpdateEvidence(ctx, h, "", "E-002", app.EvidencePatch{Title: ptr("Person")}); err != nil || e.Verification != sv {
			t.Fatalf("human edit = %s, %v", e.Verification, err)
		}
		if e, err := a.UpdateEvidence(ctx, ai, "", "E-002", app.EvidencePatch{Title: ptr("AI"), Verification: &sv}); err != nil || e.Verification != sv {
			t.Fatalf("AI edit with source_verified = %s, %v", e.Verification, err)
		}
		imp := app.Actor{Kind: app.ActorImport, Client: "import"}
		if e, err := a.UpdateEvidence(ctx, imp, "", "E-002", app.EvidencePatch{Title: ptr("Imp")}); err != nil || e.Verification != ag {
			t.Fatalf("import edit = %s, %v", e.Verification, err)
		}
		wantLast(t, a, "", "evidence.update", "E-002",
			"Updated evidence E-002: title, verification; verification reset to ai_generated after import edit")
		// A no-op edit changes nothing.
		if _, err := a.SetEvidenceVerification(ctx, h, "", "E-002", hv); err != nil {
			t.Fatal(err)
		}
		if e, err := a.UpdateEvidence(ctx, ai, "", "E-002", app.EvidencePatch{Title: ptr("Imp")}); err != nil || e.Verification != hv {
			t.Fatalf("no-op edit = %s, %v", e.Verification, err)
		}
	})

	t.Run("checkpoint", func(t *testing.T) {
		if _, err := a.SetClaimVerification(ctx, h, "", "C-001", hv); err != nil {
			t.Fatal(err)
		}
		if _, err := a.SetHypothesisVerification(ctx, h, "", "H-001", sv); err != nil {
			t.Fatal(err)
		}
		_, err := a.CheckpointBatch(ctx, ai, "", app.CheckpointInput{
			Note:              "Reworded.",
			ClaimUpdates:      []app.ClaimUpdate{{Ref: "C-001", Patch: app.ClaimPatch{Statement: ptr("Batch wording.")}}},
			HypothesisUpdates: []app.HypothesisUpdate{{Ref: "H-001", Patch: app.HypothesisPatch{Confidence: ptr(app.ConfidenceLow)}}},
		})
		if err != nil {
			t.Fatal(err)
		}
		if c, _ := a.GetClaim(ctx, "", "C-001"); c.Verification != ag {
			t.Fatalf("checkpoint claim = %s", c.Verification)
		}
		if hy, _ := a.GetHypothesis(ctx, "", "H-001"); hy.Verification != sv {
			t.Fatalf("checkpoint confidence-only hypothesis = %s", hy.Verification)
		}
	})
}
