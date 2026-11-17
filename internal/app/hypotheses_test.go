package app_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/app/apptest"
)

func mustAddHypothesis(t *testing.T, a *app.App, actor app.Actor, invRef string, in app.HypothesisInput) app.Hypothesis {
	t.Helper()
	h, err := a.AddHypothesis(context.Background(), actor, invRef, in)
	if err != nil {
		t.Fatalf("AddHypothesis: %v", err)
	}
	return h
}

func TestAddHypothesisDefaultsAndLinks(t *testing.T) {
	ctx := context.Background()
	a, inv := claimEnv(t)

	h := mustAddHypothesis(t, a, apptest.AI, "", app.HypothesisInput{Statement: "Time zones shift dates."})
	if h.Ref != "H-001" || h.InvestigationRef != inv.Ref || h.Status != app.HypothesisStatusOpen ||
		h.Confidence != app.ConfidenceUnknown || h.Verification != app.VerificationAIGenerated ||
		h.SupersededBy != "" || h.CreatedBy != apptest.AI || h.Links == nil || len(h.Links) != 0 {
		t.Fatalf("hypothesis defaults = %+v", h)
	}
	wantLast(t, a, inv.Ref, "hypothesis.add", "H-001", "Added hypothesis H-001 [unknown]")
	apptest.AssertNoJSONNulls(t, h)

	h2 := mustAddHypothesis(t, a, apptest.Human, inv.Ref, app.HypothesisInput{
		Statement:  "The tracing agent ships logs twice.",
		Confidence: app.ConfidenceMedium,
		Rationale:  "Two shippers seen.",
		Links: []app.EvidenceLinkInput{
			{EvidenceRef: "E-003", Relation: app.RelationContradicts},
			{EvidenceRef: "E-001", Relation: app.RelationSupports},
		},
	})
	if h2.Ref != "H-002" || h2.Verification != app.VerificationHumanVerified || h2.Rationale != "Two shippers seen." ||
		len(h2.Links) != 2 || h2.Links[0].EvidenceRef != "E-001" || h2.Links[0].Relation != app.RelationSupports ||
		h2.Links[1].EvidenceRef != "E-003" || h2.Links[1].Relation != app.RelationContradicts ||
		h2.Links[1].EvidenceTitle != "Evidence 3" {
		t.Fatalf("hypothesis with links = %+v", h2)
	}
	wantLast(t, a, inv.Ref, "hypothesis.add", "H-002", "Added hypothesis H-002 [medium] with 2 evidence links")
	b, _ := json.Marshal(h2)
	for _, key := range []string{`"rationale":"Two shippers seen."`, `"superseded_by":""`, `"links":[{"evidence_ref":"E-001"`} {
		if !strings.Contains(string(b), key) {
			t.Errorf("Hypothesis JSON %s lacks %s", b, key)
		}
	}

	e3, err := a.GetEvidence(ctx, "", "E-003", app.GetEvidenceOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(e3.HypothesisLinks) != 1 || e3.HypothesisLinks[0] != (app.RecordLink{Ref: "H-002", Relation: app.RelationContradicts}) {
		t.Fatalf("E-003 hypothesis links = %+v", e3.HypothesisLinks)
	}

	got, err := a.GetHypothesis(ctx, "", "h-2")
	if err != nil || got.Ref != "H-002" || len(got.Links) != 2 {
		t.Fatalf("GetHypothesis = %+v, %v", got, err)
	}
	list, err := a.ListHypotheses(ctx, "")
	if err != nil || len(list) != 2 || list[0].Ref != "H-001" || list[1].Ref != "H-002" || list[0].Links == nil {
		t.Fatalf("ListHypotheses = %+v, %v", list, err)
	}
	apptest.AssertNoJSONNulls(t, list)
	empty, err := a.ListHypotheses(ctx, newInv(t, a, "Empty").Ref)
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("empty ListHypotheses = %#v, %v", empty, err)
	}
}

func TestAddHypothesisValidation(t *testing.T) {
	ctx := context.Background()
	a, inv := claimEnv(t)
	other := newInv(t, a, "Other")
	seedEvidence(t, a, other.Ref, 1)
	before := activityCount(t, a, inv.Ref)
	for _, tc := range []struct {
		name  string
		in    app.HypothesisInput
		field string
	}{
		{"missing statement", app.HypothesisInput{}, "statement"},
		{"oversized rationale", app.HypothesisInput{Statement: "s", Rationale: strings.Repeat("r", app.MaxRationaleBytes+1)}, "rationale"},
		{"claim status", app.HypothesisInput{Statement: "s", Status: "active"}, "status"},
		{"superseded at creation", app.HypothesisInput{Statement: "s", Status: app.HypothesisStatusSuperseded}, "status"},
		{"invalid confidence", app.HypothesisInput{Statement: "s", Confidence: "75%"}, "confidence"},
		{"invalid relation", app.HypothesisInput{Statement: "s",
			Links: []app.EvidenceLinkInput{{EvidenceRef: "E-001", Relation: "maybe"}}}, "links[0].relation"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := a.AddHypothesis(ctx, apptest.AI, "", tc.in)
			wantValidation(t, err, tc.field)
		})
	}
	_, err := a.AddHypothesis(ctx, apptest.AI, "", app.HypothesisInput{Statement: "s",
		Links: []app.EvidenceLinkInput{{EvidenceRef: "INV-0002/E-001", Relation: app.RelationSupports}}})
	wantErrIs(t, err, app.ErrConflict)
	_, err = a.AddHypothesis(ctx, apptest.AI, "", app.HypothesisInput{Statement: "s", Links: []app.EvidenceLinkInput{
		{EvidenceRef: "E-001", Relation: app.RelationSupports},
		{EvidenceRef: "E-099", Relation: app.RelationSupports},
	}})
	wantErrIs(t, err, app.ErrNotFound)
	if n := activityCount(t, a, inv.Ref); n != before {
		t.Fatalf("activity rows %d, want %d", n, before)
	}
	if h := mustAddHypothesis(t, a, apptest.AI, "", app.HypothesisInput{Statement: "ok"}); h.Ref != "H-001" || len(h.Links) != 0 {
		t.Fatalf("first successful hypothesis = %+v", h)
	}
	_, err = a.GetHypothesis(ctx, "", "C-001")
	wantValidation(t, err, "ref")
	_, err = a.GetHypothesis(ctx, "", "H-002")
	wantErrIs(t, err, app.ErrNotFound)
}

func TestHypothesisUpdateLifecycle(t *testing.T) {
	ctx := context.Background()
	a, inv := claimEnv(t, app.WithClock(apptest.StepClock(apptest.FixedTime, time.Second)))
	h := mustAddHypothesis(t, a, apptest.AI, "", app.HypothesisInput{Statement: "Load runs late.", Confidence: app.ConfidenceLow})

	got, err := a.UpdateHypothesis(ctx, apptest.Human, "", "H-001", app.HypothesisPatch{
		Status: ptr(app.HypothesisStatusSupported), Confidence: ptr(app.ConfidenceMedium), Rationale: ptr("Lag seen once."),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != app.HypothesisStatusSupported || got.Confidence != app.ConfidenceMedium || got.Rationale != "Lag seen once." ||
		got.UpdatedAt <= h.UpdatedAt {
		t.Fatalf("supported = %+v", got)
	}
	wantLast(t, a, inv.Ref, "hypothesis.update", "H-001", "Updated hypothesis H-001: confidence medium, rationale, status supported")

	got, err = a.UpdateHypothesis(ctx, apptest.Human, "", "H-001", app.HypothesisPatch{Status: ptr(app.HypothesisStatusDisproven)})
	if err != nil || got.Status != app.HypothesisStatusDisproven || got.Confidence != app.ConfidenceMedium ||
		got.Rationale != "Lag seen once." {
		t.Fatalf("disproven = %+v, %v", got, err)
	}
	wantLast(t, a, inv.Ref, "hypothesis.update", "H-001", "Updated hypothesis H-001: status disproven")

	n := activityCount(t, a, inv.Ref)
	if _, err := a.UpdateHypothesis(ctx, apptest.Human, "", "H-001", app.HypothesisPatch{Status: ptr(app.HypothesisStatusDisproven)}); err != nil {
		t.Fatal(err)
	}
	if activityCount(t, a, inv.Ref) != n {
		t.Fatal("no-op hypothesis patch wrote activity")
	}
	_, err = a.UpdateHypothesis(ctx, apptest.Human, "", "H-001", app.HypothesisPatch{Status: ptr(app.HypothesisStatus("rejected"))})
	wantValidation(t, err, "status")

	got, err = a.SetHypothesisVerification(ctx, apptest.Human, "", "H-001", app.VerificationHumanVerified)
	if err != nil || got.Verification != app.VerificationHumanVerified {
		t.Fatalf("SetHypothesisVerification = %+v, %v", got, err)
	}
	wantLast(t, a, inv.Ref, "hypothesis.update", "H-001", "Set verification of H-001 to human_verified")
}

func TestSupersedeHypothesis(t *testing.T) {
	ctx := context.Background()
	a, inv := claimEnv(t)
	mustAddHypothesis(t, a, apptest.AI, "", app.HypothesisInput{Statement: "One"})
	mustAddHypothesis(t, a, apptest.AI, "", app.HypothesisInput{Statement: "Two"})

	_, err := a.UpdateHypothesis(ctx, apptest.Human, "", "H-001", app.HypothesisPatch{Status: ptr(app.HypothesisStatusSuperseded)})
	wantValidation(t, err, "superseded_by")
	_, err = a.SupersedeHypothesis(ctx, apptest.Human, "", "H-001", "H-001")
	wantErrIs(t, err, app.ErrConflict)
	_, err = a.SupersedeHypothesis(ctx, apptest.Human, "", "H-001", "H-009")
	wantErrIs(t, err, app.ErrNotFound)
	_, err = a.SupersedeHypothesis(ctx, apptest.Human, "", "H-001", "C-002")
	wantValidation(t, err, "replacement_ref")

	got, err := a.SupersedeHypothesis(ctx, apptest.Human, "", "H-001", "H-002")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != app.HypothesisStatusSuperseded || got.Verification != app.VerificationSuperseded || got.SupersededBy != "H-002" {
		t.Fatalf("superseded hypothesis = %+v", got)
	}
	wantLast(t, a, inv.Ref, "hypothesis.supersede", "H-001", "H-001 superseded by H-002")
	_, err = a.SupersedeHypothesis(ctx, apptest.Human, "", "H-002", "H-001")
	wantErrIs(t, err, app.ErrConflict)

	// The patch form behaves the same.
	mustAddHypothesis(t, a, apptest.AI, "", app.HypothesisInput{Statement: "Three"})
	got, err = a.UpdateHypothesis(ctx, apptest.Human, "", "H-003", app.HypothesisPatch{SupersededBy: ptr("H-002")})
	if err != nil || got.Status != app.HypothesisStatusSuperseded || got.SupersededBy != "H-002" {
		t.Fatalf("patch supersede = %+v, %v", got, err)
	}
	wantLast(t, a, inv.Ref, "hypothesis.supersede", "H-003", "H-003 superseded by H-002")

	// Reopening clears the replacement.
	got, err = a.UpdateHypothesis(ctx, apptest.Human, "", "H-003", app.HypothesisPatch{Status: ptr(app.HypothesisStatusOpen)})
	if err != nil || got.SupersededBy != "" || got.Status != app.HypothesisStatusOpen || got.Verification != app.VerificationHumanVerified {
		t.Fatalf("reopened hypothesis = %+v, %v", got, err)
	}
}

func TestLinkEvidenceToHypothesis(t *testing.T) {
	ctx := context.Background()
	a, inv := claimEnv(t)
	other := newInv(t, a, "Other")
	seedEvidence(t, a, other.Ref, 1)
	mustAddHypothesis(t, a, apptest.AI, "", app.HypothesisInput{Statement: "s"})

	got, err := a.LinkEvidenceToHypothesis(ctx, apptest.AI, "", "H-001", "E-001", app.RelationContradicts)
	if err != nil || len(got.Links) != 1 || got.Links[0].Relation != app.RelationContradicts || got.Links[0].EvidenceRef != "E-001" {
		t.Fatalf("link = %+v, %v", got.Links, err)
	}
	wantLast(t, a, inv.Ref, "link.add", "H-001", "Linked E-001 to H-001 (contradicts)")

	n := activityCount(t, a, inv.Ref)
	if _, err := a.LinkEvidenceToHypothesis(ctx, apptest.AI, "", "H-001", "E-001", app.RelationContradicts); err != nil {
		t.Fatal(err)
	}
	if activityCount(t, a, inv.Ref) != n {
		t.Fatal("idempotent hypothesis relink wrote activity")
	}
	got, err = a.LinkEvidenceToHypothesis(ctx, apptest.AI, "", "H-001", "E-001", app.RelationSupports)
	if err != nil || got.Links[0].Relation != app.RelationSupports {
		t.Fatalf("relink = %+v, %v", got.Links, err)
	}
	wantLast(t, a, inv.Ref, "link.update", "H-001", "Changed link E-001 to H-001 from contradicts to supports")
	ev, err := a.GetEvidence(ctx, "", "E-001", app.GetEvidenceOptions{})
	if err != nil || len(ev.HypothesisLinks) != 1 || ev.HypothesisLinks[0] != (app.RecordLink{Ref: "H-001", Relation: app.RelationSupports}) {
		t.Fatalf("E-001 hypothesis links = %+v, %v", ev.HypothesisLinks, err)
	}

	_, err = a.LinkEvidenceToHypothesis(ctx, apptest.AI, "", "H-001", "INV-0002/E-001", app.RelationSupports)
	wantErrIs(t, err, app.ErrConflict)
	_, err = a.LinkEvidenceToHypothesis(ctx, apptest.AI, "", "H-001", "E-001", "")
	wantValidation(t, err, "relation")

	got, err = a.UnlinkEvidenceFromHypothesis(ctx, apptest.Human, "", "H-001", "E-001")
	if err != nil || len(got.Links) != 0 || got.Links == nil {
		t.Fatalf("unlink = %#v, %v", got.Links, err)
	}
	wantLast(t, a, inv.Ref, "link.remove", "H-001", "Unlinked E-001 from H-001")
	_, err = a.UnlinkEvidenceFromHypothesis(ctx, apptest.Human, "", "H-001", "E-001")
	wantErrIs(t, err, app.ErrNotFound)
	if !strings.Contains(err.Error(), "no link between H-001 and E-001") {
		t.Fatalf("missing link message = %q", err)
	}
}
