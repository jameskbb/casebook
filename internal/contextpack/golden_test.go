package contextpack_test

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/app/apptest"
)

var update = flag.Bool("update", false, "rewrite golden files")

// goldenCompactMaxBytes is small enough that the fixture's compact pack
// drops at least recent_changes and evidence_index.
const goldenCompactMaxBytes = 1200

// section returns the Markdown section that starts with header (up to the
// next blank line), or "" when absent.
func section(md, header string) string {
	i := strings.Index(md, "\n\n"+header+"\n")
	if i < 0 {
		return ""
	}
	rest := md[i+2:]
	if j := strings.Index(rest, "\n\n"); j >= 0 {
		return rest[:j]
	}
	return rest
}

func checkGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden (run go test ./internal/contextpack/ -run TestGolden -update): %v", err)
	}
	if got != string(want) {
		t.Fatalf("%s differs from the golden file\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
	}
	if bytes.Contains(want, []byte(string(rune(0x2014)))) {
		t.Fatalf("%s contains an em dash", name)
	}
}

// TestGoldenImported renders an imported investigation that also holds
// records written locally afterwards: every record copied from the file
// shows "(imported)" after its ref in Markdown and "imported": true in
// JSON, and the local ones show neither.
func TestGoldenImported(t *testing.T) {
	ctx := context.Background()
	src := apptest.Open(t)
	apptest.SeedCloudSpend(t, src)
	data, err := src.ExportInvestigation(ctx, "INV-0001")
	if err != nil {
		t.Fatal(err)
	}
	a := apptest.Open(t)
	res, err := a.ImportInvestigation(ctx, apptest.Human, data)
	if err != nil {
		t.Fatal(err)
	}
	ref := res.InvestigationRef
	ev, err := a.AddEvidence(ctx, apptest.Human, ref, app.EvidenceInput{
		Title:       "Local check: payments values file still lacks the debug override",
		Summary:     "Checked after the import.",
		SourceType:  app.SourceTypeUserStatement,
		SourceLabel: "Local review",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.AddClaim(ctx, apptest.Human, ref, app.ClaimInput{
		Statement:  "The payments values file was never changed.",
		Confidence: app.ConfidenceMedium,
		Links:      []app.EvidenceLinkInput{{EvidenceRef: ev.Ref, Relation: app.RelationSupports}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.AddQuestion(ctx, apptest.Human, ref, app.QuestionInput{Question: "Who owns the chart default now?"}); err != nil {
		t.Fatal(err)
	}

	md, err := a.RenderContext(ctx, ref, "markdown", app.ContextOptions{})
	if err != nil {
		t.Fatal(err)
	}
	checkGolden(t, "imported.md", md)
	for _, want := range []string{
		"C-001 (imported) [High", "C-002 (imported) [High", "H-001 (imported) [Medium",
		"H-002 (imported) Data transfer", "Q-001 (imported)\n", "E-001 (imported) ",
		"\nC-004 [Medium", "\nQ-002\nWho owns the chart default now?", "\nE-005 Local check",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown lacks %q:\n%s", want, md)
		}
	}
	if strings.Contains(md, "C-004 (imported)") || strings.Contains(md, "Q-002 (imported)") || strings.Contains(md, "E-005 (imported)") {
		t.Errorf("a local record is marked imported:\n%s", md)
	}

	js, err := a.RenderContext(ctx, ref, "json", app.ContextOptions{})
	if err != nil {
		t.Fatal(err)
	}
	checkGolden(t, "imported.json", js)
	p, err := a.ContextPack(ctx, ref, app.ContextOptions{})
	if err != nil {
		t.Fatal(err)
	}
	imported := map[string]bool{}
	for _, c := range p.EstablishedClaims {
		imported[c.Ref] = c.Imported
	}
	for _, h := range p.Hypotheses {
		imported[h.Ref] = h.Imported
	}
	for _, h := range p.DisprovenHypotheses {
		imported[h.Ref] = h.Imported
	}
	for _, q := range p.OpenQuestions {
		imported[q.Ref] = q.Imported
	}
	for _, e := range p.EvidenceIndex {
		imported[e.Ref] = e.Imported
	}
	for r, want := range map[string]bool{"C-001": true, "C-004": false, "H-001": true, "H-002": true,
		"Q-001": true, "Q-002": false, "E-001": true, "E-005": false} {
		if got, ok := imported[r]; !ok || got != want {
			t.Errorf("%s imported = %v (present %v), want %v", r, got, ok, want)
		}
	}
}

func TestGoldenCloudSpend(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t)
	apptest.SeedCloudSpend(t, a)

	render := func(format string, o app.ContextOptions) string {
		t.Helper()
		out, err := a.RenderContext(ctx, "", format, o)
		if err != nil {
			t.Fatalf("RenderContext(%s, %+v): %v", format, o, err)
		}
		again, err := a.RenderContext(ctx, "", format, o)
		if err != nil || again != out {
			t.Fatalf("RenderContext(%s) is not repeatable (err %v)", format, err)
		}
		return out
	}

	md := render("markdown", app.ContextOptions{})
	checkGolden(t, "cloud_spend.md", md)

	// Content checks independent of the golden file, so a wrong golden
	// cannot hide a regression.
	for _, want := range []string{
		"Investigation: INV-0001\n",
		"Title: September cloud bill up 38%\n",
		"Primary question:\nWhy did the cloud bill rise 38% in September when traffic was flat?\n",
		"Current conclusion:\nDebug logging requested for payments only reached every service and drove most of the increase.\n",
		"Conclusion confidence:\nHigh\n",
		"C-001 [High",
		"Evidence: E-001 [AI generated], E-004 [Human verified]",
		"C-002 [High",
		"Evidence: E-002 [AI generated], E-003 [AI generated]",
		"H-001 [Medium, Open",
		"Q-001",
		"Can every service except payments go back to info level today?",
		"Current claims:",
		"Known disproven hypotheses:",
		"Recent material changes:",
		"Evidence index:",
	} {
		if !strings.Contains(md, want) {
			t.Fatalf("markdown lacks %q:\n%s", want, md)
		}
	}
	if strings.Contains(md, "The increase comes from the August autoscaling change adding compute nodes.") {
		t.Fatal("markdown contains the superseded C-003 statement")
	}
	claims := section(md, "Current claims:")
	if !strings.Contains(claims, "C-001 [") || !strings.Contains(claims, "C-002 [") || strings.Contains(claims, "C-003") {
		t.Fatalf("established claims section:\n%s", claims)
	}
	hyps := section(md, "Hypotheses:")
	if !strings.Contains(hyps, "H-001 [") || strings.Contains(hyps, "H-002") {
		t.Fatalf("hypotheses section:\n%s", hyps)
	}
	disproven := section(md, "Known disproven hypotheses:")
	if disproven != "Known disproven hypotheses:\nH-002 Data transfer from the region move doubled the network line. [AI generated]\nContradicted by: E-003 [AI generated]" {
		t.Fatalf("disproven section:\n%s", disproven)
	}
	if !strings.Contains(section(md, "Open questions:"), "Q-001\nCan every service except payments") {
		t.Fatalf("open questions section:\n%s", section(md, "Open questions:"))
	}

	js := render("json", app.ContextOptions{})
	checkGolden(t, "cloud_spend.json", js)
	var doc map[string]any
	if err := json.Unmarshal([]byte(js), &doc); err != nil {
		t.Fatalf("JSON does not parse: %v", err)
	}
	if doc["format"] != "casebook.context" || doc["version"] != float64(1) {
		t.Fatalf("format/version = %v/%v", doc["format"], doc["version"])
	}
	for _, key := range []string{"established_claims", "disputed_claims", "hypotheses", "open_questions",
		"deferred_questions", "disproven_hypotheses", "evidence_index", "recent_changes", "omitted_sections"} {
		if _, ok := doc[key].([]any); !ok {
			t.Fatalf("JSON %s is %v, want an array", key, doc[key])
		}
	}
	apptest.AssertNoJSONNulls(t, json.RawMessage(js))

	compactOpts := app.ContextOptions{Compact: true, MaxBytes: goldenCompactMaxBytes}
	compact := render("markdown", compactOpts)
	checkGolden(t, "cloud_spend_compact.md", compact)
	if len(compact) > goldenCompactMaxBytes {
		t.Fatalf("compact pack is %d bytes, budget %d", len(compact), goldenCompactMaxBytes)
	}
	p, err := a.ContextPack(ctx, "", compactOpts)
	if err != nil {
		t.Fatal(err)
	}
	if !p.Truncated || !slices.Contains(p.OmittedSections, "recent_changes") || !slices.Contains(p.OmittedSections, "evidence_index") {
		t.Fatalf("compact pack omitted %v (truncated %v)", p.OmittedSections, p.Truncated)
	}
	for _, want := range []string{
		"Investigation: INV-0001\n", "Title: September cloud bill up 38%\n",
		"Why did the cloud bill rise 38% in September when traffic was flat?",
		"Debug logging requested for payments only reached every service and drove most of the increase.",
		"Trimmed to fit",
	} {
		if !strings.Contains(compact, want) {
			t.Fatalf("compact pack lacks %q:\n%s", want, compact)
		}
	}
}
