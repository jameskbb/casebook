package contextpack

import (
	"regexp"
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

// A pack is read by an AI client, so it keeps codes and a check by a person
// is never "You".
func TestConclusionBasisAndCheckTime(t *testing.T) {
	c := PackConclusion{Verification: domain.VerificationHumanVerified, ReviewedAt: "2026-10-27T03:29:47.051Z", SupportingClaims: []PackConclusionClaim{
		{Ref: "C-002", Status: domain.ClaimStatusActive, Verification: domain.VerificationAIGenerated},
		{Ref: "C-004", Status: domain.ClaimStatusSuperseded, Verification: domain.VerificationHumanVerified},
		{Ref: "C-005", Status: domain.ClaimStatusSuperseded, Verification: domain.VerificationSuperseded},
		{Ref: "C-009", Missing: true},
	}, ContradictingClaims: []PackConclusionClaim{{Ref: "C-001", Status: domain.ClaimStatusDisputed, Verification: domain.VerificationDisputed}}}
	got := strings.Join(conclusionAccountabilityLines(c), "\n")
	for _, want := range []string{"Answer marked: A person checked this", "Findings behind the answer: C-002 [Not checked], C-004 [Replaced, A person checked this], C-005 [Replaced], C-009 [missing finding]", "Findings against the answer: C-001 [Sources disagree]", "Answer checked by a person: 2026-10-27 03:29 UTC"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q:\n%s", want, got)
		}
	}
}

// refCode matches a record code, which a shared answer never prints.
var refCode = regexp.MustCompile(`\b(INV|[CEHQ])-\d+\b`)

// A shared answer names findings and sources by their own text, groups the
// findings the way the question page does, and leaves empty sections out.
func TestBriefSectionsAndNoCodes(t *testing.T) {
	s := Snapshot{Investigation: domain.Investigation{Ref: "INV-0001", Title: "T", Conclusion: "Because.", ConclusionVerification: domain.VerificationHumanVerified,
		ConclusionReviewedAt: "2026-11-01T17:50:00Z", ConclusionSupportingClaims: []string{"C-001"}, ConclusionContradictingClaims: []string{"C-003", "C-009"}},
		Claims: []domain.Claim{
			{Ref: "C-001", Statement: "kept", Status: domain.ClaimStatusActive, Verification: domain.VerificationAIGenerated, ConfidenceRationale: "a reason",
				Links: []domain.EvidenceLink{{EvidenceRef: "E-001", Relation: domain.RelationSupports}, {EvidenceRef: "E-002", Relation: domain.RelationContradicts}, {EvidenceRef: "E-007", Relation: domain.RelationSupports}}},
			{Ref: "C-002", Statement: "rejected one", Status: domain.ClaimStatusRejected, Verification: domain.VerificationAIGenerated},
			{Ref: "C-003", Statement: "argued", Status: domain.ClaimStatusDisputed, Verification: domain.VerificationDisputed},
			{Ref: "C-004", Statement: "old", Status: domain.ClaimStatusSuperseded, Verification: domain.VerificationSuperseded, Imported: true}},
		Hypotheses: []domain.Hypothesis{
			{Ref: "H-001", Statement: "dead end", Status: domain.HypothesisStatusDisproven, Verification: domain.VerificationAIGenerated},
			{Ref: "H-002", Statement: "live", Status: domain.HypothesisStatusOpen, Verification: domain.VerificationAIGenerated},
			{Ref: "H-003", Statement: "holding", Status: domain.HypothesisStatusSupported, Verification: domain.VerificationSourceVerified}},
		Questions: []domain.Question{
			{Ref: "Q-001", Question: "done?", Status: domain.QuestionStatusResolved, Resolution: "yes"},
			{Ref: "Q-002", Question: "wait?", Status: domain.QuestionStatusDeferred, DeferNote: "next month"}},
		Evidence: []domain.Evidence{
			{Ref: "E-001", Title: "Call log", SourceType: domain.SourceTypeSQLQuery, SourceRef: "shared drive", CapturedAt: "2026-10-30T09:00:00Z", Verification: domain.VerificationHumanVerified, Summary: "what it shows"},
			{Ref: "E-002", Title: "Phone report", SourceLabel: "Phone system", Verification: domain.VerificationAIGenerated}}}
	got := RenderBrief(s, BriefOmitted{}, s.Evidence)
	want := `# T

## Answer

Because.

1 of 2 sources checked by a person. 1 not checked.

A person checked this answer on November 1, 2026.

Findings that back the answer:
- kept (Not checked)

Findings that go against the answer:
- argued (Sources disagree)
- A finding that is no longer here

## Findings

- kept (Not checked)
  - Why: a reason
  - Backed by: Call log (A person checked this)
  - Backed by: a source that is not included here
  - Goes against it: Phone report (Not checked)
- holding (Looks right so far; AI checked the source)

## Where sources disagree

- argued

## Not tested yet

- live (Not checked)

## Ruled out

- rejected one (Not checked)
- dead end (Not checked)

## Replaced

- old (From a file)

## Follow-up questions

- done? (Answered)
  - Answer: yes
- wait? (Later)
  - Why it can wait: next month

## Sources

- Call log (A person checked this)
  - Numbers your AI looked up
  - Where to find it: shared drive
  - what it shows
- Phone report (Not checked)
  - Phone system

This short version leaves out the history. Ask the person who shared this for the version with everything.

_Made with Casebook. These are the saved findings and sources, word for word._
`
	if got != want {
		t.Errorf("brief:\n%s\nwant:\n%s", got, want)
	}
	if codes := refCode.FindAllString(got, -1); codes != nil {
		t.Errorf("shared answer prints codes %v", codes)
	}
	empty := RenderBrief(Snapshot{Investigation: domain.Investigation{Ref: "INV-0001", Title: "T"}}, BriefOmitted{}, nil)
	if empty != "# T\n\n## Answer\n\nNo answer yet.\n\n"+BriefLeftOut+"the history. Ask the person who shared this for the version with everything.\n\n"+answerFooter+"\n" {
		t.Errorf("empty question:\n%s", empty)
	}
}

// Who checked the answer is always stated, whatever its label.
func TestAnswerCheckLine(t *testing.T) {
	const at = "2026-11-01T17:50:00Z"
	for _, tc := range []struct {
		inv  domain.Investigation
		want string
	}{
		{domain.Investigation{}, "Nobody has checked this answer yet."},
		{domain.Investigation{ConclusionVerification: domain.VerificationAIGenerated, ConclusionReviewedAt: at}, "A person checked this answer on November 1, 2026. It changed after that, and nobody has checked it since."},
		{domain.Investigation{ConclusionVerification: domain.VerificationHumanVerified}, "A person checked this answer."},
		{domain.Investigation{ConclusionVerification: domain.VerificationHumanVerified, ConclusionReviewedAt: at, ConclusionNeedsReview: true}, "A person checked this answer on November 1, 2026. Something changed after that. It needs checking again."},
		{domain.Investigation{ConclusionVerification: domain.VerificationSourceVerified}, "An AI compared this answer with its sources. No person has checked it yet."},
		{domain.Investigation{ConclusionVerification: domain.VerificationDisputed, ConclusionReviewedAt: at}, "Sources disagree about this answer. A person last checked it on November 1, 2026."},
	} {
		if got := answerCheckLine(tc.inv); got != tc.want {
			t.Errorf("answerCheckLine(%+v) = %q, want %q", tc.inv, got, tc.want)
		}
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
		{Ref: "E-005", Verification: domain.VerificationDisputed},
	}
	s := Snapshot{Investigation: domain.Investigation{Ref: "INV-0001", Conclusion: "Because.", ConclusionConfidence: domain.ConfidenceHigh, ConclusionRationale: "The sources agree."}, Evidence: all[:1]}
	got := RenderBrief(s, BriefOmitted{}, all[:4])
	if !strings.Contains(got, "\nBecause.\n\n1 of 3 sources checked by a person. 1 checked by AI, 1 not checked.\n") || !strings.Contains(got, "\nWhy: The sources agree.\n") {
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
		{all[3:4], ""},
		{all[:1], "1 of 1 source checked by a person."},
		{all[2:3], "0 of 1 source checked by a person. 1 not checked."},
		// Like the desktop, everything neither a person nor an AI has checked
		// is "not checked", a source where sources disagree included.
		{all, "1 of 4 sources checked by a person. 1 checked by AI, 2 not checked."},
	} {
		if got := sourcesCheckedLine(tc.ev); got != tc.want {
			t.Errorf("sourcesCheckedLine = %q, want %q", got, tc.want)
		}
	}
}

func TestBriefOmissionNotice(t *testing.T) {
	const tail = ". Ask the person who shared this for the version with everything."
	for _, tc := range []struct {
		o    BriefOmitted
		want string
	}{
		{BriefOmitted{ClaimRefs: []string{}}, "the history"},
		{BriefOmitted{Evidence: 1, EvidenceRefs: []string{"E-002"}}, "1 source and the history"},
		{BriefOmitted{Claims: 1, Questions: 1}, "1 finding, 1 follow-up question and the history"},
		{BriefOmitted{ClaimRefs: []string{"C-004"}, HypothesisRefs: []string{"H-003"}, QuestionRefs: []string{"Q-001", "Q-002"}}, "2 findings, 2 follow-up questions and the history"},
		{BriefOmitted{Claims: 3, Hypotheses: 1, Questions: 2, Evidence: 4}, "4 findings, 2 follow-up questions, 4 sources and the history"},
	} {
		if got := briefOmissionNotice(tc.o); got != BriefLeftOut+tc.want+tail {
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
		{BriefStateLabel(domain.ClaimStatusRejected.PlainLabel(), domain.VerificationHumanVerified), "Ruled out; A person checked this"},
		{BriefStateLabel(domain.HypothesisStatusSupported.PlainLabel(), domain.VerificationSourceVerified), "Looks right so far; AI checked the source"},
		{BriefStateLabel(domain.HypothesisStatusOpen.PlainLabel(), domain.VerificationAIGenerated), "Not tested yet; Not checked"},
	} {
		if tc.got != tc.want {
			t.Errorf("got %q, want %q", tc.got, tc.want)
		}
	}
}
