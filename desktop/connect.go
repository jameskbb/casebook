package main

import (
	"errors"

	"github.com/jameskbb/casebook/internal/integrations"
)

// Connect an AI tool without a terminal. ConnectClient does what the
// printed `casebook setup` line does, in this process: the same writers in
// internal/integrations, the same CLI path and options, the same --db. They
// read the settings file, refuse one they cannot parse or that changes
// underneath them, back it up, touch only Casebook's own entries and write
// nothing when nothing changes.

// The tools ConnectClient and DisconnectClient accept: the keys of
// ClientConnections.
const (
	toolClaude        = "claude"
	toolCodex         = "codex"
	toolClaudeDesktop = "claude_desktop"
)

// ClientChange is what ConnectClient or DisconnectClient did.
type ClientChange struct {
	Tool string `json:"tool"`
	// Connected is true after a connect, false after a disconnect.
	Connected bool `json:"connected"`
	// Changed is false when nothing had to be written.
	Changed bool `json:"changed"`
	// Message is one or two plain sentences for the person.
	Message string `json:"message"`
	// Files are the files looked at, in the order they were written. Never
	// nil.
	Files []ClientFile `json:"files"`
}

// ClientFile is one settings file of a ClientChange.
type ClientFile struct {
	File string `json:"file"`
	// Action is "added", "updated", "removed" or "unchanged".
	Action string `json:"action"`
	// Backup is the copy of the file as it was, "" when none was written.
	Backup string `json:"backup"`
}

// toolName is the tool as a person reads it in the middle of a sentence.
func toolName(tool string) string {
	switch tool {
	case toolClaude:
		return "Claude Code"
	case toolCodex:
		return "Codex"
	case toolClaudeDesktop:
		return "the Claude desktop app"
	}
	return ""
}

// ConnectClient adds Casebook to one AI tool's settings: Claude Code gets
// the MCP server, the session hooks and the /casebook skill (setup claude
// --hooks --skill), Codex the MCP server and the skill (setup codex
// --skill), the Claude desktop app the MCP server.
func (d *Desktop) ConnectClient(tool string) (ClientChange, error) {
	return d.changeClient(tool, true)
}

// DisconnectClient takes Casebook's own entries out of one AI tool's
// settings (setup --remove) and nothing else.
func (d *Desktop) DisconnectClient(tool string) (ClientChange, error) {
	return d.changeClient(tool, false)
}

func (d *Desktop) changeClient(tool string, connect bool) (ClientChange, error) {
	name := toolName(tool)
	if name == "" {
		return ClientChange{}, uiErr(kindValidation, "tool", "Casebook does not know this AI tool.")
	}
	e, err := d.clientEnv()
	if err != nil {
		return ClientChange{}, err
	}
	p := e.paths
	if connect {
		if e.cli == "" {
			return ClientChange{}, uiErr(kindNotFound, "cli", "Casebook cannot find its helper program, so it cannot connect an AI tool. Install Casebook again.")
		}
		found := map[string]bool{
			toolClaude:        e.found("claude", p.ClaudeConfigDir, p.ClaudeJSON),
			toolCodex:         e.found("codex", p.CodexHome),
			toolClaudeDesktop: e.desktopFound(),
		}[tool]
		if !found {
			return ClientChange{}, uiErr(kindNotFound, "tool", "Casebook could not find "+name+" on this computer.")
		}
	}

	// Each step returns the files it looked at; the first error stops the
	// run with the files already written still reported.
	var changes []integrations.FileChange
	one := func(c integrations.FileChange, err error) ([]integrations.FileChange, error) {
		if err != nil {
			return nil, err
		}
		return []integrations.FileChange{c}, nil
	}
	var steps []func() ([]integrations.FileChange, error)
	switch {
	case tool == toolClaude && connect:
		steps = append(steps,
			func() ([]integrations.FileChange, error) {
				return integrations.SetupClaude(e.cli, integrations.ClaudeOptions{Paths: p, DBPath: e.db, Hooks: true, GOOS: e.seams.goos, Now: d.now})
			},
			func() ([]integrations.FileChange, error) {
				return one(integrations.InstallSkill(integrations.SkillPath(p.ClaudeConfigDir), d.now()))
			})
	case tool == toolClaude:
		steps = append(steps,
			func() ([]integrations.FileChange, error) {
				return integrations.RemoveClaude(integrations.ClaudeOptions{Paths: p, Now: d.now})
			},
			func() ([]integrations.FileChange, error) {
				return one(integrations.RemoveSkill(integrations.SkillPath(p.ClaudeConfigDir)))
			})
	case tool == toolCodex && connect:
		steps = append(steps,
			func() ([]integrations.FileChange, error) {
				return one(integrations.SetupCodex(e.cli, integrations.CodexOptions{Paths: p, DBPath: e.db, Now: d.now}))
			},
			func() ([]integrations.FileChange, error) {
				return one(integrations.InstallSkill(integrations.SkillPath(p.CodexHome), d.now()))
			})
	case tool == toolCodex:
		steps = append(steps,
			func() ([]integrations.FileChange, error) {
				return one(integrations.RemoveCodex(integrations.CodexOptions{Paths: p, Now: d.now}))
			},
			func() ([]integrations.FileChange, error) {
				return one(integrations.RemoveSkill(integrations.SkillPath(p.CodexHome)))
			})
	case connect:
		steps = append(steps, func() ([]integrations.FileChange, error) {
			return one(integrations.SetupClaudeDesktop(e.cli, integrations.ClaudeDesktopOptions{Config: e.desktopConfig, DBPath: e.db, Now: d.now}))
		})
	default:
		steps = append(steps, func() ([]integrations.FileChange, error) {
			return one(integrations.RemoveClaudeDesktop(integrations.ClaudeDesktopOptions{Config: e.desktopConfig, Now: d.now}))
		})
	}
	for _, step := range steps {
		cs, err := step()
		changes = append(changes, cs...)
		if err != nil {
			return ClientChange{}, clientErr(name, changes, err)
		}
	}

	out := ClientChange{Tool: tool, Connected: connect, Files: []ClientFile{}}
	backup := false
	for _, c := range changes {
		out.Files = append(out.Files, ClientFile{File: c.File, Action: string(c.Action), Backup: c.Backup})
		out.Changed = out.Changed || c.Action != integrations.ActionUnchanged
		backup = backup || c.Backup != ""
	}
	kept := ""
	if backup {
		kept = " and kept a backup of the file"
	}
	switch {
	case connect && !out.Changed:
		out.Message = upperFirst(name) + " is already connected. Nothing was changed."
	case connect:
		out.Message = "Casebook added itself to " + name + "'s settings" + kept + ". Close " + name + " and open it again to start."
	case !out.Changed:
		out.Message = upperFirst(name) + " was not connected. Nothing was changed."
	default:
		out.Message = "Casebook took itself out of " + name + "'s settings" + kept + "."
	}
	return out, nil
}

// upperFirst capitalises a leading ASCII letter ("the Claude desktop app"
// at the start of a sentence).
func upperFirst(s string) string {
	if s != "" && s[0] >= 'a' && s[0] <= 'z' {
		return string(s[0]-'a'+'A') + s[1:]
	}
	return s
}

// clientErr is the error a person reads when a connect or disconnect stops.
// done are the files handled before it stopped: a backup among them is named,
// so a half-finished change can be put back.
func clientErr(name string, done []integrations.FileChange, err error) error {
	var refused *integrations.RefusedError
	switch {
	case errors.As(err, &refused) && !written(done):
		return uiErr(kindConflict, "settings", "Casebook could not read "+name+"'s settings safely, so it changed nothing. Use the steps under Technical details instead.")
	case errors.Is(err, integrations.ErrChangedUnderneath) && !written(done):
		return uiErr(kindConflict, "changed", upperFirst(name)+"'s settings changed while Casebook was updating them. Nothing was changed. Try again.")
	}
	msg := "Casebook could not finish changing " + name + "'s settings. Try again."
	for _, c := range done {
		if c.Backup != "" {
			msg += " A backup was kept at " + c.Backup + "."
			break
		}
	}
	return uiErr(kindOther, "", msg)
}

// written reports whether any of the files was changed.
func written(done []integrations.FileChange) bool {
	for _, c := range done {
		if c.Action != integrations.ActionUnchanged {
			return true
		}
	}
	return false
}
