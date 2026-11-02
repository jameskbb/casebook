package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/app/apptest"
	"github.com/jameskbb/casebook/internal/integrations"
)

// D-16 real-world proof: real Claude Code and Codex sessions read the seeded
// investigation through the Casebook MCP server and record a question with
// their client provenance. Opt-in only (CASEBOOK_REAL_CLIENTS=1): these
// tests make real model calls.
//
// Safety: no test here writes or copies any client config or credential.
// Claude runs with --strict-mcp-config, a temp MCP config file,
// --setting-sources project from a temp cwd, a temp CLAUDE_CONFIG_DIR and a
// temp HOME; credentials reach it only through ANTHROPIC_API_KEY or
// CLAUDE_CODE_OAUTH_TOKEN in the environment. When that cannot authenticate
// the test skips; it runs against the real Claude config dir only with the
// extra opt-in CASEBOOK_REAL_CLIENTS_USE_REAL_CLAUDE_CONFIG=1. Codex runs
// only with a temp CODEX_HOME prepared by `casebook setup codex`; it is never
// run with CODEX_HOME unset or pointing at the real ~/.codex.

// realClientTimeout bounds one client attempt.
const realClientTimeout = 240 * time.Second

// realPrompt is the prompt both clients get.
const realPrompt = `Use the casebook MCP tools. List my Casebook investigations and summarize the active one in two sentences. Then add this open question to it with casebook_add_question: "Does the billing export refresh run nightly?" Finish with the new question ref.`

// realQuestion is the question text the prompt asks for.
const realQuestion = "Does the billing export refresh run nightly?"

// realClient skips unless CASEBOOK_REAL_CLIENTS=1 and name is on PATH, and
// returns the client's path.
func realClient(t *testing.T, name string) string {
	t.Helper()
	if os.Getenv("CASEBOOK_REAL_CLIENTS") != "1" {
		t.Skip("set CASEBOOK_REAL_CLIENTS=1 to run the real " + name + " session (makes real model calls)")
	}
	path, err := exec.LookPath(name)
	if err != nil {
		t.Skip(name + " is not on PATH; real client test skipped")
	}
	return path
}

// realUseRealClaudeConfig is the extra opt-in that lets TestRealClaudeSession
// fall back to the real Claude config dir when a temp one cannot
// authenticate.
const realUseRealClaudeConfig = "CASEBOOK_REAL_CLIENTS_USE_REAL_CLAUDE_CONFIG"

// realEnv is os.Environ() without the enclosing session's variables
// (parentSessionVar), any CASEBOOK_ variable and every key in drop, plus
// extra. Credential variables (claudeCredentialEnv) are kept.
func realEnv(drop []string, extra ...string) []string {
	var env []string
	for _, kv := range os.Environ() {
		k, _, _ := strings.Cut(kv, "=")
		uk := strings.ToUpper(k)
		if parentSessionVar(uk) || strings.HasPrefix(uk, "CASEBOOK_") {
			continue
		}
		dropped := false
		for _, d := range drop {
			if uk == strings.ToUpper(d) {
				dropped = true
			}
		}
		if !dropped {
			env = append(env, kv)
		}
	}
	return append(env, extra...)
}

// realRun is one finished client run.
type realRun struct {
	Stdout, Stderr string
	Err            error
}

// runReal runs a client command from cwd with env and the per-attempt
// timeout, with stdin closed.
func runReal(t *testing.T, cwd string, env []string, name string, args ...string) realRun {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), realClientTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = env
	cmd.Dir = cwd
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	if ctx.Err() != nil {
		err = ctx.Err()
	}
	return realRun{Stdout: out.String(), Stderr: errb.String(), Err: err}
}

// realSeed returns an open App on a temp database seeded with the cloud
// spend fixture (active INV-0001).
func realSeed(t *testing.T) (*app.App, string) {
	t.Helper()
	a, db := openApp(t)
	apptest.SeedCloudSpend(t, a)
	return a, db
}

// realQuestionRow returns the question.add activity row for realQuestion,
// or false.
func realQuestionRow(t *testing.T, a *app.App) (app.Activity, bool) {
	t.Helper()
	rows, err := a.ListActivity(context.Background(), app.ActivityFilter{InvestigationRef: "INV-0001", Limit: 100})
	if err != nil {
		t.Fatalf("list activity: %v", err)
	}
	for _, r := range rows {
		if r.Action == "question.add" && strings.Contains(r.Summary, "billing export refresh") {
			return r, true
		}
	}
	return app.Activity{}, false
}

// realMentionsInvestigation reports whether a reply names the seeded
// investigation.
func realMentionsInvestigation(s string) bool {
	return strings.Contains(s, "INV-0001") || strings.Contains(strings.ToLower(s), "cloud bill")
}

// realExcerpt is s cut to n bytes.
func realExcerpt(s string, n int) string {
	if len(s) > n {
		return s[:n] + "\n[trimmed]"
	}
	return s
}

// absentMarker is what a missing member or file hashes as.
const absentMarker = "casebook-test:absent"

// realHash is the hex sha256 of b.
func realHash(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// claudeConfigHashes hashes, without printing any content, the whole real
// settings.json (where Claude Code keeps hooks) and the top-level
// mcpServers member of the real .claude.json, compacted (an absent member or
// file hashes absentMarker). The rest of .claude.json is Claude Code's own
// session state, which any running Claude Code session updates, so it is not
// compared.
func claudeConfigHashes(t *testing.T, p integrations.Paths) map[string]string {
	t.Helper()
	out := map[string]string{}
	if b, err := os.ReadFile(p.ClaudeSettings); err == nil {
		out["settings.json"] = realHash(b)
	} else if errors.Is(err, os.ErrNotExist) {
		out["settings.json"] = realHash([]byte(absentMarker))
	} else {
		t.Fatalf("read the real settings.json: %v", err)
	}
	members := map[string]json.RawMessage{}
	if b, err := os.ReadFile(p.ClaudeJSON); err == nil {
		if err := json.Unmarshal(b, &members); err != nil {
			t.Fatal("the real .claude.json is not a JSON object (content not shown)")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("read the real .claude.json: %v", err)
	}
	const key = "mcpServers"
	raw, ok := members[key]
	if !ok {
		out[".claude.json:"+key] = realHash([]byte(absentMarker))
		return out
	}
	var c bytes.Buffer
	if err := json.Compact(&c, raw); err != nil {
		t.Fatalf("compact .claude.json %s (content not shown)", key)
	}
	out[".claude.json:"+key] = realHash(c.Bytes())
	return out
}

// claudeNotLoggedIn reports whether a claude -p run failed for lack of
// authentication. Observed with Claude Code 2.1.287 and an empty
// CLAUDE_CONFIG_DIR: exit 1 with result "Not logged in · Please run /login".
func claudeNotLoggedIn(r realRun) bool {
	s := r.Stdout + r.Stderr
	return r.Err != nil && (strings.Contains(s, "Not logged in") || strings.Contains(s, "/login") ||
		strings.Contains(s, "Invalid API key"))
}

func TestRealClaudeSession(t *testing.T) {
	claude := realClient(t, "claude")
	bin := smokeBinary(t)
	a, db := realSeed(t)
	tmp := t.TempDir()
	cwd := filepath.Join(tmp, "cwd")
	if err := os.MkdirAll(cwd, 0o700); err != nil {
		t.Fatal(err)
	}
	mcpFile := filepath.Join(tmp, "mcp.json")
	server := integrations.ClaudeMCPServer(bin, integrations.ClaudeOptions{DBPath: db})
	if err := os.WriteFile(mcpFile, []byte(`{"mcpServers": {"casebook": `+string(server)+`}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	// The real config locations of this user (read only, for the hashes).
	real := integrations.ResolvePaths(os.Getenv, home)
	before := claudeConfigHashes(t, real)

	args := []string{"-p", "--strict-mcp-config", "--mcp-config", mcpFile,
		"--allowedTools", "mcp__casebook__*", "--setting-sources", "project",
		"--no-session-persistence", "--output-format", "json", realPrompt}

	// Attempt 1: a fresh, empty CLAUDE_CONFIG_DIR and HOME (nothing copied
	// in); credentials come only from ANTHROPIC_API_KEY or
	// CLAUDE_CODE_OAUTH_TOKEN when the environment has them.
	attempt := "temp CLAUDE_CONFIG_DIR"
	tempHome := t.TempDir()
	r := runReal(t, cwd, realEnv([]string{"CLAUDE_CONFIG_DIR", "HOME", "USERPROFILE", "XDG_CONFIG_HOME"},
		"CLAUDE_CONFIG_DIR="+t.TempDir(), "HOME="+tempHome, "USERPROFILE="+tempHome, "XDG_CONFIG_HOME="+t.TempDir()),
		claude, args...)
	if claudeNotLoggedIn(r) {
		reason := realExcerpt(strings.TrimSpace(claudeResultText(r.Stdout)), 200)
		if os.Getenv(realUseRealClaudeConfig) != "1" {
			t.Skipf("claude could not authenticate in a temp CLAUDE_CONFIG_DIR (set ANTHROPIC_API_KEY or CLAUDE_CODE_OAUTH_TOKEN, "+
				"or set %s=1 to allow a run against the real Claude config dir): %s", realUseRealClaudeConfig, reason)
		}
		t.Logf("attempt 1 (temp CLAUDE_CONFIG_DIR) could not authenticate: %s", reason)
		// Attempt 2, only with the extra opt-in: the real config dir (the
		// user's own CLAUDE_CONFIG_DIR, else ~/.claude) supplies
		// authentication. Claude Code itself updates its session state in
		// ~/.claude.json, as in any normal session; Casebook code never
		// writes it, and user settings, hooks and MCP servers are not loaded
		// (--setting-sources project, --strict-mcp-config). The hashes below
		// check that settings.json and the mcpServers member are unchanged.
		attempt = "real config dir (authentication only, " + realUseRealClaudeConfig + "=1)"
		r = runReal(t, cwd, realEnv(nil), claude, args...)
	}
	after := claudeConfigHashes(t, real)
	for k, v := range before {
		if after[k] != v {
			t.Errorf("real Claude config %s changed during the session: %s -> %s", k, v, after[k])
		} else {
			t.Logf("hash %s before = after = %s", k, v)
		}
	}
	if r.Err != nil {
		t.Fatalf("claude -p (%s): %v\nstdout:\n%s\nstderr:\n%s", attempt, r.Err, realExcerpt(r.Stdout, 3000), realExcerpt(r.Stderr, 2000))
	}
	result := claudeResultText(r.Stdout)
	t.Logf("authenticated with: %s", attempt)
	t.Logf("claude result (first 1500 bytes):\n%s", realExcerpt(result, 1500))
	if !realMentionsInvestigation(result) {
		t.Errorf("result does not mention INV-0001 or the cloud bill")
	}
	row, ok := realQuestionRow(t, a)
	if !ok {
		t.Fatalf("no question.add activity row for %q", realQuestion)
	}
	t.Logf("activity row: id=%d action=%s target=%s actor={%s, %s} summary=%q", row.ID, row.Action, row.TargetRef, row.Actor.Kind, row.Actor.Client, row.Summary)
	if row.Actor.Kind != app.ActorAI || row.Actor.Client != "claude-code" {
		t.Fatalf("question actor = %+v, want {ai, claude-code}", row.Actor)
	}
}

// claudeResultText is the result member of claude -p --output-format json,
// or the raw output when it does not decode.
func claudeResultText(stdout string) string {
	var out struct {
		Result string `json:"result"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &out); err != nil {
		return stdout
	}
	return out.Result
}

// codexAuthLine returns the last output line that reports an
// authentication failure (Codex's final error, after its retries), cut
// before its url, or "".
func codexAuthLine(s string) string {
	lines := strings.Split(s, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		l := strings.TrimSpace(lines[i])
		low := strings.ToLower(l)
		if strings.Contains(low, "401 unauthorized") || strings.Contains(low, "not logged in") ||
			strings.Contains(low, "codex login") || strings.Contains(low, "missing bearer") {
			if i := strings.Index(l, ", url:"); i >= 0 {
				l = l[:i]
			}
			return l
		}
	}
	return ""
}

func TestRealCodexSession(t *testing.T) {
	codex := realClient(t, "codex")
	bin := smokeBinary(t)
	a, db := realSeed(t)

	// Codex runs only with this temp CODEX_HOME, populated by setup codex
	// (isolated HOME as in the smoke tests). No credential is copied in.
	d := clientSmokeEnv(t)
	mustSmoke(t, d, bin, "setup", "codex", "--db", db)
	if d.CodexHome == "" || !strings.HasPrefix(d.CodexHome, os.TempDir()) {
		t.Fatalf("refusing to run codex with CODEX_HOME %q", d.CodexHome)
	}
	if _, err := os.Stat(filepath.Join(d.CodexHome, "auth.json")); err == nil {
		t.Fatal("a credential file is present in the temp CODEX_HOME")
	}
	cwd := filepath.Join(d.Home, "cwd")
	if err := os.MkdirAll(cwd, 0o700); err != nil {
		t.Fatal(err)
	}
	// default_tools_approval_mode="approve" is Codex's per-server approval
	// setting (accepted values auto, prompt, writes, approve); it pre-approves
	// only the casebook server's tools. The bypass flag is never used.
	r := runReal(t, cwd, d.Env, codex, "exec", "--skip-git-repo-check", "--ephemeral", "-s", "read-only",
		"-c", `mcp_servers.casebook.default_tools_approval_mode="approve"`, realPrompt)
	if r.Err != nil {
		if line := codexAuthLine(r.Stdout + "\n" + r.Stderr); line != "" {
			t.Skipf("codex could not authenticate with a temp CODEX_HOME (no credentials are copied): %s", line)
		}
		t.Fatalf("codex exec: %v\nstdout:\n%s\nstderr:\n%s", r.Err, realExcerpt(r.Stdout, 3000), realExcerpt(r.Stderr, 3000))
	}
	t.Logf("codex output (first 1500 bytes):\n%s", realExcerpt(r.Stdout, 1500))
	if !realMentionsInvestigation(r.Stdout + r.Stderr) {
		t.Errorf("codex output does not mention INV-0001 or the cloud bill")
	}
	row, ok := realQuestionRow(t, a)
	if !ok {
		t.Fatalf("no question.add activity row for %q", realQuestion)
	}
	t.Logf("activity row: id=%d action=%s target=%s actor={%s, %s} summary=%q", row.ID, row.Action, row.TargetRef, row.Actor.Kind, row.Actor.Client, row.Summary)
	if row.Actor.Kind != app.ActorAI || row.Actor.Client != "codex" {
		t.Fatalf("question actor = %+v, want {ai, codex}", row.Actor)
	}
}
