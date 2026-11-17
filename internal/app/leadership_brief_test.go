package app_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/app/apptest"
	"github.com/jameskbb/casebook/internal/contextpack"
)

func TestLeadershipBriefRetainsCaveatsAndCountsOmissions(t *testing.T) {
	a := apptest.Open(t)
	ctx := context.Background()
	inv, err := a.CreateInvestigation(ctx, apptest.Human, app.CreateInvestigationInput{Title: "Margin <gap>", ProblemStatement: "Why did margin fall?"})
	if err != nil {
		t.Fatal(err)
	}
	ev, err := a.AddEvidence(ctx, apptest.AI, inv.Ref, app.EvidenceInput{Title: "Conflicting ledger", Summary: "Expenses exceed the allocation.", SourceType: app.SourceTypeManualObservation})
	if err != nil {
		t.Fatal(err)
	}
	ignored, err := a.AddClaim(ctx, apptest.AI, inv.Ref, app.ClaimInput{Statement: "Unselected detail"})
	if err != nil {
		t.Fatal(err)
	}
	disputed, err := a.AddClaim(ctx, apptest.AI, inv.Ref, app.ClaimInput{Statement: "Disputed allocation", Status: app.ClaimStatusDisputed})
	if err != nil {
		t.Fatal(err)
	}
	contra, err := a.AddClaim(ctx, apptest.AI, inv.Ref, app.ClaimInput{Statement: "Conflicting allocation", Links: []app.EvidenceLinkInput{{EvidenceRef: ev.Ref, Relation: app.RelationContradicts}}})
	if err != nil {
		t.Fatal(err)
	}
	hyp, err := a.AddHypothesis(ctx, apptest.AI, inv.Ref, app.HypothesisInput{Statement: "Vendor fees ruled out", Status: app.HypothesisStatusDisproven, Links: []app.EvidenceLinkInput{{EvidenceRef: ev.Ref, Relation: app.RelationContradicts}}})
	if err != nil {
		t.Fatal(err)
	}
	q, err := a.AddQuestion(ctx, apptest.Human, inv.Ref, app.QuestionInput{Question: "Who owns reconciliation?"})
	if err != nil {
		t.Fatal(err)
	}
	o := app.ShareOptions{Mode: "brief", ClaimRefs: []string{}, QuestionRefs: []string{}, RuledOutRefs: []string{hyp.Ref}}
	p, err := a.SharePreview(ctx, inv.Ref, o)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{disputed.Statement, contra.Statement, hyp.Statement, "Contradicted by " + ev.Ref, "AI generated", "Disputed", "No conclusion recorded yet.", "Not included: " + ignored.Ref + " and " + q.Ref + ". Activity history is not included. Ask the person who shared this for the complete report."} {
		if !strings.Contains(p.Markdown, want) {
			t.Errorf("brief missing %q:\n%s", want, p.Markdown)
		}
	}
	if strings.Contains(p.Markdown, ignored.Statement) || strings.Contains(p.Markdown, q.Question) {
		t.Fatal("unselected nonmaterial records included")
	}
	if p.HTML != contextpack.ReportHTML(p.Markdown) || !strings.Contains(p.HTML, "Margin &lt;gap&gt;") {
		t.Fatal("HTML did not derive from same escaped Markdown")
	}
	repeat, err := a.SharePreview(ctx, inv.Ref, o)
	if err != nil || repeat.SnapshotID != p.SnapshotID || repeat.Markdown != p.Markdown {
		t.Fatal("unchanged preview is not deterministic")
	}
	for _, c := range p.Candidates {
		if c.Ref == contra.Ref || c.Ref == disputed.Ref {
			if !c.Required || !c.Selected {
				t.Errorf("material caveat selectable away: %+v", c)
			}
		}
	}
	o.QuestionRefs = []string{q.Ref}
	selected, err := a.SharePreview(ctx, inv.Ref, o)
	if err != nil || !strings.Contains(selected.Markdown, q.Question) || selected.SnapshotID == p.SnapshotID {
		t.Fatal("selected question not represented")
	}
	o.Mode = "report"
	full, err := a.SharePreview(ctx, inv.Ref, o)
	if err != nil || !strings.Contains(full.Markdown, ignored.Statement) {
		t.Fatal("complete report unavailable")
	}
	if full.HTML != contextpack.ReportHTML(full.Markdown) {
		t.Fatal("complete report HTML mismatch")
	}
}

func TestSharePreviewValidatesSelections(t *testing.T) {
	a := apptest.Open(t)
	ctx := context.Background()
	inv, err := a.CreateInvestigation(ctx, apptest.Human, app.CreateInvestigationInput{Title: "Select"})
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range []app.ShareOptions{{Mode: "summary"}, {Mode: "brief", ClaimRefs: []string{"C-999"}}, {Mode: "brief", QuestionRefs: []string{"INV-0002/Q-001"}}, {Mode: "brief", RuledOutRefs: []string{"H-001"}}} {
		if _, err := a.SharePreview(ctx, inv.Ref, o); err == nil {
			t.Errorf("invalid selection accepted: %+v", o)
		}
	}
	if _, err := a.SharePreview(ctx, "INV-9999", app.ShareOptions{}); err == nil {
		t.Fatal("missing case accepted")
	}
	p, err := a.SharePreview(ctx, inv.Ref, app.ShareOptions{})
	if err != nil {
		t.Fatal(err)
	}
	apptest.AssertNoJSONNulls(t, p)
}

func TestLeadershipBriefKeepsConclusionBasisAndMixedVerification(t *testing.T) {
	a := apptest.Open(t)
	ctx := context.Background()
	inv, err := a.CreateInvestigation(ctx, apptest.Human, app.CreateInvestigationInput{Title: "Mixed review"})
	if err != nil {
		t.Fatal(err)
	}
	ev, err := a.AddEvidence(ctx, apptest.AI, inv.Ref, app.EvidenceInput{Title: "AI captured ledger", Summary: "Recorded expense movement.", RawDetail: "Private raw capture", SourceType: app.SourceTypeFile, SourceRef: "ledger.csv"})
	if err != nil {
		t.Fatal(err)
	}
	c, err := a.AddClaim(ctx, apptest.Human, inv.Ref, app.ClaimInput{Statement: "Person checked expense explanation", Links: []app.EvidenceLinkInput{{EvidenceRef: ev.Ref, Relation: app.RelationSupports}}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = a.UpdateConclusion(ctx, apptest.Human, inv.Ref, app.ConclusionInput{Conclusion: "Expenses explain the drop.", SupportingClaims: []string{c.Ref}})
	if err != nil {
		t.Fatal(err)
	}
	p, err := a.SharePreview(ctx, inv.Ref, app.ShareOptions{ClaimRefs: []string{}, QuestionRefs: []string{}, RuledOutRefs: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{c.Statement, "Human verified", "Supported by " + ev.Ref + ": AI captured ledger (AI generated)", "**" + ev.Ref + "** AI captured ledger (File: ledger.csv; AI generated)", "Expenses explain the drop.", "Conclusion verification: Human verified", "Conclusion review: not reviewed; needs review."} {
		if !strings.Contains(p.Markdown, want) {
			t.Errorf("brief missing %q:\n%s", want, p.Markdown)
		}
	}
	if strings.Contains(p.Markdown, "Private raw capture") {
		t.Fatal("brief disclosed raw capture")
	}
}

func TestLeadershipBriefRetainsDisputedSourcesOutsideSelection(t *testing.T) {
	a := apptest.Open(t)
	ctx := context.Background()
	inv, err := a.CreateInvestigation(ctx, apptest.Human, app.CreateInvestigationInput{Title: "Source caveats"})
	if err != nil {
		t.Fatal(err)
	}
	linked, err := a.AddEvidence(ctx, apptest.Human, inv.Ref, app.EvidenceInput{Title: "Disputed linked ledger", Summary: "Unsettled ledger source.", SourceType: app.SourceTypeFile, Verification: app.VerificationDisputed})
	if err != nil {
		t.Fatal(err)
	}
	standalone, err := a.AddEvidence(ctx, apptest.Human, inv.Ref, app.EvidenceInput{Title: "Disputed standalone source", Summary: "Unsettled source.", SourceType: app.SourceTypeFile, Verification: app.VerificationDisputed})
	if err != nil {
		t.Fatal(err)
	}
	c, err := a.AddClaim(ctx, apptest.AI, inv.Ref, app.ClaimInput{Statement: "Active claim on disputed support", Links: []app.EvidenceLinkInput{{EvidenceRef: linked.Ref, Relation: app.RelationSupports}}})
	if err != nil {
		t.Fatal(err)
	}
	p, err := a.SharePreview(ctx, inv.Ref, app.ShareOptions{ClaimRefs: []string{}, QuestionRefs: []string{}, RuledOutRefs: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{c.Statement, linked.Title, standalone.Title, "Disputed", "Every claim, hypothesis, question and evidence item in the case is included."} {
		if !strings.Contains(p.Markdown, want) {
			t.Errorf("source dispute omitted %q", want)
		}
	}
	for _, candidate := range p.Candidates {
		if candidate.Ref == c.Ref && !candidate.Required {
			t.Fatal("claim on disputed source can be removed")
		}
	}
}

func TestLeadershipBriefBoundsRetiredFindings(t *testing.T) {
	a := apptest.Open(t)
	ctx := context.Background()
	inv, err := a.CreateInvestigation(ctx, apptest.Human, app.CreateInvestigationInput{Title: "Many retired explanations"})
	if err != nil {
		t.Fatal(err)
	}
	ev, err := a.AddEvidence(ctx, apptest.AI, inv.Ref, app.EvidenceInput{Title: "Contradicting source", Summary: "Checks ruled out alternatives.", SourceType: app.SourceTypeFile})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 8; i++ {
		_, err := a.AddHypothesis(ctx, apptest.AI, inv.Ref, app.HypothesisInput{Statement: fmt.Sprintf("Retired explanation %d", i), Status: app.HypothesisStatusDisproven, Links: []app.EvidenceLinkInput{{EvidenceRef: ev.Ref, Relation: app.RelationContradicts}}})
		if err != nil {
			t.Fatal(err)
		}
	}
	p, err := a.SharePreview(ctx, inv.Ref, app.ShareOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(p.Markdown, "Retired explanation"); got != 2 || p.Omitted.Hypotheses != 6 {
		t.Fatalf("default brief has %d retired findings, omissions %+v", got, p.Omitted)
	}
	none, err := a.SharePreview(ctx, inv.Ref, app.ShareOptions{RuledOutRefs: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(none.Markdown, "Retired explanation") || none.Omitted.Hypotheses != 8 {
		t.Fatal("explicit empty selection retained retired findings")
	}
	for _, candidate := range none.Candidates {
		if candidate.Group == "ruled_out" && candidate.Required {
			t.Fatal("ordinary retired finding became mandatory")
		}
	}
}

func TestSharePreviewFormatsMatchDuringConcurrentWrites(t *testing.T) {
	a := apptest.Open(t)
	ctx := context.Background()
	inv, err := a.CreateInvestigation(ctx, apptest.Human, app.CreateInvestigationInput{Title: "Initial title"})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		for i := 0; i < 30; i++ {
			title := fmt.Sprintf("Changed title %d", i)
			if _, err := a.UpdateInvestigation(ctx, apptest.AI, inv.Ref, app.InvestigationPatch{Title: &title}); err != nil {
				done <- err
				return
			}
		}
		done <- nil
	}()
	for i := 0; i < 30; i++ {
		p, err := a.SharePreview(ctx, inv.Ref, app.ShareOptions{Mode: "report"})
		if err != nil {
			t.Fatal(err)
		}
		if p.HTML != contextpack.ReportHTML(p.Markdown) {
			t.Fatal("preview HTML and Markdown came from different case versions")
		}
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestLeadershipBriefSelectsOpenExplanations(t *testing.T) {
	a := apptest.Open(t)
	ctx := context.Background()
	inv, err := a.CreateInvestigation(ctx, apptest.Human, app.CreateInvestigationInput{Title: "Open explanations"})
	if err != nil {
		t.Fatal(err)
	}
	var refs []string
	for i, status := range []app.HypothesisStatus{app.HypothesisStatusSupported, app.HypothesisStatusOpen, app.HypothesisStatusOpen} {
		h, err := a.AddHypothesis(ctx, apptest.AI, inv.Ref, app.HypothesisInput{Statement: fmt.Sprintf("Live explanation %d", i), Status: status})
		if err != nil {
			t.Fatal(err)
		}
		refs = append(refs, h.Ref)
	}
	dead, err := a.AddHypothesis(ctx, apptest.AI, inv.Ref, app.HypothesisInput{Statement: "Dead end", Status: app.HypothesisStatusDisproven})
	if err != nil {
		t.Fatal(err)
	}
	disputed, err := a.AddClaim(ctx, apptest.AI, inv.Ref, app.ClaimInput{Statement: "Disputed reading", Status: app.ClaimStatusDisputed, Verification: app.VerificationDisputed})
	if err != nil {
		t.Fatal(err)
	}
	p, err := a.SharePreview(ctx, inv.Ref, app.ShareOptions{})
	if err != nil {
		t.Fatal(err)
	}
	selected := map[string]bool{}
	for _, c := range p.Candidates {
		if c.Group == "hypotheses" {
			selected[c.Ref] = c.Selected
		}
		if c.Label == "Disputed; Disputed" || strings.Contains(c.Label, "Active") {
			t.Errorf("candidate label repeats or uses the old word: %+v", c)
		}
		if c.Ref == disputed.Ref && !strings.HasPrefix(c.Label, "Disputed; ") {
			t.Errorf("disputed claim label = %q", c.Label)
		}
		if c.Ref == dead.Ref && (c.Group != "ruled_out" || c.Label != "Disproven; AI generated") {
			t.Errorf("disproven candidate = %+v", c)
		}
	}
	if len(selected) != 3 || !selected[refs[0]] || !selected[refs[1]] || selected[refs[2]] {
		t.Fatalf("default open explanations = %v, want the first two of %v", selected, refs)
	}
	if !strings.Contains(p.Markdown, "## Explanations still open or supported\n\n- **"+refs[0]+"** Live explanation 0 (Supported; Confidence not assessed; AI generated)") || strings.Contains(p.Markdown, "Live explanation 2") {
		t.Fatalf("brief does not show the default explanations:\n%s", p.Markdown)
	}
	if got := strings.Join(p.Omitted.HypothesisRefs, ","); got != refs[2] || p.Omitted.Hypotheses != 1 {
		t.Fatalf("omitted hypotheses = %v (%d), want %s", p.Omitted.HypothesisRefs, p.Omitted.Hypotheses, refs[2])
	}
	chosen, err := a.SharePreview(ctx, inv.Ref, app.ShareOptions{HypothesisRefs: []string{refs[2]}, RuledOutRefs: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(chosen.Markdown, "Live explanation 2") || strings.Contains(chosen.Markdown, "Live explanation 0") || !strings.Contains(chosen.Markdown, "Not included: "+refs[0]+", "+refs[1]+" and "+dead.Ref+".") {
		t.Fatalf("explicit selection not honoured:\n%s", chosen.Markdown)
	}
	for _, o := range []app.ShareOptions{{HypothesisRefs: []string{dead.Ref}}, {RuledOutRefs: []string{refs[0]}}, {HypothesisRefs: []string{disputed.Ref}}} {
		if _, err := a.SharePreview(ctx, inv.Ref, o); err == nil {
			t.Errorf("selection outside its group accepted: %+v", o)
		}
	}
}
