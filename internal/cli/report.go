package cli

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/jameskbb/casebook/internal/app"
)

// newReportCommand prints the shared answer of an investigation (the
// active one when no ref is given). The text is exactly what app.Report
// returns (or, with --brief, the Markdown of app.SharePreview in brief mode
// with its default selection), so the CLI, the MCP tool and the desktop
// app's Share the answer agree byte for byte.
func newReportCommand(env *Env) *cobra.Command {
	var (
		output   string
		force    bool
		timeline int
		brief    bool
	)
	cmd := &cobra.Command{
		Use:   "report [ref]",
		Short: "Print the answer to a question, ready to share (default: the active one)",
		Long: `Print the answer to a question as Markdown for people to read. It includes the
answer and who checked it, the findings and their sources, what was ruled out,
what is still to find out and a dated history of changes. Records are named by
their own text, without refs. With no ref, use the active question.

--brief prints the short version instead: what the app's Share the answer
starts with, ending with a note of what it leaves out. The short version has
no history, so --timeline cannot be combined with --brief.

The output is deterministic: the same records always give the same bytes.
-o writes it to a new owner-only file (never overwriting one unless --force
is given). --timeline N keeps only the newest N history entries. --json
prints {"investigation_ref", "mode", "report"} instead of the bare Markdown
(mode is "report" or "brief").`,
		Example: `  casebook report
  casebook report --brief
  casebook report INV-0002 -o answer.md
  casebook report --timeline 20`,
		Args: usageArgs(cobra.MaximumNArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			ref := ""
			if len(args) == 1 {
				ref = args[0]
			}
			if brief && cmd.Flags().Changed("timeline") {
				return newUsageError("--timeline cannot be used with --brief (the short version has no history)")
			}
			a, err := env.App()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			inv, err := a.GetInvestigation(ctx, ref)
			if err != nil {
				return err
			}
			mode, kind := "report", "everything"
			var text string
			if brief {
				mode, kind = "brief", "the short version"
				// Nil selections: what Share the answer opens with.
				p, err := a.SharePreview(ctx, inv.Ref, app.ShareOptions{Mode: "brief"})
				if err != nil {
					return err
				}
				text = p.Markdown
			} else {
				text, err = a.Report(ctx, inv.Ref, app.ReportOptions{TimelineLimit: timeline})
				if err != nil {
					return err
				}
			}
			if output == "" {
				if env.JSON {
					return writeJSON(env.Stdout, map[string]any{"investigation_ref": inv.Ref, "mode": mode, "report": text})
				}
				return writeText(env.Stdout, text)
			}
			path := output
			if abs, err := filepath.Abs(output); err == nil {
				path = abs
			}
			if err := writeExportFile(path, a.DBPath(), []byte(text), force); err != nil {
				return err
			}
			if env.JSON {
				return writeJSON(env.Stdout, map[string]any{
					"investigation_ref": inv.Ref, "mode": mode, "path": path, "bytes": len(text),
				})
			}
			_, err = fmt.Fprintf(env.Stdout, "Wrote %s for %s to %s (%d bytes).\n", kind, inv.Ref, path, len(text))
			return err
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "", "write to this file instead of stdout")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite the -o file if it exists")
	cmd.Flags().IntVar(&timeline, "timeline", 0, "keep only the newest N history entries (0 means all; not with --brief)")
	cmd.Flags().BoolVar(&brief, "brief", false, "print the short version instead of everything")
	return cmd
}
