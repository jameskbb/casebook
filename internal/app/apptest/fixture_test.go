package apptest_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/app/apptest"
)

func TestSeedCloudSpend(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t)
	f := apptest.SeedCloudSpend(t, a)

	want := apptest.Fixture{
		InvestigationRef:       "INV-0001",
		Evidence:               []string{"E-001", "E-002", "E-003", "E-004"},
		Claims:                 []string{"C-001", "C-002", "C-003"},
		Hypotheses:             []string{"H-001", "H-002"},
		Questions:              []string{"Q-001"},
		SupersededClaimRef:     "C-003",
		DisprovenHypothesisRef: "H-002",
	}
	if !reflect.DeepEqual(f, want) {
		t.Fatalf("fixture = %+v, want %+v", f, want)
	}

	inv, err := a.GetInvestigation(ctx, "")
	if err != nil {
		t.Fatalf("active investigation: %v", err)
	}
	if inv.Ref != "INV-0001" || !inv.IsActive || inv.Title != "September cloud bill up 38%" ||
		inv.ProblemStatement != "Why did the cloud bill rise 38% in September when traffic was flat?" ||
		inv.Status != app.InvestigationStatusInvestigating ||
		inv.Conclusion != "Debug logging requested for payments only reached every service and drove most of the increase." ||
		inv.ConclusionConfidence != app.ConfidenceHigh || inv.ConclusionRationale == "" || inv.CreatedBy != apptest.Human {
		t.Fatalf("investigation = %+v", inv)
	}
	apptest.AssertNoJSONNulls(t, inv)

	evidence, err := a.ListEvidence(ctx, f.InvestigationRef)
	if err != nil {
		t.Fatal(err)
	}
	apptest.AssertNoJSONNulls(t, evidence)
	wantEv := []struct {
		ref, title string
		src        app.SourceType
		by         app.Actor
		label      string
	}{
		{"E-001", "Service chart default log level set to debug", app.SourceTypeCode, apptest.AI, "Platform Helm chart"},
		{"E-002", "Logs are billed per GB ingested in the month", app.SourceTypeDocumentation, apptest.AI, "Provider pricing page"},
		{"E-003", "Log ingestion is 71% of the September increase", app.SourceTypeSQLQuery, apptest.AI, "Billing export query"},
		{"E-004", "Head of platform: debug was requested for payments only", app.SourceTypeUserStatement, apptest.Human, "Head of platform, in person"},
	}
	if len(evidence) != len(wantEv) {
		t.Fatalf("evidence = %+v", evidence)
	}
	for i, w := range wantEv {
		e := evidence[i]
		if e.Ref != w.ref || e.Title != w.title || e.SourceType != w.src || e.CreatedBy != w.by ||
			e.SourceLabel != w.label || e.Summary == "" {
			t.Fatalf("evidence[%d] = %+v, want %+v", i, e, w)
		}
	}
	if evidence[0].SourceRef != "platform-charts/service/values.yaml" || !evidence[0].HasRawDetail {
		t.Fatalf("E-001 provenance = %+v", evidence[0])
	}
	raw, err := a.GetEvidence(ctx, f.InvestigationRef, "E-001", app.GetEvidenceOptions{IncludeRawDetail: true})
	if err != nil || raw.RawDetail != "--- a/platform-charts/service/values.yaml\n+++ b/platform-charts/service/values.yaml\n logging:\n-  level: info\n+  level: debug\n   format: json" {
		t.Fatalf("E-001 raw detail = %q, %v", raw.RawDetail, err)
	}
	if evidence[1].SourceRef != "Provider pricing page, Logs and monitoring section" {
		t.Fatalf("E-002 source_ref = %q", evidence[1].SourceRef)
	}
	if evidence[2].Metadata != `{"affected_records":214}` {
		t.Fatalf("E-003 metadata = %q", evidence[2].Metadata)
	}
	if evidence[0].Verification != app.VerificationAIGenerated || evidence[3].Verification != app.VerificationHumanVerified {
		t.Fatalf("evidence verification = %s, %s", evidence[0].Verification, evidence[3].Verification)
	}
	e3 := evidence[2]
	if !reflect.DeepEqual(e3.ClaimLinks, []app.RecordLink{
		{Ref: "C-002", Relation: app.RelationSupports}, {Ref: "C-003", Relation: app.RelationContradicts},
	}) || !reflect.DeepEqual(e3.HypothesisLinks, []app.RecordLink{{Ref: "H-002", Relation: app.RelationContradicts}}) {
		t.Fatalf("E-003 links = %+v / %+v", e3.ClaimLinks, e3.HypothesisLinks)
	}

	claims, err := a.ListClaims(ctx, f.InvestigationRef)
	if err != nil {
		t.Fatal(err)
	}
	apptest.AssertNoJSONNulls(t, claims)
	type link struct {
		ref string
		rel app.Relation
	}
	links := func(ls []app.EvidenceLink) []link {
		out := []link{}
		for _, l := range ls {
			out = append(out, link{l.EvidenceRef, l.Relation})
		}
		return out
	}
	sup, con := app.RelationSupports, app.RelationContradicts
	wantClaims := []struct {
		ref        string
		conf       app.Confidence
		status     app.ClaimStatus
		ver        app.Verification
		supersede  string
		by         app.Actor
		statement  string
		claimLinks []link
	}{
		{"C-001", app.ConfidenceHigh, app.ClaimStatusActive, app.VerificationHumanVerified, "", apptest.Human,
			"A shared platform default moved every service to debug logging on August 28; only payments asked for it.",
			[]link{{"E-001", sup}, {"E-004", sup}}},
		{"C-002", app.ConfidenceHigh, app.ClaimStatusActive, app.VerificationAIGenerated, "", apptest.AI,
			"Log ingestion, billed per GB, is $96,400 of the $135,800 increase; compute and network are flat.",
			[]link{{"E-002", sup}, {"E-003", sup}}},
		{"C-003", app.ConfidenceLow, app.ClaimStatusSuperseded, app.VerificationSuperseded, "C-002", apptest.AI,
			"The increase comes from the August autoscaling change adding compute nodes.",
			[]link{{"E-003", con}}},
	}
	if len(claims) != len(wantClaims) {
		t.Fatalf("claims = %+v", claims)
	}
	for i, w := range wantClaims {
		c := claims[i]
		if c.Ref != w.ref || c.Confidence != w.conf || c.Status != w.status || c.Verification != w.ver ||
			c.SupersededBy != w.supersede || c.CreatedBy != w.by || c.Statement != w.statement ||
			!reflect.DeepEqual(links(c.Links), w.claimLinks) {
			t.Fatalf("claims[%d] = %+v (links %+v)", i, c, links(c.Links))
		}
	}
	if claims[0].ConfidenceRationale == "" || claims[1].ConfidenceRationale == "" {
		t.Fatal("established claims lack a confidence rationale")
	}

	hyps, err := a.ListHypotheses(ctx, f.InvestigationRef)
	if err != nil {
		t.Fatal(err)
	}
	apptest.AssertNoJSONNulls(t, hyps)
	if len(hyps) != 2 {
		t.Fatalf("hypotheses = %+v", hyps)
	}
	h1, h2 := hyps[0], hyps[1]
	if h1.Ref != "H-001" || h1.Confidence != app.ConfidenceMedium || h1.Status != app.HypothesisStatusOpen ||
		h1.Statement != "The new tracing agent may also be shipping some services' logs twice." || len(h1.Links) != 0 {
		t.Fatalf("H-001 = %+v", h1)
	}
	if h2.Ref != "H-002" || h2.Confidence != app.ConfidenceLow || h2.Status != app.HypothesisStatusDisproven ||
		h2.Rationale != "Network spend fell $600 month over month." ||
		!reflect.DeepEqual(links(h2.Links), []link{{"E-003", con}}) {
		t.Fatalf("H-002 = %+v", h2)
	}

	questions, err := a.ListQuestions(ctx, f.InvestigationRef)
	if err != nil {
		t.Fatal(err)
	}
	apptest.AssertNoJSONNulls(t, questions)
	if len(questions) != 1 || questions[0].Ref != "Q-001" || questions[0].Status != app.QuestionStatusOpen ||
		questions[0].Question != "Can every service except payments go back to info level today?" ||
		questions[0].CreatedBy != apptest.Human {
		t.Fatalf("questions = %+v", questions)
	}

	activity, err := a.ListActivity(ctx, app.ActivityFilter{InvestigationRef: f.InvestigationRef, Limit: 500})
	if err != nil {
		t.Fatal(err)
	}
	apptest.AssertNoJSONNulls(t, activity)
	if len(activity) == 0 || activity[0].Action != "active.set" {
		t.Fatalf("newest activity = %+v", activity)
	}
}

// TestSeedCloudSpendDeterministic seeds two databases and expects
// identical DTOs (fixed clock), so golden files built from the fixture are
// stable.
func TestSeedCloudSpendDeterministic(t *testing.T) {
	ctx := context.Background()
	snapshot := func() []any {
		a := apptest.Open(t)
		f := apptest.SeedCloudSpend(t, a)
		inv, err1 := a.GetInvestigation(ctx, f.InvestigationRef)
		ev, err2 := a.ListEvidence(ctx, f.InvestigationRef)
		cl, err3 := a.ListClaims(ctx, f.InvestigationRef)
		hy, err4 := a.ListHypotheses(ctx, f.InvestigationRef)
		qu, err5 := a.ListQuestions(ctx, f.InvestigationRef)
		act, err6 := a.ListActivity(ctx, app.ActivityFilter{Limit: 500})
		for _, err := range []error{err1, err2, err3, err4, err5, err6} {
			if err != nil {
				t.Fatal(err)
			}
		}
		return []any{f, inv, ev, cl, hy, qu, act}
	}
	first, second := snapshot(), snapshot()
	for i := range first {
		if !reflect.DeepEqual(first[i], second[i]) {
			t.Fatalf("seed %d differs between runs:\n%+v\n%+v", i, first[i], second[i])
		}
	}
}
