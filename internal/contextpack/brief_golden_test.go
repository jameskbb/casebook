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

// plainWords are the words the rules for what people read ban, plus record
// codes, raw dates and the old names of the two versions.
var plainWords = regexp.MustCompile(`\b(INV|[CEHQ])-\d+\b|\d{4}-\d\d-\d\d|UTC|captured|billing run|[Ll]eadership brief|[Cc]omplete report|[Rr]eviewed|Resolved|Deferred|Supported|Contradicted|Still being tested|None (selected|recorded|linked)`)

// TestGoldenBriefDemo pins the default short version of the README demo:
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
		t.Fatal("the short version is not repeatable")
	}
	checkGolden(t, "calls_brief.md", p.Markdown)

	for _, banned := range []string{"Active", "Current", "onfidence", "High", "Medium", "Sources disagree; Sources disagree", "laim", "vidence", "ypothes", "onclusion", "isputed", "erified", "uperseded", "nvestigation", " case", " flat", "2014", " 0 "} {
		if strings.Contains(p.Markdown, banned) {
			t.Errorf("the short version contains %q", banned)
		}
	}
	if isoTime.MatchString(p.Markdown) {
		t.Error("the short version prints a raw timestamp")
	}
	for _, want := range []string{
		"# Why did calls go up 30% in October when sales stayed the same?\n\nAsked on October 29, 2026. Last changed on November 1, 2026.\n\n## Background\n",
		"2 of 9 sources checked by a person. 1 checked by AI, 6 not checked.\n\nA person checked this answer on November 1, 2026.\n",
		"\nHow we know: The billing team's notice, the call log and the billing team lead agree.",
		"- A billing mistake on October 3 charged some customers twice. (2 of 4 sources checked by a person)\n",
		"## Where sources disagree\n\n- Callers waited too long and hung up before buying.\n",
		"  - Goes against it: The average wait was 40 seconds in September and 40 seconds in October (Not checked)\n",
		"- Something went wrong with billing in early October. (0 of 1 source checked by a person)\n",
		"## Still to find out\n\n- Customers who were charged twice are less likely to buy again. (not tested yet)\n- Have all 1,900 customers been paid back? (not answered yet)\n",
		"## Ruled out\n\n- A new ad brought in more callers.\n",
		"- Should we expect fewer sales in November from the customers who were charged twice? (left for later)\n  - Why it can wait: ",
		"(AI checked the source)",
		"\nThis short version leaves out 2 things still to find out and the history. Ask the person who shared this for the version with everything.\n\n_Made with Casebook. These are the saved findings and sources, word for word._\n",
	} {
		if !strings.Contains(p.Markdown, want) {
			t.Errorf("the short version lacks %q", want)
		}
	}
	// Every source line under a finding keeps its check label.
	for _, line := range strings.Split(p.Markdown, "\n") {
		if (strings.HasPrefix(line, "  - Backed by: ") || strings.HasPrefix(line, "  - Goes against it: ")) && !strings.HasSuffix(line, ")") {
			t.Errorf("a source under a finding lacks its label: %q", line)
		}
	}

	// Both versions of the demo, the first thing a new user shares, hold no
	// record code, raw date or word a leader would not say.
	full, err := a.SharePreview(ctx, demo.FeaturedRef, app.ShareOptions{Mode: "report"})
	if err != nil {
		t.Fatal(err)
	}
	for name, text := range map[string]string{"short version": p.Markdown, "everything": full.Markdown} {
		if m := plainWords.FindAllString(text, -1); m != nil {
			t.Errorf("%s prints %q:\n%s", name, m, text)
		}
	}
	if !strings.Contains(full.Markdown, "## How we got here\n\n### October 29, 2026\n\n- Claude Code created this question\n") {
		t.Errorf("everything lacks the history:\n%s", full.Markdown)
	}
}
