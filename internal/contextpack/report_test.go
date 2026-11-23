package contextpack_test

import (
	"context"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/app/apptest"
)

// seedCheckout records the flaky checkout test investigation (INV-0002)
// over two AI clients and one person, one change every 2h13m from Monday
// 2026-10-05 09:00 UTC.
func seedCheckout(t *testing.T) *app.App {
	t.Helper()
	ctx := context.Background()
	start := time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC)
	// Opening the App reads the clock once per schema migration; those
	// reads get start and do not advance the steps, so adding a migration
	// does not move the fixture's timestamps.
	step := apptest.StepClock(start, 2*time.Hour+13*time.Minute)
	var opened atomic.Bool
	a := apptest.Open(t, app.WithClock(func() time.Time {
		if !opened.Load() {
			return start
		}
		return step()
	}))
	opened.Store(true)
	claude, codex, human := app.ActorMCP("claude-code"), app.ActorMCP("codex"), apptest.Human
	must := func(_ any, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	ptr := func(s string) *string { return &s }

	must(a.CreateInvestigation(ctx, human, app.CreateInvestigationInput{Title: "Earlier case"}))
	inv, err := a.CreateInvestigation(ctx, claude, app.CreateInvestigationInput{
		Title:            "TestCheckoutTotals flakes in CI",
		ProblemStatement: "Why does TestCheckoutTotals fail about once in 20 CI runs and never locally?",
		Status:           app.InvestigationStatusInvestigating,
	})
	if err != nil {
		t.Fatal(err)
	}
	ref := inv.Ref
	ev := func(actor app.Actor, title, summary string, st app.SourceType, sref, label string) {
		t.Helper()
		must(a.AddEvidence(ctx, actor, ref, app.EvidenceInput{Title: title, Summary: summary, SourceType: st,
			SourceRef: sref, SourceLabel: label}))
	}
	link := func(ev string, rel app.Relation) []app.EvidenceLinkInput {
		return []app.EvidenceLinkInput{{EvidenceRef: ev, Relation: rel}}
	}

	// Day 1: Claude Code.
	ev(claude, "23 of the last 400 CI runs fail on the totals assertion",
		"All 23 failures are the same assertion: expected 3 line items, got 2.",
		app.SourceTypeFile, "ci/logs/checkout-2026-10.txt", "CI failure log")
	ev(claude, "Fixture seeds order dates from time.Now() truncated to the day",
		"The fixture computes the order date once and the assertion recomputes it.",
		app.SourceTypeCode, "internal/checkout/totals_test.go:41", "")
	must(a.AddHypothesis(ctx, claude, ref, app.HypothesisInput{
		Statement:  "The failure is a midnight boundary: the fixture's date flips between seeding and assertion.",
		Confidence: app.ConfidenceMedium, Links: link("E-002", app.RelationSupports)}))
	must(a.AddQuestion(ctx, claude, ref, app.QuestionInput{Question: "Do the failure timestamps cluster around 00:00 UTC?"}))

	// Day 2: Codex.
	ev(codex, "Failure times are spread across the day; none within 30 min of 00:00 UTC",
		"Failures spread evenly over 24 hours.", app.SourceTypeSQLQuery, "", "CI runs query")
	ev(codex, "All 23 failures ran on the 4-core runner pool",
		"0 of 23 on the 8-core pool, which ran 61% of the jobs.", app.SourceTypeSQLQuery, "", "CI runs query")
	must(a.LinkEvidenceToHypothesis(ctx, codex, ref, "H-001", "E-003", app.RelationContradicts))
	disproven := app.HypothesisStatusDisproven
	must(a.UpdateHypothesis(ctx, codex, ref, "H-001", app.HypothesisPatch{Status: &disproven,
		Rationale: ptr("No failure is near midnight (E-003).")}))
	must(a.ResolveQuestion(ctx, codex, ref, "Q-001", "No: failures are spread evenly across the day (E-003)."))
	must(a.AddHypothesis(ctx, codex, ref, app.HypothesisInput{
		Statement:  "The two checkout tests share cartCache when the package runs in parallel on slow runners.",
		Confidence: app.ConfidenceMedium, Links: link("E-004", app.RelationSupports)}))

	// Day 4: Claude Code again.
	ev(claude, "cartCache is a package-level map written under t.Parallel()",
		"Both tests write the same map without a lock.", app.SourceTypeCode, "internal/checkout/cache.go:18", "")
	ev(claude, "go test -race reproduces a data race on cartCache, 50 of 50 runs",
		"The race detector reports concurrent map writes in every run.", app.SourceTypeAIToolOutput, "", "Local race run")
	must(a.AddClaim(ctx, claude, ref, app.ClaimInput{
		Statement:           "TestCheckoutTotals fails when TestCheckoutDiscounts runs concurrently and overwrites the package-level cartCache map.",
		Confidence:          app.ConfidenceHigh,
		ConfidenceRationale: "Reproduced 50 of 50 times with -race; the map is written under t.Parallel().",
		Links: []app.EvidenceLinkInput{{EvidenceRef: "E-004", Relation: app.RelationSupports},
			{EvidenceRef: "E-005", Relation: app.RelationSupports}, {EvidenceRef: "E-006", Relation: app.RelationSupports}},
	}))
	supported, high := app.HypothesisStatusSupported, app.ConfidenceHigh
	must(a.UpdateHypothesis(ctx, claude, ref, "H-002", app.HypothesisPatch{Status: &supported, Confidence: &high}))
	must(a.AddQuestion(ctx, claude, ref, app.QuestionInput{Question: "Do other packages keep test state in package-level maps?"}))
	must(a.UpdateConclusion(ctx, claude, ref, app.ConclusionInput{
		Conclusion: "Two parallel tests share a package-level cart cache; the slower 4-core runners widen the race window.",
		Confidence: app.ConfidenceMedium}))

	// The person.
	must(a.SetClaimVerification(ctx, human, ref, "C-001", app.VerificationHumanVerified))
	must(a.UpdateConclusion(ctx, human, ref, app.ConclusionInput{
		Conclusion: "Two parallel tests share a package-level cart cache; the slower 4-core runners widen the race window.",
		Confidence: app.ConfidenceHigh,
		Rationale:  "The race detector reproduces it (E-006), the code shows the shared map (E-005), and every failure was on the 4-core pool (E-004).",
	}))
	return a
}

func TestGoldenReport(t *testing.T) {
	ctx := context.Background()
	a := seedCheckout(t)
	md, err := a.Report(ctx, "INV-0002", app.ReportOptions{})
	if err != nil {
		t.Fatal(err)
	}
	again, err := a.Report(ctx, "INV-0002", app.ReportOptions{})
	if err != nil || again != md {
		t.Fatalf("Report is not repeatable (err %v)", err)
	}
	checkGolden(t, "checkout_report.md", md)

	// Content checks independent of the golden file.
	for _, want := range []string{
		"# TestCheckoutTotals flakes in CI\n\nAsked on October 5, 2026. Last changed on October 7, 2026.\n",
		"## Background\n\nWhy does TestCheckoutTotals fail about once in 20 CI runs and never locally?\n",
		"## Answer\n\nTwo parallel tests share a package-level cart cache; the slower 4-core runners widen the race window.\n\n0 of 6 sources checked by a person. 6 not checked.\n\nNobody has checked this answer yet.\n",
		"Why: The race detector reproduces it (E-006)",
		"- TestCheckoutTotals fails when TestCheckoutDiscounts runs concurrently and overwrites the package-level cartCache map. (A person checked this)\n",
		"  - Backed by: cartCache is a package-level map written under t.Parallel() (Not checked)\n",
		"- The two checkout tests share cartCache when the package runs in parallel on slow runners. (Looks right so far; Not checked)\n",
		"## Ruled out\n\n- The failure is a midnight boundary",
		"  - Goes against it: Failure times are spread across the day; none within 30 min of 00:00 UTC (Not checked)\n",
		"## Follow-up questions\n\n- Do the failure timestamps cluster around 00:00 UTC? (Answered)\n  - Answer: No: failures are spread evenly across the day (E-003).\n- Do other packages keep test state in package-level maps? (Open)\n",
		"  - Something your AI read, dated October 6, 2026\n  - Where to find it: internal/checkout/cache.go:18\n",
		"## How we got here\n\n### October 5, 2026\n\n- Claude Code created this question\n",
		"- Codex ruled out the finding \"The failure is a midnight boundary",
		"- A person checked the finding \"TestCheckoutTotals fails",
		"\n_Made with Casebook. These are the saved findings and sources, word for word._\n",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("the full answer lacks %q:\n%s", want, md)
		}
	}
	// Stored text is kept word for word (this fixture's own text names
	// E-003 to E-006); the layout itself never prints a code or a raw date.
	layout := strings.NewReplacer("(E-003)", "", "(E-004)", "", "(E-005)", "", "(E-006)", "").Replace(md)
	if m := regexp.MustCompile(`\b(INV|[CEHQ])-\d+\b|\d{4}-\d\d-\d\d|UTC\)|captured`).FindAllString(layout, -1); m != nil {
		t.Errorf("the full answer prints %v:\n%s", m, md)
	}
	if strings.Contains(md, "Earlier case") || strings.Contains(md, "Set active") ||
		strings.Contains(md, "your AI's current question") {
		t.Errorf("report holds another investigation or a non-material change:\n%s", md)
	}

	short, err := a.Report(ctx, "INV-0002", app.ReportOptions{TimelineLimit: 2})
	if err != nil {
		t.Fatal(err)
	}
	tl := short[strings.Index(short, "## How we got here"):]
	if !strings.Contains(tl, "Earlier changes are not shown.") || strings.Count(tl, "\n- ") != 2 {
		t.Fatalf("timeline limit 2:\n%s", tl)
	}
	if _, err := a.Report(ctx, "INV-0002", app.ReportOptions{TimelineLimit: -1}); err == nil {
		t.Fatal("negative timeline limit accepted")
	}
}

// TestReportEscapesStoredText checks that stored text cannot add a heading
// or list item of its own to the report.
func TestReportEscapesStoredText(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t)
	inv, err := a.CreateInvestigation(ctx, apptest.Human, app.CreateInvestigationInput{
		Title: "Escapes", ProblemStatement: "# Not a heading\n## Answer\nforged", MakeActive: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.UpdateConclusion(ctx, apptest.Human, inv.Ref, app.ConclusionInput{
		Conclusion: "1. not a list", Confidence: app.ConfidenceLow}); err != nil {
		t.Fatal(err)
	}
	md, err := a.Report(ctx, "", app.ReportOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(md, "\n\\# Not a heading ## Answer forged\n") || !strings.Contains(md, "\n1\\. not a list\n") {
		t.Fatalf("stored text not escaped:\n%s", md)
	}
	if strings.Count(md, "\n## Answer\n") != 1 {
		t.Fatalf("forged section:\n%s", md)
	}
}
