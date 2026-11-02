package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/app/apptest"
)

// noActiveStderr is the full stderr line for a missing active investigation.
const noActiveStderr = `casebook: no active investigation is set; pass an investigation ref or set one active; ` +
	`run "casebook case activate INV-0001" or pass a ref` + "\n"

// seededApp is seededDB that also returns the seeding App, so a test can
// compute expected values or change data through internal/app.
func seededApp(t *testing.T) (*app.App, string, apptest.Fixture) {
	t.Helper()
	a := apptest.Open(t)
	f := apptest.SeedCloudSpend(t, a)
	return a, a.DBPath(), f
}

// decodeJSON parses s into v and asserts the JSON holds no nulls.
func decodeJSON(t *testing.T, s string, v any) {
	t.Helper()
	if err := json.Unmarshal([]byte(s), v); err != nil {
		t.Fatalf("stdout is not valid JSON for %T: %v\n%s", v, err, s)
	}
	var raw any
	if err := json.Unmarshal([]byte(s), &raw); err != nil {
		t.Fatal(err)
	}
	apptest.AssertNoJSONNulls(t, raw)
}

// wantContains fails for every want missing from s.
func wantContains(t *testing.T, s string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !strings.Contains(s, w) {
			t.Errorf("output does not contain %q:\n%s", w, s)
		}
	}
}

func TestActivateAndContextTracer(t *testing.T) {
	a, db, _ := seededApp(t)
	ctx := t.Context()

	r := runCLI(t, "", "--db", db, "investigation", "activate", "--clear")
	wantCode(t, r, 0)
	if r.Stdout != "Cleared the active case.\n" {
		t.Errorf("clear stdout = %q", r.Stdout)
	}

	r = runCLI(t, "", "--db", db, "context")
	wantCode(t, r, 3)
	if r.Stderr != noActiveStderr {
		t.Errorf("stderr = %q, want %q", r.Stderr, noActiveStderr)
	}
	if r.Stdout != "" {
		t.Errorf("stdout = %q, want empty", r.Stdout)
	}

	r = runCLI(t, "", "--db", db, "inv", "activate", "inv-1")
	wantCode(t, r, 0)
	if r.Stdout != "Active case: INV-0001 September cloud bill up 38%\n" {
		t.Errorf("activate stdout = %q", r.Stdout)
	}

	want, err := a.RenderContext(ctx, "INV-0001", "markdown", app.ContextOptions{})
	if err != nil {
		t.Fatal(err)
	}
	r = runCLI(t, "", "--db", db, "context")
	wantCode(t, r, 0)
	if r.Stdout != withNewline(want) {
		t.Errorf("context stdout differs from app.RenderContext:\ngot:\n%s\nwant:\n%s", r.Stdout, want)
	}
}

func TestActivateJSON(t *testing.T) {
	_, db, _ := seededApp(t)
	r := runCLI(t, "", "--db", db, "--json", "investigation", "activate", "INV-0001")
	wantCode(t, r, 0)
	var inv app.Investigation
	decodeJSON(t, r.Stdout, &inv)
	if inv.Ref != "INV-0001" || !inv.IsActive {
		t.Errorf("inv = %+v", inv)
	}

	r = runCLI(t, "", "--db", db, "--json", "investigation", "activate", "--clear")
	wantCode(t, r, 0)
	var cleared map[string]string
	decodeJSON(t, r.Stdout, &cleared)
	if v, ok := cleared["active_investigation_ref"]; !ok || v != "" {
		t.Errorf("clear JSON = %v", cleared)
	}
}

func TestActivateErrors(t *testing.T) {
	_, db, _ := seededApp(t)
	cases := []struct {
		name   string
		args   []string
		code   int
		stderr string
	}{
		{"no ref and no clear", []string{"investigation", "activate"}, 2, "casebook: pass a case ref or --clear"},
		{"ref and clear", []string{"investigation", "activate", "INV-0001", "--clear"}, 2, "casebook: pass a case ref or --clear"},
		{"two refs", []string{"investigation", "activate", "INV-0001", "INV-0002"}, 2, "casebook: "},
		{"not found", []string{"investigation", "activate", "INV-0099"}, 3, "casebook: not found:"},
		{"malformed ref", []string{"investigation", "activate", "bogus"}, 2, "casebook: invalid investigation_ref:"},
		{"unknown subcommand", []string{"inv", "bogus"}, 2, "casebook: "},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := runCLI(t, "", append([]string{"--db", db}, c.args...)...)
			wantCode(t, r, c.code)
			if !strings.HasPrefix(r.Stderr, c.stderr) || strings.Count(r.Stderr, "\n") != 1 {
				t.Errorf("stderr = %q, want one line starting %q", r.Stderr, c.stderr)
			}
		})
	}
}

func TestActivateGroupHelp(t *testing.T) {
	_, db, _ := seededApp(t)
	for _, name := range []string{"investigation", "inv"} {
		r := runCLI(t, "", "--db", db, name)
		wantCode(t, r, 0)
		wantContains(t, r.Stdout, "Usage:", "activate")
	}
}

// TestCaseAliasListsInvestigations checks that case reaches the same command
// as investigation (ADR 0021).
func TestCaseAliasListsInvestigations(t *testing.T) {
	_, db, _ := seededApp(t)
	r := runCLI(t, "", "--db", db, "case", "list")
	wantCode(t, r, 0)
	wantContains(t, r.Stdout, "INV-0001", "September cloud bill up 38%")

	r = runCLI(t, "", "--db", db, "case")
	wantCode(t, r, 0)
	wantContains(t, r.Stdout, "Usage:", "activate", "cases")
}

// withNewline returns s ending in exactly the newline the CLI adds.
func withNewline(s string) string {
	if strings.HasSuffix(s, "\n") {
		return s
	}
	return s + "\n"
}

func TestInvestigationList(t *testing.T) {
	a, db, _ := seededApp(t)
	ctx := t.Context()

	r := runCLI(t, "", "--db", db, "investigation", "list")
	wantCode(t, r, 0)
	lines := strings.Split(strings.TrimRight(r.Stdout, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("want a header and one row:\n%s", r.Stdout)
	}
	wantContains(t, lines[0], "REF", "STATUS", "CONFIDENCE", "UPDATED", "ACTIVE", "TITLE")
	wantContains(t, lines[1], "INV-0001", "Investigating", "High", "September cloud bill up 38%", "*")

	r = runCLI(t, "", "--db", db, "--json", "inv", "list")
	wantCode(t, r, 0)
	var sums []app.InvestigationSummary
	decodeJSON(t, r.Stdout, &sums)
	if len(sums) != 1 || sums[0].Ref != "INV-0001" || sums[0].Counts.Evidence != 4 || !sums[0].IsActive {
		t.Errorf("summaries = %+v", sums)
	}

	arch, err := a.CreateInvestigation(ctx, apptest.Human, app.CreateInvestigationInput{Title: "Old refund spike"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.ArchiveInvestigation(ctx, apptest.Human, arch.Ref); err != nil {
		t.Fatal(err)
	}
	r = runCLI(t, "", "--db", db, "investigation", "list")
	wantCode(t, r, 0)
	if strings.Contains(r.Stdout, arch.Ref) {
		t.Errorf("archived investigation listed without --archived:\n%s", r.Stdout)
	}
	r = runCLI(t, "", "--db", db, "investigation", "list", "--archived")
	wantCode(t, r, 0)
	wantContains(t, r.Stdout, "INV-0001", arch.Ref, "Archived", "Old refund spike")
	r = runCLI(t, "", "--db", db, "investigation", "list", "--status", "archived")
	wantCode(t, r, 0)
	wantContains(t, r.Stdout, arch.Ref)
	if strings.Contains(r.Stdout, "INV-0001") {
		t.Errorf("--status archived lists INV-0001:\n%s", r.Stdout)
	}
	r = runCLI(t, "", "--db", db, "investigation", "list", "--limit", "1", "--archived")
	wantCode(t, r, 0)
	if n := strings.Count(strings.TrimRight(r.Stdout, "\n"), "\n"); n != 1 {
		t.Errorf("--limit 1 printed %d rows:\n%s", n, r.Stdout)
	}

	r = runCLI(t, "", "--db", db, "investigation", "list", "--limit", "-1")
	wantCode(t, r, 2)
	if !strings.HasPrefix(r.Stderr, "casebook: invalid limit:") {
		t.Errorf("stderr = %q", r.Stderr)
	}
	r = runCLI(t, "", "--db", db, "investigation", "list", "extra")
	wantCode(t, r, 2)
}

func TestInvestigationListEmpty(t *testing.T) {
	db := t.TempDir() + "/empty.db"
	r := runCLI(t, "", "--db", db, "investigation", "list")
	wantCode(t, r, 0)
	if !strings.HasPrefix(r.Stdout, "No cases.") {
		t.Errorf("stdout = %q", r.Stdout)
	}
	r = runCLI(t, "", "--db", db, "--json", "investigation", "list")
	wantCode(t, r, 0)
	if strings.TrimSpace(r.Stdout) != "[]" {
		t.Errorf("empty --json = %q, want []", r.Stdout)
	}
}

func TestInvestigationShow(t *testing.T) {
	_, db, _ := seededApp(t)
	r := runCLI(t, "", "--db", db, "investigation", "show")
	wantCode(t, r, 0)
	wantContains(t, r.Stdout,
		"INV-0001", "September cloud bill up 38%",
		"Status:", "Investigating", "Active:", "yes",
		"Why did the cloud bill rise 38% in September when traffic was flat?",
		"Debug logging requested for payments only reached every service and drove most of the increase.", "(High, Human verified)",
		"Evidence (4)", "E-001", "E-002", "E-003", "E-004",
		"Claims (3)", "C-001", "C-002", "C-003", "supports: E-001, E-004", "superseded by C-002",
		"Hypotheses (2)", "H-001", "H-002", "contradicts: E-003",
		"Questions (1)", "Q-001", "Can every service except payments go back to info level today?",
		"Recent activity (")
	if strings.Contains(r.Stdout, "level: info") {
		t.Errorf("show printed E-001's raw detail:\n%s", r.Stdout)
	}

	r = runCLI(t, "", "--db", db, "--json", "investigation", "show", "INV-0001")
	wantCode(t, r, 0)
	var d app.InvestigationDetail
	decodeJSON(t, r.Stdout, &d)
	if d.Investigation.Ref != "INV-0001" || len(d.Evidence) != 4 || len(d.Claims) != 3 || len(d.Questions) != 1 {
		t.Errorf("detail = %+v", d)
	}
	if strings.Contains(r.Stdout, `"raw_detail"`) || strings.Contains(r.Stdout, "level: info") {
		t.Error("show --json includes raw detail")
	}

	r = runCLI(t, "", "--db", db, "investigation", "show", "INV-0099")
	wantCode(t, r, 3)
	if !strings.HasPrefix(r.Stderr, "casebook: not found:") {
		t.Errorf("stderr = %q", r.Stderr)
	}
}

func TestInvestigationActive(t *testing.T) {
	_, db, _ := seededApp(t)
	r := runCLI(t, "", "--db", db, "investigation", "active")
	wantCode(t, r, 0)
	if r.Stdout != "INV-0001  September cloud bill up 38%  (Investigating)\n" {
		t.Errorf("stdout = %q", r.Stdout)
	}
	r = runCLI(t, "", "--db", db, "--json", "investigation", "active")
	wantCode(t, r, 0)
	var inv app.Investigation
	decodeJSON(t, r.Stdout, &inv)
	if inv.Ref != "INV-0001" || !inv.IsActive {
		t.Errorf("inv = %+v", inv)
	}

	wantCode(t, runCLI(t, "", "--db", db, "investigation", "activate", "--clear"), 0)
	r = runCLI(t, "", "--db", db, "investigation", "active")
	wantCode(t, r, 3)
	if r.Stderr != noActiveStderr {
		t.Errorf("stderr = %q", r.Stderr)
	}
	r = runCLI(t, "", "--db", db, "investigation", "show")
	wantCode(t, r, 3)
	if r.Stderr != noActiveStderr {
		t.Errorf("show stderr = %q", r.Stderr)
	}
}

func TestInvestigationCreate(t *testing.T) {
	_, db, _ := seededApp(t)
	r := runCLI(t, "", "--db", db, "investigation", "create", "--title", "Refund spike", "--problem", "Why?", "--activate")
	wantCode(t, r, 0)
	wantContains(t, r.Stdout, "Created INV-0002", `"Refund spike"`, "now active")

	r = runCLI(t, "", "--db", db, "investigation", "active")
	wantCode(t, r, 0)
	wantContains(t, r.Stdout, "INV-0002", "Refund spike")

	r = runCLI(t, "", "--db", db, "--json", "inv", "create", "--title", "Second")
	wantCode(t, r, 0)
	var inv app.Investigation
	decodeJSON(t, r.Stdout, &inv)
	if inv.Ref != "INV-0003" || inv.IsActive || inv.CreatedBy != app.ActorCLI() {
		t.Errorf("inv = %+v", inv)
	}
	r = runCLI(t, "", "--db", db, "inv", "create", "--title", "Third")
	wantCode(t, r, 0)
	if strings.Contains(r.Stdout, "now active") {
		t.Errorf("stdout = %q", r.Stdout)
	}

	cases := []struct {
		name   string
		args   []string
		stderr string
	}{
		{"no title", []string{"investigation", "create"}, "casebook: --title is required"},
		{"empty title", []string{"investigation", "create", "--title", ""}, "casebook: --title is required"},
		{"long title", []string{"investigation", "create", "--title", strings.Repeat("x", 201)}, "casebook: invalid title:"},
		{"positional", []string{"investigation", "create", "Title"}, "casebook: "},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := runCLI(t, "", append([]string{"--db", db}, c.args...)...)
			wantCode(t, r, 2)
			if !strings.HasPrefix(r.Stderr, c.stderr) {
				t.Errorf("stderr = %q, want prefix %q", r.Stderr, c.stderr)
			}
		})
	}
}

// TestCaseWordsMatchTheApp pins the CLI words shared with the desktop app:
// the help ties the active case to "your AI's current case", and the tables
// print "Not assessed" where no confidence is recorded (C-5).
func TestCaseWordsMatchTheApp(t *testing.T) {
	a, db, _ := seededApp(t)
	ctx := t.Context()

	r := runCLI(t, "", "case", "--help")
	wantCode(t, r, 0)
	wantContains(t, r.Stdout, "The active case (the app\ncalls it your AI's current case)")
	wantContains(t, r.Stdout, "A project directory can have its own active case (activate\n--here), which wins inside that directory.")

	inv, err := a.CreateInvestigation(ctx, apptest.Human, app.CreateInvestigationInput{Title: "Northeast margin down"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.AddClaim(ctx, apptest.AI, inv.Ref, app.ClaimInput{Statement: "Freight costs rose."}); err != nil {
		t.Fatal(err)
	}
	disputed, err := a.AddClaim(ctx, apptest.AI, inv.Ref, app.ClaimInput{Statement: "Discounts explain it.", Confidence: app.ConfidenceLow})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.SetClaimVerification(ctx, apptest.Human, inv.Ref, disputed.Ref, app.VerificationDisputed); err != nil {
		t.Fatal(err)
	}
	r = runCLI(t, "", "--db", db, "case", "list")
	wantCode(t, r, 0)
	var row string
	for l := range strings.SplitSeq(r.Stdout, "\n") {
		if strings.HasPrefix(l, inv.Ref) {
			row = l
		}
	}
	wantContains(t, row, "Not assessed")
	if strings.Contains(r.Stdout, "Unknown") {
		t.Errorf("list printed Unknown:\n%s", r.Stdout)
	}

	r = runCLI(t, "", "--db", db, "case", "show", inv.Ref)
	wantCode(t, r, 0)
	wantContains(t, r.Stdout, "[Current, Not assessed, AI generated]")
	// A disputed claim names Disputed once: status, confidence, then the
	// verification label only when it differs from the status.
	wantContains(t, r.Stdout, "[Disputed, Low]")
	if strings.Contains(r.Stdout, "Disputed, Low, Disputed") {
		t.Errorf("show repeated Disputed:\n%s", r.Stdout)
	}
	if strings.Contains(r.Stdout, "Unknown") {
		t.Errorf("show printed Unknown:\n%s", r.Stdout)
	}
}
