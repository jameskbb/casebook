package app_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/app/apptest"
)

// Compatibility with data from versions that had the Evernote integration.
// The integration is gone, but its dormant evernote_links table, the
// evernote_note source type and the evernote.post activity rows must survive
// backup, restore, import and export unchanged. Every fixture here is
// fictional and lives in a temporary database.

// legacyPostSummary is the summary older versions logged for a post.
const legacyPostSummary = "Posted the case block to an Evernote note (FICTIONAL-legacy-note)"

// execRaw runs one statement on the database at path through its own
// connection, as an older Casebook version would have written it.
func execRaw(t *testing.T, path, query string, args ...any) {
	t.Helper()
	raw, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	if _, err := raw.ExecContext(context.Background(), query, args...); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
}

// countRaw returns the single integer query returns on the database at path.
func countRaw(t *testing.T, path, query string, args ...any) int {
	t.Helper()
	raw, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	var n int
	if err := raw.QueryRowContext(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return n
}

// TestLegacyNoteLinkSurvivesBackupRestore puts a database holding an
// evernote_links row through backup and restore: the row comes back, the
// case still opens and renders, and the stable export never includes it.
func TestLegacyNoteLinkSurvivesBackupRestore(t *testing.T) {
	ctx := context.Background()
	a, path := openTestApp(t)
	inv, err := a.CreateInvestigation(ctx, apptest.Human, app.CreateInvestigationInput{Title: "Linked case", MakeActive: true})
	if err != nil {
		t.Fatal(err)
	}
	const guid = "FICTIONAL-guid-5e21"
	execRaw(t, path, `INSERT INTO evernote_links (investigation_id, note_guid, note_title, block_hash, note_usn, posted_at)
SELECT id, ?, 'Fictional note', ?, 41, '2026-01-02T03:04:05Z' FROM investigations WHERE title = 'Linked case'`,
		guid, strings.Repeat("a", 64))

	export, err := a.ExportInvestigation(ctx, inv.Ref)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(export), guid) {
		t.Fatal("the case export holds the dormant note link")
	}

	backup := filepath.Join(t.TempDir(), "backup.db")
	if _, err := a.Backup(ctx, backup); err != nil {
		t.Fatal(err)
	}
	execRaw(t, path, `DELETE FROM evernote_links`)
	if _, err := a.Restore(ctx, apptest.Human, backup); err != nil {
		t.Fatal(err)
	}
	if n := countRaw(t, path, `SELECT count(*) FROM evernote_links WHERE note_guid = ? AND note_usn = 41`, guid); n != 1 {
		t.Fatalf("evernote_links rows after restore = %d, want 1", n)
	}
	got, err := a.GetInvestigation(ctx, inv.Ref)
	if err != nil || got.Title != "Linked case" {
		t.Fatalf("case after restore = %+v, %v", got, err)
	}
	if _, err := a.Report(ctx, inv.Ref, app.ReportOptions{}); err != nil {
		t.Fatalf("report after restore: %v", err)
	}
}

// TestLegacyEvernoteEvidenceAndActivityImport imports a case export holding
// evernote_note evidence and an evernote.post activity row (as older
// versions wrote them). Every rendering keeps the "Evernote note" label, the
// evidence is never relabelled, the post never counts as a material change,
// and a second export round-trips both.
func TestLegacyEvernoteEvidenceAndActivityImport(t *testing.T) {
	ctx := context.Background()
	src, srcPath := openTestApp(t)
	inv, err := src.CreateInvestigation(ctx, apptest.Human, app.CreateInvestigationInput{
		Title: "Fictional margin question", ProblemStatement: "Why did the fictional margin fall?", MakeActive: true})
	if err != nil {
		t.Fatal(err)
	}
	// No source label, so renderings fall back to the source type's label.
	ev, err := src.AddEvidence(ctx, apptest.Human, inv.Ref, app.EvidenceInput{
		Title: "Fictional meeting notes", Summary: "The fictional team noted a pricing change.",
		SourceType: app.SourceTypeEvernoteNote, SourceRef: "FICTIONAL-note-1"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := src.AddClaim(ctx, apptest.Human, inv.Ref, app.ClaimInput{
		Statement: "A fictional pricing change lowered the margin.",
		Links:     []app.EvidenceLinkInput{{EvidenceRef: ev.Ref, Relation: app.RelationSupports}}}); err != nil {
		t.Fatal(err)
	}
	execRaw(t, srcPath, `INSERT INTO activity (investigation_id, actor_kind, actor_client, action, target_ref, summary, created_at)
SELECT id, 'ai', 'claude-code', 'evernote.post', '', ?, '2099-01-01T00:00:00Z' FROM investigations WHERE title = 'Fictional margin question'`,
		legacyPostSummary)
	export, err := src.ExportInvestigation(ctx, inv.Ref)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"evernote_note"`, `"evernote.post"`} {
		if !strings.Contains(string(export), want) {
			t.Fatalf("fixture export lacks %s", want)
		}
	}

	a := apptest.Open(t)
	res, err := a.ImportInvestigation(ctx, apptest.Human, export)
	if err != nil {
		t.Fatal(err)
	}
	ref := res.InvestigationRef

	got, err := a.GetEvidence(ctx, ref, ev.Ref, app.GetEvidenceOptions{})
	if err != nil || got.SourceType != app.SourceTypeEvernoteNote || got.SourceType.Label() != "Evernote note" {
		t.Fatalf("imported evidence = %+v, %v", got, err)
	}
	acts, err := a.ListActivity(ctx, app.ActivityFilter{InvestigationRef: ref, Limit: 100})
	if err != nil || !slices.ContainsFunc(acts, func(x app.Activity) bool { return x.Action == "evernote.post" }) {
		t.Fatalf("imported activity lacks the evernote.post row: %v, %v", acts, err)
	}

	report, err := a.Report(ctx, ref, app.ReportOptions{})
	if err != nil {
		t.Fatal(err)
	}
	brief, err := a.SharePreview(ctx, ref, app.ShareOptions{Mode: "brief"})
	if err != nil {
		t.Fatal(err)
	}
	pack, err := a.RenderContext(ctx, ref, "markdown", app.ContextOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for name, text := range map[string]string{"report": report, "brief": brief.Markdown, "context pack": pack} {
		if !strings.Contains(text, "Evernote note") {
			t.Errorf("%s lost the Evernote note label:\n%s", name, text)
		}
		if strings.Contains(text, legacyPostSummary) {
			t.Errorf("%s shows the legacy post as a material change:\n%s", name, text)
		}
	}
	p, err := a.ContextPack(ctx, ref, app.ContextOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range p.RecentChanges {
		if c.Action == "evernote.post" {
			t.Errorf("recent changes include the legacy post: %+v", c)
		}
	}

	again, err := a.ExportInvestigation(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"evernote_note"`, `"evernote.post"`} {
		if !strings.Contains(string(again), want) {
			t.Errorf("round-trip export lacks %s", want)
		}
	}
}
