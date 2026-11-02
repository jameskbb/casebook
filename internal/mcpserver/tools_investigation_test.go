package mcpserver

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/app/apptest"
)

func TestGetActiveInvestigation(t *testing.T) {
	t.Run("none set", func(t *testing.T) {
		cs := newTestSession(t, apptest.Open(t), sessionOpts{})
		assertToolMeta(t, cs, "casebook_get_active_investigation")
		res := callTool(t, cs, "casebook_get_active_investigation", nil)
		out := decode[activeOutput](t, res)
		if out.Active || out.Message == "" || out.Investigation != nil {
			t.Fatalf("out = %+v", out)
		}
		if !strings.Contains(resultText(res), "casebook_set_active_investigation") {
			t.Errorf("text = %q", resultText(res))
		}
	})
	t.Run("seeded", func(t *testing.T) {
		a, _ := seeded(t)
		cs := newTestSession(t, a, sessionOpts{})
		res := callTool(t, cs, "casebook_get_active_investigation", map[string]any{})
		out := decode[activeOutput](t, res)
		if !out.Active || out.Investigation == nil || out.Investigation.Ref != "INV-0001" {
			t.Fatalf("out = %+v", out)
		}
		want := app.RecordCounts{Evidence: 4, Claims: 3, Hypotheses: 2, Questions: 1, OpenQuestions: 1}
		if out.Investigation.Counts != want {
			t.Errorf("counts = %+v, want %+v", out.Investigation.Counts, want)
		}
		if out.Investigation.ConclusionConfidence != app.ConfidenceHigh || out.Investigation.Conclusion == "" {
			t.Errorf("conclusion = %q (%s)", out.Investigation.Conclusion, out.Investigation.ConclusionConfidence)
		}
		txt := resultText(res)
		if !strings.HasPrefix(txt, `Active investigation INV-0001 "September cloud bill up 38%" (investigating): 4 evidence, 3 claims, 2 hypotheses, 1 open questions.`) {
			t.Errorf("text = %q", txt)
		}
	})
	t.Run("unknown property", func(t *testing.T) {
		cs := newTestSession(t, apptest.Open(t), sessionOpts{})
		wantToolError(t, callTool(t, cs, "casebook_get_active_investigation", map[string]any{"x": 1}), "x")
	})
}

func refsOf[T any](items []T, ref func(T) string) []string {
	out := []string{}
	for _, it := range items {
		out = append(out, ref(it))
	}
	return out
}

func TestGetInvestigation(t *testing.T) {
	a, f := seeded(t)
	cs := newTestSession(t, a, sessionOpts{})
	assertToolMeta(t, cs, "casebook_get_investigation")

	t.Run("all sections, active default", func(t *testing.T) {
		res := callTool(t, cs, "casebook_get_investigation", nil)
		v := decode[investigationView](t, res)
		if v.Ref != f.InvestigationRef || !v.IsActive || v.Truncated {
			t.Fatalf("header = %+v", v)
		}
		if v.Claims == nil || v.Hypotheses == nil || v.Questions == nil || v.Evidence == nil || v.RecentActivity == nil {
			t.Fatalf("missing sections: %+v", v)
		}
		srec := func(r statementRecord) string { return r.Ref }
		if got := refsOf(v.Claims.Items, srec); !slices.Equal(got, []string{"C-001", "C-002", "C-003"}) {
			t.Errorf("claims = %v", got)
		}
		c1 := v.Claims.Items[0]
		if !slices.Equal(c1.Supports, []string{"E-001", "E-004"}) || len(c1.Contradicts) != 0 || c1.Text == "" {
			t.Errorf("C-001 = %+v", c1)
		}
		c3 := v.Claims.Items[2]
		if c3.SupersededBy != "C-002" || !slices.Equal(c3.Contradicts, []string{"E-003"}) {
			t.Errorf("C-003 = %+v", c3)
		}
		if got := refsOf(v.Hypotheses.Items, srec); !slices.Equal(got, []string{"H-001", "H-002"}) {
			t.Errorf("hypotheses = %v", got)
		}
		if got := refsOf(v.Questions.Items, func(r questionRecord) string { return r.Ref }); !slices.Equal(got, []string{"Q-001"}) {
			t.Errorf("questions = %v", got)
		}
		ev := v.Evidence.Items
		if got := refsOf(ev, func(r evidenceRecord) string { return r.Ref }); !slices.Equal(got, []string{"E-001", "E-002", "E-003", "E-004"}) {
			t.Errorf("evidence = %v", got)
		}
		if ev[2].ClaimLinks != 2 || ev[2].HypothesisLinks != 1 {
			t.Errorf("E-003 link counts = %+v", ev[2])
		}
		if n := len(*v.RecentActivity); n == 0 || n > 10 {
			t.Errorf("recent activity rows = %d, want 1..10", n)
		}
		txt := resultText(res)
		for _, want := range []string{"INV-0001", "C-001 [active, high, human_verified]", "(supports E-001, E-004)", "Q-001", "E-003"} {
			if !strings.Contains(txt, want) {
				t.Errorf("text does not contain %q:\n%s", want, txt)
			}
		}
	})

	t.Run("sections", func(t *testing.T) {
		v := decode[investigationView](t, callTool(t, cs, "casebook_get_investigation",
			map[string]any{"investigation_ref": "INV-0001", "sections": []string{"claims"}}))
		if v.Claims == nil || v.Hypotheses != nil || v.Questions != nil || v.Evidence != nil || v.RecentActivity != nil {
			t.Errorf("sections [claims] gave %+v", v)
		}
	})

	t.Run("limit truncates", func(t *testing.T) {
		res := callTool(t, cs, "casebook_get_investigation", map[string]any{"limit": 1})
		v := decode[investigationView](t, res)
		if len(v.Claims.Items) != 1 || v.Claims.Total != 3 || !v.Claims.Truncated || !v.Truncated {
			t.Errorf("claims = %+v, truncated %v", v.Claims, v.Truncated)
		}
		if !strings.Contains(resultText(res), "(truncated: 1 of 3 claims shown)") {
			t.Errorf("text = %s", resultText(res))
		}
	})

	t.Run("errors", func(t *testing.T) {
		wantToolError(t, callTool(t, cs, "casebook_get_investigation", map[string]any{"limit": 201}), "limit")
		res := callTool(t, cs, "casebook_get_investigation", map[string]any{"investigation_ref": "INV-0099"})
		wantToolError(t, res, "not found:")
		if !strings.HasPrefix(resultText(res), "not found:") {
			t.Errorf("text = %q", resultText(res))
		}
		wantToolError(t, callTool(t, cs, "casebook_get_investigation", map[string]any{"sections": []string{"bogus"}}), "sections")
	})
}

func TestGetInvestigationNoActive(t *testing.T) {
	cs := newTestSession(t, apptest.Open(t), sessionOpts{})
	wantToolError(t, callTool(t, cs, "casebook_get_investigation", nil), "casebook_set_active_investigation")
}

// TestGetInvestigationShrinksToBudget fills an investigation with records
// whose one-line texts are long and checks the view still fits the budget by
// halving the limit.
func TestGetInvestigationShrinksToBudget(t *testing.T) {
	a := apptest.Open(t)
	ctx := context.Background()
	inv, err := a.CreateInvestigation(ctx, apptest.Human, app.CreateInvestigationInput{Title: "Big", MakeActive: true})
	if err != nil {
		t.Fatal(err)
	}
	long := strings.Repeat("word ", 400)
	for i := 0; i < 200; i++ {
		if _, err := a.AddClaim(ctx, apptest.AI, inv.Ref, app.ClaimInput{Statement: long}); err != nil {
			t.Fatal(err)
		}
		if _, err := a.AddHypothesis(ctx, apptest.AI, inv.Ref, app.HypothesisInput{Statement: long}); err != nil {
			t.Fatal(err)
		}
	}
	cs := newTestSession(t, a, sessionOpts{})
	res := callTool(t, cs, "casebook_get_investigation", map[string]any{"limit": 200})
	v := decode[investigationView](t, res)
	if !v.Truncated || len(v.Claims.Items) >= 200 || v.Claims.Total != 200 {
		t.Errorf("claims %d of %d, truncated %v", len(v.Claims.Items), v.Claims.Total, v.Truncated)
	}
	if n := resultBytes(resultText(res), v); n > MaxResponseBytes {
		t.Errorf("result is %d bytes", n)
	}
}

// Tracer: an AI client creates an investigation, makes it active and writes
// into it without passing a ref.
func TestCreateInvestigation(t *testing.T) {
	a := apptest.Open(t)
	cs := newTestSession(t, a, sessionOpts{})
	assertToolMeta(t, cs, "casebook_create_investigation")
	ctx := context.Background()

	res := callTool(t, cs, "casebook_create_investigation", map[string]any{
		"title":             "Refund spike",
		"problem_statement": "Why did refunds double?",
		"make_active":       true,
	})
	out := decode[mutationResult[investigationRecord]](t, res)
	if out.Ref != "INV-0001" || out.InvestigationRef != "INV-0001" || out.Action != "created" || len(out.Changed) != 0 {
		t.Fatalf("out = %+v", out)
	}
	if !out.Record.IsActive || out.Record.Status != app.InvestigationStatusNew || out.Record.Title != "Refund spike" {
		t.Errorf("record = %+v", out.Record)
	}
	if txt := resultText(res); txt != `Created investigation INV-0001 "Refund spike" (status new). It is now the active investigation.` {
		t.Errorf("text = %q", txt)
	}

	ev := decode[mutationResult[evidenceRecord]](t, callTool(t, cs, "casebook_add_evidence", evidenceArgs(nil)))
	if ev.InvestigationRef != "INV-0001" {
		t.Errorf("evidence landed in %q", ev.InvestigationRef)
	}

	rows, err := a.ListActivity(ctx, app.ActivityFilter{InvestigationRef: "INV-0001"})
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]app.Actor{}
	for _, r := range rows {
		seen[r.Action] = r.Actor
	}
	for _, action := range []string{"investigation.create", "active.set"} {
		actor, ok := seen[action]
		if !ok {
			t.Errorf("no %s activity row in %+v", action, rows)
			continue
		}
		if actor != apptest.AI {
			t.Errorf("%s actor = %+v, want %+v", action, actor, apptest.AI)
		}
	}

	t.Run("not active", func(t *testing.T) {
		res := callTool(t, cs, "casebook_create_investigation", map[string]any{"title": "Side quest", "status": "investigating"})
		out := decode[mutationResult[investigationRecord]](t, res)
		if out.Ref != "INV-0002" || out.Record.IsActive || out.Record.Status != app.InvestigationStatusInvestigating {
			t.Errorf("out = %+v", out)
		}
		if txt := resultText(res); txt != `Created investigation INV-0002 "Side quest" (status investigating).` {
			t.Errorf("text = %q", txt)
		}
		active, err := a.GetActiveInvestigation(ctx)
		if err != nil || active == nil || active.Ref != "INV-0001" {
			t.Errorf("active = %+v, %v", active, err)
		}
	})

	t.Run("errors", func(t *testing.T) {
		wantToolError(t, callTool(t, cs, "casebook_create_investigation", map[string]any{"problem_statement": "x"}), "title")
		res := callTool(t, cs, "casebook_create_investigation", map[string]any{"title": "Nope", "status": "archived"})
		wantToolError(t, res, "invalid status")
		if !strings.HasPrefix(resultText(res), "invalid status") {
			t.Errorf("text = %q", resultText(res))
		}
		wantToolError(t, callTool(t, cs, "casebook_create_investigation", map[string]any{"title": "Nope", "status": "bogus"}), "status")
		wantToolError(t, callTool(t, cs, "casebook_create_investigation", map[string]any{"title": "   "}), "invalid title")
	})
}

func TestSetActiveInvestigation(t *testing.T) {
	a := apptest.Open(t)
	ctx := context.Background()
	for _, title := range []string{"First", "Second"} {
		if _, err := a.CreateInvestigation(ctx, apptest.Human, app.CreateInvestigationInput{Title: title}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := a.SetActiveInvestigation(ctx, apptest.Human, "INV-0001"); err != nil {
		t.Fatal(err)
	}
	cs := newTestSession(t, a, sessionOpts{})
	assertToolMeta(t, cs, "casebook_set_active_investigation")

	res := callTool(t, cs, "casebook_set_active_investigation", map[string]any{"investigation_ref": "inv-2"})
	out := decode[mutationResult[investigationRecord]](t, res)
	if out.Ref != "INV-0002" || out.InvestigationRef != "INV-0002" || out.Action != "activated" ||
		!slices.Equal(out.Changed, []string{"is_active"}) || !out.Record.IsActive {
		t.Fatalf("out = %+v", out)
	}
	if txt := resultText(res); txt != `Active investigation is now INV-0002 "Second".` {
		t.Errorf("text = %q", txt)
	}
	active, err := a.GetActiveInvestigation(ctx)
	if err != nil || active == nil || active.Ref != "INV-0002" {
		t.Fatalf("active = %+v, %v", active, err)
	}
	rows, err := a.ListActivity(ctx, app.ActivityFilter{InvestigationRef: "INV-0002", Limit: 1})
	if err != nil || len(rows) != 1 || rows[0].Action != "active.set" || rows[0].Actor != apptest.AI {
		t.Errorf("newest activity = %+v, %v", rows, err)
	}

	again := decode[mutationResult[investigationRecord]](t,
		callTool(t, cs, "casebook_set_active_investigation", map[string]any{"investigation_ref": "INV-0002"}))
	if again.Action != "unchanged" || len(again.Changed) != 0 || !again.Record.IsActive {
		t.Errorf("repeat = %+v", again)
	}

	t.Run("errors", func(t *testing.T) {
		res := callTool(t, cs, "casebook_set_active_investigation", map[string]any{"investigation_ref": "INV-0099"})
		wantToolError(t, res, "not found:")
		if !strings.HasPrefix(resultText(res), "not found:") {
			t.Errorf("text = %q", resultText(res))
		}
		res = callTool(t, cs, "casebook_set_active_investigation", map[string]any{"investigation_ref": "XYZ"})
		wantToolError(t, res, "invalid investigation_ref")
		if !strings.HasPrefix(resultText(res), "invalid investigation_ref") {
			t.Errorf("text = %q", resultText(res))
		}
		wantToolError(t, callTool(t, cs, "casebook_set_active_investigation", map[string]any{}), "investigation_ref")
		wantToolError(t, callTool(t, cs, "casebook_set_active_investigation", map[string]any{"investigation_ref": " "}),
			"invalid investigation_ref")
	})
}

// seededWithMore returns the seeded fixture plus INV-0002 (not active) and
// INV-0003 (archived).
func seededWithMore(t *testing.T) *app.App {
	t.Helper()
	a, _ := seeded(t)
	ctx := context.Background()
	if _, err := a.CreateInvestigation(ctx, apptest.Human, app.CreateInvestigationInput{Title: "Refund spike"}); err != nil {
		t.Fatal(err)
	}
	old, err := a.CreateInvestigation(ctx, apptest.Human, app.CreateInvestigationInput{Title: "Old outage"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.ArchiveInvestigation(ctx, apptest.Human, old.Ref); err != nil {
		t.Fatal(err)
	}
	return a
}

func summaryRef(s investigationSummaryOut) string { return s.Ref }

func TestListInvestigations(t *testing.T) {
	a := seededWithMore(t)
	cs := newTestSession(t, a, sessionOpts{})
	assertToolMeta(t, cs, "casebook_list_investigations")

	t.Run("default hides archived", func(t *testing.T) {
		res := callTool(t, cs, "casebook_list_investigations", nil)
		out := decode[listInvestigationsOut](t, res)
		if got := refsOf(out.Investigations, summaryRef); !slices.Equal(got, []string{"INV-0002", "INV-0001"}) {
			t.Fatalf("refs = %v", got)
		}
		if out.Total != 2 || out.Truncated || out.ActiveRef != "INV-0001" {
			t.Errorf("out = %+v", out)
		}
		inv1 := out.Investigations[1]
		if !inv1.IsActive || inv1.Archived || inv1.Counts.Evidence != 4 || inv1.Status != app.InvestigationStatusInvestigating ||
			inv1.ConclusionConfidence != app.ConfidenceHigh || inv1.UpdatedAt == "" {
			t.Errorf("INV-0001 = %+v", inv1)
		}
		if out.Investigations[0].IsActive {
			t.Errorf("INV-0002 = %+v", out.Investigations[0])
		}
		txt := resultText(res)
		want := "INV-0001 [investigating, high, active] September cloud bill up 38% (4 evidence, 3 claims, 2 hypotheses, 1 open questions; updated 2026-01-15T09:00:00.000Z)"
		if !strings.Contains(txt, want) {
			t.Errorf("text does not contain %q:\n%s", want, txt)
		}
		if strings.Contains(txt, "Showing") {
			t.Errorf("untruncated text mentions truncation:\n%s", txt)
		}
	})

	t.Run("include archived", func(t *testing.T) {
		out := decode[listInvestigationsOut](t, callTool(t, cs, "casebook_list_investigations", map[string]any{"include_archived": true}))
		if got := refsOf(out.Investigations, summaryRef); len(got) != 3 || out.Total != 3 {
			t.Errorf("refs = %v, total %d", got, out.Total)
		}
	})

	t.Run("status filter", func(t *testing.T) {
		out := decode[listInvestigationsOut](t, callTool(t, cs, "casebook_list_investigations", map[string]any{"status": "archived"}))
		if got := refsOf(out.Investigations, summaryRef); !slices.Equal(got, []string{"INV-0003"}) || !out.Investigations[0].Archived {
			t.Errorf("archived = %+v", out)
		}
		out = decode[listInvestigationsOut](t, callTool(t, cs, "casebook_list_investigations", map[string]any{"status": "new"}))
		if got := refsOf(out.Investigations, summaryRef); !slices.Equal(got, []string{"INV-0002"}) {
			t.Errorf("new = %v", got)
		}
	})

	t.Run("limit truncates", func(t *testing.T) {
		res := callTool(t, cs, "casebook_list_investigations", map[string]any{"limit": 1})
		out := decode[listInvestigationsOut](t, res)
		if len(out.Investigations) != 1 || out.Total != 2 || !out.Truncated {
			t.Errorf("out = %+v", out)
		}
		if !strings.Contains(resultText(res), "Showing 1 of 2.") {
			t.Errorf("text = %s", resultText(res))
		}
	})

	t.Run("errors", func(t *testing.T) {
		res := callTool(t, cs, "casebook_list_investigations", map[string]any{"limit": 101})
		wantToolError(t, res, "invalid limit")
		if !strings.HasPrefix(resultText(res), "invalid limit") {
			t.Errorf("text = %q", resultText(res))
		}
		wantToolError(t, callTool(t, cs, "casebook_list_investigations", map[string]any{"limit": -1}), "invalid limit")
		wantToolError(t, callTool(t, cs, "casebook_list_investigations", map[string]any{"status": "bogus"}), "status")
	})
}

func TestListInvestigationsEmpty(t *testing.T) {
	cs := newTestSession(t, apptest.Open(t), sessionOpts{})
	res := callTool(t, cs, "casebook_list_investigations", nil)
	out := decode[listInvestigationsOut](t, res)
	if len(out.Investigations) != 0 || out.Total != 0 || out.Truncated || out.ActiveRef != "" {
		t.Errorf("out = %+v", out)
	}
	if txt := resultText(res); txt != "No investigations yet. Call casebook_create_investigation." {
		t.Errorf("text = %q", txt)
	}
	res = callTool(t, cs, "casebook_list_investigations", map[string]any{"status": "resolved"})
	if txt := resultText(res); txt != "No investigations match." {
		t.Errorf("filtered text = %q", txt)
	}
}

// TestListInvestigationsShrinksToBudget lists 100 investigations with
// 200-byte titles: the full page does not fit the response target, so the
// tool halves the page and reports truncation.
func TestListInvestigationsShrinksToBudget(t *testing.T) {
	a := apptest.Open(t)
	ctx := context.Background()
	for i := range 100 {
		title := fmt.Sprintf("%03d ", i) + strings.Repeat("t", 196)
		if _, err := a.CreateInvestigation(ctx, apptest.Human, app.CreateInvestigationInput{Title: title}); err != nil {
			t.Fatal(err)
		}
	}
	cs := newTestSession(t, a, sessionOpts{})
	res := callTool(t, cs, "casebook_list_investigations", map[string]any{"limit": 100})
	out := decode[listInvestigationsOut](t, res)
	if len(out.Investigations) == 0 || len(out.Investigations) >= 100 || out.Total != 100 || !out.Truncated {
		t.Errorf("items %d of %d, truncated %v", len(out.Investigations), out.Total, out.Truncated)
	}
	if n := resultBytes(resultText(res), out); n > MaxResponseBytes {
		t.Errorf("result is %d bytes", n)
	}
	if !strings.Contains(resultText(res), fmt.Sprintf("Showing %d of 100.", len(out.Investigations))) {
		t.Errorf("text does not report truncation")
	}
}

func TestSearchInvestigations(t *testing.T) {
	a := seededWithMore(t)
	cs := newTestSession(t, a, sessionOpts{})
	assertToolMeta(t, cs, "casebook_search_investigations")
	ctx := context.Background()

	hasClaimC001 := func(hits []searchHitOut) bool {
		for _, h := range hits {
			if h.InvestigationRef == "INV-0001" && h.RecordType == app.RecordClaim && h.RecordRef == "C-001" {
				return true
			}
		}
		return false
	}

	t.Run("hits", func(t *testing.T) {
		res := callTool(t, cs, "casebook_search_investigations", map[string]any{"query": "debug"})
		out := decode[searchOut](t, res)
		if out.Query != "debug" || len(out.Hits) < 2 || out.Total != len(out.Hits) || out.Truncated {
			t.Fatalf("out = %+v", out)
		}
		if !hasClaimC001(out.Hits) {
			t.Errorf("no claim C-001 hit in %+v", out.Hits)
		}
		for _, h := range out.Hits {
			if !strings.Contains(h.Snippet, "**") || h.InvestigationTitle != "September cloud bill up 38%" {
				t.Errorf("hit = %+v", h)
			}
		}
		if !strings.Contains(resultText(res), "INV-0001 claim C-001: ") {
			t.Errorf("text = %s", resultText(res))
		}
	})

	t.Run("limit truncates", func(t *testing.T) {
		res := callTool(t, cs, "casebook_search_investigations", map[string]any{"query": "debug", "limit": 1})
		out := decode[searchOut](t, res)
		if len(out.Hits) != 1 || !out.Truncated || out.Total < 2 {
			t.Errorf("out = %+v", out)
		}
		if !strings.Contains(resultText(res), fmt.Sprintf("Showing 1 of %d.", out.Total)) {
			t.Errorf("text = %s", resultText(res))
		}
	})

	t.Run("archived skipped unless asked", func(t *testing.T) {
		out := decode[searchOut](t, callTool(t, cs, "casebook_search_investigations", map[string]any{"query": "outage"}))
		if len(out.Hits) != 0 || out.Total != 0 {
			t.Errorf("hits = %+v", out.Hits)
		}
		out = decode[searchOut](t, callTool(t, cs, "casebook_search_investigations",
			map[string]any{"query": "outage", "include_archived": true}))
		if len(out.Hits) != 1 || out.Hits[0].RecordType != app.RecordInvestigation || out.Hits[0].RecordRef != "INV-0003" {
			t.Errorf("hits = %+v", out.Hits)
		}
	})

	t.Run("scoped", func(t *testing.T) {
		out := decode[searchOut](t, callTool(t, cs, "casebook_search_investigations",
			map[string]any{"query": "debug", "investigation_ref": "inv-2"}))
		if len(out.Hits) != 0 {
			t.Errorf("INV-0002 hits = %+v", out.Hits)
		}
	})

	t.Run("errors", func(t *testing.T) {
		res := callTool(t, cs, "casebook_search_investigations", map[string]any{"query": ""})
		wantToolError(t, res, "invalid query")
		if !strings.HasPrefix(resultText(res), "invalid query") {
			t.Errorf("text = %q", resultText(res))
		}
		wantToolError(t, callTool(t, cs, "casebook_search_investigations", map[string]any{}), "query")
		wantToolError(t, callTool(t, cs, "casebook_search_investigations", map[string]any{"query": "x", "limit": 101}), "invalid limit")
		wantToolError(t, callTool(t, cs, "casebook_search_investigations",
			map[string]any{"query": "x", "investigation_ref": "INV-0099"}), "not found:")
	})

	t.Run("empty ref searches all, not the active one", func(t *testing.T) {
		if _, err := a.SetActiveInvestigation(ctx, apptest.Human, "INV-0002"); err != nil {
			t.Fatal(err)
		}
		out := decode[searchOut](t, callTool(t, cs, "casebook_search_investigations", map[string]any{"query": "debug"}))
		if !hasClaimC001(out.Hits) {
			t.Errorf("no INV-0001 claim hit with INV-0002 active: %+v", out.Hits)
		}
	})
}

// activityCount counts the activity rows of one investigation.
func activityCount(t *testing.T, a *app.App, invRef string) int {
	t.Helper()
	rows, err := a.ListActivity(context.Background(), app.ActivityFilter{InvestigationRef: invRef, Limit: 500})
	if err != nil {
		t.Fatal(err)
	}
	return len(rows)
}

func TestUpdateInvestigation(t *testing.T) {
	a, f := seeded(t)
	cs := newTestSession(t, a, sessionOpts{})
	assertToolMeta(t, cs, "casebook_update_investigation")
	ctx := context.Background()
	update := func(args map[string]any) mutationResult[investigationRecord] {
		t.Helper()
		return decode[mutationResult[investigationRecord]](t, callTool(t, cs, "casebook_update_investigation", args))
	}

	t.Run("title on the active investigation", func(t *testing.T) {
		res := callTool(t, cs, "casebook_update_investigation", map[string]any{"title": "Cloud bill up 38% in September"})
		out := decode[mutationResult[investigationRecord]](t, res)
		if out.Ref != f.InvestigationRef || out.InvestigationRef != f.InvestigationRef || out.Action != "updated" ||
			!slices.Equal(out.Changed, []string{"title"}) || out.Record.Title != "Cloud bill up 38% in September" {
			t.Fatalf("out = %+v", out)
		}
		if txt := resultText(res); txt != "Updated INV-0001: title." {
			t.Errorf("text = %q", txt)
		}
		before := activityCount(t, a, f.InvestigationRef)
		res = callTool(t, cs, "casebook_update_investigation", map[string]any{"title": "Cloud bill up 38% in September"})
		again := decode[mutationResult[investigationRecord]](t, res)
		if again.Action != "unchanged" || len(again.Changed) != 0 {
			t.Errorf("repeat = %+v", again)
		}
		if txt := resultText(res); txt != "No change to INV-0001." {
			t.Errorf("text = %q", txt)
		}
		if after := activityCount(t, a, f.InvestigationRef); after != before {
			t.Errorf("no-op wrote activity: %d rows, was %d", after, before)
		}
	})

	t.Run("several fields", func(t *testing.T) {
		res := callTool(t, cs, "casebook_update_investigation", map[string]any{
			"investigation_ref": "inv-1", "problem_statement": "Why do the systems disagree?", "status": "blocked",
		})
		out := decode[mutationResult[investigationRecord]](t, res)
		if !slices.Equal(out.Changed, []string{"problem_statement", "status"}) || out.Record.Status != app.InvestigationStatusBlocked {
			t.Errorf("out = %+v", out)
		}
		if txt := resultText(res); txt != "Updated INV-0001: problem_statement, status." {
			t.Errorf("text = %q", txt)
		}
	})

	t.Run("status resolved, archived, unarchived", func(t *testing.T) {
		out := update(map[string]any{"status": "resolved"})
		if !slices.Equal(out.Changed, []string{"status"}) || out.Record.Status != app.InvestigationStatusResolved || out.Record.Archived {
			t.Errorf("resolved = %+v", out)
		}
		out = update(map[string]any{"status": "archived"})
		if out.Action != "updated" || !out.Record.Archived || out.Record.Status != app.InvestigationStatusArchived {
			t.Errorf("archived = %+v", out)
		}
		out = update(map[string]any{"status": "investigating"})
		if out.Record.Archived || out.Record.Status != app.InvestigationStatusInvestigating {
			t.Errorf("unarchived = %+v", out)
		}
	})

	t.Run("null means unchanged", func(t *testing.T) {
		out := update(map[string]any{"title": "X", "status": nil, "problem_statement": nil})
		if !slices.Equal(out.Changed, []string{"title"}) || out.Record.Status != app.InvestigationStatusInvestigating {
			t.Errorf("out = %+v", out)
		}
		inv, err := a.GetInvestigation(ctx, f.InvestigationRef)
		if err != nil || inv.ProblemStatement != "Why do the systems disagree?" {
			t.Errorf("problem statement = %q, %v", inv.ProblemStatement, err)
		}
	})

	t.Run("errors", func(t *testing.T) {
		wantToolError(t, callTool(t, cs, "casebook_update_investigation", map[string]any{"status": "bogus"}), "status")
		for _, args := range []map[string]any{{}, {"investigation_ref": "INV-0001"}, {"title": nil}} {
			res := callTool(t, cs, "casebook_update_investigation", args)
			wantToolError(t, res, "invalid patch")
			if !strings.HasPrefix(resultText(res), "invalid patch") {
				t.Errorf("%v: text = %q", args, resultText(res))
			}
		}
		wantToolError(t, callTool(t, cs, "casebook_update_investigation", map[string]any{"title": "   "}), "invalid title")
		wantToolError(t, callTool(t, cs, "casebook_update_investigation",
			map[string]any{"investigation_ref": "INV-0099", "title": "X"}), "not found:")
		wantToolError(t, callTool(t, cs, "casebook_update_investigation",
			map[string]any{"investigation_ref": "XYZ", "title": "X"}), "invalid investigation_ref")
	})
}

func TestUpdateInvestigationNoActive(t *testing.T) {
	a := apptest.Open(t)
	if _, err := a.CreateInvestigation(context.Background(), apptest.Human, app.CreateInvestigationInput{Title: "Idle"}); err != nil {
		t.Fatal(err)
	}
	cs := newTestSession(t, a, sessionOpts{})
	wantToolError(t, callTool(t, cs, "casebook_update_investigation", map[string]any{"title": "X"}), "casebook_set_active_investigation")
	wantToolError(t, callTool(t, cs, "casebook_update_conclusion", map[string]any{"conclusion": "X"}), "casebook_set_active_investigation")
}

func TestUpdateConclusion(t *testing.T) {
	a, f := seeded(t)
	cs := newTestSession(t, a, sessionOpts{})
	assertToolMeta(t, cs, "casebook_update_conclusion")
	ctx := context.Background()
	args := map[string]any{
		"conclusion": "Debug logging drove the increase.",
		"confidence": "medium",
		"rationale":  "Two sources agree.",
	}

	t.Run("set", func(t *testing.T) {
		res := callTool(t, cs, "casebook_update_conclusion", args)
		out := decode[mutationResult[investigationRecord]](t, res)
		if out.Ref != f.InvestigationRef || out.InvestigationRef != f.InvestigationRef || out.Action != "updated" ||
			!slices.Equal(out.Changed, []string{"conclusion", "conclusion_confidence", "conclusion_rationale", "conclusion_verification"}) ||
			out.Record.ConclusionConfidence != app.ConfidenceMedium {
			t.Fatalf("out = %+v", out)
		}
		if txt := resultText(res); txt != "Updated the conclusion of INV-0001 (confidence medium)." {
			t.Errorf("text = %q", txt)
		}
		inv, err := a.GetInvestigation(ctx, f.InvestigationRef)
		if err != nil || inv.Conclusion != "Debug logging drove the increase." ||
			inv.ConclusionConfidence != app.ConfidenceMedium || inv.ConclusionRationale != "Two sources agree." {
			t.Errorf("investigation = %+v, %v", inv, err)
		}
		rows, err := a.ListActivity(ctx, app.ActivityFilter{InvestigationRef: f.InvestigationRef, Limit: 1})
		if err != nil || len(rows) != 1 || rows[0].Action != "conclusion.update" || rows[0].Actor != apptest.AI {
			t.Errorf("newest activity = %+v, %v", rows, err)
		}

		again := decode[mutationResult[investigationRecord]](t, callTool(t, cs, "casebook_update_conclusion", args))
		if again.Action != "unchanged" || len(again.Changed) != 0 {
			t.Errorf("repeat = %+v", again)
		}
	})

	t.Run("only the confidence changes", func(t *testing.T) {
		out := decode[mutationResult[investigationRecord]](t, callTool(t, cs, "casebook_update_conclusion", map[string]any{
			"investigation_ref": "inv-1", "conclusion": "Debug logging drove the increase.",
			"confidence": "high", "rationale": "Two sources agree.",
		}))
		if !slices.Equal(out.Changed, []string{"conclusion_confidence"}) {
			t.Errorf("changed = %v", out.Changed)
		}
	})

	t.Run("errors", func(t *testing.T) {
		res := callTool(t, cs, "casebook_update_conclusion", map[string]any{"conclusion": "", "confidence": "high"})
		wantToolError(t, res, "invalid confidence")
		if !strings.HasPrefix(resultText(res), "invalid confidence") {
			t.Errorf("text = %q", resultText(res))
		}
		wantToolError(t, callTool(t, cs, "casebook_update_conclusion", map[string]any{"conclusion": "", "rationale": "why"}),
			"invalid rationale")
		wantToolError(t, callTool(t, cs, "casebook_update_conclusion", map[string]any{"confidence": "high"}), "conclusion")
		wantToolError(t, callTool(t, cs, "casebook_update_conclusion", map[string]any{"conclusion": "X", "confidence": "sure"}),
			"confidence")
		wantToolError(t, callTool(t, cs, "casebook_update_conclusion",
			map[string]any{"investigation_ref": "INV-0099", "conclusion": "X"}), "not found:")
	})

	t.Run("clear", func(t *testing.T) {
		res := callTool(t, cs, "casebook_update_conclusion", map[string]any{"conclusion": ""})
		out := decode[mutationResult[investigationRecord]](t, res)
		if out.Action != "updated" || !slices.Contains(out.Changed, "conclusion") ||
			out.Record.ConclusionConfidence != app.ConfidenceUnknown {
			t.Errorf("out = %+v", out)
		}
		if txt := resultText(res); txt != "Cleared the conclusion of INV-0001." {
			t.Errorf("text = %q", txt)
		}
		inv, err := a.GetInvestigation(ctx, f.InvestigationRef)
		if err != nil || inv.Conclusion != "" || inv.ConclusionRationale != "" {
			t.Errorf("investigation = %+v, %v", inv, err)
		}
	})
}

// TestSearchTotalLowerBound is the WR-03 regression: the app counts at most
// 100 hits, so with more matches the search tool must not claim it returned
// every hit.
func TestSearchTotalLowerBound(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t)
	inv, err := a.CreateInvestigation(ctx, apptest.AI, app.CreateInvestigationInput{Title: "Many questions", MakeActive: true})
	if err != nil {
		t.Fatal(err)
	}
	for i := range 150 {
		if _, err := a.AddQuestion(ctx, apptest.AI, inv.Ref, app.QuestionInput{Question: fmt.Sprintf("Zebra question %d?", i)}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := a.AddQuestion(ctx, apptest.AI, inv.Ref, app.QuestionInput{Question: "Okapi question?"}); err != nil {
		t.Fatal(err)
	}
	cs := newTestSession(t, a, sessionOpts{})

	for _, c := range []struct {
		limit int
		text  string
	}{{100, "Showing 100 of at least 100."}, {0, "Showing 20 of at least 100."}} {
		t.Run(fmt.Sprintf("limit %d", c.limit), func(t *testing.T) {
			args := map[string]any{"query": "zebra"}
			if c.limit > 0 {
				args["limit"] = c.limit
			}
			res := callTool(t, cs, "casebook_search_investigations", args)
			out := decode[searchOut](t, res)
			if out.Total != searchScanLimit || !out.TotalIsLowerBound || !out.Truncated {
				t.Fatalf("total %d lower bound %v truncated %v", out.Total, out.TotalIsLowerBound, out.Truncated)
			}
			if !strings.Contains(resultText(res), c.text) {
				t.Errorf("text lacks %q", c.text)
			}
		})
	}
	t.Run("exact total below the cap", func(t *testing.T) {
		out := decode[searchOut](t, callTool(t, cs, "casebook_search_investigations", map[string]any{"query": "okapi"}))
		if out.Total != 1 || out.TotalIsLowerBound || out.Truncated {
			t.Fatalf("total %d lower bound %v truncated %v", out.Total, out.TotalIsLowerBound, out.Truncated)
		}
	})
}

// TestListTotalLowerBound covers the list tool at its scan cap (WR-03).
func TestListTotalLowerBound(t *testing.T) {
	sums := make([]app.InvestigationSummary, listScanLimit)
	for i := range sums {
		sums[i] = app.InvestigationSummary{Ref: fmt.Sprintf("INV-%04d", i+1), Title: "T"}
	}
	out := buildInvestigationList(sums, maxListLimit, "")
	if out.Total != listScanLimit || !out.TotalIsLowerBound || !out.Truncated {
		t.Fatalf("total %d lower bound %v truncated %v", out.Total, out.TotalIsLowerBound, out.Truncated)
	}
	if txt := renderInvestigationList(out, false); !strings.Contains(txt, "Showing 100 of at least 1000.") {
		t.Errorf("text lacks the lower bound: %.200s", txt[len(txt)-60:])
	}
	out = buildInvestigationList(sums[:listScanLimit-1], maxListLimit, "")
	if out.TotalIsLowerBound || out.Total != listScanLimit-1 || !out.Truncated {
		t.Fatalf("below the cap: total %d lower bound %v truncated %v", out.Total, out.TotalIsLowerBound, out.Truncated)
	}
}
