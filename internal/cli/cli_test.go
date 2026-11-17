package cli

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"github.com/jameskbb/casebook/internal/app/apptest"
)

// cliResult is what one in-process casebook run produced.
type cliResult struct {
	Stdout, Stderr string
	Code           int
}

// runCLI runs casebook in process with stdin text and args. CASEBOOK_
// variables read through the Env are hidden so tests never see the
// developer's environment; every test passes --db explicitly.
func runCLI(t *testing.T, stdin string, args ...string) cliResult {
	t.Helper()
	var out, errb bytes.Buffer
	env := NewEnv(strings.NewReader(stdin), &out, &errb)
	env.Getenv = func(k string) string {
		if strings.HasPrefix(k, "CASEBOOK_") {
			return ""
		}
		return os.Getenv(k)
	}
	env.StdinIsTerminal = func() bool { return false }
	code := run(context.Background(), env, args)
	return cliResult{Stdout: out.String(), Stderr: errb.String(), Code: code}
}

// seededDB returns the path of a database holding the cloud spend
// fixture. The seeding App stays open for the test (two Apps on
// one file are supported).
func seededDB(t *testing.T) (string, apptest.Fixture) {
	t.Helper()
	a := apptest.Open(t)
	f := apptest.SeedCloudSpend(t, a)
	return a.DBPath(), f
}

// wantCode fails the test when r exited with another code.
func wantCode(t *testing.T, r cliResult, code int) {
	t.Helper()
	if r.Code != code {
		t.Fatalf("exit code = %d, want %d\nstdout:\n%s\nstderr:\n%s", r.Code, code, r.Stdout, r.Stderr)
	}
}
