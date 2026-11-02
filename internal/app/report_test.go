package app_test

import (
	"context"
	"strings"
	"testing"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/app/apptest"
	"github.com/jameskbb/casebook/internal/contextpack"
)

func TestReportHTMLMatchesReport(t *testing.T) {
	a := apptest.Open(t)
	ctx := context.Background()
	inv, err := a.CreateInvestigation(ctx, apptest.Human, app.CreateInvestigationInput{Title: "Plain <reader>"})
	if err != nil {
		t.Fatal(err)
	}
	md, err := a.Report(ctx, inv.Ref, app.ReportOptions{})
	if err != nil {
		t.Fatal(err)
	}
	h, err := a.ReportHTML(ctx, inv.Ref, app.ReportOptions{})
	if err != nil || h != contextpack.ReportHTML(md) || !strings.Contains(h, "Plain &lt;reader&gt;") {
		t.Fatalf("ReportHTML = %q, err %v", h, err)
	}
	if _, err := a.ReportHTML(ctx, "INV-0099", app.ReportOptions{}); err == nil {
		t.Fatal("ReportHTML of a missing investigation succeeded")
	}
}
