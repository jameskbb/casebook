package app_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/app/apptest"
)

func ptr[T any](v T) *T { return &v }

// actions returns the activity actions for invRef ("" = all), oldest first.
func actions(t *testing.T, a *app.App, invRef string) []string {
	t.Helper()
	rows, err := a.ListActivity(context.Background(), app.ActivityFilter{InvestigationRef: invRef, Limit: 500})
	if err != nil {
		t.Fatalf("ListActivity: %v", err)
	}
	out := make([]string, 0, len(rows))
	for i := len(rows) - 1; i >= 0; i-- {
		out = append(out, rows[i].Action)
	}
	return out
}

func wantActions(t *testing.T, a *app.App, invRef string, want ...string) {
	t.Helper()
	got := actions(t, a, invRef)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("activity actions = %v, want %v", got, want)
	}
}

func newInv(t *testing.T, a *app.App, title string) app.Investigation {
	t.Helper()
	inv, err := a.CreateInvestigation(context.Background(), apptest.Human, app.CreateInvestigationInput{Title: title})
	if err != nil {
		t.Fatalf("CreateInvestigation: %v", err)
	}
	return inv
}

func wantValidation(t *testing.T, err error, field string) {
	t.Helper()
	ve, ok := app.IsValidation(err)
	if !ok {
		t.Fatalf("err = %v, want ValidationError on %s", err, field)
	}
	if ve.Field != field {
		t.Fatalf("ValidationError field = %q (%v), want %q", ve.Field, ve, field)
	}
}

func TestUpdateInvestigationDefaultsAndCreatedBy(t *testing.T) {
	a := apptest.Open(t)
	inv, err := a.CreateInvestigation(context.Background(), apptest.AI, app.CreateInvestigationInput{Title: "T"})
	if err != nil {
		t.Fatal(err)
	}
	if inv.Status != app.InvestigationStatusNew {
		t.Fatalf("Status = %q, want new", inv.Status)
	}
	if inv.CreatedBy != apptest.AI {
		t.Fatalf("CreatedBy = %+v, want %+v", inv.CreatedBy, apptest.AI)
	}
	if inv.CreatedAt != app.FormatTime(apptest.FixedTime) {
		t.Fatalf("CreatedAt = %q, want fixed clock", inv.CreatedAt)
	}
	apptest.AssertNoJSONNulls(t, inv)
}

func TestUpdateInvestigationFields(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t)
	inv := newInv(t, a, "Original title")

	got, err := a.UpdateInvestigation(ctx, apptest.Human, inv.Ref, app.InvestigationPatch{
		ProblemStatement: ptr("  Why do the numbers differ?\r\n"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Original title" {
		t.Fatalf("Title changed to %q by a nil patch field", got.Title)
	}
	if got.ProblemStatement != "Why do the numbers differ?" {
		t.Fatalf("ProblemStatement = %q", got.ProblemStatement)
	}
	got, err = a.UpdateInvestigation(ctx, apptest.Human, "inv-1", app.InvestigationPatch{
		Title:  ptr("New title"),
		Status: ptr(app.InvestigationStatusInvestigating),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "New title" || got.Status != app.InvestigationStatusInvestigating ||
		got.ProblemStatement != "Why do the numbers differ?" {
		t.Fatalf("after second patch: %+v", got)
	}
	wantActions(t, a, inv.Ref, "investigation.create", "investigation.update", "investigation.update")

	rows, err := a.ListActivity(ctx, app.ActivityFilter{InvestigationRef: inv.Ref, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if s := rows[0].Summary; !strings.Contains(s, "title") || !strings.Contains(s, "status") || strings.Contains(s, "New title") {
		t.Fatalf("update summary = %q, want changed field names only", s)
	}
}

func TestUpdateInvestigationNoOpWritesNoActivity(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t)
	inv := newInv(t, a, "Same")
	for name, p := range map[string]app.InvestigationPatch{
		"empty":       {},
		"same title":  {Title: ptr(" Same ")},
		"same status": {Status: ptr(app.InvestigationStatusNew)},
		"same text":   {ProblemStatement: ptr("")},
	} {
		got, err := a.UpdateInvestigation(ctx, apptest.Human, inv.Ref, p)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got.UpdatedAt != inv.UpdatedAt || got.Title != "Same" {
			t.Fatalf("%s: investigation changed: %+v", name, got)
		}
	}
	wantActions(t, a, inv.Ref, "investigation.create")
}

func TestUpdateInvestigationValidation(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t)
	inv := newInv(t, a, "T")
	_, err := a.UpdateInvestigation(ctx, apptest.Human, inv.Ref, app.InvestigationPatch{Title: ptr(strings.Repeat("x", 201))})
	wantValidation(t, err, "title")
	_, err = a.UpdateInvestigation(ctx, apptest.Human, inv.Ref, app.InvestigationPatch{Title: ptr("   ")})
	wantValidation(t, err, "title")
	_, err = a.UpdateInvestigation(ctx, apptest.Human, inv.Ref, app.InvestigationPatch{Status: ptr(app.InvestigationStatus("done"))})
	wantValidation(t, err, "status")
	_, err = a.UpdateInvestigation(ctx, app.Actor{Kind: "robot", Client: "x"}, inv.Ref, app.InvestigationPatch{Title: ptr("ok")})
	wantValidation(t, err, "actor.kind")
	_, err = a.UpdateInvestigation(ctx, apptest.Human, "INV-0099", app.InvestigationPatch{Title: ptr("ok")})
	if !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("missing investigation: err = %v, want ErrNotFound", err)
	}
	_, err = a.UpdateInvestigation(ctx, apptest.Human, "", app.InvestigationPatch{Title: ptr("ok")})
	if !errors.Is(err, app.ErrNoActiveInvestigation) {
		t.Fatalf("empty ref with none active: err = %v, want ErrNoActiveInvestigation", err)
	}
	wantActions(t, a, inv.Ref, "investigation.create")
}

func TestUpdateInvestigationUpdatedAtAdvances(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t, app.WithClock(apptest.StepClock(apptest.FixedTime, time.Minute)))
	inv := newInv(t, a, "T")
	got, err := a.UpdateInvestigation(ctx, apptest.Human, inv.Ref, app.InvestigationPatch{Title: ptr("T2")})
	if err != nil {
		t.Fatal(err)
	}
	if got.UpdatedAt <= inv.UpdatedAt || got.CreatedAt != inv.CreatedAt {
		t.Fatalf("updated_at %q not after %q (created %q -> %q)", got.UpdatedAt, inv.UpdatedAt, inv.CreatedAt, got.CreatedAt)
	}
	got2, err := a.ArchiveInvestigation(ctx, apptest.Human, inv.Ref)
	if err != nil {
		t.Fatal(err)
	}
	if got2.UpdatedAt <= got.UpdatedAt || got2.ArchivedAt != got2.UpdatedAt {
		t.Fatalf("archive: updated_at %q archived_at %q, previous %q", got2.UpdatedAt, got2.ArchivedAt, got.UpdatedAt)
	}
	got3, err := a.UpdateConclusion(ctx, apptest.Human, inv.Ref, app.ConclusionInput{Conclusion: "C"})
	if err != nil {
		t.Fatal(err)
	}
	if got3.UpdatedAt <= got2.UpdatedAt {
		t.Fatalf("conclusion: updated_at %q not after %q", got3.UpdatedAt, got2.UpdatedAt)
	}
}

func TestArchiveUnarchiveRoundTrip(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t)
	inv := newInv(t, a, "T")
	if _, err := a.UpdateInvestigation(ctx, apptest.Human, inv.Ref, app.InvestigationPatch{Status: ptr(app.InvestigationStatusBlocked)}); err != nil {
		t.Fatal(err)
	}
	arch, err := a.ArchiveInvestigation(ctx, apptest.AI, inv.Ref)
	if err != nil {
		t.Fatal(err)
	}
	if arch.Status != app.InvestigationStatusArchived || !arch.Archived || arch.ArchivedAt == "" {
		t.Fatalf("after archive: %+v", arch)
	}
	again, err := a.ArchiveInvestigation(ctx, apptest.AI, inv.Ref)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(again, arch) {
		t.Fatalf("second archive changed the investigation: %+v", again)
	}
	un, err := a.UnarchiveInvestigation(ctx, apptest.Human, inv.Ref)
	if err != nil {
		t.Fatal(err)
	}
	if un.Status != app.InvestigationStatusBlocked || un.Archived || un.ArchivedAt != "" {
		t.Fatalf("after unarchive: %+v, want status blocked and not archived", un)
	}
	if _, err := a.UnarchiveInvestigation(ctx, apptest.Human, inv.Ref); err != nil {
		t.Fatal(err)
	}
	wantActions(t, a, inv.Ref, "investigation.create", "investigation.update", "investigation.archive", "investigation.unarchive")

	rows, err := a.ListActivity(ctx, app.ActivityFilter{InvestigationRef: inv.Ref, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if rows[1].Actor != apptest.AI || rows[0].Actor != apptest.Human {
		t.Fatalf("activity actors = %+v, %+v", rows[1].Actor, rows[0].Actor)
	}
}

func TestArchiveViaUpdateStatus(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t)
	inv := newInv(t, a, "T")
	if _, err := a.UpdateInvestigation(ctx, apptest.Human, inv.Ref, app.InvestigationPatch{Status: ptr(app.InvestigationStatusResolved)}); err != nil {
		t.Fatal(err)
	}
	arch, err := a.UpdateInvestigation(ctx, apptest.Human, inv.Ref, app.InvestigationPatch{Status: ptr(app.InvestigationStatusArchived)})
	if err != nil {
		t.Fatal(err)
	}
	if arch.Status != app.InvestigationStatusArchived || !arch.Archived {
		t.Fatalf("status archived via update: %+v", arch)
	}
	// Unarchive restores the status remembered by the update path.
	un, err := a.UnarchiveInvestigation(ctx, apptest.Human, inv.Ref)
	if err != nil {
		t.Fatal(err)
	}
	if un.Status != app.InvestigationStatusResolved {
		t.Fatalf("unarchive after update-archive: status %q, want resolved", un.Status)
	}
	wantActions(t, a, inv.Ref, "investigation.create", "investigation.update", "investigation.archive", "investigation.unarchive")
}

func TestUnarchiveViaUpdateNonArchivedStatus(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t)
	inv := newInv(t, a, "T")
	if _, err := a.ArchiveInvestigation(ctx, apptest.Human, inv.Ref); err != nil {
		t.Fatal(err)
	}
	got, err := a.UpdateInvestigation(ctx, apptest.Human, inv.Ref, app.InvestigationPatch{Status: ptr(app.InvestigationStatusInvestigating)})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != app.InvestigationStatusInvestigating || got.Archived || got.ArchivedAt != "" {
		t.Fatalf("non-archived status on archived investigation: %+v", got)
	}
	// The pre-archive status was cleared: archiving and unarchiving again
	// restores investigating, not the original new.
	if _, err := a.ArchiveInvestigation(ctx, apptest.Human, inv.Ref); err != nil {
		t.Fatal(err)
	}
	un, err := a.UnarchiveInvestigation(ctx, apptest.Human, inv.Ref)
	if err != nil {
		t.Fatal(err)
	}
	if un.Status != app.InvestigationStatusInvestigating {
		t.Fatalf("status = %q, want investigating", un.Status)
	}
	wantActions(t, a, inv.Ref, "investigation.create", "investigation.archive", "investigation.update",
		"investigation.archive", "investigation.unarchive")
}

func TestUpdateConclusionSetAndClear(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t)
	inv := newInv(t, a, "T")
	got, err := a.UpdateConclusion(ctx, apptest.AI, inv.Ref, app.ConclusionInput{
		Conclusion: "Debug logging requested for payments only reached every service and drove most of the increase.",
		Confidence: app.ConfidenceHigh,
		Rationale:  "Both definitions were read in code.",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Conclusion != "Debug logging requested for payments only reached every service and drove most of the increase." ||
		got.ConclusionConfidence != app.ConfidenceHigh || got.ConclusionRationale != "Both definitions were read in code." {
		t.Fatalf("after set: %+v", got)
	}
	rows, err := a.ListActivity(ctx, app.ActivityFilter{InvestigationRef: inv.Ref, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if rows[0].Summary != "Updated conclusion of INV-0001 (confidence high)" || rows[0].TargetRef != "INV-0001" {
		t.Fatalf("conclusion activity = %+v", rows[0])
	}
	// Identical replacement is a no-op.
	if _, err := a.UpdateConclusion(ctx, apptest.AI, inv.Ref, app.ConclusionInput{
		Conclusion: "Debug logging requested for payments only reached every service and drove most of the increase.",
		Confidence: app.ConfidenceHigh,
		Rationale:  "Both definitions were read in code.",
	}); err != nil {
		t.Fatal(err)
	}
	got, err = a.UpdateConclusion(ctx, apptest.Human, inv.Ref, app.ConclusionInput{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Conclusion != "" || got.ConclusionConfidence != app.ConfidenceUnknown || got.ConclusionRationale != "" {
		t.Fatalf("after clear: %+v", got)
	}
	wantActions(t, a, inv.Ref, "investigation.create", "conclusion.update", "conclusion.update")
}

func TestUpdateConclusionValidation(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t)
	inv := newInv(t, a, "T")
	_, err := a.UpdateConclusion(ctx, apptest.Human, inv.Ref, app.ConclusionInput{Conclusion: "x", Confidence: "certain"})
	wantValidation(t, err, "confidence")
	_, err = a.UpdateConclusion(ctx, apptest.Human, inv.Ref, app.ConclusionInput{Conclusion: strings.Repeat("x", 4001)})
	wantValidation(t, err, "conclusion")
	_, err = a.UpdateConclusion(ctx, apptest.Human, inv.Ref, app.ConclusionInput{Conclusion: "x", Rationale: strings.Repeat("y", 2001)})
	wantValidation(t, err, "rationale")
	_, err = a.UpdateConclusion(ctx, apptest.Human, inv.Ref, app.ConclusionInput{Conclusion: " ", Confidence: app.ConfidenceHigh})
	wantValidation(t, err, "confidence")
	_, err = a.UpdateConclusion(ctx, apptest.Human, inv.Ref, app.ConclusionInput{Rationale: "because"})
	wantValidation(t, err, "rationale")
	if _, err := a.UpdateConclusion(ctx, apptest.Human, inv.Ref, app.ConclusionInput{Confidence: app.ConfidenceUnknown}); err != nil {
		t.Fatalf("clearing with confidence unknown: %v", err)
	}
	wantActions(t, a, inv.Ref, "investigation.create")
}
