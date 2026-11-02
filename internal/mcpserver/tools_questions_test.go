package mcpserver

import (
	"context"
	"reflect"
	"testing"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/app/apptest"
)

func TestQuestionModeFields(t *testing.T) {
	cases := []struct {
		name  string
		m     questionModeFields
		mode  string
		field string // ValidationError field when mode is ""
	}{
		{"resolve", questionModeFields{Resolution: "Yes."}, "resolve", ""},
		{"defer", questionModeFields{Defer: true}, "defer", ""},
		{"defer with note", questionModeFields{Defer: true, Note: "Later."}, "defer", ""},
		{"reopen", questionModeFields{Reopen: true}, "reopen", ""},
		{"none", questionModeFields{}, "", "resolution"},
		{"blank resolution", questionModeFields{Resolution: "  "}, "", "resolution"},
		{"resolution and reopen", questionModeFields{Resolution: "Yes.", Reopen: true}, "", "resolution"},
		{"defer and reopen", questionModeFields{Defer: true, Reopen: true}, "", "resolution"},
		{"note without defer", questionModeFields{Resolution: "Yes.", Note: "x"}, "", "note"},
		{"note alone", questionModeFields{Note: "x"}, "", "resolution"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			mode, err := c.m.mode()
			if mode != c.mode {
				t.Errorf("mode = %q, want %q", mode, c.mode)
			}
			if c.field == "" {
				if err != nil {
					t.Errorf("err = %v", err)
				}
				return
			}
			ve, ok := app.IsValidation(err)
			if !ok || ve.Field != c.field {
				t.Errorf("err = %v, want a ValidationError on %s", err, c.field)
			}
		})
	}
}

func TestAddQuestion(t *testing.T) {
	a, _ := seeded(t)
	cs := newTestSession(t, a, sessionOpts{})
	assertToolMeta(t, cs, "casebook_add_question")

	res := callTool(t, cs, "casebook_add_question", map[string]any{"question": "Does the billing export refresh nightly?"})
	out := decode[mutationResult[questionRecord]](t, res)
	if out.Ref != "Q-002" || out.InvestigationRef != "INV-0001" || out.Action != "created" ||
		out.Record.Status != app.QuestionStatusOpen || len(out.Changed) != 0 {
		t.Fatalf("out = %+v", out)
	}
	if _, ok := structuredKeys(t, structuredKeys(t, res.StructuredContent)["record"])["text"]; ok {
		t.Error("record echoes the question text")
	}
	if txt := resultText(res); txt != "Added question Q-002 to INV-0001." {
		t.Errorf("text = %q", txt)
	}
	q, err := a.GetQuestion(context.Background(), "", "Q-002")
	if err != nil {
		t.Fatal(err)
	}
	if q.Question != "Does the billing export refresh nightly?" || q.CreatedBy != apptest.AI {
		t.Errorf("question = %+v", q)
	}

	t.Run("missing question", func(t *testing.T) {
		wantToolError(t, callTool(t, cs, "casebook_add_question", map[string]any{}), "question")
	})
	t.Run("blank question", func(t *testing.T) {
		wantErrorPrefix(t, callTool(t, cs, "casebook_add_question", map[string]any{"question": "  "}), "invalid question")
	})
	t.Run("unknown investigation", func(t *testing.T) {
		wantErrorPrefix(t, callTool(t, cs, "casebook_add_question", map[string]any{
			"question": "x", "investigation_ref": "INV-0099",
		}), "not found:")
	})
}

func TestResolveQuestion(t *testing.T) {
	a, _ := seeded(t)
	cs := newTestSession(t, a, sessionOpts{})
	assertToolMeta(t, cs, "casebook_resolve_question")

	steps := []struct {
		args    map[string]any
		action  string
		changed []string
		status  app.QuestionStatus
		text    string
	}{
		{map[string]any{"question_ref": "q-1", "resolution": "Intentional."}, "resolved",
			[]string{"status", "resolution"}, app.QuestionStatusResolved, "Resolved Q-001."},
		{map[string]any{"question_ref": "Q-001", "resolution": "Intentional."}, "unchanged",
			[]string{}, app.QuestionStatusResolved, "No change to Q-001."},
		{map[string]any{"question_ref": "Q-001", "defer": true, "note": "Ask finance."}, "deferred",
			[]string{"status", "resolution", "defer_note"}, app.QuestionStatusDeferred, "Deferred Q-001."},
		{map[string]any{"question_ref": "INV-0001/Q-001", "reopen": true}, "reopened",
			[]string{"status", "defer_note"}, app.QuestionStatusOpen, "Reopened Q-001."},
		{map[string]any{"question_ref": "Q-001", "reopen": true}, "unchanged",
			[]string{}, app.QuestionStatusOpen, "No change to Q-001."},
	}
	for _, s := range steps {
		res := callTool(t, cs, "casebook_resolve_question", s.args)
		out := decode[mutationResult[questionRecord]](t, res)
		if out.Ref != "Q-001" || out.InvestigationRef != "INV-0001" || out.Action != s.action ||
			!reflect.DeepEqual(out.Changed, s.changed) || out.Record.Status != s.status {
			t.Fatalf("%v: out = %+v", s.args, out)
		}
		if out.Record.Resolution != "" || out.Record.DeferNote != "" || out.Record.Text != "" {
			t.Errorf("%v: record echoes text: %+v", s.args, out.Record)
		}
		if txt := resultText(res); txt != s.text {
			t.Errorf("%v: text = %q, want %q", s.args, txt, s.text)
		}
	}
	q, err := a.GetQuestion(context.Background(), "", "Q-001")
	if err != nil {
		t.Fatal(err)
	}
	if q.Status != app.QuestionStatusOpen || q.Resolution != "" || q.DeferNote != "" {
		t.Errorf("question = %+v", q)
	}

	errorCases := []struct {
		name   string
		args   map[string]any
		prefix string
	}{
		{"no mode", map[string]any{"question_ref": "Q-001"}, "invalid resolution"},
		{"two modes", map[string]any{"question_ref": "Q-001", "resolution": "Yes.", "reopen": true}, "invalid resolution"},
		{"note without defer", map[string]any{"question_ref": "Q-001", "note": "x"}, "invalid"},
		{"resolution with note", map[string]any{"question_ref": "Q-001", "resolution": "Yes.", "note": "x"}, "invalid note"},
		{"wrong ref kind", map[string]any{"question_ref": "C-001", "reopen": true}, "invalid ref"},
		{"unknown question", map[string]any{"question_ref": "Q-099", "reopen": true}, "not found:"},
	}
	for _, c := range errorCases {
		t.Run(c.name, func(t *testing.T) {
			wantErrorPrefix(t, callTool(t, cs, "casebook_resolve_question", c.args), c.prefix)
		})
	}
	t.Run("missing question_ref", func(t *testing.T) {
		wantToolError(t, callTool(t, cs, "casebook_resolve_question", map[string]any{"reopen": true}), "question_ref")
	})
}

func TestQuestionToolsNoActive(t *testing.T) {
	a := apptest.Open(t)
	cs := newTestSession(t, a, sessionOpts{})
	wantToolError(t, callTool(t, cs, "casebook_add_question", map[string]any{"question": "x"}), "casebook_set_active_investigation")
	wantToolError(t, callTool(t, cs, "casebook_resolve_question", map[string]any{"question_ref": "Q-001", "reopen": true}),
		"casebook_set_active_investigation")
}
