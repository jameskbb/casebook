package app_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/app/apptest"
)

func TestContextPackFixture(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t)
	f := apptest.SeedCloudSpend(t, a)

	p, err := a.ContextPack(ctx, "", app.ContextOptions{})
	if err != nil {
		t.Fatalf("ContextPack: %v", err)
	}
	if p.Format != "casebook.context" || p.Version != 1 {
		t.Fatalf("format/version = %q/%d", p.Format, p.Version)
	}
	if p.Investigation.Ref != f.InvestigationRef || p.Investigation.Title != "September cloud bill up 38%" ||
		p.Investigation.PrimaryQuestion != "Why did the cloud bill rise 38% in September when traffic was flat?" {
		t.Fatalf("investigation = %+v", p.Investigation)
	}
	if p.Conclusion.Text != "Debug logging requested for payments only reached every service and drove most of the increase." || p.Conclusion.Confidence != app.ConfidenceHigh {
		t.Fatalf("conclusion = %+v", p.Conclusion)
	}
	refs := func(n int, ref func(int) string) string {
		out := make([]string, 0, n)
		for i := 0; i < n; i++ {
			out = append(out, ref(i))
		}
		return strings.Join(out, ",")
	}
	if got := refs(len(p.EstablishedClaims), func(i int) string { return p.EstablishedClaims[i].Ref }); got != "C-001,C-002" {
		t.Fatalf("established = %s", got)
	}
	if got := strings.Join(p.EstablishedClaims[0].SupportingEvidence, ","); got != "E-001,E-004" {
		t.Fatalf("C-001 supporting = %s", got)
	}
	if got := strings.Join(p.EstablishedClaims[1].SupportingEvidence, ","); got != "E-002,E-003" {
		t.Fatalf("C-002 supporting = %s", got)
	}
	if len(p.DisputedClaims) != 0 {
		t.Fatalf("disputed = %+v", p.DisputedClaims)
	}
	if got := refs(len(p.Hypotheses), func(i int) string { return p.Hypotheses[i].Ref }); got != "H-001" {
		t.Fatalf("hypotheses = %s", got)
	}
	if len(p.DisprovenHypotheses) != 1 || p.DisprovenHypotheses[0].Ref != f.DisprovenHypothesisRef {
		t.Fatalf("disproven = %+v", p.DisprovenHypotheses)
	}
	if len(p.OpenQuestions) != 1 || p.OpenQuestions[0].Ref != "Q-001" {
		t.Fatalf("open questions = %+v", p.OpenQuestions)
	}
	if got := refs(len(p.EvidenceIndex), func(i int) string { return p.EvidenceIndex[i].Ref }); got != "E-001,E-002,E-003,E-004" {
		t.Fatalf("evidence index = %s", got)
	}
	if len(p.RecentChanges) != 10 {
		t.Fatalf("recent changes = %d, want 10", len(p.RecentChanges))
	}
	for _, c := range p.RecentChanges {
		if c.Action == "active.set" || c.Action == "checkpoint" {
			t.Fatalf("non-material change %s in pack", c.Action)
		}
	}

	// Raw evidence detail never reaches the pack (T-01-24).
	js, err := a.RenderContext(ctx, "", "json", app.ContextOptions{})
	if err != nil {
		t.Fatal(err)
	}
	md, err := a.RenderContext(ctx, "", "markdown", app.ContextOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, out := range []string{js, md} {
		if strings.Contains(out, "+  level: debug") || strings.Contains(out, "platform-charts/service/values.yaml") {
			t.Fatal("pack contains raw detail or source refs")
		}
	}

	// Explicit refs and RecentChanges work.
	p3, err := a.ContextPack(ctx, "inv-1", app.ContextOptions{RecentChanges: 3})
	if err != nil {
		t.Fatal(err)
	}
	if len(p3.RecentChanges) != 3 || p3.RecentChanges[0] != p.RecentChanges[0] {
		t.Fatalf("RecentChanges 3 = %+v", p3.RecentChanges)
	}
}

func TestContextPackValidation(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t)
	_, err := a.ContextPack(ctx, "", app.ContextOptions{})
	wantErrIs(t, err, app.ErrNoActiveInvestigation)

	apptest.SeedCloudSpend(t, a)
	_, err = a.ContextPack(ctx, "", app.ContextOptions{MaxBytes: -1})
	wantValidation(t, err, "max_bytes")
	_, err = a.ContextPack(ctx, "", app.ContextOptions{RecentChanges: -1})
	wantValidation(t, err, "recent_changes")
	_, err = a.RenderContext(ctx, "", "json", app.ContextOptions{MaxBytes: -5})
	wantValidation(t, err, "max_bytes")
	_, err = a.ContextPack(ctx, "INV-0099", app.ContextOptions{})
	wantErrIs(t, err, app.ErrNotFound)
}

func TestRenderContextFormats(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t)
	apptest.SeedCloudSpend(t, a)
	other := newInv(t, a, "Second investigation")

	md, err := a.RenderContext(ctx, "", "markdown", app.ContextOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, format := range []string{"md", "MD", " Markdown ", "MARKDOWN"} {
		got, err := a.RenderContext(ctx, "", format, app.ContextOptions{})
		if err != nil {
			t.Fatalf("format %q: %v", format, err)
		}
		if got != md {
			t.Fatalf("format %q differs from markdown", format)
		}
	}
	again, err := a.RenderContext(ctx, "", "markdown", app.ContextOptions{})
	if err != nil || again != md {
		t.Fatalf("second render differs (err %v)", err)
	}
	if !strings.HasPrefix(md, "Investigation: INV-0001\nTitle: September cloud bill up 38%\nStatus: Investigating\n") {
		t.Fatalf("markdown header:\n%s", md)
	}

	js, err := a.RenderContext(ctx, "", "JSON", app.ContextOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(js), &doc); err != nil {
		t.Fatalf("JSON does not parse: %v", err)
	}
	if doc["format"] != "casebook.context" || doc["version"] != float64(1) {
		t.Fatalf("format/version = %v/%v", doc["format"], doc["version"])
	}
	apptest.AssertNoJSONNulls(t, json.RawMessage(js))
	var p app.Pack
	if err := json.Unmarshal([]byte(js), &p); err != nil {
		t.Fatal(err)
	}
	want, err := a.ContextPack(ctx, "", app.ContextOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if p.Investigation != want.Investigation || len(p.EstablishedClaims) != len(want.EstablishedClaims) {
		t.Fatal("JSON rendering does not match ContextPack")
	}

	for _, bad := range []string{"xml", "", "html", "markdown2"} {
		_, err := a.RenderContext(ctx, "", bad, app.ContextOptions{})
		wantValidation(t, err, "format")
	}

	// The fixture fits the default compact budget untouched.
	compact, err := a.RenderContext(ctx, "", "markdown", app.ContextOptions{Compact: true})
	if err != nil {
		t.Fatal(err)
	}
	if compact != md || len(compact) > app.DefaultCompactMaxBytes {
		t.Fatalf("compact differs from full (%d bytes)", len(compact))
	}

	// An empty investigation renders placeholders.
	empty, err := a.RenderContext(ctx, other.Ref, "md", app.ContextOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(empty, "Current claims:\n(none)") || !strings.Contains(empty, "Current conclusion:\n(not yet set)") {
		t.Fatalf("empty pack:\n%s", empty)
	}
	emptyJSON, err := a.RenderContext(ctx, other.Ref, "json", app.ContextOptions{})
	if err != nil {
		t.Fatal(err)
	}
	apptest.AssertNoJSONNulls(t, json.RawMessage(emptyJSON))
}
