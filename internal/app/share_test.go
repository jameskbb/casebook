package app_test

import (
	"bytes"
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/app/apptest"
)

// TestExportIsStable (ADR-0016): with a clock that moves on every call, an
// unchanged investigation exports to identical bytes, switching the active
// investigation does not change the file, and a real change moves
// exported_at to the time of that change.
func TestExportIsStable(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t, app.WithClock(apptest.StepClock(apptest.FixedTime, time.Second)))
	inv, err := a.CreateInvestigation(ctx, apptest.AI, app.CreateInvestigationInput{Title: "TestCheckoutTotals flakes in CI"})
	if err != nil {
		t.Fatal(err)
	}
	_, first := exportDoc(t, a, inv.Ref)
	if _, err := a.SetActiveInvestigation(ctx, apptest.Human, inv.Ref); err != nil {
		t.Fatal(err)
	}
	if err := a.ClearActiveInvestigation(ctx, apptest.Human); err != nil {
		t.Fatal(err)
	}
	doc, second := exportDoc(t, a, inv.Ref)
	if !bytes.Equal(first, second) {
		t.Fatalf("export changed without a change to the investigation:\n%s\n---\n%s", first, second)
	}
	if doc.ExportedAt != inv.CreatedAt {
		t.Fatalf("exported_at = %q, want the newest change %q", doc.ExportedAt, inv.CreatedAt)
	}
	ev, err := a.AddEvidence(ctx, apptest.AI, inv.Ref, app.EvidenceInput{
		Title: "23 of the last 400 CI runs fail on the totals assertion", Summary: "Same assertion every time.",
		SourceType: app.SourceTypeFile})
	if err != nil {
		t.Fatal(err)
	}
	doc, _ = exportDoc(t, a, inv.Ref)
	if doc.ExportedAt != ev.CreatedAt || doc.ExportedAt <= inv.CreatedAt {
		t.Fatalf("exported_at after a change = %q, want %q", doc.ExportedAt, ev.CreatedAt)
	}
}

// TestImportSameCaseIsACopy (ADR-0016): importing a case that is already
// in the database adds a separate copy and names the existing ones.
func TestImportSameCaseIsACopy(t *testing.T) {
	ctx := context.Background()
	src := apptest.Open(t)
	f := apptest.SeedCloudSpend(t, src)
	_, data := exportDoc(t, src, f.InvestigationRef)

	res, err := src.ImportInvestigation(ctx, apptest.Human, data)
	if err != nil {
		t.Fatal(err)
	}
	if res.InvestigationRef != "INV-0002" || strings.Join(res.SameCaseRefs, ",") != f.InvestigationRef {
		t.Fatalf("import into the source = %+v", res)
	}
	// Another investigation by another creator is not the same case.
	other, err := src.CreateInvestigation(ctx, apptest.AI, app.CreateInvestigationInput{Title: "Unrelated"})
	if err != nil {
		t.Fatal(err)
	}
	res, err = src.ImportInvestigation(ctx, apptest.Human, data)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(res.SameCaseRefs, ",") != "INV-0001,INV-0002" || slices.Contains(res.SameCaseRefs, other.Ref) {
		t.Fatalf("second import = %+v", res)
	}
	dst := apptest.Open(t)
	if res, err := dst.ImportInvestigation(ctx, apptest.Human, data); err != nil || res.SameCaseRefs != nil {
		t.Fatalf("first import elsewhere = %+v, %v", res, err)
	}
}
