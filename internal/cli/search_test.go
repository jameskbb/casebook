package cli

import (
	"strings"
	"testing"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/app/apptest"
)

func TestSearch(t *testing.T) {
	a, db, _ := seededApp(t)
	ctx := t.Context()

	r := runCLI(t, "", "--db", db, "search", "debug")
	wantCode(t, r, 0)
	lines := strings.Split(strings.TrimRight(r.Stdout, "\n"), "\n")
	wantContains(t, lines[0], "CASE", "TYPE", "REF", "MATCH")
	found := false
	for _, l := range lines[1:] {
		if strings.Contains(l, "INV-0001") && strings.Contains(l, "claim") && strings.Contains(l, "C-001") {
			found = true
		}
	}
	if !found {
		t.Errorf("no row for INV-0001 claim C-001:\n%s", r.Stdout)
	}

	r = runCLI(t, "", "--db", db, "--json", "search", "debug")
	wantCode(t, r, 0)
	var hits []app.SearchHit
	decodeJSON(t, r.Stdout, &hits)
	if len(hits) == 0 || hits[0].InvestigationRef != "INV-0001" {
		t.Errorf("hits = %+v", hits)
	}

	r = runCLI(t, "", "--db", db, "search", "zzzzqqq")
	wantCode(t, r, 0)
	if r.Stdout != "No matches.\n" {
		t.Errorf("stdout = %q", r.Stdout)
	}
	r = runCLI(t, "", "--db", db, "--json", "search", "zzzzqqq")
	wantCode(t, r, 0)
	if strings.TrimSpace(r.Stdout) != "[]" {
		t.Errorf("no-match --json = %q", r.Stdout)
	}

	// Several words are joined into one query.
	r = runCLI(t, "", "--db", db, "--json", "search", "debug", "logging")
	wantCode(t, r, 0)
	decodeJSON(t, r.Stdout, &hits)
	if len(hits) == 0 {
		t.Error("multi-word search found nothing")
	}

	other, err := a.CreateInvestigation(ctx, apptest.Human, app.CreateInvestigationInput{Title: "Debug log retention"})
	if err != nil {
		t.Fatal(err)
	}
	r = runCLI(t, "", "--db", db, "search", "debug")
	wantCode(t, r, 0)
	wantContains(t, r.Stdout, other.Ref)
	r = runCLI(t, "", "--db", db, "search", "debug", "--inv", "INV-0001")
	wantCode(t, r, 0)
	if strings.Contains(r.Stdout, other.Ref) {
		t.Errorf("--inv INV-0001 returned %s:\n%s", other.Ref, r.Stdout)
	}
	wantContains(t, r.Stdout, "C-001")

	if _, err := a.ArchiveInvestigation(ctx, apptest.Human, other.Ref); err != nil {
		t.Fatal(err)
	}
	r = runCLI(t, "", "--db", db, "search", "retention")
	wantCode(t, r, 0)
	if r.Stdout != "No matches.\n" {
		t.Errorf("archived hit without --archived:\n%s", r.Stdout)
	}
	r = runCLI(t, "", "--db", db, "search", "retention", "--archived")
	wantCode(t, r, 0)
	wantContains(t, r.Stdout, other.Ref)
}

func TestSearchErrors(t *testing.T) {
	_, db, _ := seededApp(t)
	cases := []struct {
		name   string
		args   []string
		code   int
		stderr string
	}{
		{"no words", []string{"search"}, 2, "casebook: "},
		{"limit too high", []string{"search", "Billing", "--limit", "101"}, 2, "casebook: invalid limit:"},
		{"limit negative", []string{"search", "Billing", "--limit", "-1"}, 2, "casebook: invalid limit:"},
		{"unknown inv", []string{"search", "Billing", "--inv", "INV-0099"}, 3, "casebook: not found:"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := runCLI(t, "", append([]string{"--db", db}, c.args...)...)
			wantCode(t, r, c.code)
			if !strings.HasPrefix(r.Stderr, c.stderr) || strings.Count(r.Stderr, "\n") != 1 {
				t.Errorf("stderr = %q, want one line starting %q", r.Stderr, c.stderr)
			}
		})
	}
}

// TestSearchDashWord proves a query word starting with "-" works after "--"
// and that the help says so (AR-34).
func TestSearchDashWord(t *testing.T) {
	db, _ := seededDB(t)
	r := runCLI(t, "", "search", "--db", db, "--", "-claim")
	wantCode(t, r, 0)
	h := runCLI(t, "", "search", "--help")
	if !strings.Contains(h.Stdout, `after "--"`) {
		t.Errorf("search help does not explain --:\n%s", h.Stdout)
	}
}
