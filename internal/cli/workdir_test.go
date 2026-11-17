package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/app/apptest"
)

// runCLIIn is runCLI with the CLI working in dir (Env.Getwd).
func runCLIIn(t *testing.T, dir string, args ...string) cliResult {
	t.Helper()
	var out, errb bytes.Buffer
	env := NewEnv(strings.NewReader(""), &out, &errb)
	env.Getenv = func(k string) string {
		if strings.HasPrefix(k, "CASEBOOK_") {
			return ""
		}
		return os.Getenv(k)
	}
	env.StdinIsTerminal = func() bool { return false }
	env.Getwd = func() (string, error) { return dir, nil }
	code := run(context.Background(), env, args)
	return cliResult{Stdout: out.String(), Stderr: errb.String(), Code: code}
}

// boundRepo returns a seeded database (its investigation globally active),
// a second investigation INV-0002 and a git repository directory with a
// subdirectory, symlinks resolved.
func boundRepo(t *testing.T) (db, repo, sub string) {
	t.Helper()
	a, db, _ := seededApp(t)
	if _, err := a.CreateInvestigation(context.Background(), apptest.Human,
		app.CreateInvestigationInput{Title: "TestCheckoutTotals flakes in CI"}); err != nil {
		t.Fatal(err)
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repo = filepath.Join(root, "shop")
	sub = filepath.Join(repo, "internal", "checkout")
	for _, d := range []string{filepath.Join(repo, ".git"), sub} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return db, repo, sub
}

func TestActivateHere(t *testing.T) {
	db, repo, sub := boundRepo(t)
	elsewhere := t.TempDir()

	r := runCLIIn(t, sub, "--db", db, "investigation", "activate", "INV-0002", "--here")
	wantCode(t, r, 0)
	if want := "Active case for " + repo + ": INV-0002 TestCheckoutTotals flakes in CI\n"; r.Stdout != want {
		t.Fatalf("activate --here stdout = %q, want %q", r.Stdout, want)
	}

	r = runCLIIn(t, sub, "--db", db, "investigation", "active")
	wantCode(t, r, 0)
	if !strings.HasPrefix(r.Stdout, "INV-0002  ") || !strings.Contains(r.Stdout, "for "+repo) {
		t.Fatalf("active inside the repo = %q", r.Stdout)
	}
	r = runCLIIn(t, elsewhere, "--db", db, "investigation", "active")
	if !strings.HasPrefix(r.Stdout, "INV-0001  ") || strings.Contains(r.Stdout, " for ") {
		t.Fatalf("active elsewhere = %q", r.Stdout)
	}

	r = runCLIIn(t, sub, "--db", db, "status")
	if !strings.Contains(r.Stdout, "INV-0002 TestCheckoutTotals flakes in CI (your AI's current case, for "+repo+")") {
		t.Fatalf("status inside the repo = %q", r.Stdout)
	}

	// A global change inside the repo is noted: the binding still wins.
	r = runCLIIn(t, sub, "--db", db, "investigation", "activate", "INV-0001", "--global")
	wantCode(t, r, 0)
	if !strings.Contains(r.Stderr, "INV-0002 is the active case for "+repo) {
		t.Fatalf("activate --global stderr = %q, want the binding note", r.Stderr)
	}

	r = runCLIIn(t, elsewhere, "--db", db, "--json", "investigation", "dirs")
	wantCode(t, r, 0)
	var dirs []app.DirectoryBinding
	decodeJSON(t, r.Stdout, &dirs)
	if len(dirs) != 1 || dirs[0].Dir != repo || dirs[0].InvestigationRef != "INV-0002" {
		t.Fatalf("dirs = %+v", dirs)
	}

	r = runCLIIn(t, sub, "--db", db, "investigation", "activate", "--clear", "--here")
	wantCode(t, r, 0)
	if r.Stdout != "Removed the active case for "+repo+".\n" {
		t.Fatalf("clear --here = %q", r.Stdout)
	}
	r = runCLIIn(t, sub, "--db", db, "investigation", "active")
	if !strings.HasPrefix(r.Stdout, "INV-0001  ") {
		t.Fatalf("active after clear --here = %q", r.Stdout)
	}

	r = runCLIIn(t, sub, "--db", db, "investigation", "activate", "INV-0002", "--here", "--global")
	wantCode(t, r, 2)
}

// TestHookUsesDirectoryBinding: the session-start hook picks the
// investigation bound to the payload's cwd, and the global one elsewhere.
func TestHookUsesDirectoryBinding(t *testing.T) {
	db, _, sub := boundRepo(t)
	wantCode(t, runCLIIn(t, sub, "--db", db, "investigation", "activate", "INV-0002", "--here"), 0)

	r := runHook(t, hookPayloadCwd("s1", "SessionStart", sub), "--db", db, "hook", "session-start")
	wantCode(t, r, 0)
	ctx := additionalContext(t, r.Stdout)
	if !strings.Contains(ctx, "Investigation: INV-0002") || !strings.Contains(ctx, "Made the active investigation at ") {
		t.Fatalf("session-start in the bound repo:\n%s", ctx)
	}
	r = runHook(t, hookPayloadCwd("s2", "SessionStart", t.TempDir()), "--db", db, "hook", "session-start")
	if !strings.Contains(additionalContext(t, r.Stdout), "Investigation: INV-0001") {
		t.Fatalf("session-start elsewhere:\n%s", r.Stdout)
	}
}
