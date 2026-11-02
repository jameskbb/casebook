package app_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/app/apptest"
)

func TestClearActiveInvestigationIf(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t)
	first := newInv(t, a, "First")
	second := newInv(t, a, "Second")
	if _, err := a.SetActiveInvestigation(ctx, apptest.Human, second.Ref); err != nil {
		t.Fatal(err)
	}
	before, err := a.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	cleared, err := a.ClearActiveInvestigationIf(ctx, apptest.Human, first.Ref)
	if err != nil || cleared {
		t.Fatalf("clear first while second active = %v, %v; want false, nil", cleared, err)
	}
	if got, _ := a.GetActiveInvestigation(ctx); got == nil || got.Ref != second.Ref {
		t.Fatalf("active changed to %v", got)
	}
	after, _ := a.Status(ctx)
	if after.Counts.Activity != before.Counts.Activity {
		t.Fatalf("a no-op clear wrote activity: %d -> %d", before.Counts.Activity, after.Counts.Activity)
	}
	cleared, err = a.ClearActiveInvestigationIf(ctx, apptest.Human, second.Ref)
	if err != nil || !cleared {
		t.Fatalf("clear second = %v, %v; want true, nil", cleared, err)
	}
	if got, _ := a.GetActiveInvestigation(ctx); got != nil {
		t.Fatalf("still active: %v", got)
	}
	if cleared, err := a.ClearActiveInvestigationIf(ctx, apptest.Human, second.Ref); err != nil || cleared {
		t.Fatalf("clear with none active = %v, %v", cleared, err)
	}
	if _, err := a.ClearActiveInvestigationIf(ctx, apptest.Human, "INV-0099"); !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("unknown ref err = %v", err)
	}
	if _, err := a.ClearActiveInvestigationIf(ctx, apptest.Human, " "); !isValidation(err) {
		t.Fatalf("blank ref err = %v", err)
	}
}

// endless is a reader that never ends.
type endless struct{ n int64 }

func (e *endless) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = ' '
	}
	e.n += int64(len(p))
	return len(p), nil
}

func TestImportInvestigationFromLimitsReading(t *testing.T) {
	ctx := context.Background()
	src := apptest.Open(t)
	apptest.SeedCloudSpend(t, src)
	data, err := src.ExportInvestigation(ctx, "INV-0001")
	if err != nil {
		t.Fatal(err)
	}
	dst := apptest.Open(t)
	res, err := dst.ImportInvestigationFrom(ctx, apptest.Human, bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if res.Title != "September cloud bill up 38%" || res.InvestigationRef != "INV-0001" {
		t.Fatalf("result %+v", res)
	}

	r := &endless{}
	_, err = dst.ImportInvestigationFrom(ctx, apptest.Human, r)
	if !isValidation(err) || !strings.Contains(err.Error(), "20 MiB") {
		t.Fatalf("endless input err = %v", err)
	}
	if r.n > int64(app.MaxImportBytes)+64*1024 {
		t.Fatalf("read %d bytes, more than the limit allows", r.n)
	}

	_, err = dst.ImportInvestigationFrom(ctx, apptest.Human, io.MultiReader(bytes.NewReader([]byte("{")), errReader{}))
	if err == nil || !strings.Contains(err.Error(), "read import") {
		t.Fatalf("read error = %v", err)
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("disk gone") }

func TestSearchSegmentsNeverCollideWithStoredText(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t)
	inv, err := a.CreateInvestigation(ctx, apptest.Human, app.CreateInvestigationInput{
		Title:            "Literal **stars** and \x02control\x03 marks",
		ProblemStatement: "Why does the widget fail?",
	})
	if err != nil {
		t.Fatal(err)
	}
	hits, err := a.Search(ctx, "widget", app.SearchOptions{InvestigationRef: inv.Ref})
	if err != nil || len(hits) == 0 {
		t.Fatalf("search = %v, %v", hits, err)
	}
	h := hits[0]
	var matched []string
	var joined strings.Builder
	for _, s := range h.Segments {
		if s.Match {
			matched = append(matched, s.Text)
		}
		joined.WriteString(s.Text)
	}
	if len(matched) != 1 || matched[0] != "widget" {
		t.Fatalf("matched segments %q (all %+v)", matched, h.Segments)
	}
	if !strings.Contains(h.Snippet, "**widget**") {
		t.Fatalf("snippet %q", h.Snippet)
	}

	hits, err = a.Search(ctx, "stars", app.SearchOptions{InvestigationRef: inv.Ref})
	if err != nil || len(hits) == 0 {
		t.Fatalf("search stars = %v, %v", hits, err)
	}
	joined.Reset()
	matched = nil
	for _, s := range hits[0].Segments {
		if s.Match {
			matched = append(matched, s.Text)
		}
		joined.WriteString(s.Text)
	}
	if len(matched) != 1 || matched[0] != "stars" {
		t.Fatalf("matched %q in %+v", matched, hits[0].Segments)
	}
	if !strings.Contains(joined.String(), "**") {
		t.Fatalf("literal ** lost from segments: %q", joined.String())
	}
	if strings.ContainsAny(joined.String(), "\x02\x03") {
		t.Fatalf("control markers leaked: %q", joined.String())
	}
}

func isValidation(err error) bool {
	_, ok := app.IsValidation(err)
	return ok
}
