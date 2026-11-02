package main

// Cross-process proof for live refresh (UI-22, UI-25): a write made by a
// separate OS process through internal/app reaches the Desktop bindings. The
// test binary re-executes itself in a helper mode, chosen by an environment
// variable that exists only in this test file.

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/app/apptest"
	"github.com/jameskbb/casebook/internal/testenv"
)

// envHelperDB names the database the helper process writes to.
const envHelperDB = "CASEBOOK_DESKTOP_HELPER_DB"

// helperEvidenceTitle is the evidence the helper process adds.
const helperEvidenceTitle = "Written by another process"

func TestMain(m *testing.M) {
	testenv.CanonicalTempDir()
	if path := os.Getenv(envHelperDB); path != "" {
		os.Exit(helperWrite(path))
	}
	os.Exit(m.Run())
}

// helperWrite is the child side: it opens path, adds one evidence record to
// the active investigation as Claude Code and returns the exit code.
func helperWrite(path string) int {
	ctx := context.Background()
	a, err := app.Open(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "helper: open:", err)
		return 1
	}
	defer a.Close()
	active, err := a.GetActiveInvestigation(ctx)
	if err != nil || active == nil {
		fmt.Fprintln(os.Stderr, "helper: no active investigation:", err)
		return 1
	}
	_, err = a.AddEvidence(ctx, app.ActorMCP("claude-code"), active.Ref, app.EvidenceInput{
		Title:      helperEvidenceTitle,
		Summary:    "A separate process added this through internal/app.",
		SourceType: app.SourceTypeAIToolOutput,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "helper: add evidence:", err)
		return 1
	}
	return 0
}

func TestOtherProcessWriteReachesDesktop(t *testing.T) {
	d, _ := newTestDesktop(t)
	fx := apptest.SeedCloudSpend(t, d.app)
	before, err := d.ChangeToken()
	if err != nil || before == "" {
		t.Fatalf("ChangeToken = %q, %v", before, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^$")
	cmd.Env = append(os.Environ(), envHelperDB+"="+d.app.DBPath())
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("helper process: %v\n%s", err, out)
	}

	after, err := d.ChangeToken()
	if err != nil {
		t.Fatal(err)
	}
	if after == before {
		t.Fatalf("token stayed %q after another process wrote", after)
	}
	det, err := d.GetInvestigationDetail(fx.InvestigationRef, app.DetailOptions{})
	if err != nil {
		t.Fatal(err)
	}
	want := app.ActorMCP("claude-code")
	found := false
	for _, ev := range det.Evidence {
		if ev.Title == helperEvidenceTitle {
			found = true
			if ev.CreatedBy != want {
				t.Fatalf("evidence created by %v, want %v", ev.CreatedBy, want)
			}
		}
	}
	if !found {
		t.Fatal("the other process's evidence is not in the detail")
	}
	if len(det.RecentActivity) == 0 || det.RecentActivity[0].Actor != want || det.RecentActivity[0].Action != "evidence.add" {
		t.Fatalf("newest activity = %+v", det.RecentActivity[0])
	}
}
