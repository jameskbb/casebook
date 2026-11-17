package app_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/app/apptest"
)

func refsOf[T any](items []T, ref func(T) string) string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, ref(it))
	}
	return strings.Join(out, ",")
}

func TestGetInvestigationDetail(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t)
	f := apptest.SeedCloudSpend(t, a)
	other := newInv(t, a, "Unrelated investigation")

	d, err := a.GetInvestigationDetail(ctx, "", app.DetailOptions{})
	if err != nil {
		t.Fatalf("GetInvestigationDetail(active): %v", err)
	}
	if d.Investigation.Ref != f.InvestigationRef || !d.Investigation.IsActive {
		t.Fatalf("investigation = %s active=%v, want %s active", d.Investigation.Ref, d.Investigation.IsActive, f.InvestigationRef)
	}
	if d.Investigation.Conclusion != "Debug logging requested for payments only reached every service and drove most of the increase." {
		t.Fatalf("conclusion = %q", d.Investigation.Conclusion)
	}
	if got := refsOf(d.Evidence, func(e app.Evidence) string { return e.Ref }); got != strings.Join(f.Evidence, ",") {
		t.Fatalf("evidence refs = %s, want %v", got, f.Evidence)
	}
	if got := refsOf(d.Claims, func(c app.Claim) string { return c.Ref }); got != strings.Join(f.Claims, ",") {
		t.Fatalf("claim refs = %s, want %v", got, f.Claims)
	}
	if got := refsOf(d.Hypotheses, func(h app.Hypothesis) string { return h.Ref }); got != strings.Join(f.Hypotheses, ",") {
		t.Fatalf("hypothesis refs = %s, want %v", got, f.Hypotheses)
	}
	if got := refsOf(d.Questions, func(q app.Question) string { return q.Ref }); got != strings.Join(f.Questions, ",") {
		t.Fatalf("question refs = %s, want %v", got, f.Questions)
	}

	// Links come with the claims and hypotheses.
	c1 := d.Claims[0]
	if got := refsOf(c1.Links, func(l app.EvidenceLink) string { return l.EvidenceRef + ":" + string(l.Relation) }); got != "E-001:supports,E-004:supports" {
		t.Fatalf("C-001 links = %s", got)
	}
	if d.Claims[2].Status != app.ClaimStatusSuperseded || d.Claims[2].SupersededBy != "C-002" {
		t.Fatalf("C-003 = %s superseded_by %q", d.Claims[2].Status, d.Claims[2].SupersededBy)
	}
	if got := refsOf(d.Hypotheses[1].Links, func(l app.EvidenceLink) string { return l.EvidenceRef + ":" + string(l.Relation) }); got != "E-003:contradicts" {
		t.Fatalf("H-002 links = %s", got)
	}
	if d.Evidence[0].RawDetail != "" || !d.Evidence[0].HasRawDetail {
		t.Fatalf("E-001 raw detail loaded without request: %q has=%v", d.Evidence[0].RawDetail, d.Evidence[0].HasRawDetail)
	}
	if got := refsOf(d.Evidence[2].ClaimLinks, func(l app.RecordLink) string { return l.Ref }); got != "C-002,C-003" {
		t.Fatalf("E-003 claim links = %s", got)
	}

	// Recent activity equals the newest rows of ListActivity for the investigation.
	all, err := a.ListActivity(ctx, app.ActivityFilter{InvestigationRef: f.InvestigationRef, Limit: 500})
	if err != nil {
		t.Fatal(err)
	}
	want := all
	if len(want) > 20 {
		want = want[:20]
	}
	if len(d.RecentActivity) != len(want) {
		t.Fatalf("recent activity = %d rows, want %d", len(d.RecentActivity), len(want))
	}
	for i := range want {
		if d.RecentActivity[i] != want[i] {
			t.Fatalf("activity[%d] = %+v, want %+v", i, d.RecentActivity[i], want[i])
		}
		if i > 0 && d.RecentActivity[i].ID >= d.RecentActivity[i-1].ID {
			t.Fatal("recent activity is not newest first")
		}
	}
	apptest.AssertNoJSONNulls(t, d)

	// Raw detail on request.
	raw, err := a.GetInvestigationDetail(ctx, f.InvestigationRef, app.DetailOptions{IncludeRawDetail: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(raw.Evidence[0].RawDetail, "+  level: debug") {
		t.Fatalf("E-001 raw detail = %q, want the chart diff", raw.Evidence[0].RawDetail)
	}

	// ActivityLimit.
	three, err := a.GetInvestigationDetail(ctx, f.InvestigationRef, app.DetailOptions{ActivityLimit: 3})
	if err != nil {
		t.Fatal(err)
	}
	if len(three.RecentActivity) != 3 || three.RecentActivity[0] != all[0] {
		t.Fatalf("ActivityLimit 3 gave %d rows", len(three.RecentActivity))
	}
	for _, bad := range []int{-1, 201} {
		_, err := a.GetInvestigationDetail(ctx, f.InvestigationRef, app.DetailOptions{ActivityLimit: bad})
		wantValidation(t, err, "activity_limit")
	}
	if _, err := a.GetInvestigationDetail(ctx, f.InvestigationRef, app.DetailOptions{ActivityLimit: 200}); err != nil {
		t.Fatalf("ActivityLimit 200: %v", err)
	}

	// An empty investigation has empty, non-nil slices.
	empty, err := a.GetInvestigationDetail(ctx, other.Ref, app.DetailOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(empty.Evidence)+len(empty.Claims)+len(empty.Hypotheses)+len(empty.Questions) != 0 || len(empty.RecentActivity) != 1 {
		t.Fatalf("empty detail = %+v", empty)
	}
	apptest.AssertNoJSONNulls(t, empty)

	_, err = a.GetInvestigationDetail(ctx, "INV-0099", app.DetailOptions{})
	wantErrIs(t, err, app.ErrNotFound)
}

func TestGetInvestigationDetailNoActive(t *testing.T) {
	a := apptest.Open(t)
	_, err := a.GetInvestigationDetail(context.Background(), "", app.DetailOptions{})
	wantErrIs(t, err, app.ErrNoActiveInvestigation)
}

func TestListInvestigations(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t, app.WithClock(apptest.StepClock(apptest.FixedTime, time.Second)))
	inv1 := newInv(t, a, "First")
	inv2 := newInv(t, a, "Second")
	inv3 := newInv(t, a, "Third")

	list := func(o app.ListOptions) []app.InvestigationSummary {
		t.Helper()
		got, err := a.ListInvestigations(ctx, o)
		if err != nil {
			t.Fatalf("ListInvestigations(%+v): %v", o, err)
		}
		apptest.AssertNoJSONNulls(t, got)
		return got
	}
	ref := func(s app.InvestigationSummary) string { return s.Ref }

	if got := refsOf(list(app.ListOptions{}), ref); got != "INV-0003,INV-0002,INV-0001" {
		t.Fatalf("order = %s, want newest first", got)
	}

	// Touching the first investigation moves it to the top; counts follow.
	mustAddEvidence(t, a, apptest.Human, inv1.Ref, evInput("Evidence one"))
	mustAddQuestion(t, a, apptest.Human, inv1.Ref, "Open question?")
	q2 := mustAddQuestion(t, a, apptest.Human, inv1.Ref, "Answered question?")
	if _, err := a.ResolveQuestion(ctx, apptest.Human, inv1.Ref, q2.Ref, "Yes."); err != nil {
		t.Fatal(err)
	}
	got := list(app.ListOptions{})
	if refsOf(got, ref) != "INV-0001,INV-0003,INV-0002" {
		t.Fatalf("order after update = %s", refsOf(got, ref))
	}
	top := got[0]
	wantCounts := app.RecordCounts{Evidence: 1, Questions: 2, OpenQuestions: 1}
	if top.Counts != wantCounts {
		t.Fatalf("counts = %+v, want %+v", top.Counts, wantCounts)
	}
	if top.Title != "First" || top.Status != app.InvestigationStatusNew || top.ConclusionConfidence != app.ConfidenceUnknown ||
		top.CreatedAt == "" || top.UpdatedAt <= top.CreatedAt || top.Archived || top.ArchivedAt != "" || top.IsActive {
		t.Fatalf("summary = %+v", top)
	}

	// IsActive.
	if _, err := a.SetActiveInvestigation(ctx, apptest.Human, inv2.Ref); err != nil {
		t.Fatal(err)
	}
	for _, s := range list(app.ListOptions{}) {
		if s.IsActive != (s.Ref == inv2.Ref) {
			t.Fatalf("%s IsActive = %v", s.Ref, s.IsActive)
		}
	}

	// Archived rows are hidden unless asked for.
	if _, err := a.ArchiveInvestigation(ctx, apptest.Human, inv3.Ref); err != nil {
		t.Fatal(err)
	}
	if got := refsOf(list(app.ListOptions{}), ref); strings.Contains(got, inv3.Ref) {
		t.Fatalf("archived %s listed by default: %s", inv3.Ref, got)
	}
	withArchived := list(app.ListOptions{IncludeArchived: true})
	if len(withArchived) != 3 || withArchived[0].Ref != inv3.Ref || !withArchived[0].Archived || withArchived[0].ArchivedAt == "" ||
		withArchived[0].Status != app.InvestigationStatusArchived {
		t.Fatalf("IncludeArchived = %+v", withArchived)
	}
	if got := refsOf(list(app.ListOptions{Status: "archived"}), ref); got != inv3.Ref {
		t.Fatalf("Status archived = %s", got)
	}
	if got := refsOf(list(app.ListOptions{Status: "new", IncludeArchived: true}), ref); got != "INV-0001,INV-0002" {
		t.Fatalf("Status new = %s", got)
	}
	if got := list(app.ListOptions{Limit: 1}); len(got) != 1 || got[0].Ref != inv1.Ref {
		t.Fatalf("Limit 1 = %s", refsOf(got, ref))
	}

	_, err := a.ListInvestigations(ctx, app.ListOptions{Status: "bogus"})
	wantValidation(t, err, "status")
	for _, bad := range []int{-1, 1001} {
		_, err := a.ListInvestigations(ctx, app.ListOptions{Limit: bad})
		wantValidation(t, err, "limit")
	}
	if _, err := a.ListInvestigations(ctx, app.ListOptions{Limit: 1000}); err != nil {
		t.Fatalf("Limit 1000: %v", err)
	}
}

func TestListInvestigationsFixture(t *testing.T) {
	a := apptest.Open(t)
	f := apptest.SeedCloudSpend(t, a)
	got, err := a.ListInvestigations(context.Background(), app.ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d summaries", len(got))
	}
	s := got[0]
	want := app.RecordCounts{Evidence: 4, Claims: 3, Hypotheses: 2, Questions: 1, OpenQuestions: 1}
	if s.Ref != f.InvestigationRef || s.Title != "September cloud bill up 38%" || s.Status != app.InvestigationStatusInvestigating ||
		s.ConclusionConfidence != app.ConfidenceHigh || !s.IsActive || s.Counts != want {
		t.Fatalf("summary = %+v", s)
	}
}

func TestListInvestigationsEmpty(t *testing.T) {
	a := apptest.Open(t)
	got, err := a.ListInvestigations(context.Background(), app.ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || len(got) != 0 {
		t.Fatalf("empty list = %#v", got)
	}
	apptest.AssertNoJSONNulls(t, got)
}
