package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jameskbb/casebook/internal/app"
)

// newExportCommand writes one investigation in the versioned export format
// (it includes raw evidence detail by design) to stdout or a new file.
func newExportCommand(env *Env) *cobra.Command {
	var (
		output string
		force  bool
	)
	cmd := &cobra.Command{
		Use:   "export [ref]",
		Short: "Export a case as casebook.investigation JSON (default: the active one)",
		Long: `Export a case, including raw evidence detail, as versioned
casebook.investigation JSON. With no ref it exports the active case.
The JSON goes to stdout unless -o (or --out) names a file; -o creates an
owner-only file and never overwrites an existing one unless --force is given.
With --force the file is replaced atomically (a failed write leaves the old
file intact), and the live database is never a valid -o target.

The output is stable, so a case can be committed to a repository and moved
to another machine or handed to someone as a copy: exporting an unchanged
case again gives the same bytes. exported_at is the time of the newest change
in the file, and which case was active on this machine is left out. On the
other machine, casebook import adds the file; verification marks come in as
ai_generated.`,
		Example: `  casebook export INV-0001 -o inv-0001.json
  casebook export INV-0002 --out .casebook/INV-0002.json --force
  casebook export > active.json`,
		Args: usageArgs(cobra.MaximumNArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			ref := ""
			if len(args) == 1 {
				ref = args[0]
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
			data, err := a.ExportInvestigation(ctx, inv.Ref)
			if err != nil {
				return err
			}
			if output == "" {
				_, err := env.Stdout.Write(data)
				return err
			}
			path := output
			if abs, err := filepath.Abs(output); err == nil {
				path = abs
			}
			if err := writeExportFile(path, a.DBPath(), data, force); err != nil {
				return err
			}
			if env.JSON {
				return writeJSON(env.Stdout, map[string]any{
					"investigation_ref": inv.Ref, "path": path, "bytes": len(data),
				})
			}
			_, err = fmt.Fprintf(env.Stdout, "Exported %s to %s (%d bytes).\n", inv.Ref, path, len(data))
			return err
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "", "write to this file instead of stdout")
	cmd.Flags().StringVar(&output, "out", "", "same as --output")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite the -o file if it exists")
	return cmd
}

// writeExportFile writes data to path as an owner-only file. path may never
// be the live database or one of its sidecar files. Without force an existing
// path is a conflict; with force the data is written to a temporary file in
// the same directory that then replaces path, so a failed write leaves the
// previous file intact.
func writeExportFile(path, dbPath string, data []byte, force bool) error {
	if err := refuseLiveDatabase(path, dbPath); err != nil {
		return err
	}
	if !force {
		return writeNewFile(path, data)
	}
	return replaceFile(path, data)
}

// refuseLiveDatabase returns a conflict when path names the same file as the
// database at dbPath or its -wal, -shm or -journal file (through any link or
// spelling). A path that does not exist yet cannot be one of them.
func refuseLiveDatabase(path, dbPath string) error {
	fi, err := os.Stat(path)
	if err != nil || dbPath == "" {
		return nil
	}
	for _, p := range []string{dbPath, dbPath + "-wal", dbPath + "-shm", dbPath + "-journal"} {
		if di, err := os.Stat(p); err == nil && os.SameFile(fi, di) {
			return fmt.Errorf("%w: refusing to overwrite the live database file %s", app.ErrConflict, p)
		}
	}
	return nil
}

// writeNewFile writes data to a new owner-only file; an existing file is a
// conflict. The file is removed again when the write fails.
func writeNewFile(path string, data []byte) (err error) {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if errors.Is(err, fs.ErrExist) {
			return fmt.Errorf("%w: %s already exists; pass --force to overwrite", app.ErrConflict, path)
		}
		return err
	}
	defer func() {
		if err != nil {
			_ = os.Remove(path)
		}
	}()
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// replaceFile writes data to an owner-only temporary file next to path, syncs
// it and renames it over path (os.Rename replaces an existing file on every
// platform). The temporary file is removed when any step fails.
func replaceFile(path string, data []byte) (err error) {
	f, err := os.CreateTemp(filepath.Dir(path), ".casebook-export-*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer func() {
		if err != nil {
			_ = f.Close()
			_ = os.Remove(tmp)
		}
	}()
	// CreateTemp already uses 0600; Chmod keeps that true under any umask
	// quirk. Windows only honors the read-only bit, so skip the error there.
	if err := f.Chmod(0o600); err != nil && runtime.GOOS != "windows" {
		return err
	}
	if _, err := f.Write(data); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// newImportCommand imports an exported investigation from a file or stdin.
func newImportCommand(env *Env) *cobra.Command {
	return &cobra.Command{
		Use:   "import <file|->",
		Short: "Import a case from casebook.investigation JSON (- reads stdin)",
		Long: `Import a case exported by casebook export or the desktop app. The
file is validated completely before anything is written. Record refs are
kept; the case keeps its ref when it is free and otherwise gets the next
one. The active case does not change. Verification from the
file is not trusted: human_verified and source_verified records come in as
ai_generated, for someone to verify here.

Import always adds a separate copy; it never merges into or replaces a
case. When the same case is already in this database (the
original, or an earlier import of the file), the output names it, so you can
archive and delete the older copy in the desktop app.`,
		Example: `  casebook import inv-0001.json
  casebook import - < inv-0001.json`,
		Args: usageArgs(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := readImport(cmd.Context(), env, args[0])
			if err != nil {
				return err
			}
			a, err := env.App()
			if err != nil {
				return err
			}
			res, err := a.ImportInvestigation(cmd.Context(), app.ActorCLI(), data)
			if err != nil {
				return err
			}
			if env.JSON {
				return writeJSON(env.Stdout, res)
			}
			ref := res.InvestigationRef
			if res.RefChanged {
				ref = res.OriginalRef + " as " + res.InvestigationRef
			}
			c := res.Counts
			if _, err := fmt.Fprintf(env.Stdout, "Imported %s (%d evidence, %d claims, %d hypotheses, %d questions).\n",
				ref, c.Evidence, c.Claims, c.Hypotheses, c.Questions); err != nil {
				return err
			}
			if len(res.SameCaseRefs) > 0 {
				_, err = fmt.Fprintf(env.Stdout, "The same case is already here as %s; %s is a separate copy. "+
					"To keep one, archive and delete the other in the desktop app (Casebook never merges copies).\n",
					strings.Join(res.SameCaseRefs, ", "), res.InvestigationRef)
			}
			return err
		},
	}
}

// readImport reads an import document from a file or, for "-", stdin. It
// reads at most one byte past app.MaxImportBytes so memory stays bounded;
// the engine rejects the oversize document. Reading stdin stops when ctx is
// cancelled (Ctrl-C) rather than waiting for end of input.
func readImport(ctx context.Context, env *Env, name string) ([]byte, error) {
	if name == "-" {
		data, err := readWithContext(ctx, func() ([]byte, error) {
			return io.ReadAll(io.LimitReader(env.Stdin, app.MaxImportBytes+1))
		})
		if ctx.Err() != nil {
			return nil, errors.New("import cancelled; nothing was changed")
		}
		return data, err
	}
	// Only a regular file: a FIFO or device would block the read forever.
	f, err := app.OpenImportFile(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(io.LimitReader(f, app.MaxImportBytes+1))
}
