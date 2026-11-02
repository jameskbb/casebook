package app_test

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/app/apptest"
)

// Sentinels planted in sensitive fields; none may reach a log line.
const (
	sentinelRaw  = "SENTINEL-RAW-7f3a"
	sentinelMeta = "SENTINEL-META-7f3a"
	sentinelStmt = "SENTINEL-STMT-7f3a"
)

// TestLoggingNoSensitiveDataOrStdout drives seeding, every record type and
// the validation and not-found error paths with a Debug logger, then checks
// the log holds no raw detail, metadata or statement text and that nothing
// was written to stdout (D-28, APP-06). It swaps os.Stdout, so it is not
// parallel.
func TestLoggingNoSensitiveDataOrStdout(t *testing.T) {
	ctx := context.Background()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	origStdout := os.Stdout
	os.Stdout = w
	restored := false
	restore := func() {
		if !restored {
			os.Stdout = origStdout
			w.Close()
			restored = true
		}
	}
	defer restore()
	stdout := make(chan []byte, 1)
	go func() {
		b, _ := io.ReadAll(r)
		stdout <- b
	}()

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	a := apptest.Open(t, app.WithLogger(logger))

	f := apptest.SeedCloudSpend(t, a)
	ev, err := a.AddEvidence(ctx, apptest.AI, "", app.EvidenceInput{
		Title:      "Sensitive capture",
		Summary:    "Summary that is fine to index.",
		RawDetail:  "SELECT secret FROM t; -- " + sentinelRaw,
		SourceType: app.SourceTypeSQLQuery,
		Metadata:   `{"note":"` + sentinelMeta + `"}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.GetEvidence(ctx, "", ev.Ref, app.GetEvidenceOptions{IncludeRawDetail: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.UpdateEvidence(ctx, apptest.AI, "", ev.Ref, app.EvidencePatch{
		RawDetail: ptr("updated " + sentinelRaw), Metadata: ptr(`{"note":"updated ` + sentinelMeta + `"}`),
	}); err != nil {
		t.Fatal(err)
	}
	c, err := a.AddClaim(ctx, apptest.AI, "", app.ClaimInput{
		Statement: "Claim " + sentinelStmt, ConfidenceRationale: "Rationale " + sentinelStmt,
		Links: []app.EvidenceLinkInput{{EvidenceRef: ev.Ref, Relation: app.RelationSupports}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.UpdateClaim(ctx, apptest.Human, "", c.Ref, app.ClaimPatch{Statement: ptr("Edited " + sentinelStmt)}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.AddHypothesis(ctx, apptest.AI, "", app.HypothesisInput{
		Statement: "Hypothesis " + sentinelStmt, Rationale: "Why " + sentinelStmt}); err != nil {
		t.Fatal(err)
	}
	q, err := a.AddQuestion(ctx, apptest.AI, "", app.QuestionInput{Question: "Question " + sentinelStmt + "?"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.ResolveQuestion(ctx, apptest.Human, "", q.Ref, "Resolution "+sentinelStmt); err != nil {
		t.Fatal(err)
	}
	if _, err := a.UpdateConclusion(ctx, apptest.Human, "", app.ConclusionInput{
		Conclusion: "Conclusion " + sentinelStmt, Confidence: app.ConfidenceHigh}); err != nil {
		t.Fatal(err)
	}

	// Error paths carrying sentinel text.
	_, err = a.AddClaim(ctx, apptest.AI, "", app.ClaimInput{
		Statement: sentinelStmt + strings.Repeat("x", app.MaxStatementBytes)})
	wantValidation(t, err, "statement")
	_, err = a.AddClaim(ctx, apptest.AI, "", app.ClaimInput{Statement: "Missing link " + sentinelStmt,
		Links: []app.EvidenceLinkInput{{EvidenceRef: "E-099", Relation: app.RelationSupports}}})
	wantErrIs(t, err, app.ErrNotFound)
	_, err = a.AddEvidence(ctx, apptest.AI, "", app.EvidenceInput{Title: "Bad metadata", Summary: "s",
		SourceType: app.SourceTypeFile, RawDetail: sentinelRaw, Metadata: `["` + sentinelMeta + `"]`})
	wantValidation(t, err, "metadata")
	_, err = a.GetClaim(ctx, f.InvestigationRef, "C-099")
	wantErrIs(t, err, app.ErrNotFound)
	_, err = a.UpdateHypothesis(ctx, apptest.AI, "", "H-099", app.HypothesisPatch{Statement: ptr(sentinelStmt)})
	wantErrIs(t, err, app.ErrNotFound)

	restore()
	out := <-stdout

	logs := buf.String()
	if !strings.Contains(logs, "claim.add") || !strings.Contains(logs, "ref=C-004") {
		t.Fatalf("expected Debug lines naming ops and refs, got:\n%s", logs)
	}
	for _, s := range []string{sentinelRaw, sentinelMeta, sentinelStmt} {
		if strings.Contains(logs, s) {
			t.Errorf("log contains sentinel %s:\n%s", s, logs)
		}
	}
	if len(out) != 0 {
		t.Errorf("stdout received %d bytes: %q", len(out), out)
	}
}
