package integrations

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	toml "github.com/pelletier/go-toml/v2"
)

// WR-04: setup updates only the members it owns inside Casebook-owned
// entries and keeps what the user added there.

func TestSetupClaudeKeepsUserMembers(t *testing.T) {
	orig := `{
  "mcpServers": {
    "casebook": {
      "command": "/old/casebook",
      "type": "stdio",
      "args": [
        "mcp"
      ],
      "env": {
        "CASEBOOK_DB": "/work/case.db"
      },
      "timeout": 30000
    }
  }
}
`
	opts, path := claudeFixture(t, []byte(orig))
	changes, err := SetupClaude(testCLI, opts)
	if err != nil {
		t.Fatal(err)
	}
	c := oneChange(t, changes, path, ActionUpdated)
	if c.Note != "kept your env, timeout in mcpServers.casebook" {
		t.Errorf("note = %q", c.Note)
	}
	// Only the command changed, in place; member order is kept.
	want := strings.Replace(orig, `"/old/casebook"`, `"`+testCLI+`"`, 1)
	if got := string(readBytes(t, path)); got != want {
		t.Fatalf("file; %s", firstDiff([]byte(got), []byte(want)))
	}

	// A rerun changes nothing and still names the kept members.
	changes, err = SetupClaude(testCLI, opts)
	if err != nil {
		t.Fatal(err)
	}
	if c := oneChange(t, changes, path, ActionUnchanged); c.Backup != "" || !strings.Contains(c.Note, "env") {
		t.Errorf("rerun = %+v", c)
	}

	// --db updates args and still keeps env and timeout.
	opts.DBPath = "/data/other.db"
	if _, err := SetupClaude(testCLI, opts); err != nil {
		t.Fatal(err)
	}
	var doc struct {
		MCPServers map[string]struct {
			Args    []string          `json:"args"`
			Env     map[string]string `json:"env"`
			Timeout int               `json:"timeout"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(readBytes(t, path), &doc); err != nil {
		t.Fatal(err)
	}
	cb := doc.MCPServers["casebook"]
	if strings.Join(cb.Args, " ") != "mcp --db /data/other.db" || cb.Env["CASEBOOK_DB"] != "/work/case.db" || cb.Timeout != 30000 {
		t.Errorf("casebook entry = %+v", cb)
	}
}

func TestSetupClaudeAddsMissingManagedMembers(t *testing.T) {
	// An entry missing type keeps its members and gains type at the end.
	orig := `{"mcpServers": {"casebook": {"command": "` + testCLI + `", "args": ["mcp"]}}}`
	opts, path := claudeFixture(t, []byte(orig))
	changes, err := SetupClaude(testCLI, opts)
	if err != nil {
		t.Fatal(err)
	}
	if c := oneChange(t, changes, path, ActionUpdated); c.Note != "" {
		t.Errorf("note = %q, want none", c.Note)
	}
	o, err := parseJSONObject(readBytes(t, path))
	if err != nil {
		t.Fatal(err)
	}
	servers, _, _ := o.childObject("mcpServers")
	entry, _, _ := servers.childObject("casebook")
	var keys []string
	for _, m := range entry.Members {
		keys = append(keys, m.Key)
	}
	if strings.Join(keys, ",") != "command,args,type" {
		t.Errorf("keys = %v", keys)
	}
}

func TestInstallHooksKeepsUserTuning(t *testing.T) {
	doc := `{"hooks": {"SessionStart": [{"matcher": "startup", "hooks": [` +
		`{"type": "command", "command": "/old/casebook hook session-start", "timeout": 30, "statusMessage": "loading casebook"}]}],` +
		` "Stop": [{"matcher": "x", "hooks": [{"type": "command", "command": "/old/casebook hook checkpoint"}]}]}}`
	opts, _, path := hooksFixture(t, []byte(doc))
	changes, err := SetupClaude(testCLI, opts)
	if err != nil {
		t.Fatal(err)
	}
	if c := settingsChange(t, changes, path); c.Action != ActionUpdated {
		t.Errorf("change = %+v", c)
	}
	var got struct {
		Hooks map[string][]json.RawMessage `json:"hooks"`
	}
	if err := json.Unmarshal(readBytes(t, path), &got); err != nil {
		t.Fatal(err)
	}
	start := hookCommand(testCLI, "", "session-start")
	stop := hookCommand(testCLI, "", "checkpoint")
	for event, want := range map[string]string{
		// The user's matcher, timeout and extra handler member stay.
		"SessionStart": `{"matcher":"startup","hooks":[{"type":"command","command":` + string(mustMarshal(start)) +
			`,"timeout":30,"statusMessage":"loading casebook"}]}`,
		// A matcher the user added stays; a missing timeout gets the default.
		"Stop": `{"matcher":"x","hooks":[{"type":"command","command":` + string(mustMarshal(stop)) + `,"timeout":10}]}`,
		// A new event gets the desired group.
		"PreCompact": string(desiredHookGroup(casebookHookSpecs[2], testCLI, opts)),
	} {
		if g := got.Hooks[event]; len(g) != 1 || !sameJSON(g[0], json.RawMessage(want)) {
			t.Errorf("hooks.%s = %s, want [%s]", event, g, want)
		}
	}
	// A rerun changes nothing.
	changes, err = SetupClaude(testCLI, opts)
	if err != nil {
		t.Fatal(err)
	}
	if c := settingsChange(t, changes, path); c.Action != ActionUnchanged || c.Backup != "" {
		t.Errorf("rerun = %+v", c)
	}
}

func TestSetupCodexKeepsBlockAdditions(t *testing.T) {
	for _, eol := range []string{"\n", "\r\n"} {
		block := strings.ReplaceAll(codexBlockBegin+"\n"+
			"[mcp_servers.casebook]\n"+
			"command = '/old/casebook'\n"+
			"startup_timeout_sec = 30\n"+
			"args = ['mcp']\n"+
			"# my note\n"+
			"\n"+
			"[mcp_servers.casebook.env]\n"+
			"CASEBOOK_LOG_LEVEL = 'debug'\n"+
			codexBlockEnd+"\n", "\n", eol)
		orig := []byte("model = 'o3'" + eol + eol + block + eol + "[profiles.x]" + eol + "model = 'o4'" + eol)
		opts, path := codexFixture(t, orig)
		opts.DBPath = "/data/case.db"
		c, err := SetupCodex(testCLI, opts)
		if err != nil {
			t.Fatal(err)
		}
		wantCodexChange(t, c, path, ActionUpdated, true)
		got := readBytes(t, path)
		var doc struct {
			Model      string `toml:"model"`
			MCPServers map[string]struct {
				Command           string            `toml:"command"`
				Args              []string          `toml:"args"`
				StartupTimeoutSec int               `toml:"startup_timeout_sec"`
				Env               map[string]string `toml:"env"`
			} `toml:"mcp_servers"`
			Profiles map[string]map[string]any `toml:"profiles"`
		}
		if err := toml.Unmarshal(got, &doc); err != nil {
			t.Fatalf("result is not TOML: %v\n%s", err, got)
		}
		cb := doc.MCPServers["casebook"]
		if cb.Command != testCLI || strings.Join(cb.Args, " ") != "mcp --db /data/case.db" ||
			cb.StartupTimeoutSec != 30 || cb.Env["CASEBOOK_LOG_LEVEL"] != "debug" {
			t.Errorf("casebook = %+v", cb)
		}
		if doc.Model != "o3" || doc.Profiles["x"]["model"] != "o4" {
			t.Errorf("outside values changed: %+v", doc)
		}
		if !bytes.Contains(got, []byte("# my note"+eol)) {
			t.Errorf("the comment inside the block was dropped:\n%q", got)
		}
		if bytes.Count(got, []byte("command")) != 1 || bytes.Count(got, []byte("args")) != 1 {
			t.Errorf("command or args duplicated:\n%q", got)
		}
		// Everything outside the block is byte for byte the same.
		span, err := findManagedBlock(got)
		if err != nil || !span.Found {
			t.Fatalf("block markers: %v", err)
		}
		origSpan, _ := findManagedBlock(orig)
		if string(got[:span.Start]) != string(orig[:origSpan.Start]) || string(got[span.End:]) != string(orig[origSpan.End:]) {
			t.Errorf("bytes outside the block changed:\n%q", got)
		}
		// A rerun changes nothing.
		c, err = SetupCodex(testCLI, opts)
		if err != nil {
			t.Fatal(err)
		}
		wantCodexChange(t, c, path, ActionUnchanged, false)
		// Remove takes the whole block, additions included.
		if _, err := RemoveCodex(opts); err != nil {
			t.Fatal(err)
		}
		if after := string(readBytes(t, path)); strings.Contains(after, "casebook") || strings.Contains(after, "CASEBOOK_LOG_LEVEL") {
			t.Errorf("remove left block content behind:\n%q", after)
		}
	}
}

func TestSetupCodexRefusesUncarriableBlock(t *testing.T) {
	// A multi-line args array cannot be carried over line by line: refuse
	// rather than drop or mangle anything.
	orig := []byte(codexBlockBegin + "\n[mcp_servers.casebook]\ncommand = '/old/casebook'\nargs = [\n  'mcp',\n]\n" +
		"startup_timeout_sec = 30\n" + codexBlockEnd + "\n")
	opts, path := codexFixture(t, orig)
	c, err := SetupCodex(testCLI, opts)
	if c.File != "" {
		t.Errorf("change = %+v on refusal", c)
	}
	wantCodexRefused(t, err, path, orig, blockExtrasReason)
}
