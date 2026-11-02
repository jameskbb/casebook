package mcpserver

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/app/apptest"
)

// activityRows lists every activity row of INV-0001, newest first.
func activityRows(t *testing.T, a *app.App) []app.Activity {
	t.Helper()
	rows, err := a.ListActivity(context.Background(), app.ActivityFilter{InvestigationRef: "INV-0001", Limit: 500})
	if err != nil {
		t.Fatalf("list activity: %v", err)
	}
	return rows
}

// checkpointRows returns the checkpoint rows of INV-0001.
func checkpointRows(t *testing.T, a *app.App) []app.Activity {
	t.Helper()
	var out []app.Activity
	for _, r := range activityRows(t, a) {
		if r.Action == "checkpoint" {
			out = append(out, r)
		}
	}
	return out
}

func TestCheckpointTracer(t *testing.T) {
	ctx := context.Background()
	a, _ := seeded(t)
	cs := newTestSession(t, a, sessionOpts{})
	assertToolMeta(t, cs, "casebook_checkpoint")

	note := "Confirmed the chart default change."
	res := callTool(t, cs, "casebook_checkpoint", map[string]any{
		"note": note,
		"evidence": []any{map[string]any{
			"key": "new1", "title": "Services at debug level", "summary": "214 services log at debug level after the chart change.", "source_type": "sql_query",
		}},
		"claims": []any{map[string]any{
			"statement": "The chart default explains all 214 services.", "confidence": "high",
			"links": []any{map[string]any{"evidence_ref": "#new1", "relation": "supports"}},
		}},
		"question_updates": []any{map[string]any{"ref": "Q-001", "resolution": "Yes; only payments needs debug."}},
	})
	out := decode[checkpointOut](t, res)
	if out.InvestigationRef != "INV-0001" || strings.Join(out.Created, ",") != "E-005,C-004" ||
		strings.Join(out.Updated, ",") != "Q-001" || out.Checkpoint.ActivityID <= 0 {
		t.Fatalf("out = %+v", out)
	}
	txt := resultText(res)
	for _, want := range []string{"Checkpoint recorded for INV-0001", "Created: E-005, C-004.", "Updated: Q-001."} {
		if !strings.Contains(txt, want) {
			t.Errorf("text %q lacks %q", txt, want)
		}
	}

	c, err := a.GetClaim(ctx, "INV-0001", "C-004")
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Links) != 1 || c.Links[0].EvidenceRef != "E-005" || c.Links[0].Relation != app.RelationSupports {
		t.Fatalf("C-004 links = %+v", c.Links)
	}
	q, err := a.GetQuestion(ctx, "INV-0001", "Q-001")
	if err != nil {
		t.Fatal(err)
	}
	if q.Status != app.QuestionStatusResolved || q.Resolution != "Yes; only payments needs debug." {
		t.Fatalf("Q-001 = %+v", q)
	}
	rows := checkpointRows(t, a)
	if len(rows) != 1 || rows[0].Note != note || rows[0].Actor != apptest.AI || rows[0].ID != out.Checkpoint.ActivityID {
		t.Fatalf("checkpoint rows = %+v", rows)
	}
	// The batch's own writes are changes since the (absent) previous checkpoint.
	if out.Checkpoint.PreviousCheckpointAt != "" || out.Checkpoint.ChangesTotal == 0 {
		t.Fatalf("first checkpoint summary = %+v", out.Checkpoint)
	}
	found := false
	for _, ch := range out.Checkpoint.Changes {
		if ch.Action == "evidence.add" && ch.TargetRef == "E-005" {
			found = true
		}
	}
	if !found {
		t.Errorf("changes lack evidence.add E-005: %+v", out.Checkpoint.Changes)
	}

	// A second checkpoint with nothing new.
	res = callTool(t, cs, "casebook_checkpoint", map[string]any{"note": "Nothing new."})
	out2 := decode[checkpointOut](t, res)
	if out2.Checkpoint.PreviousCheckpointAt == "" || len(out2.Checkpoint.Changes) != 0 ||
		out2.Checkpoint.ChangesTotal != 0 || out2.Checkpoint.ChangesTruncated {
		t.Fatalf("second checkpoint = %+v", out2.Checkpoint)
	}
	if out2.Created == nil || out2.Updated == nil || len(out2.Created)+len(out2.Updated) != 0 {
		t.Fatalf("second created/updated = %v / %v", out2.Created, out2.Updated)
	}
	// decode already rejects JSON nulls; the lists are arrays.
	if out2.Checkpoint.OpenQuestions == nil || out2.Checkpoint.OpenHypotheses == nil || out2.Checkpoint.ClaimsToReview == nil {
		t.Fatalf("nil lists in %+v", out2.Checkpoint)
	}
	if !strings.Contains(resultText(res), "Since "+out2.Checkpoint.PreviousCheckpointAt+": 0 changes.") {
		t.Errorf("second text = %q", resultText(res))
	}
}

func TestCheckpointShapeErrors(t *testing.T) {
	evidence := func(key string) map[string]any {
		return map[string]any{"key": key, "title": "T", "summary": "S", "source_type": "code"}
	}
	many := make([]any, 51)
	for i := range many {
		many[i] = fmt.Sprintf("Question %d?", i)
	}
	cases := []struct {
		name string
		args map[string]any
		want string
	}{
		{"unknown key", map[string]any{"note": "n", "claims": []any{map[string]any{
			"statement": "s", "links": []any{map[string]any{"evidence_ref": "#missing", "relation": "supports"}}}}},
			`invalid claims[0].links[0].evidence_ref: unknown key "#missing"`},
		{"unknown key in hypothesis", map[string]any{"note": "n", "evidence": []any{evidence("a")}, "hypotheses": []any{map[string]any{
			"statement": "s", "links": []any{map[string]any{"evidence_ref": "#b", "relation": "contradicts"}}}}},
			`invalid hypotheses[0].links[0].evidence_ref: unknown key "#b"`},
		{"duplicate key", map[string]any{"note": "n", "evidence": []any{evidence("a"), evidence("a")}},
			"invalid evidence[1].key: duplicate key"},
		{"bad key", map[string]any{"note": "n", "evidence": []any{evidence("bad key!")}},
			"invalid evidence[0].key:"},
		{"two question modes", map[string]any{"note": "n", "question_updates": []any{map[string]any{
			"ref": "Q-001", "resolution": "Yes.", "reopen": true}}},
			"invalid question_updates[0].resolution:"},
		{"no question mode", map[string]any{"note": "n", "question_updates": []any{map[string]any{"ref": "Q-001"}}},
			"invalid question_updates[0].resolution:"},
		{"empty claim update", map[string]any{"note": "n", "claim_updates": []any{map[string]any{"ref": "C-001"}}},
			"invalid claim_updates[0]:"},
		{"empty hypothesis update", map[string]any{"note": "n", "hypothesis_updates": []any{map[string]any{"ref": "H-001"}}},
			"invalid hypothesis_updates[0]:"},
		{"update without ref", map[string]any{"note": "n", "claim_updates": []any{map[string]any{"ref": "", "status": "disputed"}}},
			"invalid claim_updates[0].ref:"},
		{"blank question", map[string]any{"note": "n", "questions": []any{"Q?", "  "}},
			"invalid questions[1]:"},
		{"too many items", map[string]any{"note": "n", "questions": many},
			"invalid items: at most 50"},
		{"conclusion counts as an item", map[string]any{"note": "n", "questions": many[:50],
			"conclusion": map[string]any{"conclusion": "c"}},
			"invalid items: at most 50"},
		{"empty note", map[string]any{"note": ""}, "invalid note:"},
		{"blank note", map[string]any{"note": "   "}, "invalid note:"},
		{"long note", map[string]any{"note": strings.Repeat("x", 1001)}, "invalid note:"},
		{"long note with a batch", map[string]any{"note": strings.Repeat("y", 1001), "evidence": []any{evidence("a")}},
			"invalid note:"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a, _ := seeded(t)
			cs := newTestSession(t, a, sessionOpts{})
			before := len(activityRows(t, a))
			res := callTool(t, cs, "casebook_checkpoint", c.args)
			wantToolError(t, res, c.want)
			if after := len(activityRows(t, a)); after != before {
				t.Fatalf("activity rows %d -> %d: a shape error wrote", before, after)
			}
		})
	}
}

func TestCheckpointMidBatchFailure(t *testing.T) {
	ctx := context.Background()
	a, _ := seeded(t)
	cs := newTestSession(t, a, sessionOpts{})
	before := len(activityRows(t, a))
	res := callTool(t, cs, "casebook_checkpoint", map[string]any{
		"note": "Partial work.",
		"evidence": []any{map[string]any{
			"key": "k", "title": "Audit", "summary": "Rows checked.", "source_type": "sql_query",
		}},
		"claims": []any{
			map[string]any{"statement": "ok", "links": []any{map[string]any{"evidence_ref": "#k", "relation": "supports"}}},
			map[string]any{"statement": "bad", "links": []any{map[string]any{"evidence_ref": "E-099", "relation": "supports"}}},
		},
		"questions": []any{"Q?"},
	})
	// All or nothing (AR-06): the error names the failing item and nothing
	// before or after it was written.
	wantToolError(t, res, "claims[1]: not found: evidence E-099 in INV-0001")
	if rows := checkpointRows(t, a); len(rows) != 0 {
		t.Fatalf("checkpoint rows written: %+v", rows)
	}
	if after := len(activityRows(t, a)); after != before {
		t.Fatalf("activity rows %d -> %d: a failed batch wrote", before, after)
	}
	if _, err := a.GetEvidence(ctx, "INV-0001", "E-005", app.GetEvidenceOptions{}); err == nil {
		t.Fatal("E-005 exists: an item before the failure was kept")
	}
	if _, err := a.GetClaim(ctx, "INV-0001", "C-004"); err == nil {
		t.Fatal("C-004 exists: an item before the failure was kept")
	}

	// Resending the corrected batch allocates the refs the failed one did not
	// consume, so a retry never duplicates.
	res = callTool(t, cs, "casebook_checkpoint", map[string]any{
		"note": "Partial work, fixed.",
		"evidence": []any{map[string]any{
			"key": "k", "title": "Audit", "summary": "Rows checked.", "source_type": "sql_query",
		}},
		"claims": []any{
			map[string]any{"statement": "ok", "links": []any{map[string]any{"evidence_ref": "#k", "relation": "supports"}}},
		},
		"questions": []any{"Q?"},
	})
	out := decode[checkpointOut](t, res)
	if strings.Join(out.Created, ",") != "E-005,C-004,Q-002" {
		t.Fatalf("created = %v", out.Created)
	}

	// A patch error names the flattened tool path.
	res = callTool(t, cs, "casebook_checkpoint", map[string]any{
		"note":          "n",
		"claim_updates": []any{map[string]any{"ref": "C-001", "status": "disputed", "verification": "source_verified"}},
	})
	wantToolError(t, res, "invalid claim_updates[0].verification:")
}

func TestCheckpointFullBatch(t *testing.T) {
	ctx := context.Background()
	a, _ := seeded(t)
	cs := newTestSession(t, a, sessionOpts{})
	lastID := activityRows(t, a)[0].ID
	res := callTool(t, cs, "casebook_checkpoint", map[string]any{
		"investigation_ref": "inv-1",
		"note":              "Wrapped up.",
		"evidence": []any{
			map[string]any{"key": "a", "title": "A", "summary": "SA", "source_type": "code"},
			map[string]any{"title": "B", "summary": "SB", "source_type": "file", "metadata": map[string]any{"rows": 3}},
		},
		"claims":        []any{map[string]any{"statement": "Claim.", "links": []any{map[string]any{"evidence_ref": "#a", "relation": "contradicts"}}}},
		"claim_updates": []any{map[string]any{"ref": "C-002", "status": "disputed"}},
		"hypotheses": []any{map[string]any{"statement": "Hyp.", "links": []any{
			map[string]any{"evidence_ref": "#a", "relation": "supports"},
			map[string]any{"evidence_ref": "E-006", "relation": "contradicts"},
		}}},
		"hypothesis_updates": []any{map[string]any{"ref": "H-001", "confidence": "high"}},
		"questions":          []any{"Next?"},
		"question_updates":   []any{map[string]any{"ref": "Q-001", "defer": true, "note": "Later."}},
		"conclusion":         map[string]any{"conclusion": "Debug logging drove the increase.", "confidence": "high"},
	})
	out := decode[checkpointOut](t, res)
	if got := strings.Join(out.Created, ","); got != "E-005,E-006,C-004,H-003,Q-002" {
		t.Fatalf("created = %s", got)
	}
	if got := strings.Join(out.Updated, ","); got != "C-002,H-001,Q-001,INV-0001" {
		t.Fatalf("updated = %s", got)
	}
	inv, err := a.GetInvestigation(ctx, "INV-0001")
	if err != nil {
		t.Fatal(err)
	}
	if inv.Conclusion != "Debug logging drove the increase." || inv.ConclusionConfidence != app.ConfidenceHigh {
		t.Fatalf("conclusion = %q (%s)", inv.Conclusion, inv.ConclusionConfidence)
	}
	h, err := a.GetHypothesis(ctx, "INV-0001", "H-003")
	if err != nil {
		t.Fatal(err)
	}
	if len(h.Links) != 2 {
		t.Fatalf("H-003 links = %+v", h.Links)
	}
	q, err := a.GetQuestion(ctx, "INV-0001", "Q-001")
	if err != nil || q.Status != app.QuestionStatusDeferred || q.DeferNote != "Later." {
		t.Fatalf("Q-001 = %+v, %v", q, err)
	}
	cl, err := a.GetClaim(ctx, "INV-0001", "C-002")
	if err != nil || cl.Status != app.ClaimStatusDisputed {
		t.Fatalf("C-002 = %+v, %v", cl, err)
	}
	n := 0
	for _, r := range activityRows(t, a) {
		if r.ID <= lastID {
			continue
		}
		n++
		if r.Actor != apptest.AI {
			t.Fatalf("row %+v not by claude-code", r)
		}
	}
	if n < 10 {
		t.Fatalf("only %d new activity rows", n)
	}
}

func TestCheckpointCaps(t *testing.T) {
	ctx := context.Background()
	a, _ := seeded(t)
	for i := range 24 {
		if _, err := a.AddQuestion(ctx, apptest.AI, "", app.QuestionInput{Question: fmt.Sprintf("Extra question %d?", i)}); err != nil {
			t.Fatal(err)
		}
	}
	for i := range 21 {
		if _, err := a.AddHypothesis(ctx, apptest.AI, "", app.HypothesisInput{Statement: fmt.Sprintf("Extra hypothesis %d.", i)}); err != nil {
			t.Fatal(err)
		}
		if _, err := a.AddClaim(ctx, apptest.AI, "", app.ClaimInput{Statement: fmt.Sprintf("Extra claim %d.", i)}); err != nil {
			t.Fatal(err)
		}
	}
	cs := newTestSession(t, a, sessionOpts{})
	res := callTool(t, cs, "casebook_checkpoint", map[string]any{"note": "Caps."})
	out := decode[checkpointOut](t, res)
	s := out.Checkpoint
	if len(s.OpenQuestions) != 20 || s.OpenQuestionsTotal != 25 {
		t.Errorf("open questions %d of %d, want 20 of 25", len(s.OpenQuestions), s.OpenQuestionsTotal)
	}
	if len(s.OpenHypotheses) != 20 || s.OpenHypothesesTotal != 22 {
		t.Errorf("open hypotheses %d of %d, want 20 of 22", len(s.OpenHypotheses), s.OpenHypothesesTotal)
	}
	if len(s.ClaimsToReview) != 20 || s.ClaimsToReviewTotal < 21 {
		t.Errorf("claims to review %d of %d, want 20 of at least 21", len(s.ClaimsToReview), s.ClaimsToReviewTotal)
	}
	if len(s.Changes) != 20 || s.ChangesTotal <= 20 || !s.ChangesTruncated {
		t.Errorf("changes %d of %d (truncated %v), want 20 of more", len(s.Changes), s.ChangesTotal, s.ChangesTruncated)
	}
	txt := resultText(res)
	if !strings.Contains(txt, "Open questions (25):") || !strings.Contains(txt, "(20 more)") {
		t.Errorf("text = %q", txt)
	}
	if s.OpenQuestions[0].Ref != "Q-001" {
		t.Errorf("first open question = %+v", s.OpenQuestions[0])
	}
}

func TestCheckpointNoActive(t *testing.T) {
	a := apptest.Open(t)
	var called []string
	cs := newTestSession(t, a, sessionOpts{Options: Options{OnCheckpoint: func(ref string) { called = append(called, ref) }}})
	res := callTool(t, cs, "casebook_checkpoint", map[string]any{"note": "n"})
	wantToolError(t, res, "casebook_set_active_investigation")
	if len(called) != 0 {
		t.Errorf("OnCheckpoint called for a failed checkpoint: %v", called)
	}
}

// TestCheckpointOnCheckpoint: a successful checkpoint reports the
// investigation it checkpointed to Options.OnCheckpoint, once per call.
func TestCheckpointOnCheckpoint(t *testing.T) {
	a, _ := seeded(t)
	var called []string
	cs := newTestSession(t, a, sessionOpts{Options: Options{OnCheckpoint: func(ref string) { called = append(called, ref) }}})
	callTool(t, cs, "casebook_checkpoint", map[string]any{"note": "first"})
	callTool(t, cs, "casebook_checkpoint", map[string]any{"note": "second", "investigation_ref": "inv-0001"})
	if strings.Join(called, ",") != "INV-0001,INV-0001" {
		t.Errorf("OnCheckpoint calls = %v, want INV-0001 twice", called)
	}
}

// TestCheckpointNoOpNotUpdated proves a no-op item is not reported as
// updated (AR-17): reopening a question that is already open changes
// nothing.
func TestCheckpointNoOpNotUpdated(t *testing.T) {
	a, _ := seeded(t)
	cs := newTestSession(t, a, sessionOpts{})
	res := callTool(t, cs, "casebook_checkpoint", map[string]any{
		"note":             "Nothing changed.",
		"question_updates": []any{map[string]any{"ref": "Q-001", "reopen": true}},
	})
	out := decode[checkpointOut](t, res)
	if len(out.Updated) != 0 || len(out.Created) != 0 {
		t.Fatalf("created %v, updated %v; want both empty", out.Created, out.Updated)
	}
	if strings.Contains(resultText(res), "Updated:") {
		t.Errorf("text reports an update: %s", resultText(res))
	}
}
