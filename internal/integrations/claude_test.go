package integrations

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// testCLI is the casebook path written by tests that never run it.
const testCLI = "/opt/Case Book/bin/casebook"

// claudeFixture writes data (nil means no file) as .claude.json in a fresh
// CLAUDE_CONFIG_DIR and returns options pointing at it with the fixed clock,
// plus the file's resolved path.
func claudeFixture(t *testing.T, data []byte) (ClaudeOptions, string) {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	getenv := func(k string) string {
		if k == "CLAUDE_CONFIG_DIR" {
			return dir
		}
		return ""
	}
	opts := ClaudeOptions{Paths: ResolvePaths(getenv, t.TempDir()), Now: fixedClock}
	if data != nil {
		if err := os.WriteFile(opts.Paths.ClaudeJSON, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return opts, opts.Paths.ClaudeJSON
}

// readBytes reads a file or fails the test.
func readBytes(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// topKeys returns a document's top-level keys in order with their compact
// raw values.
func topKeys(t *testing.T, data []byte) ([]string, map[string]string) {
	t.Helper()
	o, err := parseJSONObject(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	var keys []string
	vals := make(map[string]string)
	for _, m := range o.Members {
		keys = append(keys, m.Key)
		var c bytes.Buffer
		if err := json.Compact(&c, m.Value); err != nil {
			t.Fatal(err)
		}
		vals[m.Key] = c.String()
	}
	return keys, vals
}

// casebookMember is the casebook member of mcpServers as encoded at depth
// two, for testCLI with the given args.
func casebookMember(args ...string) string {
	var a []string
	for _, s := range args {
		q, _ := json.Marshal(s)
		a = append(a, "        "+string(q))
	}
	return "    \"casebook\": {\n      \"type\": \"stdio\",\n      \"command\": \"" + testCLI + "\",\n" +
		"      \"args\": [\n" + strings.Join(a, ",\n") + "\n      ],\n      \"env\": {}\n    }"
}

// oneChange checks a run reported exactly one change of path with action.
func oneChange(t *testing.T, changes []FileChange, path string, action Action) FileChange {
	t.Helper()
	if len(changes) != 1 {
		t.Fatalf("changes = %+v, want one", changes)
	}
	c := changes[0]
	if c.File != path || c.Action != action || strings.Join(c.Entries, ",") != "mcpServers.casebook" {
		t.Fatalf("change = %+v, want %s %s mcpServers.casebook", c, path, action)
	}
	return c
}

func TestClaudeMCPServer(t *testing.T) {
	got := string(ClaudeMCPServer(testCLI, ClaudeOptions{}))
	if want := `{"type":"stdio","command":"` + testCLI + `","args":["mcp"],"env":{}}`; got != want {
		t.Errorf("entry = %s, want %s", got, want)
	}
	got = string(ClaudeMCPServer(`C:\Casebook\bin\casebook.exe`, ClaudeOptions{DBPath: "/data/a&b.db"}))
	if want := `{"type":"stdio","command":"C:\\Casebook\\bin\\casebook.exe","args":["mcp","--db","/data/a&b.db"],"env":{}}`; got != want {
		t.Errorf("entry = %s, want %s", got, want)
	}
}

func TestSetupClaudeTypical(t *testing.T) {
	orig := readFixture(t, "typical.json")
	opts, path := claudeFixture(t, orig)
	changes, err := SetupClaude(testCLI, opts)
	if err != nil {
		t.Fatal(err)
	}
	c := oneChange(t, changes, path, ActionAdded)
	got := readBytes(t, path)

	// Independent check: the original text with only the casebook member
	// inserted after the last mcpServers member.
	end := bytes.Index(orig, []byte("\n  },\n  \"projects\""))
	if end < 0 {
		t.Fatal("fixture layout changed")
	}
	want := string(orig[:end]) + ",\n" + casebookMember("mcp") + string(orig[end:])
	if string(got) != want {
		t.Fatalf("new bytes differ from the original plus the casebook member; %s", firstDiff(got, []byte(want)))
	}

	// The same expectation built through the jsonObject API.
	o, _ := parseJSONObject(orig)
	srv, _, _ := o.childObject("mcpServers")
	srv.Set("casebook", ClaudeMCPServer(testCLI, opts))
	enc, _ := srv.Encode()
	o.Set("mcpServers", bytes.TrimRight(enc, "\n"))
	if api, _ := o.Encode(); !bytes.Equal(api, got) {
		t.Fatalf("jsonObject expectation differs; %s", firstDiff(got, api))
	}

	// Every original top-level value except mcpServers is unchanged, and so
	// is every other server.
	keys, before := topKeys(t, orig)
	gotKeys, after := topKeys(t, got)
	if strings.Join(keys, ",") != strings.Join(gotKeys, ",") {
		t.Errorf("top-level keys %v became %v", keys, gotKeys)
	}
	for _, k := range keys {
		if k != "mcpServers" && before[k] != after[k] {
			t.Errorf("%s changed from %s to %s", k, before[k], after[k])
		}
	}
	_, sb := topKeys(t, []byte(before["mcpServers"]))
	_, sa := topKeys(t, []byte(after["mcpServers"]))
	if sb["other"] != sa["other"] {
		t.Errorf("mcpServers.other changed from %s to %s", sb["other"], sa["other"])
	}

	if c.Backup == "" || !bytes.Equal(readBytes(t, c.Backup), orig) {
		t.Errorf("backup %q does not hold the original bytes", c.Backup)
	}
	wantMode(t, c.Backup, 0o600)
}

func TestSetupClaudeIdempotent(t *testing.T) {
	opts, path := claudeFixture(t, readFixture(t, "typical.json"))
	if _, err := SetupClaude(testCLI, opts); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Hour).Truncate(time.Second)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	first := readBytes(t, path)
	changes, err := SetupClaude(testCLI, opts)
	if err != nil {
		t.Fatal(err)
	}
	c := oneChange(t, changes, path, ActionUnchanged)
	if c.Backup != "" {
		t.Errorf("backup = %s, want none", c.Backup)
	}
	if !bytes.Equal(readBytes(t, path), first) {
		t.Error("second run changed the file")
	}
	if fi, _ := os.Stat(path); !fi.ModTime().Equal(old) {
		t.Errorf("second run touched the file (mtime %v)", fi.ModTime())
	}
	if l := leftovers(t, filepath.Dir(path)); len(l) != 1 {
		t.Errorf("leftovers = %v, want exactly the first run's backup", l)
	}
}

func TestSetupClaudeKeepsFormattingWhenRegistered(t *testing.T) {
	// A compact file that already holds the exact entry is not reformatted.
	orig := []byte(`{"a":1,"mcpServers":{"casebook":{"type":"stdio","command":"` + testCLI + `","args":["mcp"],"env":{}}}}`)
	opts, path := claudeFixture(t, orig)
	changes, err := SetupClaude(testCLI, opts)
	if err != nil {
		t.Fatal(err)
	}
	oneChange(t, changes, path, ActionUnchanged)
	if !bytes.Equal(readBytes(t, path), orig) {
		t.Error("an already registered file was rewritten")
	}
}

func TestSetupClaudeStale(t *testing.T) {
	orig := readFixture(t, "stale-casebook.json")
	opts, path := claudeFixture(t, orig)
	opts.DBPath = "/data/casebook.db"
	changes, err := SetupClaude(testCLI, opts)
	if err != nil {
		t.Fatal(err)
	}
	c := oneChange(t, changes, path, ActionUpdated)
	if c.Backup == "" {
		t.Error("no backup for an update")
	}
	got := readBytes(t, path)
	start := bytes.Index(orig, []byte("    \"casebook\": {"))
	end := bytes.Index(orig, []byte(",\n    \"zeta\""))
	want := string(orig[:start]) + casebookMember("mcp", "--db", "/data/casebook.db") + string(orig[end:])
	if string(got) != want {
		t.Fatalf("stale entry not replaced in place; %s", firstDiff(got, []byte(want)))
	}
}

func TestSetupClaudeNoMCP(t *testing.T) {
	orig := readFixture(t, "no-mcp.json")
	opts, path := claudeFixture(t, orig)
	changes, err := SetupClaude(testCLI, opts)
	if err != nil {
		t.Fatal(err)
	}
	oneChange(t, changes, path, ActionAdded)
	got := readBytes(t, path)
	want := string(orig[:len(orig)-3]) + ",\n  \"mcpServers\": {\n" + casebookMember("mcp") + "\n  }\n}\n"
	if !bytes.HasSuffix(orig, []byte("\"\n}\n")) || string(got) != want {
		t.Fatalf("mcpServers not appended last; %s", firstDiff(got, []byte(want)))
	}
}

func TestSetupClaudeMissingFile(t *testing.T) {
	opts, _ := claudeFixture(t, nil)
	dir := filepath.Join(opts.Paths.ClaudeConfigDir, "nested", "config")
	opts.Paths = ResolvePaths(func(k string) string {
		if k == "CLAUDE_CONFIG_DIR" {
			return dir
		}
		return ""
	}, t.TempDir())
	path := opts.Paths.ClaudeJSON
	changes, err := SetupClaude(testCLI, opts)
	if err != nil {
		t.Fatal(err)
	}
	c := oneChange(t, changes, path, ActionAdded)
	if c.Backup != "" {
		t.Errorf("backup = %s for a new file", c.Backup)
	}
	want := "{\n  \"mcpServers\": {\n" + casebookMember("mcp") + "\n  }\n}\n"
	if got := readBytes(t, path); string(got) != want {
		t.Fatalf("new file = %q, want %q", got, want)
	}
	wantMode(t, path, 0o600)
	wantMode(t, dir, 0o700)
	wantMode(t, filepath.Dir(dir), 0o700)
}

func TestSetupClaudeKeepsMode(t *testing.T) {
	opts, path := claudeFixture(t, readFixture(t, "typical.json"))
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	changes, err := SetupClaude(testCLI, opts)
	if err != nil {
		t.Fatal(err)
	}
	wantMode(t, path, 0o644)
	wantMode(t, changes[0].Backup, 0o600)
}

func TestSetupClaudeBackupCollision(t *testing.T) {
	opts, path := claudeFixture(t, readFixture(t, "typical.json"))
	var got []string
	for _, cli := range []string{"/a/casebook", "/b/casebook"} {
		changes, err := SetupClaude(cli, opts)
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, changes[0].Backup)
	}
	base := path + ".casebook-backup-20261002T130405Z"
	if got[0] != base || got[1] != base+"-2" {
		t.Fatalf("backups = %v, want %s then -2", got, base)
	}
}

// claudeMalformed are documents SetupClaude and RemoveClaude must refuse,
// with the reason each gives.
var claudeMalformed = []struct {
	name, doc, reason string
}{
	{"invalid JSON", `{"numStartups": 3,,}`, "not valid JSON (syntax error at byte "},
	{"top-level array", `[{"mcpServers": {}}]`, "top-level value is not a JSON object"},
	{"trailing data", "{\"a\": 1}\ngarbage", "unexpected data after the JSON object"},
	{"byte order mark", "\xEF\xBB\xBF{\"a\": 1}", "starts with a byte order mark"},
	{"duplicate top-level key", `{"mcpServers": {}, "mcpServers": {}}`, "duplicate key at byte 19"},
	{"duplicate server", `{"mcpServers": {"casebook": {}, "casebook": {}}}`, "mcpServers: duplicate key at byte 32"},
	{"mcpServers is an array", `{"mcpServers": [{"casebook": {}}]}`, "mcpServers is not a JSON object"},
	{"mcpServers is null", `{"mcpServers": null}`, "mcpServers is not a JSON object"},
	{"casebook is a string", `{"mcpServers": {"casebook": "/usr/bin/casebook mcp"}}`, "mcpServers.casebook is not a JSON object"},
}

// wantRefused checks err is a RefusedError for path with reason, and that
// the file and its directory are untouched.
func wantRefused(t *testing.T, err error, path, orig, reason string) {
	t.Helper()
	var re *RefusedError
	if !errors.As(err, &re) {
		t.Fatalf("err = %v, want a RefusedError", err)
	}
	msg := err.Error()
	if !strings.HasPrefix(msg, "refusing to change "+path+": ") || !strings.HasSuffix(msg, "; nothing was written") {
		t.Errorf("message = %q", msg)
	}
	if !strings.Contains(re.Reason, reason) {
		t.Errorf("reason = %q, want it to contain %q", re.Reason, reason)
	}
	if strings.Contains(msg, "garbage") || strings.Contains(msg, "/usr/bin/casebook mcp") {
		t.Errorf("message %q quotes the file", msg)
	}
	if got := string(readBytes(t, path)); got != orig {
		t.Errorf("file changed to %q", got)
	}
	if l := leftovers(t, filepath.Dir(path)); len(l) != 0 {
		t.Errorf("leftovers = %v", l)
	}
}

func TestSetupClaudeRefuses(t *testing.T) {
	for _, c := range claudeMalformed {
		t.Run(c.name, func(t *testing.T) {
			opts, path := claudeFixture(t, []byte(c.doc))
			changes, err := SetupClaude(testCLI, opts)
			if changes != nil {
				t.Errorf("changes = %+v on refusal", changes)
			}
			wantRefused(t, err, path, c.doc, c.reason)
		})
	}
}

func TestSetupClaudeSymlink(t *testing.T) {
	orig := readFixture(t, "typical.json")
	targetDir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(targetDir, "claude.json")
	if err := os.WriteFile(target, orig, 0o600); err != nil {
		t.Fatal(err)
	}
	opts, link := claudeFixture(t, nil)
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("cannot create symlinks here: %v", err)
	}
	changes, err := SetupClaude(testCLI, opts)
	if err != nil {
		t.Fatal(err)
	}
	c := oneChange(t, changes, target, ActionAdded)
	if fi, err := os.Lstat(link); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("%s is no longer a symlink", link)
	}
	if !bytes.Contains(readBytes(t, target), []byte(`"casebook"`)) {
		t.Error("target did not gain the entry")
	}
	if filepath.Dir(c.Backup) != targetDir || !bytes.Equal(readBytes(t, c.Backup), orig) {
		t.Errorf("backup %s is not the original next to the target", c.Backup)
	}
	if l := leftovers(t, filepath.Dir(link)); len(l) != 0 {
		t.Errorf("leftovers next to the link: %v", l)
	}
}

func TestSetupClaudeDanglingSymlink(t *testing.T) {
	opts, link := claudeFixture(t, nil)
	if err := os.Symlink(filepath.Join(t.TempDir(), "missing.json"), link); err != nil {
		t.Skipf("cannot create symlinks here: %v", err)
	}
	_, err := SetupClaude(testCLI, opts)
	var re *RefusedError
	if !errors.As(err, &re) {
		t.Fatalf("err = %v, want a RefusedError", err)
	}
}

func TestSetupClaudeChangedUnderneath(t *testing.T) {
	orig := readFixture(t, "typical.json")
	opts, path := claudeFixture(t, orig)
	concurrent := []byte("{\n  \"numStartups\": 43\n}\n")
	beforeRename = func(p string) {
		if err := os.WriteFile(p, concurrent, 0o600); err != nil {
			t.Error(err)
		}
	}
	t.Cleanup(func() { beforeRename = nil })
	_, err := SetupClaude(testCLI, opts)
	if err == nil || !strings.Contains(err.Error(), "changed while Casebook was updating it") {
		t.Fatalf("err = %v, want the changed-underneath abort", err)
	}
	if !bytes.Equal(readBytes(t, path), concurrent) {
		t.Error("the concurrent content was not kept")
	}
	if l := leftovers(t, filepath.Dir(path)); len(l) != 0 {
		t.Errorf("leftovers = %v", l)
	}
}

// removeChange checks a RemoveClaude run reported .claude.json (path) with
// action first and an unchanged settings.json second (no hooks in these
// fixtures), and returns the .claude.json change.
func removeChange(t *testing.T, changes []FileChange, path string, action Action) FileChange {
	t.Helper()
	if len(changes) != 2 {
		t.Fatalf("changes = %+v, want .claude.json and settings.json", changes)
	}
	if s := changes[1]; filepath.Base(s.File) != "settings.json" || s.Action != ActionUnchanged || s.Backup != "" {
		t.Fatalf("settings change = %+v, want unchanged", s)
	}
	return oneChange(t, changes[:1], path, action)
}

func TestRemoveClaude(t *testing.T) {
	t.Run("removes only casebook", func(t *testing.T) {
		orig := readFixture(t, "stale-casebook.json")
		opts, path := claudeFixture(t, orig)
		changes, err := RemoveClaude(opts)
		if err != nil {
			t.Fatal(err)
		}
		c := removeChange(t, changes, path, ActionRemoved)
		if c.Backup == "" || !bytes.Equal(readBytes(t, c.Backup), orig) {
			t.Errorf("backup %q does not hold the original bytes", c.Backup)
		}
		start := bytes.Index(orig, []byte(",\n    \"casebook\": {"))
		end := bytes.Index(orig, []byte(",\n    \"zeta\""))
		want := string(orig[:start]) + string(orig[end:])
		if got := readBytes(t, path); string(got) != want {
			t.Fatalf("remove touched more than mcpServers.casebook; %s", firstDiff(got, []byte(want)))
		}
	})
	t.Run("install then remove restores the original", func(t *testing.T) {
		orig := readFixture(t, "typical.json")
		opts, path := claudeFixture(t, orig)
		if _, err := SetupClaude(testCLI, opts); err != nil {
			t.Fatal(err)
		}
		if _, err := RemoveClaude(opts); err != nil {
			t.Fatal(err)
		}
		if got := readBytes(t, path); !bytes.Equal(got, orig) {
			t.Fatalf("install then remove changed the file; %s", firstDiff(got, orig))
		}
	})
	t.Run("absent entry", func(t *testing.T) {
		orig := readFixture(t, "typical.json")
		opts, path := claudeFixture(t, orig)
		changes, err := RemoveClaude(opts)
		if err != nil {
			t.Fatal(err)
		}
		if c := removeChange(t, changes, path, ActionUnchanged); c.Backup != "" {
			t.Errorf("backup = %s", c.Backup)
		}
		if !bytes.Equal(readBytes(t, path), orig) {
			t.Error("file changed")
		}
		if l := leftovers(t, filepath.Dir(path)); len(l) != 0 {
			t.Errorf("leftovers = %v", l)
		}
	})
	t.Run("no mcpServers", func(t *testing.T) {
		orig := readFixture(t, "no-mcp.json")
		opts, path := claudeFixture(t, orig)
		changes, err := RemoveClaude(opts)
		if err != nil {
			t.Fatal(err)
		}
		removeChange(t, changes, path, ActionUnchanged)
		if !bytes.Equal(readBytes(t, path), orig) {
			t.Error("file changed")
		}
	})
	t.Run("missing file", func(t *testing.T) {
		opts, path := claudeFixture(t, nil)
		changes, err := RemoveClaude(opts)
		if err != nil {
			t.Fatal(err)
		}
		removeChange(t, changes, path, ActionUnchanged)
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Errorf("remove created %s (%v)", path, err)
		}
	})
	t.Run("only server leaves an empty mcpServers", func(t *testing.T) {
		orig := "{\n  \"a\": 1,\n  \"mcpServers\": {\n" + casebookMember("mcp") + "\n  },\n  \"b\": 2\n}\n"
		opts, path := claudeFixture(t, []byte(orig))
		changes, err := RemoveClaude(opts)
		if err != nil {
			t.Fatal(err)
		}
		removeChange(t, changes, path, ActionRemoved)
		want := "{\n  \"a\": 1,\n  \"mcpServers\": {},\n  \"b\": 2\n}\n"
		if got := readBytes(t, path); string(got) != want {
			t.Fatalf("file = %q, want %q", got, want)
		}
	})
	t.Run("install on a document without mcpServers then remove", func(t *testing.T) {
		orig := readFixture(t, "no-mcp.json")
		opts, path := claudeFixture(t, orig)
		if _, err := SetupClaude(testCLI, opts); err != nil {
			t.Fatal(err)
		}
		if _, err := RemoveClaude(opts); err != nil {
			t.Fatal(err)
		}
		// The empty container stays (D-06: never delete a container the
		// user may own), as `claude mcp remove` leaves it.
		want := string(orig[:len(orig)-3]) + ",\n  \"mcpServers\": {}\n}\n"
		if got := readBytes(t, path); string(got) != want {
			t.Fatalf("file = %q, want %q", got, want)
		}
	})
}

func TestRemoveClaudeRefuses(t *testing.T) {
	for _, c := range claudeMalformed {
		t.Run(c.name, func(t *testing.T) {
			opts, path := claudeFixture(t, []byte(c.doc))
			changes, err := RemoveClaude(opts)
			if changes != nil {
				t.Errorf("changes = %+v on refusal", changes)
			}
			wantRefused(t, err, path, c.doc, c.reason)
		})
	}
}

func TestClaudeConfigPreview(t *testing.T) {
	opts, path := claudeFixture(t, nil)
	got := ClaudeConfigPreview(testCLI, opts)
	want := `Claude Code MCP server (user scope): merge into the top-level "mcpServers" object of ` + path + `
{
  "mcpServers": {
    "casebook": {
      "type": "stdio",
      "command": "` + testCLI + `",
      "args": [
        "mcp"
      ],
      "env": {}
    }
  }
}
`
	if got != want {
		t.Errorf("preview:\n%s\nwant:\n%s", got, want)
	}
	opts.DBPath = "/data/a&b.db"
	got = ClaudeConfigPreview(testCLI, opts)
	want = strings.Replace(want, "        \"mcp\"\n", "        \"mcp\",\n        \"--db\",\n        \"/data/a&b.db\"\n", 1)
	if got != want {
		t.Errorf("preview with --db:\n%s\nwant:\n%s", got, want)
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Errorf("preview wrote %s", path)
	}
}
