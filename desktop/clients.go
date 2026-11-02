package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/jameskbb/casebook/internal/integrations"
)

// The Connect AI clients panel (INT-10, D-09). It is read-only: it shows the
// bundled CLI, whether Claude Code and Codex already run Casebook, and the
// exact configuration and command for each. The desktop app never writes a
// client config itself; `casebook setup claude` and `casebook setup codex`
// own the backups, the merge and the refusals (ADR-0009), so the panel hands
// the user that one command to run.

// CLI sources reported to the page.
const (
	cliBundled = "bundled" // <dir of Casebook>/bin/casebook[.exe] (ADR-0012)
	cliNearby  = "nearby"  // next to the desktop binary (development builds)
	cliOnPath  = "path"    // found on PATH (development builds)
	cliMissing = "missing" // not found; the page explains how to get it
)

// ClientConnections is everything the Connect AI clients panel shows.
type ClientConnections struct {
	// CLIPath is the absolute path of the casebook CLI, or "" when missing.
	CLIPath string `json:"cli_path"`
	// CLISource is "bundled", "nearby", "path" or "missing".
	CLISource string `json:"cli_source"`
	// CLIName is the path the snippets use: CLIPath, else the bare command.
	CLIName string `json:"cli_name"`
	// DBPath is the database the snippets point at with --db, "" when the
	// app uses the per-user default the CLI uses too.
	DBPath string `json:"db_path"`
	// Shell is the shell the setup commands are written for: "PowerShell"
	// on Windows, "terminal" elsewhere (AR-12).
	Shell  string           `json:"shell"`
	Claude ClientConnection `json:"claude"`
	Codex  ClientConnection `json:"codex"`
}

// ClientConnection is one AI client's status and setup instructions.
type ClientConnection struct {
	// Configured is true when the client's config registers Casebook.
	Configured bool `json:"configured"`
	// MatchesCLI is true when that registration runs CLIPath.
	MatchesCLI bool `json:"matches_cli"`
	// Command is the registered command ("" when not configured).
	Command string `json:"command"`
	// CommandExists is false when the registered command is not on disk.
	CommandExists bool `json:"command_exists"`
	// ConfigPath is the client config file Casebook reads.
	ConfigPath string `json:"config_path"`
	// ConfigProblem is set when the file exists but cannot be read as the
	// client's config format.
	ConfigProblem bool `json:"config_problem"`
	// Hooks are the Claude Code events holding a Casebook hook (never nil).
	Hooks []string `json:"hooks"`
	// SetupCommand is the one-line command that writes the config safely,
	// for the shell named by ClientConnections.Shell.
	SetupCommand string `json:"setup_command"`
	// SetupCommandCmd is the same command for Command Prompt (cmd.exe) on
	// Windows, "" elsewhere or when it reads exactly like SetupCommand.
	SetupCommandCmd string `json:"setup_command_cmd"`
	// SetupCommandWSL registers the Windows CLI with the copy of the client
	// installed inside WSL, for a WSL shell (bash), on Windows only. It is ""
	// elsewhere and when the CLI path has no /mnt/<drive> form.
	SetupCommandWSL string `json:"setup_command_wsl"`
	// Snippet is the configuration to merge by hand (setup --print output).
	Snippet string `json:"snippet"`
}

// clientSeams are the environment lookups ClientConnections uses; tests
// replace them, production uses the process environment.
type clientSeams struct {
	getenv  func(string) string
	home    func() (string, error)
	cliPath func() (string, error)
	goos    string
}

func defaultClientSeams() clientSeams {
	return clientSeams{getenv: os.Getenv, home: os.UserHomeDir, cliPath: integrations.CLIPath, goos: runtime.GOOS}
}

// ClientConnections reads (never writes) the Claude Code and Codex configs
// and returns the panel's content. It needs no open database.
func (d *Desktop) ClientConnections() (ClientConnections, error) {
	s := defaultClientSeams()
	if d.clients != nil {
		s = *d.clients
	}
	home, err := s.home()
	if err != nil {
		return ClientConnections{}, uiErr(kindOther, "", "Casebook cannot find your home folder, so it cannot read AI client settings")
	}
	p := integrations.ResolvePaths(s.getenv, home)

	out := ClientConnections{CLISource: cliMissing}
	if cli, err := s.cliPath(); err == nil && cli != "" {
		out.CLIPath = cli
		out.CLISource = cliSource(cli, s.goos)
	}
	out.CLIName = out.CLIPath
	if out.CLIName == "" {
		out.CLIName = "casebook"
	}
	if d.explicitDB {
		out.DBPath = d.dbPath
	}
	out.Shell = "terminal"
	if s.goos == "windows" {
		out.Shell = "PowerShell"
	}

	cs := integrations.InspectClaude(p, out.CLIPath)
	out.Claude = ClientConnection{
		Configured:      cs.Registered,
		MatchesCLI:      cs.MatchesCLI,
		Command:         cs.Command,
		CommandExists:   cs.CommandExists,
		ConfigPath:      cs.ConfigPath,
		ConfigProblem:   cs.ConfigExists && !cs.ConfigValid,
		Hooks:           cs.Hooks,
		SetupCommand:    setupCommand(s.goos, out.CLIName, out.DBPath, "claude"),
		SetupCommandCmd: cmdSetupCommand(s.goos, out.CLIName, out.DBPath, "claude"),
		SetupCommandWSL: wslSetupCommand(s.goos, out.CLIPath, out.DBPath, "claude"),
		Snippet:         integrations.ClaudeConfigPreview(out.CLIName, integrations.ClaudeOptions{Paths: p, DBPath: out.DBPath}),
	}
	xs := integrations.InspectCodex(p, out.CLIPath)
	out.Codex = ClientConnection{
		Configured:      xs.Registered,
		MatchesCLI:      xs.MatchesCLI,
		Command:         xs.Command,
		CommandExists:   xs.CommandExists,
		ConfigPath:      xs.ConfigPath,
		ConfigProblem:   xs.ConfigExists && !xs.ConfigValid,
		Hooks:           []string{},
		SetupCommand:    setupCommand(s.goos, out.CLIName, out.DBPath, "codex"),
		SetupCommandCmd: cmdSetupCommand(s.goos, out.CLIName, out.DBPath, "codex"),
		SetupCommandWSL: wslSetupCommand(s.goos, out.CLIPath, out.DBPath, "codex"),
		Snippet:         integrations.CodexConfigPreview(out.CLIName, integrations.CodexOptions{Paths: p, DBPath: out.DBPath}),
	}
	return out, nil
}

// cliSource classifies where the CLI was found relative to this program.
func cliSource(cli, goos string) string {
	exe, err := os.Executable()
	if err != nil {
		return cliOnPath
	}
	if r, err := filepath.EvalSymlinks(exe); err == nil {
		exe = r
	}
	dir := filepath.Dir(exe)
	name := "casebook"
	if goos == "windows" {
		name += ".exe"
	}
	switch filepath.Clean(cli) {
	case filepath.Join(dir, "bin", name):
		return cliBundled
	case filepath.Join(dir, name):
		return cliNearby
	}
	return cliOnPath
}

// setupCommand is the one-line `casebook setup <client>` command (with the
// setupArgs flags), with --db when the app uses a non-default database. Arguments are quoted for
// PowerShell on Windows and for a POSIX shell elsewhere, only when needed.
func setupCommand(goos, cli, db, client string) string {
	parts := []string{quoteArg(goos, cli)}
	if db != "" {
		parts = append(parts, "--db", quoteArg(goos, db))
	}
	parts = append(parts, setupArgs(client)...)
	cmd := strings.Join(parts, " ")
	if goos == "windows" && strings.HasPrefix(parts[0], "'") {
		// PowerShell runs a quoted program path only through the call
		// operator.
		cmd = "& " + cmd
	}
	return cmd
}

// setupArgs is the `setup <client>` part of a setup command with the flags
// the panel recommends: Claude Code gets the session hooks and the
// /casebook skill, Codex the skill.
func setupArgs(client string) []string {
	switch client {
	case "claude":
		return []string{"setup", "claude", "--hooks", "--skill"}
	case "codex":
		return []string{"setup", "codex", "--skill"}
	}
	return []string{"setup", client}
}

// quoteArg returns s unchanged when every byte is a letter, a digit or one
// of the punctuation marks the target shell never interprets, and otherwise
// wraps it in single quotes. Both PowerShell and POSIX shells treat
// single-quoted text literally: $, backticks, % and backslashes are not
// expanded. An embedded quote is doubled (PowerShell) or closed and escaped
// (POSIX; integrations.QuotePOSIX, the quoting the hooks use).
// PowerShell also ends a single-quoted string at the typographic
// quotes U+2018 to U+201B, so those are doubled as well; a Windows user
// name such as O’Brien may contain one.
func quoteArg(goos, s string) string {
	if goos == "windows" {
		if s != "" && plainWord(s, `_./\:+-`) {
			return s
		}
		return "'" + psSingleQuote.Replace(s) + "'"
	}
	return integrations.QuotePOSIX(s)
}

// psSingleQuote doubles every character PowerShell reads as a single quote.
var psSingleQuote = strings.NewReplacer(
	"'", "''",
	"\u2018", "\u2018\u2018",
	"\u2019", "\u2019\u2019",
	"\u201a", "\u201a\u201a",
	"\u201b", "\u201b\u201b",
)

// plainWord reports whether every byte of s is an ASCII letter, a digit or
// one of extra.
func plainWord(s, extra string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		case strings.IndexByte(extra, c) >= 0:
		default:
			return false
		}
	}
	return true
}

// cmdSetupCommand is setupCommand for Command Prompt on Windows: arguments
// that need it are wrapped in double quotes (Windows paths cannot contain
// one). It is "" off Windows and when the PowerShell form is the same text.
// It is also "" when a path holds % (or a double quote): cmd.exe expands
// %NAME% even inside double quotes and has no escape for it on an
// interactive command line, so the panel offers only the PowerShell form.
func cmdSetupCommand(goos, cli, db, client string) string {
	if goos != "windows" || strings.ContainsAny(cli+db, `%"`) {
		return ""
	}
	parts := []string{quoteCmd(cli)}
	if db != "" {
		parts = append(parts, "--db", quoteCmd(db))
	}
	parts = append(parts, setupArgs(client)...)
	cmd := strings.Join(parts, " ")
	if cmd == setupCommand(goos, cli, db, client) {
		return ""
	}
	return cmd
}

// wslSetupCommand is the command that registers the Windows CLI with the
// client installed inside WSL (integrations.WSLSetupCommand). It is "" off
// Windows and when the CLI was not found.
func wslSetupCommand(goos, cli, db, client string) string {
	if goos != "windows" {
		return ""
	}
	return integrations.WSLSetupCommand(client, cli, db)
}

// quoteCmd wraps s in double quotes when cmd.exe would split or interpret
// it. Inside double quotes cmd.exe treats & | < > ^ ( ) as text; callers
// keep % and " out (cmdSetupCommand).
func quoteCmd(s string) string {
	if s != "" && !strings.ContainsAny(s, " \t&|<>^()%!,;=\"") {
		return s
	}
	return `"` + s + `"`
}
