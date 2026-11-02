package cli

import (
	"context"
	"fmt"
	"io"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/jameskbb/casebook/internal/app"
)

// writeStatus prints the status block as aligned lines.
func writeStatus(w io.Writer, info app.StatusInfo, active *app.Investigation) error {
	tw := newTabWriter(w)
	fmt.Fprintf(tw, "Database:\t%s\n", info.DBPath)
	fmt.Fprintf(tw, "Data dir:\t%s\n", info.DataDir)
	fmt.Fprintf(tw, "Schema:\t%d\n", info.SchemaVersion)
	if active != nil && info.ActiveDir != "" {
		fmt.Fprintf(tw, "Active:\t%s %s (your AI's current case, for %s)\n", active.Ref, active.Title, info.ActiveDir)
	} else if active != nil {
		fmt.Fprintf(tw, "Active:\t%s %s (your AI's current case)\n", active.Ref, active.Title)
	} else {
		fmt.Fprintf(tw, "Active:\t%s\n", `none (your AI has no current case; run "casebook case activate <ref>")`)
	}
	c := info.Counts
	fmt.Fprintf(tw, "Records:\t%d cases, %d evidence, %d claims, %d hypotheses, %d questions\n",
		c.Investigations, c.Evidence, c.Claims, c.Hypotheses, c.Questions)
	return tw.Flush()
}

// missingDB resolves the database path (--db, else the default location)
// and reports whether no file exists there yet. Any other stat failure reads
// as not missing, so the caller opens the database and reports the real
// error.
func missingDB(env *Env) (string, bool) {
	path, exists, err := hookDBPath(env)
	return path, err == nil && !exists
}

// missingStatusInfo is the status of a database that does not exist yet:
// its path and data directory, schema version 0 and no records.
func missingStatusInfo(path string) app.StatusInfo {
	return app.StatusInfo{DBPath: path, DataDir: filepath.Dir(path)}
}

// writeMissingStatus prints the status block for a database that does not
// exist yet (AR-22).
func writeMissingStatus(w io.Writer, path string) error {
	tw := newTabWriter(w)
	fmt.Fprintf(tw, "Database:\t%s (not created yet; the first command that records data creates it)\n", path)
	fmt.Fprintf(tw, "Data dir:\t%s\n", filepath.Dir(path))
	fmt.Fprintf(tw, "Active:\t%s\n", "none")
	return tw.Flush()
}

// loadStatus opens the database and reads its status and the active
// investigation (nil when none is set).
func loadStatus(ctx context.Context, env *Env) (app.StatusInfo, *app.Investigation, error) {
	a, err := env.App()
	if err != nil {
		return app.StatusInfo{}, nil, err
	}
	info, err := a.Status(ctx)
	if err != nil {
		return app.StatusInfo{}, nil, err
	}
	if info.ActiveInvestigationRef == "" {
		return info, nil, nil
	}
	inv, err := a.GetInvestigation(ctx, info.ActiveInvestigationRef)
	if err != nil {
		return app.StatusInfo{}, nil, err
	}
	return info, &inv, nil
}

func newStatusCommand(env *Env) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show the database location, schema version, active case and record counts",
		Long: `Show the database location, data directory, schema version, active
case and record counts.

Status never creates the database. When no database exists yet it says so
(with --json: the path, data directory, schema_version 0 and zero counts)
and exits 0; the first command that records data creates it.`,
		Args: usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if path, missing := missingDB(env); missing {
				if env.JSON {
					return writeJSON(env.Stdout, missingStatusInfo(path))
				}
				return writeMissingStatus(env.Stdout, path)
			}
			info, active, err := loadStatus(cmd.Context(), env)
			if err != nil {
				return err
			}
			if env.JSON {
				return writeJSON(env.Stdout, info)
			}
			return writeStatus(env.Stdout, info, active)
		},
	}
}
