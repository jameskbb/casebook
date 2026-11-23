package app_test

import (
	"context"
	"strings"
	"testing"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/app/apptest"
)

// The journey the desktop walks: a person adds one source they looked at and
// one they did not, a finding with a source against it, decides it still
// holds with a reason, and shares. Every sentence a reader gets must be true.
func TestDecideWithReasonAcrossSurfaces(t *testing.T) {
	ctx := context.Background()
	a := apptest.Open(t)
	inv := newInv(t, a, "Why did cafes order less?")
	summary := func() app.InvestigationSummary {
		t.Helper()
		got, err := a.ListInvestigations(ctx, app.ListOptions{})
		if err != nil || len(got) != 1 {
			t.Fatalf("list = %+v, %v", got, err)
		}
		return got[0]
	}

	sales := mustAddEvidence(t, a, apptest.Human, inv.Ref, app.EvidenceInput{
		Title: "Sales by customer type", Summary: "Cafes ordered less in September.", SourceType: app.SourceTypeManualObservation,
		RawDetail: "Cafes: Aug 412, Sep 338.", Verification: app.VerificationHumanVerified,
	})
	notes := mustAddEvidence(t, a, apptest.Human, inv.Ref, app.EvidenceInput{
		Title: "Call notes", Summary: "9 of 14 cafes named a rival.", SourceType: app.SourceTypeManualObservation,
		RawDetail: "9 of 14 cafes: new wholesaler.", Verification: app.VerificationAIGenerated,
	})
	if notes.Verification != app.VerificationAIGenerated {
		t.Fatalf("a source added without the tick = %s, want not checked", notes.Verification)
	}
	rows, err := a.ListActivity(ctx, app.ActivityFilter{InvestigationRef: inv.Ref, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if rows[1].Summary != `Added source E-001 "Sales by customer type"; checked by a person` || rows[0].Summary != `Added source E-002 "Call notes"` {
		t.Fatalf("add summaries = %q, %q", rows[1].Summary, rows[0].Summary)
	}
	if s := summary(); s.SourcesCheckedByYou != 1 || s.FindingsBacking != 0 || s.FindingsDecided != 0 {
		t.Fatalf("before any finding: %+v", s)
	}

	// Saved the way the desktop saves a finding: not decided yet.
	c := mustAddClaim(t, a, apptest.Human, inv.Ref, app.ClaimInput{Statement: "The price rise made cafes order less.", Verification: app.VerificationAIGenerated, Links: []app.EvidenceLinkInput{
		{EvidenceRef: sales.Ref, Relation: app.RelationSupports}, {EvidenceRef: notes.Ref, Relation: app.RelationContradicts},
	}})
	if s := summary(); s.FindingsDisagree != 1 || s.FindingsBacking != 0 {
		t.Fatalf("with sources disagreeing: %+v", s)
	}
	if _, err := a.SetEvidenceVerification(ctx, apptest.Human, inv.Ref, notes.Ref, app.VerificationHumanVerified); err != nil {
		t.Fatal(err)
	}
	r := reviewRow(t, a, inv.Ref, c.Ref)
	got, err := a.ReviewStatement(ctx, apptest.Human, inv.Ref, app.ReviewStatementInput{
		Kind: app.RecordClaim, Ref: c.Ref, Basis: r.Basis, Verification: app.VerificationHumanVerified, SettleDispute: true,
		Rationale: "Only cafes fell. Still holds: Shops and offices stayed the same, so the rival explains only part of it.",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Rationale != "Only cafes fell. Still holds: Shops and offices stayed the same, so the rival explains only part of it." || got.Verification != app.VerificationHumanVerified {
		t.Fatalf("decided = %+v", got)
	}
	if s := summary(); s.FindingsDisagree != 0 || s.FindingsBacking != 1 || s.FindingsDecided != 1 {
		t.Fatalf("after deciding: %+v", s)
	}

	for _, mode := range []string{"brief", "report"} {
		p, err := a.SharePreview(ctx, inv.Ref, app.ShareOptions{Mode: mode})
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{
			"- The price rise made cafes order less. (A person decided this holds) (1 source goes against it)\n",
			"  - Why we think so: Only cafes fell.\n  - Why it still holds: Shops and offices stayed the same, so the rival explains only part of it.\n",
			"  - Exact words: Cafes: Aug 412, Sep 338.\n",
			"  - Exact words: 9 of 14 cafes: new wholesaler.\n",
			"  - Added by a person",
		} {
			if !strings.Contains(p.Markdown, want) {
				t.Errorf("%s missing %q:\n%s", mode, want, p.Markdown)
			}
		}
		if strings.Contains(p.Markdown, "Seen by the person") {
			t.Errorf("%s still says Seen by the person:\n%s", mode, p.Markdown)
		}
		if mode == "report" {
			for _, want := range []string{`- A person added the source "Sales by customer type" and checked it`, `- A person added the source "Call notes"` + "\n"} {
				if !strings.Contains(p.Markdown, want) {
					t.Errorf("report history missing %q:\n%s", want, p.Markdown)
				}
			}
		}
	}
}
