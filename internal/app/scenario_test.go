package app_test

import (
	"context"
	"testing"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/app/apptest"
)

// TestBriefScenario walks every TEST-01 category of the brief through
// internal/app, in order, on one investigation. Each subtest asserts the
// returned DTOs and the activity row the step wrote.
func TestBriefScenario(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t)
	var inv app.Investigation

	t.Run("create investigation", func(t *testing.T) {
		var err error
		inv, err = a.CreateInvestigation(ctx, apptest.Human, app.CreateInvestigationInput{
			Title:            "September cloud bill up 38%",
			ProblemStatement: "Why did the cloud bill rise 38% in September when traffic was flat?",
			Status:           app.InvestigationStatusInvestigating,
			MakeActive:       true,
		})
		if err != nil {
			t.Fatal(err)
		}
		if inv.Ref != "INV-0001" || !inv.IsActive || inv.Status != app.InvestigationStatusInvestigating ||
			inv.ConclusionConfidence != app.ConfidenceUnknown {
			t.Fatalf("investigation = %+v", inv)
		}
		wantActions(t, a, inv.Ref, "investigation.create", "active.set")
	})

	t.Run("add evidence", func(t *testing.T) {
		ev, err := a.AddEvidence(ctx, apptest.AI, "", app.EvidenceInput{
			Title:       "Service chart default log level set to debug",
			Summary:     "The shared service chart now defaults every service to debug logging.",
			SourceType:  app.SourceTypeCode,
			SourceRef:   "platform-charts/service/values.yaml",
			SourceLabel: "Platform Helm chart",
		})
		if err != nil {
			t.Fatal(err)
		}
		if ev.Ref != "E-001" || ev.Verification != app.VerificationAIGenerated || ev.SourceType != app.SourceTypeCode {
			t.Fatalf("evidence = %+v", ev)
		}
		wantLast(t, a, inv.Ref, "evidence.add", "E-001", `Added evidence E-001 "Service chart default log level set to debug"`)
	})

	t.Run("add claim", func(t *testing.T) {
		c, err := a.AddClaim(ctx, apptest.Human, "", app.ClaimInput{
			Statement:  "A shared platform default moved every service to debug logging on August 28.",
			Confidence: app.ConfidenceHigh,
		})
		if err != nil {
			t.Fatal(err)
		}
		if c.Ref != "C-001" || c.Status != app.ClaimStatusActive || c.Verification != app.VerificationHumanVerified ||
			len(c.Links) != 0 {
			t.Fatalf("claim = %+v", c)
		}
		wantLast(t, a, inv.Ref, "claim.add", "C-001", "Added claim C-001 [high]")
	})

	t.Run("link supporting evidence", func(t *testing.T) {
		c, err := a.LinkEvidenceToClaim(ctx, apptest.Human, "", "C-001", "E-001", app.RelationSupports)
		if err != nil {
			t.Fatal(err)
		}
		if len(c.Links) != 1 || c.Links[0].EvidenceRef != "E-001" || c.Links[0].Relation != app.RelationSupports {
			t.Fatalf("links = %+v", c.Links)
		}
		ev, err := a.GetEvidence(ctx, "", "E-001", app.GetEvidenceOptions{})
		if err != nil || len(ev.ClaimLinks) != 1 || ev.ClaimLinks[0].Ref != "C-001" {
			t.Fatalf("evidence claim links = %+v, %v", ev.ClaimLinks, err)
		}
		wantLast(t, a, inv.Ref, "link.add", "C-001", "Linked E-001 to C-001 (supports)")
	})

	t.Run("link contradicting evidence", func(t *testing.T) {
		if _, err := a.AddEvidence(ctx, apptest.AI, "", app.EvidenceInput{
			Title: "Log ingestion is 71% of the increase", Summary: "Log ingestion rose $96,400; compute and network were flat.",
			SourceType: app.SourceTypeSQLQuery,
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := a.AddClaim(ctx, apptest.AI, "", app.ClaimInput{
			Statement: "The August autoscaling change added compute nodes.", Confidence: app.ConfidenceLow}); err != nil {
			t.Fatal(err)
		}
		c, err := a.LinkEvidenceToClaim(ctx, apptest.AI, "", "C-002", "E-002", app.RelationContradicts)
		if err != nil {
			t.Fatal(err)
		}
		if len(c.Links) != 1 || c.Links[0].Relation != app.RelationContradicts {
			t.Fatalf("links = %+v", c.Links)
		}
		wantLast(t, a, inv.Ref, "link.add", "C-002", "Linked E-002 to C-002 (contradicts)")
	})

	t.Run("hypothesis lifecycle", func(t *testing.T) {
		h, err := a.AddHypothesis(ctx, apptest.AI, "", app.HypothesisInput{
			Statement: "Data transfer from the region move doubled the network line.", Confidence: app.ConfidenceLow,
			Links: []app.EvidenceLinkInput{{EvidenceRef: "E-002", Relation: app.RelationSupports}},
		})
		if err != nil || h.Ref != "H-001" || h.Status != app.HypothesisStatusOpen || len(h.Links) != 1 {
			t.Fatalf("add = %+v, %v", h, err)
		}
		wantLast(t, a, inv.Ref, "hypothesis.add", "H-001", "Added hypothesis H-001 [low] with 1 evidence link")
		h, err = a.UpdateHypothesis(ctx, apptest.AI, "", "H-001", app.HypothesisPatch{Status: ptr(app.HypothesisStatusSupported)})
		if err != nil || h.Status != app.HypothesisStatusSupported {
			t.Fatalf("supported = %+v, %v", h, err)
		}
		h, err = a.LinkEvidenceToHypothesis(ctx, apptest.Human, "", "H-001", "E-002", app.RelationContradicts)
		if err != nil || h.Links[0].Relation != app.RelationContradicts {
			t.Fatalf("relink = %+v, %v", h, err)
		}
		wantLast(t, a, inv.Ref, "link.update", "H-001", "Changed link E-002 to H-001 from supports to contradicts")
		h, err = a.UpdateHypothesis(ctx, apptest.Human, "", "H-001", app.HypothesisPatch{
			Status: ptr(app.HypothesisStatusDisproven), Rationale: ptr("Network spend fell $600 month over month.")})
		if err != nil || h.Status != app.HypothesisStatusDisproven || h.Rationale == "" {
			t.Fatalf("disproven = %+v, %v", h, err)
		}
		wantLast(t, a, inv.Ref, "hypothesis.update", "H-001", "Updated hypothesis H-001: rationale, status disproven")
	})

	t.Run("question lifecycle", func(t *testing.T) {
		q, err := a.AddQuestion(ctx, apptest.Human, "", app.QuestionInput{Question: "Can the other services go back to info level?"})
		if err != nil || q.Ref != "Q-001" || q.Status != app.QuestionStatusOpen {
			t.Fatalf("add = %+v, %v", q, err)
		}
		q, err = a.DeferQuestion(ctx, apptest.Human, "", "Q-001", "waiting on finance")
		if err != nil || q.Status != app.QuestionStatusDeferred || q.DeferNote != "waiting on finance" {
			t.Fatalf("defer = %+v, %v", q, err)
		}
		q, err = a.ReopenQuestion(ctx, apptest.Human, "", "Q-001")
		if err != nil || q.Status != app.QuestionStatusOpen || q.DeferNote != "" {
			t.Fatalf("reopen = %+v, %v", q, err)
		}
		q, err = a.ResolveQuestion(ctx, apptest.Human, "", "Q-001", "Yes, after the chart fix.")
		if err != nil || q.Status != app.QuestionStatusResolved || q.ResolvedAt == "" {
			t.Fatalf("resolve = %+v, %v", q, err)
		}
		wantLast(t, a, inv.Ref, "question.resolve", "Q-001", "Resolved question Q-001: Yes, after the chart fix.")
	})

	t.Run("confidence updates", func(t *testing.T) {
		c, err := a.UpdateClaim(ctx, apptest.Human, "", "C-001", app.ClaimPatch{
			Confidence: ptr(app.ConfidenceMedium), ConfidenceRationale: ptr("Only the code so far."),
		})
		if err != nil || c.Confidence != app.ConfidenceMedium || c.ConfidenceRationale != "Only the code so far." {
			t.Fatalf("claim = %+v, %v", c, err)
		}
		wantLast(t, a, inv.Ref, "claim.update", "C-001", "Updated claim C-001: confidence medium, confidence_rationale")
	})

	t.Run("conclusion updates", func(t *testing.T) {
		got, err := a.UpdateConclusion(ctx, apptest.Human, "", app.ConclusionInput{
			Conclusion: "Debug logging requested for payments only reached every service and drove most of the increase.",
			Confidence: app.ConfidenceHigh,
			Rationale:  "C-001 is supported by code.",
		})
		if err != nil || got.Conclusion != "Debug logging requested for payments only reached every service and drove most of the increase." ||
			got.ConclusionConfidence != app.ConfidenceHigh {
			t.Fatalf("conclusion = %+v, %v", got, err)
		}
		if row := lastActivity(t, a, inv.Ref); row.Action != "conclusion.update" || row.TargetRef != inv.Ref {
			t.Fatalf("last activity = %+v", row)
		}
	})

	t.Run("verification state", func(t *testing.T) {
		ev, err := a.SetEvidenceVerification(ctx, apptest.Human, "", "E-001", app.VerificationSourceVerified)
		if err != nil || ev.Verification != app.VerificationSourceVerified {
			t.Fatalf("evidence = %+v, %v", ev, err)
		}
		wantLast(t, a, inv.Ref, "evidence.update", "E-001", "Set verification of E-001 to source_verified")
		c, err := a.SetClaimVerification(ctx, apptest.Human, "", "C-002", app.VerificationDisputed)
		if err != nil || c.Verification != app.VerificationDisputed || c.Status != app.ClaimStatusDisputed {
			t.Fatalf("claim = %+v, %v", c, err)
		}
		wantLast(t, a, inv.Ref, "claim.update", "C-002", "Set verification of C-002 to disputed; status disputed")
		h, err := a.SetHypothesisVerification(ctx, apptest.Human, "", "H-001", app.VerificationHumanVerified)
		if err != nil || h.Verification != app.VerificationHumanVerified {
			t.Fatalf("hypothesis = %+v, %v", h, err)
		}
		wantLast(t, a, inv.Ref, "hypothesis.update", "H-001", "Set verification of H-001 to human_verified")
	})

	t.Run("superseding records", func(t *testing.T) {
		if _, err := a.AddClaim(ctx, apptest.AI, "", app.ClaimInput{
			Statement: "Log ingestion, billed per GB, is most of the increase.", Confidence: app.ConfidenceHigh}); err != nil {
			t.Fatal(err)
		}
		c, err := a.SupersedeClaim(ctx, apptest.Human, "", "C-002", "C-003")
		if err != nil || c.Status != app.ClaimStatusSuperseded || c.Verification != app.VerificationSuperseded ||
			c.SupersededBy != "C-003" {
			t.Fatalf("superseded claim = %+v, %v", c, err)
		}
		wantLast(t, a, inv.Ref, "claim.supersede", "C-002", "C-002 superseded by C-003")
		if _, err := a.AddHypothesis(ctx, apptest.AI, "", app.HypothesisInput{Statement: "Debug logs drive the increase."}); err != nil {
			t.Fatal(err)
		}
		h, err := a.SupersedeHypothesis(ctx, apptest.Human, "", "H-001", "H-002")
		if err != nil || h.Status != app.HypothesisStatusSuperseded || h.SupersededBy != "H-002" {
			t.Fatalf("superseded hypothesis = %+v, %v", h, err)
		}
		wantLast(t, a, inv.Ref, "hypothesis.supersede", "H-001", "H-001 superseded by H-002")
	})
}
