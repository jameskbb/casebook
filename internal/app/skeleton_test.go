package app_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/jameskbb/casebook/internal/app"
)

// TestWalkingSkeleton proves the thinnest real path end to end: app.Open on a
// fresh data dir, CreateInvestigation into a real SQLite file through embedded
// migrations, close, reopen, and read the investigation and its activity back.
func TestWalkingSkeleton(t *testing.T) {
	ctx := context.Background()
	dataDir := filepath.Join(t.TempDir(), "data") // does not exist yet
	dbPath := filepath.Join(dataDir, "casebook.db")
	actor := app.Actor{Kind: app.ActorHuman, Client: "casebook-cli"}

	a, err := app.Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if a.DBPath() != dbPath {
		t.Fatalf("DBPath() = %q, want %q", a.DBPath(), dbPath)
	}
	inv, err := a.CreateInvestigation(ctx, actor, app.CreateInvestigationInput{
		Title:            "September cloud bill up 38%",
		ProblemStatement: "Why did the cloud bill rise 38% in September when traffic was flat?",
		MakeActive:       true,
	})
	if err != nil {
		t.Fatalf("CreateInvestigation: %v", err)
	}
	if inv.Ref != "INV-0001" {
		t.Fatalf("Ref = %q, want INV-0001", inv.Ref)
	}
	if inv.Status != app.InvestigationStatusNew {
		t.Fatalf("Status = %q, want new", inv.Status)
	}
	if inv.CreatedBy != actor {
		t.Fatalf("CreatedBy = %+v, want %+v", inv.CreatedBy, actor)
	}
	if !inv.IsActive {
		t.Fatal("IsActive = false, want true after MakeActive")
	}
	if inv.ConclusionConfidence != app.ConfidenceUnknown {
		t.Fatalf("ConclusionConfidence = %q, want unknown", inv.ConclusionConfidence)
	}
	for name, ts := range map[string]string{"created_at": inv.CreatedAt, "updated_at": inv.UpdatedAt} {
		if _, err := time.Parse(app.TimeLayout, ts); err != nil {
			t.Fatalf("%s %q does not match TimeLayout: %v", name, ts, err)
		}
		if _, err := time.Parse(time.RFC3339, ts); err != nil {
			t.Fatalf("%s %q is not RFC3339: %v", name, ts, err)
		}
	}
	if err := a.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := a.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}

	b, err := app.Open(dbPath)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer b.Close()

	for _, ref := range []string{"INV-0001", "inv-1", ""} {
		got, err := b.GetInvestigation(ctx, ref)
		if err != nil {
			t.Fatalf("GetInvestigation(%q): %v", ref, err)
		}
		if got.Ref != "INV-0001" || got.Title != "September cloud bill up 38%" {
			t.Fatalf("GetInvestigation(%q) = %s %q", ref, got.Ref, got.Title)
		}
		if got.ProblemStatement != inv.ProblemStatement {
			t.Fatalf("GetInvestigation(%q) problem statement = %q", ref, got.ProblemStatement)
		}
		if !got.IsActive {
			t.Fatalf("GetInvestigation(%q).IsActive = false, want true", ref)
		}
		if got.Archived || got.ArchivedAt != "" {
			t.Fatalf("GetInvestigation(%q) archived = %v %q", ref, got.Archived, got.ArchivedAt)
		}
	}

	acts, err := b.ListActivity(ctx, app.ActivityFilter{})
	if err != nil {
		t.Fatalf("ListActivity: %v", err)
	}
	if len(acts) != 2 {
		t.Fatalf("ListActivity returned %d rows, want 2: %+v", len(acts), acts)
	}
	if acts[0].Action != "active.set" || acts[1].Action != "investigation.create" {
		t.Fatalf("activity actions = %q, %q; want active.set, investigation.create", acts[0].Action, acts[1].Action)
	}
	for _, act := range acts {
		if act.Actor != actor {
			t.Fatalf("activity actor = %+v, want %+v", act.Actor, actor)
		}
		if act.InvestigationRef != "INV-0001" || act.TargetRef != "INV-0001" {
			t.Fatalf("activity refs = %q/%q, want INV-0001", act.InvestigationRef, act.TargetRef)
		}
	}
	if want := `Created investigation INV-0001 "September cloud bill up 38%"`; acts[1].Summary != want {
		t.Fatalf("create summary = %q, want %q", acts[1].Summary, want)
	}
	scoped, err := b.ListActivity(ctx, app.ActivityFilter{InvestigationRef: "INV-1", AfterID: acts[1].ID})
	if err != nil {
		t.Fatalf("ListActivity scoped: %v", err)
	}
	if len(scoped) != 1 || scoped[0].ID != acts[0].ID {
		t.Fatalf("ListActivity AfterID = %+v, want only the newest row", scoped)
	}

	if runtime.GOOS != "windows" {
		assertMode(t, dbPath, 0o600)
		assertMode(t, dataDir, 0o700)
	}

	second, err := b.CreateInvestigation(ctx, actor, app.CreateInvestigationInput{Title: "Second"})
	if err != nil {
		t.Fatalf("second CreateInvestigation: %v", err)
	}
	if second.Ref != "INV-0002" {
		t.Fatalf("second Ref = %q, want INV-0002", second.Ref)
	}
	if second.IsActive {
		t.Fatal("second investigation is active without MakeActive")
	}
}

func assertMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("%s mode = %o, want %o", path, got, want)
	}
}
