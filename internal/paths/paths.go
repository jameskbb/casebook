// Package paths resolves the per-user Casebook data directory and database
// file, and creates directories with owner-only permissions.
//
// Resolution order for the database file:
//
//  1. CASEBOOK_DB (a file path) wins over everything.
//  2. Otherwise DataDir()/casebook.db, where DataDir is CASEBOOK_HOME when set,
//     else the per-OS default:
//     Windows %LocalAppData%\Casebook, macOS ~/Library/Application Support/Casebook,
//     Linux and others $XDG_DATA_HOME/casebook or ~/.local/share/casebook.
//
// Relative environment values are made absolute against the current working
// directory.
package paths

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
)

// Environment variable names honored by DataDir and DBPath.
const (
	EnvHome = "CASEBOOK_HOME"
	EnvDB   = "CASEBOOK_DB"
)

// DBFileName is the database file name inside the data directory.
const DBFileName = "casebook.db"

// DataDir returns the absolute Casebook data directory. It does not create it.
func DataDir() (string, error) {
	if v := os.Getenv(EnvHome); v != "" {
		return filepath.Abs(v)
	}
	switch runtime.GOOS {
	case "windows":
		// os.UserCacheDir returns %LocalAppData% on Windows.
		base, err := os.UserCacheDir()
		if err != nil {
			return "", fmt.Errorf("locate data directory: %w", err)
		}
		return filepath.Join(base, "Casebook"), nil
	case "darwin":
		base, err := os.UserConfigDir()
		if err != nil {
			return "", fmt.Errorf("locate data directory: %w", err)
		}
		return filepath.Join(base, "Casebook"), nil
	default:
		if x := os.Getenv("XDG_DATA_HOME"); x != "" && filepath.IsAbs(x) {
			return filepath.Join(x, "casebook"), nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("locate data directory: %w", err)
		}
		return filepath.Join(home, ".local", "share", "casebook"), nil
	}
}

// DBPath returns the absolute path of the Casebook database file. It does not
// create anything.
func DBPath() (string, error) {
	if v := os.Getenv(EnvDB); v != "" {
		return filepath.Abs(v)
	}
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, DBFileName), nil
}

// EnsureDir creates dir and any missing parents with mode 0700. Only the
// directories it creates are chmodded to 0700 (on non-Windows, to defeat the
// umask); a directory that already exists keeps its permissions untouched,
// because a user may point CASEBOOK_DB into a shared directory such as /tmp.
func EnsureDir(dir string) error {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	// Collect the missing directories from the leaf up.
	var missing []string
	for p := abs; ; {
		info, err := os.Stat(p)
		if err == nil {
			if !info.IsDir() {
				return fmt.Errorf("%s exists and is not a directory", p)
			}
			break
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		missing = append(missing, p)
		parent := filepath.Dir(p)
		if parent == p {
			break
		}
		p = parent
	}
	// Create from the top down.
	for i := len(missing) - 1; i >= 0; i-- {
		p := missing[i]
		if err := os.Mkdir(p, 0o700); err != nil {
			if errors.Is(err, fs.ErrExist) {
				// Created concurrently by someone else: leave it alone.
				continue
			}
			return err
		}
		if runtime.GOOS != "windows" {
			if err := os.Chmod(p, 0o700); err != nil {
				return err
			}
		}
	}
	return nil
}
