package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	toml "github.com/pelletier/go-toml/v2"
)

// TestSetupCodexLaunchesServer is the plan 04-03 tracer: the real binary
// runs `casebook setup codex --db <db>` against temp client dirs, and the
// command and args it wrote into config.toml, read back with go-toml, start
// a working MCP server (INT-05).
func TestSetupCodexLaunchesServer(t *testing.T) {
	bin := binary(t)
	a, dbPath := openApp(t)
	createActive(t, a)

	home, claudeDir, codexHome := t.TempDir(), t.TempDir(), t.TempDir()
	cfg := filepath.Join(codexHome, "config.toml")
	const original = "# user settings\nmodel = \"o3\"\n\n[mcp_servers.other]\ncommand = \"other\"\n"
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
	setup := exec.Command(bin, "setup", "codex", "--db", dbPath)
	setup.Env = childEnv(t, isolated...)
	out, err := setup.CombinedOutput()
	if err != nil {
		t.Fatalf("casebook setup codex: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "added mcp_servers.casebook") {
		t.Errorf("setup output = %q, want it to report added mcp_servers.casebook", out)
	}

	data, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(data), original+"\n# BEGIN casebook (") || !strings.HasSuffix(string(data), "# END casebook\n") {
		t.Errorf("config.toml does not keep the original and end with the managed block:\n%s", data)
	}
	var doc struct {
		Model      string `toml:"model"`
		MCPServers map[string]struct {
			Command string   `toml:"command"`
			Args    []string `toml:"args"`
		} `toml:"mcp_servers"`
	}
	if err := toml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse written config: %v\n%s", err, data)
	}
	if doc.Model != "o3" || doc.MCPServers["other"].Command != "other" {
		t.Errorf("unrelated settings lost: %+v", doc)
	}
	entry, ok := doc.MCPServers["casebook"]
	if !ok {
		t.Fatalf("no mcp_servers.casebook in:\n%s", data)
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
	if got, want := strings.Join(entry.Args, "\x00"), strings.Join([]string{"mcp", "--db", dbPath}, "\x00"); got != want {
		t.Fatalf("args = %q, want [mcp --db %s]", entry.Args, dbPath)
	}

	// Launch exactly what Codex would.
	cmd := exec.Command(entry.Command, entry.Args...)
	cmd.Env = childEnv(t, isolated...)
	stderr := &lockedBuffer{}
	cmd.Stderr = stderr
	client := mcp.NewClient(&mcp.Implementation{Name: "codex", Version: "test"}, nil)
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
	if err := cs.Close(); err != nil {
		t.Fatalf("close session: %v\nstderr:\n%s", err, stderr.String())
	}
}
