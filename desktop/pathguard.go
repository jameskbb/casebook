package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/jameskbb/casebook/internal/paths"
)

// Path comparisons for the desktop safety rails: --seed-demo never touches
// the per-user database (D-08), and export never writes over Casebook's own
// files. Both compare resolved paths, not the way a path was chosen.

// foldCase is true where the usual filesystems ignore letter case, so
// C:\Users\Me\Casebook and c:\users\me\casebook name the same directory.
var foldCase = runtime.GOOS == "windows" || runtime.GOOS == "darwin"

// envMu serializes defaultDBPath's temporary changes to the environment.
var envMu sync.Mutex

// defaultDBPath is the per-user database path internal/paths resolves when
// CASEBOOK_HOME and CASEBOOK_DB are unset, whatever they are set to now. It
// briefly unsets both and restores them, so it must only run while nothing
// else reads the environment (startup, before the window opens).
func defaultDBPath() (string, error) {
	envMu.Lock()
	defer envMu.Unlock()
	saved := map[string]string{}
	for _, k := range []string{paths.EnvHome, paths.EnvDB} {
		if v, ok := os.LookupEnv(k); ok {
			saved[k] = v
			os.Unsetenv(k)
		}
	}
	defer func() {
		for k, v := range saved {
			os.Setenv(k, v)
		}
	}()
	return paths.DBPath()
}

// resolvePath makes p absolute and clean and resolves symbolic links in the
// longest part of it that exists, so a path that does not exist yet still
// compares equal to the same location reached through a link.
func resolvePath(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		abs = filepath.Clean(p)
	}
	rest := ""
	for cur := abs; ; {
		if r, err := filepath.EvalSymlinks(cur); err == nil {
			return filepath.Join(r, rest)
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return abs
		}
		rest = filepath.Join(filepath.Base(cur), rest)
		cur = parent
	}
}

// pathKey is resolvePath, lower cased where the filesystem ignores case.
func pathKey(p string) string {
	r := resolvePath(p)
	if foldCase {
		r = strings.ToLower(r)
	}
	return r
}

// samePath reports whether a and b name the same file: equal after
// resolving, or the same existing file (hard links, 8.3 names).
func samePath(a, b string) bool {
	if pathKey(a) == pathKey(b) {
		return true
	}
	ia, errA := os.Stat(a)
	ib, errB := os.Stat(b)
	return errA == nil && errB == nil && os.SameFile(ia, ib)
}

// isWithin reports whether p is dir itself or anything below it.
func isWithin(p, dir string) bool {
	rel, err := filepath.Rel(pathKey(dir), pathKey(p))
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}
