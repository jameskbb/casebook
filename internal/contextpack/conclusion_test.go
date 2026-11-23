package contextpack

import (
	"github.com/jameskbb/casebook/internal/domain"
	"strings"
	"testing"
)

func TestConclusionAccountabilitySurvivesAllRenderingsAndBudget(t *testing.T) {
	s := testSnapshot()
	s.Investigation.ConclusionVerification = domain.VerificationHumanVerified
	s.Investigation.ConclusionSupportingClaims = []string{"C-001"}
	s.Investigation.ConclusionContradictingClaims = []string{"C-002"}
	s.Investigation.ConclusionReviewedAt = "2026-10-20T12:00:00.000Z"
	s.Investigation.ConclusionReviewBasis = "Checked the export against selected claims."
	s.Investigation.ConclusionNeedsReview = true
	// The old human statement label remains, while the support now says disputed.
	s.Claims[0].Status = domain.ClaimStatusDisputed
	s.Claims[0].Verification = domain.VerificationDisputed
	p := Build(s, Options{Compact: true, MaxBytes: 100})
	if !p.Conclusion.NeedsReview || p.Conclusion.Verification != domain.VerificationHumanVerified || len(p.Conclusion.SupportingClaims) != 1 || len(p.Conclusion.ContradictingClaims) != 1 {
		t.Fatalf("budget dropped conclusion accountability: %+v", p.Conclusion)
	}
	js, err := RenderJSON(p)
	if err != nil {
		t.Fatal(err)
	}
	for name, text := range map[string]string{"markdown": RenderMarkdown(p), "json": string(js)} {
		for _, ref := range []string{"C-001", "C-002"} {
			if !strings.Contains(text, ref) {
				t.Errorf("%s lost selected ref %s", name, ref)
			}
		}
		if name == "json" {
			if !strings.Contains(text, `"verification": "human_verified"`) || !strings.Contains(text, `"needs_review": true`) {
				t.Errorf("JSON lost labels: %s", text)
			}
		} else if !strings.Contains(text, "Answer marked: A person checked this before, and it has changed since") || strings.Contains(text, "Answer marked: A person checked this\n") || !strings.Contains(text, "The answer needs checking again:") {
			t.Errorf("%s lost conclusion label or attention", name)
		}
	}
	// The shared answer names the same findings by their text, once each.
	shared := RenderReport(s, ReportOptions{})
	for _, want := range []string{
		"A person checked this answer on October 20, 2026. Something changed after that. It needs checking again.",
		"What was checked: Checked the export against selected claims.",
		"## Where sources disagree\n\n- " + s.Claims[0].Statement + "\n",
		"- " + s.Claims[1].Statement,
	} {
		if !strings.Contains(shared, want) {
			t.Errorf("shared answer lost %q:\n%s", want, shared)
		}
	}
}
