package contextpack

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/jameskbb/casebook/internal/domain"
)

// fidelitySnapshot deliberately separates a finding's verification from the
// evidence it cites: a person's check must never make disputed or superseded
// support look checked, and ruling out an AI idea must not imply human review.
func fidelitySnapshot() Snapshot {
	return Snapshot{
		Investigation: domain.Investigation{Ref: "INV-0001", Title: "Margin decline", Status: domain.InvestigationStatusInvestigating},
		Evidence: []domain.Evidence{
			{Ref: "E-001", Title: "Contested rebate estimate", SourceType: domain.SourceTypeFile, Verification: domain.VerificationDisputed},
			{Ref: "E-002", Title: "Prior margin extract", SourceType: domain.SourceTypeFile, Verification: domain.VerificationSuperseded},
			{Ref: "E-003", Title: "Regional ledger", SourceType: domain.SourceTypeFile, Verification: domain.VerificationSourceVerified},
			{Ref: "E-004", Title: "Unlinked interview", SourceType: domain.SourceTypeUserStatement, Verification: domain.VerificationAIGenerated},
		},
		Claims: []domain.Claim{
			{Ref: "C-001", Statement: "Rebates explain the margin decline.", Confidence: domain.ConfidenceHigh,
				Status: domain.ClaimStatusActive, Verification: domain.VerificationHumanVerified,
				Links: []domain.EvidenceLink{link("E-001", domain.RelationSupports), link("E-002", domain.RelationSupports), link("E-003", domain.RelationContradicts)}},
			{Ref: "C-002", Statement: "The old extract is complete.", Confidence: domain.ConfidenceLow, Status: domain.ClaimStatusDisputed, Verification: domain.VerificationDisputed,
				Links: []domain.EvidenceLink{link("E-002", domain.RelationSupports)}},
			{Ref: "C-003", Statement: "Exchange rates explain the gap.", Status: domain.ClaimStatusRejected, Verification: domain.VerificationAIGenerated,
				Links: []domain.EvidenceLink{link("E-003", domain.RelationContradicts)}},
		},
		Hypotheses: []domain.Hypothesis{
			{Ref: "H-001", Statement: "Duplicate invoices caused the decline.", Status: domain.HypothesisStatusDisproven, Verification: domain.VerificationAIGenerated,
				Links: []domain.EvidenceLink{link("E-003", domain.RelationContradicts)}},
		},
	}
}

func TestEvidenceVerificationSurvivesEveryPackRendering(t *testing.T) {
	s := fidelitySnapshot()
	full := Build(s, Options{})
	compact := Pack{}
	for limit := len(RenderMarkdown(full)) - 1; limit > 0; limit-- {
		p := Build(s, Options{Compact: true, MaxBytes: limit})
		if len(p.EvidenceIndex) == 0 && len(p.EstablishedClaims) > 0 && len(p.DisprovenHypotheses) > 0 {
			compact = p
			break
		}
	}
	if compact.Format == "" {
		t.Fatal("fixture did not exercise removal of evidence details with findings kept")
	}
	for name, p := range map[string]Pack{"full": full, "compact": compact} {
		t.Run(name, func(t *testing.T) {
			md := RenderMarkdown(p)
			for _, want := range []string{
				"Current claims:", "E-001 [Disputed]", "E-002 [Superseded]", "E-003 [Source verified]",
				"H-001 Duplicate invoices caused the decline. [AI generated]",
				"Evidence outside the included findings: 1 omitted.",
			} {
				if !strings.Contains(md, want) {
					t.Errorf("Markdown lacks %q:\n%s", want, md)
				}
			}
			if strings.Contains(md, "Established claims:") {
				t.Fatal("old human-facing heading remains")
			}
			js, err := RenderJSON(p)
			if err != nil {
				t.Fatal(err)
			}
			var decoded Pack
			if err := json.Unmarshal(js, &decoded); err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(decoded.EvidenceVerification, full.EvidenceVerification) {
				t.Fatalf("JSON evidence verification changed: %+v", decoded.EvidenceVerification)
			}
			if len(decoded.EvidenceIndex) > 0 && decoded.EvidenceIndex[0].Verification != domain.VerificationDisputed {
				t.Fatal("evidence index dropped verification")
			}
		})
	}
}

func TestReportLabelsInlineEvidenceAndRuledOutFindings(t *testing.T) {
	md := RenderReport(fidelitySnapshot(), ReportOptions{})
	for _, want := range []string{
		"## Current claims", "Supported by E-001: Contested rebate estimate (File; Disputed)",
		"Supported by E-002: Prior margin extract (File; Superseded)",
		"Contradicted by E-003: Regional ledger (File; Source verified)",
		"- **H-001** Duplicate invoices caused the decline. (disproven, AI generated)",
		"- **C-003** Exchange rates explain the gap. (claim rejected, AI generated)",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("report lacks %q:\n%s", want, md)
		}
	}
	html := ReportHTML(md)
	for _, want := range []string{"File; Disputed", "File; Superseded", "disproven, AI generated", "claim rejected, AI generated"} {
		if !strings.Contains(html, want) {
			t.Errorf("HTML report lacks %q", want)
		}
	}
}

func TestCompactWarningsAndVerificationStayHonestAtEveryBudget(t *testing.T) {
	s := fidelitySnapshot()
	full := Build(s, Options{})
	for limit := 1; limit < len(RenderMarkdown(full)); limit += 7 {
		p := Build(s, Options{MaxBytes: limit})
		md := RenderMarkdown(p)
		if p.Omitted.DisputedClaims > 0 {
			want := fmt.Sprintf("(%d more disputed claim omitted)", p.Omitted.DisputedClaims)
			if !strings.Contains(md, want) {
				t.Fatalf("budget %d lost dispute count: %s", limit, md)
			}
		}
		for _, c := range p.EstablishedClaims {
			for _, ref := range append(slices.Clone(c.SupportingEvidence), c.ContradictingEvidence...) {
				if p.evidenceVerificationLabel(ref) != full.evidenceVerificationLabel(ref) {
					t.Fatalf("budget %d lost %s verification", limit, ref)
				}
			}
		}
		if len(p.EstablishedClaims)+len(p.DisputedClaims)+len(p.Hypotheses)+len(p.DisprovenHypotheses) == 0 && len(p.EvidenceVerification) != 0 {
			t.Fatalf("budget %d retained unused verification entries", limit)
		}
		if !strings.Contains(md, "Evidence outside the included findings: 1 omitted.") {
			t.Fatalf("budget %d lost evidence omission warning", limit)
		}
	}
	slices.Reverse(s.Evidence)
	slices.Reverse(s.Claims)
	if RenderMarkdown(Build(s, Options{})) != RenderMarkdown(full) {
		t.Fatal("verification rendering depends on input order")
	}
}

func TestMissingEvidenceNeverBorrowsFindingVerification(t *testing.T) {
	s := fidelitySnapshot()
	s.Evidence = nil
	p := Build(s, Options{})
	for _, body := range []string{RenderMarkdown(p), RenderReport(s, ReportOptions{})} {
		if !strings.Contains(body, "Verification unavailable") {
			t.Fatalf("missing evidence rendered as checked: %s", body)
		}
	}
}
