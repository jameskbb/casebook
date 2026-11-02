package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jameskbb/casebook/internal/app"
)

// newInvestigationCommand is the investigation group (alias inv). With no
// subcommand it prints its help.
func newInvestigationCommand(env *Env) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "investigation",
		Aliases: []string{"case", "inv"},
		Short:   "List, show, create and activate cases",
		Long: `List, show, create and activate cases.

A case is one question and everything learned about it; the app and the
guides call it a case, and its ref starts with INV. The active case (the app
calls it your AI's current case) is shared by the desktop app, this CLI and
every MCP client: commands that take an optional case ref use it when the ref
is omitted. A project directory can have its own active case (activate
--here), which wins inside that directory.`,
		Args: usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(
		newInvestigationListCommand(env),
		newInvestigationShowCommand(env),
		newInvestigationActiveCommand(env),
		newInvestigationActivateCommand(env),
		newInvestigationCreateCommand(env),
		newInvestigationConclusionCommand(env),
		newInvestigationDirsCommand(env),
	)
	return cmd
}

// Byte caps for user text in human output; JSON output is never cut.
const (
	listTitleBytes = 120
	showTextBytes  = 240
)

// newInvestigationListCommand lists investigation summaries, newest updated
// first.
func newInvestigationListCommand(env *Env) *cobra.Command {
	var o app.ListOptions
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List cases, newest updated first (archived hidden unless --archived)",
		Args:  usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			a, err := env.App()
			if err != nil {
				return err
			}
			sums, err := a.ListInvestigations(cmd.Context(), o)
			if err != nil {
				return err
			}
			if sums == nil {
				sums = []app.InvestigationSummary{}
			}
			if env.JSON {
				return writeJSON(env.Stdout, sums)
			}
			if len(sums) == 0 {
				_, err := fmt.Fprintln(env.Stdout, `No cases. Create one with: casebook case create --title "..."`)
				return err
			}
			tw := newTabWriter(env.Stdout)
			fmt.Fprintln(tw, "REF\tSTATUS\tCONFIDENCE\tUPDATED\tACTIVE\tTITLE")
			for _, s := range sums {
				active := ""
				if s.IsActive {
					active = "*"
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", s.Ref, s.Status.Label(), confidenceLabel(s.ConclusionConfidence),
					s.UpdatedAt, active, app.TruncateLine(s.Title, listTitleBytes))
			}
			return tw.Flush()
		},
	}
	cmd.Flags().BoolVar(&o.IncludeArchived, "archived", false, "include archived cases")
	cmd.Flags().StringVar(&o.Status, "status", "", "only cases with this status (new, investigating, blocked, resolved, archived)")
	cmd.Flags().IntVar(&o.Limit, "limit", 0, "maximum number of cases (0 means 100, maximum 1000)")
	return cmd
}

// newInvestigationShowCommand prints one investigation with every record.
// It never requests raw evidence detail.
func newInvestigationShowCommand(env *Env) *cobra.Command {
	return &cobra.Command{
		Use:   "show [ref]",
		Short: "Show a case with its evidence, claims, hypotheses, questions and recent activity",
		Args:  usageArgs(cobra.MaximumNArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			ref := ""
			if len(args) == 1 {
				ref = args[0]
			}
			a, err := env.App()
			if err != nil {
				return err
			}
			d, err := a.GetInvestigationDetail(cmd.Context(), ref, app.DetailOptions{})
			if err != nil {
				return err
			}
			if env.JSON {
				return writeJSON(env.Stdout, d)
			}
			return writeDetail(env.Stdout, d)
		},
	}
}

// writeDetail prints an investigation detail as plain aligned text. Every
// user text value is cut to one line.
func writeDetail(w io.Writer, d app.InvestigationDetail) error {
	line := func(s string) string { return app.TruncateLine(s, showTextBytes) }
	inv := d.Investigation
	tw := newTabWriter(w)
	fmt.Fprintf(tw, "%s  %s\n", inv.Ref, line(inv.Title))
	fmt.Fprintf(tw, "Status:\t%s\n", inv.Status.Label())
	fmt.Fprintf(tw, "Active:\t%s\n", yesNo(inv.IsActive))
	fmt.Fprintf(tw, "Question:\t%s\n", orNone(line(inv.ProblemStatement)))
	if inv.Conclusion == "" {
		fmt.Fprintf(tw, "Conclusion:\t%s\n", "(none)")
	} else {
		fmt.Fprintf(tw, "Conclusion:\t%s (%s)\n", line(inv.Conclusion), confidenceLabel(inv.ConclusionConfidence)+", "+inv.ConclusionVerification.Label())
	}
	if inv.Conclusion != "" {
		fmt.Fprintf(tw, "Conclusion review:\t%s; needs review: %t\n", orNone(inv.ConclusionReviewedAt), inv.ConclusionNeedsReview)
		fmt.Fprintf(tw, "Review basis:\t%s\n", orNone(line(inv.ConclusionReviewBasis)))
		fmt.Fprintf(tw, "Supporting claims:\t%s\n", orNone(strings.Join(inv.ConclusionSupportingClaims, ", ")))
		fmt.Fprintf(tw, "Contradicting claims:\t%s\n", orNone(strings.Join(inv.ConclusionContradictingClaims, ", ")))
	}
	if inv.ConclusionRationale != "" {
		fmt.Fprintf(tw, "Rationale:\t%s\n", line(inv.ConclusionRationale))
	}
	fmt.Fprintf(tw, "Updated:\t%s\n", inv.UpdatedAt)

	fmt.Fprintf(tw, "\nEvidence (%d)\n", len(d.Evidence))
	for _, e := range d.Evidence {
		src := ""
		if e.SourceLabel != "" {
			src = "source: " + line(e.SourceLabel)
		}
		fmt.Fprintf(tw, "  %s\t[%s, %s]\t%s%s\n", e.Ref, e.SourceType.Label(), e.Verification.Label()+importedNote(e.Imported), line(e.Title), cell(src))
	}
	fmt.Fprintf(tw, "\nClaims (%d)\n", len(d.Claims))
	for _, c := range d.Claims {
		fmt.Fprintf(tw, "  %s\t[%s]\t%s%s\n", c.Ref, statementState(c.Status.Label(), c.Confidence, c.Verification.Label(), c.Imported),
			line(c.Statement), cell(linkText(c.Links, c.SupersededBy)))
	}
	fmt.Fprintf(tw, "\nHypotheses (%d)\n", len(d.Hypotheses))
	for _, h := range d.Hypotheses {
		fmt.Fprintf(tw, "  %s\t[%s]\t%s%s\n", h.Ref, statementState(h.Status.Label(), h.Confidence, h.Verification.Label(), h.Imported),
			line(h.Statement), cell(linkText(h.Links, h.SupersededBy)))
	}
	fmt.Fprintf(tw, "\nQuestions (%d)\n", len(d.Questions))
	for _, q := range d.Questions {
		extra := ""
		switch {
		case q.Resolution != "":
			extra = "resolution: " + line(q.Resolution)
		case q.DeferNote != "":
			extra = "note: " + line(q.DeferNote)
		}
		fmt.Fprintf(tw, "  %s\t[%s]\t%s%s\n", q.Ref, q.Status.Label(), line(q.Question), cell(extra))
	}
	fmt.Fprintf(tw, "\nRecent activity (%d)\n", len(d.RecentActivity))
	for _, act := range d.RecentActivity {
		fmt.Fprintf(tw, "  %s\t%s %s%s\t%s\t%s\t%s\n", act.CreatedAt, act.Actor.Kind, line(act.Actor.Client), importedNote(act.Imported),
			act.Action, act.TargetRef, line(act.Summary))
	}
	return tw.Flush()
}

// importedNote is " (imported)" for a row copied from an imported file, so
// its author and verification are not read as this database's own (AR-09).
func importedNote(imported bool) string {
	if imported {
		return " (imported)"
	}
	return ""
}

// linkText lists supporting and contradicting evidence refs and the
// replacement of a superseded record.
func linkText(links []app.EvidenceLink, supersededBy string) string {
	var sup, con []string
	for _, l := range links {
		if l.Relation == app.RelationContradicts {
			con = append(con, l.EvidenceRef)
		} else {
			sup = append(sup, l.EvidenceRef)
		}
	}
	var parts []string
	if len(sup) > 0 {
		parts = append(parts, "supports: "+strings.Join(sup, ", "))
	}
	if len(con) > 0 {
		parts = append(parts, "contradicts: "+strings.Join(con, ", "))
	}
	if supersededBy != "" {
		parts = append(parts, "superseded by "+supersededBy)
	}
	return strings.Join(parts, "; ")
}

// cell returns s as a trailing tab-separated cell, or nothing when s is
// empty, so rows without it carry no trailing padding.
func cell(s string) string {
	if s == "" {
		return ""
	}
	return "\t" + s
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func orNone(s string) string {
	if s == "" {
		return "(none)"
	}
	return s
}

// newInvestigationActiveCommand prints the active investigation.
func newInvestigationActiveCommand(env *Env) *cobra.Command {
	return &cobra.Command{
		Use:   "active",
		Short: "Show the active case",
		Args:  usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			a, err := env.App()
			if err != nil {
				return err
			}
			inv, err := a.GetActiveInvestigation(cmd.Context())
			if err != nil {
				return err
			}
			if inv == nil {
				return app.ErrNoActiveInvestigation
			}
			if env.JSON {
				return writeJSON(env.Stdout, inv)
			}
			info, err := a.ActiveInvestigationInfo(cmd.Context())
			if err != nil {
				return err
			}
			scope := ""
			if info.Scope == app.ActiveScopeDirectory {
				scope = "  for " + info.Dir
			}
			_, err = fmt.Fprintf(env.Stdout, "%s  %s  (%s)%s\n", inv.Ref, app.TruncateLine(inv.Title, showTextBytes), inv.Status.Label(), scope)
			return err
		},
	}
}

// newInvestigationCreateCommand creates an investigation, optionally making
// it active.
func newInvestigationCreateCommand(env *Env) *cobra.Command {
	var in app.CreateInvestigationInput
	cmd := &cobra.Command{
		Use:     "create --title T [--problem P] [--activate]",
		Short:   "Create a case",
		Example: `  casebook case create --title "Refund spike" --problem "Why did refunds double?" --activate`,
		Args:    usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if in.Title == "" {
				return newUsageError("--title is required")
			}
			a, err := env.App()
			if err != nil {
				return err
			}
			inv, err := a.CreateInvestigation(cmd.Context(), app.ActorCLI(), in)
			if err != nil {
				return err
			}
			if env.JSON {
				return writeJSON(env.Stdout, inv)
			}
			suffix := ""
			if inv.IsActive {
				suffix = " (now active)"
			}
			_, err = fmt.Fprintf(env.Stdout, "Created %s %q%s\n", inv.Ref, inv.Title, suffix)
			return err
		},
	}
	cmd.Flags().StringVar(&in.Title, "title", "", "title (required, at most 200 bytes)")
	cmd.Flags().StringVar(&in.ProblemStatement, "problem", "", "the primary question the case answers")
	cmd.Flags().BoolVar(&in.MakeActive, "activate", false, "make the new case the active one")
	return cmd
}

// newInvestigationActivateCommand sets or clears the active investigation,
// globally or for the current project directory (ADR-0015).
func newInvestigationActivateCommand(env *Env) *cobra.Command {
	var clearActive, here, global bool
	cmd := &cobra.Command{
		Use:   "activate [ref]",
		Short: "Set the active case, or clear it with --clear",
		Long: `Set the active case, or clear it with --clear.

--here makes it the active case for this project only: the git repository
root holding the current directory (or the current directory outside a
repository). The CLI, the hooks and the MCP server working in that
directory or below it then use it instead of the global active case, so
Claude Code and Codex sessions in different repositories each pick up
their own case. --clear --here removes that binding.

Without --here, inside a directory that already has a binding the binding
is changed; elsewhere the global active case is set. --global always sets
the global one (a binding still wins where it applies).`,
		Example: `  casebook case activate INV-0001
  casebook case activate INV-0002 --here
  casebook inv activate --clear --here
  casebook inv activate --clear`,
		Args: usageArgs(cobra.MaximumNArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			if (len(args) == 1) == clearActive {
				return newUsageError("pass a case ref or --clear")
			}
			if here && global {
				return newUsageError("--here cannot be combined with --global")
			}
			a, err := env.App()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			switch {
			case clearActive && here:
				return runUnbindHere(cmd, env, a)
			case clearActive:
				if err := a.ClearActiveInvestigation(ctx, app.ActorCLI()); err != nil {
					return err
				}
				if env.JSON {
					return writeJSON(env.Stdout, map[string]string{"active_investigation_ref": ""})
				}
				if _, err := fmt.Fprintln(env.Stdout, "Cleared the active case."); err != nil {
					return err
				}
				return noteBindingWins(cmd, env, a)
			case here:
				b, err := a.BindDirectory(ctx, app.ActorCLI(), "", args[0])
				if err != nil {
					return err
				}
				if env.JSON {
					return writeJSON(env.Stdout, b)
				}
				_, err = fmt.Fprintf(env.Stdout, "Active case for %s: %s %s\n", b.Dir, b.InvestigationRef, b.Title)
				return err
			}
			set := a.SetActiveInvestigation
			if global {
				set = a.SetGlobalActiveInvestigation
			}
			inv, err := set(ctx, app.ActorCLI(), args[0])
			if err != nil {
				return err
			}
			if env.JSON {
				return writeJSON(env.Stdout, inv)
			}
			info, err := a.ActiveInvestigationInfo(ctx)
			if err != nil {
				return err
			}
			if !global && info.Scope == app.ActiveScopeDirectory {
				_, err = fmt.Fprintf(env.Stdout, "Active case for %s: %s %s\n", info.Dir, inv.Ref, inv.Title)
				return err
			}
			if _, err := fmt.Fprintf(env.Stdout, "Active case: %s %s\n", inv.Ref, inv.Title); err != nil {
				return err
			}
			return noteBindingWins(cmd, env, a)
		},
	}
	cmd.Flags().BoolVar(&clearActive, "clear", false, "clear the active case")
	cmd.Flags().BoolVar(&here, "here", false, "set or clear the active case for this project directory only")
	cmd.Flags().BoolVar(&global, "global", false, "set the global active case even inside a bound directory")
	return cmd
}

// runUnbindHere is activate --clear --here: it removes the binding that
// applies to the current directory.
func runUnbindHere(cmd *cobra.Command, env *Env, a *app.App) error {
	dir, err := a.UnbindDirectory(cmd.Context(), app.ActorCLI(), "")
	if err != nil {
		return err
	}
	if env.JSON {
		return writeJSON(env.Stdout, map[string]string{"dir": dir})
	}
	if dir == "" {
		_, err := fmt.Fprintln(env.Stdout, "No active case is set for this directory.")
		return err
	}
	_, err = fmt.Fprintf(env.Stdout, "Removed the active case for %s.\n", dir)
	return err
}

// noteBindingWins prints a stderr note when a directory binding decides the
// active investigation here, so a global change is not what this directory
// sees.
func noteBindingWins(cmd *cobra.Command, env *Env, a *app.App) error {
	info, err := a.ActiveInvestigationInfo(cmd.Context())
	if err != nil {
		return err
	}
	if info.Scope == app.ActiveScopeDirectory {
		fmt.Fprintf(env.Stderr, "casebook: note: %s is the active case for %s; change it with --here\n",
			info.Investigation.Ref, info.Dir)
	}
	return nil
}

// newInvestigationDirsCommand lists the directory bindings.
func newInvestigationDirsCommand(env *Env) *cobra.Command {
	return &cobra.Command{
		Use:   "dirs",
		Short: "List directories with their own active case (set with activate --here)",
		Args:  usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			a, err := env.App()
			if err != nil {
				return err
			}
			list, err := a.ListDirectoryBindings(cmd.Context())
			if err != nil {
				return err
			}
			if env.JSON {
				return writeJSON(env.Stdout, list)
			}
			if len(list) == 0 {
				_, err := fmt.Fprintln(env.Stdout, "No directories have their own active case. Set one with: casebook case activate <ref> --here")
				return err
			}
			tw := newTabWriter(env.Stdout)
			fmt.Fprintln(tw, "REF\tDIRECTORY\tTITLE")
			for _, b := range list {
				fmt.Fprintf(tw, "%s\t%s\t%s\n", b.InvestigationRef, b.Dir, app.TruncateLine(b.Title, listTitleBytes))
			}
			return tw.Flush()
		},
	}
}

// statementState is the bracket in a case show row: status, confidence, then
// the verification label only when it differs from the status, so a disputed
// claim reads "Disputed, Low" rather than "Disputed, Low, Disputed".
func statementState(status string, c app.Confidence, verification string, imported bool) string {
	parts := []string{status, confidenceLabel(c)}
	if verification != status {
		parts = append(parts, verification)
	}
	return strings.Join(parts, ", ") + importedNote(imported)
}

// confidenceLabel is the confidence word the CLI tables print: the app's
// "Not assessed" where no confidence is recorded (Confidence.Label says
// Unknown, which context packs and other formats keep).
func confidenceLabel(c app.Confidence) string {
	if c == app.ConfidenceUnknown {
		return "Not assessed"
	}
	return c.Label()
}
