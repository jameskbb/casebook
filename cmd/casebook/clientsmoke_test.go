package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// INT-09 smoke tests: the real Codex and Claude Code CLIs read the config
// `casebook setup` writes. They run only with CASEBOOK_CLIENT_SMOKE=1 and
// the client on PATH, and always against temp config dirs (D-15).

// clientSmokeTimeout bounds every client run.
const clientSmokeTimeout = 90 * time.Second

// clientSmoke skips unless CASEBOOK_CLIENT_SMOKE=1 and name is on PATH, and
// returns the client's path.
func clientSmoke(t *testing.T, name string) string {
	t.Helper()
	if os.Getenv("CASEBOOK_CLIENT_SMOKE") != "1" {
		t.Skip("set CASEBOOK_CLIENT_SMOKE=1 to run the " + name + " client smoke test")
	}
	path, err := exec.LookPath(name)
	if err != nil {
		t.Skip(name + " is not on PATH; client smoke test skipped")
	}
	return path
}

// claudeCredentialEnv are the variables that carry Claude credentials
// through the environment. They are kept so a temp CLAUDE_CONFIG_DIR can
// authenticate without reading the real config dir.
var claudeCredentialEnv = map[string]bool{
	"CLAUDE_CODE_OAUTH_TOKEN": true,
	"ANTHROPIC_API_KEY":       true,
}

// parentSessionVar reports whether the upper-cased key uk is one an
// enclosing Claude Code session sets (CLAUDECODE, CLAUDE_PID,
// CLAUDE_EFFORT and CLAUDE_CODE_*), which would attach a child client to
// that session. Credential variables are never session variables.
func parentSessionVar(uk string) bool {
	if claudeCredentialEnv[uk] {
		return false
	}
	return uk == "CLAUDECODE" || uk == "CLAUDE_PID" || uk == "CLAUDE_EFFORT" || strings.HasPrefix(uk, "CLAUDE_CODE_")
}

// clientSmokeDirs are the isolated locations of one smoke test.
type clientSmokeDirs struct {
	Home, CodexHome, ClaudeDir string
	// Env is the child environment: os.Environ() without the enclosing
	// session's variables (parentSessionVar) or any CASEBOOK_ variable, with
	// HOME, USERPROFILE, CODEX_HOME, CLAUDE_CONFIG_DIR and every XDG_*_HOME
	// replaced by fresh temp dirs and CASEBOOK_HOME pointing at a temp dir.
	Env []string
}

// clientSmokeEnv builds the isolated environment, so a client never sees
// the developer's real config.
func clientSmokeEnv(t *testing.T) clientSmokeDirs {
	t.Helper()
	d := clientSmokeDirs{Home: t.TempDir(), CodexHome: t.TempDir(), ClaudeDir: t.TempDir()}
	replaced := map[string]string{
		"HOME":              d.Home,
		"USERPROFILE":       d.Home,
		"CODEX_HOME":        d.CodexHome,
		"CLAUDE_CONFIG_DIR": d.ClaudeDir,
		"XDG_CONFIG_HOME":   t.TempDir(),
		"XDG_DATA_HOME":     t.TempDir(),
		"XDG_STATE_HOME":    t.TempDir(),
		"XDG_CACHE_HOME":    t.TempDir(),
		"CASEBOOK_HOME":     t.TempDir(),
	}
	for _, kv := range os.Environ() {
		k, _, _ := strings.Cut(kv, "=")
		uk := strings.ToUpper(k)
		if parentSessionVar(uk) || strings.HasPrefix(uk, "CASEBOOK_") {
			continue
		}
		if _, ok := replaced[uk]; ok {
			continue
		}
		d.Env = append(d.Env, kv)
	}
	for k, v := range replaced {
		d.Env = append(d.Env, k+"="+v)
	}
	return d
}

// smokeRun is one finished client run.
type smokeRun struct {
	Stdout, Stderr string
	Err            error
}

// trimmed is the run's output for a failure log, cut to 2000 bytes.
func (r smokeRun) trimmed() string {
	s := "stdout:\n" + r.Stdout + "\nstderr:\n" + r.Stderr
	if len(s) > 2000 {
		s = s[:2000] + "\n[trimmed]"
	}
	return s
}

// runSmoke runs name with args in the isolated env from the temp home
// directory (so no project config in this repository is read), with a 90
// second timeout.
func runSmoke(t *testing.T, d clientSmokeDirs, name string, args ...string) smokeRun {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), clientSmokeTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = d.Env
	cmd.Dir = d.Home
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	if ctx.Err() != nil {
		err = ctx.Err()
	}
	return smokeRun{Stdout: out.String(), Stderr: errb.String(), Err: err}
}

// mustSmoke runs a client command and fails the test on a non-zero exit.
func mustSmoke(t *testing.T, d clientSmokeDirs, name string, args ...string) smokeRun {
	t.Helper()
	r := runSmoke(t, d, name, args...)
	if r.Err != nil {
		t.Fatalf("%s %s: %v\n%s", filepath.Base(name), strings.Join(args, " "), r.Err, r.trimmed())
	}
	return r
}

// smokeBinary is the casebook binary path with symlinks resolved, the form
// setup writes into client config.
func smokeBinary(t *testing.T) string {
	t.Helper()
	p, err := filepath.EvalSymlinks(binary(t))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestClientSmokeCodex(t *testing.T) {
	codex := clientSmoke(t, "codex")
	bin := smokeBinary(t)
	a, dbPath := openApp(t)
	createActive(t, a)
	d := clientSmokeEnv(t)

	mustSmoke(t, d, bin, "setup", "codex", "--db", dbPath)

	list := mustSmoke(t, d, codex, "mcp", "list")
	found := false
	for _, line := range strings.Split(list.Stdout, "\n") {
		if f := strings.Fields(line); len(f) >= 3 && f[0] == "casebook" && f[1] == bin && f[2] == "mcp" {
			found = true
		}
	}
	if !found {
		t.Errorf("codex mcp list has no casebook row running %s mcp\n%s", bin, list.trimmed())
	}

	get := mustSmoke(t, d, codex, "mcp", "get", "casebook", "--json")
	var got struct {
		Name      string `json:"name"`
		Enabled   bool   `json:"enabled"`
		Transport struct {
			Type    string   `json:"type"`
			Command string   `json:"command"`
			Args    []string `json:"args"`
		} `json:"transport"`
	}
	if err := json.Unmarshal([]byte(get.Stdout), &got); err != nil {
		t.Fatalf("codex mcp get casebook --json is not JSON: %v\n%s", err, get.trimmed())
	}
	want := []string{"mcp", "--db", dbPath}
	if got.Name != "casebook" || !got.Enabled || got.Transport.Type != "stdio" || got.Transport.Command != bin ||
		strings.Join(got.Transport.Args, "\x00") != strings.Join(want, "\x00") {
		t.Errorf("codex mcp get casebook = %+v, want enabled stdio %s %q\n%s", got, bin, want, get.trimmed())
	}
}

func TestClientSmokeClaude(t *testing.T) {
	claude := clientSmoke(t, "claude")
	bin := smokeBinary(t)
	a, dbPath := openApp(t)
	createActive(t, a)
	d := clientSmokeEnv(t)

	mustSmoke(t, d, bin, "setup", "claude", "--db", dbPath)

	// claude mcp list health-checks every server: the casebook line must
	// report a live connection to our command.
	list := mustSmoke(t, d, claude, "mcp", "list")
	var line string
	for _, l := range strings.Split(list.Stdout, "\n") {
		if strings.HasPrefix(l, "casebook: ") {
			line = l
		}
	}
	if line == "" {
		t.Fatalf("claude mcp list has no casebook line\n%s", list.trimmed())
	}
	if !strings.HasPrefix(line, "casebook: "+bin+" mcp --db "+dbPath) || !strings.HasSuffix(line, "Connected") {
		t.Errorf("claude mcp list casebook line = %q, want our command and Connected", line)
	}

	get := mustSmoke(t, d, claude, "mcp", "get", "casebook")
	for _, want := range []string{"Scope: User config", "Connected", "Type: stdio", "Command: " + bin, "Args: mcp --db " + dbPath} {
		if !strings.Contains(get.Stdout, want) {
			t.Errorf("claude mcp get casebook lacks %q\n%s", want, get.trimmed())
		}
	}
}
