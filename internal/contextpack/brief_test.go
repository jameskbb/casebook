package contextpack

import (
	"slices"
	"strings"
	"testing"

	"github.com/jameskbb/casebook/internal/domain"
)

func TestBriefStableOrderAndUntrustedText(t *testing.T) {
	s := Snapshot{Investigation: domain.Investigation{Ref: "INV-0001", Title: "Title\n## Fake", Conclusion: "# Answer\n- injected", ConclusionVerification: domain.VerificationAIGenerated}, Claims: []domain.Claim{{Ref: "C-002", Statement: "second"}, {Ref: "C-001", Statement: "first\n## Added"}}, Evidence: []domain.Evidence{{Ref: "E-001", Title: "<script>unsafe</script>", Verification: domain.VerificationAIGenerated}}}
	first := RenderBrief(s, BriefOmitted{Questions: 4})
	slices.Reverse(s.Claims)
	if got := RenderBrief(s, BriefOmitted{Questions: 4}); got != first {
		t.Fatal("input order changes bytes")
	}
	if strings.Contains(first, "\n## Fake") || strings.Contains(first, "\n## Added") || strings.Contains(first, "\n# Answer") {
		t.Fatal("stored text injected Markdown blocks")
	}
	if !strings.Contains(first, "4 questions") || !strings.Contains(first, "AI generated") {
		t.Fatal("omissions or verification missing")
	}
	if strings.Contains(ReportHTML(first), "<script>") {
		t.Fatal("stored HTML executed")
	}
}

func TestConclusionBasisStylesAndReviewTime(t *testing.T) {
	c := PackConclusion{Verification: domain.VerificationHumanVerified, ReviewedAt: "2026-10-27T03:29:47.051Z", SupportingClaims: []PackConclusionClaim{
		{Ref: "C-002", Status: domain.ClaimStatusActive, Verification: domain.VerificationAIGenerated},
		{Ref: "C-004", Status: domain.ClaimStatusSuperseded, Verification: domain.VerificationHumanVerified},
		{Ref: "C-005", Status: domain.ClaimStatusSuperseded, Verification: domain.VerificationSuperseded},
		{Ref: "C-009", Missing: true},
	}, ContradictingClaims: []PackConclusionClaim{{Ref: "C-001", Status: domain.ClaimStatusDisputed, Verification: domain.VerificationDisputed}}}
	parens := strings.Join(conclusionAccountabilityLines(c, basisParens), "\n")
	brackets := strings.Join(conclusionAccountabilityLines(c, basisBrackets), "\n")
	for _, want := range []string{"Supporting claims: C-002 (AI generated); C-004 (Superseded; Human verified); C-005 (Superseded); C-009 (missing claim)", "Contradicting claims: C-001 (Disputed)", "Conclusion reviewed: 2026-10-27 03:29 UTC"} {
		if !strings.Contains(parens, want) {
			t.Errorf("parens style missing %q:\n%s", want, parens)
		}
	}
	for _, want := range []string{"Supporting claims: C-002 [AI generated], C-004 [Superseded, Human verified], C-005 [Superseded], C-009 [missing claim]", "Contradicting claims: C-001 [Disputed]", "Conclusion reviewed: 2026-10-27 03:29 UTC"} {
		if !strings.Contains(brackets, want) {
			t.Errorf("brackets style missing %q:\n%s", want, brackets)
		}
	}
}

func TestBriefRuledOutSection(t *testing.T) {
	s := Snapshot{Investigation: domain.Investigation{Ref: "INV-0001", Title: "T"},
		Claims:     []domain.Claim{{Ref: "C-001", Statement: "kept", Status: domain.ClaimStatusActive, Verification: domain.VerificationAIGenerated}, {Ref: "C-002", Statement: "rejected one", Status: domain.ClaimStatusRejected, Verification: domain.VerificationAIGenerated}},
		Hypotheses: []domain.Hypothesis{{Ref: "H-001", Statement: "dead end", Status: domain.HypothesisStatusDisproven, Verification: domain.VerificationAIGenerated}, {Ref: "H-002", Statement: "live", Status: domain.HypothesisStatusOpen, Verification: domain.VerificationAIGenerated}}}
	got := RenderBrief(s, BriefOmitted{})
	findings, live, ruled, questions := strings.Index(got, "## Selected findings"), strings.Index(got, "## Explanations still open"), strings.Index(got, "## Ruled out\n\n- **C-002** rejected one (Rejected; AI generated)\n\n- **H-001** dead end (Disproven; AI generated)\n"), strings.Index(got, "## Selected questions")
	if !(findings < live && live < ruled && ruled < questions) || findings < 0 {
		t.Fatalf("Ruled out section missing or out of order:\n%s", got)
	}
	if c := strings.Index(got, "**C-001**"); !(findings < c && c < live) {
		t.Errorf("current claim is not under the findings:\n%s", got)
	}
	if empty := RenderBrief(Snapshot{Investigation: domain.Investigation{Ref: "INV-0001"}}, BriefOmitted{}); !strings.Contains(empty, "## Ruled out\n\nNone selected.\n") {
		t.Errorf("empty Ruled out section does not say so:\n%s", empty)
	}
}

func TestBriefOmissionNoticeCounts(t *testing.T) {
	for _, tc := range []struct {
		o    BriefOmitted
		want string
	}{
		{BriefOmitted{}, "Every claim, hypothesis, question and evidence item in the case is included."},
		{BriefOmitted{Evidence: 1}, "Not included: 1 evidence item."},
		{BriefOmitted{Claims: 1, Questions: 1}, "Not included: 1 claim and 1 question."},
		{BriefOmitted{Claims: 3, Hypotheses: 1, Questions: 2, Evidence: 4}, "Not included: 3 claims, 1 hypothesis, 2 questions and 4 evidence items."},
	} {
		got := briefOmissionNotice(tc.o)
		if !strings.HasPrefix(got, tc.want+" Activity history is not included.") {
			t.Errorf("briefOmissionNotice(%+v) = %q, want prefix %q", tc.o, got, tc.want)
		}
	}
}

func TestBriefOmissionNoticeNamesRefs(t *testing.T) {
	const tail = " Activity history is not included. Ask the person who shared this for the complete report."
	for _, tc := range []struct {
		o    BriefOmitted
		want string
	}{
		{BriefOmitted{ClaimRefs: []string{}, EvidenceRefs: []string{}}, "Every claim, hypothesis, question and evidence item in the case is included." + tail},
		{BriefOmitted{Evidence: 1, EvidenceRefs: []string{"E-002"}}, "Not included: E-002." + tail},
		{BriefOmitted{Hypotheses: 1, Questions: 1, HypothesisRefs: []string{"H-003"}, QuestionRefs: []string{"Q-001"}}, "Not included: H-003 and Q-001." + tail},
		{BriefOmitted{Claims: 1, Hypotheses: 1, Questions: 2, Evidence: 1, ClaimRefs: []string{"C-004"}, HypothesisRefs: []string{"H-003"}, QuestionRefs: []string{"Q-001", "Q-002"}, EvidenceRefs: []string{"E-002"}}, "Not included: C-004, H-003, Q-001, Q-002 and E-002." + tail},
	} {
		if got := briefOmissionNotice(tc.o); got != tc.want {
			t.Errorf("briefOmissionNotice(%+v) = %q, want %q", tc.o, got, tc.want)
		}
	}
}

func TestBriefStateTail(t *testing.T) {
	for _, tc := range []struct {
		got, want string
	}{
		{briefStateTail(domain.ClaimStatusActive.Label(), domain.ConfidenceHigh, domain.VerificationAIGenerated, true), "Current; High confidence; AI generated"},
		{briefStateTail(domain.ClaimStatusDisputed.Label(), domain.ConfidenceLow, domain.VerificationDisputed, true), "Disputed; Low confidence"},
		{briefStateTail(domain.HypothesisStatusDisproven.Label(), domain.ConfidenceMedium, domain.VerificationAIGenerated, false), "Disproven; AI generated"},
		{briefStateTail(domain.ClaimStatusRejected.Label(), domain.ConfidenceUnknown, domain.VerificationHumanVerified, false), "Rejected; Human verified"},
		{briefStateTail(domain.ClaimStatusActive.Label(), domain.ConfidenceUnknown, domain.VerificationSourceVerified, true), "Current; Confidence not assessed; Source verified"},
	} {
		if tc.got != tc.want {
			t.Errorf("got %q, want %q", tc.got, tc.want)
		}
	}
}
