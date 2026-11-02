package app_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/app/apptest"
)

const rawSentinel = "RAW-DETAIL-SENTINEL-7f3a"

func evInput(title string) app.EvidenceInput {
	return app.EvidenceInput{
		Title:       title,
		Summary:     "Query shows 214 services logging at debug level.",
		RawDetail:   "SELECT * FROM billing.monthly_costs;\n" + rawSentinel,
		SourceType:  app.SourceTypeSQLQuery,
		SourceRef:   "billing.monthly_costs",
		SourceLabel: "Billing export query",
	}
}

func mustAddEvidence(t *testing.T, a *app.App, actor app.Actor, invRef string, in app.EvidenceInput) app.Evidence {
	t.Helper()
	ev, err := a.AddEvidence(context.Background(), actor, invRef, in)
	if err != nil {
		t.Fatalf("AddEvidence: %v", err)
	}
	return ev
}

func TestAddEvidenceRefsAndVerificationDefaults(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t)
	inv, err := a.CreateInvestigation(ctx, apptest.Human, app.CreateInvestigationInput{Title: "One", MakeActive: true})
	if err != nil {
		t.Fatal(err)
	}
	other := newInv(t, a, "Two")

	e1 := mustAddEvidence(t, a, apptest.AI, "", evInput("First"))
	if e1.Ref != "E-001" || e1.InvestigationRef != inv.Ref || e1.ID == 0 {
		t.Fatalf("first evidence = %+v", e1)
	}
	if e1.Verification != app.VerificationAIGenerated || e1.CreatedBy != apptest.AI {
		t.Fatalf("AI evidence verification %q created_by %+v", e1.Verification, e1.CreatedBy)
	}
	e2 := mustAddEvidence(t, a, apptest.Human, inv.Ref, evInput("Second"))
	if e2.Ref != "E-002" || e2.Verification != app.VerificationHumanVerified {
		t.Fatalf("second evidence = %s %s", e2.Ref, e2.Verification)
	}
	in := evInput("Third")
	in.Verification = app.VerificationSourceVerified
	e3 := mustAddEvidence(t, a, apptest.AI, "inv-1", in)
	if e3.Ref != "E-003" || e3.Verification != app.VerificationSourceVerified {
		t.Fatalf("explicit verification: %s %s", e3.Ref, e3.Verification)
	}
	o1 := mustAddEvidence(t, a, apptest.AI, other.Ref, evInput("Other"))
	if o1.Ref != "E-001" || o1.InvestigationRef != other.Ref {
		t.Fatalf("second investigation evidence = %s in %s, want E-001 in %s", o1.Ref, o1.InvestigationRef, other.Ref)
	}
	if e1.RawDetail != "" || !e1.HasRawDetail {
		t.Fatalf("AddEvidence result RawDetail %q HasRawDetail %v; raw detail must be omitted", e1.RawDetail, e1.HasRawDetail)
	}
	if e1.CapturedAt != e1.CreatedAt || e1.CreatedAt != app.FormatTime(apptest.FixedTime) {
		t.Fatalf("captured_at %q created_at %q", e1.CapturedAt, e1.CreatedAt)
	}
	if e1.Title != "First" || e1.SourceType != app.SourceTypeSQLQuery || e1.SourceRef != "billing.monthly_costs" ||
		e1.SourceLabel != "Billing export query" || e1.Metadata != "" {
		t.Fatalf("fields = %+v", e1)
	}
	apptest.AssertNoJSONNulls(t, e1)
	wantActions(t, a, inv.Ref, "investigation.create", "active.set", "evidence.add", "evidence.add", "evidence.add")

	rows, err := a.ListActivity(ctx, app.ActivityFilter{InvestigationRef: inv.Ref, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if rows[0].TargetRef != "E-003" || rows[0].Summary != `Added evidence E-003 "Third"` {
		t.Fatalf("evidence.add row = %+v", rows[0])
	}
}

func TestAddEvidenceUpdatesInvestigation(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t, app.WithClock(apptest.StepClock(apptest.FixedTime, time.Second)))
	inv := newInv(t, a, "T")
	ev := mustAddEvidence(t, a, apptest.AI, inv.Ref, evInput("E"))
	got, err := a.GetInvestigation(ctx, inv.Ref)
	if err != nil {
		t.Fatal(err)
	}
	if got.UpdatedAt != ev.CreatedAt || got.UpdatedAt <= inv.UpdatedAt {
		t.Fatalf("investigation updated_at %q, evidence created_at %q, before %q", got.UpdatedAt, ev.CreatedAt, inv.UpdatedAt)
	}
}

func TestAddEvidenceValidation(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t)
	inv := newInv(t, a, "T")
	bigMeta := `{"k":"` + strings.Repeat("x", app.MaxMetadataBytes-8+1) + `"}`
	cases := map[string]struct {
		mut   func(*app.EvidenceInput)
		field string
	}{
		"missing title":    {func(in *app.EvidenceInput) { in.Title = " " }, "title"},
		"long title":       {func(in *app.EvidenceInput) { in.Title = strings.Repeat("t", 201) }, "title"},
		"missing summary":  {func(in *app.EvidenceInput) { in.Summary = "" }, "summary"},
		"missing source":   {func(in *app.EvidenceInput) { in.SourceType = "" }, "source_type"},
		"invalid source":   {func(in *app.EvidenceInput) { in.SourceType = "rumor" }, "source_type"},
		"raw detail +1":    {func(in *app.EvidenceInput) { in.RawDetail = strings.Repeat("r", 64*1024+1) }, "raw_detail"},
		"metadata array":   {func(in *app.EvidenceInput) { in.Metadata = "[1,2]" }, "metadata"},
		"metadata invalid": {func(in *app.EvidenceInput) { in.Metadata = "{bad" }, "metadata"},
		"metadata +1":      {func(in *app.EvidenceInput) { in.Metadata = bigMeta }, "metadata"},
		"captured_at":      {func(in *app.EvidenceInput) { in.CapturedAt = "yesterday" }, "captured_at"},
		"verification":     {func(in *app.EvidenceInput) { in.Verification = "maybe" }, "verification"},
		"source_ref":       {func(in *app.EvidenceInput) { in.SourceRef = strings.Repeat("s", 2001) }, "source_ref"},
		"source_label":     {func(in *app.EvidenceInput) { in.SourceLabel = strings.Repeat("s", 201) }, "source_label"},
	}
	for name, c := range cases {
		in := evInput("Valid")
		c.mut(&in)
		_, err := a.AddEvidence(ctx, apptest.AI, inv.Ref, in)
		ve, ok := app.IsValidation(err)
		if !ok || ve.Field != c.field {
			t.Errorf("%s: err = %v, want ValidationError on %s", name, err, c.field)
		}
	}
	in := evInput("Exact")
	in.RawDetail = strings.Repeat("r", 64*1024)
	if _, err := a.AddEvidence(ctx, apptest.AI, inv.Ref, in); err != nil {
		t.Fatalf("64 KiB raw detail rejected: %v", err)
	}
	if _, err := a.AddEvidence(ctx, apptest.AI, "", evInput("x")); !errors.Is(err, app.ErrNoActiveInvestigation) {
		t.Fatalf("no active: err = %v", err)
	}
	if _, err := a.AddEvidence(ctx, apptest.AI, "INV-0009", evInput("x")); !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("missing investigation: err = %v", err)
	}
	wantActions(t, a, inv.Ref, "investigation.create", "evidence.add")
}

func TestAddEvidenceMetadataAndCapturedAt(t *testing.T) {
	a := apptest.Open(t)
	inv := newInv(t, a, "T")
	in := evInput("M")
	in.Metadata = "{ \"rows\" : 22 }"
	in.CapturedAt = "2026-01-10T08:30:00+02:00"
	ev := mustAddEvidence(t, a, apptest.AI, inv.Ref, in)
	if ev.Metadata != `{"rows":22}` {
		t.Fatalf("Metadata = %q", ev.Metadata)
	}
	if ev.CapturedAt != "2026-01-10T06:30:00.000Z" {
		t.Fatalf("CapturedAt = %q", ev.CapturedAt)
	}
}

func TestGetEvidenceRawDetail(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t)
	inv := newInv(t, a, "T")
	mustAddEvidence(t, a, apptest.AI, inv.Ref, evInput("With raw"))
	noRaw := evInput("Without raw")
	noRaw.RawDetail = ""
	mustAddEvidence(t, a, apptest.AI, inv.Ref, noRaw)

	got, err := a.GetEvidence(ctx, inv.Ref, "e-1", app.GetEvidenceOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got.RawDetail != "" || !got.HasRawDetail || got.Ref != "E-001" {
		t.Fatalf("without IncludeRawDetail: RawDetail %q HasRawDetail %v", got.RawDetail, got.HasRawDetail)
	}
	got, err = a.GetEvidence(ctx, inv.Ref, "E-001", app.GetEvidenceOptions{IncludeRawDetail: true})
	if err != nil {
		t.Fatal(err)
	}
	if got.RawDetail != "SELECT * FROM billing.monthly_costs;\n"+rawSentinel {
		t.Fatalf("with IncludeRawDetail: RawDetail %q", got.RawDetail)
	}
	got, err = a.GetEvidence(ctx, inv.Ref, "E-002", app.GetEvidenceOptions{IncludeRawDetail: true})
	if err != nil {
		t.Fatal(err)
	}
	if got.HasRawDetail || got.RawDetail != "" {
		t.Fatalf("no raw stored: %+v", got)
	}
	if got.ClaimLinks == nil || got.HypothesisLinks == nil {
		t.Fatal("links must be non-nil")
	}
	apptest.AssertNoJSONNulls(t, got)
}

func TestGetEvidenceRefResolution(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t)
	newInv(t, a, "One")
	two := newInv(t, a, "Two")
	mustAddEvidence(t, a, apptest.AI, two.Ref, evInput("In two"))

	got, err := a.GetEvidence(ctx, "", "INV-0002/E-001", app.GetEvidenceOptions{})
	if err != nil || got.InvestigationRef != "INV-0002" || got.Title != "In two" {
		t.Fatalf("qualified ref with empty invRef: %+v, %v", got, err)
	}
	if _, err := a.GetEvidence(ctx, "INV-0002", "inv-2/e-1", app.GetEvidenceOptions{}); err != nil {
		t.Fatalf("qualified ref matching invRef: %v", err)
	}
	_, err = a.GetEvidence(ctx, "INV-0001", "INV-0002/E-001", app.GetEvidenceOptions{})
	wantValidation(t, err, "ref")
	_, err = a.GetEvidence(ctx, "INV-0002", "C-001", app.GetEvidenceOptions{})
	wantValidation(t, err, "ref")
	_, err = a.GetEvidence(ctx, "INV-0001", "E-001", app.GetEvidenceOptions{})
	if !errors.Is(err, app.ErrNotFound) || !strings.Contains(err.Error(), "evidence E-001 in INV-0001") {
		t.Fatalf("missing evidence: err = %v", err)
	}
	if _, err := a.GetEvidence(ctx, "", "E-001", app.GetEvidenceOptions{}); !errors.Is(err, app.ErrNoActiveInvestigation) {
		t.Fatalf("unqualified ref, no active: err = %v", err)
	}
}

func TestListEvidenceOrderAndOmission(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t)
	inv := newInv(t, a, "T")
	empty, err := a.ListEvidence(ctx, inv.Ref)
	if err != nil {
		t.Fatal(err)
	}
	if empty == nil || len(empty) != 0 {
		t.Fatalf("empty list = %#v, want non-nil empty", empty)
	}
	apptest.AssertNoJSONNulls(t, empty)
	for _, title := range []string{"A", "B", "C"} {
		mustAddEvidence(t, a, apptest.AI, inv.Ref, evInput(title))
	}
	list, err := a.ListEvidence(ctx, inv.Ref)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 3 {
		t.Fatalf("len = %d", len(list))
	}
	for i, ev := range list {
		want := app.FormatRecordRef(app.RecordEvidence, int64(i+1))
		if ev.Ref != want || ev.RawDetail != "" || !ev.HasRawDetail {
			t.Fatalf("list[%d] = %s raw %q has %v", i, ev.Ref, ev.RawDetail, ev.HasRawDetail)
		}
		if ev.ClaimLinks == nil || ev.HypothesisLinks == nil {
			t.Fatalf("list[%d] has nil links", i)
		}
	}
	apptest.AssertNoJSONNulls(t, list)
}

func TestUpdateEvidencePatch(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t, app.WithClock(apptest.StepClock(apptest.FixedTime, time.Second)))
	inv := newInv(t, a, "T")
	ev := mustAddEvidence(t, a, apptest.AI, inv.Ref, evInput("Original"))

	got, err := a.UpdateEvidence(ctx, apptest.Human, inv.Ref, "E-001", app.EvidencePatch{
		Title:    ptr("Renamed"),
		Metadata: ptr(`{"b": 2}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Renamed" || got.Metadata != `{"b":2}` || got.Summary != ev.Summary ||
		got.SourceRef != ev.SourceRef || got.Verification != ev.Verification || !got.HasRawDetail {
		t.Fatalf("after patch: %+v", got)
	}
	if got.UpdatedAt <= ev.UpdatedAt || got.CreatedAt != ev.CreatedAt || got.CreatedBy != apptest.AI {
		t.Fatalf("timestamps/provenance after patch: %+v", got)
	}
	// Same values again: no activity.
	if _, err := a.UpdateEvidence(ctx, apptest.Human, inv.Ref, "E-001", app.EvidencePatch{Title: ptr("Renamed")}); err != nil {
		t.Fatal(err)
	}
	// Clearing raw detail.
	got, err = a.UpdateEvidence(ctx, apptest.Human, inv.Ref, "E-001", app.EvidencePatch{RawDetail: ptr("")})
	if err != nil {
		t.Fatal(err)
	}
	if got.HasRawDetail {
		t.Fatal("raw detail not cleared")
	}
	wantActions(t, a, inv.Ref, "investigation.create", "evidence.add", "evidence.update", "evidence.update")

	rows, err := a.ListActivity(ctx, app.ActivityFilter{InvestigationRef: inv.Ref, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if s := rows[1].Summary; !strings.Contains(s, "title") || !strings.Contains(s, "metadata") || strings.Contains(s, "Renamed") {
		t.Fatalf("update summary = %q, want changed field names only", s)
	}
	if rows[1].TargetRef != "E-001" || rows[1].Actor != apptest.Human {
		t.Fatalf("update row = %+v", rows[1])
	}
}

func TestUpdateEvidenceErrors(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t)
	inv := newInv(t, a, "T")
	mustAddEvidence(t, a, apptest.AI, inv.Ref, evInput("E"))

	_, err := a.UpdateEvidence(ctx, apptest.Human, inv.Ref, "C-001", app.EvidencePatch{Title: ptr("x")})
	wantValidation(t, err, "ref")
	_, err = a.UpdateEvidence(ctx, apptest.Human, inv.Ref, "E-099", app.EvidencePatch{Title: ptr("x")})
	if !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("E-099: err = %v, want ErrNotFound", err)
	}
	_, err = a.UpdateEvidence(ctx, apptest.Human, inv.Ref, "E-001", app.EvidencePatch{Summary: ptr("")})
	wantValidation(t, err, "summary")
	_, err = a.UpdateEvidence(ctx, apptest.Human, inv.Ref, "E-001", app.EvidencePatch{SourceType: ptr(app.SourceType("gossip"))})
	wantValidation(t, err, "source_type")
	_, err = a.UpdateEvidence(ctx, apptest.Human, inv.Ref, "E-001", app.EvidencePatch{CapturedAt: ptr("soon")})
	wantValidation(t, err, "captured_at")
	_, err = a.UpdateEvidence(ctx, apptest.Human, inv.Ref, "E-001", app.EvidencePatch{Metadata: ptr("[]")})
	wantValidation(t, err, "metadata")
	wantActions(t, a, inv.Ref, "investigation.create", "evidence.add")
}

func TestUpdateEvidenceCapturedAtReset(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t)
	inv := newInv(t, a, "T")
	in := evInput("E")
	in.CapturedAt = "2025-12-01T00:00:00Z"
	ev := mustAddEvidence(t, a, apptest.AI, inv.Ref, in)
	if ev.CapturedAt != "2025-12-01T00:00:00.000Z" {
		t.Fatalf("CapturedAt = %q", ev.CapturedAt)
	}
	got, err := a.UpdateEvidence(ctx, apptest.AI, inv.Ref, "E-001", app.EvidencePatch{CapturedAt: ptr("")})
	if err != nil {
		t.Fatal(err)
	}
	if got.CapturedAt != got.CreatedAt {
		t.Fatalf("empty captured_at patch: %q, want created_at %q", got.CapturedAt, got.CreatedAt)
	}
}

func TestSetEvidenceVerification(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t)
	inv := newInv(t, a, "T")
	mustAddEvidence(t, a, apptest.AI, inv.Ref, evInput("E"))
	got, err := a.SetEvidenceVerification(ctx, apptest.Human, inv.Ref, "E-001", app.VerificationDisputed)
	if err != nil {
		t.Fatal(err)
	}
	if got.Verification != app.VerificationDisputed {
		t.Fatalf("Verification = %q", got.Verification)
	}
	if _, err := a.SetEvidenceVerification(ctx, apptest.Human, inv.Ref, "E-001", app.VerificationDisputed); err != nil {
		t.Fatal(err)
	}
	_, err = a.SetEvidenceVerification(ctx, apptest.Human, inv.Ref, "E-001", "trusted")
	wantValidation(t, err, "verification")
	wantActions(t, a, inv.Ref, "investigation.create", "evidence.add", "evidence.update")
	rows, err := a.ListActivity(ctx, app.ActivityFilter{InvestigationRef: inv.Ref, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if rows[0].Summary != "Set verification of E-001 to disputed" {
		t.Fatalf("summary = %q", rows[0].Summary)
	}
}

func TestEvidenceActivityNeverContainsRawDetail(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t)
	inv := newInv(t, a, "T")
	in := evInput("E")
	in.Metadata = `{"secret":"` + rawSentinel + `"}`
	mustAddEvidence(t, a, apptest.AI, inv.Ref, in)
	if _, err := a.UpdateEvidence(ctx, apptest.AI, inv.Ref, "E-001", app.EvidencePatch{
		RawDetail: ptr("changed " + rawSentinel),
		Summary:   ptr("summary " + strings.Repeat("s", 500)),
	}); err != nil {
		t.Fatal(err)
	}
	rows, err := a.ListActivity(ctx, app.ActivityFilter{Limit: 500})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if strings.Contains(r.Summary, rawSentinel) || strings.Contains(r.Note, rawSentinel) {
			t.Fatalf("activity row leaks raw detail or metadata: %+v", r)
		}
		if strings.Contains(r.Summary, "ssssssssss") {
			t.Fatalf("activity row contains long text: %+v", r)
		}
	}
}

// TestUpdateEvidenceSummaryNamesValues checks that an evidence update names
// the new value of enum fields and only the name of free-text fields.
func TestUpdateEvidenceSummaryNamesValues(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t, app.WithClock(apptest.StepClock(apptest.FixedTime, time.Second)))
	inv := newInv(t, a, "T")
	mustAddEvidence(t, a, apptest.AI, inv.Ref, evInput("Original"))
	st := app.SourceTypeFile
	v := app.VerificationSourceVerified
	if _, err := a.UpdateEvidence(ctx, apptest.Human, inv.Ref, "E-001", app.EvidencePatch{
		Title: ptr("Renamed"), SourceType: &st, Verification: &v,
	}); err != nil {
		t.Fatal(err)
	}
	wantLast(t, a, inv.Ref, "evidence.update", "E-001",
		"Updated evidence E-001: title, source_type file, verification source_verified")
}
