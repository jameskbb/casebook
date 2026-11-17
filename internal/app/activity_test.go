package app_test

import (
	"context"
	"strings"
	"testing"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/app/apptest"
)

// TestListActivityAfterIDPagesForwardWithoutGaps proves AfterID is a
// gap-free forward cursor: with more rows after the cursor than Limit, the
// oldest come back first and paging by the largest id seen returns every
// row exactly once.
func TestListActivityAfterIDPagesForwardWithoutGaps(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t)
	inv, err := a.CreateInvestigation(ctx, apptest.Human, app.CreateInvestigationInput{Title: "Feed"})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 14; i++ {
		if _, err := a.AddQuestion(ctx, apptest.Human, inv.Ref, app.QuestionInput{Question: "Why?"}); err != nil {
			t.Fatal(err)
		}
	}
	all, err := a.ListActivity(ctx, app.ActivityFilter{Limit: 100})
	if err != nil || len(all) != 15 {
		t.Fatalf("ListActivity = %d rows, %v; want 15", len(all), err)
	}
	for i := 1; i < len(all); i++ {
		if all[i-1].ID <= all[i].ID {
			t.Fatalf("without AfterID rows are not newest first: %d then %d", all[i-1].ID, all[i].ID)
		}
	}
	first := all[len(all)-1].ID

	// The reviewer's case: AfterID 1, Limit 3 must return 2, 3, 4.
	page, err := a.ListActivity(ctx, app.ActivityFilter{AfterID: first, Limit: 3})
	if err != nil {
		t.Fatal(err)
	}
	if len(page) != 3 || page[0].ID != first+1 || page[1].ID != first+2 || page[2].ID != first+3 {
		t.Fatalf("AfterID page = %v, want ids %d, %d, %d", ids(page), first+1, first+2, first+3)
	}

	for _, scope := range []string{"", inv.Ref} {
		seen := []int64{}
		cursor := first
		for calls := 0; calls < 20; calls++ {
			page, err := a.ListActivity(ctx, app.ActivityFilter{InvestigationRef: scope, AfterID: cursor, Limit: 4})
			if err != nil {
				t.Fatal(err)
			}
			seen = append(seen, ids(page)...)
			if len(page) > 0 {
				cursor = page[len(page)-1].ID
			}
			if len(page) < 4 {
				break
			}
		}
		if len(seen) != 14 {
			t.Fatalf("scope %q: paging returned %d rows, want 14: %v", scope, len(seen), seen)
		}
		for i, id := range seen {
			if id != first+1+int64(i) {
				t.Fatalf("scope %q: paging returned %v, want consecutive ids from %d", scope, seen, first+1)
			}
		}
	}
}

func ids(acts []app.Activity) []int64 {
	out := make([]int64, len(acts))
	for i, a := range acts {
		out[i] = a.ID
	}
	return out
}

// TestActorMCPUnusualClientNamesCanWrite proves an MCP client name the
// server does not control (too long, empty, multi-line) never makes writes
// fail.
func TestActorMCPUnusualClientNamesCanWrite(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t)
	for _, name := range []string{strings.Repeat("n", 75), "", " \n\t", "line one\nline two"} {
		actor := app.ActorMCP(name)
		inv, err := a.CreateInvestigation(ctx, actor, app.CreateInvestigationInput{Title: "From MCP"})
		if err != nil {
			t.Fatalf("CreateInvestigation with ActorMCP(%q): %v", name, err)
		}
		if inv.CreatedBy != actor || len(actor.Client) > app.MaxClientBytes || actor.Client == "" {
			t.Fatalf("ActorMCP(%q) = %+v, created_by %+v", name, actor, inv.CreatedBy)
		}
	}
	if got := app.ActorMCP(""); got.Client != app.DefaultMCPClient {
		t.Fatalf("ActorMCP(\"\") = %+v, want client %q", got, app.DefaultMCPClient)
	}
}

// TestWhitespaceInvRefMeansEmpty proves a whitespace-only investigation ref
// behaves exactly like an empty one everywhere.
func TestWhitespaceInvRefMeansEmpty(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t)
	fx := apptest.SeedCloudSpend(t, a)
	got, err := a.GetInvestigation(ctx, "  ")
	if err != nil || got.Ref != fx.InvestigationRef {
		t.Fatalf("GetInvestigation(\"  \") = %q, %v; want the active %s", got.Ref, err, fx.InvestigationRef)
	}
	if _, err := a.GetEvidence(ctx, " \t", "E-001", app.GetEvidenceOptions{}); err != nil {
		t.Fatalf("GetEvidence with a blank invRef: %v", err)
	}
	if _, err := a.GetEvidence(ctx, " ", "INV-0001/E-001", app.GetEvidenceOptions{}); err != nil {
		t.Fatalf("GetEvidence with a blank invRef and a qualified ref: %v", err)
	}
	all, err := a.ListActivity(ctx, app.ActivityFilter{Limit: 500})
	if err != nil {
		t.Fatal(err)
	}
	blank, err := a.ListActivity(ctx, app.ActivityFilter{InvestigationRef: " ", Limit: 500})
	if err != nil || len(blank) != len(all) {
		t.Fatalf("ListActivity with a blank ref = %d rows, %v; want %d", len(blank), err, len(all))
	}
	if _, err := a.Search(ctx, "debug", app.SearchOptions{InvestigationRef: " "}); err != nil {
		t.Fatalf("Search with a blank ref: %v", err)
	}
	_, err = a.SetActiveInvestigation(ctx, apptest.Human, " ")
	wantValidation(t, err, "investigation_ref")
}

// TestListActivityLimitRange proves ListActivity rejects out-of-range limits
// like every other limit option.
func TestListActivityLimitRange(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t)
	for _, limit := range []int{-1, 501} {
		_, err := a.ListActivity(ctx, app.ActivityFilter{Limit: limit})
		wantValidation(t, err, "limit")
	}
	if _, err := a.ListActivity(ctx, app.ActivityFilter{Limit: 500}); err != nil {
		t.Fatalf("ListActivity limit 500: %v", err)
	}
}
