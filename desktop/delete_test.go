package main

import (
	"testing"

	"github.com/jameskbb/casebook/internal/app"
)

// TestDeleteBindings drives the AR-10 delete bindings: the guards come back
// as conflicts the page can show, and the deletes remove the records.
func TestDeleteBindings(t *testing.T) {
	d, _ := newTestDesktop(t)
	inv, err := d.CreateInvestigation(app.CreateInvestigationInput{Title: "Delete me"})
	if err != nil {
		t.Fatal(err)
	}
	ev, err := d.AddEvidence(inv.Ref, app.EvidenceInput{Title: "E", Summary: "s", SourceType: app.SourceTypeManualObservation})
	if err != nil {
		t.Fatal(err)
	}
	c, err := d.AddClaim(inv.Ref, app.ClaimInput{Statement: "C", Links: []app.EvidenceLinkInput{{EvidenceRef: ev.Ref, Relation: app.RelationSupports}}})
	if err != nil {
		t.Fatal(err)
	}
	q, err := d.AddQuestion(inv.Ref, app.QuestionInput{Question: "Q?"})
	if err != nil {
		t.Fatal(err)
	}
	if ue := decodeUIError(t, d.DeleteEvidence(inv.Ref, ev.Ref)); ue.Kind != kindConflict {
		t.Fatalf("linked evidence delete = %+v, want conflict", ue)
	}
	if err := d.DeleteClaim(inv.Ref, c.Ref); err != nil {
		t.Fatal(err)
	}
	if err := d.DeleteEvidence(inv.Ref, ev.Ref); err != nil {
		t.Fatal(err)
	}
	if err := d.DeleteQuestion(inv.Ref, q.Ref); err != nil {
		t.Fatal(err)
	}
	h, err := d.AddHypothesis(inv.Ref, app.HypothesisInput{Statement: "H"})
	if err != nil {
		t.Fatal(err)
	}
	if err := d.DeleteHypothesis(inv.Ref, h.Ref); err != nil {
		t.Fatal(err)
	}
	det, err := d.GetInvestigationDetail(inv.Ref, app.DetailOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(det.Evidence)+len(det.Claims)+len(det.Hypotheses)+len(det.Questions) != 0 {
		t.Fatalf("records left after deletes: %+v", det)
	}
	if ue := decodeUIError(t, d.DeleteInvestigation(inv.Ref)); ue.Kind != kindConflict {
		t.Fatalf("live investigation delete = %+v, want conflict", ue)
	}
	if _, err := d.ArchiveInvestigation(inv.Ref); err != nil {
		t.Fatal(err)
	}
	if err := d.DeleteInvestigation(inv.Ref); err != nil {
		t.Fatal(err)
	}
	if ue := decodeUIError(t, d.DeleteInvestigation(inv.Ref)); ue.Kind != kindNotFound {
		t.Fatalf("second delete = %+v, want not_found", ue)
	}
}
