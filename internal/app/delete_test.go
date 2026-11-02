package app_test

import (
	"context"
	"strings"
	"testing"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/app/apptest"
)

// TestDeleteRecords covers AR-10 for records: guards against dependents,
// cascades of a record's own links, one activity row per delete, search
// cleanup and refs that are never reused.
func TestDeleteRecords(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t)
	f := apptest.SeedCloudSpend(t, a)
	inv := f.InvestigationRef

	// E-001 is linked by C-001: refused until unlinked.
	err := a.DeleteEvidence(ctx, apptest.Human, inv, "E-001")
	wantErrIs(t, err, app.ErrConflict)
	if !strings.Contains(err.Error(), "unlink it first") {
		t.Fatalf("err = %v", err)
	}
	// C-002 replaces the superseded C-003: refused.
	err = a.DeleteClaim(ctx, apptest.Human, inv, "C-002")
	wantErrIs(t, err, app.ErrConflict)
	if !strings.Contains(err.Error(), "C-002 replaces C-003") {
		t.Fatalf("err = %v", err)
	}
	wantErrIs(t, a.DeleteQuestion(ctx, apptest.Human, inv, "Q-099"), app.ErrNotFound)
	wantValidation(t, a.DeleteClaim(ctx, apptest.Human, inv, "E-001"), "ref")

	// Deleting the superseded C-003 takes its own links with it.
	if err := a.DeleteClaim(ctx, apptest.Human, inv, "C-003"); err != nil {
		t.Fatal(err)
	}
	wantLast(t, a, inv, "claim.delete", "C-003", "Deleted claim C-003")
	if _, err := a.GetClaim(ctx, inv, "C-003"); err == nil {
		t.Fatal("C-003 still readable")
	}
	// Now C-002 can go too, then E-002 and E-003 once nothing links them.
	if err := a.DeleteClaim(ctx, apptest.Human, inv, "C-002"); err != nil {
		t.Fatal(err)
	}
	if err := a.DeleteHypothesis(ctx, apptest.Human, inv, "H-002"); err != nil {
		t.Fatal(err)
	}
	wantLast(t, a, inv, "hypothesis.delete", "H-002", "Deleted hypothesis H-002")
	if err := a.DeleteEvidence(ctx, apptest.Human, inv, "E-003"); err != nil {
		t.Fatal(err)
	}
	last := lastActivity(t, a, inv)
	if last.Action != "evidence.delete" || !strings.HasPrefix(last.Summary, "Deleted evidence E-003 ") {
		t.Fatalf("last activity = %+v", last)
	}
	if err := a.DeleteQuestion(ctx, apptest.AI, inv, "Q-001"); err != nil {
		t.Fatal(err)
	}
	wantLast(t, a, inv, "question.delete", "Q-001", "Deleted question Q-001")

	hits, err := a.Search(ctx, "comparison", app.SearchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range hits {
		if h.RecordRef == "E-003" {
			t.Fatal("search still finds deleted E-003")
		}
	}
	// Refs are never reused.
	ev, err := a.AddEvidence(ctx, apptest.Human, inv, app.EvidenceInput{Title: "t", Summary: "s", SourceType: app.SourceTypeFile})
	if err != nil || ev.Ref != "E-005" {
		t.Fatalf("next evidence = %q, %v", ev.Ref, err)
	}
	if err := a.CheckIntegrity(ctx); err != nil {
		t.Fatal(err)
	}
}

// TestDeleteInvestigation: only archived investigations, everything inside
// goes, the active setting is cleared and one global row remains.
func TestDeleteInvestigation(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t)
	f := apptest.SeedCloudSpend(t, a)
	other, err := a.CreateInvestigation(ctx, apptest.Human, app.CreateInvestigationInput{Title: "Keep me"})
	if err != nil {
		t.Fatal(err)
	}

	wantValidation(t, a.DeleteInvestigation(ctx, apptest.Human, " "), "investigation_ref")
	err = a.DeleteInvestigation(ctx, apptest.Human, f.InvestigationRef)
	wantErrIs(t, err, app.ErrConflict)
	if _, err := a.ArchiveInvestigation(ctx, apptest.Human, f.InvestigationRef); err != nil {
		t.Fatal(err)
	}
	if err := a.DeleteInvestigation(ctx, apptest.Human, f.InvestigationRef); err != nil {
		t.Fatal(err)
	}
	wantErrIs(t, func() error { _, err := a.GetInvestigation(ctx, f.InvestigationRef); return err }(), app.ErrNotFound)
	if active, err := a.GetActiveInvestigation(ctx); err != nil || active != nil {
		t.Fatalf("active after delete = %v, %v", active, err)
	}
	st, err := a.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if st.Counts.Investigations != 1 || st.Counts.Evidence != 0 || st.Counts.Claims != 0 || st.Counts.ClaimLinks != 0 {
		t.Fatalf("counts after delete = %+v", st.Counts)
	}
	rows, err := a.ListActivity(ctx, app.ActivityFilter{Limit: 1})
	if err != nil || len(rows) != 1 {
		t.Fatal(err)
	}
	if r := rows[0]; r.Action != "investigation.delete" || r.InvestigationRef != "" || r.TargetRef != f.InvestigationRef ||
		r.Summary != `Deleted INV-0001 "September cloud bill up 38%" (4 evidence, 3 claims, 2 hypotheses, 1 questions)` {
		t.Fatalf("delete row = %+v", r)
	}
	if hits, err := a.Search(ctx, "debug", app.SearchOptions{IncludeArchived: true}); err != nil || len(hits) != 0 {
		t.Fatalf("search after delete = %v, %v", hits, err)
	}
	if _, err := a.GetInvestigation(ctx, other.Ref); err != nil {
		t.Fatal(err)
	}
	// The INV ref is not reused.
	next, err := a.CreateInvestigation(ctx, apptest.Human, app.CreateInvestigationInput{Title: "Next"})
	if err != nil || next.Ref != "INV-0003" {
		t.Fatalf("next = %q, %v", next.Ref, err)
	}
	if err := a.CheckIntegrity(ctx); err != nil {
		t.Fatal(err)
	}
}

// TestCheckIntegrityAfterOtherWrites: integrity_check on a pooled reader
// that already ran it must not report a stale FTS5 checksum mismatch after
// later writes (found while testing deletes).
func TestCheckIntegrityAfterOtherWrites(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t)
	apptest.SeedCloudSpend(t, a)
	if err := a.CheckIntegrity(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := a.UpdateQuestion(ctx, apptest.Human, "", "Q-001", "Rewritten question?"); err != nil {
		t.Fatal(err)
	}
	if err := a.DeleteQuestion(ctx, apptest.Human, "", "Q-001"); err != nil {
		t.Fatal(err)
	}
	if err := a.CheckIntegrity(ctx); err != nil {
		t.Fatal(err)
	}
}
