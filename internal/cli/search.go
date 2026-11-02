package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jameskbb/casebook/internal/app"
)

// searchSnippetBytes caps a snippet in human output.
const searchSnippetBytes = 160

// newSearchCommand runs a full-text search over investigations and records.
func newSearchCommand(env *Env) *cobra.Command {
	var o app.SearchOptions
	cmd := &cobra.Command{
		Use:   "search <query...>",
		Short: "Full-text search over cases, evidence, claims, hypotheses and questions",
		Long: `Full-text search over case titles, problem statements and
conclusions, evidence titles and summaries (never raw detail), claim and
hypothesis statements, questions and resolutions. The words are matched as
plain text (all must match; the last also matches as a prefix). Results are
ranked best first. Archived cases are skipped unless --archived.
Put words that start with "-" after "--" so they are not read as flags.`,
		Example: `  casebook search debug
  casebook search log ingestion --inv INV-0001
  casebook search --inv INV-0001 -- -claim`,
		Args: usageArgs(cobra.MinimumNArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := env.App()
			if err != nil {
				return err
			}
			hits, err := a.Search(cmd.Context(), strings.Join(args, " "), o)
			if err != nil {
				return err
			}
			if hits == nil {
				hits = []app.SearchHit{}
			}
			if env.JSON {
				return writeJSON(env.Stdout, hits)
			}
			if len(hits) == 0 {
				_, err := fmt.Fprintln(env.Stdout, "No matches.")
				return err
			}
			tw := newTabWriter(env.Stdout)
			fmt.Fprintln(tw, "CASE\tTYPE\tREF\tMATCH")
			for _, h := range hits {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", h.InvestigationRef, h.RecordType, h.RecordRef,
					app.TruncateLine(h.Snippet, searchSnippetBytes))
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&o.InvestigationRef, "inv", "", "search only this case")
	cmd.Flags().BoolVar(&o.IncludeArchived, "archived", false, "include archived cases")
	cmd.Flags().IntVar(&o.Limit, "limit", 0, "maximum number of results (0 means 20, maximum 100)")
	return cmd
}
