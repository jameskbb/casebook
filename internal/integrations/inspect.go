package integrations

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// ClaudeStatus is a read-only view of the Claude Code config for
// diagnostics (casebook doctor). It never carries env values, args or any
// other config content: only paths, booleans and hook event names.
type ClaudeStatus struct {
	ConfigPath   string
	ConfigExists bool
	// ConfigValid is true when the file is a JSON object whose mcpServers
	// (when present) is an object.
	ConfigValid bool
	// Registered is true when mcpServers.casebook exists.
	Registered bool
	// Command is mcpServers.casebook.command ("" when not registered or not
	// a string).
	Command           string
	CommandExists     bool
	CommandExecutable bool
	// MatchesCLI is true when Command is the given CLI after symlink
	// resolution.
	MatchesCLI     bool
	SettingsPath   string
	SettingsExists bool
	SettingsValid  bool
	// Hooks are the events holding a Casebook handler for their hook, in
	// install order (SessionStart, Stop, PreCompact); never nil.
	Hooks []string
}

// CodexStatus is a read-only view of the Codex config for diagnostics. Like
// ClaudeStatus it carries no env values, args or config content.
type CodexStatus struct {
	ConfigPath   string
	ConfigExists bool
	ConfigValid  bool
	// Registered is true when mcp_servers.casebook exists.
	Registered bool
	// Managed is true when that table is inside the Casebook-managed block.
	Managed           bool
	Command           string
	CommandExists     bool
	CommandExecutable bool
	MatchesCLI        bool
	// ForeignInBlock is true when the managed block holds tables that are
	// not Casebook's, such as a trust entry Codex appended at the end of
	// the file; casebook setup codex moves them out.
	ForeignInBlock bool
}

// InspectClaude reads (never writes) the Claude Code user config and
// settings at paths and reports Casebook's registration and hooks. cliPath
// is the running CLI ("" never matches).
func InspectClaude(paths Paths, cliPath string) ClaudeStatus {
	st := ClaudeStatus{ConfigPath: paths.ClaudeJSON, SettingsPath: paths.ClaudeSettings, Hooks: []string{}}
	if data, exists := inspectRead(paths.ClaudeJSON); exists {
		st.ConfigExists = true
		if root, err := parseJSONObject(data); err == nil {
			servers, present, err := root.childObject(claudeServersKey)
			if err == nil {
				st.ConfigValid = true
				if present {
					if raw, ok := servers.Get(claudeServerName); ok {
						st.Registered = true
						st.Command = jsonCommand(raw)
					}
				}
			}
		}
	}
	if data, exists := inspectRead(paths.ClaudeSettings); exists {
		st.SettingsExists = true
		if root, err := parseJSONObject(data); err == nil {
			if hooks, err := hooksObject(root); err == nil {
				st.SettingsValid = true
				st.Hooks = installedHooks(hooks)
			}
		}
	}
	st.CommandExists, st.CommandExecutable, st.MatchesCLI = inspectCommand(st.Command, cliPath)
	return st
}

// InspectCodex reads (never writes) the Codex config at paths and reports
// Casebook's registration and whether it is in the managed block.
func InspectCodex(paths Paths, cliPath string) CodexStatus {
	st := CodexStatus{ConfigPath: paths.CodexConfig}
	data, exists := inspectRead(paths.CodexConfig)
	if !exists {
		return st
	}
	st.ConfigExists = true
	server, found, err := codexCasebook(data)
	if err != nil {
		return st
	}
	st.ConfigValid = true
	st.Registered = found
	if cmd, ok := server["command"].(string); ok {
		st.Command = cmd
	}
	if span, err := findManagedBlock(data); err == nil && span.Found {
		if _, inBlock, err := codexCasebook(data[span.Start:span.End]); err == nil && inBlock {
			st.Managed = true
		}
		st.ForeignInBlock = foreignInBlock(data)
	}
	st.CommandExists, st.CommandExecutable, st.MatchesCLI = inspectCommand(st.Command, cliPath)
	return st
}

// inspectRead reads path. exists is false only when nothing is there; an
// unreadable file exists but yields no data (and so does not parse).
func inspectRead(path string) (data []byte, exists bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, !errors.Is(err, os.ErrNotExist)
	}
	return data, true
}

// jsonCommand is the string command member of a server entry, or "".
func jsonCommand(raw json.RawMessage) string {
	var entry struct {
		Command json.RawMessage `json:"command"`
	}
	if err := json.Unmarshal(raw, &entry); err != nil || entry.Command == nil {
		return ""
	}
	var cmd string
	if err := json.Unmarshal(entry.Command, &cmd); err != nil {
		return ""
	}
	return cmd
}

// installedHooks lists the Casebook events whose array holds a Casebook
// handler running that event's hook, in casebookHookSpecs order. Mixed
// groups (user handlers plus a Casebook one) count.
func installedHooks(hooks *jsonObject) []string {
	out := []string{}
	for _, spec := range casebookHookSpecs {
		raw, ok := hooks.Get(spec.Event)
		if !ok {
			continue
		}
		items, err := decodeRawArray(raw)
		if err != nil {
			continue
		}
		for _, it := range items {
			if _, hasSub := classifyGroup(it, spec.Sub); hasSub {
				out = append(out, spec.Event)
				break
			}
		}
	}
	return out
}

// inspectCommand checks a configured command: whether it exists as a
// regular file (a bare name is looked up on PATH, as the client would),
// whether it is executable (any execute bit; any regular file on windows)
// and whether it is cliPath after symlink resolution.
func inspectCommand(command, cliPath string) (exists, executable, matches bool) {
	if command == "" {
		return false, false, false
	}
	path := command
	if filepath.Base(path) == path {
		if p, err := exec.LookPath(path); err == nil {
			path = p
		}
	}
	fi, err := os.Stat(path)
	if err != nil || !fi.Mode().IsRegular() {
		return false, false, false
	}
	executable = runtime.GOOS == "windows" || fi.Mode().Perm()&0o111 != 0
	if cliPath != "" {
		matches = resolved(path) == resolved(cliPath)
	}
	return true, executable, matches
}

// resolved is p with symlinks resolved and made absolute, falling back to
// p itself.
func resolved(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		p = r
	}
	if abs, err := filepath.Abs(p); err == nil {
		p = abs
	}
	return filepath.Clean(p)
}
