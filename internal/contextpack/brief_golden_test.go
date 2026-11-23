package contextpack_test

import (
	"context"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/app/apptest"
	"github.com/jameskbb/casebook/internal/demo"
)

// demoNow is the fixed "now" the demo is seeded against; the brief carries
// no time of its own, so any value gives the same bytes.
var demoNow = time.Date(2026, 11, 1, 18, 0, 0, 0, time.UTC)

// seedDemo writes the README demo (INV-0002 is the featured case).
func seedDemo(t *testing.T) *app.App {
	t.Helper()
	clock := demo.NewClock(demo.Start(demoNow))
	a := apptest.Open(t, app.WithClock(clock.Now))
	if err := demo.Seed(context.Background(), a, clock, demoNow, demo.StageFull); err != nil {
		t.Fatal(err)
	}
	return a
}

// isoTime matches a raw RFC 3339 time of day, which a brief must not print.
var isoTime = regexp.MustCompile(`T\d\d:\d\d:\d\d`)

// TestGoldenBriefDemo pins the default Leadership brief of the README demo:
// the bytes a person copies to a recipient who has no Casebook.
func TestGoldenBriefDemo(t *testing.T) {
	ctx := context.Background()
	a := seedDemo(t)
	p, err := a.SharePreview(ctx, demo.FeaturedRef, app.ShareOptions{Mode: "brief"})
	if err != nil {
		t.Fatal(err)
	}
	again, err := a.SharePreview(ctx, demo.FeaturedRef, app.ShareOptions{Mode: "brief"})
	if err != nil {
		t.Fatal(err)
	}
	if again.Markdown != p.Markdown {
		t.Fatal("brief is not repeatable")
	}
	checkGolden(t, "calls_brief.md", p.Markdown)

	for _, banned := range []string{"Active", "Current", "onfidence", "High", "Medium", "Sources disagree; Sources disagree", "laim", "vidence", "ypothes", "onclusion", "isputed", "erified", "uperseded", "nvestigation", " case", " flat", "2014", " 0 "} {
		if strings.Contains(p.Markdown, banned) {
			t.Errorf("brief contains %q", banned)
		}
	}
	if isoTime.MatchString(p.Markdown) {
		t.Error("brief prints a raw timestamp")
	}
	for _, want := range []string{
		"# INV-0002: Why did calls go up 30% in October when sales stayed the same?\n",
		"2 of 9 sources checked by you. 1 checked by AI, 6 not checked.",
		"Answer marked: You checked this",
		"Findings behind the answer: C-002 (Not checked); C-003 (You checked this)",
		"Findings against the answer: C-001 (Sources disagree)",
		"- **C-001** Callers waited too long and hung up before buying. (Sources disagree)",
		"(Ruled out; Not checked)",
		"(AI checked the source)",
		"Labels: You checked this means the person who shared this checked it against its source.",
		"Replaced means a newer finding took its place.",
		"Not included: Q-001 and Q-002. The history is not included. Ask the person who shared this for the full version.",
		"## Still being tested\n\n- **H-003** ",
		"- **H-004** Customers who were charged twice are less likely to buy again. (Not tested yet; Not checked)",
		"Findings where sources disagree, sources that contradict a finding and the findings behind the answer are always included.",
		"_Prepared by Casebook from the saved records of INV-0002. Casebook did not rewrite them or add to them._",
	} {
		if !strings.Contains(p.Markdown, want) {
			t.Errorf("brief missing %q", want)
		}
	}
	// The finding not tested yet sits under its own heading, after the
	// findings and before the follow-up questions.
	if f, h, q := strings.Index(p.Markdown, "## Findings"), strings.Index(p.Markdown, "- **H-004**"), strings.Index(p.Markdown, "## Follow-up questions"); !(f < h && h < q) {
		t.Error("H-004 is not between the findings and the questions")
	}
	// Ruled-out hypotheses sit under their own heading, after the open
	// explanations and before the questions, not among the things in doubt.
	if e, r, d, q := strings.Index(p.Markdown, "## Still being tested"), strings.Index(p.Markdown, "## Ruled out\n\n- **H-"), strings.Index(p.Markdown, "(Ruled out; Not checked)"), strings.Index(p.Markdown, "## Follow-up questions"); !(e < r && r < d && d < q) {
		t.Error("the Ruled out section is not between the open explanations and the questions")
	}
	// Every evidence line under a finding keeps its verification label; the
	// source path prints once, under Evidence and source caveats.
	for _, line := range strings.Split(p.Markdown, "\n") {
		if strings.HasPrefix(line, "  - Supported by ") || strings.HasPrefix(line, "  - Contradicted by ") {
			if !strings.HasSuffix(line, ")") || strings.Contains(line, "Call log, ") {
				t.Errorf("finding evidence line lacks its label or repeats its source: %q", line)
			}
		}
	}
}
