package integrations

import (
	"path/filepath"
	"time"
)

// The Claude desktop app keeps its MCP servers in the "mcpServers" object of
// claude_desktop_config.json, the shape Claude Code's user config uses, so
// the same reader, merge and commit apply: strict JSON or a refusal, only
// mcpServers.casebook touched, a backup first, nothing written when nothing
// changes.

// claudeDesktopManagedKeys are the members of mcpServers.casebook that setup
// owns in the Claude desktop app's config. Every other member (env and so
// on) belongs to the user and is kept as it is.
var claudeDesktopManagedKeys = []string{"command", "args"}

// ClaudeDesktopOptions configures the Claude desktop app setup functions.
type ClaudeDesktopOptions struct {
	// Config is claude_desktop_config.json (from ClaudeDesktopConfig).
	Config string
	// DBPath, when set, is an absolute database path; the server's args
	// become ["mcp", "--db", DBPath].
	DBPath string
	// Now is the backup timestamp clock; nil means time.Now.
	Now func() time.Time
}

func (o ClaudeDesktopOptions) now() time.Time {
	if o.Now != nil {
		return o.Now()
	}
	return time.Now()
}

// ClaudeDesktopConfig is the Claude desktop app's config file for the
// system: %APPDATA%\Claude on Windows, ~/Library/Application Support/Claude
// on macOS, ~/.config/Claude elsewhere. It is "" when the folder it lives in
// cannot be named (Windows without APPDATA, or no home directory).
func ClaudeDesktopConfig(goos string, getenv Getenv, home string) string {
	const name = "claude_desktop_config.json"
	switch goos {
	case "windows":
		if appData := getenv("APPDATA"); appData != "" {
			return filepath.Join(appData, "Claude", name)
		}
		return ""
	case "darwin":
		if home != "" {
			return filepath.Join(home, "Library", "Application Support", "Claude", name)
		}
	default:
		if home != "" {
			return filepath.Join(home, ".config", "Claude", name)
		}
	}
	return ""
}

// claudeDesktopServer is the entry `casebook setup mcp --client
// claude-desktop` prints: {"command":<cliPath>,"args":["mcp"]}.
func claudeDesktopServer(cliPath, dbPath string) []byte {
	raw, err := marshalNoEscape(mcpServersEntry{Command: cliPath, Args: mcpArgs(dbPath)})
	if err != nil {
		// A string and a string slice always encode.
		panic("integrations: encode Claude desktop MCP server: " + err.Error())
	}
	return raw
}

// SetupClaudeDesktop registers the Casebook MCP server in the Claude desktop
// app's config as mcpServers.casebook, under the rules of SetupClaude: a
// file that is not one strict JSON object is refused, an existing casebook
// entry keeps every member but command and args, a backup of the previous
// file is written first, and a run that changes nothing writes nothing.
func SetupClaudeDesktop(cliPath string, opts ClaudeDesktopOptions) (FileChange, error) {
	d, err := readClaudeDoc(opts.Config)
	if err != nil {
		return FileChange{}, err
	}
	return setupMCP(d, claudeDesktopServer(cliPath, opts.DBPath), claudeDesktopManagedKeys, opts.now())
}

// RemoveClaudeDesktop deletes mcpServers.casebook from the Claude desktop
// app's config and nothing else; the mcpServers object stays, even empty. A
// missing file, or one without the entry, reports unchanged and is left
// alone.
func RemoveClaudeDesktop(opts ClaudeDesktopOptions) (FileChange, error) {
	d, err := readClaudeDoc(opts.Config)
	if err != nil {
		return FileChange{}, err
	}
	change := FileChange{File: d.file.Path, Action: ActionUnchanged, Entries: []string{claudeServerEntry}}
	if d.servers != nil && d.servers.Delete(claudeServerName) {
		backup, changed, err := d.save(opts.now())
		if err != nil {
			return FileChange{}, err
		}
		if changed {
			change.Action = ActionRemoved
			change.Backup = backup
		}
	}
	return change, nil
}

// InspectClaudeDesktop reads (never writes) the Claude desktop app's config
// and reports Casebook's registration, like InspectClaude without hooks.
func InspectClaudeDesktop(config, cliPath string) ClaudeStatus {
	return InspectClaude(Paths{ClaudeJSON: config}, cliPath)
}
