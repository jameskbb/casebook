package app_test

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/app/apptest"
	"github.com/jameskbb/casebook/internal/store"

	_ "modernc.org/sqlite"
)

// rawDB opens the database file directly, bypassing internal/app, with
// foreign keys on or off. Tests use it to seed rows no app method creates yet
// and to inject corruption.
func rawDB(t *testing.T, path string, foreignKeys bool) *sql.DB {
	t.Helper()
	fk := "0"
	if foreignKeys {
		fk = "1"
	}
	p := filepath.ToSlash(path)
	if !strings.HasPrefix(p, "/") {
		// Windows drive path C:/x becomes /C:/x in a file URI.
		p = "/" + p
	}
	u := url.URL{Scheme: "file", Path: p}
	db, err := sql.Open("sqlite", u.String()+"?_pragma=busy_timeout(5000)&_pragma=foreign_keys("+fk+")")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func token(t *testing.T, a *app.App) string {
	t.Helper()
	tok, err := a.ChangeToken(context.Background())
	if err != nil {
		t.Fatalf("ChangeToken: %v", err)
	}
	if tok == "" {
		t.Fatal("ChangeToken returned an empty token")
	}
	return tok
}

func TestChangeTokenStableWithoutWrites(t *testing.T) {
	a := apptest.Open(t)
	newInv(t, a, "T")
	t1, t2 := token(t, a), token(t, a)
	if t1 != t2 {
		t.Fatalf("tokens differ with no writes: %q then %q", t1, t2)
	}
	// Reads do not move it either.
	if _, err := a.ListEvidence(context.Background(), "INV-0001"); err != nil {
		t.Fatal(err)
	}
	if t3 := token(t, a); t3 != t1 {
		t.Fatalf("token moved after a read: %q then %q", t1, t3)
	}
}

func TestChangeTokenSameAppWrite(t *testing.T) {
	a := apptest.Open(t)
	inv := newInv(t, a, "T")
	before := token(t, a)
	mustAddEvidence(t, a, apptest.AI, inv.Ref, evInput("E"))
	after := token(t, a)
	if after == before {
		t.Fatalf("token unchanged after AddEvidence: %q", after)
	}
	if again := token(t, a); again != after {
		t.Fatalf("token not stable after the write: %q then %q", after, again)
	}
}

func TestChangeTokenCrossApp(t *testing.T) {
	a := apptest.Open(t)
	b := apptest.Reopen(t, a.DBPath())
	newInv(t, a, "T")
	before := token(t, a)
	if _, err := b.UpdateInvestigation(context.Background(), apptest.Human, "INV-0001", app.InvestigationPatch{Title: ptr("Changed by B")}); err != nil {
		t.Fatal(err)
	}
	if after := token(t, a); after == before {
		t.Fatalf("App A token unchanged after a write through App B: %q", after)
	}
}

func TestChangeTokenRawWriteWithoutActivity(t *testing.T) {
	// A commit by another connection that writes no activity row (another
	// tool, a restore) still moves the token through data_version.
	a := apptest.Open(t)
	newInv(t, a, "T")
	before := token(t, a)
	db := rawDB(t, a.DBPath(), true)
	if _, err := db.Exec(`UPDATE investigations SET updated_at = '2030-01-01T00:00:00.000Z'`); err != nil {
		t.Fatal(err)
	}
	if after := token(t, a); after == before {
		t.Fatalf("token unchanged after an external commit: %q", after)
	}
}

func TestChangeTokenCloseReleasesConn(t *testing.T) {
	a := apptest.Open(t)
	path := a.DBPath()
	token(t, a)
	if err := a.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := a.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	if _, err := a.ChangeToken(context.Background()); err == nil {
		t.Fatal("ChangeToken after Close succeeded")
	}
	b := apptest.Reopen(t, path)
	token(t, b)
	newInv(t, b, "After reopen")
}

func TestStatusReportsDatabase(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t)
	st, err := a.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if st.DBPath != a.DBPath() || st.DataDir != filepath.Dir(a.DBPath()) {
		t.Fatalf("paths: %+v", st)
	}
	if st.SchemaVersion != store.LatestVersion() || st.SchemaVersion < 1 {
		t.Fatalf("SchemaVersion = %d, want %d", st.SchemaVersion, store.LatestVersion())
	}
	if st.ActiveInvestigationRef != "" || st.Counts != (app.Counts{}) {
		t.Fatalf("empty database status = %+v", st)
	}
	apptest.AssertNoJSONNulls(t, st)
}

func TestStatusCountsAndActive(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t)
	inv := newInv(t, a, "One")
	newInv(t, a, "Two")
	for _, title := range []string{"A", "B", "C"} {
		mustAddEvidence(t, a, apptest.AI, inv.Ref, evInput(title))
	}
	if _, err := a.SetActiveInvestigation(ctx, apptest.Human, inv.Ref); err != nil {
		t.Fatal(err)
	}
	// Claims, hypotheses, questions and links have no app methods yet in
	// this plan; seed them directly so every count is exercised.
	db := rawDB(t, a.DBPath(), true)
	const ts = "'2026-01-15T09:00:00.000Z'"
	for _, stmt := range []string{
		`INSERT INTO claims (id, investigation_id, ref_num, statement, verification, created_at, updated_at, created_by_kind, created_by_client)
		 VALUES (1, 1, 1, 'c1', 'ai_generated', ` + ts + `, ` + ts + `, 'ai', 'x'), (2, 1, 2, 'c2', 'ai_generated', ` + ts + `, ` + ts + `, 'ai', 'x')`,
		`INSERT INTO hypotheses (id, investigation_id, ref_num, statement, verification, created_at, updated_at, created_by_kind, created_by_client)
		 VALUES (1, 1, 1, 'h1', 'ai_generated', ` + ts + `, ` + ts + `, 'ai', 'x')`,
		`INSERT INTO questions (investigation_id, ref_num, question, created_at, updated_at, created_by_kind, created_by_client)
		 VALUES (1, 1, 'q1', ` + ts + `, ` + ts + `, 'ai', 'x'), (1, 2, 'q2', ` + ts + `, ` + ts + `, 'ai', 'x'), (2, 1, 'q3', ` + ts + `, ` + ts + `, 'ai', 'x')`,
		`INSERT INTO claim_evidence (claim_id, evidence_id, relation, created_at, created_by_kind, created_by_client)
		 VALUES (1, 1, 'supports', ` + ts + `, 'ai', 'x'), (1, 2, 'supports', ` + ts + `, 'ai', 'x'), (2, 3, 'contradicts', ` + ts + `, 'ai', 'x')`,
		`INSERT INTO hypothesis_evidence (hypothesis_id, evidence_id, relation, created_at, created_by_kind, created_by_client)
		 VALUES (1, 1, 'supports', ` + ts + `, 'ai', 'x')`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	st, err := a.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	want := app.Counts{Investigations: 2, Evidence: 3, Claims: 2, Hypotheses: 1, Questions: 3,
		ClaimLinks: 3, HypothesisLinks: 1, Activity: 6}
	if st.Counts != want {
		t.Fatalf("Counts = %+v, want %+v", st.Counts, want)
	}
	if st.ActiveInvestigationRef != "INV-0001" {
		t.Fatalf("ActiveInvestigationRef = %q", st.ActiveInvestigationRef)
	}
	if err := a.ClearActiveInvestigation(ctx, apptest.Human); err != nil {
		t.Fatal(err)
	}
	st, err = a.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if st.ActiveInvestigationRef != "" || st.Counts.Activity != 7 {
		t.Fatalf("after clear: active %q activity %d", st.ActiveInvestigationRef, st.Counts.Activity)
	}
}

func TestCheckIntegrityHealthy(t *testing.T) {
	a := apptest.Open(t)
	inv := newInv(t, a, "T")
	mustAddEvidence(t, a, apptest.AI, inv.Ref, evInput("E"))
	if err := a.CheckIntegrity(context.Background()); err != nil {
		t.Fatalf("CheckIntegrity on a healthy database: %v", err)
	}
}

func TestCheckIntegrityForeignKeyViolation(t *testing.T) {
	a := apptest.Open(t)
	newInv(t, a, "T")
	db := rawDB(t, a.DBPath(), false)
	if _, err := db.Exec(`INSERT INTO claim_evidence (claim_id, evidence_id, relation, created_at, created_by_kind, created_by_client)
VALUES (777, 888, 'supports', '2026-01-15T09:00:00.000Z', 'ai', 'secret-client-value')`); err != nil {
		t.Fatalf("inject dangling link: %v", err)
	}
	err := a.CheckIntegrity(context.Background())
	if err == nil {
		t.Fatal("CheckIntegrity returned nil with a dangling claim_evidence row")
	}
	msg := err.Error()
	for _, want := range []string{"integrity check failed", "foreign key violation in claim_evidence", "referencing claims", "referencing evidence"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error %q does not mention %q", msg, want)
		}
	}
	if strings.Contains(msg, "secret-client-value") || strings.Contains(msg, "777") {
		t.Errorf("error %q leaks column values", msg)
	}
	if !errors.Is(err, app.ErrInternal) {
		t.Errorf("CheckIntegrity error %v does not wrap ErrInternal", err)
	}
	ie, ok := app.IsIntegrity(err)
	if !ok || ie.Total != 2 || len(ie.Problems) != 2 {
		t.Fatalf("IsIntegrity = %+v, %v; want 2 problems", ie, ok)
	}
	for _, p := range ie.Problems {
		if !strings.HasPrefix(p, "foreign key violation in claim_evidence (rowid ") {
			t.Errorf("problem %q is not a sanitized foreign key line", p)
		}
	}
}
