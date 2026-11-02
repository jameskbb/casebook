package cli

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/jameskbb/casebook/internal/app"
)

// newReportCommand prints the Markdown write-up of an investigation (the
// active one when no ref is given). The text is exactly what app.Report
// returns (or, with --brief, the Markdown of app.SharePreview in brief mode
// with its default selection), so the CLI, the MCP tool and the desktop
// app's Share summary agree byte for byte.
func newReportCommand(env *Env) *cobra.Command {
	var (
		output   string
		force    bool
		timeline int
		brief    bool
	)
	cmd := &cobra.Command{
		Use:   "report [ref]",
		Short: "Print a Markdown write-up of a case (default: the active one)",
		Long: `Print a case report for people to read or share. It includes the conclusion
with its confidence and verification, findings and their evidence, what was
ruled out, answered and open questions, and a dated history of changes (UTC).
With no ref, use the active case.

--brief prints the leadership brief instead: the short summary the app's
Share summary starts with, including its note of what it leaves out. A brief
has no timeline, so --timeline cannot be combined with --brief.

The output is deterministic: the same records always give the same bytes.
-o writes it to a new owner-only file (never overwriting one unless --force
is given). --timeline N keeps only the newest N timeline entries. --json
prints {"investigation_ref", "mode", "report"} instead of the bare Markdown
(mode is "report" or "brief").`,
		Example: `  casebook report
  casebook report --brief
  casebook report INV-0002 -o postmortem.md
  casebook report --timeline 20 | gh pr create --body-file -`,
		Args: usageArgs(cobra.MaximumNArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			ref := ""
			if len(args) == 1 {
				ref = args[0]
			}
			if brief && cmd.Flags().Changed("timeline") {
				return newUsageError("--timeline cannot be used with --brief (a brief has no timeline)")
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
			mode, kind := "report", "report"
			var text string
			if brief {
				mode, kind = "brief", "leadership brief"
				// Nil selections: the default brief the Share summary opens with.
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
			_, err = fmt.Fprintf(env.Stdout, "Wrote the %s %s to %s (%d bytes).\n", inv.Ref, kind, path, len(text))
			return err
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "", "write to this file instead of stdout")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite the -o file if it exists")
	cmd.Flags().IntVar(&timeline, "timeline", 0, "keep only the newest N timeline entries (0 means all; not with --brief)")
	cmd.Flags().BoolVar(&brief, "brief", false, "print the leadership brief instead of the complete report")
	return cmd
}
