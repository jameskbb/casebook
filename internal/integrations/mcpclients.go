package integrations

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

// Any MCP client (ADR-0017): `casebook setup mcp` prints the configuration
// that registers Casebook with a client and where it goes. It never writes
// another program's config file; `casebook setup claude` and `casebook setup
// codex` remain the only writers (ADR-0009).

// Snippet formats.
const (
	FormatJSON  = "json"
	FormatTOML  = "toml"
	FormatShell = "shell"
)

// MCPClient is one client `casebook setup mcp` knows.
type MCPClient struct {
	// Name is the --client value.
	Name string `json:"name"`
	// Title is the client's display name.
	Title string `json:"title"`
}

// mcpClients lists the known clients in the order --list prints them.
var mcpClients = []MCPClient{
	{"generic", "Any MCP client (\"mcpServers\" JSON)"},
	{"claude-desktop", "Claude Desktop"},
	{"cursor", "Cursor"},
	{"vscode", "Visual Studio Code (GitHub Copilot agent mode)"},
	{"windsurf", "Windsurf"},
	{"gemini", "Gemini CLI"},
	{"claude-code", "Claude Code"},
	{"codex", "Codex"},
}

// MCPClients returns the clients `casebook setup mcp` can print a
// configuration for.
func MCPClients() []MCPClient {
	return append([]MCPClient(nil), mcpClients...)
}

// MCPSnippetOptions configure MCPSnippet.
type MCPSnippetOptions struct {
	// Client is an MCPClient name; "" means generic.
	Client string
	// CLIPath is the absolute casebook CLI path the client runs.
	CLIPath string
	// DBPath, when set, is passed as --db.
	DBPath string
	// GOOS is the operating system the CLI runs on (runtime.GOOS).
	GOOS string
	// Home is the user's home directory and Getenv reads the environment
	// (APPDATA, CODEX_HOME), for the config locations.
	Home   string
	Getenv Getenv
	// WSL prints the configuration for a client running inside WSL that
	// starts this Windows CLI through its /mnt/<drive> path. Only valid when
	// GOOS is windows.
	WSL bool
}

// MCPSnippet is a client configuration and where it goes.
type MCPSnippet struct {
	Client string `json:"client"`
	Title  string `json:"title"`
	// Format is json, toml or shell (a command to run).
	Format string `json:"format"`
	// Location says where the snippet goes (a file, or "run in a terminal").
	Location string `json:"location"`
	// Command and Args are what the client runs.
	Command string   `json:"command"`
	Args    []string `json:"args"`
	// Snippet is the text to merge or run, ending with a newline.
	Snippet string `json:"snippet"`
	// Note is an extra hint, "" when none.
	Note string `json:"note"`
}

// mcpServersEntry is one stdio server in the common "mcpServers" shape.
type mcpServersEntry struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

// vscodeEntry is one stdio server in VS Code's "servers" shape.
type vscodeEntry struct {
	Type    string   `json:"type"`
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

// MCPSnippetFor returns the configuration that registers the casebook CLI as
// the stdio MCP server "casebook" with o.Client. It reads and writes
// nothing. An unknown client, or WSL off Windows or with a CLI path WSL
// cannot run, is an error.
func MCPSnippetFor(o MCPSnippetOptions) (MCPSnippet, error) {
	name := o.Client
	if name == "" {
		name = "generic"
	}
	var client *MCPClient
	for i := range mcpClients {
		if mcpClients[i].Name == name {
			client = &mcpClients[i]
		}
	}
	if client == nil {
		names := make([]string, len(mcpClients))
		for i, c := range mcpClients {
			names[i] = c.Name
		}
		return MCPSnippet{}, fmt.Errorf("unknown client %q; use one of: %s", name, strings.Join(names, ", "))
	}
	command := o.CLIPath
	if o.WSL {
		if o.GOOS != "windows" {
			return MCPSnippet{}, fmt.Errorf("--wsl is for the Windows casebook.exe; inside WSL or on another system run casebook setup mcp without it")
		}
		wsl, ok := WSLPath(o.CLIPath)
		if !ok {
			return MCPSnippet{}, fmt.Errorf("WSL cannot run %s through a /mnt/<drive> path", o.CLIPath)
		}
		command = wsl
	}
	s := MCPSnippet{
		Client:  client.Name,
		Title:   client.Title,
		Format:  FormatJSON,
		Command: command,
		Args:    mcpArgs(o.DBPath),
	}
	loc := newLocator(o)
	switch client.Name {
	case "generic":
		s.Location = `the MCP server settings of your client; most accept this "mcpServers" object`
		s.Snippet = mcpServersJSON(s.Command, s.Args)
	case "claude-desktop":
		s.Location = loc.claudeDesktop()
		s.Snippet = mcpServersJSON(s.Command, s.Args)
		s.Note = "Claude Desktop starts servers outside any project, so it uses the case your AI uses everywhere (not a per-folder one). Restart Claude Desktop after editing the file."
	case "cursor":
		s.Location = loc.path(".cursor", "mcp.json") + " (all projects) or .cursor/mcp.json in a project"
		s.Snippet = mcpServersJSON(s.Command, s.Args)
	case "windsurf":
		s.Location = loc.path(".codeium", "windsurf", "mcp_config.json")
		s.Snippet = mcpServersJSON(s.Command, s.Args)
	case "gemini":
		s.Location = loc.path(".gemini", "settings.json") + " (all projects) or .gemini/settings.json in a project"
		s.Snippet = mcpServersJSON(s.Command, s.Args)
	case "vscode":
		s.Location = `.vscode/mcp.json in a workspace, or your user configuration (command palette: "MCP: Open User Configuration")`
		s.Snippet = encodeIndented(map[string]any{"servers": map[string]vscodeEntry{
			"casebook": {Type: "stdio", Command: s.Command, Args: s.Args}}})
	case "claude-code":
		s.Format = FormatShell
		s.Location = "run in a terminal"
		if o.WSL {
			s.Location = "run in a WSL shell"
			s.Snippet = WSLSetupCommand("claude", o.CLIPath, o.DBPath) + "\n"
		} else {
			s.Snippet = shellCommand(o.GOOS, "claude mcp add --scope user casebook --", s.Command, s.Args)
			s.Note = "casebook setup claude writes this for you, and --hooks adds the session hooks."
		}
	case "codex":
		if o.WSL {
			s.Format = FormatShell
			s.Location = "run in a WSL shell"
			s.Snippet = WSLSetupCommand("codex", o.CLIPath, o.DBPath) + "\n"
		} else {
			s.Format = FormatTOML
			s.Location = ResolvePaths(loc.getenv, o.Home).CodexConfig
			s.Snippet = codexBlock(s.Command, CodexOptions{DBPath: o.DBPath}, "\n")
			s.Note = "casebook setup codex writes this block for you."
		}
	}
	return s, nil
}

// locator spells config locations for MCPSnippetFor.
type locator struct {
	o      MCPSnippetOptions
	getenv Getenv
}

func newLocator(o MCPSnippetOptions) locator {
	g := o.Getenv
	if g == nil {
		g = func(string) string { return "" }
	}
	return locator{o: o, getenv: g}
}

// path is a file under the home directory: resolved on this system, or
// spelled with ~ for a client inside WSL (whose home this program cannot
// see).
func (l locator) path(elem ...string) string {
	if l.o.WSL || l.o.Home == "" {
		return "~/" + strings.Join(elem, "/")
	}
	return filepath.Join(append([]string{l.o.Home}, elem...)...)
}

// claudeDesktop is claude_desktop_config.json for the system.
func (l locator) claudeDesktop() string {
	switch l.o.GOOS {
	case "windows":
		if l.o.WSL {
			return "Claude Desktop does not run inside WSL; run casebook setup mcp --client claude-desktop without --wsl"
		}
		if appData := l.getenv("APPDATA"); appData != "" {
			return filepath.Join(appData, "Claude", "claude_desktop_config.json")
		}
		return `%APPDATA%\Claude\claude_desktop_config.json`
	case "darwin":
		return l.path("Library", "Application Support", "Claude", "claude_desktop_config.json")
	default:
		return l.path(".config", "Claude", "claude_desktop_config.json")
	}
}

// mcpServersJSON is {"mcpServers": {"casebook": {"command", "args"}}},
// indented.
func mcpServersJSON(command string, args []string) string {
	return encodeIndented(map[string]any{"mcpServers": map[string]mcpServersEntry{
		"casebook": {Command: command, Args: args}}})
}

// encodeIndented encodes v with two-space indentation, no HTML escaping and
// a trailing newline. Map keys are sorted, so the output is deterministic.
func encodeIndented(v any) string {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		// Strings and string slices always encode.
		panic("integrations: encode MCP snippet: " + err.Error())
	}
	return b.String()
}

// shellCommand is prefix followed by the quoted command and args, for a
// POSIX shell, or for PowerShell on Windows.
func shellCommand(goos, prefix, command string, args []string) string {
	quote := QuotePOSIX
	if goos == "windows" {
		quote = quotePowerShell
	}
	parts := []string{prefix, quote(command)}
	for _, a := range args {
		parts = append(parts, quote(a))
	}
	return strings.Join(parts, " ") + "\n"
}

// quotePowerShell quotes s as one PowerShell argument: unchanged when every
// byte is an ASCII letter, a digit or one of _ . / \ : + -, else a
// single-quoted string with every character PowerShell reads as a single
// quote doubled (the rule the desktop panel's commands use).
func quotePowerShell(s string) string {
	plain := s != ""
	for i := 0; i < len(s) && plain; i++ {
		c := s[i]
		plain = c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.IndexByte(`_./\:+-`, c) >= 0
	}
	if plain {
		return s
	}
	return "'" + psSingleQuote.Replace(s) + "'"
}

// psSingleQuote doubles every character PowerShell reads as a single quote.
var psSingleQuote = strings.NewReplacer(
	"'", "''",
	"‘", "‘‘",
	"’", "’’",
	"‚", "‚‚",
	"‛", "‛‛",
)
