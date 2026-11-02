package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/jameskbb/casebook/internal/app/apptest"
)

// hookSettings mirrors the hooks object of settings.json.
type hookSettings struct {
	Hooks map[string][]struct {
		Matcher *string `json:"matcher"`
		Hooks   []struct {
			Type    string   `json:"type"`
			Command string   `json:"command"`
			Args    []string `json:"args"`
			Timeout int      `json:"timeout"`
		} `json:"hooks"`
	} `json:"hooks"`
}

// hookCmd is one installed hook handler: a shell command string (Args nil)
// or, in exec form (what setup writes on Windows), a program and its
// arguments.
type hookCmd struct {
	Command string
	Args    []string
}

// hookInstalled runs `casebook setup claude --hooks --db <db>` against temp
// client dirs and returns the parsed settings.json and the isolated child
// environment.
func hookInstalled(t *testing.T, dbPath string) (hookSettings, []string) {
	t.Helper()
	bin := binary(t)
	home, claudeDir, codexHome := t.TempDir(), t.TempDir(), t.TempDir()
	env := childEnv(t,
		"HOME="+home,
		"USERPROFILE="+home,
		"CLAUDE_CONFIG_DIR="+claudeDir,
		"CODEX_HOME="+codexHome,
		"XDG_CONFIG_HOME="+t.TempDir(),
	)
	setup := exec.Command(bin, "setup", "claude", "--hooks", "--db", dbPath)
	setup.Env = env
	out, err := setup.CombinedOutput()
	if err != nil {
		t.Fatalf("casebook setup claude --hooks: %v\n%s", err, out)
	}
	data, err := os.ReadFile(filepath.Join(claudeDir, "settings.json"))
	if err != nil {
		t.Fatalf("read settings.json: %v\nsetup output:\n%s", err, out)
	}
	var s hookSettings
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatalf("parse settings.json: %v\n%s", err, data)
	}
	return s, env
}

// hookCommandFor returns the single handler installed for event.
func hookCommandFor(t *testing.T, s hookSettings, event string) hookCmd {
	t.Helper()
	groups := s.Hooks[event]
	if len(groups) != 1 || len(groups[0].Hooks) != 1 {
		t.Fatalf("hooks.%s = %+v, want one group with one handler", event, groups)
	}
	h := groups[0].Hooks[0]
	if (runtime.GOOS == "windows") != (h.Args != nil) {
		t.Fatalf("hooks.%s handler = %+v, want the exec form on windows only", event, h)
	}
	return hookCmd{Command: h.Command, Args: h.Args}
}

// runHookShell runs an installed hook handler with stdin and env, as
// Claude Code does, and returns stdout and stderr: a shell-form command
// string through sh, an exec-form handler directly with no shell.
func runHookShell(t *testing.T, h hookCmd, stdin string, env []string) (string, string) {
	t.Helper()
	var cmd *exec.Cmd
	if h.Args != nil {
		cmd = exec.Command(h.Command, h.Args...)
	} else {
		if runtime.GOOS == "windows" {
			t.Skip("shell-form hook commands are run through sh; not available on windows")
		}
		cmd = exec.Command("sh", "-c", h.Command)
	}
	cmd.Env = env
	cmd.Stdin = strings.NewReader(stdin)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		t.Fatalf("run %+v: %v (want exit 0)\nstdout:\n%s\nstderr:\n%s", h, err, out.String(), errb.String())
	}
	return out.String(), errb.String()
}

// hookJSON decodes stdout as exactly one JSON object plus a newline.
func hookJSON(t *testing.T, stdout string) map[string]any {
	t.Helper()
	if !strings.HasSuffix(stdout, "}\n") || strings.Count(stdout, "\n") != 1 {
		t.Fatalf("stdout = %q, want one JSON object on one line", stdout)
	}
	var v map[string]any
	if err := json.Unmarshal([]byte(stdout), &v); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout)
	}
	return v
}

// TestHookSessionStartInstalled is the plan 04-02 tracer: setup claude
// --hooks writes the three hook groups, and the SessionStart command string
// it wrote, run through sh with a SessionStart payload, prints the active
// investigation's compact context as additionalContext (INT-02, INT-07).
func TestHookSessionStartInstalled(t *testing.T) {
	bin := binary(t)
	a, dbPath := openApp(t)
	apptest.SeedCloudSpend(t, a)

	s, env := hookInstalled(t, dbPath)
	want := map[string]string{
		"SessionStart": "startup|resume|clear|compact",
		"Stop":         "",
		"PreCompact":   "manual|auto",
	}
	if len(s.Hooks) != len(want) {
		t.Errorf("hooks has %d events, want %d: %+v", len(s.Hooks), len(want), s.Hooks)
	}
	for event, matcher := range want {
		groups := s.Hooks[event]
		if len(groups) != 1 || len(groups[0].Hooks) != 1 {
			t.Fatalf("hooks.%s = %+v, want one group with one handler", event, groups)
		}
		g := groups[0]
		switch {
		case matcher == "" && g.Matcher != nil:
			t.Errorf("hooks.%s matcher = %q, want none", event, *g.Matcher)
		case matcher != "" && (g.Matcher == nil || *g.Matcher != matcher):
			t.Errorf("hooks.%s matcher = %v, want %q", event, g.Matcher, matcher)
		}
		h := g.Hooks[0]
		if h.Type != "command" || h.Timeout != 10 {
			t.Errorf("hooks.%s handler = %+v, want type command, timeout 10", event, h)
		}
		if !strings.Contains(h.Command, bin) {
			t.Errorf("hooks.%s command = %q, want it to run %s", event, h.Command, bin)
		}
	}

	cmd := hookCommandFor(t, s, "SessionStart")
	payload := `{"session_id":"t-1","hook_event_name":"SessionStart","source":"startup","transcript_path":"/nonexistent/transcript.jsonl"}`
	stdout, stderr := runHookShell(t, cmd, payload, env)
	if stderr != "" {
		t.Errorf("stderr = %q, want empty", stderr)
	}
	v := hookJSON(t, stdout)
	hso, _ := v["hookSpecificOutput"].(map[string]any)
	if hso["hookEventName"] != "SessionStart" {
		t.Errorf("hookEventName = %v, want SessionStart", hso["hookEventName"])
	}
	text, _ := hso["additionalContext"].(string)
	for _, w := range []string{
		"Casebook active investigation (shared memory across AI sessions).",
		"INV-0001",
		"September cloud bill up 38%",
	} {
		if !strings.Contains(text, w) {
			t.Errorf("additionalContext lacks %q:\n%s", w, text)
		}
	}
}

// TestHookCheckpointInstalled runs the installed SessionStart, Stop and
// PreCompact command strings through sh: activity recorded before a session
// started never triggers a reminder, the Stop reminder for the session's own
// material activity is printed once per session, and PreCompact prints the
// user-visible systemMessage (INT-08, WR-02).
func TestHookCheckpointInstalled(t *testing.T) {
	a, dbPath := openApp(t)
	fx := apptest.SeedCloudSpend(t, a)
	s, env := hookInstalled(t, dbPath)

	start := hookCommandFor(t, s, "SessionStart")
	stop := hookCommandFor(t, s, "Stop")
	for _, session := range []string{"t-1", "t-2"} {
		runHookShell(t, start, `{"session_id":"`+session+`","hook_event_name":"SessionStart","source":"startup"}`, env)
	}
	payload := `{"session_id":"t-1","hook_event_name":"Stop","stop_hook_active":false,"transcript_path":"/nonexistent/t.jsonl"}`
	// A session that just started is not reminded yet (AR-01: the gate is
	// the session's age, not what it wrote to Casebook).
	if stdout, stderr := runHookShell(t, stop, payload, env); stdout != "" || stderr != "" {
		t.Errorf("Stop in a new session: stdout %q, stderr %q; want nothing", stdout, stderr)
	}
	// Age session t-1 by an hour in the state file the hooks share.
	backdateHookSession(t, filepath.Join(filepath.Dir(dbPath), "hook-state.json"), "t-1", time.Hour)
	stdout, stderr := runHookShell(t, stop, payload, env)
	if stderr != "" {
		t.Errorf("stderr = %q, want empty", stderr)
	}
	v := hookJSON(t, stdout)
	hso, _ := v["hookSpecificOutput"].(map[string]any)
	reminder, _ := hso["additionalContext"].(string)
	if hso["hookEventName"] != "Stop" || !strings.Contains(reminder, fx.InvestigationRef) ||
		!strings.Contains(reminder, "casebook_checkpoint") {
		t.Errorf("Stop output = %s, want the checkpoint reminder for %s", stdout, fx.InvestigationRef)
	}
	if _, ok := v["decision"]; ok {
		t.Errorf("Stop output blocks: %s", stdout)
	}
	if stdout, stderr = runHookShell(t, stop, payload, env); stdout != "" || stderr != "" {
		t.Errorf("second Stop run for t-1: stdout %q, stderr %q; want nothing", stdout, stderr)
	}

	pre := hookCommandFor(t, s, "PreCompact")
	stdout, _ = runHookShell(t, pre, `{"session_id":"t-2","hook_event_name":"PreCompact","trigger":"manual"}`, env)
	v = hookJSON(t, stdout)
	msg, _ := v["systemMessage"].(string)
	if len(v) != 1 || !strings.Contains(msg, fx.InvestigationRef) {
		t.Errorf("PreCompact output = %s, want only a systemMessage naming %s", stdout, fx.InvestigationRef)
	}
}

// backdateHookSession moves session's first-seen time in the hook state file
// at path back by d, as if the session had started d earlier.
func backdateHookSession(t *testing.T, path, session string, d time.Duration) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var st struct {
		Version  int                          `json:"version"`
		Sessions map[string]map[string]string `json:"sessions"`
	}
	if err := json.Unmarshal(data, &st); err != nil {
		t.Fatal(err)
	}
	cur, ok := st.Sessions[session]
	if !ok {
		t.Fatalf("session %s not in %s", session, path)
	}
	at, err := time.Parse(time.RFC3339Nano, cur["first_seen"])
	if err != nil {
		t.Fatal(err)
	}
	cur["first_seen"] = at.Add(-d).UTC().Format(time.RFC3339Nano)
	out, err := json.Marshal(st)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, out, 0o600); err != nil {
		t.Fatal(err)
	}
}
