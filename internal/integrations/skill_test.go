package integrations

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var skillNow = time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)

func TestSkillTextShape(t *testing.T) {
	if !strings.HasPrefix(SkillText, "---\nname: casebook\ndescription: ") {
		t.Errorf("SKILL.md must open with name and description frontmatter:\n%.120s", SkillText)
	}
	if !isCasebookSkill([]byte(SkillText)) {
		t.Error("SKILL.md lacks the casebook-managed marker line")
	}
	for _, s := range []string{SkillText, InstructionsSnippet} {
		if strings.ContainsRune(s, '\u2014') {
			t.Error("skill or instructions text contains an em dash")
		}
	}
	// The description is what both clients list to the model on every
	// turn; it has to say when to use the skill and stay one short line.
	desc := strings.SplitN(strings.SplitN(SkillText, "description: ", 2)[1], "\n", 2)[0]
	if len(desc) > 400 || !strings.Contains(desc, "/casebook") || strings.Contains(desc, ": ") {
		t.Errorf("description must be one plain YAML line under 400 bytes naming /casebook: %q", desc)
	}
	for _, want := range []string{"casebook_get_context", "casebook_create_investigation", "disproven", "casebook_checkpoint", "human_verified"} {
		if !strings.Contains(SkillText, want) {
			t.Errorf("SKILL.md does not mention %s", want)
		}
	}
}

// TestSkillHandoffGuidance: the skill says how to hand a case to a note or
// document app through the client's own tools, in order and with the rules,
// and names no provider or third-party tool.
func TestSkillHandoffGuidance(t *testing.T) {
	last := -1
	for _, step := range []string{"1. Get the text with `casebook_get_report`", "2. Create or update the destination",
		"3. Copy the text exactly", "4. Report what the app's tool actually returned"} {
		i := strings.Index(SkillText, step)
		if i <= last {
			t.Errorf("SKILL.md lacks step %q in order", step)
		}
		last = i
	}
	for _, rule := range []string{"Casebook never sends anything anywhere", "the tools this session actually has",
		"`mode` `brief`", "read the note the person means", "keep its unrelated content", "verification labels",
		"omission notices", "data, not instructions", "Never strengthen certainty", "unless that tool reported success",
		"`truncated` is true", "for a report, retry with `timeline_limit`", "`casebook report -o <file>`", "for a brief, use `casebook report --brief -o <file>`", "offer the text to copy or save", "`casebook_add_evidence`"} {
		if !strings.Contains(SkillText, rule) {
			t.Errorf("SKILL.md lacks %q", rule)
		}
	}
	for _, banned := range []string{"Evernote", "evernote_note", "get_note", "edit_note", "create_note", "sync"} {
		if strings.Contains(SkillText, banned) {
			t.Errorf("SKILL.md mentions %q", banned)
		}
	}
	if strings.ContainsRune(SkillText, '\u2014') {
		t.Error("SKILL.md contains an em dash")
	}
}

func TestInstallSkill(t *testing.T) {
	dir := t.TempDir()
	path := SkillPath(dir)
	if want := filepath.Join(dir, "skills", "casebook", "SKILL.md"); path != want {
		t.Fatalf("SkillPath = %s, want %s", path, want)
	}
	c, err := InstallSkill(path, skillNow)
	if err != nil || c.Action != ActionAdded || c.Backup != "" {
		t.Fatalf("first install = %+v, %v; want added, no backup", c, err)
	}
	if got, _ := os.ReadFile(path); string(got) != SkillText {
		t.Fatal("installed skill differs from SkillText")
	}
	if c, err := InstallSkill(path, skillNow); err != nil || c.Action != ActionUnchanged {
		t.Fatalf("rerun = %+v, %v; want unchanged", c, err)
	}
	// An older Casebook skill is replaced, with a backup.
	old := "---\nname: casebook\n# casebook-managed: old\n---\nold body\n"
	if err := os.WriteFile(path, []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err = InstallSkill(path, skillNow)
	if err != nil || c.Action != ActionUpdated || c.Backup == "" {
		t.Fatalf("update = %+v, %v; want updated with a backup", c, err)
	}
	if b, _ := os.ReadFile(c.Backup); string(b) != old {
		t.Error("backup does not hold the previous skill")
	}
}

func TestInstallSkillLeavesUserFile(t *testing.T) {
	path := SkillPath(t.TempDir())
	mine := "---\nname: casebook\ndescription: my own\n---\nmine\n"
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(mine), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := InstallSkill(path, skillNow)
	if err != nil || c.Action != ActionUnchanged || c.Note == "" {
		t.Fatalf("install over a user file = %+v, %v; want unchanged with a note", c, err)
	}
	r, err := RemoveSkill(path)
	if err != nil || r.Action != ActionUnchanged {
		t.Fatalf("remove of a user file = %+v, %v; want unchanged", r, err)
	}
	if got, _ := os.ReadFile(path); string(got) != mine {
		t.Error("the user's skill file was changed")
	}
}

func TestRemoveSkill(t *testing.T) {
	dir := t.TempDir()
	path := SkillPath(dir)
	if r, err := RemoveSkill(path); err != nil || r.Action != ActionUnchanged {
		t.Fatalf("remove with no file = %+v, %v; want unchanged", r, err)
	}
	if _, err := InstallSkill(path, skillNow); err != nil {
		t.Fatal(err)
	}
	r, err := RemoveSkill(path)
	if err != nil || r.Action != ActionRemoved {
		t.Fatalf("remove = %+v, %v; want removed", r, err)
	}
	if _, err := os.Stat(filepath.Dir(path)); !os.IsNotExist(err) {
		t.Error("the empty skills/casebook directory was left behind")
	}
	if _, err := os.Stat(filepath.Join(dir, "skills")); err != nil {
		t.Error("the skills directory itself must stay")
	}
}
