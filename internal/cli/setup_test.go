package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jameskbb/casebook/internal/app/apptest"
	"github.com/jameskbb/casebook/internal/integrations"
)

// clientDirs are the isolated AI client config locations of one test.
type clientDirs struct {
	// Home is the fake home directory.
	Home string
	// ClaudeDir is Paths.ClaudeConfigDir: CLAUDE_CONFIG_DIR when set, else
	// <Home>/.claude (not created).
	ClaudeDir string
	// CodexHome is CODEX_HOME.
	CodexHome string
	// Paths are ResolvePaths of the isolated environment.
	Paths integrations.Paths
}

// isolateClients points HOME, USERPROFILE, CLAUDE_CONFIG_DIR (or "" when
// withConfigDir is false), CODEX_HOME and XDG_CONFIG_HOME at fresh temp
// directories so no test can reach the developer's real Claude Code or Codex
// config. It returns the resolved paths of that environment.
func isolateClients(t *testing.T, withConfigDir bool) clientDirs {
	t.Helper()
	d := clientDirs{Home: t.TempDir(), CodexHome: t.TempDir()}
	t.Setenv("HOME", d.Home)
	t.Setenv("USERPROFILE", d.Home)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("CODEX_HOME", d.CodexHome)
	if withConfigDir {
		t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	} else {
		t.Setenv("CLAUDE_CONFIG_DIR", "")
	}
	d.Paths = integrations.ResolvePaths(os.Getenv, d.Home)
	d.ClaudeDir = d.Paths.ClaudeConfigDir
	return d
}

// runClientCLI runs casebook in process against the isolated client dirs:
// CASEBOOK_ variables are hidden, the home directory is d.Home, the running
// executable is exe and stdin is not a terminal.
func runClientCLI(t *testing.T, d clientDirs, exe string, stdin string, args ...string) cliResult {
	t.Helper()
	var out, errb bytes.Buffer
	env := NewEnv(strings.NewReader(stdin), &out, &errb)
	env.Getenv = func(k string) string {
		if strings.HasPrefix(k, "CASEBOOK_") {
			return ""
		}
		return os.Getenv(k)
	}
	env.UserHomeDir = func() (string, error) { return d.Home, nil }
	env.Executable = func() (string, error) { return exe, nil }
	env.StdinIsTerminal = func() bool { return false }
	code := run(context.Background(), env, args)
	return cliResult{Stdout: out.String(), Stderr: errb.String(), Code: code}
}

// fakeCLIPath is an absolute casebook path for tests that never run it.
func fakeCLIPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "bin", "casebook")
}

// claudeServerJSON mirrors mcpServers.casebook.
type claudeServerJSON struct {
	Type    string            `json:"type"`
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env"`
}

// readClaudeServers returns the top-level keys of .claude.json in order and
// its mcpServers entries.
func readClaudeServers(t *testing.T, path string) ([]string, map[string]json.RawMessage) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	if _, err := dec.Token(); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	var keys []string
	var servers map[string]json.RawMessage
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		k := tok.(string)
		keys = append(keys, k)
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		if k == "mcpServers" {
			if err := json.Unmarshal(raw, &servers); err != nil {
				t.Fatalf("parse mcpServers: %v", err)
			}
		}
	}
	return keys, servers
}

// backups lists the Casebook backups of path.
func backups(t *testing.T, path string) []string {
	t.Helper()
	m, err := filepath.Glob(path + ".casebook-backup-*")
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestSetupClaude(t *testing.T) {
	const original = `{"numStartups": 3, "mcpServers": {"other": {"command": "x"}}, "projects": {}}`
	for _, withConfigDir := range []bool{true, false} {
		name := "home"
		if withConfigDir {
			name = "CLAUDE_CONFIG_DIR"
		}
		t.Run(name, func(t *testing.T) {
			d := isolateClients(t, withConfigDir)
			if withConfigDir {
				if got, want := d.Paths.ClaudeJSON, filepath.Join(os.Getenv("CLAUDE_CONFIG_DIR"), ".claude.json"); got != want {
					t.Fatalf("ClaudeJSON = %s, want %s", got, want)
				}
			} else if got, want := d.Paths.ClaudeJSON, filepath.Join(d.Home, ".claude.json"); got != want {
				t.Fatalf("ClaudeJSON = %s, want %s", got, want)
			}
			if err := os.WriteFile(d.Paths.ClaudeJSON, []byte(original), 0o600); err != nil {
				t.Fatal(err)
			}
			exe := fakeCLIPath(t)
			r := runClientCLI(t, d, exe, "", "setup", "claude")
			wantCode(t, r, 0)
			wantContains(t, r.Stdout, d.Paths.ClaudeJSON+": added mcpServers.casebook", "backup: ", "claude mcp get casebook")

			keys, servers := readClaudeServers(t, d.Paths.ClaudeJSON)
			if got := strings.Join(keys, ","); got != "numStartups,mcpServers,projects" {
				t.Errorf("top-level keys = %s, want numStartups,mcpServers,projects", got)
			}
			if got := string(servers["other"]); got != `{"command":"x"}` && !sameCompact(got, `{"command":"x"}`) {
				t.Errorf("mcpServers.other = %s", got)
			}
			var cb claudeServerJSON
			if err := json.Unmarshal(servers["casebook"], &cb); err != nil {
				t.Fatalf("decode casebook entry: %v", err)
			}
			if cb.Type != "stdio" || cb.Command != exe || strings.Join(cb.Args, " ") != "mcp" || cb.Env == nil || len(cb.Env) != 0 {
				t.Errorf("casebook entry = %+v, want stdio %s [mcp] {}", cb, exe)
			}
			bs := backups(t, d.Paths.ClaudeJSON)
			if len(bs) != 1 {
				t.Fatalf("backups = %v, want one", bs)
			}
			if b, _ := os.ReadFile(bs[0]); string(b) != original {
				t.Errorf("backup = %q, want the original bytes", b)
			}
		})
	}
}

// sameCompact reports whether two JSON texts have identical tokens.
func sameCompact(a, b string) bool {
	var ca, cb bytes.Buffer
	if json.Compact(&ca, []byte(a)) != nil || json.Compact(&cb, []byte(b)) != nil {
		return false
	}
	return ca.String() == cb.String()
}

// setupJSON mirrors setupReport.
type setupJSON struct {
	Client  string `json:"client"`
	CLIPath string `json:"cli_path"`
	DryRun  bool   `json:"dry_run"`
	Preview string `json:"preview"`
	Next    string `json:"next"`
	Changes []struct {
		File    string   `json:"file"`
		Action  string   `json:"action"`
		Backup  string   `json:"backup"`
		Entries []string `json:"entries"`
	} `json:"changes"`
}

// dirNames lists a directory's entries ("" list when it does not exist).
func dirNames(t *testing.T, dir string) []string {
	t.Helper()
	ents, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range ents {
		out = append(out, e.Name())
	}
	return out
}

func TestSetupClaudePrint(t *testing.T) {
	d := isolateClients(t, true)
	exe := fakeCLIPath(t)
	r := runClientCLI(t, d, exe, "", "setup", "claude", "--print")
	wantCode(t, r, 0)
	want := integrations.ClaudeConfigPreview(exe, integrations.ClaudeOptions{Paths: d.Paths})
	if r.Stdout != want {
		t.Errorf("stdout:\n%s\nwant exactly the preview:\n%s", r.Stdout, want)
	}
	if names := dirNames(t, d.ClaudeDir); len(names) != 0 {
		t.Errorf("--print wrote %v", names)
	}

	db := filepath.Join(t.TempDir(), "x.db")
	r = runClientCLI(t, d, exe, "", "setup", "claude", "--print", "--json", "--db", db)
	wantCode(t, r, 0)
	var got setupJSON
	decodeJSON(t, r.Stdout, &got)
	want = integrations.ClaudeConfigPreview(exe, integrations.ClaudeOptions{Paths: d.Paths, DBPath: db})
	if !got.DryRun || got.Preview != want || got.Client != "claude" || got.CLIPath != exe || got.Changes == nil || len(got.Changes) != 0 {
		t.Errorf("--print --json = %+v", got)
	}
	if !strings.Contains(r.Stdout, `"changes": []`) {
		t.Errorf("changes is not an empty array:\n%s", r.Stdout)
	}
	if names := dirNames(t, d.ClaudeDir); len(names) != 0 {
		t.Errorf("--print --json wrote %v", names)
	}
	if _, err := os.Stat(db); !os.IsNotExist(err) {
		t.Errorf("--print opened or created the database (%v)", err)
	}
}

func TestSetupClaudePrintRemove(t *testing.T) {
	d := isolateClients(t, true)
	r := runClientCLI(t, d, fakeCLIPath(t), "", "setup", "claude", "--print", "--remove")
	wantCode(t, r, 2)
	wantContains(t, r.Stderr, "--print cannot be combined with --remove")
	if names := dirNames(t, d.ClaudeDir); len(names) != 0 {
		t.Errorf("usage error wrote %v", names)
	}
}

func TestSetupClaudeRemove(t *testing.T) {
	d := isolateClients(t, false)
	exe := fakeCLIPath(t)
	const original = "{\n  \"mcpServers\": {\n    \"other\": {\n      \"command\": \"x\"\n    }\n  }\n}\n"
	if err := os.WriteFile(d.Paths.ClaudeJSON, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	wantCode(t, runClientCLI(t, d, exe, "", "setup", "claude"), 0)

	r := runClientCLI(t, d, exe, "", "setup", "claude", "--remove")
	wantCode(t, r, 0)
	wantContains(t, r.Stdout, d.Paths.ClaudeJSON+": removed mcpServers.casebook", "backup: ", "Restart Claude Code to unload Casebook.")
	if b, _ := os.ReadFile(d.Paths.ClaudeJSON); string(b) != original {
		t.Errorf("after remove = %q, want the original", b)
	}
	n := len(backups(t, d.Paths.ClaudeJSON))

	r = runClientCLI(t, d, exe, "", "setup", "claude", "--remove")
	wantCode(t, r, 0)
	wantContains(t, r.Stdout, d.Paths.ClaudeJSON+": unchanged mcpServers.casebook")
	if strings.Contains(r.Stdout, "backup: ") || len(backups(t, d.Paths.ClaudeJSON)) != n {
		t.Errorf("second remove wrote a backup:\n%s", r.Stdout)
	}
}

func TestSetupClaudeRemoveMissingFile(t *testing.T) {
	d := isolateClients(t, true)
	r := runClientCLI(t, d, fakeCLIPath(t), "", "setup", "claude", "--remove")
	wantCode(t, r, 0)
	wantContains(t, r.Stdout, "unchanged mcpServers.casebook")
	if names := dirNames(t, d.ClaudeDir); len(names) != 0 {
		t.Errorf("remove created %v", names)
	}
}

func TestSetupClaudeJSON(t *testing.T) {
	d := isolateClients(t, true)
	exe := fakeCLIPath(t)
	r := runClientCLI(t, d, exe, "", "--json", "setup", "claude")
	wantCode(t, r, 0)
	var raw any
	decodeJSON(t, r.Stdout, &raw)
	apptest.AssertNoJSONNulls(t, raw)
	var got setupJSON
	decodeJSON(t, r.Stdout, &got)
	if got.Client != "claude" || got.CLIPath != exe || got.DryRun || got.Next == "" || got.Preview != "" {
		t.Errorf("report = %+v", got)
	}
	if len(got.Changes) != 1 || got.Changes[0].Action != "added" || got.Changes[0].File != d.Paths.ClaudeJSON ||
		got.Changes[0].Backup != "" || strings.Join(got.Changes[0].Entries, ",") != "mcpServers.casebook" {
		t.Errorf("changes = %+v", got.Changes)
	}

	r = runClientCLI(t, d, exe, "", "--json", "setup", "claude")
	wantCode(t, r, 0)
	decodeJSON(t, r.Stdout, &got)
	if len(got.Changes) != 1 || got.Changes[0].Action != "unchanged" {
		t.Errorf("second run changes = %+v", got.Changes)
	}
}

func TestSetupClaudeRefused(t *testing.T) {
	d := isolateClients(t, true)
	const broken = `{"mcpServers": [], "secretish": "do-not-print"}`
	if err := os.WriteFile(d.Paths.ClaudeJSON, []byte(broken), 0o600); err != nil {
		t.Fatal(err)
	}
	r := runClientCLI(t, d, fakeCLIPath(t), "", "setup", "claude")
	wantCode(t, r, 1)
	want := "casebook: refusing to change " + d.Paths.ClaudeJSON + ": mcpServers is not a JSON object; nothing was written\n"
	if r.Stderr != want {
		t.Errorf("stderr = %q, want %q", r.Stderr, want)
	}
	if b, _ := os.ReadFile(d.Paths.ClaudeJSON); string(b) != broken {
		t.Errorf("file changed to %q", b)
	}
	if names := dirNames(t, d.ClaudeDir); len(names) != 1 {
		t.Errorf("dir = %v, want only .claude.json", names)
	}
}

func TestSetupClaudeRefusedJSON(t *testing.T) {
	d := isolateClients(t, true)
	exe := fakeCLIPath(t)
	const broken = `{"mcpServers": [], "secretish": "do-not-print"}`
	if err := os.WriteFile(d.Paths.ClaudeJSON, []byte(broken), 0o600); err != nil {
		t.Fatal(err)
	}
	r := runClientCLI(t, d, exe, "", "--json", "setup", "claude", "--hooks")
	wantCode(t, r, 1)
	var raw any
	decodeJSON(t, r.Stdout, &raw)
	apptest.AssertNoJSONNulls(t, raw)
	var got setupJSON
	decodeJSON(t, r.Stdout, &got)
	want := integrations.ClaudeConfigPreview(exe, integrations.ClaudeOptions{Paths: d.Paths, Hooks: true})
	if got.Client != "claude" || got.Preview != want || got.DryRun || len(got.Changes) != 0 || got.Next == "" {
		t.Errorf("refusal report = %+v", got)
	}
	if strings.Contains(r.Stdout, "do-not-print") {
		t.Error("the report quotes the refused file")
	}
	wantContains(t, r.Stderr, "refusing to change "+d.Paths.ClaudeJSON+": mcpServers is not a JSON object")
	if b, _ := os.ReadFile(d.Paths.ClaudeJSON); string(b) != broken {
		t.Errorf("file changed to %q", b)
	}
}

func TestSetupClaudeDB(t *testing.T) {
	d := isolateClients(t, true)
	exe := fakeCLIPath(t)
	dir := t.TempDir()
	t.Chdir(dir)
	r := runClientCLI(t, d, exe, "", "--db", "rel.db", "setup", "claude")
	wantCode(t, r, 0)
	_, servers := readClaudeServers(t, d.Paths.ClaudeJSON)
	var cb claudeServerJSON
	if err := json.Unmarshal(servers["casebook"], &cb); err != nil {
		t.Fatal(err)
	}
	wd, _ := os.Getwd()
	if want := []string{"mcp", "--db", filepath.Join(wd, "rel.db")}; strings.Join(cb.Args, "|") != strings.Join(want, "|") {
		t.Errorf("args = %q, want %q (an absolute --db)", cb.Args, want)
	}
	if _, err := os.Stat(filepath.Join(dir, "rel.db")); !os.IsNotExist(err) {
		t.Errorf("setup opened or created the database (%v)", err)
	}
}

func TestSetupNotesCasebookEnv(t *testing.T) {
	d := isolateClients(t, true)
	for _, c := range []struct {
		vars map[string]string
		args []string
		want string
	}{
		{map[string]string{"CASEBOOK_HOME": "/x"}, nil, "casebook: note: CASEBOOK_HOME is set in this shell but AI clients may start casebook without it; pass --db to wire this database explicitly\n"},
		{map[string]string{"CASEBOOK_DB": "/x.db", "CASEBOOK_HOME": "/x"}, nil, "casebook: note: CASEBOOK_DB and CASEBOOK_HOME are set in this shell"},
		{map[string]string{"CASEBOOK_DB": "/x.db"}, []string{"--db", filepath.Join(t.TempDir(), "a.db")}, ""},
		{nil, nil, ""},
	} {
		var out, errb bytes.Buffer
		env := NewEnv(strings.NewReader(""), &out, &errb)
		env.Getenv = func(k string) string {
			if strings.HasPrefix(k, "CASEBOOK_") {
				return c.vars[k]
			}
			return os.Getenv(k)
		}
		env.UserHomeDir = func() (string, error) { return d.Home, nil }
		env.Executable = func() (string, error) { return "/opt/casebook", nil }
		args := append(append([]string{}, c.args...), "setup", "claude", "--print")
		if code := run(context.Background(), env, args); code != 0 {
			t.Fatalf("exit %d: %s", code, errb.String())
		}
		if c.want == "" && errb.Len() != 0 || c.want != "" && !strings.HasPrefix(errb.String(), c.want) {
			t.Errorf("vars %v args %v: stderr = %q, want prefix %q", c.vars, c.args, errb.String(), c.want)
		}
	}
}

func TestSetupHelp(t *testing.T) {
	d := isolateClients(t, true)
	r := runClientCLI(t, d, fakeCLIPath(t), "", "setup")
	wantCode(t, r, 0)
	wantContains(t, r.Stdout, "claude", "backup", "--print")
	r = runClientCLI(t, d, fakeCLIPath(t), "", "setup", "claude", "--help")
	wantCode(t, r, 0)
	wantContains(t, r.Stdout, "--remove", "--print")
}
