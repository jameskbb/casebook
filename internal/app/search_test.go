package app_test

import (
	"context"
	"sort"
	"strings"
	"testing"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/app/apptest"
)

func search(t *testing.T, a *app.App, q string, o app.SearchOptions) []app.SearchHit {
	t.Helper()
	hits, err := a.Search(context.Background(), q, o)
	if err != nil {
		t.Fatalf("Search(%q, %+v): %v", q, o, err)
	}
	if hits == nil {
		t.Fatalf("Search(%q) returned a nil slice", q)
	}
	apptest.AssertNoJSONNulls(t, hits)
	return hits
}

// hitKeys returns "type:INV/ref" for each hit, sorted.
func hitKeys(hits []app.SearchHit) []string {
	out := make([]string, 0, len(hits))
	for _, h := range hits {
		out = append(out, string(h.RecordType)+":"+h.InvestigationRef+"/"+h.RecordRef)
	}
	sort.Strings(out)
	return out
}

func hasHit(hits []app.SearchHit, recType app.RecordType, ref string) bool {
	for _, h := range hits {
		if h.RecordType == recType && h.RecordRef == ref {
			return true
		}
	}
	return false
}

// searchEnv seeds the fixture (INV-0001), an archived INV-0002 about zebras
// and an active-free INV-0003 for ranking.
func searchEnv(t *testing.T) *app.App {
	t.Helper()
	ctx := context.Background()
	a := apptest.Open(t)
	apptest.SeedCloudSpend(t, a)

	zebra := newInv(t, a, "Zebra crossing audit")
	mustAddEvidence(t, a, apptest.Human, zebra.Ref, app.EvidenceInput{
		Title: "Zebra stripes faded", Summary: "Paint is worn at the crossing.", SourceType: app.SourceTypeManualObservation,
	})
	if _, err := a.ArchiveInvestigation(ctx, apptest.Human, zebra.Ref); err != nil {
		t.Fatal(err)
	}

	wild := newInv(t, a, "Wildlife survey")
	mustAddEvidence(t, a, apptest.Human, wild.Ref, app.EvidenceInput{
		Title: "Field notes", Summary: "Late in the day we saw an okapi near the river bank behind the trees.",
		SourceType: app.SourceTypeManualObservation,
	})
	mustAddEvidence(t, a, apptest.Human, wild.Ref, app.EvidenceInput{
		Title: "Okapi sighting", Summary: "A short report from the ranger station.", SourceType: app.SourceTypeManualObservation,
	})
	return a
}

func TestSearchFindsRecords(t *testing.T) {
	a := searchEnv(t)

	hits := search(t, a, "debug", app.SearchOptions{})
	want := []string{
		"claim:INV-0001/C-001",
		"evidence:INV-0001/E-001", "evidence:INV-0001/E-004",
		"investigation:INV-0001/INV-0001",
	}
	if got := hitKeys(hits); strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("debug hits = %v, want %v", got, want)
	}
	for _, h := range hits {
		if h.InvestigationTitle != "September cloud bill up 38%" {
			t.Fatalf("hit title = %q", h.InvestigationTitle)
		}
		if !strings.Contains(strings.ToLower(h.Snippet), "**debug") {
			t.Fatalf("snippet %q does not mark the match", h.Snippet)
		}
		if h.Rank >= 0 {
			t.Fatalf("bm25 rank = %v, want negative", h.Rank)
		}
	}
	for i := 1; i < len(hits); i++ {
		if hits[i].Rank < hits[i-1].Rank {
			t.Fatalf("hits not ordered by rank: %v then %v", hits[i-1].Rank, hits[i].Rank)
		}
	}

	// Investigation hits use the INV ref; superseded claims stay searchable.
	if bill := search(t, a, "bill", app.SearchOptions{}); !hasHit(bill, app.RecordInvestigation, "INV-0001") {
		t.Fatalf("bill hits = %v", hitKeys(bill))
	}
	if auto := search(t, a, "autoscaling", app.SearchOptions{}); !hasHit(auto, app.RecordClaim, "C-003") {
		t.Fatalf("autoscaling hits = %v", hitKeys(auto))
	}
	// Questions and hypotheses are indexed.
	if !hasHit(search(t, a, "except payments", app.SearchOptions{}), app.RecordQuestion, "Q-001") {
		t.Fatal("question Q-001 not found")
	}
	if !hasHit(search(t, a, "transfer", app.SearchOptions{}), app.RecordHypothesis, "H-002") {
		t.Fatal("hypothesis H-002 not found")
	}
	// Terms are ANDed.
	if got := search(t, a, "debug zebra", app.SearchOptions{IncludeArchived: true}); len(got) != 0 {
		t.Fatalf("debug AND zebra = %v", hitKeys(got))
	}
}

func TestSearchPrefixAndRawDetail(t *testing.T) {
	a := searchEnv(t)

	// The last term is a prefix: "Ingest" matches ingestion and ingested.
	hits := search(t, a, "Ingest", app.SearchOptions{})
	if !hasHit(hits, app.RecordClaim, "C-002") || !hasHit(hits, app.RecordEvidence, "E-002") ||
		!hasHit(hits, app.RecordEvidence, "E-003") {
		t.Fatalf("Ingest hits = %v", hitKeys(hits))
	}
	// Only the last term is a prefix: C-002 says "Log ingestion".
	if got := search(t, a, "Ingest log", app.SearchOptions{}); len(got) != 0 {
		t.Fatalf("non-final term matched as prefix: %v", hitKeys(got))
	}

	// json, delta_usd and monthly_costs only appear in the raw_detail of
	// E-001 and E-003, which is never indexed.
	for _, q := range []string{"json", "delta_usd", "monthly_costs"} {
		if got := search(t, a, q, app.SearchOptions{}); len(got) != 0 {
			t.Fatalf("%s found raw-only text: %v", q, hitKeys(got))
		}
	}
}

func TestSearchArchivedAndScope(t *testing.T) {
	a := searchEnv(t)

	if got := search(t, a, "zebra", app.SearchOptions{}); len(got) != 0 {
		t.Fatalf("archived investigation searched by default: %v", hitKeys(got))
	}
	got := search(t, a, "zebra", app.SearchOptions{IncludeArchived: true})
	want := []string{"evidence:INV-0002/E-001", "investigation:INV-0002/INV-0002"}
	if strings.Join(hitKeys(got), " ") != strings.Join(want, " ") {
		t.Fatalf("IncludeArchived zebra = %v, want %v", hitKeys(got), want)
	}
	// Naming an archived investigation searches it.
	if got := search(t, a, "zebra", app.SearchOptions{InvestigationRef: "inv-2"}); len(got) != 2 {
		t.Fatalf("scoped archived search = %v", hitKeys(got))
	}

	// Scope to one investigation.
	if got := search(t, a, "debug", app.SearchOptions{InvestigationRef: "INV-0003"}); len(got) != 0 {
		t.Fatalf("scoped debug in INV-0003 = %v", hitKeys(got))
	}
	for _, h := range search(t, a, "debug", app.SearchOptions{InvestigationRef: "INV-0001"}) {
		if h.InvestigationRef != "INV-0001" {
			t.Fatalf("scoped hit from %s", h.InvestigationRef)
		}
	}
	// An empty InvestigationRef searches everything, not the active one.
	if got := search(t, a, "okapi", app.SearchOptions{}); len(got) != 2 {
		t.Fatalf("unscoped okapi = %v", hitKeys(got))
	}
	_, err := a.Search(context.Background(), "x", app.SearchOptions{InvestigationRef: "INV-0042"})
	wantErrIs(t, err, app.ErrNotFound)
	_, err = a.Search(context.Background(), "x", app.SearchOptions{InvestigationRef: "nope"})
	wantValidation(t, err, "investigation_ref")
}

func TestSearchRanking(t *testing.T) {
	a := searchEnv(t)
	hits := search(t, a, "okapi", app.SearchOptions{InvestigationRef: "INV-0003"})
	if len(hits) != 2 {
		t.Fatalf("okapi hits = %v", hitKeys(hits))
	}
	if hits[0].RecordRef != "E-002" || hits[1].RecordRef != "E-001" {
		t.Fatalf("ranking = %s (%v) then %s (%v); want the title match E-002 first",
			hits[0].RecordRef, hits[0].Rank, hits[1].RecordRef, hits[1].Rank)
	}
}

func TestSearchLimitAndValidation(t *testing.T) {
	a := searchEnv(t)
	ctx := context.Background()
	if got := search(t, a, "debug", app.SearchOptions{Limit: 2}); len(got) != 2 {
		t.Fatalf("Limit 2 gave %d hits", len(got))
	}
	for _, bad := range []int{-1, 101} {
		_, err := a.Search(ctx, "debug", app.SearchOptions{Limit: bad})
		wantValidation(t, err, "limit")
	}
	for _, q := range []string{"", "   ", "\n\t"} {
		_, err := a.Search(ctx, q, app.SearchOptions{})
		wantValidation(t, err, "query")
	}
	_, err := a.Search(ctx, strings.Repeat("a", app.MaxSearchQueryBytes+1), app.SearchOptions{})
	wantValidation(t, err, "query")
}

func TestSearchHostileInput(t *testing.T) {
	a := searchEnv(t)
	ctx := context.Background()
	hostile := []string{
		`"`,
		`""""`,
		`AND OR NOT`,
		`NEAR(`,
		`NEAR(debug billing, 2)`,
		`title:debug`,
		`{title body}: debug`,
		`*`,
		`^x`,
		`debug*`,
		`de"bug`,
		`(debug OR billing`,
		`'; DROP TABLE investigations; --`,
		`" OR 1=1 --`,
		"\U0001F600",
		"\U0001F600 debug \U0001F525",
		"-debug +billing",
		"\x00debug\x07",
		strings.Repeat("debug ", 60),
		strings.Repeat("a ", 40),
	}
	for _, q := range hostile {
		hits, err := a.Search(ctx, q, app.SearchOptions{IncludeArchived: true})
		if err != nil {
			msg := strings.ToLower(err.Error())
			if strings.Contains(msg, "fts5") || strings.Contains(msg, "syntax") || strings.Contains(msg, "sqlite") {
				t.Fatalf("Search(%q) leaked raw FTS error: %v", q, err)
			}
			if _, ok := app.IsValidation(err); !ok {
				t.Fatalf("Search(%q) = %v, want hits or a ValidationError", q, err)
			}
			continue
		}
		if hits == nil {
			t.Fatalf("Search(%q) returned nil", q)
		}
	}

	// Operator words are literal text: "OR" alone does not match everything.
	if got := search(t, a, "debug OR zebra", app.SearchOptions{IncludeArchived: true}); len(got) != 0 {
		t.Fatalf("OR acted as an operator: %v", hitKeys(got))
	}
	if got := search(t, a, "title:debug", app.SearchOptions{}); len(got) != 0 {
		t.Fatalf("column filter acted as a filter: %v", hitKeys(got))
	}

	// The schema survived.
	list, err := a.ListInvestigations(ctx, app.ListOptions{IncludeArchived: true})
	if err != nil || len(list) != 3 {
		t.Fatalf("investigations after hostile search = %d, %v", len(list), err)
	}
	if got := search(t, a, "debug", app.SearchOptions{}); len(got) != 4 {
		t.Fatalf("debug after hostile search = %v", hitKeys(got))
	}
}
