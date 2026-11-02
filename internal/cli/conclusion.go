package cli

import (
	"fmt"
	"github.com/jameskbb/casebook/internal/app"
	"github.com/spf13/cobra"
)

func newInvestigationConclusionCommand(env *Env) *cobra.Command {
	var in app.ConclusionInput
	var verification, confidence string
	cmd := &cobra.Command{Use: "conclude [ref]", Short: "Write or review a case conclusion", Args: usageArgs(cobra.MaximumNArgs(1)), RunE: func(cmd *cobra.Command, args []string) error {
		ref := ""
		if len(args) > 0 {
			ref = args[0]
		}
		if !cmd.Flags().Changed("text") && !in.Review {
			return newUsageError("pass --text to write or clear a conclusion, or --review to review the saved conclusion")
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
		_, err = fmt.Fprintf(env.Stdout, "Conclusion of %s: %s; needs review: %t\n", got.Ref, got.ConclusionVerification.Label(), got.ConclusionNeedsReview)
		return err
	}}
	cmd.Flags().StringVar(&in.Conclusion, "text", "", "Conclusion text; an empty string clears it")
	cmd.Flags().StringVar(&confidence, "confidence", "", "unknown, low, medium or high")
	cmd.Flags().StringVar(&in.Rationale, "rationale", "", "Why this confidence")
	cmd.Flags().StringVar(&verification, "verification", "", "ai_generated, human_verified, source_verified or disputed")
	cmd.Flags().StringSliceVar(&in.SupportingClaims, "supporting-claims", nil, "Selected supporting claim refs (comma separated)")
	cmd.Flags().StringSliceVar(&in.ContradictingClaims, "contradicting-claims", nil, "Selected contradicting claim refs (comma separated)")
	cmd.Flags().BoolVar(&in.Review, "review", false, "Record a person's review of the saved selected basis")
	cmd.Flags().StringVar(&in.ReviewBasis, "review-basis", "", "What you checked in this review")
	cmd.Flags().StringVar(&in.ExpectedReviewToken, "expected-review-token", "", "conclusion_review_token from investigation show --json; refuses changed material")
	return cmd
}
