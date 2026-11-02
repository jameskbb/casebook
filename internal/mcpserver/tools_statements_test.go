package mcpserver

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/app/apptest"
)

// wantErrorPrefix asserts res is a tool error whose text starts with prefix.
func wantErrorPrefix(t *testing.T, res *mcp.CallToolResult, prefix string) {
	t.Helper()
	if !res.IsError {
		t.Fatalf("want a tool error starting %q, got success: %s", prefix, resultText(res))
	}
	if txt := resultText(res); !strings.HasPrefix(txt, prefix) {
		t.Fatalf("tool error %q does not start with %q", txt, prefix)
	}
}

// structuredKeys returns the top-level keys of a JSON object value.
func structuredKeys(t *testing.T, v any) map[string]any {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("not a JSON object: %s", b)
	}
	return m
}

func TestAddClaim(t *testing.T) {
	a, f := seeded(t)
	cs := newTestSession(t, a, sessionOpts{})
	assertToolMeta(t, cs, "casebook_add_claim")
	ctx := context.Background()

	// Error cases run first so the happy path proves no C ref was consumed.
	errorCases := []struct {
		name string
		args map[string]any
		want string
	}{
		{"unknown evidence", map[string]any{
			"statement": "x",
			"links":     []any{map[string]any{"evidence_ref": "E-099", "relation": "supports"}},
		}, "not found: "},
		{"bad relation", map[string]any{
			"statement": "x",
			"links":     []any{map[string]any{"evidence_ref": "E-001", "relation": "maybe"}},
		}, "relation"},
		{"missing statement", map[string]any{"confidence": "low"}, "statement"},
		{"superseded status", map[string]any{"statement": "x", "status": "superseded"}, "invalid status"},
		{"bad confidence", map[string]any{"statement": "x", "confidence": "certain"}, "confidence"},
		{"unknown investigation", map[string]any{"statement": "x", "investigation_ref": "INV-0099"}, "not found: "},
	}
	for _, c := range errorCases {
		t.Run(c.name, func(t *testing.T) {
			res := callTool(t, cs, "casebook_add_claim", c.args)
			wantToolError(t, res, c.want)
			if c.want == "not found: " || c.want == "invalid status" {
				wantErrorPrefix(t, res, c.want)
			}
		})
	}

	t.Run("happy path with plain, lowercase and qualified links", func(t *testing.T) {
		res := callTool(t, cs, "casebook_add_claim", map[string]any{
			"statement":  "The billing export lags one day.",
			"confidence": "medium",
			"rationale":  "Export runs nightly.",
			"links": []any{
				map[string]any{"evidence_ref": "e-1", "relation": "supports"},
				map[string]any{"evidence_ref": "INV-0001/E-003", "relation": "contradicts"},
			},
		})
		out := decode[mutationResult[statementRecord]](t, res)
		if out.Ref != "C-004" || out.InvestigationRef != f.InvestigationRef || out.Action != "created" {
			t.Fatalf("out = %+v", out)
		}
		if len(out.Changed) != 0 {
			t.Errorf("changed = %v", out.Changed)
		}
		if !reflect.DeepEqual(out.Record.Supports, []string{"E-001"}) ||
			!reflect.DeepEqual(out.Record.Contradicts, []string{"E-003"}) {
			t.Errorf("links = %v / %v", out.Record.Supports, out.Record.Contradicts)
		}
		if out.Record.Verification != app.VerificationAIGenerated || out.Record.Confidence != app.ConfidenceMedium ||
			out.Record.Status != string(app.ClaimStatusActive) {
			t.Errorf("record = %+v", out.Record)
		}
		rec := structuredKeys(t, structuredKeys(t, res.StructuredContent)["record"])
		if _, ok := rec["text"]; ok {
			t.Error("record echoes the statement text")
		}
		want := "Added claim C-004 to INV-0001 (medium, ai_generated; supports E-001; contradicts E-003)."
		if txt := resultText(res); txt != want {
			t.Errorf("text = %q, want %q", txt, want)
		}

		c, err := a.GetClaim(ctx, "INV-0001", "C-004")
		if err != nil {
			t.Fatal(err)
		}
		if c.ConfidenceRationale != "Export runs nightly." || c.Statement != "The billing export lags one day." {
			t.Errorf("claim = %+v", c)
		}
		if c.CreatedBy != apptest.AI || len(c.Links) != 2 {
			t.Fatalf("claim created_by %+v links %+v", c.CreatedBy, c.Links)
		}
		for _, l := range c.Links {
			if l.CreatedBy != apptest.AI {
				t.Errorf("link %s created_by = %+v", l.EvidenceRef, l.CreatedBy)
			}
		}
	})

	t.Run("no links", func(t *testing.T) {
		res := callTool(t, cs, "casebook_add_claim", map[string]any{"statement": "Another claim."})
		out := decode[mutationResult[statementRecord]](t, res)
		if out.Ref != "C-005" || out.Record.Confidence != app.ConfidenceUnknown {
			t.Fatalf("out = %+v", out)
		}
		if txt := resultText(res); txt != "Added claim C-005 to INV-0001 (unknown, ai_generated)." {
			t.Errorf("text = %q", txt)
		}
	})
}

func TestAddClaimNoActive(t *testing.T) {
	a := apptest.Open(t)
	cs := newTestSession(t, a, sessionOpts{})
	res := callTool(t, cs, "casebook_add_claim", map[string]any{"statement": "x"})
	wantToolError(t, res, "casebook_set_active_investigation")
}

func TestUpdateClaim(t *testing.T) {
	a, _ := seeded(t)
	cs := newTestSession(t, a, sessionOpts{})
	assertToolMeta(t, cs, "casebook_update_claim")
	ctx := context.Background()

	t.Run("confidence and rationale", func(t *testing.T) {
		args := map[string]any{"claim_ref": "c-1", "confidence": "medium", "rationale": "Rechecked."}
		res := callTool(t, cs, "casebook_update_claim", args)
		out := decode[mutationResult[statementRecord]](t, res)
		if out.Ref != "C-001" || out.InvestigationRef != "INV-0001" || out.Action != "updated" ||
			!reflect.DeepEqual(out.Changed, []string{"confidence", "rationale"}) {
			t.Fatalf("out = %+v", out)
		}
		if out.Record.Confidence != app.ConfidenceMedium {
			t.Errorf("record = %+v", out.Record)
		}
		if txt := resultText(res); txt != "Updated C-001: confidence, rationale." {
			t.Errorf("text = %q", txt)
		}
		c, err := a.GetClaim(ctx, "", "C-001")
		if err != nil {
			t.Fatal(err)
		}
		if c.ConfidenceRationale != "Rechecked." {
			t.Errorf("confidence_rationale = %q", c.ConfidenceRationale)
		}

		res = callTool(t, cs, "casebook_update_claim", args)
		out = decode[mutationResult[statementRecord]](t, res)
		if out.Action != "unchanged" || len(out.Changed) != 0 {
			t.Fatalf("repeat out = %+v", out)
		}
		if txt := resultText(res); txt != "No change to C-001." {
			t.Errorf("repeat text = %q", txt)
		}
	})

	t.Run("null means unchanged", func(t *testing.T) {
		res := callTool(t, cs, "casebook_update_claim", map[string]any{
			"claim_ref": "C-001", "statement": "The chart default sets every service to debug.", "confidence": nil,
		})
		out := decode[mutationResult[statementRecord]](t, res)
		// C-001 was human_verified; an AI statement edit lowers it (ADR-0014).
		if out.Action != "updated" || !reflect.DeepEqual(out.Changed, []string{"statement", "verification"}) ||
			out.Record.Verification != app.VerificationAIGenerated {
			t.Fatalf("out = %+v", out)
		}
		if out.Record.Confidence != app.ConfidenceMedium {
			t.Errorf("confidence changed: %+v", out.Record)
		}
	})

	t.Run("supersede", func(t *testing.T) {
		res := callTool(t, cs, "casebook_update_claim", map[string]any{"claim_ref": "C-001", "superseded_by": "c-2"})
		out := decode[mutationResult[statementRecord]](t, res)
		if out.Action != "superseded" || out.Record.Status != string(app.ClaimStatusSuperseded) ||
			out.Record.SupersededBy != "C-002" {
			t.Fatalf("out = %+v", out)
		}
		if !reflect.DeepEqual(out.Changed, []string{"status", "verification", "superseded_by"}) {
			t.Errorf("changed = %v", out.Changed)
		}
		if txt := resultText(res); txt != "C-001 is superseded by C-002." {
			t.Errorf("text = %q", txt)
		}
	})

	errorCases := []struct {
		name   string
		args   map[string]any
		prefix string
	}{
		{"supersede by a superseded claim", map[string]any{"claim_ref": "C-002", "superseded_by": "C-003"}, "conflict"},
		{"empty patch", map[string]any{"claim_ref": "C-001"}, "invalid patch"},
		{"wrong ref kind", map[string]any{"claim_ref": "E-001", "statement": "x"}, "invalid ref"},
		{"unknown claim", map[string]any{"claim_ref": "C-099", "statement": "x"}, "not found:"},
		{"bad status", map[string]any{"claim_ref": "C-002", "status": "bogus"}, ""},
		{"missing claim_ref", map[string]any{"statement": "x"}, ""},
		{"verification superseded is not offered", map[string]any{"claim_ref": "C-001", "verification": "superseded"}, ""},
	}
	// Status superseded without a replacement points at superseded_by, never
	// at an engine method the model cannot call (AR-16).
	t.Run("superseded status without superseded_by", func(t *testing.T) {
		added := decode[mutationResult[statementRecord]](t, callTool(t, cs, "casebook_add_claim", map[string]any{"statement": "Fresh claim."}))
		res := callTool(t, cs, "casebook_update_claim", map[string]any{"claim_ref": added.Ref, "status": "superseded"})
		wantToolError(t, res, "superseded_by")
		if txt := resultText(res); strings.Contains(txt, "Supersede") {
			t.Errorf("error names an engine method: %s", txt)
		}
	})
	for _, c := range errorCases {
		t.Run(c.name, func(t *testing.T) {
			res := callTool(t, cs, "casebook_update_claim", c.args)
			if c.prefix == "" {
				wantToolError(t, res, "")
				return
			}
			wantErrorPrefix(t, res, c.prefix)
		})
	}
}

// linkCases runs the link, relink, no-op, unlink and error cases of a link
// tool on one record and one evidence ref the record is not linked to.
func linkCases(t *testing.T, cs *mcp.ClientSession, tool, refField, ref string) {
	t.Helper()
	args := func(extra map[string]any) map[string]any {
		m := map[string]any{refField: ref, "evidence_ref": "e-4"}
		for k, v := range extra {
			m[k] = v
		}
		return m
	}
	steps := []struct {
		extra       map[string]any
		action      string
		changed     []string
		supports    bool
		contradicts bool
		text        string
	}{
		{map[string]any{"relation": "supports"}, "linked", []string{"links"}, true, false,
			"Linked E-004 to " + ref + " as supports."},
		{map[string]any{"relation": "contradicts"}, "relinked", []string{"links"}, false, true,
			"Relinked E-004 to " + ref + " as contradicts."},
		{map[string]any{"relation": "contradicts"}, "unchanged", []string{}, false, true,
			"No change to " + ref + "."},
		{map[string]any{"unlink": true}, "unlinked", []string{"links"}, false, false,
			"Unlinked E-004 from " + ref + "."},
	}
	for _, s := range steps {
		res := callTool(t, cs, tool, args(s.extra))
		out := decode[mutationResult[statementRecord]](t, res)
		if out.Ref != ref || out.InvestigationRef != "INV-0001" || out.Action != s.action ||
			!reflect.DeepEqual(out.Changed, s.changed) {
			t.Fatalf("%v: out = %+v", s.extra, out)
		}
		if slicesContains(out.Record.Supports, "E-004") != s.supports ||
			slicesContains(out.Record.Contradicts, "E-004") != s.contradicts {
			t.Errorf("%v: record = %+v", s.extra, out.Record)
		}
		if txt := resultText(res); txt != s.text {
			t.Errorf("%v: text = %q, want %q", s.extra, txt, s.text)
		}
	}
	wantErrorPrefix(t, callTool(t, cs, tool, args(map[string]any{"unlink": true})), "not found:")
	wantErrorPrefix(t, callTool(t, cs, tool, args(map[string]any{"relation": "supports", "unlink": true})), "invalid relation")
	wantErrorPrefix(t, callTool(t, cs, tool, args(nil)), "invalid relation")
	wantToolError(t, callTool(t, cs, tool, args(map[string]any{"relation": "maybe"})), "relation")
	wantErrorPrefix(t, callTool(t, cs, tool, args(map[string]any{"evidence_ref": "E-099", "relation": "supports"})), "not found:")
	wantErrorPrefix(t, callTool(t, cs, tool, args(map[string]any{"evidence_ref": "INV-0002/E-001", "relation": "supports"})), "conflict")
}

func slicesContains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// seedSecondInvestigation adds INV-0002 with E-001 without changing the
// active investigation.
func seedSecondInvestigation(t *testing.T, a *app.App) {
	t.Helper()
	ctx := context.Background()
	inv, err := a.CreateInvestigation(ctx, apptest.Human, app.CreateInvestigationInput{Title: "Other"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.AddEvidence(ctx, apptest.Human, inv.Ref, app.EvidenceInput{
		Title: "Other evidence", Summary: "Elsewhere.", SourceType: app.SourceTypeFile,
	}); err != nil {
		t.Fatal(err)
	}
}

func TestLinkEvidenceToClaim(t *testing.T) {
	a, _ := seeded(t)
	seedSecondInvestigation(t, a)
	cs := newTestSession(t, a, sessionOpts{})
	assertToolMeta(t, cs, "casebook_link_evidence_to_claim")
	linkCases(t, cs, "casebook_link_evidence_to_claim", "claim_ref", "C-002")

	t.Run("qualified refs and provenance", func(t *testing.T) {
		res := callTool(t, cs, "casebook_link_evidence_to_claim", map[string]any{
			"claim_ref": "INV-0001/C-002", "evidence_ref": "INV-0001/E-001", "relation": "supports",
		})
		out := decode[mutationResult[statementRecord]](t, res)
		if out.Action != "linked" {
			t.Fatalf("out = %+v", out)
		}
		c, err := a.GetClaim(context.Background(), "INV-0001", "C-002")
		if err != nil {
			t.Fatal(err)
		}
		for _, l := range c.Links {
			if l.EvidenceRef == "E-001" && l.CreatedBy != apptest.AI {
				t.Errorf("link created_by = %+v", l.CreatedBy)
			}
		}
	})
	t.Run("wrong record kind", func(t *testing.T) {
		wantErrorPrefix(t, callTool(t, cs, "casebook_link_evidence_to_claim", map[string]any{
			"claim_ref": "H-001", "evidence_ref": "E-001", "relation": "supports",
		}), "invalid ref")
	})
}

func TestAddHypothesis(t *testing.T) {
	a, _ := seeded(t)
	cs := newTestSession(t, a, sessionOpts{})
	assertToolMeta(t, cs, "casebook_add_hypothesis")

	res := callTool(t, cs, "casebook_add_hypothesis", map[string]any{
		"statement": "Debug logs are kept longer than info logs.",
		"links":     []any{map[string]any{"evidence_ref": "E-002", "relation": "supports"}},
	})
	out := decode[mutationResult[statementRecord]](t, res)
	if out.Ref != "H-003" || out.InvestigationRef != "INV-0001" || out.Action != "created" ||
		out.Record.Status != string(app.HypothesisStatusOpen) || !reflect.DeepEqual(out.Record.Supports, []string{"E-002"}) {
		t.Fatalf("out = %+v", out)
	}
	if txt := resultText(res); txt != "Added hypothesis H-003 to INV-0001 (unknown, ai_generated; supports E-002)." {
		t.Errorf("text = %q", txt)
	}
	h, err := a.GetHypothesis(context.Background(), "", "H-003")
	if err != nil {
		t.Fatal(err)
	}
	if h.CreatedBy != apptest.AI || h.Statement != "Debug logs are kept longer than info logs." {
		t.Errorf("hypothesis = %+v", h)
	}

	t.Run("rationale maps to rationale", func(t *testing.T) {
		out := decode[mutationResult[statementRecord]](t, callTool(t, cs, "casebook_add_hypothesis", map[string]any{
			"statement": "Rounding differs.", "confidence": "low", "rationale": "Seen once.",
		}))
		h, err := a.GetHypothesis(context.Background(), "", out.Ref)
		if err != nil {
			t.Fatal(err)
		}
		if h.Rationale != "Seen once." || h.Confidence != app.ConfidenceLow {
			t.Errorf("hypothesis = %+v", h)
		}
	})
	t.Run("missing statement", func(t *testing.T) {
		wantToolError(t, callTool(t, cs, "casebook_add_hypothesis", map[string]any{"confidence": "low"}), "statement")
	})
	t.Run("superseded status", func(t *testing.T) {
		wantErrorPrefix(t, callTool(t, cs, "casebook_add_hypothesis", map[string]any{
			"statement": "x", "status": "superseded",
		}), "invalid status")
	})
	t.Run("unknown evidence", func(t *testing.T) {
		wantErrorPrefix(t, callTool(t, cs, "casebook_add_hypothesis", map[string]any{
			"statement": "x", "links": []any{map[string]any{"evidence_ref": "E-099", "relation": "contradicts"}},
		}), "not found:")
	})
}

func TestUpdateHypothesis(t *testing.T) {
	a, _ := seeded(t)
	cs := newTestSession(t, a, sessionOpts{})
	assertToolMeta(t, cs, "casebook_update_hypothesis")

	res := callTool(t, cs, "casebook_update_hypothesis", map[string]any{"hypothesis_ref": "H-001", "status": "supported"})
	out := decode[mutationResult[statementRecord]](t, res)
	if out.Ref != "H-001" || out.Action != "updated" || !reflect.DeepEqual(out.Changed, []string{"status"}) ||
		out.Record.Status != string(app.HypothesisStatusSupported) {
		t.Fatalf("out = %+v", out)
	}
	if txt := resultText(res); txt != "Updated H-001: status." {
		t.Errorf("text = %q", txt)
	}

	t.Run("rationale", func(t *testing.T) {
		out := decode[mutationResult[statementRecord]](t, callTool(t, cs, "casebook_update_hypothesis", map[string]any{
			"hypothesis_ref": "h-1", "rationale": "Matches E-002.",
		}))
		if !reflect.DeepEqual(out.Changed, []string{"rationale"}) {
			t.Fatalf("out = %+v", out)
		}
		h, err := a.GetHypothesis(context.Background(), "", "H-001")
		if err != nil {
			t.Fatal(err)
		}
		if h.Rationale != "Matches E-002." {
			t.Errorf("rationale = %q", h.Rationale)
		}
	})
	t.Run("supersede", func(t *testing.T) {
		out := decode[mutationResult[statementRecord]](t, callTool(t, cs, "casebook_update_hypothesis", map[string]any{
			"hypothesis_ref": "H-002", "superseded_by": "H-001",
		}))
		if out.Action != "superseded" || out.Record.SupersededBy != "H-001" {
			t.Fatalf("out = %+v", out)
		}
	})
	t.Run("errors", func(t *testing.T) {
		wantToolError(t, callTool(t, cs, "casebook_update_hypothesis", map[string]any{"hypothesis_ref": "H-001", "status": "bogus"}), "status")
		wantErrorPrefix(t, callTool(t, cs, "casebook_update_hypothesis", map[string]any{"hypothesis_ref": "H-001"}), "invalid patch")
		wantErrorPrefix(t, callTool(t, cs, "casebook_update_hypothesis", map[string]any{"hypothesis_ref": "C-001", "statement": "x"}), "invalid ref")
		wantErrorPrefix(t, callTool(t, cs, "casebook_update_hypothesis", map[string]any{"hypothesis_ref": "H-099", "statement": "x"}), "not found:")
	})
}

func TestLinkEvidenceToHypothesis(t *testing.T) {
	a, _ := seeded(t)
	seedSecondInvestigation(t, a)
	cs := newTestSession(t, a, sessionOpts{})
	assertToolMeta(t, cs, "casebook_link_evidence_to_hypothesis")
	linkCases(t, cs, "casebook_link_evidence_to_hypothesis", "hypothesis_ref", "H-001")
}

func TestStatementToolsNoActive(t *testing.T) {
	a := apptest.Open(t)
	cs := newTestSession(t, a, sessionOpts{})
	cases := map[string]map[string]any{
		"casebook_add_claim":                   {"statement": "x"},
		"casebook_update_claim":                {"claim_ref": "C-001", "statement": "x"},
		"casebook_link_evidence_to_claim":      {"claim_ref": "C-001", "evidence_ref": "E-001", "relation": "supports"},
		"casebook_add_hypothesis":              {"statement": "x"},
		"casebook_update_hypothesis":           {"hypothesis_ref": "H-001", "statement": "x"},
		"casebook_link_evidence_to_hypothesis": {"hypothesis_ref": "H-001", "evidence_ref": "E-001", "unlink": true},
	}
	for tool, args := range cases {
		t.Run(tool, func(t *testing.T) {
			wantToolError(t, callTool(t, cs, tool, args), "casebook_set_active_investigation")
		})
	}
}
