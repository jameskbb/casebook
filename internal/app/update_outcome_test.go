package app_test

import (
	"context"
	"testing"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/app/apptest"
)

// TestUpdateOutcomeReportsReset pins the result side of the ADR-0014 reset:
// an AI edit of a checked claim, hypothesis or evidence item returns
// VerificationReset with the label it replaced, a person's edit does not,
// and a checkpoint lists the reset refs.
func TestUpdateOutcomeReportsReset(t *testing.T) {
	ctx := context.Background()
	a := verifyEnv(t)
	h, ai := apptest.Human, apptest.AI
	hv, sv := app.VerificationHumanVerified, app.VerificationSourceVerified
	want := func(t *testing.T, got app.UpdateOutcome, reset bool, prev app.Verification) {
		t.Helper()
		if got.VerificationReset != reset || got.PreviousVerification != prev {
			t.Fatalf("outcome = %+v, want reset %v previous %q", got, reset, prev)
		}
	}

	t.Run("claim", func(t *testing.T) {
		if _, err := a.SetClaimVerification(ctx, h, "", "C-001", hv); err != nil {
			t.Fatal(err)
		}
		_, oc, err := a.UpdateClaimOutcome(ctx, h, "", "C-001", app.ClaimPatch{Statement: ptr("Person wording.")})
		if err != nil {
			t.Fatal(err)
		}
		want(t, oc, false, "")
		_, oc, err = a.UpdateClaimOutcome(ctx, ai, "", "C-001", app.ClaimPatch{Confidence: ptr(app.ConfidenceHigh)})
		if err != nil {
			t.Fatal(err)
		}
		want(t, oc, false, "")
		c, oc, err := a.UpdateClaimOutcome(ctx, ai, "", "C-001", app.ClaimPatch{Statement: ptr("AI wording.")})
		if err != nil || c.Verification != app.VerificationAIGenerated {
			t.Fatalf("AI edit = %s, %v", c.Verification, err)
		}
		want(t, oc, true, hv)
	})

	t.Run("hypothesis", func(t *testing.T) {
		if _, err := a.SetHypothesisVerification(ctx, ai, "", "H-001", sv); err != nil {
			t.Fatal(err)
		}
		_, oc, err := a.UpdateHypothesisOutcome(ctx, ai, "", "H-001", app.HypothesisPatch{Statement: ptr("New H.")})
		if err != nil {
			t.Fatal(err)
		}
		want(t, oc, true, sv)
	})

	t.Run("evidence", func(t *testing.T) {
		if _, err := a.SetEvidenceVerification(ctx, h, "", "E-002", hv); err != nil {
			t.Fatal(err)
		}
		_, oc, err := a.UpdateEvidenceOutcome(ctx, h, "", "E-002", app.EvidencePatch{Title: ptr("Person title")})
		if err != nil {
			t.Fatal(err)
		}
		want(t, oc, false, "")
		_, oc, err = a.UpdateEvidenceOutcome(ctx, ai, "", "E-002", app.EvidencePatch{Title: ptr("AI title")})
		if err != nil {
			t.Fatal(err)
		}
		want(t, oc, true, hv)
	})

	t.Run("checkpoint", func(t *testing.T) {
		if _, err := a.SetClaimVerification(ctx, h, "", "C-001", hv); err != nil {
			t.Fatal(err)
		}
		if _, err := a.SetHypothesisVerification(ctx, h, "", "H-001", hv); err != nil {
			t.Fatal(err)
		}
		res, err := a.CheckpointBatch(ctx, ai, "", app.CheckpointInput{
			Note:              "Reworded both.",
			ClaimUpdates:      []app.ClaimUpdate{{Ref: "C-001", Patch: app.ClaimPatch{Statement: ptr("Checkpoint wording.")}}},
			HypothesisUpdates: []app.HypothesisUpdate{{Ref: "H-001", Patch: app.HypothesisPatch{Rationale: ptr("Only the rationale.")}}},
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(res.VerificationResets) != 1 || res.VerificationResets[0] != (app.VerificationReset{
			Type: app.RecordClaim, Ref: "C-001", PreviousVerification: hv,
		}) {
			t.Fatalf("resets = %+v, want only C-001 (human_verified)", res.VerificationResets)
		}
		res, err = a.CheckpointBatch(ctx, h, "", app.CheckpointInput{
			Note:              "A person reworded it.",
			HypothesisUpdates: []app.HypothesisUpdate{{Ref: "H-001", Patch: app.HypothesisPatch{Statement: ptr("Person H.")}}},
		})
		if err != nil || res.VerificationResets != nil {
			t.Fatalf("human checkpoint resets = %+v, %v", res.VerificationResets, err)
		}
	})
}
