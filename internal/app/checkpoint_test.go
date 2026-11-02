package app_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/app/apptest"
)

func itemRefs(items []app.CheckpointItem) string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, string(it.Type)+":"+it.Ref)
	}
	return strings.Join(out, ",")
}

func activityByID(t *testing.T, a *app.App, invRef string, id int64) app.Activity {
	t.Helper()
	rows, err := a.ListActivity(context.Background(), app.ActivityFilter{InvestigationRef: invRef, Limit: 500})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if r.ID == id {
			return r
		}
	}
	t.Fatalf("activity %d not found", id)
	return app.Activity{}
}

func TestCheckpoint(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t, app.WithClock(apptest.StepClock(apptest.FixedTime, time.Second)))
	f := apptest.SeedCloudSpend(t, a)
	before, err := a.GetInvestigation(ctx, f.InvestigationRef)
	if err != nil {
		t.Fatal(err)
	}
	fixtureActivity, err := a.ListActivity(ctx, app.ActivityFilter{InvestigationRef: f.InvestigationRef, Limit: 500})
	if err != nil {
		t.Fatal(err)
	}

	note := "Confirmed the chart default change.\nNext: ask finance about log ingestion."
	first, err := a.Checkpoint(ctx, apptest.AI, "", note)
	if err != nil {
		t.Fatalf("Checkpoint: %v", err)
	}
	apptest.AssertNoJSONNulls(t, first)
	if first.InvestigationRef != f.InvestigationRef || first.Note != note || first.ActivityID == 0 {
		t.Fatalf("first = %+v", first)
	}
	if first.PreviousCheckpointAt != "" {
		t.Fatalf("PreviousCheckpointAt = %q, want empty on the first checkpoint", first.PreviousCheckpointAt)
	}
	if len(first.Changes) != len(fixtureActivity) || first.ChangesTruncated {
		t.Fatalf("first changes = %d (truncated %v), want %d", len(first.Changes), first.ChangesTruncated, len(fixtureActivity))
	}
	for i := range fixtureActivity {
		if first.Changes[i] != fixtureActivity[i] {
			t.Fatalf("change[%d] = %+v, want %+v (newest first)", i, first.Changes[i], fixtureActivity[i])
		}
	}
	if got := itemRefs(first.OpenQuestions); got != "question:Q-001" {
		t.Fatalf("open questions = %s", got)
	}
	if got := itemRefs(first.OpenHypotheses); got != "hypothesis:H-001" {
		t.Fatalf("open hypotheses = %s", got)
	}
	if got := itemRefs(first.ClaimsToReview); got != "claim:C-002" {
		t.Fatalf("claims to review = %s", got)
	}
	h1 := first.OpenHypotheses[0]
	if h1.Status != "open" || h1.Confidence != app.ConfidenceMedium || h1.Verification != app.VerificationAIGenerated ||
		!strings.HasPrefix(h1.Text, "The new tracing agent") {
		t.Fatalf("H-001 item = %+v", h1)
	}
	c2 := first.ClaimsToReview[0]
	if c2.Status != "active" || c2.Verification != app.VerificationAIGenerated || c2.Confidence != app.ConfidenceHigh {
		t.Fatalf("C-002 item = %+v", c2)
	}
	if q := first.OpenQuestions[0]; q.Status != "open" ||
		q.Text != "Can every service except payments go back to info level today?" {
		t.Fatalf("Q-001 item = %+v", q)
	}

	// The activity row keeps the full note and a one-line summary.
	row := activityByID(t, a, f.InvestigationRef, first.ActivityID)
	if row.Action != "checkpoint" || row.Note != note || row.TargetRef != f.InvestigationRef || row.Actor != apptest.AI {
		t.Fatalf("checkpoint row = %+v", row)
	}
	if row.Summary != "Confirmed the chart default change. Next: ask finance about log ingestion." {
		t.Fatalf("summary = %q", row.Summary)
	}
	// A checkpoint records a note; it does not change the investigation.
	after, err := a.GetInvestigation(ctx, f.InvestigationRef)
	if err != nil {
		t.Fatal(err)
	}
	if after.UpdatedAt != before.UpdatedAt {
		t.Fatalf("updated_at moved from %s to %s", before.UpdatedAt, after.UpdatedAt)
	}

	// After one change, the next checkpoint reports exactly that change.
	ev := mustAddEvidence(t, a, apptest.Human, f.InvestigationRef, evInput("New evidence"))
	second, err := a.Checkpoint(ctx, apptest.AI, f.InvestigationRef, "Added one piece of evidence.")
	if err != nil {
		t.Fatal(err)
	}
	if second.PreviousCheckpointAt != row.CreatedAt {
		t.Fatalf("PreviousCheckpointAt = %q, want %q", second.PreviousCheckpointAt, row.CreatedAt)
	}
	if len(second.Changes) != 1 || second.Changes[0].Action != "evidence.add" || second.Changes[0].TargetRef != ev.Ref {
		t.Fatalf("second changes = %+v", second.Changes)
	}
	if second.ActivityID <= first.ActivityID {
		t.Fatal("second checkpoint id not after the first")
	}

	// Resolving the question and disproving the hypothesis clears them.
	if _, err := a.ResolveQuestion(ctx, apptest.Human, "", "Q-001", "Intentional."); err != nil {
		t.Fatal(err)
	}
	if _, err := a.SetClaimVerification(ctx, apptest.Human, "", "C-002", app.VerificationHumanVerified); err != nil {
		t.Fatal(err)
	}
	third, err := a.Checkpoint(ctx, apptest.Human, "", "Wrapped up.")
	if err != nil {
		t.Fatal(err)
	}
	if len(third.OpenQuestions) != 0 || len(third.ClaimsToReview) != 0 || len(third.Changes) != 2 {
		t.Fatalf("third = %+v", third)
	}
	apptest.AssertNoJSONNulls(t, third)
}

func TestCheckpointDisputedClaimsAndScope(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t)
	f := apptest.SeedCloudSpend(t, a)
	if _, err := a.UpdateClaim(ctx, apptest.Human, "", "C-001", app.ClaimPatch{Status: ptr(app.ClaimStatusDisputed)}); err != nil {
		t.Fatal(err)
	}
	res, err := a.Checkpoint(ctx, apptest.AI, "", "Disputed C-001.")
	if err != nil {
		t.Fatal(err)
	}
	if got := itemRefs(res.ClaimsToReview); got != "claim:C-001,claim:C-002" {
		t.Fatalf("claims to review = %s", got)
	}

	// Checkpoints are per investigation.
	other := newInv(t, a, "Other")
	o1, err := a.Checkpoint(ctx, apptest.AI, other.Ref, "Other investigation.")
	if err != nil {
		t.Fatal(err)
	}
	if o1.PreviousCheckpointAt != "" || len(o1.Changes) != 1 || o1.Changes[0].Action != "investigation.create" {
		t.Fatalf("other first checkpoint = %+v", o1)
	}
	again, err := a.Checkpoint(ctx, apptest.AI, f.InvestigationRef, "Back to the first.")
	if err != nil {
		t.Fatal(err)
	}
	if len(again.Changes) != 0 || again.PreviousCheckpointAt == "" {
		t.Fatalf("first investigation after other checkpoint = %+v", again)
	}
}

func TestCheckpointCapsChanges(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t)
	inv, err := a.CreateInvestigation(ctx, apptest.Human, app.CreateInvestigationInput{Title: "Busy", MakeActive: true})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 55; i++ {
		mustAddQuestion(t, a, apptest.AI, inv.Ref, "Question?")
	}
	all, err := a.ListActivity(ctx, app.ActivityFilter{InvestigationRef: inv.Ref, Limit: 500})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) <= 51 {
		t.Fatalf("setup produced %d rows", len(all))
	}
	res, err := a.Checkpoint(ctx, apptest.AI, "", "Lots happened.")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Changes) != 50 || !res.ChangesTruncated || res.Changes[0] != all[0] || res.Changes[49] != all[49] {
		t.Fatalf("changes = %d truncated=%v", len(res.Changes), res.ChangesTruncated)
	}
	if len(res.OpenQuestions) != 55 || res.OpenQuestions[0].Ref != "Q-001" || res.OpenQuestions[54].Ref != "Q-055" {
		t.Fatalf("open questions = %d", len(res.OpenQuestions))
	}

	// Exactly 50 changes is not truncated.
	for i := 0; i < 50; i++ {
		mustAddQuestion(t, a, apptest.AI, inv.Ref, "More?")
	}
	res, err = a.Checkpoint(ctx, apptest.AI, "", "Fifty more.")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Changes) != 50 || res.ChangesTruncated {
		t.Fatalf("exactly 50: changes = %d truncated=%v", len(res.Changes), res.ChangesTruncated)
	}
}

func TestCheckpointValidation(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t)
	_, err := a.Checkpoint(ctx, apptest.AI, "", "note")
	wantErrIs(t, err, app.ErrNoActiveInvestigation)

	apptest.SeedCloudSpend(t, a)
	for _, bad := range []string{"", "   \n\t "} {
		_, err := a.Checkpoint(ctx, apptest.AI, "", bad)
		wantValidation(t, err, "note")
	}
	_, err = a.Checkpoint(ctx, apptest.AI, "", strings.Repeat("n", app.MaxCheckpointNoteBytes+1))
	wantValidation(t, err, "note")
	long := strings.Repeat("n", app.MaxCheckpointNoteBytes)
	res, err := a.Checkpoint(ctx, apptest.AI, "", long)
	if err != nil {
		t.Fatalf("1000-byte note: %v", err)
	}
	row := activityByID(t, a, "", res.ActivityID)
	if row.Note != long || len(row.Summary) > app.MaxActivitySummaryBytes || !strings.HasSuffix(row.Summary, "...") {
		t.Fatalf("long note row: note %d bytes, summary %q", len(row.Note), row.Summary)
	}
	_, err = a.Checkpoint(ctx, app.Actor{Kind: "robot", Client: "x"}, "", "note")
	wantValidation(t, err, "actor.kind")
	_, err = a.Checkpoint(ctx, apptest.AI, "INV-0042", "note")
	wantErrIs(t, err, app.ErrNotFound)

	// Failed checkpoints write nothing.
	rows, err := a.ListActivity(ctx, app.ActivityFilter{Limit: 500})
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, r := range rows {
		if r.Action == "checkpoint" {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("checkpoint rows = %d, want 1", n)
	}
}
