package main

import (
	"context"
	"strings"
	"testing"

	"github.com/jameskbb/casebook/internal/app"
)

// TestAcceptanceScenario automates the product brief's acceptance steps 2 to
// 20 (ACC-01): the desktop actor builds the investigation through
// internal/app, the database is closed and reopened, a real `casebook mcp`
// subprocess reads the active investigation and records evidence and claim
// changes, and a reopened engine (what the desktop shows after a refresh)
// sees them, as does the compact context pack. Steps 1 and 12 to 19 in the
// real window are recorded in docs/acceptance.md.
func TestAcceptanceScenario(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dbPath := dir + "/casebook.db"
	desktop := app.ActorDesktop()

	a, err := app.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	must := func(step string, err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("step %s: %v", step, err)
		}
	}

	// 2 and 3. Create an investigation with its problem statement.
	inv, err := a.CreateInvestigation(ctx, desktop, app.CreateInvestigationInput{
		Title:            "September cloud bill up 38%",
		ProblemStatement: "Why did the cloud bill rise 38% in September when traffic was flat?",
	})
	must("2", err)
	// 4. Evidence, added manually.
	ev, err := a.AddEvidence(ctx, desktop, inv.Ref, app.EvidenceInput{
		Title:      "Billing export shows 214 services at debug level",
		Summary:    "The September billing export lists log ingestion for 214 services that now log at debug level.",
		SourceType: app.SourceTypeManualObservation,
	})
	must("4", err)
	// 5. A claim.
	cl, err := a.AddClaim(ctx, desktop, inv.Ref, app.ClaimInput{
		Statement: "Debug logging is the largest part of the increase.",
	})
	must("5", err)
	// 6. Link the evidence to the claim.
	_, err = a.LinkEvidenceToClaim(ctx, desktop, inv.Ref, cl.Ref, ev.Ref, app.RelationSupports)
	must("6", err)
	// 7. Give the claim a confidence level.
	medium := app.ConfidenceMedium
	_, err = a.UpdateClaim(ctx, desktop, inv.Ref, cl.Ref, app.ClaimPatch{Confidence: &medium})
	must("7", err)
	// 8. A hypothesis.
	_, err = a.AddHypothesis(ctx, desktop, inv.Ref, app.HypothesisInput{Statement: "The new tracing agent may also be shipping some services' logs twice."})
	must("8", err)
	// 9. An open question.
	_, err = a.AddQuestion(ctx, desktop, inv.Ref, app.QuestionInput{Question: "Can every service except payments go back to info level today?"})
	must("9", err)
	// 10. The current conclusion.
	_, err = a.UpdateConclusion(ctx, desktop, inv.Ref, app.ConclusionInput{Conclusion: "Debug logging requested for payments only reached every service and drove most of the increase.", Confidence: app.ConfidenceMedium})
	must("10", err)
	// 11. Mark it active.
	_, err = a.SetActiveInvestigation(ctx, desktop, inv.Ref)
	must("11", err)

	// 12 and 13. Close, reopen, and everything persists.
	must("12 close", a.Close())
	a, err = app.Open(dbPath)
	must("12 reopen", err)
	t.Cleanup(func() { _ = a.Close() })
	det, err := a.GetInvestigationDetail(ctx, "", app.DetailOptions{})
	must("13", err)
	if det.Investigation.Ref != inv.Ref || !det.Investigation.IsActive || det.Investigation.Conclusion == "" ||
		len(det.Evidence) != 1 || len(det.Claims) != 1 || len(det.Hypotheses) != 1 || len(det.Questions) != 1 {
		t.Fatalf("step 13: after reopen got %+v", det)
	}
	if c := det.Claims[0]; c.Confidence != app.ConfidenceMedium || len(c.Links) != 1 || c.Links[0].EvidenceRef != ev.Ref {
		t.Fatalf("step 13: claim after reopen %+v", c)
	}

	// 14. Launch the MCP server (a real subprocess on the same file).
	cs, stderr := connectMCP(t, dbPath, "claude-code", "2025-06-18")
	// 15. Retrieve the same active investigation.
	res := callTool(t, cs, "casebook_get_active_investigation", map[string]any{})
	if res.IsError || !strings.Contains(resultText(res), inv.Ref) {
		t.Fatalf("step 15: %s\nstderr:\n%s", resultText(res), stderr.String())
	}
	// 16. New evidence through MCP.
	res = callTool(t, cs, "casebook_add_evidence", map[string]any{
		"source_type": "sql_query", "title": "Log ingestion is 71% of the September increase",
		"summary": "Log ingestion rose from $12,100 to $108,500, which is $96,400 of the $135,800 increase.",
	})
	var mev mutationJSON
	decodeStructured(t, res, &mev)
	if res.IsError || mev.Ref != "E-002" {
		t.Fatalf("step 16: %s", resultText(res))
	}
	// 17. Add a claim and update the existing one through MCP.
	res = callTool(t, cs, "casebook_add_claim", map[string]any{
		"statement":  "Log ingestion, billed per GB, is $96,400 of the $135,800 increase.",
		"confidence": "high",
		"links":      []map[string]any{{"evidence_ref": "E-002", "relation": "supports"}},
	})
	if res.IsError {
		t.Fatalf("step 17 add: %s", resultText(res))
	}
	res = callTool(t, cs, "casebook_update_claim", map[string]any{"claim_ref": cl.Ref, "confidence": "high"})
	if res.IsError {
		t.Fatalf("step 17 update: %s", resultText(res))
	}

	// 18 and 19. A refreshed view (a fresh engine on the file, as the desktop
	// reloads) shows the MCP-created records with the client's provenance.
	b, err := app.Open(dbPath)
	must("18", err)
	t.Cleanup(func() { _ = b.Close() })
	det, err = b.GetInvestigationDetail(ctx, inv.Ref, app.DetailOptions{})
	must("19", err)
	if len(det.Evidence) != 2 || len(det.Claims) != 2 {
		t.Fatalf("step 19: %d evidence, %d claims", len(det.Evidence), len(det.Claims))
	}
	for _, c := range det.Claims {
		if c.Confidence != app.ConfidenceHigh {
			t.Fatalf("step 19: claim %s confidence %s", c.Ref, c.Confidence)
		}
	}
	e2, err := b.GetEvidence(ctx, inv.Ref, "E-002", app.GetEvidenceOptions{})
	must("19", err)
	if e2.CreatedBy.Kind != "ai" || e2.CreatedBy.Client != "claude-code" {
		t.Fatalf("step 19: E-002 created by %+v", e2.CreatedBy)
	}

	// 20. A compact context pack, through MCP and the engine, names both.
	res = callTool(t, cs, "casebook_get_context", map[string]any{"compact": true})
	pack := resultText(res)
	for _, want := range []string{"Investigation: " + inv.Ref, "C-001", "C-002", "E-002", "Q-001", "H-001", "Debug logging requested for payments only reached every service and drove most of the increase."} {
		if !strings.Contains(pack, want) {
			t.Fatalf("step 20: compact pack lacks %q:\n%s", want, pack)
		}
	}
	if len(pack) > 6000 {
		t.Fatalf("step 20: compact pack is %d bytes", len(pack))
	}
}
