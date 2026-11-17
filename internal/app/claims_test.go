package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/app/apptest"
)

// lastActivity returns the newest activity row of invRef.
func lastActivity(t *testing.T, a *app.App, invRef string) app.Activity {
	t.Helper()
	rows, err := a.ListActivity(context.Background(), app.ActivityFilter{InvestigationRef: invRef, Limit: 1})
	if err != nil || len(rows) != 1 {
		t.Fatalf("ListActivity: %v (%d rows)", err, len(rows))
	}
	return rows[0]
}

// activityCount returns how many activity rows invRef has.
func activityCount(t *testing.T, a *app.App, invRef string) int {
	t.Helper()
	return len(actions(t, a, invRef))
}

// wantLast asserts the newest activity row of invRef.
func wantLast(t *testing.T, a *app.App, invRef, action, target, summary string) {
	t.Helper()
	row := lastActivity(t, a, invRef)
	if row.Action != action || row.TargetRef != target || row.Summary != summary {
		t.Fatalf("last activity = %s %s %q, want %s %s %q", row.Action, row.TargetRef, row.Summary, action, target, summary)
	}
}

// seedEvidence adds n evidence rows titled "Evidence 1".. to invRef.
func seedEvidence(t *testing.T, a *app.App, invRef string, n int) []app.Evidence {
	t.Helper()
	out := make([]app.Evidence, 0, n)
	for i := 1; i <= n; i++ {
		in := evInput("Evidence " + string(rune('0'+i)))
		out = append(out, mustAddEvidence(t, a, apptest.AI, invRef, in))
	}
	return out
}

func mustAddClaim(t *testing.T, a *app.App, actor app.Actor, invRef string, in app.ClaimInput) app.Claim {
	t.Helper()
	c, err := a.AddClaim(context.Background(), actor, invRef, in)
	if err != nil {
		t.Fatalf("AddClaim: %v", err)
	}
	return c
}

func wantErrIs(t *testing.T, err, target error) {
	t.Helper()
	if !errors.Is(err, target) {
		t.Fatalf("err = %v, want %v", err, target)
	}
}

// claimEnv is one active investigation with three evidence rows.
func claimEnv(t *testing.T, opts ...app.Option) (*app.App, app.Investigation) {
	t.Helper()
	a := apptest.Open(t, opts...)
	inv, err := a.CreateInvestigation(context.Background(), apptest.Human,
		app.CreateInvestigationInput{Title: "Claims", MakeActive: true})
	if err != nil {
		t.Fatal(err)
	}
	seedEvidence(t, a, inv.Ref, 3)
	return a, inv
}

func TestAddClaimWithLinks(t *testing.T) {
	ctx := context.Background()
	a, inv := claimEnv(t)

	c := mustAddClaim(t, a, apptest.Human, "", app.ClaimInput{
		Statement:           "  The chart default sets every service to debug.  ",
		Confidence:          app.ConfidenceHigh,
		ConfidenceRationale: "Code and interview agree.",
		Links: []app.EvidenceLinkInput{
			{EvidenceRef: "e-2", Relation: app.RelationSupports},
			{EvidenceRef: "E-001", Relation: app.RelationSupports},
			{EvidenceRef: "INV-0001/E-003", Relation: app.RelationContradicts},
			{EvidenceRef: "E-001", Relation: app.RelationSupports}, // duplicate collapses
		},
	})
	if c.Ref != "C-001" || c.InvestigationRef != inv.Ref || c.ID == 0 {
		t.Fatalf("claim = %+v", c)
	}
	if c.Statement != "The chart default sets every service to debug." || c.Confidence != app.ConfidenceHigh ||
		c.ConfidenceRationale != "Code and interview agree." || c.Status != app.ClaimStatusActive ||
		c.Verification != app.VerificationHumanVerified || c.SupersededBy != "" || c.CreatedBy != apptest.Human {
		t.Fatalf("claim fields = %+v", c)
	}
	want := []struct {
		ref, title string
		rel        app.Relation
	}{
		{"E-001", "Evidence 1", app.RelationSupports},
		{"E-002", "Evidence 2", app.RelationSupports},
		{"E-003", "Evidence 3", app.RelationContradicts},
	}
	if len(c.Links) != len(want) {
		t.Fatalf("links = %+v", c.Links)
	}
	for i, w := range want {
		l := c.Links[i]
		if l.EvidenceRef != w.ref || l.EvidenceTitle != w.title || l.Relation != w.rel || l.CreatedBy != apptest.Human ||
			l.CreatedAt != app.FormatTime(apptest.FixedTime) {
			t.Fatalf("links[%d] = %+v, want %s %s %s", i, l, w.ref, w.title, w.rel)
		}
	}
	apptest.AssertNoJSONNulls(t, c)
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"id"`, `"ref"`, `"investigation_ref"`, `"statement"`, `"confidence"`,
		`"confidence_rationale"`, `"status"`, `"verification"`, `"superseded_by":""`, `"created_at"`, `"updated_at"`,
		`"created_by":{"kind":"human"`, `"links":[{"evidence_ref":"E-001","relation":"supports","evidence_title"`} {
		if !strings.Contains(string(b), key) {
			t.Errorf("Claim JSON %s lacks %s", b, key)
		}
	}
	wantLast(t, a, inv.Ref, "claim.add", "C-001", "Added claim C-001 [high] with 3 evidence links")
	// One activity row for the claim and its links.
	wantActions(t, a, inv.Ref, "investigation.create", "active.set",
		"evidence.add", "evidence.add", "evidence.add", "claim.add")

	// Evidence shows the reverse direction.
	e1, err := a.GetEvidence(ctx, "", "E-001", app.GetEvidenceOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(e1.ClaimLinks) != 1 || e1.ClaimLinks[0] != (app.RecordLink{Ref: "C-001", Relation: app.RelationSupports}) {
		t.Fatalf("E-001 claim links = %+v", e1.ClaimLinks)
	}
	e3, err := a.GetEvidence(ctx, "", "E-003", app.GetEvidenceOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(e3.ClaimLinks) != 1 || e3.ClaimLinks[0].Relation != app.RelationContradicts {
		t.Fatalf("E-003 claim links = %+v", e3.ClaimLinks)
	}

	// Defaults for an AI claim without links.
	c2 := mustAddClaim(t, a, apptest.AI, inv.Ref, app.ClaimInput{Statement: "Second"})
	if c2.Ref != "C-002" || c2.Confidence != app.ConfidenceUnknown || c2.Status != app.ClaimStatusActive ||
		c2.Verification != app.VerificationAIGenerated || c2.Links == nil || len(c2.Links) != 0 {
		t.Fatalf("AI claim defaults = %+v", c2)
	}
	wantLast(t, a, inv.Ref, "claim.add", "C-002", "Added claim C-002 [unknown]")
	apptest.AssertNoJSONNulls(t, c2)

	// Get and List agree with what Add returned.
	got, err := a.GetClaim(ctx, "inv-1", "c-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Ref != "C-001" || len(got.Links) != 3 || got.Statement != c.Statement {
		t.Fatalf("GetClaim = %+v", got)
	}
	list, err := a.ListClaims(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].Ref != "C-001" || list[1].Ref != "C-002" || len(list[0].Links) != 3 || list[1].Links == nil {
		t.Fatalf("ListClaims = %+v", list)
	}
	apptest.AssertNoJSONNulls(t, list)

	// Refs are per investigation.
	other := newInv(t, a, "Other")
	oc := mustAddClaim(t, a, apptest.AI, other.Ref, app.ClaimInput{Statement: "Elsewhere"})
	if oc.Ref != "C-001" || oc.InvestigationRef != other.Ref {
		t.Fatalf("other claim = %s in %s", oc.Ref, oc.InvestigationRef)
	}
	empty, err := a.ListClaims(ctx, newInv(t, a, "Empty").Ref)
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("empty ListClaims = %#v, %v", empty, err)
	}
}

func TestAddClaimValidation(t *testing.T) {
	ctx := context.Background()
	a, inv := claimEnv(t)
	before := activityCount(t, a, inv.Ref)
	for _, tc := range []struct {
		name  string
		actor app.Actor
		in    app.ClaimInput
		field string
	}{
		{"missing statement", apptest.AI, app.ClaimInput{Statement: "  \n "}, "statement"},
		{"oversized statement", apptest.AI, app.ClaimInput{Statement: strings.Repeat("x", app.MaxStatementBytes+1)}, "statement"},
		{"invalid confidence", apptest.AI, app.ClaimInput{Statement: "s", Confidence: "certain"}, "confidence"},
		{"numeric confidence", apptest.AI, app.ClaimInput{Statement: "s", Confidence: "0.9"}, "confidence"},
		{"oversized rationale", apptest.AI, app.ClaimInput{Statement: "s",
			ConfidenceRationale: strings.Repeat("r", app.MaxRationaleBytes+1)}, "confidence_rationale"},
		{"invalid status", apptest.AI, app.ClaimInput{Statement: "s", Status: "open"}, "status"},
		{"superseded at creation", apptest.AI, app.ClaimInput{Statement: "s", Status: app.ClaimStatusSuperseded}, "status"},
		{"invalid verification", apptest.AI, app.ClaimInput{Statement: "s", Verification: "trusted"}, "verification"},
		{"invalid relation", apptest.AI, app.ClaimInput{Statement: "s",
			Links: []app.EvidenceLinkInput{{EvidenceRef: "E-001", Relation: "refutes"}}}, "links[0].relation"},
		{"missing relation", apptest.AI, app.ClaimInput{Statement: "s",
			Links: []app.EvidenceLinkInput{{EvidenceRef: "E-001"}}}, "links[0].relation"},
		{"wrong prefix link ref", apptest.AI, app.ClaimInput{Statement: "s",
			Links: []app.EvidenceLinkInput{{EvidenceRef: "C-001", Relation: app.RelationSupports}}}, "links[0].evidence_ref"},
		{"conflicting duplicate link", apptest.AI, app.ClaimInput{Statement: "s", Links: []app.EvidenceLinkInput{
			{EvidenceRef: "E-001", Relation: app.RelationSupports},
			{EvidenceRef: "e-1", Relation: app.RelationContradicts},
		}}, "links[1].relation"},
		{"bad actor", app.Actor{Kind: "robot", Client: "x"}, app.ClaimInput{Statement: "s"}, "actor.kind"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := a.AddClaim(ctx, tc.actor, "", tc.in)
			wantValidation(t, err, tc.field)
		})
	}
	list, err := a.ListClaims(ctx, "")
	if err != nil || len(list) != 0 {
		t.Fatalf("claims after rejected adds = %+v, %v", list, err)
	}
	if n := activityCount(t, a, inv.Ref); n != before {
		t.Fatalf("activity rows %d, want %d", n, before)
	}
	// The failed adds consumed no ref.
	if c := mustAddClaim(t, a, apptest.AI, "", app.ClaimInput{Statement: "ok"}); c.Ref != "C-001" {
		t.Fatalf("first successful claim ref = %s", c.Ref)
	}
}

func TestAddClaimCrossInvestigationLinks(t *testing.T) {
	ctx := context.Background()
	a, inv := claimEnv(t) // INV-0001 with E-001..E-003
	other := newInv(t, a, "Other")
	seedEvidence(t, a, other.Ref, 4) // INV-0002 has E-004; INV-0001 does not

	_, err := a.AddClaim(ctx, apptest.AI, inv.Ref, app.ClaimInput{Statement: "s",
		Links: []app.EvidenceLinkInput{{EvidenceRef: "INV-0002/E-001", Relation: app.RelationSupports}}})
	wantErrIs(t, err, app.ErrConflict)
	if !strings.Contains(err.Error(), "cross-investigation link") {
		t.Fatalf("conflict message = %q", err)
	}
	_, err = a.AddClaim(ctx, apptest.AI, inv.Ref, app.ClaimInput{Statement: "s",
		Links: []app.EvidenceLinkInput{{EvidenceRef: "E-004", Relation: app.RelationSupports}}})
	wantErrIs(t, err, app.ErrNotFound)
	if !strings.Contains(err.Error(), "evidence E-004 in INV-0001") {
		t.Fatalf("not found message = %q", err)
	}
	if list, _ := a.ListClaims(ctx, inv.Ref); len(list) != 0 {
		t.Fatalf("claims written despite bad links: %+v", list)
	}

	c := mustAddClaim(t, a, apptest.AI, inv.Ref, app.ClaimInput{Statement: "s"})
	_, err = a.LinkEvidenceToClaim(ctx, apptest.AI, inv.Ref, c.Ref, "INV-0002/E-002", app.RelationSupports)
	wantErrIs(t, err, app.ErrConflict)
	_, err = a.LinkEvidenceToClaim(ctx, apptest.AI, inv.Ref, c.Ref, "E-004", app.RelationSupports)
	wantErrIs(t, err, app.ErrNotFound)
	// A qualified ref to the claim's own investigation is fine.
	got, err := a.LinkEvidenceToClaim(ctx, apptest.AI, inv.Ref, c.Ref, "INV-0001/E-002", app.RelationSupports)
	if err != nil || len(got.Links) != 1 || got.Links[0].EvidenceRef != "E-002" {
		t.Fatalf("same-investigation qualified link = %+v, %v", got.Links, err)
	}
}

func TestAddClaimRefErrors(t *testing.T) {
	ctx := context.Background()
	a, _ := claimEnv(t)
	_, err := a.GetClaim(ctx, "", "E-001")
	wantValidation(t, err, "ref")
	_, err = a.GetClaim(ctx, "", "C-009")
	wantErrIs(t, err, app.ErrNotFound)
	if !strings.Contains(err.Error(), "claim C-009 in INV-0001") {
		t.Fatalf("not found message = %q", err)
	}
	_, err = a.UpdateClaim(ctx, apptest.AI, "", "H-001", app.ClaimPatch{Statement: ptr("x")})
	wantValidation(t, err, "ref")
	_, err = a.ListClaims(ctx, "INV-0042")
	wantErrIs(t, err, app.ErrNotFound)

	b := apptest.Open(t)
	_, err = b.AddClaim(ctx, apptest.AI, "", app.ClaimInput{Statement: "s"})
	wantErrIs(t, err, app.ErrNoActiveInvestigation)
}

func TestUpdateClaimFields(t *testing.T) {
	ctx := context.Background()
	a, inv := claimEnv(t, app.WithClock(apptest.StepClock(apptest.FixedTime, time.Second)))
	c := mustAddClaim(t, a, apptest.AI, "", app.ClaimInput{Statement: "Old", Confidence: app.ConfidenceHigh})

	got, err := a.UpdateClaim(ctx, apptest.Human, "", "C-001", app.ClaimPatch{
		Confidence:          ptr(app.ConfidenceMedium),
		ConfidenceRationale: ptr("Only one source so far."),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Confidence != app.ConfidenceMedium || got.ConfidenceRationale != "Only one source so far." ||
		got.Statement != "Old" || got.Verification != app.VerificationAIGenerated {
		t.Fatalf("updated claim = %+v", got)
	}
	if got.UpdatedAt <= c.UpdatedAt || got.CreatedAt != c.CreatedAt || got.CreatedBy != apptest.AI {
		t.Fatalf("timestamps/created_by: before %+v after %+v", c, got)
	}
	wantLast(t, a, inv.Ref, "claim.update", "C-001", "Updated claim C-001: confidence medium, confidence_rationale")

	// No-op patches write nothing.
	n := activityCount(t, a, inv.Ref)
	same, err := a.UpdateClaim(ctx, apptest.Human, "", "C-001", app.ClaimPatch{
		Statement: ptr(" Old "), Confidence: ptr(app.ConfidenceMedium),
	})
	if err != nil || same.UpdatedAt != got.UpdatedAt {
		t.Fatalf("no-op patch = %+v, %v", same, err)
	}
	if _, err := a.UpdateClaim(ctx, apptest.Human, "", "C-001", app.ClaimPatch{}); err != nil {
		t.Fatal(err)
	}
	if activityCount(t, a, inv.Ref) != n {
		t.Fatal("no-op patch wrote activity")
	}

	// Status and statement.
	got, err = a.UpdateClaim(ctx, apptest.Human, "", "C-001", app.ClaimPatch{
		Statement: ptr("New"), Status: ptr(app.ClaimStatusDisputed),
	})
	if err != nil || got.Statement != "New" || got.Status != app.ClaimStatusDisputed ||
		got.Verification != app.VerificationDisputed {
		t.Fatalf("statement/status patch = %+v, %v", got, err)
	}
	wantLast(t, a, inv.Ref, "claim.update", "C-001", "Updated claim C-001: statement, status disputed, verification disputed")

	// Patch validation.
	for _, tc := range []struct {
		p     app.ClaimPatch
		field string
	}{
		{app.ClaimPatch{Statement: ptr("")}, "statement"},
		{app.ClaimPatch{Confidence: ptr(app.Confidence("0.7"))}, "confidence"},
		{app.ClaimPatch{Status: ptr(app.ClaimStatus("open"))}, "status"},
		{app.ClaimPatch{Verification: ptr(app.Verification("yes"))}, "verification"},
		{app.ClaimPatch{SupersededBy: ptr("E-001")}, "superseded_by"},
	} {
		_, err := a.UpdateClaim(ctx, apptest.Human, "", "C-001", tc.p)
		wantValidation(t, err, tc.field)
	}
}

func TestUpdateClaimSupersedeRules(t *testing.T) {
	ctx := context.Background()
	a, inv := claimEnv(t)
	mustAddClaim(t, a, apptest.Human, "", app.ClaimInput{Statement: "One"})
	mustAddClaim(t, a, apptest.Human, "", app.ClaimInput{Statement: "Two"})

	// Status superseded needs a replacement.
	_, err := a.UpdateClaim(ctx, apptest.Human, "", "C-001", app.ClaimPatch{Status: ptr(app.ClaimStatusSuperseded)})
	wantValidation(t, err, "superseded_by")
	// SupersededBy with another status is contradictory.
	_, err = a.UpdateClaim(ctx, apptest.Human, "", "C-001", app.ClaimPatch{
		SupersededBy: ptr("C-002"), Status: ptr(app.ClaimStatusActive)})
	wantValidation(t, err, "status")
	// Self replacement through the patch is a conflict, as with SupersedeClaim.
	_, err = a.UpdateClaim(ctx, apptest.Human, "", "C-001", app.ClaimPatch{SupersededBy: ptr("c-1")})
	wantErrIs(t, err, app.ErrConflict)

	got, err := a.UpdateClaim(ctx, apptest.Human, "", "C-001", app.ClaimPatch{
		SupersededBy: ptr("C-002"), Statement: ptr("One, revised")})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != app.ClaimStatusSuperseded || got.Verification != app.VerificationSuperseded ||
		got.SupersededBy != "C-002" || got.Statement != "One, revised" {
		t.Fatalf("superseded via patch = %+v", got)
	}
	wantLast(t, a, inv.Ref, "claim.supersede", "C-001", "C-001 superseded by C-002; also updated statement")

	// Status superseded again is accepted (it already has a replacement) and
	// changes nothing.
	n := activityCount(t, a, inv.Ref)
	if _, err := a.UpdateClaim(ctx, apptest.Human, "", "C-001", app.ClaimPatch{Status: ptr(app.ClaimStatusSuperseded)}); err != nil {
		t.Fatal(err)
	}
	if activityCount(t, a, inv.Ref) != n {
		t.Fatal("repeating status superseded wrote activity")
	}

	// Leaving superseded clears the replacement and re-asserts verification
	// for the acting actor.
	got, err = a.UpdateClaim(ctx, apptest.AI, "", "C-001", app.ClaimPatch{Status: ptr(app.ClaimStatusActive)})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != app.ClaimStatusActive || got.SupersededBy != "" || got.Verification != app.VerificationAIGenerated {
		t.Fatalf("reactivated claim = %+v", got)
	}
	wantLast(t, a, inv.Ref, "claim.update", "C-001", "Updated claim C-001: status active, superseded_by, verification ai_generated")

	// An explicit verification wins over the default when leaving superseded.
	if _, err := a.SupersedeClaim(ctx, apptest.Human, "", "C-001", "C-002"); err != nil {
		t.Fatal(err)
	}
	got, err = a.UpdateClaim(ctx, apptest.AI, "", "C-001", app.ClaimPatch{
		Status: ptr(app.ClaimStatusRejected), Verification: ptr(app.VerificationSourceVerified)})
	if err != nil || got.Verification != app.VerificationSourceVerified || got.SupersededBy != "" {
		t.Fatalf("rejected claim = %+v, %v", got, err)
	}
}

func TestSupersedeClaim(t *testing.T) {
	ctx := context.Background()
	a, inv := claimEnv(t)
	for _, s := range []string{"One", "Two", "Three"} {
		mustAddClaim(t, a, apptest.AI, "", app.ClaimInput{Statement: s, Confidence: app.ConfidenceLow})
	}
	other := newInv(t, a, "Other")
	mustAddClaim(t, a, apptest.AI, other.Ref, app.ClaimInput{Statement: "Elsewhere"})

	got, err := a.SupersedeClaim(ctx, apptest.Human, "", "C-003", "c-2")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != app.ClaimStatusSuperseded || got.Verification != app.VerificationSuperseded ||
		got.SupersededBy != "C-002" || got.Statement != "Three" || got.Confidence != app.ConfidenceLow {
		t.Fatalf("superseded claim = %+v", got)
	}
	wantLast(t, a, inv.Ref, "claim.supersede", "C-003", "C-003 superseded by C-002")
	apptest.AssertNoJSONNulls(t, got)

	n := activityCount(t, a, inv.Ref)
	again, err := a.SupersedeClaim(ctx, apptest.Human, "", "C-003", "C-002")
	if err != nil || again.SupersededBy != "C-002" {
		t.Fatalf("repeat supersede = %+v, %v", again, err)
	}
	if activityCount(t, a, inv.Ref) != n {
		t.Fatal("repeat supersede wrote activity")
	}

	for _, tc := range []struct {
		name, claim, replacement string
		want                     error
	}{
		{"self", "C-001", "C-001", app.ErrConflict},
		{"replacement already superseded", "C-001", "C-003", app.ErrConflict},
		{"cycle", "C-002", "C-003", app.ErrConflict},
		{"cross-investigation replacement", "C-001", "INV-0002/C-001", app.ErrConflict},
		{"missing replacement", "C-001", "C-009", app.ErrNotFound},
		{"missing claim", "C-009", "C-001", app.ErrNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := a.SupersedeClaim(ctx, apptest.Human, "", tc.claim, tc.replacement)
			wantErrIs(t, err, tc.want)
		})
	}
	_, err = a.SupersedeClaim(ctx, apptest.Human, "", "C-001", "H-001")
	wantValidation(t, err, "replacement_ref")
	if activityCount(t, a, inv.Ref) != n {
		t.Fatal("rejected supersedes wrote activity")
	}

	// Re-pointing a superseded claim at a different current replacement.
	got, err = a.SupersedeClaim(ctx, apptest.Human, "", "C-003", "C-001")
	if err != nil || got.SupersededBy != "C-001" {
		t.Fatalf("re-pointed supersede = %+v, %v", got, err)
	}
	wantLast(t, a, inv.Ref, "claim.supersede", "C-003", "C-003 superseded by C-001")
}

func TestSetClaimVerification(t *testing.T) {
	ctx := context.Background()
	a, inv := claimEnv(t)
	mustAddClaim(t, a, apptest.AI, "", app.ClaimInput{Statement: "s"})
	got, err := a.SetClaimVerification(ctx, apptest.Human, "", "C-001", app.VerificationSourceVerified)
	if err != nil || got.Verification != app.VerificationSourceVerified {
		t.Fatalf("SetClaimVerification = %+v, %v", got, err)
	}
	wantLast(t, a, inv.Ref, "claim.update", "C-001", "Set verification of C-001 to source_verified")
	n := activityCount(t, a, inv.Ref)
	if _, err := a.SetClaimVerification(ctx, apptest.Human, "", "C-001", app.VerificationSourceVerified); err != nil {
		t.Fatal(err)
	}
	if activityCount(t, a, inv.Ref) != n {
		t.Fatal("same verification wrote activity")
	}
	_, err = a.SetClaimVerification(ctx, apptest.Human, "", "C-001", "")
	wantValidation(t, err, "verification")
	_, err = a.SetClaimVerification(ctx, apptest.Human, "", "C-002", app.VerificationDisputed)
	wantErrIs(t, err, app.ErrNotFound)
}

func TestLinkEvidenceToClaim(t *testing.T) {
	ctx := context.Background()
	a, inv := claimEnv(t, app.WithClock(apptest.StepClock(apptest.FixedTime, time.Second)))
	c := mustAddClaim(t, a, apptest.AI, "", app.ClaimInput{Statement: "s"})

	got, err := a.LinkEvidenceToClaim(ctx, apptest.Human, "", "C-001", "e-2", app.RelationSupports)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Links) != 1 || got.Links[0].EvidenceRef != "E-002" || got.Links[0].Relation != app.RelationSupports ||
		got.Links[0].EvidenceTitle != "Evidence 2" || got.Links[0].CreatedBy != apptest.Human {
		t.Fatalf("links after link = %+v", got.Links)
	}
	if got.UpdatedAt <= c.UpdatedAt {
		t.Fatalf("linking did not touch the claim: %s then %s", c.UpdatedAt, got.UpdatedAt)
	}
	wantLast(t, a, inv.Ref, "link.add", "C-001", "Linked E-002 to C-001 (supports)")

	// Same relation: idempotent, no activity row.
	n := activityCount(t, a, inv.Ref)
	same, err := a.LinkEvidenceToClaim(ctx, apptest.Human, "", "C-001", "E-002", app.RelationSupports)
	if err != nil || len(same.Links) != 1 || same.UpdatedAt != got.UpdatedAt {
		t.Fatalf("idempotent relink = %+v, %v", same, err)
	}
	if activityCount(t, a, inv.Ref) != n {
		t.Fatal("idempotent relink wrote activity")
	}

	// Different relation: updates and writes link.update.
	got, err = a.LinkEvidenceToClaim(ctx, apptest.AI, "", "C-001", "E-002", app.RelationContradicts)
	if err != nil || len(got.Links) != 1 || got.Links[0].Relation != app.RelationContradicts {
		t.Fatalf("relink = %+v, %v", got.Links, err)
	}
	wantLast(t, a, inv.Ref, "link.update", "C-001", "Changed link E-002 to C-001 from supports to contradicts")
	ev, err := a.GetEvidence(ctx, "", "E-002", app.GetEvidenceOptions{})
	if err != nil || len(ev.ClaimLinks) != 1 || ev.ClaimLinks[0].Relation != app.RelationContradicts {
		t.Fatalf("E-002 claim links = %+v, %v", ev.ClaimLinks, err)
	}

	// A second link comes back ordered by evidence ref.
	got, err = a.LinkEvidenceToClaim(ctx, apptest.AI, "", "C-001", "E-001", app.RelationSupports)
	if err != nil || len(got.Links) != 2 || got.Links[0].EvidenceRef != "E-001" || got.Links[1].EvidenceRef != "E-002" {
		t.Fatalf("ordered links = %+v, %v", got.Links, err)
	}

	_, err = a.LinkEvidenceToClaim(ctx, apptest.AI, "", "C-001", "E-001", "refutes")
	wantValidation(t, err, "relation")
	_, err = a.LinkEvidenceToClaim(ctx, apptest.AI, "", "C-001", "H-001", app.RelationSupports)
	wantValidation(t, err, "evidence_ref")
	_, err = a.LinkEvidenceToClaim(ctx, apptest.AI, "", "C-001", "E-009", app.RelationSupports)
	wantErrIs(t, err, app.ErrNotFound)
	_, err = a.LinkEvidenceToClaim(ctx, apptest.AI, "", "C-009", "E-001", app.RelationSupports)
	wantErrIs(t, err, app.ErrNotFound)
}

func TestUnlinkEvidenceFromClaim(t *testing.T) {
	ctx := context.Background()
	a, inv := claimEnv(t)
	mustAddClaim(t, a, apptest.AI, "", app.ClaimInput{Statement: "s", Links: []app.EvidenceLinkInput{
		{EvidenceRef: "E-001", Relation: app.RelationSupports},
		{EvidenceRef: "E-002", Relation: app.RelationContradicts},
	}})
	got, err := a.UnlinkEvidenceFromClaim(ctx, apptest.Human, "", "C-001", "E-001")
	if err != nil || len(got.Links) != 1 || got.Links[0].EvidenceRef != "E-002" {
		t.Fatalf("after unlink = %+v, %v", got.Links, err)
	}
	wantLast(t, a, inv.Ref, "link.remove", "C-001", "Unlinked E-001 from C-001")
	ev, err := a.GetEvidence(ctx, "", "E-001", app.GetEvidenceOptions{})
	if err != nil || len(ev.ClaimLinks) != 0 || ev.ClaimLinks == nil {
		t.Fatalf("E-001 claim links after unlink = %#v, %v", ev.ClaimLinks, err)
	}

	n := activityCount(t, a, inv.Ref)
	_, err = a.UnlinkEvidenceFromClaim(ctx, apptest.Human, "", "C-001", "E-001")
	wantErrIs(t, err, app.ErrNotFound)
	if !strings.Contains(err.Error(), "no link between C-001 and E-001") {
		t.Fatalf("missing link message = %q", err)
	}
	_, err = a.UnlinkEvidenceFromClaim(ctx, apptest.Human, "", "C-001", "E-003")
	wantErrIs(t, err, app.ErrNotFound)
	_, err = a.UnlinkEvidenceFromClaim(ctx, apptest.Human, "", "C-001", "E-009")
	wantErrIs(t, err, app.ErrNotFound)
	if activityCount(t, a, inv.Ref) != n {
		t.Fatal("failed unlinks wrote activity")
	}
}
