package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

// hostGOOS is the operating system findDesktopApp resolves for; tests
// replace it to exercise another platform's launch path.
var hostGOOS = runtime.GOOS

// desktopTarget is the desktop app to launch. ViaOpen means "open <Path>"
// (a macOS .app bundle); otherwise Path is run directly.
type desktopTarget struct {
	Path    string
	ViaOpen bool
}

// desktopExeName is the desktop app's executable name on goos.
func desktopExeName(goos string) string {
	if goos == "windows" {
		return "Casebook.exe"
	}
	return "Casebook"
}

// findDesktopApp locates the Casebook desktop app for the CLI at exePath
// (D-10). It touches nothing but statFn. It returns the target, every path
// it looked at in order, and whether one was found.
//
// Order: a non-empty override (CASEBOOK_UI) is the only candidate; on darwin
// a directory named *.app is opened, otherwise a file is run. Without an
// override: on windows Casebook.exe next to the CLI, then in its parent
// directory; on darwin the .app bundle enclosing the CLI (at any depth), a
// Casebook executable next to the CLI, /Applications/Casebook.app, then
// <home>/Applications/Casebook.app; elsewhere Casebook next to the CLI, then
// in its parent directory.
//
// Windows NTFS and macOS APFS are case-insensitive by default, so
// "Casebook.exe" next to "casebook.exe" can be the CLI itself. A candidate
// that is the same file as exePath never counts.
func findDesktopApp(goos, exePath string, statFn func(string) (os.FileInfo, error), override, home string) (desktopTarget, []string, bool) {
	var (
		looked  []string
		exeInfo os.FileInfo
	)
	if exePath != "" {
		if fi, err := statFn(exePath); err == nil {
			exeInfo = fi
		}
	}
	// isFile reports whether p is an existing file other than the CLI.
	isFile := func(p string) bool {
		looked = append(looked, p)
		fi, err := statFn(p)
		return err == nil && !fi.IsDir() && !sameFile(fi, exeInfo)
	}
	// isBundle reports whether p is an existing directory.
	isBundle := func(p string) bool {
		looked = append(looked, p)
		fi, err := statFn(p)
		return err == nil && fi.IsDir()
	}
	found := func(p string, viaOpen bool) (desktopTarget, []string, bool) {
		return desktopTarget{Path: p, ViaOpen: viaOpen}, looked, true
	}
	notFound := func() (desktopTarget, []string, bool) { return desktopTarget{}, looked, false }

	if override != "" {
		looked = append(looked, override)
		fi, err := statFn(override)
		switch {
		case err != nil:
		case fi.IsDir():
			if goos == "darwin" && hasAppSuffix(override) {
				return found(override, true)
			}
		case !sameFile(fi, exeInfo):
			return found(override, false)
		}
		return notFound()
	}

	name := desktopExeName(goos)
	var exeDir string
	if exePath != "" {
		exeDir = filepath.Dir(exePath)
	}
	switch goos {
	case "darwin":
		if exeDir != "" {
			for d := exeDir; ; {
				// Open an enclosing bundle only when it is Casebook's: the
				// packaged layout is Casebook.app/Contents/MacOS/Casebook with
				// the CLI at Contents/MacOS/bin/casebook (ADR-0012).
				if hasAppSuffix(d) && isFile(filepath.Join(d, "Contents", "MacOS", name)) {
					return found(d, true)
				}
				parent := filepath.Dir(d)
				if parent == d {
					break
				}
				d = parent
			}
			if p := filepath.Join(exeDir, name); isFile(p) {
				return found(p, false)
			}
		}
		if p := filepath.Join(string(filepath.Separator)+"Applications", "Casebook.app"); isBundle(p) {
			return found(p, true)
		}
		if home != "" {
			if p := filepath.Join(home, "Applications", "Casebook.app"); isBundle(p) {
				return found(p, true)
			}
		}
	default:
		if exeDir != "" {
			if p := filepath.Join(exeDir, name); isFile(p) {
				return found(p, false)
			}
			if parent := filepath.Dir(exeDir); parent != exeDir {
				if p := filepath.Join(parent, name); isFile(p) {
					return found(p, false)
				}
			}
		}
	}
	return notFound()
}

// hasAppSuffix reports whether p's base name ends in ".app", ignoring case.
func hasAppSuffix(p string) bool {
	return strings.HasSuffix(strings.ToLower(filepath.Base(p)), ".app")
}

// sameFile reports whether a and b describe one file. os.SameFile handles
// values from os.Stat; values from another stat function (tests) are the
// same file when it returned the same value for both paths.
func sameFile(a, b os.FileInfo) bool {
	if a == nil || b == nil {
		return false
	}
	if os.SameFile(a, b) {
		return true
	}
	ta, tb := reflect.TypeOf(a), reflect.TypeOf(b)
	return ta == tb && ta.Comparable() && a == b
}

// macOpen is the macOS open tool, by absolute path so PATH cannot swap it.
const macOpen = "/usr/bin/open"

// absOverride makes a relative CASEBOOK_UI absolute against the working
// directory, so the file findDesktopApp stats is the file that runs: a bare
// name would otherwise be looked up on PATH when started. A rooted path is
// kept as given.
func absOverride(p string) string {
	if p == "" || filepath.IsAbs(p) || os.IsPathSeparator(p[0]) {
		return p
	}
	if abs, err := filepath.Abs(p); err == nil {
		return abs
	}
	return p
}

// newUICommand launches the desktop app without waiting for it. It never
// opens the database.
func newUICommand(env *Env) *cobra.Command {
	return &cobra.Command{
		Use:   "ui",
		Short: "Open the Casebook desktop app",
		Long: `Open the Casebook desktop app without waiting for it. The app is installed
separately from the casebook CLI. casebook ui looks next to the CLI (and, on
macOS, for the .app bundle that contains it, then /Applications and
~/Applications). Set CASEBOOK_UI to the app's executable or .app bundle to
override the search.`,
		Args: usageArgs(cobra.NoArgs),
		RunE: func(_ *cobra.Command, _ []string) error {
			exe, err := env.Executable()
			if err != nil {
				exe = ""
			}
			home, err := env.UserHomeDir()
			if err != nil {
				home = ""
			}
			target, looked, ok := findDesktopApp(hostGOOS, exe, env.Stat, absOverride(env.Getenv("CASEBOOK_UI")), home)
			if !ok {
				where := "nowhere: the CLI's own location is unknown"
				if len(looked) > 0 {
					where = strings.Join(looked, ", ")
				}
				return fmt.Errorf("the Casebook desktop app was not found (looked in: %s); "+
					"it is installed separately from the casebook CLI; set CASEBOOK_UI to its path", where)
			}
			if target.ViaOpen {
				err = env.StartProcess(macOpen, target.Path)
			} else {
				err = env.StartProcess(target.Path)
			}
			if err != nil {
				return fmt.Errorf("could not start the Casebook desktop app %s: %w", target.Path, err)
			}
			if env.JSON {
				return writeJSON(env.Stdout, map[string]any{"path": target.Path, "launched": true})
			}
			_, err = fmt.Fprintf(env.Stdout, "Started Casebook from %s.\n", target.Path)
			return err
		},
	}
}
