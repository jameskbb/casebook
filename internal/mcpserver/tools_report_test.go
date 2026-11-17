package mcpserver

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/app/apptest"
)

func TestGetReport(t *testing.T) {
	a, f := seeded(t)
	cs := newTestSession(t, a, sessionOpts{})
	assertToolMeta(t, cs, "casebook_get_report")
	ctx := context.Background()

	want, err := a.Report(ctx, f.InvestigationRef, app.ReportOptions{})
	if err != nil {
		t.Fatal(err)
	}
	res := callTool(t, cs, "casebook_get_report", nil)
	out := decode[reportOutput](t, res)
	if resultText(res) != app.FrameContext(want) || out.Report != app.FrameContext(want) ||
		out.InvestigationRef != "INV-0001" || out.Mode != "report" || out.TimelineLimit != 0 || out.Bytes != len(out.Report) {
		t.Errorf("out = %+v", out)
	}
	out = decode[reportOutput](t, callTool(t, cs, "casebook_get_report", map[string]any{"mode": "report"}))
	if out.Report != app.FrameContext(want) || out.Mode != "report" {
		t.Errorf("mode report: out = %+v", out)
	}

	// mode brief: the desktop Share summary's default brief, byte for byte,
	// omission notice included.
	brief, err := a.SharePreview(ctx, f.InvestigationRef, app.ShareOptions{Mode: "brief"})
	if err != nil {
		t.Fatal(err)
	}
	res = callTool(t, cs, "casebook_get_report", map[string]any{"mode": "brief"})
	out = decode[reportOutput](t, res)
	if resultText(res) != app.FrameContext(brief.Markdown) || out.Report != app.FrameContext(brief.Markdown) ||
		out.Mode != "brief" || out.TimelineLimit != 0 || out.Truncated || out.Bytes != len(out.Report) ||
		!strings.Contains(out.Report, "Leadership brief.") || !strings.Contains(out.Report, "## Scope and omissions") {
		t.Errorf("mode brief: out = %+v", out)
	}
	wantToolError(t, callTool(t, cs, "casebook_get_report", map[string]any{"mode": "brief", "timeline_limit": 5}), "invalid timeline_limit")
	if res := callTool(t, cs, "casebook_get_report", map[string]any{"mode": "summary"}); !res.IsError {
		t.Error("unknown mode accepted")
	}

	short, err := a.Report(ctx, f.InvestigationRef, app.ReportOptions{TimelineLimit: 2})
	if err != nil {
		t.Fatal(err)
	}
	out = decode[reportOutput](t, callTool(t, cs, "casebook_get_report",
		map[string]any{"investigation_ref": "inv-1", "timeline_limit": 2}))
	if out.Report != app.FrameContext(short) || out.TimelineLimit != 2 {
		t.Errorf("timeline_limit 2: out = %+v", out)
	}

	wantToolError(t, callTool(t, cs, "casebook_get_report", map[string]any{"timeline_limit": -1}), "invalid timeline_limit")
	wantToolError(t, callTool(t, cs, "casebook_get_report", map[string]any{"investigation_ref": "INV-0099"}), "not found:")
}

// TestGetReportShrinksTimeline proves a report whose timeline does not fit
// the response budget comes back with a shorter timeline.
func TestGetReportShrinksTimeline(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t)
	inv, err := a.CreateInvestigation(ctx, apptest.Human, app.CreateInvestigationInput{Title: "Busy", MakeActive: true})
	if err != nil {
		t.Fatal(err)
	}
	q, err := a.AddQuestion(ctx, apptest.Human, inv.Ref, app.QuestionInput{Question: "Start?"})
	if err != nil {
		t.Fatal(err)
	}
	long := make([]byte, 0, 250)
	for len(long) < 250 {
		long = append(long, "Edited question text "...)
	}
	for i := range 400 {
		if _, err := a.UpdateQuestion(ctx, apptest.Human, inv.Ref, q.Ref, fmt.Sprintf("%d %s", i, long)); err != nil {
			t.Fatal(err)
		}
	}
	cs := newTestSession(t, a, sessionOpts{})
	res := callTool(t, cs, "casebook_get_report", nil)
	out := decode[reportOutput](t, res)
	if out.TimelineLimit == 0 || resultBytes(resultText(res), out) > responseTarget {
		t.Fatalf("timeline not shortened: limit %d, %d bytes", out.TimelineLimit, out.Bytes)
	}
}

// TestCutBriefKeepsOmissionNotice proves a brief cut to fit the response
// keeps its whole "Scope and omissions" section after the cut body, with a
// note worded for the brief.
func TestCutBriefKeepsOmissionNotice(t *testing.T) {
	a, f := seeded(t)
	p, err := a.SharePreview(context.Background(), f.InvestigationRef, app.ShareOptions{Mode: "brief"})
	if err != nil {
		t.Fatal(err)
	}
	i := strings.LastIndex(p.Markdown, briefScopeHeading)
	if i < 2*minContextMaxBytes {
		t.Fatalf("fixture brief too small to cut (%d bytes before the scope section):\n%s", i, p.Markdown)
	}
	scope := p.Markdown[i:]
	full, text, _ := fitReport(f.InvestigationRef, reportModeBrief, 0, p.Markdown, false, responseTarget)
	budget := resultBytes(text, full) - 300
	out, text, ok := cutReport(f.InvestigationRef, reportModeBrief, 0, p.Markdown, budget)
	if !ok {
		t.Fatal("no cut fits")
	}
	if !out.Truncated || out.Mode != "brief" || out.Report != text || resultBytes(text, out) > budget {
		t.Fatalf("out = %+v", out)
	}
	if !strings.Contains(text, "(Brief cut to fit the response; the Scope and omissions section below describes the full brief. Run casebook report --brief in a terminal for the full text.)") ||
		strings.Contains(text, "Report cut") {
		t.Errorf("cut note missing or worded for a report:\n%s", text)
	}
	if !strings.Contains(text, scope) || !strings.Contains(text, "Not included: ") {
		t.Errorf("cut brief lost its omission notice:\n%s", text)
	}
	if strings.Index(text, "(Brief cut") > strings.Index(text, briefScopeHeading) {
		t.Error("cut note comes after the scope section")
	}

	// A report is cut from its end, with the report's note.
	out, text, ok = cutReport(f.InvestigationRef, reportModeReport, 0, p.Markdown, budget)
	if !ok || !out.Truncated || !strings.Contains(text, "(Report cut to fit the response; run casebook report in a terminal") {
		t.Errorf("report cut = %+v", out)
	}
}
