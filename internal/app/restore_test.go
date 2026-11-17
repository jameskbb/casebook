package app_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/app/apptest"
)

// safetyCopies lists the safety copies in the App's backups directory.
func safetyCopies(t *testing.T, a *app.App) []string {
	t.Helper()
	m, err := filepath.Glob(filepath.Join(filepath.Dir(a.DBPath()), "backups", "casebook-pre-restore-*"))
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// evidenceRefs lists the evidence refs of invRef.
func evidenceRefs(t *testing.T, a *app.App, invRef string) string {
	t.Helper()
	evs, err := a.ListEvidence(context.Background(), invRef)
	if err != nil {
		t.Fatalf("ListEvidence: %v", err)
	}
	var refs []string
	for _, e := range evs {
		refs = append(refs, e.Ref)
	}
	return strings.Join(refs, ",")
}

func TestRestoreRoundTrip(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t)
	f := apptest.SeedCloudSpend(t, a)
	// A second App on the same file, opened before the restore.
	other := apptest.Reopen(t, a.DBPath())
	backup := backupOf(t, a)

	// Mutate after the backup.
	if _, err := a.AddEvidence(ctx, apptest.Human, f.InvestigationRef, app.EvidenceInput{
		Title: "Zanzibar artefact", Summary: "Only after the backup: xylophonequartz", SourceType: app.SourceTypeFile,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.ArchiveInvestigation(ctx, apptest.Human, f.InvestigationRef); err != nil {
		t.Fatal(err)
	}
	inv2, err := a.CreateInvestigation(ctx, apptest.Human, app.CreateInvestigationInput{Title: "Second", MakeActive: true})
	if err != nil {
		t.Fatal(err)
	}
	pre, err := a.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got := evidenceRefs(t, other, f.InvestigationRef); got != "E-001,E-002,E-003,E-004,E-005" {
		t.Fatalf("other App before restore sees %s", got)
	}
	tokenBefore := token(t, a)
	otherTokenBefore := token(t, other)

	res, err := a.Restore(ctx, apptest.Human, backup.Path)
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if res.SourcePath != backup.Path || res.SourceSchemaVersion != backup.SchemaVersion {
		t.Fatalf("result = %+v", res)
	}
	wantCounts := backup.Counts
	wantCounts.Activity++ // the restore row
	if res.Counts != wantCounts {
		t.Fatalf("counts = %+v, want %+v", res.Counts, wantCounts)
	}

	if got := evidenceRefs(t, a, f.InvestigationRef); got != "E-001,E-002,E-003,E-004" {
		t.Fatalf("evidence after restore = %s", got)
	}
	inv, err := a.GetInvestigation(ctx, f.InvestigationRef)
	if err != nil || inv.Archived || inv.Status != app.InvestigationStatusInvestigating {
		t.Fatalf("investigation after restore = %+v, %v", inv, err)
	}
	if _, err := a.GetInvestigation(ctx, inv2.Ref); !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("INV-0002 after restore: %v", err)
	}
	active, err := a.GetActiveInvestigation(ctx)
	if err != nil || active == nil || active.Ref != f.InvestigationRef {
		t.Fatalf("active after restore = %v, %v", active, err)
	}
	if hits := search(t, a, "debug logging", app.SearchOptions{}); len(hits) == 0 {
		t.Fatal("search lost fixture text")
	}
	if hits := search(t, a, "xylophonequartz", app.SearchOptions{IncludeArchived: true}); len(hits) != 0 {
		t.Fatalf("search still finds E-005 text: %+v", hits)
	}
	if err := a.CheckIntegrity(ctx); err != nil {
		t.Fatalf("CheckIntegrity: %v", err)
	}
	if token(t, a) == tokenBefore {
		t.Fatal("ChangeToken did not move")
	}
	acts, err := a.ListActivity(ctx, app.ActivityFilter{Limit: 1})
	if err != nil || len(acts) != 1 {
		t.Fatalf("ListActivity = %v, %v", acts, err)
	}
	row := acts[0]
	wantSummary := "Restored backup casebook-backup.db (safety copy " + filepath.Base(res.SafetyCopyPath) + ")"
	if row.Action != "restore" || row.InvestigationRef != "" || row.Actor != apptest.Human || row.Summary != wantSummary {
		t.Fatalf("restore row = %+v", row)
	}

	// Safety copy: name, permissions, and the pre-restore state inside.
	dir := filepath.Join(filepath.Dir(a.DBPath()), "backups")
	wantName := "casebook-pre-restore-" + apptest.FixedTime.UTC().Format("20060102T150405Z") + ".db"
	if res.SafetyCopyPath != filepath.Join(dir, wantName) {
		t.Fatalf("safety copy = %s, want %s", res.SafetyCopyPath, wantName)
	}
	if runtime.GOOS != "windows" {
		assertMode(t, res.SafetyCopyPath, 0o600)
		assertMode(t, dir, 0o700)
	}
	safety, err := a.ValidateBackup(ctx, res.SafetyCopyPath)
	if err != nil {
		t.Fatalf("ValidateBackup(safety copy): %v", err)
	}
	if safety.Counts != pre.Counts {
		t.Fatalf("safety copy counts = %+v, want pre-restore %+v", safety.Counts, pre.Counts)
	}
	// Only the safety copy is left in the backups directory.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("backups dir holds %v, want only the safety copy", names)
	}

	// The other App sees the restored state without reopening.
	if got := evidenceRefs(t, other, f.InvestigationRef); got != "E-001,E-002,E-003,E-004" {
		t.Fatalf("other App after restore sees %s", got)
	}
	if _, err := other.GetInvestigation(ctx, inv2.Ref); !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("other App still sees INV-0002: %v", err)
	}
	if token(t, other) == otherTokenBefore {
		t.Fatal("other App's ChangeToken did not move")
	}
	// Writes keep working on both Apps.
	if _, err := other.AddEvidence(ctx, apptest.AI, f.InvestigationRef, app.EvidenceInput{
		Title: "After restore", Summary: "s", SourceType: app.SourceTypeFile}); err != nil {
		t.Fatalf("write after restore: %v", err)
	}
	if got := evidenceRefs(t, a, f.InvestigationRef); got != "E-001,E-002,E-003,E-004,E-005" {
		t.Fatalf("evidence after a post-restore write = %s", got)
	}
}

func TestRestoreSafetyCopyNamesAreUnique(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t)
	apptest.SeedCloudSpend(t, a)
	backup := backupOf(t, a)
	first, err := a.Restore(ctx, apptest.Human, backup.Path)
	if err != nil {
		t.Fatalf("first Restore: %v", err)
	}
	second, err := a.Restore(ctx, apptest.Human, backup.Path)
	if err != nil {
		t.Fatalf("second Restore: %v", err)
	}
	if first.SafetyCopyPath == second.SafetyCopyPath {
		t.Fatalf("both restores used %s", first.SafetyCopyPath)
	}
	if !strings.HasSuffix(second.SafetyCopyPath, "Z-2.db") {
		t.Fatalf("second safety copy = %s, want a -2 suffix", filepath.Base(second.SafetyCopyPath))
	}
	if n := len(safetyCopies(t, a)); n != 2 {
		t.Fatalf("safety copies = %d, want 2", n)
	}
}

func TestRestoreRefusesBadBackups(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t)
	apptest.SeedCloudSpend(t, a)
	good := backupOf(t, a).Path
	if _, err := a.AddEvidence(ctx, apptest.Human, "", app.EvidenceInput{
		Title: "Live only", Summary: "s", SourceType: app.SourceTypeFile}); err != nil {
		t.Fatal(err)
	}
	before, err := a.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	tok := token(t, a)

	bad := map[string]func(t *testing.T) string{
		"corrupt": func(t *testing.T) string {
			p := filepath.Join(t.TempDir(), "corrupt.db")
			os.WriteFile(p, []byte(strings.Repeat("garbage!", 2048)), 0o600)
			return p
		},
		"foreign": func(t *testing.T) string {
			p := filepath.Join(t.TempDir(), "foreign.db")
			execFile(t, p, `CREATE TABLE investigations (id INTEGER PRIMARY KEY, title TEXT)`)
			return p
		},
		"tampered trigger": func(t *testing.T) string {
			p := copyFile(t, good)
			execFile(t, p, `CREATE TRIGGER evil AFTER DELETE ON evidence BEGIN DELETE FROM investigations; END`)
			return p
		},
		"oversized row": func(t *testing.T) string {
			p := copyFile(t, good)
			execFile(t, p, `UPDATE claims SET statement = '`+strings.Repeat("s", 2001)+`'`)
			return p
		},
		"newer": func(t *testing.T) string {
			p := copyFile(t, good)
			execFile(t, p, `INSERT INTO schema_migrations (version, applied_at) VALUES (99, 'x')`)
			return p
		},
	}
	for name, mk := range bad {
		t.Run(name, func(t *testing.T) {
			_, err := a.Restore(ctx, apptest.Human, mk(t))
			wantErrIs(t, err, app.ErrConflict)
			assertNoRawSQLite(t, err)
		})
	}
	_, err = a.Restore(ctx, apptest.Human, a.DBPath())
	wantErrIs(t, err, app.ErrConflict)
	_, err = a.Restore(ctx, apptest.Human, filepath.Join(t.TempDir(), "missing.db"))
	wantErrIs(t, err, app.ErrNotFound)
	_, err = a.Restore(ctx, app.Actor{Kind: app.ActorHuman}, good)
	wantValidation(t, err, "actor.client")

	if n := len(safetyCopies(t, a)); n != 0 {
		t.Fatalf("refused restores left %d safety copies", n)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(a.DBPath()), "backups")); err == nil {
		t.Fatal("refused restores created the backups directory")
	}
	after, err := a.Status(ctx)
	if err != nil || after.Counts != before.Counts {
		t.Fatalf("refused restores changed counts: %+v -> %+v (%v)", before.Counts, after.Counts, err)
	}
	if token(t, a) != tok {
		t.Fatal("refused restores changed the database")
	}
}

// TestRestoreRepairsLowCounters proves a backup whose ref counters lag
// behind its refs (crafted or damaged) restores fine and never makes later
// writes collide with existing refs: restore raises every counter to at
// least the highest ref in use.
func TestRestoreRepairsLowCounters(t *testing.T) {
	ctx := context.Background()
	for name, stmts := range map[string][]string{
		"zeroed": {
			`UPDATE counters SET value = 0`,
			`UPDATE investigations SET last_evidence_num = 0, last_claim_num = 0, last_hypothesis_num = 0, last_question_num = 0`,
		},
		"missing counter row": {
			`DELETE FROM counters`,
			`UPDATE investigations SET last_evidence_num = 1, last_claim_num = 1, last_hypothesis_num = 1, last_question_num = 0`,
		},
	} {
		t.Run(name, func(t *testing.T) {
			a := apptest.Open(t)
			apptest.SeedCloudSpend(t, a)
			if _, err := a.CreateInvestigation(ctx, apptest.Human, app.CreateInvestigationInput{Title: "Second"}); err != nil {
				t.Fatal(err)
			}
			p := copyFile(t, backupOf(t, a).Path)
			execFile(t, p, stmts...)
			if _, err := a.Restore(ctx, apptest.Human, p); err != nil {
				t.Fatalf("Restore: %v", err)
			}
			inv, err := a.CreateInvestigation(ctx, apptest.Human, app.CreateInvestigationInput{Title: "Third"})
			if err != nil || inv.Ref != "INV-0003" {
				t.Fatalf("CreateInvestigation = %q, %v; want INV-0003", inv.Ref, err)
			}
			ev, err := a.AddEvidence(ctx, apptest.Human, "INV-0001", app.EvidenceInput{
				Title: "More", Summary: "s", SourceType: app.SourceTypeFile})
			if err != nil || ev.Ref != "E-005" {
				t.Fatalf("AddEvidence = %q, %v; want E-005", ev.Ref, err)
			}
			c, err := a.AddClaim(ctx, apptest.Human, "INV-0001", app.ClaimInput{Statement: "More"})
			if err != nil || c.Ref != "C-004" {
				t.Fatalf("AddClaim = %q, %v; want C-004", c.Ref, err)
			}
			h, err := a.AddHypothesis(ctx, apptest.Human, "INV-0001", app.HypothesisInput{Statement: "More"})
			if err != nil || h.Ref != "H-003" {
				t.Fatalf("AddHypothesis = %q, %v; want H-003", h.Ref, err)
			}
			q, err := a.AddQuestion(ctx, apptest.Human, "INV-0001", app.QuestionInput{Question: "More?"})
			if err != nil || q.Ref != "Q-002" {
				t.Fatalf("AddQuestion = %q, %v; want Q-002", q.Ref, err)
			}
		})
	}
}

// TestRestoreRefusesBrokenInvariants proves validation refuses backups that
// break cross-row rules the schema's triggers only enforce on writes, before
// any safety copy or write transaction.
func TestRestoreRefusesBrokenInvariants(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t)
	apptest.SeedCloudSpend(t, a)
	if _, err := a.CreateInvestigation(ctx, apptest.Human, app.CreateInvestigationInput{Title: "Second"}); err != nil {
		t.Fatal(err)
	}
	good := backupOf(t, a).Path
	const (
		inv1 = `(SELECT id FROM investigations WHERE ref_num = 1)`
		inv2 = `(SELECT id FROM investigations WHERE ref_num = 2)`
		c2   = `(SELECT id FROM claims WHERE ref_num = 2 AND investigation_id = ` + inv1 + `)`
	)
	cases := []struct {
		name   string
		stmts  []string
		reason string
	}{
		{"cross-investigation claim link", []string{
			`UPDATE evidence SET investigation_id = ` + inv2 + `, ref_num = 1 WHERE ref_num = 1 AND investigation_id = ` + inv1,
		}, "claim_evidence links records of different investigations"},
		{"cross-investigation supersede", []string{
			`DELETE FROM claim_evidence WHERE claim_id = ` + c2,
			`UPDATE claims SET investigation_id = ` + inv2 + `, ref_num = 1 WHERE id = ` + c2,
		}, "claims.superseded_by_id names a claim of another investigation"},
		{"superseded without replacement", []string{
			`UPDATE claims SET superseded_by_id = NULL WHERE status = 'superseded'`,
		}, "claims.superseded_by_id is not set exactly when status is superseded"},
		{"resolution on an open question", []string{
			`UPDATE questions SET resolution = 'sneaky'`,
		}, "questions hold a resolution or defer note"},
		{"ref number past MaxRefNum", []string{
			`UPDATE investigations SET ref_num = 1000000000000 WHERE ref_num = 2`,
		}, "investigations.ref_num is out of range"},
		{"counter past MaxRefNum", []string{
			`UPDATE counters SET value = 1000000000000`,
		}, "counters holds a value out of range"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := copyFile(t, good)
			execFile(t, p, tc.stmts...)
			_, err := a.ValidateBackup(ctx, p)
			wantErrIs(t, err, app.ErrConflict)
			if !strings.Contains(err.Error(), tc.reason) {
				t.Fatalf("ValidateBackup error = %v, want reason %q", err, tc.reason)
			}
			_, err = a.Restore(ctx, apptest.Human, p)
			wantErrIs(t, err, app.ErrConflict)
			assertNoRawSQLite(t, err)
		})
	}
	if n := len(safetyCopies(t, a)); n != 0 {
		t.Fatalf("refused restores left %d safety copies", n)
	}
}
