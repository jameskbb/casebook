package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os/exec"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/jameskbb/casebook/internal/app"
)

// recordRefPattern finds record refs (E, C, H, Q) in rendered text.
var recordRefPattern = regexp.MustCompile(`\b[ECHQ]-\d{3,}\b`)

// refSet returns the distinct record refs in s, sorted.
func refSet(s string) []string {
	refs := recordRefPattern.FindAllString(s, -1)
	slices.Sort(refs)
	return slices.Compact(refs)
}

// TestCrossSurface is the brief's cross-surface integration test (D-23,
// MCP-26): the desktop/core layer, a real `casebook mcp` subprocess and the
// CLI binary share one database file and see the same state.
func TestCrossSurface(t *testing.T) {
	ctx := context.Background()
	a, dbPath := openApp(t)
	desktop := app.ActorDesktop()

	// 1. The core layer (actor desktop) creates INV-0001 with E-001 and C-001.
	inv, err := a.CreateInvestigation(ctx, desktop, app.CreateInvestigationInput{
		Title:            "September cloud bill up 38%",
		ProblemStatement: "Why did the cloud bill rise 38% in September when traffic was flat?",
		MakeActive:       true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if inv.Ref != "INV-0001" {
		t.Fatalf("investigation ref = %s", inv.Ref)
	}
	if _, err := a.AddEvidence(ctx, desktop, inv.Ref, app.EvidenceInput{
		Title: "Chart default log level is debug", Summary: "The shared service chart sets every service to debug logging.", SourceType: app.SourceTypeCode,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.AddClaim(ctx, desktop, inv.Ref, app.ClaimInput{
		Statement: "A shared chart default moved every service to debug logging.",
		Links:     []app.EvidenceLinkInput{{EvidenceRef: "E-001", Relation: app.RelationSupports}},
	}); err != nil {
		t.Fatal(err)
	}

	// 2. A real casebook mcp subprocess reads it.
	cs, stderr := connectMCP(t, dbPath, "codex-mcp-client", "2025-06-18")
	res := callTool(t, cs, "casebook_get_investigation", map[string]any{"investigation_ref": "INV-0001"})
	if res.IsError {
		t.Fatalf("get investigation: %s\nstderr:\n%s", resultText(res), stderr.String())
	}
	var view struct {
		Ref   string `json:"ref"`
		Title string `json:"title"`
	}
	decodeStructured(t, res, &view)
	got := resultText(res)
	if view.Ref != "INV-0001" || view.Title != inv.Title || !strings.Contains(got, "C-001") || !strings.Contains(got, "E-001") {
		t.Fatalf("MCP view %+v, text:\n%s", view, got)
	}

	// 3. Evidence and a claim are added through MCP.
	res = callTool(t, cs, "casebook_add_evidence", map[string]any{
		"title": "Log ingestion leads the increase", "summary": "The billing export shows log ingestion up $96,400 month over month.",
		"source_type": "sql_query",
	})
	var ev mutationJSON
	decodeStructured(t, res, &ev)
	if res.IsError || ev.Ref != "E-002" {
		t.Fatalf("add evidence: %s", resultText(res))
	}
	res = callTool(t, cs, "casebook_add_claim", map[string]any{
		"statement": "Log ingestion drives most of the increase.",
		"links":     []any{map[string]any{"evidence_ref": "E-002", "relation": "supports"}},
	})
	var cl mutationJSON
	decodeStructured(t, res, &cl)
	if res.IsError || cl.Ref != "C-002" {
		t.Fatalf("add claim: %s", resultText(res))
	}

	// 4. The core layer reads the MCP evidence with the MCP client's actor.
	e2, err := a.GetEvidence(ctx, "INV-0001", "E-002", app.GetEvidenceOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if e2.CreatedBy != app.ActorMCP("codex") {
		t.Fatalf("E-002 created_by = %+v, want {ai, codex}", e2.CreatedBy)
	}

	// The investigation detail lists the same refs on both sides.
	detail, err := a.GetInvestigationDetail(ctx, "INV-0001", app.DetailOptions{})
	if err != nil {
		t.Fatal(err)
	}
	detailJSON, err := json.Marshal(detail)
	if err != nil {
		t.Fatal(err)
	}
	res = callTool(t, cs, "casebook_get_investigation", map[string]any{"investigation_ref": "INV-0001"})
	mcpJSON, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	wantRefs := []string{"C-001", "C-002", "E-001", "E-002"}
	if a, m := refSet(string(detailJSON)), refSet(string(mcpJSON)); !slices.Equal(a, wantRefs) || !slices.Equal(m, wantRefs) {
		t.Fatalf("detail refs: app %v, MCP %v, want %v", a, m, wantRefs)
	}

	// 5 and 6. Context through the core layer, MCP and the CLI binary is
	// byte-identical and lists the same refs, given the same budget: the
	// defaults differ (no limit for the CLI, 24000 bytes for MCP), so every
	// side passes 24000 explicitly.
	for _, f := range []struct{ app, mcp, cli string }{
		{"markdown", "markdown", "md"},
		{"json", "json", "json"},
	} {
		want, err := a.RenderContext(ctx, "INV-0001", f.app, app.ContextOptions{MaxBytes: 24000})
		if err != nil {
			t.Fatal(err)
		}
		res := callTool(t, cs, "casebook_get_context", map[string]any{
			"investigation_ref": "INV-0001", "format": f.mcp, "max_bytes": 24000,
		})
		if res.IsError {
			t.Fatalf("get context %s: %s", f.mcp, resultText(res))
		}
		if txt := resultText(res); txt != app.FrameContext(want) {
			t.Errorf("%s: casebook_get_context text is not the framed app.RenderContext:\n%s\n---\n%s", f.mcp, txt, want)
		}
		var out struct {
			Context string `json:"context"`
		}
		decodeStructured(t, res, &out)
		if out.Context != app.FrameContext(want) {
			t.Errorf("%s: structured context is not the framed app.RenderContext", f.mcp)
		}
		if refs := refSet(want); !slices.Equal(refs, wantRefs) || !slices.Equal(refSet(out.Context), refs) {
			t.Errorf("%s: refs app %v, MCP %v, want %v", f.mcp, refs, refSet(out.Context), wantRefs)
		}

		cmd := exec.Command(binary(t), "context", "INV-0001", "--format", f.cli, "--max-bytes", "24000", "--db", dbPath)
		cmd.Env = childEnv(t)
		var stdout, cliErr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &cliErr
		if err := cmd.Run(); err != nil {
			t.Fatalf("casebook context --format %s: %v\nstderr:\n%s", f.cli, err, cliErr.String())
		}
		wantCLI := want
		if !strings.HasSuffix(wantCLI, "\n") {
			wantCLI += "\n"
		}
		if stdout.String() != wantCLI {
			t.Errorf("%s: casebook context output differs from app.RenderContext:\n%s\n---\n%s", f.cli, stdout.String(), wantCLI)
		}
	}
	// The compact pack (what the context resources and hooks use) agrees too.
	compact, err := a.RenderContext(ctx, "INV-0001", "markdown", app.ContextOptions{Compact: true})
	if err != nil {
		t.Fatal(err)
	}
	res = callTool(t, cs, "casebook_get_context", map[string]any{"investigation_ref": "INV-0001", "compact": true})
	if res.IsError || resultText(res) != app.FrameContext(compact) {
		t.Errorf("compact casebook_get_context differs from app.RenderContext:\n%s\n---\n%s", resultText(res), compact)
	}

	// connectMCP's cleanup closes the session and fails the test unless the
	// subprocess exits cleanly.
}
