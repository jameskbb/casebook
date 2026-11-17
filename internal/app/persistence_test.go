package app_test

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/app/apptest"
)

// dbSnapshot is everything a caller can read about one investigation.
type dbSnapshot struct {
	Investigation app.Investigation
	Active        *app.Investigation
	Evidence      []app.Evidence
	RawE001       app.Evidence
	Claims        []app.Claim
	Hypotheses    []app.Hypothesis
	Questions     []app.Question
	Activity      []app.Activity
	Counts        app.Counts
}

func snapshotOf(t *testing.T, a *app.App, invRef string) dbSnapshot {
	t.Helper()
	ctx := context.Background()
	var (
		s   dbSnapshot
		err error
	)
	check := func(what string, err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("%s: %v", what, err)
		}
	}
	s.Investigation, err = a.GetInvestigation(ctx, invRef)
	check("GetInvestigation", err)
	s.Active, err = a.GetActiveInvestigation(ctx)
	check("GetActiveInvestigation", err)
	s.Evidence, err = a.ListEvidence(ctx, invRef)
	check("ListEvidence", err)
	s.RawE001, err = a.GetEvidence(ctx, invRef, "E-001", app.GetEvidenceOptions{IncludeRawDetail: true})
	check("GetEvidence", err)
	s.Claims, err = a.ListClaims(ctx, invRef)
	check("ListClaims", err)
	s.Hypotheses, err = a.ListHypotheses(ctx, invRef)
	check("ListHypotheses", err)
	s.Questions, err = a.ListQuestions(ctx, invRef)
	check("ListQuestions", err)
	s.Activity, err = a.ListActivity(ctx, app.ActivityFilter{Limit: 500})
	check("ListActivity", err)
	st, err := a.Status(ctx)
	check("Status", err)
	s.Counts = st.Counts
	return s
}

func TestPersistenceRoundTrip(t *testing.T) {
	a := apptest.Open(t)
	path := a.DBPath()
	f := apptest.SeedCloudSpend(t, a)
	before := snapshotOf(t, a, f.InvestigationRef)
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}

	b := apptest.Reopen(t, path)
	after := snapshotOf(t, b, f.InvestigationRef)
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("state changed across close and reopen:\nbefore %+v\nafter  %+v", before, after)
	}
	if after.Active == nil || after.Active.Ref != f.InvestigationRef {
		t.Fatalf("active investigation after reopen = %+v", after.Active)
	}
	if len(after.Claims) != 3 || len(after.Claims[0].Links) != 2 || after.RawE001.RawDetail == "" ||
		after.Counts.Activity != len(after.Activity) {
		t.Fatalf("reopened data = %+v", after)
	}
}

func TestPersistenceRelationships(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t)
	f := apptest.SeedCloudSpend(t, a)

	// Claim to evidence.
	c1, err := a.GetClaim(ctx, f.InvestigationRef, "C-001")
	if err != nil {
		t.Fatal(err)
	}
	if len(c1.Links) != 2 || c1.Links[0].EvidenceRef != "E-001" || c1.Links[0].EvidenceTitle != "Service chart default log level set to debug" ||
		c1.Links[1].EvidenceRef != "E-004" {
		t.Fatalf("C-001 links = %+v", c1.Links)
	}
	// Evidence to claims and hypotheses.
	e3, err := a.GetEvidence(ctx, f.InvestigationRef, "E-003", app.GetEvidenceOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(e3.ClaimLinks, []app.RecordLink{
		{Ref: "C-002", Relation: app.RelationSupports}, {Ref: "C-003", Relation: app.RelationContradicts}}) ||
		!reflect.DeepEqual(e3.HypothesisLinks, []app.RecordLink{{Ref: "H-002", Relation: app.RelationContradicts}}) {
		t.Fatalf("E-003 links = %+v %+v", e3.ClaimLinks, e3.HypothesisLinks)
	}
	// Superseded claim to its replacement.
	c3, err := a.GetClaim(ctx, f.InvestigationRef, f.SupersededClaimRef)
	if err != nil || c3.SupersededBy != "C-002" {
		t.Fatalf("C-003 = %+v, %v", c3, err)
	}
	// Unlinking one side is visible from the other.
	if _, err := a.UnlinkEvidenceFromClaim(ctx, apptest.Human, f.InvestigationRef, "C-003", "E-003"); err != nil {
		t.Fatal(err)
	}
	e3, err = a.GetEvidence(ctx, f.InvestigationRef, "E-003", app.GetEvidenceOptions{})
	if err != nil || len(e3.ClaimLinks) != 1 || e3.ClaimLinks[0].Ref != "C-002" {
		t.Fatalf("E-003 claim links after unlink = %+v, %v", e3.ClaimLinks, err)
	}
}

func TestPersistenceCrossInvestigationTriggers(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t)
	f := apptest.SeedCloudSpend(t, a)
	other := newInv(t, a, "Other")
	mustAddEvidence(t, a, apptest.AI, other.Ref, evInput("Foreign"))
	mustAddClaim(t, a, apptest.AI, other.Ref, app.ClaimInput{Statement: "Foreign claim"})
	mustAddHypothesis(t, a, apptest.AI, other.Ref, app.HypothesisInput{Statement: "Foreign hypothesis"})

	db := rawDB(t, a.DBPath(), true)
	id := func(table string, invRef string, num int) int64 {
		t.Helper()
		n, err := app.ParseInvestigationRef(invRef)
		if err != nil {
			t.Fatal(err)
		}
		var v int64
		// table is a fixed literal from this test, never input.
		err = db.QueryRowContext(ctx, `SELECT r.id FROM `+table+` r JOIN investigations i ON i.id = r.investigation_id
WHERE i.ref_num = ? AND r.ref_num = ?`, n, num).Scan(&v)
		if err != nil {
			t.Fatalf("lookup %s %s/%d: %v", table, invRef, num, err)
		}
		return v
	}
	claim1 := id("claims", f.InvestigationRef, 1)
	hyp1 := id("hypotheses", f.InvestigationRef, 1)
	foreignEv := id("evidence", other.Ref, 1)
	foreignClaim := id("claims", other.Ref, 1)
	foreignHyp := id("hypotheses", other.Ref, 1)

	for _, tc := range []struct {
		name, sql string
		args      []any
		want      string
	}{
		{"claim link", `INSERT INTO claim_evidence (claim_id, evidence_id, relation, created_at, created_by_kind, created_by_client)
VALUES (?, ?, 'supports', '2026-01-01T00:00:00.000Z', 'human', 'raw')`, []any{claim1, foreignEv}, "cross-investigation link"},
		{"hypothesis link", `INSERT INTO hypothesis_evidence (hypothesis_id, evidence_id, relation, created_at, created_by_kind, created_by_client)
VALUES (?, ?, 'supports', '2026-01-01T00:00:00.000Z', 'human', 'raw')`, []any{hyp1, foreignEv}, "cross-investigation link"},
		{"claim supersede", `UPDATE claims SET superseded_by_id = ? WHERE id = ?`, []any{foreignClaim, claim1}, "invalid supersede"},
		{"hypothesis supersede", `UPDATE hypotheses SET superseded_by_id = ? WHERE id = ?`, []any{foreignHyp, hyp1}, "invalid supersede"},
		{"claim self supersede", `UPDATE claims SET superseded_by_id = id WHERE id = ?`, []any{claim1}, "invalid supersede"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := db.ExecContext(ctx, tc.sql, tc.args...)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("raw %s err = %v, want %q", tc.name, err, tc.want)
			}
		})
	}
	// Nothing changed.
	c1, err := a.GetClaim(ctx, f.InvestigationRef, "C-001")
	if err != nil || len(c1.Links) != 2 || c1.SupersededBy != "" {
		t.Fatalf("C-001 after rejected raw writes = %+v, %v", c1, err)
	}
	if err := a.CheckIntegrity(ctx); err != nil {
		t.Fatalf("CheckIntegrity: %v", err)
	}
}

func TestPersistenceArchive(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t)
	path := a.DBPath()
	f := apptest.SeedCloudSpend(t, a)
	before := snapshotOf(t, a, f.InvestigationRef)

	inv, err := a.ArchiveInvestigation(ctx, apptest.Human, f.InvestigationRef)
	if err != nil || !inv.Archived || inv.Status != app.InvestigationStatusArchived || !inv.IsActive {
		t.Fatalf("archived = %+v, %v", inv, err)
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	b := apptest.Reopen(t, path)
	archived := snapshotOf(t, b, f.InvestigationRef)
	if !archived.Investigation.Archived || archived.Active == nil || archived.Active.Ref != f.InvestigationRef {
		t.Fatalf("archived after reopen = %+v", archived.Investigation)
	}
	// Every record stays readable and unchanged.
	if !reflect.DeepEqual(before.Evidence, archived.Evidence) || !reflect.DeepEqual(before.Claims, archived.Claims) ||
		!reflect.DeepEqual(before.Hypotheses, archived.Hypotheses) || !reflect.DeepEqual(before.Questions, archived.Questions) ||
		!reflect.DeepEqual(before.RawE001, archived.RawE001) {
		t.Fatal("archiving changed or hid records")
	}
	c3, err := b.GetClaim(ctx, f.InvestigationRef, "C-003")
	if err != nil || c3.Status != app.ClaimStatusSuperseded {
		t.Fatalf("archived C-003 = %+v, %v", c3, err)
	}

	inv, err = b.UnarchiveInvestigation(ctx, apptest.Human, f.InvestigationRef)
	if err != nil || inv.Archived || inv.Status != app.InvestigationStatusInvestigating {
		t.Fatalf("unarchived = %+v, %v", inv, err)
	}
	want := before.Counts
	want.Activity += 2 // investigation.archive and investigation.unarchive
	if after := snapshotOf(t, b, f.InvestigationRef); after.Counts != want {
		t.Fatalf("counts after archive and unarchive = %+v, before %+v", after.Counts, before.Counts)
	}
}

func TestRollbackAddClaimMidTransaction(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t)
	f := apptest.SeedCloudSpend(t, a)
	before := snapshotOf(t, a, f.InvestigationRef)

	// The claim row and its first link are written before E-099 fails.
	_, err := a.AddClaim(ctx, apptest.AI, f.InvestigationRef, app.ClaimInput{
		Statement: "Partial claim",
		Links: []app.EvidenceLinkInput{
			{EvidenceRef: "E-001", Relation: app.RelationSupports},
			{EvidenceRef: "E-099", Relation: app.RelationSupports},
		},
	})
	wantErrIs(t, err, app.ErrNotFound)

	after := snapshotOf(t, a, f.InvestigationRef)
	if len(after.Claims) != len(before.Claims) || len(after.Activity) != len(before.Activity) || after.Counts != before.Counts {
		t.Fatalf("partial write: claims %d->%d, activity %d->%d, counts %+v->%+v", len(before.Claims), len(after.Claims),
			len(before.Activity), len(after.Activity), before.Counts, after.Counts)
	}
	if !reflect.DeepEqual(before.Evidence, after.Evidence) || !reflect.DeepEqual(before.Investigation, after.Investigation) {
		t.Fatal("failed AddClaim changed evidence links or the investigation")
	}
	var links int
	if err := rawDB(t, a.DBPath(), true).QueryRowContext(ctx, `SELECT count(*) FROM claim_evidence`).Scan(&links); err != nil {
		t.Fatal(err)
	}
	if links != 5 {
		t.Fatalf("claim_evidence rows = %d, want the fixture's 5", links)
	}
	// The ref was not consumed.
	c := mustAddClaim(t, a, apptest.AI, f.InvestigationRef, app.ClaimInput{Statement: "Next"})
	if c.Ref != "C-004" {
		t.Fatalf("next claim ref = %s, want C-004", c.Ref)
	}

	// The same holds for hypotheses.
	_, err = a.AddHypothesis(ctx, apptest.AI, f.InvestigationRef, app.HypothesisInput{Statement: "Partial",
		Links: []app.EvidenceLinkInput{{EvidenceRef: "E-002", Relation: app.RelationSupports},
			{EvidenceRef: "INV-0002/E-001", Relation: app.RelationSupports}}})
	wantErrIs(t, err, app.ErrConflict)
	h := mustAddHypothesis(t, a, apptest.AI, f.InvestigationRef, app.HypothesisInput{Statement: "Next"})
	if h.Ref != "H-003" {
		t.Fatalf("next hypothesis ref = %s, want H-003", h.Ref)
	}
}
