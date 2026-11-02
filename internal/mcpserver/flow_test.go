package mcpserver

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/app/apptest"
)

// TestMCPBriefFlow is the brief's MCP flow on an empty database, through MCP
// calls only: create investigation, add evidence, add claim, link evidence,
// update conclusion, get investigation, get context (D-22, MCP-25).
// internal/app is used only to read the activity log at the end.
func TestMCPBriefFlow(t *testing.T) {
	a := apptest.Open(t)
	cs := newTestSession(t, a, sessionOpts{ClientName: "claude-code"})

	inv := decode[mutationResult[investigationRecord]](t, callTool(t, cs, "casebook_create_investigation", map[string]any{
		"title":             "September cloud bill up 38%",
		"problem_statement": "Why did the cloud bill rise 38% in September when traffic was flat?",
		"make_active":       true,
	}))
	if inv.Ref != "INV-0001" || !inv.Record.IsActive {
		t.Fatalf("create = %+v", inv)
	}

	e1 := decode[mutationResult[evidenceRecord]](t, callTool(t, cs, "casebook_add_evidence", map[string]any{
		"title": "Chart default log level is debug", "summary": "The shared service chart sets every service to debug logging.",
		"source_type": "code", "source_ref": "platform-charts/service/values.yaml",
	}))
	e2 := decode[mutationResult[evidenceRecord]](t, callTool(t, cs, "casebook_add_evidence", map[string]any{
		"title": "Traces line rose $29,800", "summary": "The billing export shows traces up $29,800, which debug logging does not explain.",
		"source_type": "sql_query",
	}))
	if e1.Ref != "E-001" || e2.Ref != "E-002" {
		t.Fatalf("evidence refs %s, %s", e1.Ref, e2.Ref)
	}

	claim := decode[mutationResult[statementRecord]](t, callTool(t, cs, "casebook_add_claim", map[string]any{
		"statement": "Debug logging explains the whole increase.", "confidence": "medium",
		"links": []any{map[string]any{"evidence_ref": "E-001", "relation": "supports"}},
	}))
	if claim.Ref != "C-001" {
		t.Fatalf("claim = %+v", claim)
	}
	link := decode[mutationResult[statementRecord]](t, callTool(t, cs, "casebook_link_evidence_to_claim", map[string]any{
		"claim_ref": "C-001", "evidence_ref": "E-002", "relation": "contradicts",
	}))
	if link.Action != "linked" || !slices.Equal(link.Record.Contradicts, []string{"E-002"}) {
		t.Fatalf("link = %+v", link)
	}

	conclusion := "Debug logging drove most of the increase; traces are still open."
	upd := decode[mutationResult[investigationRecord]](t, callTool(t, cs, "casebook_update_conclusion", map[string]any{
		"conclusion": conclusion, "confidence": "high", "rationale": "C-001, backed by E-001.",
	}))
	if upd.Action != "updated" {
		t.Fatalf("update conclusion = %+v", upd)
	}

	view := decode[investigationView](t, callTool(t, cs, "casebook_get_investigation", map[string]any{}))
	if view.Conclusion != conclusion || view.ConclusionConfidence != app.ConfidenceHigh || view.Claims == nil ||
		len(view.Claims.Items) != 1 {
		t.Fatalf("view = %+v", view)
	}
	c := view.Claims.Items[0]
	if c.Ref != "C-001" || !slices.Equal(c.Supports, []string{"E-001"}) || !slices.Equal(c.Contradicts, []string{"E-002"}) {
		t.Fatalf("claim in view = %+v", c)
	}

	pack := decode[contextOutput](t, callTool(t, cs, "casebook_get_context", map[string]any{}))
	for _, want := range []string{"C-001", "E-001", "E-002", conclusion} {
		if !strings.Contains(pack.Context, want) {
			t.Errorf("context lacks %q", want)
		}
	}

	rows, err := a.ListActivity(context.Background(), app.ActivityFilter{Limit: 500})
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, r := range rows {
		if r.Actor != app.ActorMCP("claude-code") {
			t.Errorf("activity %s %s by %+v, want {ai, claude-code}", r.Action, r.TargetRef, r.Actor)
		}
		counts[r.Action]++
	}
	want := map[string]int{
		"investigation.create": 1, "active.set": 1, "evidence.add": 2, "claim.add": 1, "link.add": 1, "conclusion.update": 1,
	}
	for action, n := range want {
		if counts[action] != n {
			t.Errorf("%s rows = %d, want %d (all: %v)", action, counts[action], n, counts)
		}
	}
}
