// Package integrations connects AI clients (Claude Code and Codex) to
// Casebook: it renders and merges the client config that registers the
// Casebook MCP server and session hooks, and inspects that config for
// diagnostics.
//
// Every writer follows ADR-0009: read the file, merge only Casebook-owned
// keys, refuse to touch a file it cannot parse, write a timestamped backup
// before any change, and replace the file atomically. A run that would change
// nothing writes nothing.
package integrations

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Getenv reads an environment variable (os.Getenv in production; tests
// inject their own).
type Getenv func(string) string

// Paths are the client config files Casebook reads and writes (D-02).
// Relative environment values are made absolute.
type Paths struct {
	// ClaudeConfigDir is $CLAUDE_CONFIG_DIR, else <home>/.claude.
	ClaudeConfigDir string `json:"claude_config_dir"`
	// ClaudeJSON is $CLAUDE_CONFIG_DIR/.claude.json, else <home>/.claude.json
	// (user-scope MCP servers live in its top-level "mcpServers").
	ClaudeJSON string `json:"claude_json"`
	// ClaudeSettings is <ClaudeConfigDir>/settings.json (hooks).
	ClaudeSettings string `json:"claude_settings"`
	// CodexHome is $CODEX_HOME, else <home>/.codex.
	CodexHome string `json:"codex_home"`
	// CodexConfig is <CodexHome>/config.toml.
	CodexConfig string `json:"codex_config"`
}

// ResolvePaths computes every client config path from the environment and
// the user's home directory. CLAUDE_CONFIG_DIR, when set to a non-blank
// value, moves both .claude.json and settings.json into it (verified against
// Claude Code); CODEX_HOME moves config.toml. No tilde expansion is done.
func ResolvePaths(getenv Getenv, home string) Paths {
	var p Paths
	if dir := envDir(getenv, "CLAUDE_CONFIG_DIR"); dir != "" {
		p.ClaudeConfigDir = dir
		p.ClaudeJSON = filepath.Join(dir, ".claude.json")
	} else {
		p.ClaudeConfigDir = filepath.Join(home, ".claude")
		p.ClaudeJSON = filepath.Join(home, ".claude.json")
	}
	p.ClaudeSettings = filepath.Join(p.ClaudeConfigDir, "settings.json")
	if dir := envDir(getenv, "CODEX_HOME"); dir != "" {
		p.CodexHome = dir
	} else {
		p.CodexHome = filepath.Join(home, ".codex")
	}
	p.CodexConfig = filepath.Join(p.CodexHome, "config.toml")
	return p
}

// envDir returns the named variable made absolute, or "" when it is unset or
// blank.
func envDir(getenv Getenv, name string) string {
	v := getenv(name)
	if strings.TrimSpace(v) == "" {
		return ""
	}
	if abs, err := filepath.Abs(v); err == nil {
		return abs
	}
	return v
}

// CLIPath returns the absolute, symlink-resolved path of the casebook CLI
// for a caller that is not necessarily the CLI (the desktop app's "Connect
// AI clients" panel, D-09). The CLI itself writes its own path instead.
func CLIPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		exe = ""
	}
	return cliPathFrom(exe, runtime.GOOS, exec.LookPath, filepath.EvalSymlinks, os.Stat)
}

// cliPathFrom finds the casebook CLI for the program at exe (symlinks in
// exe are resolved first when possible). Candidates, in order:
//
//  1. exe itself when its base name is exactly casebook (casebook.exe on
//     windows); the comparison is case-sensitive, so the desktop binary
//     Casebook never matches;
//  2. <dir>/bin/casebook[.exe], the packaged layout (ADR-0012: Windows
//     <install>/Casebook.exe plus <install>/bin/casebook.exe, macOS
//     Casebook.app/Contents/MacOS/Casebook plus .../MacOS/bin/casebook);
//  3. <dir>/casebook[.exe];
//  4. lookPath("casebook[.exe]").
//
// Candidates 2 and 3 count only when they are regular files other than exe
// (on case-insensitive volumes casebook.exe next to Casebook.exe is the
// desktop binary itself). The result is symlink-resolved (unresolved when
// resolution fails) and absolute.
func cliPathFrom(exe, goos string, lookPath func(string) (string, error), evalSymlinks func(string) (string, error), stat func(string) (os.FileInfo, error)) (string, error) {
	name := "casebook"
	if goos == "windows" {
		name += ".exe"
	}
	finish := func(p string) (string, error) {
		if r, err := evalSymlinks(p); err == nil {
			p = r
		}
		return filepath.Abs(p)
	}
	if exe != "" {
		if r, err := evalSymlinks(exe); err == nil {
			exe = r
		}
		if filepath.Base(exe) == name {
			return finish(exe)
		}
		var exeInfo os.FileInfo
		if fi, err := stat(exe); err == nil {
			exeInfo = fi
		}
		dir := filepath.Dir(exe)
		for _, c := range []string{filepath.Join(dir, "bin", name), filepath.Join(dir, name)} {
			fi, err := stat(c)
			if err != nil || !fi.Mode().IsRegular() || (exeInfo != nil && os.SameFile(fi, exeInfo)) {
				continue
			}
			return finish(c)
		}
	}
	if p, err := lookPath(name); err == nil && p != "" {
		return finish(p)
	}
	if exe == "" {
		return "", errors.New("casebook CLI not found on PATH")
	}
	return "", fmt.Errorf("casebook CLI not found next to %s or on PATH", exe)
}
