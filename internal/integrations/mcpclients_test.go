package integrations

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func snippet(t *testing.T, o MCPSnippetOptions) MCPSnippet {
	t.Helper()
	s, err := MCPSnippetFor(o)
	if err != nil {
		t.Fatalf("MCPSnippetFor(%+v): %v", o, err)
	}
	if !strings.HasSuffix(s.Snippet, "\n") || s.Location == "" || s.Title == "" {
		t.Fatalf("snippet %+v: want a newline-terminated snippet, a location and a title", s)
	}
	return s
}

// TestMCPSnippetJSONClients decodes every JSON snippet and checks it runs
// the CLI with mcp (and --db) under the server name casebook, including a
// Windows path whose backslashes JSON must escape.
func TestMCPSnippetJSONClients(t *testing.T) {
	for _, tc := range []struct{ goos, cli, db string }{
		{"linux", "/usr/local/bin/casebook", ""},
		{"darwin", "/Applications/Casebook.app/Contents/MacOS/bin/casebook", "/Users/jo/test db/casebook.db"},
		{"windows", `C:\Program Files\Casebook\bin\casebook.exe`, `D:\data\casebook.db`},
	} {
		wantArgs := []string{"mcp"}
		if tc.db != "" {
			wantArgs = append(wantArgs, "--db", tc.db)
		}
		for _, client := range []string{"", "generic", "claude-desktop", "cursor", "windsurf", "gemini", "vscode"} {
			s := snippet(t, MCPSnippetOptions{Client: client, CLIPath: tc.cli, DBPath: tc.db, GOOS: tc.goos, Home: "/home/jo"})
			if s.Format != FormatJSON {
				t.Fatalf("%s: format %s", client, s.Format)
			}
			var doc map[string]map[string]struct {
				Type    string   `json:"type"`
				Command string   `json:"command"`
				Args    []string `json:"args"`
			}
			if err := json.Unmarshal([]byte(s.Snippet), &doc); err != nil {
				t.Fatalf("%s/%s: snippet is not JSON: %v\n%s", tc.goos, client, err, s.Snippet)
			}
			key := "mcpServers"
			if client == "vscode" {
				key = "servers"
			}
			srv, ok := doc[key]["casebook"]
			if !ok || len(doc) != 1 {
				t.Fatalf("%s/%s: want only %s.casebook:\n%s", tc.goos, client, key, s.Snippet)
			}
			if srv.Command != tc.cli || !reflect.DeepEqual(srv.Args, wantArgs) {
				t.Fatalf("%s/%s: server = %+v", tc.goos, client, srv)
			}
			if (client == "vscode") != (srv.Type == "stdio") {
				t.Fatalf("%s/%s: type = %q", tc.goos, client, srv.Type)
			}
		}
	}
}

func TestMCPSnippetLocations(t *testing.T) {
	get := func(goos, client string, env map[string]string) string {
		return snippet(t, MCPSnippetOptions{Client: client, CLIPath: "/x/casebook", GOOS: goos, Home: "/home/jo",
			Getenv: func(k string) string { return env[k] }}).Location
	}
	if got := get("darwin", "claude-desktop", nil); got != filepath.Join("/home/jo", "Library", "Application Support", "Claude", "claude_desktop_config.json") {
		t.Errorf("claude-desktop on darwin: %q", got)
	}
	if got := get("windows", "claude-desktop", map[string]string{"APPDATA": "/appdata"}); got != filepath.Join("/appdata", "Claude", "claude_desktop_config.json") {
		t.Errorf("claude-desktop on windows: %q", got)
	}
	if got := get("linux", "cursor", nil); !strings.HasPrefix(got, filepath.Join("/home/jo", ".cursor", "mcp.json")) {
		t.Errorf("cursor: %q", got)
	}
	if got := get("linux", "windsurf", nil); got != filepath.Join("/home/jo", ".codeium", "windsurf", "mcp_config.json") {
		t.Errorf("windsurf: %q", got)
	}
	if got := get("linux", "gemini", nil); !strings.HasPrefix(got, filepath.Join("/home/jo", ".gemini", "settings.json")) {
		t.Errorf("gemini: %q", got)
	}
	// CODEX_HOME is made absolute with filepath.Abs, which on Windows gives
	// a rooted path like /codex the current drive, so use a path that is
	// absolute on every platform.
	codexHome := filepath.Join(t.TempDir(), "codex")
	if got := get("linux", "codex", map[string]string{"CODEX_HOME": codexHome}); got != filepath.Join(codexHome, "config.toml") {
		t.Errorf("codex with CODEX_HOME: %q", got)
	}
}

func TestMCPSnippetCodexAndClaudeCode(t *testing.T) {
	s := snippet(t, MCPSnippetOptions{Client: "codex", CLIPath: `C:\Program Files\Casebook\bin\casebook.exe`, GOOS: "windows", Home: `C:\Users\jo`})
	var doc struct {
		MCPServers map[string]struct {
			Command string   `toml:"command"`
			Args    []string `toml:"args"`
		} `toml:"mcp_servers"`
	}
	if err := toml.Unmarshal([]byte(s.Snippet), &doc); err != nil {
		t.Fatalf("codex snippet is not TOML: %v\n%s", err, s.Snippet)
	}
	if srv := doc.MCPServers["casebook"]; srv.Command != `C:\Program Files\Casebook\bin\casebook.exe` || !reflect.DeepEqual(srv.Args, []string{"mcp"}) {
		t.Fatalf("codex server = %+v", srv)
	}

	for _, tc := range []struct{ goos, cli, want string }{
		{"linux", "/opt/case book/casebook", "claude mcp add --scope user casebook -- '/opt/case book/casebook' mcp\n"},
		{"darwin", "/usr/local/bin/casebook", "claude mcp add --scope user casebook -- /usr/local/bin/casebook mcp\n"},
		{"windows", `C:\Program Files\Casebook\bin\casebook.exe`, `claude mcp add --scope user casebook -- 'C:\Program Files\Casebook\bin\casebook.exe' mcp` + "\n"},
		{"windows", `C:\Users\Jo's\casebook.exe`, `claude mcp add --scope user casebook -- 'C:\Users\Jo''s\casebook.exe' mcp` + "\n"},
	} {
		s := snippet(t, MCPSnippetOptions{Client: "claude-code", CLIPath: tc.cli, GOOS: tc.goos})
		if s.Format != FormatShell || s.Snippet != tc.want {
			t.Errorf("claude-code on %s = %q, want %q", tc.goos, s.Snippet, tc.want)
		}
	}
}

func TestMCPSnippetWSL(t *testing.T) {
	cli := `C:\Program Files\Casebook\bin\casebook.exe`
	s := snippet(t, MCPSnippetOptions{Client: "cursor", CLIPath: cli, DBPath: `C:\data\casebook.db`, GOOS: "windows", WSL: true, Home: `C:\Users\jo`})
	if s.Command != "/mnt/c/Program Files/Casebook/bin/casebook.exe" || s.Args[2] != `C:\data\casebook.db` {
		t.Fatalf("wsl cursor = %+v", s)
	}
	if s.Location != "~/.cursor/mcp.json (all projects) or .cursor/mcp.json in a project" {
		t.Errorf("wsl location = %q", s.Location)
	}
	for _, client := range []string{"claude-code", "codex"} {
		s := snippet(t, MCPSnippetOptions{Client: client, CLIPath: cli, GOOS: "windows", WSL: true})
		name := strings.TrimSuffix(client, "-code")
		if s.Format != FormatShell || s.Snippet != WSLSetupCommand(name, cli, "")+"\n" {
			t.Errorf("wsl %s = %q", client, s.Snippet)
		}
	}
	if _, err := MCPSnippetFor(MCPSnippetOptions{CLIPath: "/usr/bin/casebook", GOOS: "linux", WSL: true}); err == nil {
		t.Error("--wsl off Windows succeeded")
	}
	if _, err := MCPSnippetFor(MCPSnippetOptions{CLIPath: `\\server\share\casebook.exe`, GOOS: "windows", WSL: true}); err == nil {
		t.Error("--wsl with a UNC path succeeded")
	}
}

func TestMCPSnippetUnknownClient(t *testing.T) {
	_, err := MCPSnippetFor(MCPSnippetOptions{Client: "emacs", CLIPath: "/x/casebook", GOOS: "linux"})
	if err == nil || !strings.Contains(err.Error(), "cursor") {
		t.Fatalf("err = %v, want the list of clients", err)
	}
	if len(MCPClients()) != len(mcpClients) {
		t.Fatal("MCPClients does not list every client")
	}
}
