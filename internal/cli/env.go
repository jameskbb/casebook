package cli

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/update"
)

// Env carries everything a command touches outside its own arguments: the
// standard streams, OS hooks (replaced in tests) and the global flags. The App
// is opened lazily, at most once, so a command that needs no database (such
// as version or help) never opens one.
type Env struct {
	Stdin          io.Reader
	Stdout, Stderr io.Writer

	// Getenv reads an environment variable (os.Getenv by default).
	Getenv func(string) string
	// StdinIsTerminal reports whether Stdin is an interactive terminal.
	StdinIsTerminal func() bool
	// Executable returns the running executable's path, with symlinks
	// resolved when that succeeds.
	Executable func() (string, error)
	// Stat is os.Stat by default.
	Stat func(string) (os.FileInfo, error)
	// UserHomeDir is os.UserHomeDir by default.
	UserHomeDir func() (string, error)
	// StartProcess starts a program without waiting for it (casebook ui).
	StartProcess func(name string, args ...string) error
	// Getwd is os.Getwd by default: the working directory the App resolves
	// per-directory active investigations from (ADR-0015).
	Getwd func() (string, error)
	// CheckUpdate asks GitHub for the latest release (update.Check by
	// default; casebook update is its only caller, ADR 0022).
	CheckUpdate func(ctx context.Context, o update.Options) (update.Result, error)
	// WorkDir, when set before the App opens, replaces Getwd (a hook sets
	// it to the session's cwd from the payload).
	WorkDir string

	// DBPath is the --db flag: a database file overriding the default
	// location.
	DBPath string
	// JSON is the --json flag: machine-readable output.
	JSON bool

	app *app.App
}

// NewEnv returns an Env bound to the given streams with every OS hook set to
// its real implementation.
func NewEnv(stdin io.Reader, stdout, stderr io.Writer) *Env {
	e := &Env{
		Stdin:       stdin,
		Stdout:      stdout,
		Stderr:      stderr,
		Getenv:      os.Getenv,
		Stat:        os.Stat,
		UserHomeDir: os.UserHomeDir,
		Getwd:       os.Getwd,
		CheckUpdate: update.Check,
		Executable: func() (string, error) {
			return resolveExecutable(os.Executable, filepath.EvalSymlinks)
		},
		StartProcess: func(name string, args ...string) error {
			cmd := exec.Command(name, args...)
			if err := cmd.Start(); err != nil {
				return err
			}
			return cmd.Process.Release()
		},
	}
	e.StdinIsTerminal = func() bool {
		f, ok := e.Stdin.(*os.File)
		return ok && isTerminal(f)
	}
	return e
}

// resolveExecutable returns the path exe reports with symlinks resolved by
// eval. When eval fails (it does on Windows for some mapped, network and subst
// drives and for app execution aliases) the unresolved path is still a usable
// location, so it is returned instead of an error.
func resolveExecutable(exe func() (string, error), eval func(string) (string, error)) (string, error) {
	p, err := exe()
	if err != nil {
		return "", err
	}
	if r, err := eval(p); err == nil {
		return homebrewStablePath(r, eval), nil
	}
	return p, nil
}

// homebrewStablePath maps a resolved Homebrew keg path,
// <prefix>/Cellar/casebook/<version>/bin/casebook, to the stable
// <prefix>/opt/casebook/bin/casebook link when that link resolves to the same
// file. AI client configurations store the path setup writes, and brew
// upgrade removes the old keg, so the versioned path would break on the next
// upgrade while the opt link always points at the installed version. Any
// other path is returned unchanged.
func homebrewStablePath(resolved string, eval func(string) (string, error)) string {
	bin := filepath.Dir(resolved)
	keg := filepath.Dir(bin)
	formula := filepath.Dir(keg)
	cellar := filepath.Dir(formula)
	if filepath.Base(bin) != "bin" || filepath.Base(formula) != "casebook" || filepath.Base(cellar) != "Cellar" {
		return resolved
	}
	stable := filepath.Join(filepath.Dir(cellar), "opt", "casebook", "bin", filepath.Base(resolved))
	if r, err := eval(stable); err == nil && r == resolved {
		return stable
	}
	return resolved
}

// App opens the database on first use (the --db path when set, otherwise
// app.OpenDefault, which honors CASEBOOK_DB and CASEBOOK_HOME) and returns the
// same App on later calls. Options apply only to the first successful open.
// The App works in e.WorkDir, else the current directory (app.WithWorkDir);
// an option the caller passes wins.
func (e *Env) App(opts ...app.Option) (*app.App, error) {
	if e.app != nil {
		return e.app, nil
	}
	if wd := e.workDir(); wd != "" {
		opts = append([]app.Option{app.WithWorkDir(wd)}, opts...)
	}
	var (
		a   *app.App
		err error
	)
	if e.DBPath != "" {
		a, err = app.Open(e.DBPath, opts...)
	} else {
		a, err = app.OpenDefault(opts...)
	}
	if err != nil {
		return nil, err
	}
	e.app = a
	return a, nil
}

// Close closes the App if one was opened. It is safe to call more than once.
func (e *Env) Close() error {
	if e.app == nil {
		return nil
	}
	a := e.app
	e.app = nil
	return a.Close()
}

// workDir is e.WorkDir, else the current directory, else "".
func (e *Env) workDir() string {
	if e.WorkDir != "" {
		return e.WorkDir
	}
	if e.Getwd == nil {
		return ""
	}
	wd, err := e.Getwd()
	if err != nil {
		return ""
	}
	return wd
}
