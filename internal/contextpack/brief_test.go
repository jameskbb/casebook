package contextpack

import (
	"slices"
	"strings"
	"testing"

	"github.com/jameskbb/casebook/internal/domain"
)

func TestBriefStableOrderAndUntrustedText(t *testing.T) {
	s := Snapshot{Investigation: domain.Investigation{Ref: "INV-0001", Title: "Title\n## Fake", Conclusion: "# Answer\n- injected", ConclusionVerification: domain.VerificationAIGenerated}, Claims: []domain.Claim{{Ref: "C-002", Statement: "second"}, {Ref: "C-001", Statement: "first\n## Added"}}, Evidence: []domain.Evidence{{Ref: "E-001", Title: "<script>unsafe</script>", Verification: domain.VerificationAIGenerated}}}
	first := RenderBrief(s, BriefOmitted{Questions: 4}, s.Evidence)
	slices.Reverse(s.Claims)
	if got := RenderBrief(s, BriefOmitted{Questions: 4}, s.Evidence); got != first {
		t.Fatal("input order changes bytes")
	}
	if strings.Contains(first, "\n## Fake") || strings.Contains(first, "\n## Added") || strings.Contains(first, "\n# Answer") {
		t.Fatal("stored text injected Markdown blocks")
	}
	if !strings.Contains(first, "4 follow-up questions") || !strings.Contains(first, "Not checked") {
		t.Fatal("omissions or check label missing")
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
	for _, want := range []string{"Answer marked: You checked this", "Findings behind the answer: C-002 (Not checked); C-004 (Replaced; You checked this); C-005 (Replaced); C-009 (missing finding)", "Findings against the answer: C-001 (Sources disagree)", "Answer reviewed: 2026-10-27 03:29 UTC"} {
		if !strings.Contains(parens, want) {
			t.Errorf("parens style missing %q:\n%s", want, parens)
		}
	}
	// A pack is read by an AI client, so a person's check is never "You".
	for _, want := range []string{"Answer marked: A person checked this", "Findings behind the answer: C-002 [Not checked], C-004 [Replaced, A person checked this], C-005 [Replaced], C-009 [missing finding]", "Findings against the answer: C-001 [Sources disagree]", "Answer reviewed: 2026-10-27 03:29 UTC"} {
		if !strings.Contains(brackets, want) {
			t.Errorf("brackets style missing %q:\n%s", want, brackets)
		}
	}
}

func TestBriefRuledOutSection(t *testing.T) {
	s := Snapshot{Investigation: domain.Investigation{Ref: "INV-0001", Title: "T"},
		Claims:     []domain.Claim{{Ref: "C-001", Statement: "kept", Status: domain.ClaimStatusActive, Verification: domain.VerificationAIGenerated}, {Ref: "C-002", Statement: "rejected one", Status: domain.ClaimStatusRejected, Verification: domain.VerificationAIGenerated}},
		Hypotheses: []domain.Hypothesis{{Ref: "H-001", Statement: "dead end", Status: domain.HypothesisStatusDisproven, Verification: domain.VerificationAIGenerated}, {Ref: "H-002", Statement: "live", Status: domain.HypothesisStatusOpen, Verification: domain.VerificationAIGenerated}}}
	got := RenderBrief(s, BriefOmitted{}, nil)
	findings, live, ruled, questions := strings.Index(got, "## Findings"), strings.Index(got, "## Still being tested\n\n- **H-002** live (Not tested yet; Not checked)\n"), strings.Index(got, "## Ruled out\n\n- **C-002** rejected one (Ruled out; Not checked)\n\n- **H-001** dead end (Ruled out; Not checked)\n"), strings.Index(got, "## Follow-up questions")
	if !(findings < live && live < ruled && ruled < questions) || findings < 0 {
		t.Fatalf("Ruled out section missing or out of order:\n%s", got)
	}
	if c := strings.Index(got, "**C-001** kept (Not checked)"); !(findings < c && c < live) {
		t.Errorf("current finding is not under the findings:\n%s", got)
	}
	if empty := RenderBrief(Snapshot{Investigation: domain.Investigation{Ref: "INV-0001"}}, BriefOmitted{}, nil); !strings.Contains(empty, "## Ruled out\n\nNone selected.\n") || !strings.Contains(empty, "## Answer\n\nNo answer yet.\n") {
		t.Errorf("empty sections do not say so:\n%s", empty)
	}
}

// The count of sources checked stands where confidence used to. It counts
// every source of the question, not only the ones selected for the answer.
func TestBriefCountsEverySource(t *testing.T) {
	all := []domain.Evidence{
		{Ref: "E-001", Verification: domain.VerificationHumanVerified},
		{Ref: "E-002", Verification: domain.VerificationSourceVerified},
		{Ref: "E-003", Verification: domain.VerificationAIGenerated},
		{Ref: "E-004", Verification: domain.VerificationSuperseded},
	}
	s := Snapshot{Investigation: domain.Investigation{Ref: "INV-0001", Conclusion: "Because.", ConclusionConfidence: domain.ConfidenceHigh, ConclusionRationale: "The sources agree."}, Evidence: all[:1]}
	got := RenderBrief(s, BriefOmitted{}, all)
	if !strings.Contains(got, "\nBecause.\n\n1 of 3 sources checked by you. 1 checked by AI, 1 not checked.\n") || !strings.Contains(got, "\nWhy: The sources agree.\n") {
		t.Errorf("answer section:\n%s", got)
	}
	for _, banned := range []string{"onfidence", "High"} {
		if strings.Contains(got, banned) {
			t.Errorf("shared answer prints %q:\n%s", banned, got)
		}
	}
	for _, tc := range []struct {
		ev   []domain.Evidence
		want string
	}{
		{nil, ""},
		{all[3:], ""},
		{all[:1], "1 of 1 source checked by you."},
		{all[2:3], "0 of 1 source checked by you. 1 not checked."},
	} {
		if got := sourcesCheckedLine(tc.ev); got != tc.want {
			t.Errorf("sourcesCheckedLine = %q, want %q", got, tc.want)
		}
	}
}

func TestBriefOmissionNoticeCounts(t *testing.T) {
	for _, tc := range []struct {
		o    BriefOmitted
		want string
	}{
		{BriefOmitted{}, "Every finding, follow-up question and source is included."},
		{BriefOmitted{Evidence: 1}, "Not included: 1 source."},
		{BriefOmitted{Claims: 1, Questions: 1}, "Not included: 1 finding and 1 follow-up question."},
		{BriefOmitted{Claims: 3, Hypotheses: 1, Questions: 2, Evidence: 4}, "Not included: 4 findings, 2 follow-up questions and 4 sources."},
	} {
		got := briefOmissionNotice(tc.o)
		if !strings.HasPrefix(got, tc.want+" The history is not included.") {
			t.Errorf("briefOmissionNotice(%+v) = %q, want prefix %q", tc.o, got, tc.want)
		}
	}
}

func TestBriefOmissionNoticeNamesRefs(t *testing.T) {
	const tail = " The history is not included. Ask the person who shared this for the full version."
	for _, tc := range []struct {
		o    BriefOmitted
		want string
	}{
		{BriefOmitted{ClaimRefs: []string{}, EvidenceRefs: []string{}}, "Every finding, follow-up question and source is included." + tail},
		{BriefOmitted{Evidence: 1, EvidenceRefs: []string{"E-002"}}, "Not included: E-002." + tail},
		{BriefOmitted{Hypotheses: 1, Questions: 1, HypothesisRefs: []string{"H-003"}, QuestionRefs: []string{"Q-001"}}, "Not included: H-003 and Q-001." + tail},
		{BriefOmitted{Claims: 1, Hypotheses: 1, Questions: 2, Evidence: 1, ClaimRefs: []string{"C-004"}, HypothesisRefs: []string{"H-003"}, QuestionRefs: []string{"Q-001", "Q-002"}, EvidenceRefs: []string{"E-002"}}, "Not included: C-004, H-003, Q-001, Q-002 and E-002." + tail},
	} {
		if got := briefOmissionNotice(tc.o); got != tc.want {
			t.Errorf("briefOmissionNotice(%+v) = %q, want %q", tc.o, got, tc.want)
		}
	}
}

func TestBriefStateLabel(t *testing.T) {
	for _, tc := range []struct {
		got, want string
	}{
		{BriefStateLabel(domain.ClaimStatusActive.PlainLabel(), domain.VerificationAIGenerated), "Not checked"},
		{BriefStateLabel(domain.ClaimStatusDisputed.PlainLabel(), domain.VerificationDisputed), "Sources disagree"},
		{BriefStateLabel(domain.HypothesisStatusDisproven.PlainLabel(), domain.VerificationAIGenerated), "Ruled out; Not checked"},
		{BriefStateLabel(domain.ClaimStatusRejected.PlainLabel(), domain.VerificationHumanVerified), "Ruled out; You checked this"},
		{BriefStateLabel(domain.HypothesisStatusSupported.PlainLabel(), domain.VerificationSourceVerified), "Supported; AI checked the source"},
		{BriefStateLabel(domain.HypothesisStatusOpen.PlainLabel(), domain.VerificationAIGenerated), "Not tested yet; Not checked"},
	} {
		if tc.got != tc.want {
			t.Errorf("got %q, want %q", tc.got, tc.want)
		}
	}
}
