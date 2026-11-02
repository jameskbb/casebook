package integrations

import (
	_ "embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SkillText is the Casebook agent skill (SKILL.md) that `setup claude
// --skill` and `setup codex --skill` install. Claude Code offers it as the
// /casebook command and both clients list its description to the model, so
// it can start or resume a case when the person agrees or asks.
//
//go:embed skill/SKILL.md
var SkillText string

// skillMarker marks a SKILL.md as Casebook's. A file without it belongs to
// the person and is never changed or removed (ADR-0009).
const skillMarker = "# casebook-managed:"

// skillEntry names the skill in FileChange.Entries.
const skillEntry = "skills/casebook"

// SkillPath is the skill file under a client's config directory: Claude
// Code's ClaudeConfigDir or Codex's CodexHome.
func SkillPath(configDir string) string {
	return filepath.Join(configDir, "skills", "casebook", "SKILL.md")
}

// isCasebookSkill reports whether data is a skill file Casebook wrote.
func isCasebookSkill(data []byte) bool {
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), skillMarker) {
			return true
		}
	}
	return false
}

// InstallSkill writes SkillText to path, the conservative way every setup
// writer works: a missing file is created (its directories 0700), a file
// Casebook wrote earlier is replaced after a backup when it differs, and a
// file without the Casebook marker (the person's own, or one they took over
// by deleting the marker line) is left alone with a note.
func InstallSkill(path string, now time.Time) (FileChange, error) {
	change := FileChange{File: path, Action: ActionUnchanged, Entries: []string{skillEntry}}
	f, err := readConfigFile(path)
	if err != nil {
		return change, err
	}
	change.File = f.Path
	if f.Exists && !isCasebookSkill(f.Data) {
		change.Note = "not written by Casebook (no casebook-managed line); left unchanged"
		return change, nil
	}
	backup, changed, err := f.commit([]byte(SkillText), now)
	if err != nil {
		return change, err
	}
	if changed {
		change.Backup = backup
		change.Action = ActionUpdated
		if !f.Exists {
			change.Action = ActionAdded
		}
	}
	return change, nil
}

// RemoveSkill deletes the skill file at path when Casebook wrote it, then
// its casebook directory when that is left empty. A missing file, or one
// without the Casebook marker, is left alone.
func RemoveSkill(path string) (FileChange, error) {
	change := FileChange{File: path, Action: ActionUnchanged, Entries: []string{skillEntry}}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return change, nil
	}
	if err != nil {
		return change, fmt.Errorf("read %s: %w", path, err)
	}
	if !isCasebookSkill(data) {
		change.Note = "not written by Casebook (no casebook-managed line); left unchanged"
		return change, nil
	}
	if err := os.Remove(path); err != nil {
		return change, fmt.Errorf("remove %s: %w", path, err)
	}
	// Only an empty directory is removed; anything else the person put
	// there stays.
	_ = os.Remove(filepath.Dir(path))
	change.Action = ActionRemoved
	return change, nil
}

// SkillPreview is the --print text for the skill at path.
func SkillPreview(path string) string {
	return "Casebook skill: write to " + path + "\n" + SkillText
}

// InstructionsSnippet is what `casebook setup instructions` prints: a short
// section for a project's CLAUDE.md or AGENTS.md. Setup never writes it
// into those files (ADR-0009); the person pastes it where they want it.
const InstructionsSnippet = `## Casebook

This project keeps investigation memory in Casebook (the casebook_* MCP tools).

- When a question needs more than one look (why a number moved, which of several explanations holds, a failure whose cause is not obvious), or I say we will continue it later, offer to open a Casebook case, and open it once I agree.
- When I refer to earlier investigation work, read it with casebook_get_context before exploring, and do not re-test hypotheses it marks disproven.
- While a case is open, record evidence with its source (path:line or the command), hypotheses (mark the ones you rule out disproven), open questions and the conclusion, and call casebook_checkpoint before your final reply.
`
