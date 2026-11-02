package integrations

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// inspectPaths returns Paths rooted in fresh temp dirs (nothing written).
func inspectPaths(t *testing.T) Paths {
	t.Helper()
	claude, codex := t.TempDir(), t.TempDir()
	getenv := func(k string) string {
		switch k {
		case "CLAUDE_CONFIG_DIR":
			return claude
		case "CODEX_HOME":
			return codex
		}
		return ""
	}
	return ResolvePaths(getenv, t.TempDir())
}

// inspectCLI writes an executable stand-in for the casebook CLI and returns
// its symlink-resolved path.
func inspectCLI(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "bin", "casebook")
	if runtime.GOOS == "windows" {
		p += ".exe"
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

// inspectWrite writes data to path.
func inspectWrite(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
}

// inspectJSONString encodes s as a JSON string.
func inspectJSONString(t *testing.T, s string) string {
	t.Helper()
	b, err := marshalNoEscape(s)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestInspectClaudeMissing(t *testing.T) {
	p := inspectPaths(t)
	st := InspectClaude(p, inspectCLI(t))
	if st.ConfigPath != p.ClaudeJSON || st.ConfigExists || st.Registered || st.SettingsExists {
		t.Fatalf("missing config: %+v", st)
	}
	if st.Hooks == nil || len(st.Hooks) != 0 {
		t.Fatalf("Hooks = %#v, want empty non-nil", st.Hooks)
	}
}

func TestInspectClaudeInvalidAndUnregistered(t *testing.T) {
	p := inspectPaths(t)
	inspectWrite(t, p.ClaudeJSON, `{"mcpServers": {`)
	st := InspectClaude(p, "")
	if !st.ConfigExists || st.ConfigValid || st.Registered {
		t.Fatalf("invalid config: %+v", st)
	}
	inspectWrite(t, p.ClaudeJSON, `{"numStartups": 3, "mcpServers": {"other": {"command": "x"}}}`)
	st = InspectClaude(p, "")
	if !st.ConfigExists || !st.ConfigValid || st.Registered || st.Command != "" {
		t.Fatalf("unregistered config: %+v", st)
	}
	inspectWrite(t, p.ClaudeJSON, `{"mcpServers": []}`)
	if st := InspectClaude(p, ""); st.ConfigValid {
		t.Fatalf("mcpServers array counted as valid: %+v", st)
	}
}

func TestInspectClaudeRegistered(t *testing.T) {
	p := inspectPaths(t)
	cli := inspectCLI(t)
	if _, err := SetupClaude(cli, ClaudeOptions{Paths: p, Hooks: true, Now: fixedClock}); err != nil {
		t.Fatal(err)
	}
	st := InspectClaude(p, cli)
	want := ClaudeStatus{
		ConfigPath: p.ClaudeJSON, ConfigExists: true, ConfigValid: true, Registered: true,
		Command: cli, CommandExists: true, CommandExecutable: true, MatchesCLI: true,
		SettingsPath: p.ClaudeSettings, SettingsExists: true, SettingsValid: true,
		Hooks: []string{"SessionStart", "Stop", "PreCompact"},
	}
	if !reflect.DeepEqual(st, want) {
		t.Fatalf("status = %+v\nwant %+v", st, want)
	}
	// Another CLI path does not match.
	if st := InspectClaude(p, inspectCLI(t)); st.MatchesCLI || !st.CommandExists {
		t.Fatalf("other CLI matched: %+v", st)
	}
}

func TestInspectClaudeMissingCommand(t *testing.T) {
	p := inspectPaths(t)
	gone := filepath.Join(t.TempDir(), "old", "casebook")
	inspectWrite(t, p.ClaudeJSON, `{"mcpServers": {"casebook": {"type": "stdio", "command": `+inspectJSONString(t, gone)+`, "args": ["mcp"], "env": {}}}}`)
	st := InspectClaude(p, inspectCLI(t))
	if !st.Registered || st.Command != gone || st.CommandExists || st.CommandExecutable || st.MatchesCLI {
		t.Fatalf("missing command: %+v", st)
	}
}

func TestInspectClaudeNotExecutable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("execute bits do not exist on windows")
	}
	p := inspectPaths(t)
	cli := inspectCLI(t)
	if err := os.Chmod(cli, 0o644); err != nil {
		t.Fatal(err)
	}
	inspectWrite(t, p.ClaudeJSON, `{"mcpServers": {"casebook": {"command": `+inspectJSONString(t, cli)+`}}}`)
	st := InspectClaude(p, cli)
	if !st.CommandExists || st.CommandExecutable || !st.MatchesCLI {
		t.Fatalf("non-executable command: %+v", st)
	}
}

func TestInspectClaudeHooksMixedGroups(t *testing.T) {
	p := inspectPaths(t)
	cli := inspectCLI(t)
	inspectWrite(t, p.ClaudeJSON, `{}`)
	// Stop: a user group that also runs the casebook checkpoint hook
	// counts; SessionStart holds only a user hook; PreCompact is absent.
	inspectWrite(t, p.ClaudeSettings, `{"hooks": {
  "SessionStart": [{"matcher": "startup", "hooks": [{"type": "command", "command": "echo hi"}]}],
  "Stop": [{"hooks": [{"type": "command", "command": "notify"}, {"type": "command", "command": `+inspectJSONString(t, cli+" hook checkpoint")+`}]}]
}}`)
	st := InspectClaude(p, cli)
	if !slices.Equal(st.Hooks, []string{"Stop"}) || !st.SettingsValid {
		t.Fatalf("Hooks = %v (settings valid %v), want [Stop]", st.Hooks, st.SettingsValid)
	}
	inspectWrite(t, p.ClaudeSettings, `{"hooks": `)
	st = InspectClaude(p, cli)
	if !st.SettingsExists || st.SettingsValid || len(st.Hooks) != 0 || st.Hooks == nil {
		t.Fatalf("invalid settings: %+v", st)
	}
}

// TestInspectClaudeNoSecrets proves the status carries no env, args or
// other config content: no field of ClaudeStatus or CodexStatus is named
// after them, and a sentinel in the config never appears in the result.
func TestInspectClaudeNoSecrets(t *testing.T) {
	for _, typ := range []reflect.Type{reflect.TypeOf(ClaudeStatus{}), reflect.TypeOf(CodexStatus{})} {
		for i := 0; i < typ.NumField(); i++ {
			name := strings.ToLower(typ.Field(i).Name)
			if strings.Contains(name, "env") || strings.Contains(name, "arg") || strings.Contains(name, "content") {
				t.Errorf("%s.%s may carry config content", typ.Name(), typ.Field(i).Name)
			}
		}
	}
	const sentinel = "SENTINEL-inspect-7f3a"
	p := inspectPaths(t)
	cli := inspectCLI(t)
	inspectWrite(t, p.ClaudeJSON, `{"apiKey": "`+sentinel+`", "mcpServers": {"casebook": {"command": `+inspectJSONString(t, cli)+`, "args": ["mcp", "`+sentinel+`"], "env": {"TOKEN": "`+sentinel+`"}}}}`)
	if st := InspectClaude(p, cli); strings.Contains(fmt.Sprintf("%+v", st), sentinel) || !st.Registered {
		t.Fatalf("status leaks the sentinel or lost the entry: %+v", st)
	}
	inspectWrite(t, p.CodexConfig, "[mcp_servers.casebook]\ncommand = "+inspectJSONString(t, cli)+"\nargs = [\"mcp\", \""+sentinel+"\"]\n[mcp_servers.casebook.env]\nTOKEN = \""+sentinel+"\"\n")
	if st := InspectCodex(p, cli); strings.Contains(fmt.Sprintf("%+v", st), sentinel) || !st.Registered {
		t.Fatalf("codex status leaks the sentinel or lost the entry: %+v", st)
	}
}

func TestInspectCodexMissingAndInvalid(t *testing.T) {
	p := inspectPaths(t)
	st := InspectCodex(p, "")
	if st.ConfigPath != p.CodexConfig || st.ConfigExists || st.Registered {
		t.Fatalf("missing: %+v", st)
	}
	inspectWrite(t, p.CodexConfig, "model = \n")
	st = InspectCodex(p, "")
	if !st.ConfigExists || st.ConfigValid || st.Registered {
		t.Fatalf("invalid: %+v", st)
	}
}

func TestInspectCodexManaged(t *testing.T) {
	p := inspectPaths(t)
	cli := inspectCLI(t)
	inspectWrite(t, p.CodexConfig, "model = \"o3\"\n")
	if _, err := SetupCodex(cli, CodexOptions{Paths: p, Now: fixedClock}); err != nil {
		t.Fatal(err)
	}
	st := InspectCodex(p, cli)
	want := CodexStatus{
		ConfigPath: p.CodexConfig, ConfigExists: true, ConfigValid: true, Registered: true, Managed: true,
		Command: cli, CommandExists: true, CommandExecutable: true, MatchesCLI: true,
	}
	if st != want {
		t.Fatalf("status = %+v\nwant %+v", st, want)
	}
}

func TestInspectCodexHandWritten(t *testing.T) {
	p := inspectPaths(t)
	cli := inspectCLI(t)
	inspectWrite(t, p.CodexConfig, "[mcp_servers.casebook]\ncommand = "+inspectJSONString(t, cli)+"\nargs = [\"mcp\"]\n\n[mcp_servers.casebook.env]\nTOKEN = \"SENTINEL-codex\"\n")
	st := InspectCodex(p, cli)
	if !st.ConfigValid || !st.Registered || st.Managed || !st.MatchesCLI || !st.CommandExists {
		t.Fatalf("hand-written: %+v", st)
	}
	// A managed block that does not hold the table leaves it hand-written.
	inspectWrite(t, p.CodexConfig, codexBlockBegin+"\n"+codexBlockEnd+"\n[mcp_servers.casebook]\ncommand = "+inspectJSONString(t, cli)+"\n")
	if st := InspectCodex(p, cli); !st.Registered || st.Managed {
		t.Fatalf("empty block: %+v", st)
	}
	inspectWrite(t, p.CodexConfig, "[mcp_servers.other]\ncommand = \"x\"\n")
	if st := InspectCodex(p, cli); !st.ConfigValid || st.Registered {
		t.Fatalf("unregistered: %+v", st)
	}
}
