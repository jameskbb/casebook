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
var demoNow = time.Date(2026, 10, 9, 14, 0, 0, 0, time.UTC)

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
	checkGolden(t, "margin_brief.md", p.Markdown)

	for _, banned := range []string{"Active", "Disputed, Disputed", "Disputed; Low confidence; Disputed", "retained automatically", "selected separately", "material caveats", "No new claims", "1 evidence items", " 0 "} {
		if strings.Contains(p.Markdown, banned) {
			t.Errorf("brief contains %q", banned)
		}
	}
	if isoTime.MatchString(p.Markdown) {
		t.Error("brief prints a raw timestamp")
	}
	for _, want := range []string{
		"Supporting claims: C-002 (AI generated); C-003 (Human verified)",
		"Contradicting claims: C-001 (Disputed)",
		"(Disputed; Low confidence)",
		"(Disproven; AI generated)",
		"Labels: Human verified means a person checked this and vouched for it.",
		"Superseded means it was replaced by a newer statement.",
		"Not included: Q-001, Q-002 and E-002. Activity history is not included. Ask the person who shared this for the complete report.",
		"## Explanations still open or supported\n\n- **H-003** ",
		"- **H-004** Travel for on-site white-glove sessions is growing with the same accounts. (Open; Medium confidence; AI generated)",
		"Disputed findings, contradicting evidence and the claims behind the answer are always included.",
		"_Prepared by Casebook from the saved records of INV-0002. Casebook did not rewrite them or add to them._",
	} {
		if !strings.Contains(p.Markdown, want) {
			t.Errorf("brief missing %q", want)
		}
	}
	// The open explanation the confidence sentence cites sits under its own
	// heading, after the findings and before the questions.
	if f, h, q := strings.Index(p.Markdown, "## Selected findings"), strings.Index(p.Markdown, "- **H-004**"), strings.Index(p.Markdown, "## Selected questions"); !(f < h && h < q) {
		t.Error("H-004 is not between the findings and the questions")
	}
	// Ruled-out hypotheses sit under their own heading, after the open
	// explanations and before the questions, not among the things in doubt.
	if e, r, d, q := strings.Index(p.Markdown, "## Explanations still open"), strings.Index(p.Markdown, "## Ruled out\n\n- **H-"), strings.Index(p.Markdown, "(Disproven; AI generated)"), strings.Index(p.Markdown, "## Selected questions"); !(e < r && r < d && d < q) {
		t.Error("the Ruled out section is not between the open explanations and the questions")
	}
	// Every evidence line under a finding keeps its verification label; the
	// source path prints once, under Evidence and source caveats.
	for _, line := range strings.Split(p.Markdown, "\n") {
		if strings.HasPrefix(line, "  - Supported by ") || strings.HasPrefix(line, "  - Contradicted by ") {
			if !strings.HasSuffix(line, ")") || strings.Contains(line, "finance/") {
				t.Errorf("finding evidence line lacks its label or repeats its source: %q", line)
			}
		}
	}
}
