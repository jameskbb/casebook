package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/app/apptest"
)

// sizeAlphabet mixes ASCII with two- and three-byte runes so byte limits and
// cuts land inside multibyte runes.
const sizeAlphabet = "Billing check: the debug logs ship nightly; é ü 中文 ok. "

// longText returns valid UTF-8 of at most n bytes (n minus at most 2), built
// from sizeAlphabet and tagged with tag so texts differ.
func longText(tag string, n int) string {
	s := tag + " " + strings.Repeat(sizeAlphabet, n/len(sizeAlphabet)+1)
	s = s[:n]
	for !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return s
}

// seedLarge builds a large investigation through internal/app only and
// leaves it active: long header fields, 120 evidence (five with 64 KiB raw
// detail, E-001 also with near-16 KiB metadata), 150 claims and 150
// hypotheses each linked to three evidence, 150 questions (50 resolved), and
// 120 other investigations. It returns the big investigation's ref.
func seedLarge(t *testing.T, a *app.App) string {
	t.Helper()
	ctx := context.Background()
	actor := apptest.AI
	for i := range 120 {
		if _, err := a.CreateInvestigation(ctx, actor, app.CreateInvestigationInput{
			Title: longText(fmt.Sprintf("Other %d cloud bill", i), app.MaxTitleBytes),
		}); err != nil {
			t.Fatal(err)
		}
	}
	inv, err := a.CreateInvestigation(ctx, actor, app.CreateInvestigationInput{
		Title:            longText("September cloud bill up 38%", app.MaxTitleBytes),
		ProblemStatement: longText("Why did the cloud bill rise", app.MaxProblemStatementBytes),
		Status:           app.InvestigationStatusInvestigating,
		MakeActive:       true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.UpdateConclusion(ctx, actor, inv.Ref, app.ConclusionInput{
		Conclusion: longText("Debug logging", app.MaxConclusionBytes),
		Confidence: app.ConfidenceHigh,
		Rationale:  longText("Because", app.MaxConclusionRationaleBytes),
	}); err != nil {
		t.Fatal(err)
	}
	meta, err := json.Marshal(map[string]string{"notes": longText("meta", app.MaxMetadataBytes-100)})
	if err != nil {
		t.Fatal(err)
	}
	var evRefs []string
	for i := range 120 {
		in := app.EvidenceInput{
			Title:       longText(fmt.Sprintf("Evidence %d", i), app.MaxTitleBytes),
			Summary:     longText(fmt.Sprintf("Summary %d", i), app.MaxEvidenceSummaryBytes),
			SourceType:  app.SourceTypeSQLQuery,
			SourceRef:   longText("query", 500),
			SourceLabel: longText("billing", app.MaxSourceLabelBytes),
		}
		if i < 5 {
			in.RawDetail = longText("raw", app.MaxRawDetailBytes)
		}
		if i == 0 {
			in.Metadata = string(meta)
		}
		ev, err := a.AddEvidence(ctx, actor, inv.Ref, in)
		if err != nil {
			t.Fatal(err)
		}
		evRefs = append(evRefs, ev.Ref)
	}
	links := func(i int) []app.EvidenceLinkInput {
		return []app.EvidenceLinkInput{
			{EvidenceRef: evRefs[i%120], Relation: app.RelationSupports},
			{EvidenceRef: evRefs[(i+1)%120], Relation: app.RelationSupports},
			{EvidenceRef: evRefs[(i+2)%120], Relation: app.RelationContradicts},
		}
	}
	for i := range 150 {
		if _, err := a.AddClaim(ctx, actor, inv.Ref, app.ClaimInput{
			Statement:           longText(fmt.Sprintf("Claim %d", i), app.MaxStatementBytes),
			Confidence:          app.ConfidenceMedium,
			ConfidenceRationale: longText("Why", app.MaxRationaleBytes),
			Links:               links(i),
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := a.AddHypothesis(ctx, actor, inv.Ref, app.HypothesisInput{
			Statement: longText(fmt.Sprintf("Hypothesis %d", i), app.MaxStatementBytes),
			Rationale: longText("Why", app.MaxRationaleBytes),
			Links:     links(i + 7),
		}); err != nil {
			t.Fatal(err)
		}
		q, err := a.AddQuestion(ctx, actor, inv.Ref, app.QuestionInput{Question: longText(fmt.Sprintf("Question %d", i), app.MaxQuestionBytes)})
		if err != nil {
			t.Fatal(err)
		}
		if i < 50 {
			if _, err := a.ResolveQuestion(ctx, actor, inv.Ref, q.Ref, longText("Answer", app.MaxResolutionBytes)); err != nil {
				t.Fatal(err)
			}
		}
	}
	return inv.Ref
}

func TestResponseSizeBudget(t *testing.T) {
	if testing.Short() {
		t.Skip("large seed; skipped in -short")
	}
	a := apptest.Open(t)
	invRef := seedLarge(t, a)
	cs := newTestSession(t, a, sessionOpts{})

	evidenceArgs := func(tag string) map[string]any {
		return map[string]any{
			"title":        longText(tag, app.MaxTitleBytes),
			"summary":      longText(tag, app.MaxEvidenceSummaryBytes),
			"source_type":  "code",
			"source_ref":   longText(tag, app.MaxSourceRefBytes),
			"source_label": longText(tag, app.MaxSourceLabelBytes),
			"raw_detail":   longText(tag, app.MaxRawDetailBytes),
			"metadata":     map[string]any{"notes": longText(tag, app.MaxMetadataBytes-100)},
		}
	}
	manyLinks := make([]any, 100)
	for i := range manyLinks {
		manyLinks[i] = map[string]any{"evidence_ref": fmt.Sprintf("E-%03d", i+1), "relation": "supports"}
	}
	claimArgs := func(tag string, links []any) map[string]any {
		return map[string]any{
			"statement": longText(tag, app.MaxStatementBytes), "confidence": "high",
			"rationale": longText(tag, app.MaxRationaleBytes), "verification": "source_verified", "links": links,
		}
	}
	checkpointArgs := func() map[string]any {
		var ev, claims, updates, hyps, qs []any
		for i := range 10 {
			e := evidenceArgs(fmt.Sprintf("cp%d", i))
			e["key"] = fmt.Sprintf("k%d", i)
			ev = append(ev, e)
			claims = append(claims, claimArgs(fmt.Sprintf("cp claim %d", i), []any{
				map[string]any{"evidence_ref": fmt.Sprintf("#k%d", i), "relation": "supports"}}))
			updates = append(updates, map[string]any{"ref": fmt.Sprintf("C-%03d", i+1),
				"statement": longText("upd", app.MaxStatementBytes), "rationale": longText("upd", app.MaxRationaleBytes)})
			hyps = append(hyps, map[string]any{"statement": longText("cp hyp", app.MaxStatementBytes),
				"rationale": longText("cp", app.MaxRationaleBytes), "links": []any{
					map[string]any{"evidence_ref": fmt.Sprintf("#k%d", i), "relation": "contradicts"}}})
			qs = append(qs, longText("cp question", app.MaxQuestionBytes))
		}
		return map[string]any{"note": longText("Checkpoint", app.MaxCheckpointNoteBytes),
			"evidence": ev, "claims": claims, "claim_updates": updates, "hypotheses": hyps, "questions": qs}
	}

	type step struct {
		tool  string
		calls []map[string]any
		check func(t *testing.T, res *mcp.CallToolResult)
		// tooLarge: the tool may refuse with its documented "too large for
		// one tool response" error (the note tools return a body that
		// must be passed on whole, so they never cut it).
		tooLarge bool
	}
	structured := func(t *testing.T, res *mcp.CallToolResult) map[string]any {
		t.Helper()
		b, _ := json.Marshal(res.StructuredContent)
		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatal(err)
		}
		return m
	}
	steps := []step{
		{tool: "casebook_list_investigations", calls: []map[string]any{{"limit": 100, "include_archived": true}}},
		{tool: "casebook_search_investigations", calls: []map[string]any{
			{"query": "billing", "limit": 100, "include_archived": true},
			{"query": "debug", "investigation_ref": invRef, "limit": 100},
		}},
		{tool: "casebook_get_investigation", calls: []map[string]any{{"limit": 200}},
			check: func(t *testing.T, res *mcp.CallToolResult) {
				if structured(t, res)["truncated"] != true {
					t.Error("get_investigation did not report truncated")
				}
			}},
		{tool: "casebook_get_active_investigation", calls: []map[string]any{{}}},
		{tool: "casebook_get_evidence", calls: []map[string]any{
			{"evidence_ref": "E-001", "include_raw_detail": true, "max_raw_bytes": 65536},
		}, check: func(t *testing.T, res *mcp.CallToolResult) {
			if structured(t, res)["raw_detail_truncated"] != true {
				t.Error("get_evidence did not report raw_detail_truncated")
			}
		}},
		{tool: "casebook_get_context", calls: []map[string]any{
			{"format": "markdown", "max_bytes": 28000},
			{"format": "json", "max_bytes": 28000},
			{"format": "markdown", "compact": true},
			{"format": "json", "compact": true, "max_bytes": 28000},
		}},
		{tool: "casebook_get_report", calls: []map[string]any{{}, {"timeline_limit": 1000}, {"mode": "brief"}}},
		{tool: "casebook_create_investigation", calls: []map[string]any{{
			"title": longText("New", app.MaxTitleBytes), "problem_statement": longText("New", app.MaxProblemStatementBytes),
			"status": "investigating",
		}}},
		{tool: "casebook_set_active_investigation", calls: []map[string]any{{"investigation_ref": invRef}}},
		{tool: "casebook_update_investigation", calls: []map[string]any{{
			"title": longText("Renamed", app.MaxTitleBytes), "problem_statement": longText("Restated", app.MaxProblemStatementBytes),
			"status": "resolved",
		}}},
		{tool: "casebook_add_evidence", calls: []map[string]any{evidenceArgs("added")}},
		{tool: "casebook_add_claim", calls: []map[string]any{claimArgs("added claim", manyLinks)}},
		{tool: "casebook_update_claim", calls: []map[string]any{{
			"claim_ref": "C-151", "statement": longText("again", app.MaxStatementBytes),
			"rationale": longText("again", app.MaxRationaleBytes), "confidence": "low", "status": "disputed",
			"verification": "disputed",
		}}},
		{tool: "casebook_link_evidence_to_claim", calls: []map[string]any{{"claim_ref": "C-151", "evidence_ref": "E-120", "relation": "contradicts"}}},
		{tool: "casebook_add_hypothesis", calls: []map[string]any{claimArgs("added hypothesis", manyLinks)}},
		{tool: "casebook_update_hypothesis", calls: []map[string]any{{
			"hypothesis_ref": "H-151", "statement": longText("again", app.MaxStatementBytes),
			"rationale": longText("again", app.MaxRationaleBytes), "confidence": "low", "status": "supported",
		}}},
		{tool: "casebook_link_evidence_to_hypothesis", calls: []map[string]any{{"hypothesis_ref": "H-151", "evidence_ref": "E-120", "relation": "contradicts"}}},
		{tool: "casebook_add_question", calls: []map[string]any{{"question": longText("added question", app.MaxQuestionBytes)}}},
		{tool: "casebook_resolve_question", calls: []map[string]any{{"question_ref": "Q-151", "resolution": longText("answer", app.MaxResolutionBytes)}}},
		{tool: "casebook_update_conclusion", calls: []map[string]any{{
			"conclusion": longText("Final", app.MaxConclusionBytes), "confidence": "medium",
			"rationale": longText("Final", app.MaxConclusionRationaleBytes),
		}}},
		{tool: "casebook_checkpoint", calls: []map[string]any{checkpointArgs()}},
	}
	if len(steps) != len(toolSpecs) {
		t.Fatalf("%d steps, want one per tool (%d)", len(steps), len(toolSpecs))
	}
	for _, s := range steps {
		t.Run(s.tool, func(t *testing.T) {
			for i, args := range s.calls {
				res := callTool(t, cs, s.tool, args)
				if res.IsError && s.tooLarge && strings.Contains(resultText(res), "too large for one tool response") {
					t.Logf("call %d: refused as too large", i)
					continue
				}
				if res.IsError {
					t.Fatalf("call %d: tool error: %s", i, resultText(res))
				}
				b, err := json.Marshal(res)
				if err != nil {
					t.Fatal(err)
				}
				t.Logf("call %d: %d bytes", i, len(b))
				if len(b) >= MaxResponseBytes {
					t.Errorf("call %d: result is %d bytes, budget %d", i, len(b), MaxResponseBytes)
				}
				if s.check != nil && i == 0 {
					s.check(t, res)
				}
			}
		})
	}
}

// resultSize is the JSON size of a tool result; it fails the test on a tool
// error or a result at or over the budget.
func resultSize(t *testing.T, res *mcp.CallToolResult) int {
	t.Helper()
	if res.IsError {
		t.Fatalf("tool error: %s", resultText(res))
	}
	b, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) >= MaxResponseBytes {
		t.Fatalf("result is %d bytes, budget %d", len(b), MaxResponseBytes)
	}
	return len(b)
}

// TestGetEvidenceWorstCase reproduces the CR-01 review probes: evidence at
// the app limits with JSON-escape-heavy text, and evidence linked to more
// claims than fit. Every one must be readable within the budget, with the
// cut fields named.
func TestGetEvidenceWorstCase(t *testing.T) {
	if testing.Short() {
		t.Skip("large seed; skipped in -short")
	}
	ctx := context.Background()
	a := apptest.Open(t)
	inv, err := a.CreateInvestigation(ctx, apptest.AI, app.CreateInvestigationInput{Title: "Escapes", MakeActive: true})
	if err != nil {
		t.Fatal(err)
	}
	// Each `<&\"` unit is 4 bytes stored and up to 16 bytes once JSON
	// escaped as a string.
	meta := `{"n":"` + strings.Repeat(`<&\"`, (app.MaxMetadataBytes-8)/4) + `"}`
	if len(meta) > app.MaxMetadataBytes {
		t.Fatalf("metadata is %d bytes", len(meta))
	}
	heavy, err := a.AddEvidence(ctx, apptest.AI, inv.Ref, app.EvidenceInput{
		Title:       strings.Repeat("<", app.MaxTitleBytes),
		Summary:     strings.Repeat("<", app.MaxEvidenceSummaryBytes),
		SourceType:  app.SourceTypeSQLQuery,
		SourceRef:   strings.Repeat("&", app.MaxSourceRefBytes),
		SourceLabel: strings.Repeat(">", app.MaxSourceLabelBytes),
		RawDetail:   strings.Repeat("<", app.MaxRawDetailBytes),
		Metadata:    meta,
	})
	if err != nil {
		t.Fatal(err)
	}
	if heavy.Metadata != meta {
		t.Fatalf("stored metadata differs: %d bytes", len(heavy.Metadata))
	}
	linked, err := a.AddEvidence(ctx, apptest.AI, inv.Ref, app.EvidenceInput{
		Title: "Linked everywhere", Summary: "Supports many claims.", SourceType: app.SourceTypeCode,
	})
	if err != nil {
		t.Fatal(err)
	}
	const claims = 1300
	for i := range claims {
		if _, err := a.AddClaim(ctx, apptest.AI, inv.Ref, app.ClaimInput{
			Statement: fmt.Sprintf("Claim %d", i),
			Links:     []app.EvidenceLinkInput{{EvidenceRef: linked.Ref, Relation: app.RelationSupports}},
		}); err != nil {
			t.Fatal(err)
		}
	}
	cs := newTestSession(t, a, sessionOpts{})

	for _, raw := range []bool{false, true} {
		t.Run(fmt.Sprintf("escape heavy, include_raw_detail %v", raw), func(t *testing.T) {
			res := callTool(t, cs, "casebook_get_evidence", map[string]any{
				"evidence_ref": heavy.Ref, "include_raw_detail": raw, "max_raw_bytes": 65536})
			t.Logf("%d bytes", resultSize(t, res))
			v := decode[evidenceView](t, res)
			if v.Title != heavy.Title || v.SourceLabel != heavy.SourceLabel {
				t.Error("title or source label was cut")
			}
			if !v.Truncated || len(v.TruncatedFields) == 0 {
				t.Fatalf("truncated %v fields %v, want cut fields named", v.Truncated, v.TruncatedFields)
			}
			for name, got := range map[string]string{"summary": v.Summary, "source_ref": v.SourceRef, "metadata": v.Metadata} {
				want := map[string]string{"summary": heavy.Summary, "source_ref": heavy.SourceRef, "metadata": heavy.Metadata}[name]
				cut := slices.Contains(v.TruncatedFields, name)
				if got == "" || (!cut && got != want) || (cut && !strings.HasPrefix(want, strings.TrimSuffix(got, "..."))) {
					t.Errorf("%s: %d bytes, cut %v, does not match the stored text", name, len(got), cut)
				}
			}
			if raw && (v.RawDetailTruncated == nil || !*v.RawDetailTruncated || v.RawDetail == "") {
				t.Error("raw window missing or not marked truncated")
			}
			if txt := resultText(res); !strings.Contains(txt, "cut to fit the 64 KiB response budget") {
				t.Error("text does not say fields were cut")
			}
		})
	}

	for _, raw := range []bool{false, true} {
		t.Run(fmt.Sprintf("1300 claim links, include_raw_detail %v", raw), func(t *testing.T) {
			res := callTool(t, cs, "casebook_get_evidence", map[string]any{"evidence_ref": linked.Ref, "include_raw_detail": raw})
			t.Logf("%d bytes", resultSize(t, res))
			v := decode[evidenceView](t, res)
			if v.ClaimLinksTotal != claims || len(v.ClaimLinks) == 0 || len(v.ClaimLinks) >= claims {
				t.Fatalf("claim links %d of total %d", len(v.ClaimLinks), v.ClaimLinksTotal)
			}
			if !v.Truncated || !slices.Contains(v.TruncatedFields, "claim_links") || slices.Contains(v.TruncatedFields, "summary") {
				t.Errorf("truncated fields %v, want only the link list", v.TruncatedFields)
			}
			if v.Summary != linked.Summary {
				t.Errorf("summary = %q", v.Summary)
			}
			want := fmt.Sprintf("Claims (%d of %d shown):", len(v.ClaimLinks), claims)
			if !strings.Contains(resultText(res), want) {
				t.Errorf("text lacks %q", want)
			}
		})
	}
}

// TestInvestigationHeaderWorstCase reproduces the WR-01 review probe: a
// header at the app limits made of characters JSON escapes to six bytes.
// Both investigation read tools must answer within the budget and name the
// cut header fields.
func TestInvestigationHeaderWorstCase(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t)
	inv, err := a.CreateInvestigation(ctx, apptest.AI, app.CreateInvestigationInput{
		Title:            strings.Repeat("<", app.MaxTitleBytes),
		ProblemStatement: strings.Repeat("<", app.MaxProblemStatementBytes),
		MakeActive:       true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.UpdateConclusion(ctx, apptest.AI, inv.Ref, app.ConclusionInput{
		Conclusion: strings.Repeat("<", app.MaxConclusionBytes),
		Confidence: app.ConfidenceLow,
		Rationale:  strings.Repeat("&", app.MaxConclusionRationaleBytes),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.AddClaim(ctx, apptest.AI, inv.Ref, app.ClaimInput{Statement: strings.Repeat(">", app.MaxStatementBytes)}); err != nil {
		t.Fatal(err)
	}
	cs := newTestSession(t, a, sessionOpts{})

	t.Run("casebook_get_investigation", func(t *testing.T) {
		res := callTool(t, cs, "casebook_get_investigation", map[string]any{"limit": 200})
		t.Logf("%d bytes", resultSize(t, res))
		v := decode[investigationView](t, res)
		if !v.Truncated || len(v.TruncatedFields) == 0 {
			t.Fatalf("truncated %v fields %v, want the cut header fields named", v.Truncated, v.TruncatedFields)
		}
		if v.Title != inv.Title || v.ProblemStatement == "" || v.Conclusion == "" || v.ConclusionRationale == "" {
			t.Error("header fields missing")
		}
		if v.Claims == nil || len(v.Claims.Items) != 1 {
			t.Error("the claim list was dropped")
		}
		if !strings.Contains(resultText(res), "cut to fit the 64 KiB response budget") {
			t.Error("text does not say header fields were cut")
		}
	})
	t.Run("casebook_get_active_investigation", func(t *testing.T) {
		res := callTool(t, cs, "casebook_get_active_investigation", map[string]any{})
		t.Logf("%d bytes", resultSize(t, res))
		out := decode[activeOutput](t, res)
		if out.Investigation == nil || out.Investigation.ProblemStatement == "" || out.Investigation.Conclusion == "" {
			t.Fatalf("output = %+v", out)
		}
		if got := out.Investigation.TruncatedFields; !slices.Equal(got, []string{"problem_statement", "conclusion"}) {
			t.Errorf("truncated_fields = %v", got)
		}
		if !strings.Contains(resultText(res), "cut to fit the 64 KiB response budget") {
			t.Error("text does not say header fields were cut")
		}
	})
}

// TestStatementLinkRefsCapped proves a claim linked to more evidence than
// maxLinkRefs lists at most maxLinkRefs refs per relation and counts the rest.
func TestStatementLinkRefsCapped(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t)
	inv, err := a.CreateInvestigation(ctx, apptest.AI, app.CreateInvestigationInput{Title: "Many links", MakeActive: true})
	if err != nil {
		t.Fatal(err)
	}
	const n = maxLinkRefs + 30
	links := make([]app.EvidenceLinkInput, 0, n)
	for i := 0; i < n; i++ {
		ev, err := a.AddEvidence(ctx, apptest.AI, inv.Ref, app.EvidenceInput{
			Title: "E", Summary: "s", SourceType: app.SourceTypeCode,
		})
		if err != nil {
			t.Fatal(err)
		}
		links = append(links, app.EvidenceLinkInput{EvidenceRef: ev.Ref, Relation: app.RelationSupports})
	}
	if _, err := a.AddClaim(ctx, apptest.AI, inv.Ref, app.ClaimInput{Statement: "Heavily linked", Links: links}); err != nil {
		t.Fatal(err)
	}
	cs := newTestSession(t, a, sessionOpts{})
	res := callTool(t, cs, "casebook_get_investigation", map[string]any{"sections": []string{"claims"}})
	v := decode[investigationView](t, res)
	if v.Claims == nil || len(v.Claims.Items) != 1 {
		t.Fatalf("claims = %+v", v.Claims)
	}
	c := v.Claims.Items[0]
	if len(c.Supports) != maxLinkRefs || c.LinksOmitted != n-maxLinkRefs {
		t.Fatalf("supports %d omitted %d, want %d and %d", len(c.Supports), c.LinksOmitted, maxLinkRefs, n-maxLinkRefs)
	}
	if !strings.Contains(resultText(res), "30 more links omitted") {
		t.Errorf("text does not report omitted links: %s", resultText(res))
	}
}
