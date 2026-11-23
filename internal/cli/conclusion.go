package cli

import (
	"fmt"
	"github.com/jameskbb/casebook/internal/app"
	"github.com/spf13/cobra"
)

func newInvestigationConclusionCommand(env *Env) *cobra.Command {
	var in app.ConclusionInput
	var verification, confidence string
	cmd := &cobra.Command{Use: "conclude [ref]", Short: "Write the answer to a question, or record that you checked it (the conclusion is the answer)", Args: usageArgs(cobra.MaximumNArgs(1)), RunE: func(cmd *cobra.Command, args []string) error {
		ref := ""
		if len(args) > 0 {
			ref = args[0]
		}
		if !cmd.Flags().Changed("text") && !in.Review {
			return newUsageError("pass --text to write or clear the answer, or --review to record that you checked the saved answer")
		}
		a, err := env.App()
		if err != nil {
			return err
		}
		if !cmd.Flags().Changed("text") {
			before, err := a.GetInvestigation(cmd.Context(), ref)
			if err != nil {
				return err
			}
			in.Conclusion, in.Confidence, in.Rationale = before.Conclusion, before.ConclusionConfidence, before.ConclusionRationale
		}
		if cmd.Flags().Changed("confidence") {
			in.Confidence = app.Confidence(confidence)
		}
		in.Verification = app.Verification(verification)
		got, err := a.UpdateConclusion(cmd.Context(), app.ActorCLI(), ref, in)
		if err != nil {
			return err
		}
		if env.JSON {
			return writeJSON(env.Stdout, got)
		}
		_, err = fmt.Fprintf(env.Stdout, "Answer to %s: %s; needs checking again: %t\n", got.Ref, got.ConclusionVerification.PlainLabel(), got.ConclusionNeedsReview)
		return err
	}}
	cmd.Flags().StringVar(&in.Conclusion, "text", "", "The answer, in plain words; an empty string clears it")
	cmd.Flags().StringVar(&confidence, "confidence", "", "unknown, low, medium or high (stored; the app does not show it)")
	cmd.Flags().StringVar(&in.Rationale, "rationale", "", "Why the answer holds")
	cmd.Flags().StringVar(&verification, "verification", "", "ai_generated (not checked), human_verified (you checked this), source_verified (AI checked the source) or disputed (sources disagree)")
	cmd.Flags().StringSliceVar(&in.SupportingClaims, "supporting-claims", nil, "Refs of the findings (claims) the answer rests on (comma separated)")
	cmd.Flags().StringSliceVar(&in.ContradictingClaims, "contradicting-claims", nil, "Refs of the findings (claims) that go against the answer (comma separated)")
	cmd.Flags().BoolVar(&in.Review, "review", false, "Record that you checked the saved answer and the findings it rests on")
	cmd.Flags().StringVar(&in.ReviewBasis, "review-basis", "", "What you checked")
	cmd.Flags().StringVar(&in.ExpectedReviewToken, "expected-review-token", "", "conclusion_review_token from investigation show --json; refuses changed material")
	return cmd
}
