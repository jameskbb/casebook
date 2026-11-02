package app_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/app/apptest"
)

// TestCheckpointBatchApplies covers the AR-06 contract: every item applies in
// order inside one transaction, #key links resolve to evidence created in the
// same batch, Created and Updated list exactly what changed, and the changes
// since the previous checkpoint include the batch's own rows.
func TestCheckpointBatchApplies(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t)
	f := apptest.SeedCloudSpend(t, a)
	n := activityCount(t, a, f.InvestigationRef)

	res, err := a.CheckpointBatch(ctx, apptest.AI, "", app.CheckpointInput{
		Note: "Batch checkpoint",
		Evidence: []app.CheckpointEvidence{
			{Key: "new1", Evidence: app.EvidenceInput{Title: "Batch log", Summary: "s", SourceType: app.SourceTypeFile}},
			{Evidence: app.EvidenceInput{Title: "Batch note", Summary: "s", SourceType: app.SourceTypeCode}},
		},
		Claims: []app.ClaimInput{{Statement: "Batch claim", Links: []app.EvidenceLinkInput{
			{EvidenceRef: "#new1", Relation: app.RelationSupports},
			{EvidenceRef: "E-001", Relation: app.RelationContradicts},
		}}},
		ClaimUpdates: []app.ClaimUpdate{
			{Ref: "c-1", Patch: app.ClaimPatch{Confidence: ptr(app.ConfidenceMedium)}},
			{Ref: "C-002", Patch: app.ClaimPatch{Confidence: ptr(app.ConfidenceHigh)}}, // no-op
		},
		Hypotheses:        []app.HypothesisInput{{Statement: "Batch hypothesis", Links: []app.EvidenceLinkInput{{EvidenceRef: "#new1", Relation: app.RelationContradicts}}}},
		HypothesisUpdates: []app.HypothesisUpdate{{Ref: "H-001", Patch: app.HypothesisPatch{Status: ptr(app.HypothesisStatusSupported)}}},
		Questions:         []app.QuestionInput{{Question: "Batch question?"}},
		QuestionUpdates: []app.QuestionUpdate{
			{Ref: "Q-001", Resolution: ptr("Answered in the batch.")},
			{Ref: "Q-002", Defer: true, Note: "Later."},
		},
		Conclusion: &app.ConclusionInput{Conclusion: "Batch conclusion.", Confidence: app.ConfidenceMedium},
	})
	if err != nil {
		t.Fatalf("CheckpointBatch: %v", err)
	}
	apptest.AssertNoJSONNulls(t, res)
	if got := strings.Join(res.Created, ","); got != "E-005,E-006,C-004,H-003,Q-002" {
		t.Fatalf("Created = %s", got)
	}
	if got := strings.Join(res.Updated, ","); got != "C-001,H-001,Q-001,Q-002,INV-0001" {
		t.Fatalf("Updated = %s", got)
	}
	c, err := a.GetClaim(ctx, "", "C-004")
	if err != nil || len(c.Links) != 2 || c.Links[1].EvidenceRef != "E-005" || c.Links[1].Relation != app.RelationSupports {
		t.Fatalf("C-004 = %+v, %v", c, err)
	}
	h, _ := a.GetHypothesis(ctx, "", "H-003")
	if len(h.Links) != 1 || h.Links[0].EvidenceRef != "E-005" {
		t.Fatalf("H-003 links = %+v", h.Links)
	}
	// One row per changed item plus the checkpoint row.
	if got := activityCount(t, a, f.InvestigationRef); got != n+11 {
		t.Fatalf("activity rows = %d, want %d", got, n+11)
	}
	if res.Changes[0].Action != "conclusion.update" {
		t.Fatalf("newest change = %+v; the batch rows must be included", res.Changes[0])
	}
	if last := lastActivity(t, a, f.InvestigationRef); last.Action != "checkpoint" || last.ID != res.ActivityID {
		t.Fatalf("last activity = %+v", last)
	}
	if itemRefs(res.OpenQuestions) != "" {
		t.Fatalf("open questions = %s", itemRefs(res.OpenQuestions))
	}
}

// TestCheckpointBatchAtomic: a failure anywhere leaves nothing written, not
// even the earlier items or the checkpoint row.
func TestCheckpointBatchAtomic(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t)
	f := apptest.SeedCloudSpend(t, a)
	before, err := a.ChangeToken(ctx)
	if err != nil {
		t.Fatal(err)
	}
	n := activityCount(t, a, f.InvestigationRef)
	_, err = a.CheckpointBatch(ctx, apptest.AI, "", app.CheckpointInput{
		Note:            "Will fail",
		Evidence:        []app.CheckpointEvidence{{Evidence: app.EvidenceInput{Title: "t", Summary: "s", SourceType: app.SourceTypeFile}}},
		Claims:          []app.ClaimInput{{Statement: "x"}},
		Questions:       []app.QuestionInput{{Question: "q?"}},
		QuestionUpdates: []app.QuestionUpdate{{Ref: "Q-099", Reopen: true}},
	})
	if !errors.Is(err, app.ErrNotFound) || !strings.HasPrefix(err.Error(), "question_updates[0]: not found: question Q-099") {
		t.Fatalf("err = %v", err)
	}
	after, _ := a.ChangeToken(ctx)
	if after != before || activityCount(t, a, f.InvestigationRef) != n {
		t.Fatal("a failed batch wrote something")
	}
	if ev, _ := a.ListEvidence(ctx, ""); len(ev) != 4 {
		t.Fatalf("evidence = %d, want 4", len(ev))
	}
	// Refs were not consumed either.
	ev, err := a.AddEvidence(ctx, apptest.AI, "", app.EvidenceInput{Title: "t", Summary: "s", SourceType: app.SourceTypeFile})
	if err != nil || ev.Ref != "E-005" {
		t.Fatalf("next evidence = %q, %v", ev.Ref, err)
	}
}

func TestCheckpointBatchValidation(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t)
	apptest.SeedCloudSpend(t, a)
	ev := app.EvidenceInput{Title: "t", Summary: "s", SourceType: app.SourceTypeFile}
	many := make([]app.QuestionInput, app.MaxCheckpointItems+1)
	for i := range many {
		many[i].Question = "q?"
	}
	for _, tc := range []struct {
		in    app.CheckpointInput
		field string
	}{
		{app.CheckpointInput{}, "note"},
		{app.CheckpointInput{Note: "n", Questions: many}, "items"},
		{app.CheckpointInput{Note: "n", Evidence: []app.CheckpointEvidence{{Key: "bad key", Evidence: ev}}}, "evidence[0].key"},
		{app.CheckpointInput{Note: "n", Evidence: []app.CheckpointEvidence{{Key: "k", Evidence: ev}, {Key: "k", Evidence: ev}}}, "evidence[1].key"},
		{app.CheckpointInput{Note: "n", Evidence: []app.CheckpointEvidence{{Evidence: app.EvidenceInput{Title: "t", SourceType: app.SourceTypeFile}}}}, "evidence[0].summary"},
		{app.CheckpointInput{Note: "n", Claims: []app.ClaimInput{{Statement: "x", Links: []app.EvidenceLinkInput{{EvidenceRef: "#nope", Relation: app.RelationSupports}}}}}, "claims[0].links[0].evidence_ref"},
		{app.CheckpointInput{Note: "n", Hypotheses: []app.HypothesisInput{{Statement: ""}}}, "hypotheses[0].statement"},
		{app.CheckpointInput{Note: "n", ClaimUpdates: []app.ClaimUpdate{{Ref: "C-001"}}}, "claim_updates[0].patch"},
		{app.CheckpointInput{Note: "n", ClaimUpdates: []app.ClaimUpdate{{Ref: "", Patch: app.ClaimPatch{Statement: ptr("x")}}}}, "claim_updates[0].ref"},
		{app.CheckpointInput{Note: "n", HypothesisUpdates: []app.HypothesisUpdate{{Ref: "H-001", Patch: app.HypothesisPatch{Verification: ptr(app.VerificationSuperseded)}}}}, "hypothesis_updates[0].patch.verification"},
		{app.CheckpointInput{Note: "n", QuestionUpdates: []app.QuestionUpdate{{Ref: "Q-001"}}}, "question_updates[0].question"},
		{app.CheckpointInput{Note: "n", QuestionUpdates: []app.QuestionUpdate{{Ref: "Q-001", Reopen: true, Defer: true}}}, "question_updates[0].resolution"},
		{app.CheckpointInput{Note: "n", QuestionUpdates: []app.QuestionUpdate{{Ref: "Q-001", Note: "x"}}}, "question_updates[0].note"},
		{app.CheckpointInput{Note: "n", Conclusion: &app.ConclusionInput{Confidence: app.ConfidenceHigh}}, "conclusion.confidence"},
	} {
		_, err := a.CheckpointBatch(ctx, apptest.AI, "", tc.in)
		wantValidation(t, err, tc.field)
	}
	// A validation error raised inside the transaction is prefixed too.
	_, err := a.CheckpointBatch(ctx, apptest.AI, "", app.CheckpointInput{Note: "n", ClaimUpdates: []app.ClaimUpdate{
		{Ref: "C-001", Patch: app.ClaimPatch{Status: ptr(app.ClaimStatusDisputed), Verification: ptr(app.VerificationHumanVerified)}},
	}})
	wantValidation(t, err, "claim_updates[0].patch.verification")
}
