package app_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/app/apptest"
)

func TestActiveInvestigationNoneSet(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t)
	newInv(t, a, "T")
	got, err := a.GetActiveInvestigation(ctx)
	if err != nil || got != nil {
		t.Fatalf("GetActiveInvestigation = %v, %v; want nil, nil", got, err)
	}
	if _, err := a.GetInvestigation(ctx, ""); !errors.Is(err, app.ErrNoActiveInvestigation) {
		t.Fatalf("GetInvestigation(\"\") err = %v, want ErrNoActiveInvestigation", err)
	}
	if errors.Is(app.ErrNoActiveInvestigation, app.ErrNotFound) {
		t.Fatal("ErrNoActiveInvestigation must be distinct from ErrNotFound")
	}
}

func TestActiveInvestigationSetAndDefault(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t)
	newInv(t, a, "First")
	second := newInv(t, a, "Second")

	set, err := a.SetActiveInvestigation(ctx, apptest.Human, "inv-2")
	if err != nil {
		t.Fatal(err)
	}
	if set.Ref != second.Ref || !set.IsActive {
		t.Fatalf("SetActiveInvestigation = %+v", set)
	}
	got, err := a.GetInvestigation(ctx, "")
	if err != nil || got.Ref != "INV-0002" || !got.IsActive {
		t.Fatalf("GetInvestigation(\"\") = %+v, %v", got, err)
	}
	act, err := a.GetActiveInvestigation(ctx)
	if err != nil || act == nil || act.Ref != "INV-0002" {
		t.Fatalf("GetActiveInvestigation = %+v, %v", act, err)
	}
	if first, _ := a.GetInvestigation(ctx, "INV-0001"); first.IsActive {
		t.Fatal("INV-0001 reports IsActive")
	}
	// Setting it again is a no-op.
	if _, err := a.SetActiveInvestigation(ctx, apptest.Human, "INV-0002"); err != nil {
		t.Fatal(err)
	}
	wantActions(t, a, "INV-0002", "investigation.create", "active.set")

	_, err = a.SetActiveInvestigation(ctx, apptest.Human, "")
	wantValidation(t, err, "investigation_ref")
	if _, err := a.SetActiveInvestigation(ctx, apptest.Human, "INV-0042"); !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("missing ref: err = %v, want ErrNotFound", err)
	}
}

func TestActiveInvestigationSharedAcrossApps(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t)
	b := apptest.Reopen(t, a.DBPath())
	newInv(t, a, "T")

	if _, err := a.SetActiveInvestigation(ctx, apptest.Human, "INV-0001"); err != nil {
		t.Fatal(err)
	}
	got, err := b.GetActiveInvestigation(ctx)
	if err != nil || got == nil || got.Ref != "INV-0001" {
		t.Fatalf("second App GetActiveInvestigation = %+v, %v", got, err)
	}
	if inv, err := b.GetInvestigation(ctx, ""); err != nil || inv.Ref != "INV-0001" {
		t.Fatalf("second App GetInvestigation(\"\") = %+v, %v", inv, err)
	}
	if err := b.ClearActiveInvestigation(ctx, apptest.AI); err != nil {
		t.Fatal(err)
	}
	if got, err := a.GetActiveInvestigation(ctx); err != nil || got != nil {
		t.Fatalf("first App after clear by second = %+v, %v", got, err)
	}
}

func TestActiveInvestigationArchiveKeepsActive(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t)
	inv, err := a.CreateInvestigation(ctx, apptest.Human, app.CreateInvestigationInput{Title: "T", MakeActive: true})
	if err != nil {
		t.Fatal(err)
	}
	arch, err := a.ArchiveInvestigation(ctx, apptest.Human, inv.Ref)
	if err != nil {
		t.Fatal(err)
	}
	if !arch.IsActive {
		t.Fatal("archiving the active investigation cleared IsActive")
	}
	got, err := a.GetActiveInvestigation(ctx)
	if err != nil || got == nil || got.Ref != inv.Ref || !got.Archived {
		t.Fatalf("GetActiveInvestigation after archive = %+v, %v", got, err)
	}
}

func TestActiveInvestigationClear(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t)
	if err := a.ClearActiveInvestigation(ctx, apptest.Human); err != nil {
		t.Fatalf("clear with none set: %v", err)
	}
	wantActions(t, a, "")

	inv, err := a.CreateInvestigation(ctx, apptest.Human, app.CreateInvestigationInput{Title: "T", MakeActive: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.ClearActiveInvestigation(ctx, apptest.AI); err != nil {
		t.Fatal(err)
	}
	if err := a.ClearActiveInvestigation(ctx, apptest.AI); err != nil {
		t.Fatal(err)
	}
	wantActions(t, a, "", "investigation.create", "active.set", "active.clear")
	wantActions(t, a, inv.Ref, "investigation.create", "active.set")

	rows, err := a.ListActivity(ctx, app.ActivityFilter{Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	clr := rows[0]
	if clr.Action != "active.clear" || clr.InvestigationRef != "" || clr.Actor != apptest.AI || clr.TargetRef != "INV-0001" {
		t.Fatalf("active.clear row = %+v", clr)
	}
	if got, _ := a.GetInvestigation(ctx, inv.Ref); got.IsActive {
		t.Fatal("IsActive still true after clear")
	}
}
