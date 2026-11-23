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
	if list[0].Ref != demo.FeaturedRef || !list[0].IsActive || list[0].Title != demo.FeaturedTitle {
		t.Fatalf("top of the list = %+v, want the active featured question", list[0])
	}
	byRef := map[string]app.InvestigationSummary{}
	for _, s := range list {
		byRef[s.Ref] = s
	}
	if s := byRef["INV-0001"]; s.Title != "Why did more customers cancel in March?" || s.Status != app.InvestigationStatusResolved ||
		s.Conclusion != "Customers who joined with two free months in January cancelled when they had to start paying in March." || s.SourcesTotal != 3 || s.SourcesCheckedByYou != 3 || s.AnswerCheckedByYou {
		t.Fatalf("INV-0001 = %+v", s)
	}
	// The list carries what the sidebar and Home show: the answer, the
	// count of sources checked, the findings where sources disagree and
	// whether the answer is checked. Between them the questions show each
	// thing Home can ask for: INV-0001 an answer to check, the featured one
	// sources to check and a finding to decide, INV-0003 an answer to write.
	if s := byRef[demo.FeaturedRef]; s.Conclusion != demo.Conclusion || s.SourcesTotal != 9 || s.SourcesCheckedByYou != 2 || s.FindingsDisagree != 1 || !s.AnswerCheckedByYou {
		t.Fatalf("%s = %+v", demo.FeaturedRef, s)
	}
	if s := byRef["INV-0003"]; s.Title != "Why are help desk tickets taking longer to close this month?" || s.Status != app.InvestigationStatusBlocked ||
		s.Conclusion != "" || s.SourcesTotal != 2 || s.SourcesCheckedByYou != 2 || s.Counts.OpenQuestions != 2 {
		t.Fatalf("INV-0003 = %+v", s)
	}
	if s := byRef["INV-0004"]; s.Status != app.InvestigationStatusResolved || s.Archived || s.Conclusion == "" || s.SourcesTotal != 3 || s.SourcesCheckedByYou != 0 {
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
	e6, err := a.GetEvidence(ctx, demo.FeaturedRef, "E-006", app.GetEvidenceOptions{IncludeRawDetail: true})
	if err != nil {
		t.Fatal(err)
	}
	if e6.RawDetail == "" || !strings.Contains(e6.Metadata, "calls_about_a_bill_october") {
		t.Errorf("E-006 raw detail %q, metadata %q", e6.RawDetail, e6.Metadata)
	}

	checkFeatured(t, a, true)

	rows, err := a.ListActivity(ctx, app.ActivityFilter{Limit: 500})
	if err != nil {
		t.Fatal(err)
	}
	clients := map[string]bool{}
	var prev string
	reviewed := false
	for i := len(rows) - 1; i >= 0; i-- {
		r := rows[i]
		reviewed = reviewed || (r.InvestigationRef == demo.FeaturedRef && r.Action == "conclusion.update" && r.Summary == "Recorded a review of the answer")
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
	if !reviewed {
		t.Error("no history row records the person's review of the answer")
	}
	if !clients["claude-code"] || !clients["codex"] {
		t.Errorf("AI activity clients = %v, want claude-code and codex", clients)
	}
}

// checkFeatured checks INV-0002 against the canonical scenario; full says
// whether Thursday (Claude Code's checkpoint and the person's check) was
// seeded.
func checkFeatured(t *testing.T, a *app.App, full bool) {
	t.Helper()
	ctx := context.Background()
	det, err := a.GetInvestigationDetail(ctx, demo.FeaturedRef, app.DetailOptions{ActivityLimit: 200})
	if err != nil {
		t.Fatal(err)
	}
	inv := det.Investigation
	if inv.Title != demo.FeaturedTitle || inv.ProblemStatement != "Calls to the call center went from 2,400 in September to 3,120 in October. Sales stayed the same. Why?" {
		t.Errorf("question = %q, %q", inv.Title, inv.ProblemStatement)
	}
	wantEvidence := map[string]string{
		"E-001": "Calls to buy: 2,000 in September, 2,050 in October. Sales: $510,000 in September, $512,000 in October",
		"E-002": "Sales manager: we were two people short in October",
		"E-003": "Callers waited 40 seconds in September and 40 seconds in October",
		"E-004": "No new ads ran in October",
		"E-005": "Prices were the same in September and October",
		"E-006": "Calls about a bill: 400 in September, 1,070 in October",
	}
	if full {
		wantEvidence["E-007"] = "Notice dated October 4: 1,900 customers were charged twice on October 3"
		wantEvidence["E-008"] = "Call notes: 9 in 10 October calls about a bill mention being charged twice"
		wantEvidence["E-009"] = "Billing team lead: the mistake was fixed on October 5"
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
	if h1.Status != app.HypothesisStatusDisproven || len(h1.Links) != 1 || !hasLink(h1.Links, "E-004", app.RelationContradicts) {
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
		q1.Resolution != "They were about a bill: 400 calls in September, 1,070 in October." {
		t.Errorf("Q-001 = %+v", q1)
	}
	claims := map[string]app.Claim{}
	for _, c := range det.Claims {
		claims[c.Ref] = c
	}
	if c1 := claims["C-001"]; c1.Status != app.ClaimStatusDisputed || c1.Verification != app.VerificationDisputed ||
		!hasLink(c1.Links, "E-002", app.RelationSupports) || !hasLink(c1.Links, "E-003", app.RelationContradicts) {
		t.Errorf("C-001 = %+v", c1)
	}
	if verif["E-001"] != app.VerificationSourceVerified {
		t.Errorf("E-001 = %s, want the one source an AI checked", verif["E-001"])
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
	if verif["E-007"] != app.VerificationHumanVerified || verif["E-008"] != app.VerificationAIGenerated || verif["E-009"] != app.VerificationHumanVerified {
		t.Errorf("verifications = %v", verif)
	}
	if h4 := hyps["H-004"]; h4.Status != app.HypothesisStatusOpen {
		t.Errorf("H-004 = %+v", h4)
	}
	if h3 := hyps["H-003"]; h3.Status != app.HypothesisStatusSupported || h3.Confidence != app.ConfidenceHigh {
		t.Errorf("H-003 = %+v", h3)
	}
	if len(det.Claims) != 3 {
		t.Fatalf("claims = %+v", det.Claims)
	}
	c3 := claims["C-003"]
	if c3.Verification != app.VerificationAIGenerated || c3.Confidence != app.ConfidenceHigh ||
		len(c3.Links) != 4 || c3.CreatedBy != demo.ClaudeCode {
		t.Errorf("C-003 = %+v", c3)
	}
	for _, ref := range []string{"E-006", "E-007", "E-008", "E-009"} {
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
		for _, banned := range []string{"flat", "margin", "Q3", "Q4", "SLA", "P&L", "hypothes", "disputed", "verified", "superseded", "confidence", " case", "claim", "evidence", "conclusion", "investigation"} {
			if strings.Contains(strings.ToLower(s), strings.ToLower(banned)) {
				t.Errorf("demo text uses %q: %q", banned, s)
			}
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
