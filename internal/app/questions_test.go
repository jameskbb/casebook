package app_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/app/apptest"
)

func mustAddQuestion(t *testing.T, a *app.App, actor app.Actor, invRef, text string) app.Question {
	t.Helper()
	q, err := a.AddQuestion(context.Background(), actor, invRef, app.QuestionInput{Question: text})
	if err != nil {
		t.Fatalf("AddQuestion: %v", err)
	}
	return q
}

// questionEnv is an active investigation using a stepping clock.
func questionEnv(t *testing.T) (*app.App, app.Investigation) {
	t.Helper()
	a := apptest.Open(t, app.WithClock(apptest.StepClock(apptest.FixedTime, time.Second)))
	inv, err := a.CreateInvestigation(context.Background(), apptest.Human,
		app.CreateInvestigationInput{Title: "Questions", MakeActive: true})
	if err != nil {
		t.Fatal(err)
	}
	return a, inv
}

func TestQuestionAddGetList(t *testing.T) {
	ctx := context.Background()
	a, inv := questionEnv(t)

	q := mustAddQuestion(t, a, apptest.Human, "", "  Can the other services go back to info level?  ")
	if q.Ref != "Q-001" || q.InvestigationRef != inv.Ref || q.Question != "Can the other services go back to info level?" ||
		q.Status != app.QuestionStatusOpen || q.Resolution != "" || q.DeferNote != "" || q.ResolvedAt != "" ||
		q.CreatedBy != apptest.Human || q.CreatedAt == "" || q.UpdatedAt != q.CreatedAt {
		t.Fatalf("question = %+v", q)
	}
	wantLast(t, a, inv.Ref, "question.add", "Q-001", "Added question Q-001: Can the other services go back to info level?")
	apptest.AssertNoJSONNulls(t, q)
	b, _ := json.Marshal(q)
	for _, key := range []string{`"question":"Can the other services go back to info level?"`, `"status":"open"`, `"resolution":""`,
		`"defer_note":""`, `"resolved_at":""`, `"created_by":{"kind":"human"`} {
		if !strings.Contains(string(b), key) {
			t.Errorf("Question JSON %s lacks %s", b, key)
		}
	}

	long := strings.Repeat("why ", 400) // 1600 bytes, within the limit
	q2 := mustAddQuestion(t, a, apptest.AI, inv.Ref, long)
	if q2.Ref != "Q-002" || q2.Question != strings.TrimSpace(long) {
		t.Fatalf("long question = %s %d bytes", q2.Ref, len(q2.Question))
	}
	row := lastActivity(t, a, inv.Ref)
	if !strings.HasPrefix(row.Summary, "Added question Q-002: why why") || len(row.Summary) > 300 ||
		!strings.HasSuffix(row.Summary, "...") {
		t.Fatalf("long question summary = %q (%d bytes)", row.Summary, len(row.Summary))
	}

	for _, tc := range []struct{ name, text string }{
		{"empty", " \t "},
		{"oversized", strings.Repeat("q", app.MaxQuestionBytes+1)},
	} {
		_, err := a.AddQuestion(ctx, apptest.AI, "", app.QuestionInput{Question: tc.text})
		wantValidation(t, err, "question")
	}

	got, err := a.GetQuestion(ctx, "inv-1", "q-1")
	if err != nil || got != q {
		t.Fatalf("GetQuestion = %+v, %v", got, err)
	}
	_, err = a.GetQuestion(ctx, "", "Q-009")
	wantErrIs(t, err, app.ErrNotFound)
	_, err = a.GetQuestion(ctx, "", "H-001")
	wantValidation(t, err, "ref")
	list, err := a.ListQuestions(ctx, "")
	if err != nil || len(list) != 2 || list[0].Ref != "Q-001" || list[1].Ref != "Q-002" {
		t.Fatalf("ListQuestions = %+v, %v", list, err)
	}
	apptest.AssertNoJSONNulls(t, list)
	empty, err := a.ListQuestions(ctx, newInv(t, a, "Empty").Ref)
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("empty ListQuestions = %#v, %v", empty, err)
	}
}

func TestQuestionUpdate(t *testing.T) {
	ctx := context.Background()
	a, inv := questionEnv(t)
	q := mustAddQuestion(t, a, apptest.AI, "", "Old?")
	got, err := a.UpdateQuestion(ctx, apptest.Human, "", "Q-001", "New?")
	if err != nil || got.Question != "New?" || got.UpdatedAt <= q.UpdatedAt || got.Status != app.QuestionStatusOpen {
		t.Fatalf("UpdateQuestion = %+v, %v", got, err)
	}
	wantLast(t, a, inv.Ref, "question.update", "Q-001", "Updated question Q-001: New?")
	n := activityCount(t, a, inv.Ref)
	if _, err := a.UpdateQuestion(ctx, apptest.Human, "", "Q-001", " New? "); err != nil {
		t.Fatal(err)
	}
	if activityCount(t, a, inv.Ref) != n {
		t.Fatal("unchanged question text wrote activity")
	}
	_, err = a.UpdateQuestion(ctx, apptest.Human, "", "Q-001", "")
	wantValidation(t, err, "question")
	_, err = a.UpdateQuestion(ctx, apptest.Human, "", "Q-002", "x")
	wantErrIs(t, err, app.ErrNotFound)
}

func TestResolveQuestion(t *testing.T) {
	ctx := context.Background()
	a, inv := questionEnv(t)
	mustAddQuestion(t, a, apptest.Human, "", "Intentional?")

	_, err := a.ResolveQuestion(ctx, apptest.Human, "", "Q-001", "  ")
	wantValidation(t, err, "resolution")
	_, err = a.ResolveQuestion(ctx, apptest.Human, "", "Q-001", strings.Repeat("r", app.MaxResolutionBytes+1))
	wantValidation(t, err, "resolution")

	got, err := a.ResolveQuestion(ctx, apptest.Human, "", "Q-001", "Yes, after the chart fix.")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != app.QuestionStatusResolved || got.Resolution != "Yes, after the chart fix." || got.ResolvedAt == "" ||
		got.ResolvedAt != got.UpdatedAt || got.DeferNote != "" {
		t.Fatalf("resolved = %+v", got)
	}
	wantLast(t, a, inv.Ref, "question.resolve", "Q-001", "Resolved question Q-001: Yes, after the chart fix.")

	// Same resolution: no-op. New resolution: updates text and resolved_at.
	n := activityCount(t, a, inv.Ref)
	same, err := a.ResolveQuestion(ctx, apptest.Human, "", "Q-001", "Yes, after the chart fix.")
	if err != nil || same.ResolvedAt != got.ResolvedAt || activityCount(t, a, inv.Ref) != n {
		t.Fatalf("repeat resolve = %+v, %v", same, err)
	}
	again, err := a.ResolveQuestion(ctx, apptest.Human, "", "Q-001", "Yes, by design.")
	if err != nil || again.Resolution != "Yes, by design." || again.ResolvedAt <= got.ResolvedAt {
		t.Fatalf("re-resolve = %+v, %v", again, err)
	}
	wantLast(t, a, inv.Ref, "question.resolve", "Q-001", "Resolved question Q-001: Yes, by design.")
}

func TestDeferQuestion(t *testing.T) {
	ctx := context.Background()
	a, inv := questionEnv(t)
	mustAddQuestion(t, a, apptest.Human, "", "Intentional?")
	if _, err := a.ResolveQuestion(ctx, apptest.Human, "", "Q-001", "Yes."); err != nil {
		t.Fatal(err)
	}
	got, err := a.DeferQuestion(ctx, apptest.AI, "", "Q-001", "waiting on finance")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != app.QuestionStatusDeferred || got.DeferNote != "waiting on finance" || got.Resolution != "" || got.ResolvedAt != "" {
		t.Fatalf("deferred = %+v", got)
	}
	wantLast(t, a, inv.Ref, "question.defer", "Q-001", "Deferred question Q-001: waiting on finance")
	n := activityCount(t, a, inv.Ref)
	if _, err := a.DeferQuestion(ctx, apptest.AI, "", "Q-001", "waiting on finance"); err != nil || activityCount(t, a, inv.Ref) != n {
		t.Fatalf("repeat defer: %v", err)
	}

	mustAddQuestion(t, a, apptest.Human, "", "Second?")
	got, err = a.DeferQuestion(ctx, apptest.AI, "", "Q-002", "")
	if err != nil || got.Status != app.QuestionStatusDeferred || got.DeferNote != "" {
		t.Fatalf("defer without note = %+v, %v", got, err)
	}
	wantLast(t, a, inv.Ref, "question.defer", "Q-002", "Deferred question Q-002")
	_, err = a.DeferQuestion(ctx, apptest.AI, "", "Q-002", strings.Repeat("n", app.MaxDeferNoteBytes+1))
	wantValidation(t, err, "note")
}

func TestReopenQuestion(t *testing.T) {
	ctx := context.Background()
	a, inv := questionEnv(t)
	q := mustAddQuestion(t, a, apptest.Human, "", "Intentional?")

	n := activityCount(t, a, inv.Ref)
	got, err := a.ReopenQuestion(ctx, apptest.Human, "", "Q-001")
	if err != nil || got != q || activityCount(t, a, inv.Ref) != n {
		t.Fatalf("reopen of open question = %+v, %v", got, err)
	}

	if _, err := a.ResolveQuestion(ctx, apptest.Human, "", "Q-001", "Yes."); err != nil {
		t.Fatal(err)
	}
	got, err = a.ReopenQuestion(ctx, apptest.Human, "", "Q-001")
	if err != nil || got.Status != app.QuestionStatusOpen || got.Resolution != "" || got.ResolvedAt != "" || got.DeferNote != "" {
		t.Fatalf("reopened from resolved = %+v, %v", got, err)
	}
	wantLast(t, a, inv.Ref, "question.reopen", "Q-001", "Reopened question Q-001")

	if _, err := a.DeferQuestion(ctx, apptest.Human, "", "Q-001", "later"); err != nil {
		t.Fatal(err)
	}
	got, err = a.ReopenQuestion(ctx, apptest.Human, "", "Q-001")
	if err != nil || got.Status != app.QuestionStatusOpen || got.DeferNote != "" {
		t.Fatalf("reopened from deferred = %+v, %v", got, err)
	}
	wantActions(t, a, inv.Ref, "investigation.create", "active.set", "question.add", "question.resolve",
		"question.reopen", "question.defer", "question.reopen")
	_, err = a.ReopenQuestion(ctx, apptest.Human, "", "Q-002")
	wantErrIs(t, err, app.ErrNotFound)
}
