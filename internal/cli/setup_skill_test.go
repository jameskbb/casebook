package cli

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/jameskbb/casebook/internal/integrations"
)

func TestSetupClaudeSkill(t *testing.T) {
	d := isolateClients(t, true)
	exe := fakeCLIPath(t)
	skill := integrations.SkillPath(d.ClaudeDir)

	r := runClientCLI(t, d, exe, "", "setup", "claude", "--print", "--skill")
	if r.Code != 0 || !strings.Contains(r.Stdout, "Casebook skill: write to "+skill) || !strings.Contains(r.Stdout, "name: casebook") {
		t.Fatalf("--print --skill = %d %q", r.Code, r.Stdout)
	}
	if _, err := os.Stat(skill); !os.IsNotExist(err) {
		t.Fatal("--print wrote the skill")
	}

	r = runClientCLI(t, d, exe, "", "setup", "claude", "--skill")
	if r.Code != 0 || !strings.Contains(r.Stdout, skill+": added skills/casebook") {
		t.Fatalf("setup claude --skill = %d %q %q", r.Code, r.Stdout, r.Stderr)
	}
	if got, _ := os.ReadFile(skill); string(got) != integrations.SkillText {
		t.Fatal("skill file differs from SkillText")
	}

	if r := runClientCLI(t, d, exe, "", "setup", "claude", "--remove", "--skill"); r.Code != 2 {
		t.Errorf("--remove --skill exit = %d, want 2 (usage)", r.Code)
	}
	r = runClientCLI(t, d, exe, "", "setup", "claude", "--remove")
	if r.Code != 0 || !strings.Contains(r.Stdout, skill+": removed skills/casebook") {
		t.Fatalf("setup claude --remove = %d %q %q", r.Code, r.Stdout, r.Stderr)
	}
	if _, err := os.Stat(skill); !os.IsNotExist(err) {
		t.Fatal("--remove left the skill")
	}
	// Without a skill, --remove reports only the config files, as before.
	r = runClientCLI(t, d, exe, "", "setup", "claude", "--remove")
	if r.Code != 0 || strings.Contains(r.Stdout, "skills/casebook") {
		t.Fatalf("second --remove = %d %q", r.Code, r.Stdout)
	}
}

func TestSetupCodexSkill(t *testing.T) {
	d := isolateClients(t, true)
	exe := fakeCLIPath(t)
	skill := integrations.SkillPath(d.CodexHome)

	r := runClientCLI(t, d, exe, "", "--json", "setup", "codex", "--skill")
	if r.Code != 0 {
		t.Fatalf("setup codex --skill = %d %q", r.Code, r.Stderr)
	}
	var rep setupReport
	if err := json.Unmarshal([]byte(r.Stdout), &rep); err != nil {
		t.Fatal(err)
	}
	if len(rep.Changes) != 2 || rep.Changes[1].File != skill || rep.Changes[1].Action != integrations.ActionAdded {
		t.Fatalf("changes = %+v; want the config then the added skill", rep.Changes)
	}
	r = runClientCLI(t, d, exe, "", "setup", "codex", "--remove")
	if r.Code != 0 || !strings.Contains(r.Stdout, skill+": removed skills/casebook") {
		t.Fatalf("setup codex --remove = %d %q %q", r.Code, r.Stdout, r.Stderr)
	}
}

func TestSetupInstructions(t *testing.T) {
	d := isolateClients(t, true)
	r := runClientCLI(t, d, fakeCLIPath(t), "", "setup", "instructions")
	if r.Code != 0 || r.Stdout != integrations.InstructionsSnippet {
		t.Fatalf("setup instructions = %d %q", r.Code, r.Stdout)
	}
	r = runClientCLI(t, d, fakeCLIPath(t), "", "--json", "setup", "instructions")
	var got map[string]string
	if r.Code != 0 || json.Unmarshal([]byte(r.Stdout), &got) != nil || got["text"] != integrations.InstructionsSnippet {
		t.Fatalf("setup instructions --json = %d %q", r.Code, r.Stdout)
	}
	// It writes nothing anywhere.
	for _, dir := range []string{d.Home, d.ClaudeDir, d.CodexHome} {
		if entries, _ := os.ReadDir(dir); len(entries) != 0 {
			t.Errorf("setup instructions wrote into %s", dir)
		}
	}
}
