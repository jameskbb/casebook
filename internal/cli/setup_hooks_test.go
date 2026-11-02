package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/jameskbb/casebook/internal/integrations"
)

// hookSettingsEvents returns the hook events in settings.json, each with
// the invocations of its groups: the command of a shell-form handler, or
// for an exec-form handler (the form setup writes on Windows) its command
// and args joined by spaces. Temp paths need no quoting in the shell form,
// so both forms of the same invocation read the same.
func hookSettingsEvents(t *testing.T, path string) map[string][]string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Hooks map[string][]struct {
			Hooks []struct {
				Command string   `json:"command"`
				Args    []string `json:"args"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse %s: %v\n%s", path, err, data)
	}
	out := map[string][]string{}
	for event, groups := range doc.Hooks {
		for _, g := range groups {
			for _, h := range g.Hooks {
				if runtime.GOOS == "windows" && strings.Contains(h.Command, "casebook") && h.Args == nil {
					t.Errorf("hooks.%s: Casebook handler %q is not in exec form on windows", event, h.Command)
				}
				out[event] = append(out[event], strings.Join(append([]string{h.Command}, h.Args...), " "))
			}
		}
	}
	return out
}

// jsonInner is s as it appears inside a JSON string, without the quotes.
func jsonInner(s string) string {
	b, _ := json.Marshal(s)
	return string(b[1 : len(b)-1])
}

func TestSetupClaudeHooks(t *testing.T) {
	d := isolateClients(t, true)
	exe := fakeCLIPath(t)
	db := filepath.Join(t.TempDir(), "x.db")
	const user = "{\n  \"hooks\": {\n    \"Stop\": [\n      {\n        \"hooks\": [\n          {\n            \"type\": \"command\",\n            \"command\": \"notify-send done\"\n          }\n        ]\n      }\n    ]\n  }\n}\n"
	if err := os.WriteFile(d.Paths.ClaudeSettings, []byte(user), 0o600); err != nil {
		t.Fatal(err)
	}

	r := runClientCLI(t, d, exe, "", "setup", "claude", "--hooks", "--db", db)
	wantCode(t, r, 0)
	wantContains(t, r.Stdout,
		d.Paths.ClaudeJSON+": added mcpServers.casebook",
		d.Paths.ClaudeSettings+": added hooks.SessionStart, hooks.Stop, hooks.PreCompact",
		"backup: "+d.Paths.ClaudeSettings+".casebook-backup-",
		"hooks run in new sessions")
	got := hookSettingsEvents(t, d.Paths.ClaudeSettings)
	wantStop := []string{"notify-send done", exe + " --db " + db + " hook checkpoint"}
	if strings.Join(got["Stop"], "|") != strings.Join(wantStop, "|") {
		t.Errorf("Stop commands = %q, want %q", got["Stop"], wantStop)
	}
	if c := got["SessionStart"]; len(c) != 1 || c[0] != exe+" --db "+db+" hook session-start" {
		t.Errorf("SessionStart commands = %q", c)
	}
	if c := got["PreCompact"]; len(c) != 1 || !strings.HasSuffix(c[0], " hook checkpoint") {
		t.Errorf("PreCompact commands = %q", c)
	}
	if _, err := os.Stat(db); !os.IsNotExist(err) {
		t.Errorf("setup opened or created the database (%v)", err)
	}

	// Rerun: both files unchanged, no new backup.
	n := len(backups(t, d.Paths.ClaudeSettings))
	r = runClientCLI(t, d, exe, "", "--json", "setup", "claude", "--hooks", "--db", db)
	wantCode(t, r, 0)
	var rep setupJSON
	decodeJSON(t, r.Stdout, &rep)
	if len(rep.Changes) != 2 || rep.Changes[0].Action != "unchanged" || rep.Changes[1].Action != "unchanged" ||
		rep.Changes[1].File != d.Paths.ClaudeSettings {
		t.Errorf("rerun changes = %+v", rep.Changes)
	}
	if len(backups(t, d.Paths.ClaudeSettings)) != n {
		t.Error("rerun wrote a settings.json backup")
	}

	// Remove reports both files and restores the user's settings, except
	// that the event arrays install added stay as empty arrays (remove never
	// deletes a hooks container; WR-03).
	r = runClientCLI(t, d, exe, "", "setup", "claude", "--remove")
	wantCode(t, r, 0)
	wantContains(t, r.Stdout,
		d.Paths.ClaudeJSON+": removed mcpServers.casebook",
		d.Paths.ClaudeSettings+": removed hooks.SessionStart, hooks.Stop, hooks.PreCompact")
	wantUser := strings.Replace(user, "    ]\n  }\n}\n", "    ],\n    \"SessionStart\": [],\n    \"PreCompact\": []\n  }\n}\n", 1)
	if b, _ := os.ReadFile(d.Paths.ClaudeSettings); string(b) != wantUser {
		t.Errorf("settings.json after remove = %q, want %q", b, wantUser)
	}

	r = runClientCLI(t, d, exe, "", "--json", "setup", "claude", "--remove")
	wantCode(t, r, 0)
	decodeJSON(t, r.Stdout, &rep)
	if len(rep.Changes) != 2 || rep.Changes[1].Action != "unchanged" || rep.Changes[1].Entries == nil ||
		len(rep.Changes[1].Entries) != 0 {
		t.Errorf("second remove changes = %+v", rep.Changes)
	}
	r = runClientCLI(t, d, exe, "", "setup", "claude", "--remove")
	if strings.Contains(r.Stdout, " \n") {
		t.Errorf("text report has trailing spaces:\n%q", r.Stdout)
	}
}

func TestSetupClaudeHooksJSON(t *testing.T) {
	d := isolateClients(t, false)
	exe := fakeCLIPath(t)
	r := runClientCLI(t, d, exe, "", "--json", "setup", "claude", "--hooks")
	wantCode(t, r, 0)
	var rep setupJSON
	decodeJSON(t, r.Stdout, &rep)
	if len(rep.Changes) != 2 {
		t.Fatalf("changes = %+v, want two", rep.Changes)
	}
	if c := rep.Changes[0]; c.File != d.Paths.ClaudeJSON || c.Action != "added" {
		t.Errorf("first change = %+v", c)
	}
	if c := rep.Changes[1]; c.File != d.Paths.ClaudeSettings || c.Action != "added" ||
		strings.Join(c.Entries, ",") != "hooks.SessionStart,hooks.Stop,hooks.PreCompact" {
		t.Errorf("second change = %+v", c)
	}
}

func TestSetupClaudeHooksPrint(t *testing.T) {
	d := isolateClients(t, true)
	exe := fakeCLIPath(t)
	r := runClientCLI(t, d, exe, "", "setup", "claude", "--print", "--hooks")
	wantCode(t, r, 0)
	want := integrations.ClaudeConfigPreview(exe, integrations.ClaudeOptions{Paths: d.Paths, Hooks: true})
	if r.Stdout != want {
		t.Errorf("stdout:\n%s\nwant exactly the preview:\n%s", r.Stdout, want)
	}
	wantContains(t, r.Stdout, `"mcpServers"`, `"hooks"`, jsonInner(exe), "session-start", d.Paths.ClaudeSettings)
	if names := dirNames(t, d.ClaudeDir); len(names) != 0 {
		t.Errorf("--print --hooks wrote %v", names)
	}
}

func TestSetupClaudeHooksRefused(t *testing.T) {
	d := isolateClients(t, true)
	const broken = `{"hooks": {"Stop": {"hooks": []}}}`
	if err := os.WriteFile(d.Paths.ClaudeSettings, []byte(broken), 0o600); err != nil {
		t.Fatal(err)
	}
	r := runClientCLI(t, d, fakeCLIPath(t), "", "setup", "claude", "--hooks")
	wantCode(t, r, 1)
	want := "casebook: refusing to change " + d.Paths.ClaudeSettings + ": hooks.Stop is not an array; nothing was written\n"
	if r.Stderr != want {
		t.Errorf("stderr = %q, want %q", r.Stderr, want)
	}
	if names := dirNames(t, d.ClaudeDir); len(names) != 1 {
		t.Errorf("refusal left %v, want only settings.json", names)
	}
}

func TestSetupClaudeHelpMentionsHooks(t *testing.T) {
	r := runCLI(t, "", "setup", "claude", "--help")
	wantCode(t, r, 0)
	wantContains(t, r.Stdout, "--hooks", "remove Casebook-owned entries (the MCP server and any Casebook hooks) and nothing else")
	r = runCLI(t, "", "hook", "--help")
	wantCode(t, r, 0)
	wantContains(t, r.Stdout, "session-start", "checkpoint", "never reads transcripts")
}

// TestSetupClaudeHooksScope: --scope marked writes "--scope marked" into
// every Casebook hook command, a rerun with the default --scope all takes
// it out again, and --scope needs --hooks and a known value.
func TestSetupClaudeHooksScope(t *testing.T) {
	d := isolateClients(t, true)
	exe := fakeCLIPath(t)
	r := runClientCLI(t, d, exe, "", "setup", "claude", "--hooks", "--scope", "marked")
	wantCode(t, r, 0)
	got := hookSettingsEvents(t, d.Paths.ClaudeSettings)
	for event, sub := range map[string]string{"SessionStart": "session-start", "Stop": "checkpoint", "PreCompact": "checkpoint"} {
		if c := got[event]; len(c) != 1 || c[0] != exe+" hook "+sub+" --scope marked" {
			t.Errorf("%s commands = %q, want the marked scope", event, c)
		}
	}
	r = runClientCLI(t, d, exe, "", "--json", "setup", "claude", "--hooks", "--scope", "marked")
	wantCode(t, r, 0)
	var rep setupJSON
	decodeJSON(t, r.Stdout, &rep)
	if len(rep.Changes) != 2 || rep.Changes[1].Action != "unchanged" {
		t.Errorf("rerun changes = %+v, want settings.json unchanged", rep.Changes)
	}

	r = runClientCLI(t, d, exe, "", "setup", "claude", "--hooks")
	wantCode(t, r, 0)
	got = hookSettingsEvents(t, d.Paths.ClaudeSettings)
	if c := got["Stop"]; len(c) != 1 || c[0] != exe+" hook checkpoint" {
		t.Errorf("Stop commands after the default scope = %q, want no scope flag", c)
	}

	for _, args := range [][]string{
		{"setup", "claude", "--scope", "marked"},
		{"setup", "claude", "--hooks", "--scope", "everywhere"},
		{"setup", "claude", "--remove", "--hooks", "--scope", "marked"},
	} {
		r = runClientCLI(t, d, exe, "", args...)
		if r.Code == 0 || !strings.Contains(r.Stderr, "--scope") {
			t.Errorf("%v: code %d, stderr %q; want a usage error naming --scope", args, r.Code, r.Stderr)
		}
	}
	r = runCLI(t, "", "setup", "claude", "--help")
	wantContains(t, r.Stdout, "--scope", "marked")
}

// TestSetupClaudeHooksQuiet: --quiet-when-inactive writes the flag into the
// session-start command only, a rerun without it takes the flag out again,
// and the flag needs --hooks.
func TestSetupClaudeHooksQuiet(t *testing.T) {
	d := isolateClients(t, true)
	exe := fakeCLIPath(t)
	r := runClientCLI(t, d, exe, "", "setup", "claude", "--hooks", "--quiet-when-inactive", "--scope", "marked")
	wantCode(t, r, 0)
	got := hookSettingsEvents(t, d.Paths.ClaudeSettings)
	if c := got["SessionStart"]; len(c) != 1 || c[0] != exe+" hook session-start --quiet-when-inactive --scope marked" {
		t.Errorf("SessionStart commands = %q, want the quiet flag before the scope", c)
	}
	if c := got["Stop"]; len(c) != 1 || c[0] != exe+" hook checkpoint --scope marked" {
		t.Errorf("Stop commands = %q, want no quiet flag", c)
	}
	r = runClientCLI(t, d, exe, "", "setup", "claude", "--hooks")
	wantCode(t, r, 0)
	got = hookSettingsEvents(t, d.Paths.ClaudeSettings)
	if c := got["SessionStart"]; len(c) != 1 || c[0] != exe+" hook session-start" {
		t.Errorf("SessionStart commands after a plain rerun = %q, want the default command", c)
	}
	for _, args := range [][]string{
		{"setup", "claude", "--quiet-when-inactive"},
		{"setup", "claude", "--remove", "--hooks", "--quiet-when-inactive"},
	} {
		r = runClientCLI(t, d, exe, "", args...)
		if r.Code == 0 || !strings.Contains(r.Stderr, "--quiet-when-inactive") {
			t.Errorf("%v: code %d, stderr %q; want a usage error naming --quiet-when-inactive", args, r.Code, r.Stderr)
		}
	}
	r = runClientCLI(t, d, exe, "", "setup", "claude", "--hooks", "--quiet-when-inactive", "--print")
	wantCode(t, r, 0)
	// The preview carries the shell form on macOS and Linux and the exec
	// form (command plus an args list) on Windows, so compare it whole.
	want := integrations.ClaudeConfigPreview(exe, integrations.ClaudeOptions{Paths: d.Paths, Hooks: true, HookQuiet: true})
	if r.Stdout != want {
		t.Errorf("stdout:\n%s\nwant exactly the preview:\n%s", r.Stdout, want)
	}
	wantContains(t, r.Stdout, "--quiet-when-inactive")
	if c := hookSettingsEvents(t, d.Paths.ClaudeSettings)["SessionStart"]; len(c) != 1 || c[0] != exe+" hook session-start" {
		t.Errorf("--print changed the SessionStart commands to %q", c)
	}
}
