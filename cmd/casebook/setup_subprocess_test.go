package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestSetupClaudeLaunchesServer is the phase 4 tracer: the real binary runs
// `casebook setup claude --db <db>` against temp client dirs, and the exact
// command and args it wrote into .claude.json start a working MCP server on
// that database (INT-01).
func TestSetupClaudeLaunchesServer(t *testing.T) {
	bin := binary(t)
	a, dbPath := openApp(t)
	inv := createActive(t, a)

	home, claudeDir, codexHome := t.TempDir(), t.TempDir(), t.TempDir()
	cfg := filepath.Join(claudeDir, ".claude.json")
	original := "{\n  \"numStartups\": 7,\n  \"projects\": {}\n}\n"
	if err := os.WriteFile(cfg, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	isolated := []string{
		"HOME=" + home,
		"USERPROFILE=" + home,
		"CLAUDE_CONFIG_DIR=" + claudeDir,
		"CODEX_HOME=" + codexHome,
		"XDG_CONFIG_HOME=" + t.TempDir(),
	}
	setup := exec.Command(bin, "setup", "claude", "--db", dbPath)
	setup.Env = childEnv(t, isolated...)
	out, err := setup.CombinedOutput()
	if err != nil {
		t.Fatalf("casebook setup claude: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "added mcpServers.casebook") {
		t.Errorf("setup output = %q, want it to report added mcpServers.casebook", out)
	}

	data, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		NumStartups int `json:"numStartups"`
		MCPServers  map[string]struct {
			Type    string            `json:"type"`
			Command string            `json:"command"`
			Args    []string          `json:"args"`
			Env     map[string]string `json:"env"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse written config: %v\n%s", err, data)
	}
	if doc.NumStartups != 7 {
		t.Errorf("numStartups = %d, want 7 (unrelated keys must survive)", doc.NumStartups)
	}
	entry, ok := doc.MCPServers["casebook"]
	if !ok {
		t.Fatalf("no mcpServers.casebook in:\n%s", data)
	}
	wantCmd, err := filepath.EvalSymlinks(bin)
	if err != nil {
		t.Fatal(err)
	}
	gotCmd, err := filepath.EvalSymlinks(entry.Command)
	if err != nil {
		t.Fatalf("command %s does not resolve: %v", entry.Command, err)
	}
	if !filepath.IsAbs(entry.Command) || gotCmd != wantCmd {
		t.Errorf("command = %s, want the absolute binary path %s", entry.Command, wantCmd)
	}
	if entry.Type != "stdio" || entry.Env == nil || len(entry.Env) != 0 {
		t.Errorf("entry = %+v, want type stdio and env {}", entry)
	}
	if got, want := strings.Join(entry.Args, "\x00"), strings.Join([]string{"mcp", "--db", dbPath}, "\x00"); got != want {
		t.Fatalf("args = %q, want [mcp --db %s]", entry.Args, dbPath)
	}

	// Launch exactly what Claude Code would.
	cmd := exec.Command(entry.Command, entry.Args...)
	cmd.Env = childEnv(t, isolated...)
	stderr := &lockedBuffer{}
	cmd.Stderr = stderr
	client := mcp.NewClient(&mcp.Implementation{Name: "claude-code", Version: "test"}, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cs, err := client.Connect(ctx, &mcp.CommandTransport{Command: cmd}, &mcp.ClientSessionOptions{ProtocolVersion: "2025-06-18"})
	if err != nil {
		t.Fatalf("connect to the configured server: %v\nstderr:\n%s", err, stderr.String())
	}
	tools, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("list tools: %v\nstderr:\n%s", err, stderr.String())
	}
	if len(tools.Tools) != 21 {
		t.Errorf("tools/list returned %d tools, want 21", len(tools.Tools))
	}
	res := callTool(t, cs, "casebook_get_active_investigation", map[string]any{})
	if res.IsError {
		t.Fatalf("get active investigation: %s", resultText(res))
	}
	var active struct {
		Active        bool `json:"active"`
		Investigation struct {
			Ref string `json:"ref"`
		} `json:"investigation"`
	}
	decodeStructured(t, res, &active)
	if !active.Active || active.Investigation.Ref != inv.Ref {
		t.Errorf("active = %+v, want %s (the server must open the --db database)", active, inv.Ref)
	}
	if err := cs.Close(); err != nil {
		t.Fatalf("close session: %v\nstderr:\n%s", err, stderr.String())
	}
}
