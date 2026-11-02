// Package cli implements the casebook command tree. Every command reaches
// data only through internal/app.
package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
)

const rootLong = `Keep the evidence and findings for a question in one case. Return to
them in a later session, review them in the desktop app, or print a summary
to share.

A case holds evidence, claims, hypotheses, questions and a conclusion.
This CLI, the desktop app and connected AI clients use the same local cases.
Run casebook setup --help for AI client setup.

Exit codes: 0 success; 1 runtime failure; 2 usage or invalid input; 3 not found or no active case.`

// NewRootCommand builds the casebook command tree bound to env.
func NewRootCommand(env *Env) *cobra.Command {
	root := &cobra.Command{
		Use:           "casebook",
		Short:         "Keep evidence and findings for each question in a case",
		Long:          rootLong,
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runRoot(cmd, env)
		},
	}
	root.CompletionOptions.DisableDefaultCmd = true
	// casebook --version prints the same line as casebook version.
	root.Version = Version
	root.SetVersionTemplate("casebook {{.Version}}\n")
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return &usageError{err: err}
	})
	root.PersistentFlags().StringVar(&env.DBPath, "db", "",
		"database file (default: CASEBOOK_DB, CASEBOOK_HOME or the per-user data directory)")
	root.PersistentFlags().BoolVar(&env.JSON, "json", false, "machine-readable JSON output")
	root.SetIn(env.Stdin)
	root.SetOut(env.Stdout)
	root.SetErr(env.Stderr)
	for _, f := range commandFactories() {
		root.AddCommand(f(env))
	}
	return root
}

// runRoot is `casebook` with no arguments (CLI-01): a short status block
// followed by the help. With --json it prints the status as JSON. When the
// status cannot be read (the database does not open, is busy, or the active
// investigation cannot be loaded) it still prints the help, adds a one-line
// note on stderr and succeeds. A database that does not exist yet is not
// created (AR-22): the status says so (schema 0, no records) and the first
// command that needs data creates it.
func runRoot(cmd *cobra.Command, env *Env) error {
	if path, missing := missingDB(env); missing {
		if env.JSON {
			return writeJSON(env.Stdout, missingStatusInfo(path))
		}
		fmt.Fprintf(env.Stdout, "Casebook %s\n", Version)
		if err := writeMissingStatus(env.Stdout, path); err != nil {
			return err
		}
		fmt.Fprintln(env.Stdout)
		return cmd.Help()
	}
	info, active, err := loadStatus(cmd.Context(), env)
	if env.JSON {
		if err != nil {
			return err
		}
		return writeJSON(env.Stdout, info)
	}
	if err != nil {
		if herr := cmd.Help(); herr != nil {
			return herr
		}
		fmt.Fprintln(env.Stderr, "casebook: note: could not read Casebook status: "+
			strings.TrimPrefix(formatError(err), "casebook: "))
		return nil
	}
	fmt.Fprintf(env.Stdout, "Casebook %s\n", Version)
	if err := writeStatus(env.Stdout, info, active); err != nil {
		return err
	}
	fmt.Fprintln(env.Stdout)
	return cmd.Help()
}

// Execute runs casebook with args and the given streams and returns the exit
// code.
func Execute(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return run(ctx, NewEnv(stdin, stdout, stderr), args)
}

// run executes the command tree against env, closes the App the command
// opened, prints any error as one line on stderr and returns the exit code.
func run(ctx context.Context, env *Env, args []string) int {
	if args == nil {
		args = []string{} // cobra reads os.Args when args are nil
	}
	root := NewRootCommand(env)
	root.SetArgs(args)
	err := root.ExecuteContext(ctx)
	if cerr := env.Close(); cerr != nil && err == nil {
		err = cerr
	}
	if err != nil {
		fmt.Fprintln(env.Stderr, formatError(err))
	}
	return exitCode(err)
}
