package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jameskbb/casebook/internal/app"
)

// newBackupCommand writes a consistent snapshot of the whole database.
func newBackupCommand(env *Env) *cobra.Command {
	return &cobra.Command{
		Use:   "backup <dest>",
		Short: "Write a backup of all Casebook data to a new file",
		Long: `Write a consistent snapshot of the whole Casebook database to a new
owner-only file. It is safe while the desktop app or MCP servers are writing.
An existing destination is never overwritten.`,
		Example: `  casebook backup ~/casebook-2026-10-02.db`,
		Args:    usageArgs(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := env.App()
			if err != nil {
				return err
			}
			info, err := a.Backup(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if env.JSON {
				return writeJSON(env.Stdout, info)
			}
			_, err = fmt.Fprintf(env.Stdout, "Backup written to %s (%d bytes, schema version %d, %d cases).\n",
				info.Path, info.SizeBytes, info.SchemaVersion, info.Counts.Investigations)
			return err
		},
	}
}

// newRestoreCommand replaces all data with a backup after a safety copy.
func newRestoreCommand(env *Env) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "restore <src>",
		Short: "Replace all Casebook data with a backup (a safety copy is written first)",
		Long: `Replace all Casebook data with a backup written by casebook backup or the
desktop app. The backup is validated first, then a safety copy of the current
data is written and its path printed. Without --yes casebook asks for
confirmation, and refuses when stdin is not a terminal.`,
		Example: `  casebook restore ~/casebook-2026-10-02.db --yes`,
		Args:    usageArgs(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			src := args[0]
			a, err := env.App()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			if _, err := a.ValidateBackup(ctx, src); err != nil {
				return err
			}
			if !yes {
				if !env.StdinIsTerminal() {
					return newUsageError("restore replaces all Casebook data; pass --yes to confirm when not running interactively")
				}
				fmt.Fprintf(env.Stderr, "Restore replaces all Casebook data in %s with %s. "+
					"A safety copy of the current data is written first. Type \"yes\" to continue: ", a.DBPath(), src)
				line, _ := readWithContext(ctx, func() (string, error) {
					return bufio.NewReader(env.Stdin).ReadString('\n')
				})
				if ctx.Err() != nil {
					fmt.Fprintln(env.Stderr) // interrupted: finish the prompt line
					return errors.New("restore cancelled; nothing was changed")
				}
				if !strings.HasSuffix(line, "\n") {
					fmt.Fprintln(env.Stderr) // end of input: finish the prompt line
				}
				if !strings.EqualFold(strings.TrimSpace(line), "yes") {
					return errors.New("restore cancelled; nothing was changed")
				}
			}
			res, err := a.Restore(ctx, app.ActorCLI(), src)
			if err != nil {
				return err
			}
			if env.JSON {
				return writeJSON(env.Stdout, res)
			}
			_, err = fmt.Fprintf(env.Stdout, "Restored from %s.\nSafety copy of the previous data: %s\n",
				res.SourcePath, res.SafetyCopyPath)
			return err
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "confirm the restore without asking")
	return cmd
}

// readWithContext runs a blocking read and returns its result, or ctx.Err()
// as soon as ctx is cancelled (Ctrl-C). Reads from stdin cannot be
// interrupted, so on cancel the read goroutine is left blocked; the command
// returns and the process exits right after.
func readWithContext[T any](ctx context.Context, read func() (T, error)) (T, error) {
	type result struct {
		v   T
		err error
	}
	ch := make(chan result, 1)
	go func() {
		v, err := read()
		ch <- result{v, err}
	}()
	select {
	case <-ctx.Done():
		var zero T
		return zero, ctx.Err()
	case r := <-ch:
		return r.v, r.err
	}
}
