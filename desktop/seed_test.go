package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/app/apptest"
	"github.com/jameskbb/casebook/internal/demo"
	"github.com/jameskbb/casebook/internal/paths"
)

func TestSeedDemoCoversEveryState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "casebook.db")
	now := time.Date(2026, 10, 8, 14, 0, 0, 0, time.UTC)
	if err := seedDemo(path, now, demo.StageFull); err != nil {
		t.Fatalf("seedDemo: %v", err)
	}
	a := apptest.Reopen(t, path)
	ctx := context.Background()

	list, err := a.ListInvestigations(ctx, app.ListOptions{IncludeArchived: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 5 {
		t.Fatalf("seeded %d investigations, want 5", len(list))
	}
	if list[0].Ref != demo.FeaturedRef || !list[0].IsActive || list[0].Title != "Northeast margin down 6 points in Q3" {
		t.Fatalf("top of the list = %+v, want the active featured case", list[0])
	}
	byRef := map[string]app.InvestigationSummary{}
	for _, s := range list {
		byRef[s.Ref] = s
	}
	if s := byRef["INV-0001"]; s.Title != "TestCheckoutTotals flakes in CI" || s.Status != app.InvestigationStatusResolved {
		t.Fatalf("INV-0001 = %+v", s)
	}
	if s := byRef["INV-0003"]; s.Title != "Checkout outage on October 2" || s.Status != app.InvestigationStatusBlocked {
		t.Fatalf("INV-0003 = %+v", s)
	}
	if s := byRef["INV-0004"]; s.Status != app.InvestigationStatusResolved || s.Archived {
		t.Fatalf("INV-0004 = %+v", s)
	}
	if s := byRef["INV-0005"]; !s.Archived {
		t.Fatalf("INV-0005 = %+v", s)
	}

	// The featured case keeps every record kind and state.
	det, err := a.GetInvestigationDetail(ctx, demo.FeaturedRef, app.DetailOptions{ActivityLimit: 200})
	if err != nil {
		t.Fatal(err)
	}
	claimStatus := map[app.ClaimStatus]bool{}
	for _, c := range det.Claims {
		claimStatus[c.Status] = true
	}
	for _, st := range []app.ClaimStatus{app.ClaimStatusActive, app.ClaimStatusDisputed} {
		if !claimStatus[st] {
			t.Errorf("no %s claim", st)
		}
	}
	for _, c := range det.Claims {
		if c.Ref == "C-001" && (c.Status != app.ClaimStatusDisputed || c.Verification != app.VerificationDisputed) {
			t.Errorf("C-001 = %+v, want disputed", c)
		}
	}
	hypStatus := map[app.HypothesisStatus]bool{}
	for _, h := range det.Hypotheses {
		hypStatus[h.Status] = true
	}
	for _, st := range []app.HypothesisStatus{app.HypothesisStatusOpen, app.HypothesisStatusSupported, app.HypothesisStatusDisproven} {
		if !hypStatus[st] {
			t.Errorf("no %s hypothesis", st)
		}
	}
	qStatus := map[app.QuestionStatus]bool{}
	for _, q := range det.Questions {
		qStatus[q.Status] = true
	}
	for _, st := range []app.QuestionStatus{app.QuestionStatusOpen, app.QuestionStatusResolved, app.QuestionStatusDeferred} {
		if !qStatus[st] {
			t.Errorf("no %s question", st)
		}
	}
	e5, err := a.GetEvidence(ctx, demo.FeaturedRef, "E-005", app.GetEvidenceOptions{IncludeRawDetail: true})
	if err != nil {
		t.Fatal(err)
	}
	if e5.RawDetail == "" || !strings.Contains(e5.Metadata, "implementation_services_usd") {
		t.Errorf("E-005 raw detail %q, metadata %q", e5.RawDetail, e5.Metadata)
	}

	checkFeatured(t, a, true)

	rows, err := a.ListActivity(ctx, app.ActivityFilter{Limit: 500})
	if err != nil {
		t.Fatal(err)
	}
	clients := map[string]bool{}
	var prev string
	for i := len(rows) - 1; i >= 0; i-- {
		r := rows[i]
		if r.Actor.Kind == app.ActorAI {
			clients[r.Actor.Client] = true
		}
		if prev != "" && r.CreatedAt < prev {
			t.Errorf("activity %d at %s is older than the row before it (%s)", r.ID, r.CreatedAt, prev)
		}
		prev = r.CreatedAt
		ts, err := time.Parse(app.TimeLayout, r.CreatedAt)
		if err != nil {
			t.Fatal(err)
		}
		if ts.Before(now.Add(-10*24*time.Hour)) || ts.After(now.Add(-30*time.Minute)) {
			t.Errorf("activity at %s falls outside the demo window", r.CreatedAt)
		}
	}
	if !clients["claude-code"] || !clients["codex"] {
		t.Errorf("AI activity clients = %v, want claude-code and codex", clients)
	}
}

// checkFeatured checks INV-0002 against the canonical scenario; full says
// whether Thursday (Claude Code's checkpoint and the person's Verify) was
// seeded.
func checkFeatured(t *testing.T, a *app.App, full bool) {
	t.Helper()
	ctx := context.Background()
	det, err := a.GetInvestigationDetail(ctx, demo.FeaturedRef, app.DetailOptions{ActivityLimit: 200})
	if err != nil {
		t.Fatal(err)
	}
	inv := det.Investigation
	if inv.ProblemStatement != "Why did gross margin in the Northeast fall from 41% to 35% in Q3 when revenue was up 4%?" {
		t.Errorf("question = %q", inv.ProblemStatement)
	}
	wantEvidence := map[string]string{
		"E-001": "Northeast Q3: revenue $27.0M (+4%), cost of delivery $17.6M (+15%), gross margin 35% against 41%",
		"E-002": "Q3 bookings: 47 new Northeast deals, seven above $250k, the two biggest with Pinecrest Logistics",
		"E-003": "Sales VP: we cut prices hard to win the quarter's two biggest deals",
		"E-004": "Onboarding team added four contractors in July",
		"E-005": "Implementation services are $1.9M of the $2.3M cost rise; contractor rate flat, hours up 78%",
		"E-006": "Discounts are flat: 11.6% in Q3 against 11.8% in Q2; the two biggest deals got 12% and 14%",
		"E-007": "Three accounts moved to white-glove onboarding in July carry $1.4M of the $1.9M",
	}
	if full {
		wantEvidence["E-008"] = "July 6 playbook update: white-glove onboarding included free on every deal above $250k"
		wantEvidence["E-009"] = "Deal desk: seven Northeast deals above $250k since July 6, onboarding fee $0 on every one"
		wantEvidence["E-010"] = "Regional sales-ops lead: the free white-glove clause was written for the two Pinecrest deals only"
	}
	if len(det.Evidence) != len(wantEvidence) {
		t.Errorf("%d evidence, want %d", len(det.Evidence), len(wantEvidence))
	}
	verif := map[string]app.Verification{}
	for _, e := range det.Evidence {
		if wantEvidence[e.Ref] != e.Title {
			t.Errorf("%s title = %q, want %q", e.Ref, e.Title, wantEvidence[e.Ref])
		}
		verif[e.Ref] = e.Verification
	}
	hyps := map[string]app.Hypothesis{}
	for _, h := range det.Hypotheses {
		hyps[h.Ref] = h
	}
	hasLink := func(links []app.EvidenceLink, ref string, rel app.Relation) bool {
		for _, l := range links {
			if l.EvidenceRef == ref && l.Relation == rel {
				return true
			}
		}
		return false
	}
	h1 := hyps["H-001"]
	if h1.Status != app.HypothesisStatusDisproven || len(h1.Links) != 2 ||
		!hasLink(h1.Links, "E-004", app.RelationSupports) || !hasLink(h1.Links, "E-005", app.RelationContradicts) {
		t.Errorf("H-001 = %+v", h1)
	}
	h2 := hyps["H-002"]
	if h2.Status != app.HypothesisStatusDisproven || len(h2.Links) != 1 || !hasLink(h2.Links, "E-005", app.RelationContradicts) {
		t.Errorf("H-002 = %+v", h2)
	}
	qs := map[string]app.Question{}
	for _, q := range det.Questions {
		qs[q.Ref] = q
	}
	if q1 := qs["Q-001"]; q1.Status != app.QuestionStatusResolved ||
		q1.Resolution != "Implementation services +$1.9M, travel +$210k, customer support +$110k, hosting +$90k, FX revaluation +$40k; everything else -$50k (E-005)." {
		t.Errorf("Q-001 = %+v", q1)
	}
	claims := map[string]app.Claim{}
	for _, c := range det.Claims {
		claims[c.Ref] = c
	}
	if c1 := claims["C-001"]; c1.Status != app.ClaimStatusDisputed || c1.Verification != app.VerificationDisputed {
		t.Errorf("C-001 = %+v", c1)
	}
	if c2 := claims["C-002"]; c2.Status != app.ClaimStatusActive || c2.Verification != app.VerificationAIGenerated {
		t.Errorf("C-002 = %+v", c2)
	}
	if !full {
		if len(det.Questions) != 2 || qs["Q-002"].Status != app.QuestionStatusOpen || len(det.Claims) != 2 ||
			hyps["H-003"].Status != app.HypothesisStatusOpen || inv.Conclusion != "" {
			t.Errorf("before Thursday: questions %+v, claims %+v, H-003 %+v, conclusion %q",
				det.Questions, det.Claims, hyps["H-003"], inv.Conclusion)
		}
		return
	}
	if verif["E-008"] != app.VerificationSourceVerified || verif["E-009"] != app.VerificationAIGenerated {
		t.Errorf("verifications = %v", verif)
	}
	if h3 := hyps["H-003"]; h3.Status != app.HypothesisStatusSupported || h3.Confidence != app.ConfidenceHigh {
		t.Errorf("H-003 = %+v", h3)
	}
	if len(det.Claims) != 3 {
		t.Fatalf("claims = %+v", det.Claims)
	}
	c3 := claims["C-003"]
	if c3.Verification != app.VerificationHumanVerified || c3.Confidence != app.ConfidenceHigh ||
		len(c3.Links) != 4 || c3.CreatedBy != demo.ClaudeCode {
		t.Errorf("C-003 = %+v", c3)
	}
	for _, ref := range []string{"E-007", "E-008", "E-009", "E-010"} {
		if !hasLink(c3.Links, ref, app.RelationSupports) {
			t.Errorf("C-003 links = %+v, want %s supports", c3.Links, ref)
		}
	}
	if qs["Q-002"].Status != app.QuestionStatusResolved || qs["Q-003"].Status != app.QuestionStatusOpen ||
		qs["Q-004"].Status != app.QuestionStatusDeferred {
		t.Errorf("questions = %+v", det.Questions)
	}
	if inv.Conclusion != demo.Conclusion || inv.ConclusionConfidence != app.ConfidenceHigh || inv.ConclusionRationale != demo.PersonRationale {
		t.Errorf("conclusion = %q (%s) %q", inv.Conclusion, inv.ConclusionConfidence, inv.ConclusionRationale)
	}
	strs := []string{inv.Title, inv.ProblemStatement, inv.Conclusion, inv.ConclusionRationale, demo.ThursdayNote}
	for _, e := range det.Evidence {
		strs = append(strs, e.Title, e.Summary)
	}
	for _, c := range det.Claims {
		strs = append(strs, c.Statement, c.ConfidenceRationale)
	}
	for _, h := range det.Hypotheses {
		strs = append(strs, h.Statement)
	}
	for _, q := range det.Questions {
		strs = append(strs, q.Question, q.Resolution)
	}
	for _, s := range strs {
		if strings.ContainsRune(s, '\u2014') {
			t.Errorf("em dash in %q", s)
		}
	}
}

func TestSeedDemoBeforeThursday(t *testing.T) {
	path := filepath.Join(t.TempDir(), "casebook.db")
	if err := seedDemo(path, time.Now(), demo.StageBeforeThursday); err != nil {
		t.Fatalf("seedDemo: %v", err)
	}
	a := apptest.Reopen(t, path)
	checkFeatured(t, a, false)
}

func TestSeedDemoRefusesNonEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "casebook.db")
	a, err := app.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.CreateInvestigation(context.Background(), app.ActorCLI(), app.CreateInvestigationInput{Title: "Already here"}); err != nil {
		t.Fatal(err)
	}
	a.Close()

	if err := seedDemo(path, time.Now(), demo.StageFull); !errors.Is(err, errSeedNotEmpty) {
		t.Fatalf("seedDemo on a non-empty database = %v, want errSeedNotEmpty", err)
	}
	b := apptest.Reopen(t, path)
	list, err := b.ListInvestigations(context.Background(), app.ListOptions{IncludeArchived: true})
	if err != nil || len(list) != 1 {
		t.Fatalf("after refused seed: %+v, %v", list, err)
	}
}

// fakeUserHome points every per-OS default location into a temp directory,
// clears CASEBOOK_HOME and CASEBOOK_DB, and returns the default database
// path internal/paths then resolves.
func fakeUserHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("LocalAppData", filepath.Join(home, "AppData", "Local"))
	t.Setenv(paths.EnvHome, "")
	t.Setenv(paths.EnvDB, "")
	def, err := paths.DBPath()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(def, home) {
		t.Fatalf("default database %q is not under the fake home %q", def, home)
	}
	return def
}

func TestSeedDemoRefusesDefaultDatabase(t *testing.T) {
	def := fakeUserHome(t)
	defDir := filepath.Dir(def)

	cases := []struct {
		name  string
		setup func(t *testing.T) string
	}{
		{"no flags", func(t *testing.T) string { return mustDBPath(t, flags{}) }},
		{"data dir at the default", func(t *testing.T) string { return mustDBPath(t, flags{DataDir: defDir}) }},
		{"data dir with a trailing separator", func(t *testing.T) string {
			return mustDBPath(t, flags{DataDir: defDir + string(filepath.Separator) + "."})
		}},
		{"CASEBOOK_HOME at the default", func(t *testing.T) string {
			t.Setenv(paths.EnvHome, defDir)
			return mustDBPath(t, flags{})
		}},
		{"CASEBOOK_DB at the default", func(t *testing.T) string {
			t.Setenv(paths.EnvDB, def)
			return mustDBPath(t, flags{})
		}},
		{"link to the default directory", func(t *testing.T) string {
			if err := os.MkdirAll(defDir, 0o700); err != nil {
				t.Fatal(err)
			}
			link := filepath.Join(t.TempDir(), "link")
			if err := os.Symlink(defDir, link); err != nil {
				t.Skipf("symlinks unavailable: %v", err)
			}
			return mustDBPath(t, flags{DataDir: link})
		}},
	}
	if foldCase {
		cases = append(cases, struct {
			name  string
			setup func(t *testing.T) string
		}{"different letter case", func(t *testing.T) string { return strings.ToUpper(def) }})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			target := tc.setup(t)
			home, homeSet := os.LookupEnv(paths.EnvHome)
			if err := seedDemo(target, time.Now(), demo.StageFull); !errors.Is(err, errSeedDefault) {
				t.Fatalf("seedDemo(%q) = %v, want errSeedDefault", target, err)
			}
			if _, err := os.Stat(def); !os.IsNotExist(err) {
				t.Fatalf("default database was created: %v", err)
			}
			if h, ok := os.LookupEnv(paths.EnvHome); h != home || ok != homeSet {
				t.Fatalf("CASEBOOK_HOME = %q (%v) after the check, want %q (%v)", h, ok, home, homeSet)
			}
		})
	}
}

func TestSeedDemoAllowsOtherDatabaseWithEnvSet(t *testing.T) {
	fakeUserHome(t)
	dir := t.TempDir()
	t.Setenv(paths.EnvHome, dir)
	target := mustDBPath(t, flags{})
	if err := seedDemo(target, time.Now(), demo.StageFull); err != nil {
		t.Fatalf("seedDemo into CASEBOOK_HOME %q: %v", dir, err)
	}
	if os.Getenv(paths.EnvHome) != dir {
		t.Fatal("CASEBOOK_HOME was not restored")
	}
}

func TestSeedDemoRefusesArchivedOnlyDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "casebook.db")
	a, err := app.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	inv, err := a.CreateInvestigation(ctx, app.ActorCLI(), app.CreateInvestigationInput{Title: "Archived already"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.ArchiveInvestigation(ctx, app.ActorCLI(), inv.Ref); err != nil {
		t.Fatal(err)
	}
	a.Close()
	if err := seedDemo(path, time.Now(), demo.StageFull); !errors.Is(err, errSeedNotEmpty) {
		t.Fatalf("seedDemo on a database with only an archived investigation = %v, want errSeedNotEmpty", err)
	}
}

func mustDBPath(t *testing.T, cfg flags) string {
	t.Helper()
	p, err := dbPathFor(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return p
}
