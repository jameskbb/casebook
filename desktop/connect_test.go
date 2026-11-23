package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jameskbb/casebook/internal/integrations"
)

// connectDesktop is clientsDesktop with a CLI file on disk, so a connected
// tool reads as running this CLI. Every path is under a temp folder.
func connectDesktop(t *testing.T) (d *Desktop, home, cli string) {
	t.Helper()
	cli = filepath.Join(t.TempDir(), "casebook")
	if err := os.WriteFile(cli, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	d, home = clientsDesktop(t, cli)
	return d, home, cli
}

// uiErrOf decodes the JSON error a binding returns.
func uiErrOf(t *testing.T, err error) uiError {
	t.Helper()
	if err == nil {
		t.Fatal("no error")
	}
	var e uiError
	if jerr := json.Unmarshal([]byte(err.Error()), &e); jerr != nil {
		t.Fatalf("error is not the JSON shape: %v", err)
	}
	return e
}

func mkdir(t *testing.T, elem ...string) string {
	t.Helper()
	dir := filepath.Join(elem...)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestConnectClaudeCode(t *testing.T) {
	d, home, cli := connectDesktop(t)

	// Not on this computer: nothing is offered and nothing is written.
	if cc, _ := d.ClientConnections(); cc.Claude.Found || cc.Codex.Found || cc.ClaudeDesktop.Found {
		t.Fatalf("found in an empty home: %+v", cc)
	}
	if e := uiErrOf(t, errOnly(d.ConnectClient("claude"))); e.Kind != kindNotFound || e.Field != "tool" || e.Message != "Casebook could not find Claude Code on this computer." {
		t.Fatalf("not found = %+v", e)
	}
	if files := snapshotFiles(t, home); len(files) != 0 {
		t.Fatalf("a refused connect wrote files: %v", files)
	}

	// Claude Code's own settings, which must survive word for word.
	mkdir(t, home, ".claude")
	const own = "{\n  \"numStartups\": 7,\n  \"mcpServers\": {\n    \"files\": {\n      \"command\": \"npx\"\n    }\n  }\n}\n"
	config := filepath.Join(home, ".claude.json")
	if err := os.WriteFile(config, []byte(own), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := d.ConnectClient("claude")
	if err != nil {
		t.Fatal(err)
	}
	if got.Tool != "claude" || !got.Connected || !got.Changed ||
		got.Message != "Casebook added itself to Claude Code's settings and kept a backup of the file. Close Claude Code and open it again to start." {
		t.Fatalf("connect = %+v", got)
	}
	if len(got.Files) != 3 || got.Files[0].File != config || got.Files[0].Action != "added" || got.Files[0].Backup == "" ||
		filepath.Base(got.Files[1].File) != "settings.json" || got.Files[1].Action != "added" ||
		filepath.Base(got.Files[2].File) != "SKILL.md" || got.Files[2].Action != "added" {
		t.Fatalf("files = %+v", got.Files)
	}
	if b, _ := os.ReadFile(got.Files[0].Backup); string(b) != own {
		t.Error("the backup is not the file as it was")
	}
	after, _ := os.ReadFile(config)
	if !strings.HasPrefix(string(after), "{\n  \"numStartups\": 7,\n  \"mcpServers\": {\n    \"files\": {\n      \"command\": \"npx\"\n    },\n    \"casebook\": {") {
		t.Fatalf("the settings Claude Code had were not kept:\n%s", after)
	}
	cc, err := d.ClientConnections()
	if err != nil {
		t.Fatal(err)
	}
	if c := cc.Claude; !c.Found || !c.Configured || !c.MatchesCLI || c.ConfigProblem || c.Command != cli || len(c.Hooks) != 3 {
		t.Fatalf("after connect = %+v", c)
	}

	// Again: nothing to do, nothing written.
	before := snapshotFiles(t, home)
	again, err := d.ConnectClient("claude")
	if err != nil || again.Changed || !again.Connected || again.Message != "Claude Code is already connected. Nothing was changed." {
		t.Fatalf("second connect = %+v, %v", again, err)
	}
	if now := snapshotFiles(t, home); len(now) != len(before) {
		t.Fatalf("a connect with nothing to do wrote files: %d, then %d", len(before), len(now))
	}

	// Disconnect takes out only what connect put in.
	off, err := d.DisconnectClient("claude")
	if err != nil || !off.Changed || off.Connected || off.Message != "Casebook took itself out of Claude Code's settings and kept a backup of the file." {
		t.Fatalf("disconnect = %+v, %v", off, err)
	}
	if b, _ := os.ReadFile(config); string(b) != own {
		t.Fatalf("after disconnect the settings are not as they were:\n%s", b)
	}
	if _, err := os.Stat(filepath.Join(home, ".claude", "skills", "casebook", "SKILL.md")); !os.IsNotExist(err) {
		t.Error("the skill file was left behind")
	}
	if cc, _ := d.ClientConnections(); cc.Claude.Configured || len(cc.Claude.Hooks) != 0 {
		t.Fatalf("after disconnect = %+v", cc.Claude)
	}
	if off, err := d.DisconnectClient("claude"); err != nil || off.Changed || off.Message != "Claude Code was not connected. Nothing was changed." {
		t.Fatalf("second disconnect = %+v, %v", off, err)
	}
}

func errOnly(_ ClientChange, err error) error { return err }

// Codex is found by its program on PATH too, and its settings file is created
// when it has none; with an explicit data folder the entry carries --db.
func TestConnectCodex(t *testing.T) {
	d, home, cli := connectDesktop(t)
	d.clients.lookPath = func(name string) (string, error) {
		if name == "codex" {
			return "/usr/bin/codex", nil
		}
		return "", os.ErrNotExist
	}
	d.explicitDB = true
	got, err := d.ConnectClient("codex")
	if err != nil {
		t.Fatal(err)
	}
	if !got.Changed || got.Message != "Casebook added itself to Codex's settings. Close Codex and open it again to start." {
		t.Fatalf("connect = %+v", got)
	}
	config, _ := os.ReadFile(filepath.Join(home, ".codex", "config.toml"))
	if !strings.Contains(string(config), "[mcp_servers.casebook]") || !strings.Contains(string(config), "--db") || !strings.Contains(string(config), filepath.Base(d.dbPath)) {
		t.Fatalf("config.toml:\n%s", config)
	}
	if cc, _ := d.ClientConnections(); !cc.Codex.Found || !cc.Codex.Configured || !cc.Codex.MatchesCLI || cc.Codex.Command != cli {
		t.Fatalf("after connect = %+v", cc.Codex)
	}
	off, err := d.DisconnectClient("codex")
	if err != nil || !off.Changed {
		t.Fatalf("disconnect = %+v, %v", off, err)
	}
	if cc, _ := d.ClientConnections(); cc.Codex.Configured {
		t.Fatalf("after disconnect = %+v", cc.Codex)
	}
}

func TestConnectClaudeDesktop(t *testing.T) {
	d, home, cli := connectDesktop(t)
	dir := mkdir(t, home, ".config", "Claude")
	config := filepath.Join(dir, "claude_desktop_config.json")
	cc, err := d.ClientConnections()
	if err != nil {
		t.Fatal(err)
	}
	if c := cc.ClaudeDesktop; !c.Found || c.Configured || c.ConfigPath != config || c.SetupCommand != "" || c.Hooks == nil ||
		!strings.Contains(c.Snippet, `"mcpServers"`) || !strings.Contains(c.Snippet, jsonString(cli)) {
		t.Fatalf("before connect = %+v", c)
	}
	const own = "{\n  \"preferences\": {\n    \"menuBarEnabled\": false\n  }\n}\n"
	if err := os.WriteFile(config, []byte(own), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := d.ConnectClient("claude_desktop")
	if err != nil {
		t.Fatal(err)
	}
	if !got.Changed || len(got.Files) != 1 || got.Files[0].Backup == "" ||
		got.Message != "Casebook added itself to the Claude desktop app's settings and kept a backup of the file. Close the Claude desktop app and open it again to start." {
		t.Fatalf("connect = %+v", got)
	}
	want := "{\n  \"preferences\": {\n    \"menuBarEnabled\": false\n  },\n  \"mcpServers\": {\n    \"casebook\": {\n      \"command\": " + jsonString(cli) + ",\n      \"args\": [\n        \"mcp\"\n      ]\n    }\n  }\n}\n"
	if b, _ := os.ReadFile(config); string(b) != want {
		t.Fatalf("config:\n%s\nwant:\n%s", b, want)
	}
	if cc, _ := d.ClientConnections(); !cc.ClaudeDesktop.Configured || !cc.ClaudeDesktop.MatchesCLI {
		t.Fatalf("after connect = %+v", cc.ClaudeDesktop)
	}
	if again, err := d.ConnectClient("claude_desktop"); err != nil || again.Changed || again.Message != "The Claude desktop app is already connected. Nothing was changed." {
		t.Fatalf("second connect = %+v, %v", again, err)
	}
	off, err := d.DisconnectClient("claude_desktop")
	if err != nil || !off.Changed {
		t.Fatalf("disconnect = %+v, %v", off, err)
	}
	// The mcpServers object connect added stays, empty: Casebook cannot tell
	// whose it is once it exists.
	if b, _ := os.ReadFile(config); string(b) != "{\n  \"preferences\": {\n    \"menuBarEnabled\": false\n  },\n  \"mcpServers\": {}\n}\n" {
		t.Fatalf("after disconnect:\n%s", b)
	}
}

// A settings file Casebook cannot read safely is left exactly as it is, and
// the person is told in plain words.
func TestConnectRefusesAndExplains(t *testing.T) {
	d, home, _ := connectDesktop(t)
	if e := uiErrOf(t, errOnly(d.ConnectClient("cursor"))); e.Kind != kindValidation || e.Field != "tool" {
		t.Fatalf("unknown tool = %+v", e)
	}

	dir := mkdir(t, home, ".config", "Claude")
	config := filepath.Join(dir, "claude_desktop_config.json")
	const broken = "{\n  // my note\n  \"mcpServers\": {}\n}\n"
	if err := os.WriteFile(config, []byte(broken), 0o600); err != nil {
		t.Fatal(err)
	}
	if cc, _ := d.ClientConnections(); !cc.ClaudeDesktop.ConfigProblem {
		t.Fatalf("a file with a comment reads as fine: %+v", cc.ClaudeDesktop)
	}
	for _, change := range []func(string) (ClientChange, error){d.ConnectClient, d.DisconnectClient} {
		e := uiErrOf(t, errOnly(change("claude_desktop")))
		if e.Kind != kindConflict || e.Field != "settings" ||
			e.Message != "The settings file has a mistake in it, so Casebook changed nothing. Ask the person who set up the Claude desktop app to fix it, then try again." {
			t.Fatalf("refusal = %+v", e)
		}
	}
	if files := snapshotFiles(t, home); len(files) != 1 || files[config] != broken {
		t.Fatalf("a refused file was changed or a file was added: %v", files)
	}

	// Without the helper program there is nothing to point a tool at.
	missing, home2 := clientsDesktop(t, "")
	mkdir(t, home2, ".claude")
	if e := uiErrOf(t, errOnly(missing.ConnectClient("claude"))); e.Kind != kindNotFound || e.Field != "cli" {
		t.Fatalf("missing CLI = %+v", e)
	}
	if files := snapshotFiles(t, home2); len(files) != 0 {
		t.Fatalf("wrote without a CLI: %v", files)
	}
}

// The error says "nothing was changed" only when that is true.
func TestClientErr(t *testing.T) {
	refused := &integrations.RefusedError{File: "x", Reason: "not valid JSON"}
	changed := fmt.Errorf("writing: %w", integrations.ErrChangedUnderneath)
	first := []integrations.FileChange{{File: "a", Action: integrations.ActionAdded, Backup: "a.casebook-backup-1"}}
	for _, tc := range []struct {
		name        string
		done        []integrations.FileChange
		err         error
		kind, field string
		message     string
	}{
		{"refused", nil, refused, kindConflict, "settings", "The settings file has a mistake in it, so Casebook changed nothing. Ask the person who set up Codex to fix it, then try again."},
		{"changed underneath", nil, changed, kindConflict, "changed", "Codex's settings changed while Casebook was updating them. Nothing was changed. Try again."},
		{"second file changed underneath", first, changed, kindOther, "", "Casebook could not finish changing Codex's settings. Try again. A backup was kept at a.casebook-backup-1."},
		{"disk", nil, errors.New("disk full"), kindOther, "", "Casebook could not finish changing Codex's settings. Try again."},
	} {
		e := uiErrOf(t, clientErr("Codex", tc.done, tc.err))
		if e.Kind != tc.kind || e.Field != tc.field || e.Message != tc.message {
			t.Errorf("%s = %+v", tc.name, e)
		}
	}
}
