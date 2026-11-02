package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jameskbb/casebook/internal/integrations"
)

// newSetupCommand is the setup group. With no subcommand it prints its help.
func newSetupCommand(env *Env) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Connect AI clients (Claude Code, Codex, any MCP client) to Casebook",
		Long: `Connect AI clients (Claude Code, Codex) to Casebook by registering the
Casebook MCP server in their user config. For any other MCP client (Cursor,
VS Code, Windsurf, Gemini CLI, Claude Desktop, ...), "casebook setup mcp"
prints the configuration to add by hand and writes nothing.

Setup merges only Casebook-owned entries and leaves everything else in the
file untouched. Before changing a file it writes a timestamped backup next to
it (<file>.casebook-backup-<UTC time>) and keeps the newest 5 such backups
of that file, removing older ones. It refuses to change a file it cannot
parse. It is safe to rerun: a run that would change nothing writes
nothing. --print shows the configuration without writing anything.`,
		Args: usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newSetupClaudeCommand(env), newSetupCodexCommand(env), newSetupMCPCommand(env), newSetupInstructionsCommand(env))
	return cmd
}

// setupReport is what a setup command did (or would do, with --print).
type setupReport struct {
	// Client is "claude" or "codex".
	Client  string `json:"client"`
	CLIPath string `json:"cli_path"`
	DryRun  bool   `json:"dry_run"`
	// Preview is the --print text, "" otherwise.
	Preview string `json:"preview"`
	// Next is the one-line next step.
	Next string `json:"next"`
	// Changes lists the files written. Never nil.
	Changes []integrations.FileChange `json:"changes"`
}

// writeSetupReport prints r as JSON with --json, otherwise as text: the
// preview verbatim for a dry run, else one line per file with its backup
// and note, then the next step.
func writeSetupReport(env *Env, r setupReport) error {
	if r.Changes == nil {
		r.Changes = []integrations.FileChange{}
	}
	for i := range r.Changes {
		if r.Changes[i].Entries == nil {
			r.Changes[i].Entries = []string{}
		}
	}
	if env.JSON {
		return writeJSON(env.Stdout, r)
	}
	if r.DryRun {
		// The preview verbatim, so it can be piped or pasted as is.
		_, err := fmt.Fprint(env.Stdout, r.Preview)
		return err
	}
	var b strings.Builder
	for _, c := range r.Changes {
		line := fmt.Sprintf("%s: %s", c.File, c.Action)
		if len(c.Entries) > 0 {
			line += " " + strings.Join(c.Entries, ", ")
		}
		b.WriteString(line + "\n")
		if c.Backup != "" {
			fmt.Fprintf(&b, "  backup: %s\n", c.Backup)
		}
		if c.Note != "" {
			fmt.Fprintf(&b, "  note: %s\n", c.Note)
		}
	}
	if r.Next != "" {
		b.WriteString(r.Next + "\n")
	}
	_, err := fmt.Fprint(env.Stdout, b.String())
	return err
}

// setupPaths resolves the client config paths from the Env's environment
// and home directory.
func setupPaths(env *Env) (integrations.Paths, error) {
	home, err := env.UserHomeDir()
	if err != nil {
		return integrations.Paths{}, fmt.Errorf("cannot locate your home directory: %w", err)
	}
	return integrations.ResolvePaths(env.Getenv, home), nil
}

// setupDBPath is "" without --db, else the absolute --db path.
func setupDBPath(env *Env) (string, error) {
	if env.DBPath == "" {
		return "", nil
	}
	abs, err := filepath.Abs(env.DBPath)
	if err != nil {
		return "", fmt.Errorf("resolve --db %s: %w", env.DBPath, err)
	}
	return abs, nil
}

// setupCLIPath is the absolute path of the running casebook CLI, which is
// the command written into client config (D-03).
func setupCLIPath(env *Env) (string, error) {
	exe, err := env.Executable()
	if err != nil {
		return "", fmt.Errorf("cannot locate the casebook executable: %w", err)
	}
	abs, err := filepath.Abs(exe)
	if err != nil {
		return "", fmt.Errorf("cannot locate the casebook executable: %w", err)
	}
	return abs, nil
}

// noteCasebookEnv prints a one-line stderr note when CASEBOOK_DB or
// CASEBOOK_HOME is set and --db is not: AI clients may start casebook without
// this shell's environment, so they would use the default database.
func noteCasebookEnv(env *Env) {
	if env.DBPath != "" {
		return
	}
	var set []string
	for _, name := range []string{"CASEBOOK_DB", "CASEBOOK_HOME"} {
		if env.Getenv(name) != "" {
			set = append(set, name)
		}
	}
	if len(set) == 0 {
		return
	}
	verb := "is"
	if len(set) > 1 {
		verb = "are"
	}
	fmt.Fprintf(env.Stderr, "casebook: note: %s %s set in this shell but AI clients may start casebook without it; pass --db to wire this database explicitly\n",
		strings.Join(set, " and "), verb)
}
