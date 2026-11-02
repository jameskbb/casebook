package integrations

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	toml "github.com/pelletier/go-toml/v2"
)

// codexFixture writes data (nil means no file) as config.toml in a fresh
// CODEX_HOME and returns options pointing at it with the fixed clock, plus
// the file's path.
func codexFixture(t *testing.T, data []byte) (CodexOptions, string) {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	getenv := func(k string) string {
		if k == "CODEX_HOME" {
			return dir
		}
		return ""
	}
	opts := CodexOptions{Paths: ResolvePaths(getenv, t.TempDir()), Now: fixedClock}
	if data != nil {
		if err := os.WriteFile(opts.Paths.CodexConfig, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return opts, opts.Paths.CodexConfig
}

// readCodexFixture returns a testdata/codex file's bytes.
func readCodexFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "codex", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// wantCodexChange checks a change's file, action, entries and whether it
// reports a backup.
func wantCodexChange(t *testing.T, c FileChange, path string, action Action, backup bool) {
	t.Helper()
	if c.File != path || c.Action != action || strings.Join(c.Entries, ",") != codexServerEntry {
		t.Errorf("change = %+v, want %s %s %s", c, path, action, codexServerEntry)
	}
	if (c.Backup != "") != backup {
		t.Errorf("backup = %q, want backup %v", c.Backup, backup)
	}
}

// wantBackupOf checks the change's backup holds orig with mode 0600.
func wantBackupOf(t *testing.T, c FileChange, orig []byte) {
	t.Helper()
	if c.Backup == "" {
		t.Fatal("no backup reported")
	}
	if got := readBytes(t, c.Backup); string(got) != string(orig) {
		t.Errorf("backup = %q, want the previous bytes", got)
	}
	wantMode(t, c.Backup, 0o600)
}

func TestFindManagedBlock(t *testing.T) {
	block := codexBlock(testCLI, CodexOptions{}, "\n")
	crlf := codexBlock(testCLI, CodexOptions{}, "\r\n")
	noEOL := strings.TrimSuffix(block, "\n")
	cases := []struct {
		name       string
		data       string
		found      bool
		start, end int
		damaged    bool
	}{
		{"empty", "", false, 0, 0, false},
		{"no markers", "model = \"o3\"\n", false, 0, 0, false},
		{"block only", block, true, 0, len(block), false},
		{"block in the middle", "a = 1\n\n" + block + "b = 2\n", true, 7, 7 + len(block), false},
		{"crlf", "a = 1\r\n" + crlf + "b = 2\r\n", true, 7, 7 + len(crlf), false},
		{"end line without line ending", "a = 1\n" + noEOL, true, 6, 6 + len(noEOL), false},
		{"indented marker is not a marker", "  " + codexBlockBegin + "\n", false, 0, 0, false},
		{"marker with trailing text is not a marker", codexBlockEnd + " here\n", false, 0, 0, false},
		{"begin without end", "a = 1\n" + codexBlockBegin + "\n[mcp_servers.casebook]\n", false, 0, 0, true},
		{"two begin lines", codexBlockBegin + "\n" + block, false, 0, 0, true},
		{"two blocks", block + block, false, 0, 0, true},
		{"end before begin", codexBlockEnd + "\n" + block, false, 0, 0, true},
		{"end without begin", "a = 1\n" + codexBlockEnd + "\n", false, 0, 0, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			span, err := findManagedBlock([]byte(c.data))
			if c.damaged {
				if err == nil || err.Error() != damagedMarkersReason {
					t.Fatalf("err = %v, want %q", err, damagedMarkersReason)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			want := blockSpan{Start: c.start, End: c.end, Found: c.found}
			if span != want {
				t.Errorf("span = %+v, want %+v", span, want)
			}
		})
	}
}

func TestCodexBlock(t *testing.T) {
	want := codexBlockBegin + "\n" + codexBlockNote + "\n[mcp_servers.casebook]\ncommand = '/opt/casebook/bin/casebook'\nargs = ['mcp']\n" + codexBlockEnd + "\n"
	if got := codexBlock("/opt/casebook/bin/casebook", CodexOptions{}, "\n"); got != want {
		t.Errorf("block =\n%s\nwant\n%s", got, want)
	}
	// A path with a single quote needs a basic string; one with a newline
	// needs an escape. Both must read back exactly.
	for _, p := range []string{`/tmp/it's here/casebook`, "/tmp/line\nbreak/casebook", `C:\Users\x\casebook.exe`, `/tmp/"quoted"/casebook`} {
		b := codexBlock(p, CodexOptions{DBPath: p + ".db"}, "\n")
		server, found, err := codexCasebook([]byte(b))
		if err != nil {
			t.Fatalf("%q: %v", p, err)
		}
		if why := checkCodexServer(server, found, p, []string{"mcp", "--db", p + ".db"}, nil); why != "" {
			t.Errorf("%q: %s\n%s", p, why, b)
		}
		if strings.Count(b, "\n") != 6 {
			t.Errorf("%q: block is not six lines:\n%s", p, b)
		}
	}
}

func TestSetupCodexTypical(t *testing.T) {
	orig := readCodexFixture(t, "typical.toml")
	opts, path := codexFixture(t, orig)
	c, err := SetupCodex(testCLI, opts)
	if err != nil {
		t.Fatal(err)
	}
	wantCodexChange(t, c, path, ActionAdded, true)
	wantBackupOf(t, c, orig)
	got := readBytes(t, path)
	want := string(orig) + "\n" + codexBlock(testCLI, opts, "\n")
	if string(got) != want {
		t.Errorf("config.toml differs: %s", firstDiff(got, []byte(want)))
	}
	if string(got[:len(orig)]) != string(orig) {
		t.Errorf("the original bytes are not a prefix of the result")
	}
	wantMode(t, path, 0o600)

	var doc struct {
		Model    string `toml:"model"`
		Profiles map[string]map[string]any
		Servers  map[string]struct {
			Command string            `toml:"command"`
			Args    []string          `toml:"args"`
			Env     map[string]string `toml:"env"`
		} `toml:"mcp_servers"`
	}
	if err := toml.Unmarshal(got, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Model != "o3" || doc.Servers["other"].Env["OTHER_API_KEY"] != "placeholder-other-key" ||
		doc.Servers["casebook"].Command != testCLI || strings.Join(doc.Servers["casebook"].Args, ",") != "mcp" {
		t.Errorf("parsed result = %+v", doc)
	}

	// A rerun is unchanged and writes nothing, not even a backup.
	before := leftovers(t, filepath.Dir(path))
	c, err = SetupCodex(testCLI, opts)
	if err != nil {
		t.Fatal(err)
	}
	wantCodexChange(t, c, path, ActionUnchanged, false)
	if after := leftovers(t, filepath.Dir(path)); len(after) != len(before) {
		t.Errorf("rerun left %v (was %v)", after, before)
	}
	if string(readBytes(t, path)) != want {
		t.Errorf("rerun changed the file")
	}
}

func TestSetupCodexCRLF(t *testing.T) {
	orig := []byte(strings.ReplaceAll(string(readCodexFixture(t, "typical.toml")), "\n", "\r\n"))
	opts, path := codexFixture(t, orig)
	c, err := SetupCodex(testCLI, opts)
	if err != nil {
		t.Fatal(err)
	}
	wantCodexChange(t, c, path, ActionAdded, true)
	got := readBytes(t, path)
	want := string(orig) + "\r\n" + codexBlock(testCLI, opts, "\r\n")
	if string(got) != want {
		t.Errorf("config.toml differs: %s", firstDiff(got, []byte(want)))
	}
	if n := strings.Count(string(got), "\n"); n != strings.Count(string(got), "\r\n") {
		t.Errorf("result mixes line endings")
	}
	c, err = SetupCodex(testCLI, opts)
	if err != nil {
		t.Fatal(err)
	}
	wantCodexChange(t, c, path, ActionUnchanged, false)
}

func TestSetupCodexNoTrailingNewline(t *testing.T) {
	orig := []byte("model = \"o3\"\n# last line has no newline")
	opts, path := codexFixture(t, orig)
	if _, err := SetupCodex(testCLI, opts); err != nil {
		t.Fatal(err)
	}
	want := string(orig) + "\n\n" + codexBlock(testCLI, opts, "\n")
	if got := readBytes(t, path); string(got) != want {
		t.Errorf("config.toml = %q, want %q", got, want)
	}
}

func TestSetupCodexMissingFile(t *testing.T) {
	opts, _ := codexFixture(t, nil)
	home := filepath.Join(t.TempDir(), "nested", "codex")
	opts.Paths.CodexHome = home
	opts.Paths.CodexConfig = filepath.Join(home, "config.toml")
	c, err := SetupCodex(testCLI, opts)
	if err != nil {
		t.Fatal(err)
	}
	wantCodexChange(t, c, opts.Paths.CodexConfig, ActionAdded, false)
	if got := readBytes(t, opts.Paths.CodexConfig); string(got) != codexBlock(testCLI, opts, "\n") {
		t.Errorf("config.toml = %q, want exactly the block", got)
	}
	wantMode(t, opts.Paths.CodexConfig, 0o600)
	wantMode(t, home, 0o700)
	wantMode(t, filepath.Dir(home), 0o700)
	if l := leftovers(t, home); len(l) != 0 {
		t.Errorf("leftovers = %v", l)
	}
}

func TestSetupCodexEmptyFile(t *testing.T) {
	opts, path := codexFixture(t, []byte{})
	c, err := SetupCodex(testCLI, opts)
	if err != nil {
		t.Fatal(err)
	}
	wantCodexChange(t, c, path, ActionAdded, true)
	if got := readBytes(t, path); string(got) != codexBlock(testCLI, opts, "\n") {
		t.Errorf("config.toml = %q, want exactly the block", got)
	}
}

func TestSetupCodexReplacesBlock(t *testing.T) {
	const prefix = "# mine\nmodel = \"o3\"\n\n"
	const suffix = "\n[profiles.slow]\nmodel = \"o3-pro\"\n"
	old := CodexOptions{DBPath: "/old/place.db"}
	orig := []byte(prefix + codexBlock("/old/casebook", old, "\n") + suffix)
	opts, path := codexFixture(t, orig)
	c, err := SetupCodex(testCLI, opts)
	if err != nil {
		t.Fatal(err)
	}
	wantCodexChange(t, c, path, ActionUpdated, true)
	wantBackupOf(t, c, orig)
	want := prefix + codexBlock(testCLI, opts, "\n") + suffix
	if got := readBytes(t, path); string(got) != want {
		t.Errorf("config.toml differs: %s", firstDiff(got, []byte(want)))
	}
}

// wantCodexRefused checks err is a RefusedError for path whose reason holds
// reason, that the file and its directory are untouched, and that the
// message quotes none of the secrets.
func wantCodexRefused(t *testing.T, err error, path string, orig []byte, reason string, secrets ...string) {
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
	for _, s := range secrets {
		if strings.Contains(msg, s) {
			t.Errorf("message %q quotes the file (%q)", msg, s)
		}
	}
	if got := readBytes(t, path); string(got) != string(orig) {
		t.Errorf("file changed to %q", got)
	}
	if l := leftovers(t, filepath.Dir(path)); len(l) != 0 {
		t.Errorf("leftovers = %v", l)
	}
}

// codexMalformed are config.toml files setup and remove refuse.
var codexMalformed = []struct {
	name, doc, reason string
}{
	{"invalid TOML", "model = \"o3\"\nsecret_token = = \"placeholder-secret-zz\"\n", "not valid TOML (line 2, column "},
	{"begin without end", "model = \"o3\"\n" + codexBlockBegin + "\n[mcp_servers.casebook]\ncommand = 'placeholder-secret-zz'\n", damagedMarkersReason},
	{"two begin lines", codexBlockBegin + "\n" + codexBlock("/x/placeholder-secret-zz", CodexOptions{}, "\n"), damagedMarkersReason},
	{"end before begin", codexBlockEnd + "\nkey = 'placeholder-secret-zz'\n" + codexBlock("/x/casebook", CodexOptions{}, "\n"), damagedMarkersReason},
}

func TestSetupCodexRefuses(t *testing.T) {
	cases := append([]struct{ name, doc, reason string }{
		{"unmanaged fixture", "", unmanagedCodexReason},
		{"unmanaged env subtable only", "[mcp_servers.casebook.env]\nTOKEN = \"placeholder-secret-zz\"\n", unmanagedCodexReason},
		{"inline mcp_servers table", "mcp_servers = { other = { command = \"placeholder-secret-zz\" } }\n",
			"the merged file would not load as intended (it would not be valid TOML at line "},
		{"stray key after the block", codexBlock("/old/casebook", CodexOptions{}, "\n") + "model = \"placeholder-secret-zz\"\n",
			"the merged file would not load as intended (mcp_servers.casebook would hold keys from outside the Casebook block)"},
		{"mcp_servers is an array of tables", "[[mcp_servers]]\nname = \"placeholder-secret-zz\"\n",
			"the merged file would not load as intended"},
	}, codexMalformed...)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			doc := []byte(c.doc)
			secrets := []string{"placeholder-secret-zz"}
			if c.doc == "" {
				doc = readCodexFixture(t, "unmanaged.toml")
				secrets = []string{"placeholder-handwritten-value", "casebook-by-hand", "handwritten.db"}
			}
			opts, path := codexFixture(t, doc)
			c2, err := SetupCodex(testCLI, opts)
			if c2.File != "" {
				t.Errorf("change = %+v on refusal", c2)
			}
			wantCodexRefused(t, err, path, doc, c.reason, secrets...)
		})
	}
}

func TestRemoveCodex(t *testing.T) {
	t.Run("typical round trip", func(t *testing.T) {
		orig := readCodexFixture(t, "typical.toml")
		opts, path := codexFixture(t, orig)
		if _, err := SetupCodex(testCLI, opts); err != nil {
			t.Fatal(err)
		}
		installed := readBytes(t, path)
		c, err := RemoveCodex(opts)
		if err != nil {
			t.Fatal(err)
		}
		wantCodexChange(t, c, path, ActionRemoved, true)
		if got := readBytes(t, path); string(got) != string(orig) {
			t.Errorf("after remove: %s", firstDiff(got, orig))
		}
		if got := readBytes(t, c.Backup); string(got) != string(installed) {
			t.Errorf("remove backup does not hold the installed bytes")
		}

		// Nothing left to remove: unchanged, no write.
		before := leftovers(t, filepath.Dir(path))
		c, err = RemoveCodex(opts)
		if err != nil {
			t.Fatal(err)
		}
		wantCodexChange(t, c, path, ActionUnchanged, false)
		if c.Note != "" {
			t.Errorf("note = %q, want none", c.Note)
		}
		if after := leftovers(t, filepath.Dir(path)); len(after) != len(before) {
			t.Errorf("second remove left %v", after)
		}
	})
	t.Run("crlf round trip", func(t *testing.T) {
		orig := []byte(strings.ReplaceAll(string(readCodexFixture(t, "typical.toml")), "\n", "\r\n"))
		opts, path := codexFixture(t, orig)
		if _, err := SetupCodex(testCLI, opts); err != nil {
			t.Fatal(err)
		}
		if _, err := RemoveCodex(opts); err != nil {
			t.Fatal(err)
		}
		if got := readBytes(t, path); string(got) != string(orig) {
			t.Errorf("after remove: %s", firstDiff(got, orig))
		}
	})
	t.Run("no trailing newline", func(t *testing.T) {
		orig := []byte("model = \"o3\"")
		opts, path := codexFixture(t, orig)
		if _, err := SetupCodex(testCLI, opts); err != nil {
			t.Fatal(err)
		}
		if _, err := RemoveCodex(opts); err != nil {
			t.Fatal(err)
		}
		if got := readBytes(t, path); string(got) != string(orig)+"\n" {
			t.Errorf("after remove = %q, want the original plus one newline", got)
		}
	})
	t.Run("block in the middle", func(t *testing.T) {
		const prefix, suffix = "a = 1\n", "\n[profiles.x]\nb = 2\n"
		opts, path := codexFixture(t, []byte(prefix+codexBlock(testCLI, CodexOptions{}, "\n")+suffix))
		c, err := RemoveCodex(opts)
		if err != nil {
			t.Fatal(err)
		}
		wantCodexChange(t, c, path, ActionRemoved, true)
		// No blank line before the block, so the one after it goes.
		if got := readBytes(t, path); string(got) != "a = 1\n[profiles.x]\nb = 2\n" {
			t.Errorf("after remove = %q", got)
		}
	})
	t.Run("block only", func(t *testing.T) {
		opts, path := codexFixture(t, nil)
		if _, err := SetupCodex(testCLI, opts); err != nil {
			t.Fatal(err)
		}
		if _, err := RemoveCodex(opts); err != nil {
			t.Fatal(err)
		}
		if got := readBytes(t, path); len(got) != 0 {
			t.Errorf("after remove = %q, want an empty file", got)
		}
	})
	t.Run("no block", func(t *testing.T) {
		orig := readCodexFixture(t, "typical.toml")
		opts, path := codexFixture(t, orig)
		c, err := RemoveCodex(opts)
		if err != nil {
			t.Fatal(err)
		}
		wantCodexChange(t, c, path, ActionUnchanged, false)
		if got := readBytes(t, path); string(got) != string(orig) {
			t.Errorf("file changed")
		}
		if l := leftovers(t, filepath.Dir(path)); len(l) != 0 {
			t.Errorf("leftovers = %v", l)
		}
	})
	t.Run("missing file stays missing", func(t *testing.T) {
		opts, path := codexFixture(t, nil)
		c, err := RemoveCodex(opts)
		if err != nil {
			t.Fatal(err)
		}
		wantCodexChange(t, c, path, ActionUnchanged, false)
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("remove created %s (%v)", path, err)
		}
	})
	t.Run("unmanaged casebook only", func(t *testing.T) {
		orig := readCodexFixture(t, "unmanaged.toml")
		opts, path := codexFixture(t, orig)
		c, err := RemoveCodex(opts)
		if err != nil {
			t.Fatal(err)
		}
		wantCodexChange(t, c, path, ActionUnchanged, false)
		if c.Note != unmanagedRemainsNote {
			t.Errorf("note = %q, want %q", c.Note, unmanagedRemainsNote)
		}
		if got := readBytes(t, path); string(got) != string(orig) {
			t.Errorf("file changed")
		}
		if l := leftovers(t, filepath.Dir(path)); len(l) != 0 {
			t.Errorf("leftovers = %v", l)
		}
	})
	t.Run("env table outside the block stays and is flagged", func(t *testing.T) {
		outside := "\n[mcp_servers.casebook.env]\nCASEBOOK_LOG_LEVEL = 'debug'\n"
		orig := []byte("model = 'o3'\n\n" + codexBlock(testCLI, CodexOptions{}, "\n") + outside)
		opts, path := codexFixture(t, orig)
		c, err := RemoveCodex(opts)
		if err != nil {
			t.Fatal(err)
		}
		wantCodexChange(t, c, path, ActionRemoved, true)
		if c.Note != orphanRemainsNote {
			t.Errorf("note = %q, want %q", c.Note, orphanRemainsNote)
		}
		// The block and its separator go; the table outside it stays.
		if got := string(readBytes(t, path)); got != "model = 'o3'\n"+outside {
			t.Errorf("file = %q", got)
		}
		// Without the block, a second remove changes nothing and still warns.
		c, err = RemoveCodex(opts)
		if err != nil {
			t.Fatal(err)
		}
		if c.Action != ActionUnchanged || c.Note != orphanRemainsNote {
			t.Errorf("second remove = %+v", c)
		}
	})
}

func TestRemoveCodexRefuses(t *testing.T) {
	for _, c := range codexMalformed {
		t.Run(c.name, func(t *testing.T) {
			opts, path := codexFixture(t, []byte(c.doc))
			ch, err := RemoveCodex(opts)
			if ch.File != "" {
				t.Errorf("change = %+v on refusal", ch)
			}
			wantCodexRefused(t, err, path, []byte(c.doc), c.reason, "placeholder-secret-zz")
		})
	}
}

func TestCodexConfigPreview(t *testing.T) {
	opts := CodexOptions{Paths: Paths{CodexConfig: "/home/u/.codex/config.toml"}}
	want := "# Codex MCP server: add this block to /home/u/.codex/config.toml\n" +
		"# BEGIN casebook (managed by \"casebook setup codex\"; remove with \"casebook setup codex --remove\")\n" +
		"# Keep root-level keys above this block: TOML reads keys below a table header as part of that table.\n" +
		"[mcp_servers.casebook]\n" +
		"command = '/opt/casebook/bin/casebook'\n" +
		"args = ['mcp']\n" +
		"# END casebook\n"
	if got := CodexConfigPreview("/opt/casebook/bin/casebook", opts); got != want {
		t.Errorf("preview =\n%s\nwant\n%s", got, want)
	}

	opts.DBPath = "/data/my cases.db"
	got := CodexConfigPreview(testCLI, opts)
	wantArgs := "args = ['mcp', '--db', '/data/my cases.db']\n"
	if !strings.Contains(got, "command = '"+testCLI+"'\n"+wantArgs) {
		t.Errorf("preview with --db =\n%s", got)
	}
	server, found, err := codexCasebook([]byte(got))
	if err != nil {
		t.Fatalf("preview is not valid TOML: %v", err)
	}
	if why := checkCodexServer(server, found, testCLI, []string{"mcp", "--db", "/data/my cases.db"}, nil); why != "" {
		t.Errorf("preview parses to the wrong table: %s", why)
	}
}

// TestSetupCodexUpgradesOldBlock checks that a block written before the
// note line existed (IN-11) is rewritten with exactly one note line.
func TestSetupCodexUpgradesOldBlock(t *testing.T) {
	old := "model = 'o3'\n\n" + codexBlockBegin + "\n[mcp_servers.casebook]\ncommand = '" + testCLI + "'\nargs = ['mcp']\n" + codexBlockEnd + "\n"
	opts, path := codexFixture(t, []byte(old))
	c, err := SetupCodex(testCLI, opts)
	if err != nil {
		t.Fatal(err)
	}
	wantCodexChange(t, c, path, ActionUpdated, true)
	want := "model = 'o3'\n\n" + codexBlock(testCLI, opts, "\n")
	if got := string(readBytes(t, path)); got != want {
		t.Errorf("file = %q, want %q", got, want)
	}
	c, err = SetupCodex(testCLI, opts)
	if err != nil {
		t.Fatal(err)
	}
	wantCodexChange(t, c, path, ActionUnchanged, false)
}
