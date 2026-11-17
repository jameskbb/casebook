package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jameskbb/casebook/internal/integrations"
)

// TestSetupMCP: stdout is only the snippet (so it can be redirected into a
// file), stderr says where it goes, nothing is written anywhere and the
// database is never opened.
func TestSetupMCP(t *testing.T) {
	d := isolateClients(t, false)
	exe := filepath.Join(t.TempDir(), "casebook")
	db := filepath.Join(t.TempDir(), "never.db")

	r := runClientCLI(t, d, exe, "", "--db", db, "setup", "mcp", "--client", "cursor")
	wantCode(t, r, 0)
	var doc struct {
		MCPServers map[string]struct {
			Command string   `json:"command"`
			Args    []string `json:"args"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal([]byte(r.Stdout), &doc); err != nil {
		t.Fatalf("stdout is not only the JSON snippet: %v\n%s", err, r.Stdout)
	}
	srv := doc.MCPServers["casebook"]
	if srv.Command != exe || strings.Join(srv.Args, " ") != "mcp --db "+db {
		t.Fatalf("server = %+v", srv)
	}
	wantContains(t, r.Stderr, "casebook: Cursor: merge into "+filepath.Join(d.Home, ".cursor", "mcp.json"))
	if _, err := os.Stat(db); !os.IsNotExist(err) {
		t.Fatalf("setup mcp touched the database: %v", err)
	}
	if entries, _ := os.ReadDir(d.Home); len(entries) != 0 {
		t.Fatalf("setup mcp wrote into the home directory: %v", entries)
	}

	r = runClientCLI(t, d, exe, "", "--json", "setup", "mcp", "--client", "codex")
	wantCode(t, r, 0)
	var s integrations.MCPSnippet
	decodeJSON(t, r.Stdout, &s)
	if s.Client != "codex" || s.Format != integrations.FormatTOML || s.Location != d.Paths.CodexConfig ||
		!strings.Contains(s.Snippet, "[mcp_servers.casebook]") {
		t.Fatalf("--json codex = %+v", s)
	}

	r = runClientCLI(t, d, exe, "", "setup", "mcp")
	wantCode(t, r, 0)
	wantContains(t, r.Stdout, `"mcpServers"`)

	r = runClientCLI(t, d, exe, "", "setup", "mcp", "--list")
	wantCode(t, r, 0)
	for _, c := range integrations.MCPClients() {
		wantContains(t, r.Stdout, c.Name)
	}

	wantCode(t, runClientCLI(t, d, exe, "", "setup", "mcp", "--client", "emacs"), 2)
	wantCode(t, runClientCLI(t, d, exe, "", "setup", "mcp", "--list", "--client", "cursor"), 2)
	old := setupMCPGOOS
	setupMCPGOOS = "linux"
	t.Cleanup(func() { setupMCPGOOS = old })
	wantCode(t, runClientCLI(t, d, exe, "", "setup", "mcp", "--wsl"), 2)
}
