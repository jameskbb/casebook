package cli

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/jameskbb/casebook/internal/doctor"
)

// doctorLookPath finds client CLIs on PATH (tests replace it). Doctor never
// runs what it finds.
var doctorLookPath = exec.LookPath

// doctorMCPCheck replaces the MCP startup check when non-nil (tests: the
// in-process test binary must never be spawned as casebook).
var doctorMCPCheck func(ctx context.Context, cliPath, dbPath string) (doctor.MCPResult, error)

const doctorLong = `Check the Casebook installation, database and AI client setup.

Checks, in order: data_dir (the data directory exists and is writable),
database (opens; schema version and record counts), integrity
(integrity_check and foreign_key_check), migrations (the schema version
after opening is the latest this build knows), write_access (the database
file is writable, checked without opening it), mcp_startup (spawns
"casebook mcp", completes the initialize handshake and tools/list within 10
seconds), cli_path (this CLI and the desktop app next to it),
bundle_version (the desktop app next to this CLI carries the same release
version; a difference means a folder was only partly replaced during an
update; skipped for development builds or without a desktop app), claude_config
and codex_config (whether Casebook is registered and its command exists and
matches this CLI, and the Claude Code hooks), claude_cli and codex_cli
(whether the clients are on PATH; they are never run).

Doctor never creates the database and never changes its records. Opening an
existing database applies any pending schema migrations, after writing a
casebook-pre-upgrade backup to the backups folder next to it, as every
casebook command does; with no database yet, the MCP check runs against a temporary
one. Doctor prints only paths, counts, versions and statuses, never record
text, environment values or the contents of config files, so its output is
safe to paste into a bug report. It exits 1 when any check fails; warnings
alone exit 0.`

func newDoctorCommand(env *Env) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check the Casebook installation, database and AI client setup",
		Long:  doctorLong,
		Args:  usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			dbPath, _, err := hookDBPath(env)
			if err != nil {
				return err
			}
			p, err := setupPaths(env)
			if err != nil {
				return err
			}
			cliPath, err := setupCLIPath(env)
			if err != nil {
				cliPath = ""
			}
			r := doctor.Run(cmd.Context(), doctor.Options{
				DBPath:   dbPath,
				CLIPath:  cliPath,
				Paths:    p,
				Version:  Version,
				LookPath: doctorLookPath,
				MCPCheck: doctorMCPCheck,
				WorkDir:  env.workDir(),
			})
			if env.JSON {
				err = writeJSON(env.Stdout, r)
			} else {
				err = writeDoctor(env, r)
			}
			if err != nil {
				return err
			}
			if n := r.Failed(); n > 0 {
				noun := "checks"
				if n == 1 {
					noun = "check"
				}
				return fmt.Errorf("doctor found %d failing %s", n, noun)
			}
			return nil
		},
	}
}

// writeDoctor prints the report as a header, an aligned table and a
// summary line.
func writeDoctor(env *Env, r doctor.Report) error {
	fmt.Fprintf(env.Stdout, "Casebook doctor (casebook %s)\n\n", Version)
	tw := newTabWriter(env.Stdout)
	warn, skip := 0, 0
	for _, c := range r.Checks {
		switch c.Status {
		case doctor.StatusWarn:
			warn++
		case doctor.StatusSkip:
			skip++
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\n", c.ID, c.Status, oneLine(c.Detail))
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	var err error
	if n := r.Failed(); n > 0 {
		noun := "checks"
		if n == 1 {
			noun = "check"
		}
		_, err = fmt.Fprintf(env.Stdout, "\n%d %s failed.\n", n, noun)
	} else {
		_, err = fmt.Fprintf(env.Stdout, "\nAll checks passed (%d warnings, %d skipped).\n", warn, skip)
	}
	return err
}
