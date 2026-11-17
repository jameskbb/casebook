package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/app/apptest"
	"github.com/jameskbb/casebook/internal/demo"
)

// seeded returns a data dir whose casebook.db holds the demo case
// investigation (INV-0001, active), closed so run can open it.
func seeded(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	a, err := app.Open(filepath.Join(dir, dbFileName), app.WithClock(apptest.FixedClock()))
	if err != nil {
		t.Fatal(err)
	}
	apptest.SeedCloudSpend(t, a)
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestRefusesWithoutDataDir(t *testing.T) {
	// Point every default location at an empty temp dir: nothing may appear
	// there.
	home := t.TempDir()
	t.Setenv("CASEBOOK_HOME", home)
	t.Setenv("CASEBOOK_DB", "")
	var out bytes.Buffer
	err := run([]string{"--client", "claude-code"}, &out)
	if err == nil {
		t.Fatal("run without --data-dir succeeded")
	}
	if !strings.Contains(err.Error(), "never writes to the default database") {
		t.Fatalf("error %q does not say it never writes to the default database", err)
	}
	if exitCode(err) != 2 {
		t.Fatalf("exit code %d, want 2", exitCode(err))
	}
	entries, _ := os.ReadDir(home)
	if len(entries) != 0 {
		t.Fatalf("run wrote %d entries into the default location", len(entries))
	}
	if out.Len() != 0 {
		t.Fatalf("run printed %q", out.String())
	}
}

func TestWritesAsAIClient(t *testing.T) {
	dir := seeded(t)
	var out bytes.Buffer
	if err := run([]string{"--data-dir", dir}, &out); err != nil {
		t.Fatalf("run: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "E-005") || !strings.Contains(got, "C-004") {
		t.Fatalf("output %q does not name E-005 and C-004", got)
	}

	a := apptest.Reopen(t, filepath.Join(dir, dbFileName))
	ctx := context.Background()
	det, err := a.GetInvestigationDetail(ctx, "INV-0001", app.DetailOptions{})
	if err != nil {
		t.Fatal(err)
	}
	want := app.ActorMCP("claude-code")
	var ev *app.Evidence
	for i := range det.Evidence {
		if det.Evidence[i].Ref == "E-005" {
			ev = &det.Evidence[i]
		}
	}
	if ev == nil {
		t.Fatal("E-005 missing")
	}
	if ev.Title != evidenceTitle || ev.CreatedBy != want || ev.SourceType != app.SourceTypeAIToolOutput || ev.SourceLabel != evidenceSourceLabel {
		t.Fatalf("E-005 = %+v", *ev)
	}
	var cl *app.Claim
	for i := range det.Claims {
		if det.Claims[i].Ref == "C-004" {
			cl = &det.Claims[i]
		}
	}
	if cl == nil {
		t.Fatal("C-004 missing")
	}
	if cl.CreatedBy != want || cl.Confidence != app.ConfidenceMedium || len(cl.Links) != 1 ||
		cl.Links[0].EvidenceRef != "E-005" || cl.Links[0].Relation != app.RelationSupports {
		t.Fatalf("C-004 = %+v", *cl)
	}
	if len(det.RecentActivity) == 0 || det.RecentActivity[0].Actor != want {
		t.Fatalf("newest activity is not by %v: %+v", want, det.RecentActivity)
	}
}

func TestWritesToNamedInvestigationAndClient(t *testing.T) {
	dir := seeded(t)
	var out bytes.Buffer
	if err := run([]string{"--data-dir", dir, "--client", "codex", "--inv", "INV-0001"}, &out); err != nil {
		t.Fatalf("run: %v", err)
	}
	a := apptest.Reopen(t, filepath.Join(dir, dbFileName))
	ev, err := a.GetEvidence(context.Background(), "INV-0001", "E-005", app.GetEvidenceOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if ev.CreatedBy != app.ActorMCP("codex") {
		t.Fatalf("E-005 created by %v", ev.CreatedBy)
	}
}

func TestBackupToWritesValidBackupWithoutActivity(t *testing.T) {
	dir := seeded(t)
	a := apptest.Reopen(t, filepath.Join(dir, dbFileName))
	ctx := context.Background()
	before, err := a.ListActivity(ctx, app.ActivityFilter{InvestigationRef: "INV-0001", Limit: 200})
	if err != nil {
		t.Fatal(err)
	}

	dest := filepath.Join(t.TempDir(), "demo-backup.db")
	var out bytes.Buffer
	if err := run([]string{"--data-dir", dir, "--backup-to", dest}, &out); err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(out.String(), dest) {
		t.Fatalf("output %q does not name %s", out.String(), dest)
	}
	info, err := a.ValidateBackup(ctx, dest)
	if err != nil {
		t.Fatalf("ValidateBackup: %v", err)
	}
	if info.Counts.Investigations != 1 {
		t.Fatalf("backup counts %+v", info.Counts)
	}
	after, err := a.ListActivity(ctx, app.ActivityFilter{InvestigationRef: "INV-0001", Limit: 200})
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(before) {
		t.Fatalf("backup added activity: %d rows, was %d", len(after), len(before))
	}
}

func TestRejectsMissingDatabase(t *testing.T) {
	var out bytes.Buffer
	err := run([]string{"--data-dir", filepath.Join(t.TempDir(), "nope")}, &out)
	if err == nil || !strings.Contains(err.Error(), "no Casebook database") {
		t.Fatalf("err = %v, want a missing database error", err)
	}
}

func TestRawDetailBytes(t *testing.T) {
	dir := seeded(t)
	var out bytes.Buffer
	if err := run([]string{"--data-dir", dir, "--raw-detail-bytes", "65536"}, &out); err != nil {
		t.Fatalf("run: %v", err)
	}
	a := apptest.Reopen(t, filepath.Join(dir, dbFileName))
	ev, err := a.GetEvidence(context.Background(), "INV-0001", "E-005", app.GetEvidenceOptions{IncludeRawDetail: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(ev.RawDetail) != 65536 || !ev.HasRawDetail {
		t.Fatalf("raw detail is %d bytes, want 65536", len(ev.RawDetail))
	}
	if exitCode(run([]string{"--data-dir", dir, "--raw-detail-bytes", "-1"}, &out)) != 2 {
		t.Fatal("a negative size is not a usage error")
	}
}

// seededBeforeThursday returns a data dir holding the demo story up to
// Tuesday (internal/demo, StageBeforeThursday).
func seededBeforeThursday(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	now := time.Now()
	clock := demo.NewClock(demo.Start(now))
	a, err := app.Open(filepath.Join(dir, dbFileName), app.WithClock(clock.Now))
	if err != nil {
		t.Fatal(err)
	}
	if err := demo.Seed(context.Background(), a, clock, now, demo.StageBeforeThursday); err != nil {
		t.Fatal(err)
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestCheckpointWritesThursdayToFeaturedCase(t *testing.T) {
	dir := seededBeforeThursday(t)
	var out bytes.Buffer
	if err := run([]string{"--data-dir", dir, "--checkpoint"}, &out); err != nil {
		t.Fatalf("run: %v", err)
	}
	if got := out.String(); !strings.HasPrefix(got, demo.FeaturedRef+" checkpoint created E-008,E-009,C-003") {
		t.Fatalf("output %q", got)
	}
	a := apptest.Reopen(t, filepath.Join(dir, dbFileName))
	ctx := context.Background()
	det, err := a.GetInvestigationDetail(ctx, demo.FeaturedRef, app.DetailOptions{})
	if err != nil {
		t.Fatal(err)
	}
	want := app.ActorMCP("claude-code")
	var c3 *app.Claim
	for i := range det.Claims {
		if det.Claims[i].Ref == "C-003" {
			c3 = &det.Claims[i]
		}
	}
	if len(det.Claims) != 3 || c3 == nil || c3.Verification != app.VerificationAIGenerated || c3.CreatedBy != want {
		t.Fatalf("claims = %+v", det.Claims)
	}
	if det.Investigation.Conclusion != demo.Conclusion || det.Investigation.ConclusionConfidence != app.ConfidenceMedium {
		t.Fatalf("conclusion = %+v", det.Investigation)
	}
	if len(det.RecentActivity) == 0 || det.RecentActivity[0].Action != "checkpoint" || det.RecentActivity[0].Actor != want {
		t.Fatalf("newest activity = %+v", det.RecentActivity)
	}
	// A second run would double Thursday: refused.
	if err := run([]string{"--data-dir", dir, "--checkpoint"}, &out); err == nil {
		t.Fatal("second --checkpoint accepted")
	}
}
