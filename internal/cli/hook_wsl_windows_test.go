package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jameskbb/casebook/internal/integrations"
)

// TestHookScopeWSLPaths runs the Windows hooks with the Linux cwd a Claude
// Code inside WSL passes: a /mnt/<drive> path is translated to the Windows
// drive path and the marker is found there; a Linux-only path with no
// distribution to translate it with stays silent instead of failing.
func TestHookScopeWSLPaths(t *testing.T) {
	_, db, f := seededApp(t)
	project := filepath.Join(t.TempDir(), "project")
	inside := filepath.Join(project, "src")
	unrelated := filepath.Join(t.TempDir(), "unrelated")
	for _, d := range []string{inside, unrelated} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(project, integrations.ScopeMarker), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(integrations.WSLDistroEnv, "")
	t.Chdir(inside)
	marked := map[string]string{integrations.HookScopeEnv: "marked"}
	start := func(cwd string) cliResult {
		t.Helper()
		return runHookEnv(t, marked, hookPayloadCwd("w1", "SessionStart", cwd), "--db", db, "hook", "session-start")
	}
	wslPath := func(win string) string {
		t.Helper()
		p, ok := integrations.WSLPath(win)
		if !ok {
			t.Skipf("temp dir %s has no /mnt form", win)
		}
		return p
	}

	r := start(wslPath(inside))
	wantCode(t, r, 0)
	if r.Stderr != "" || !strings.Contains(r.Stdout, "Investigation: "+f.InvestigationRef) {
		t.Errorf("WSL /mnt cwd inside the project: stdout %q, stderr %q; want the pack", r.Stdout, r.Stderr)
	}
	wantHookQuiet(t, start(wslPath(unrelated)))
	// No WSL_DISTRO_NAME and a drive working directory (inside the marked
	// project, so a fallback to it would speak): no distribution, so
	// a Linux-only path stays silent rather than falling back to the
	// working directory.
	wantHookQuiet(t, start("/home/jo/proj"))
}
