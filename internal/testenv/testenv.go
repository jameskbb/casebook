// Package testenv holds setup shared by test binaries. Only tests import it.
package testenv

import (
	"os"
	"path/filepath"
	"runtime"
)

// CanonicalTempDir points the process temp directory at its fully resolved
// form, so t.TempDir() returns the same spelling the product reports after
// filepath.EvalSymlinks: on macOS /var is a link to /private/var, and on
// Windows CI the temp path holds an 8.3 short name (RUNNER~1) that resolves
// to the long one. Call it from TestMain before m.Run. It leaves the
// environment unchanged when the path cannot be resolved.
func CanonicalTempDir() {
	resolved, err := filepath.EvalSymlinks(os.TempDir())
	if err != nil {
		return
	}
	if runtime.GOOS == "windows" {
		_ = os.Setenv("TMP", resolved)
		_ = os.Setenv("TEMP", resolved)
		return
	}
	_ = os.Setenv("TMPDIR", resolved)
}
