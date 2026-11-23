package integrations

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// desktopFixture writes data (nil for no file) as the Claude desktop app's
// config in a temp folder and returns options pointing at it.
func desktopFixture(t *testing.T, data []byte) (ClaudeDesktopOptions, string) {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "Claude", "claude_desktop_config.json")
	if data != nil {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return ClaudeDesktopOptions{Config: path, Now: fixedClock}, path
}

const desktopOrig = `{
  "preferences": {
    "sidebarMode": "chat"
  },
  "mcpServers": {
    "files": {
      "command": "npx",
      "args": [
        "-y",
        "files-server"
      ]
    }
  }
}
`

// Connect adds only mcpServers.casebook, keeps every other byte, backs the
// file up first, writes nothing on a rerun, and disconnect puts the file
// back byte for byte.
func TestSetupClaudeDesktopRoundTrip(t *testing.T) {
	opts, path := desktopFixture(t, []byte(desktopOrig))
	c, err := SetupClaudeDesktop(testCLI, opts)
	if err != nil {
		t.Fatal(err)
	}
	if c.File != path || c.Action != ActionAdded || c.Backup == "" || len(c.Entries) != 1 || c.Entries[0] != "mcpServers.casebook" {
		t.Fatalf("change = %+v", c)
	}
	if !bytes.Equal(readBytes(t, c.Backup), []byte(desktopOrig)) {
		t.Error("the backup is not the file as it was")
	}
	want := strings.Replace(desktopOrig, "      ]\n    }\n  }\n}\n",
		"      ]\n    },\n    \"casebook\": {\n      \"command\": \""+testCLI+"\",\n      \"args\": [\n        \"mcp\"\n      ]\n    }\n  }\n}\n", 1)
	if got := readBytes(t, path); string(got) != want {
		t.Fatalf("file; %s", firstDiff(got, []byte(want)))
	}
	if st := InspectClaudeDesktop(path, testCLI); !st.Registered || st.Command != testCLI || !st.ConfigValid {
		t.Errorf("inspect after connect = %+v", st)
	}

	again, err := SetupClaudeDesktop(testCLI, opts)
	if err != nil || again.Action != ActionUnchanged || again.Backup != "" {
		t.Fatalf("rerun = %+v, %v", again, err)
	}

	r, err := RemoveClaudeDesktop(opts)
	if err != nil || r.Action != ActionRemoved || r.Backup == "" {
		t.Fatalf("remove = %+v, %v", r, err)
	}
	if got := readBytes(t, path); string(got) != desktopOrig {
		t.Fatalf("after remove; %s", firstDiff(got, []byte(desktopOrig)))
	}
	if r, err := RemoveClaudeDesktop(opts); err != nil || r.Action != ActionUnchanged || r.Backup != "" {
		t.Fatalf("second remove = %+v, %v", r, err)
	}
}

// An existing casebook entry keeps what the person added to it; only command
// and args are replaced, and --db goes into args.
func TestSetupClaudeDesktopReplacesOnlyItsEntry(t *testing.T) {
	orig := `{
  "mcpServers": {
    "casebook": {
      "command": "/old/casebook",
      "args": [
        "mcp"
      ],
      "env": {
        "CASEBOOK_DB": "/work/case.db"
      }
    }
  }
}
`
	opts, path := desktopFixture(t, []byte(orig))
	opts.DBPath = "/data/other.db"
	c, err := SetupClaudeDesktop(testCLI, opts)
	if err != nil || c.Action != ActionUpdated || c.Note != "kept your env in mcpServers.casebook" {
		t.Fatalf("change = %+v, %v", c, err)
	}
	want := strings.Replace(orig, `"/old/casebook"`, `"`+testCLI+`"`, 1)
	want = strings.Replace(want, "        \"mcp\"\n", "        \"mcp\",\n        \"--db\",\n        \"/data/other.db\"\n", 1)
	if got := readBytes(t, path); string(got) != want {
		t.Fatalf("file; %s", firstDiff(got, []byte(want)))
	}
}

// A missing file is created with only Casebook's entry; removing from a
// missing file creates nothing.
func TestSetupClaudeDesktopMissingFile(t *testing.T) {
	opts, path := desktopFixture(t, nil)
	if r, err := RemoveClaudeDesktop(opts); err != nil || r.Action != ActionUnchanged {
		t.Fatalf("remove = %+v, %v", r, err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("remove created the file")
	}
	c, err := SetupClaudeDesktop(testCLI, opts)
	if err != nil || c.Action != ActionAdded || c.Backup != "" {
		t.Fatalf("change = %+v, %v", c, err)
	}
	want := "{\n  \"mcpServers\": {\n    \"casebook\": {\n      \"command\": \"" + testCLI + "\",\n      \"args\": [\n        \"mcp\"\n      ]\n    }\n  }\n}\n"
	if got := readBytes(t, path); string(got) != want {
		t.Fatalf("file; %s", firstDiff(got, []byte(want)))
	}
}

// Anything that is not one strict JSON object with an mcpServers object is
// refused, for connect and for disconnect, and the file is left as it was.
func TestClaudeDesktopRefuses(t *testing.T) {
	for name, data := range map[string]string{
		"comment":            "{\n  // mine\n  \"mcpServers\": {}\n}\n",
		"trailing comma":     "{\n  \"mcpServers\": {},\n}\n",
		"not an object":      "[]\n",
		"cut short":          "{\n  \"mcpServers\": {",
		"byte order mark":    "\xEF\xBB\xBF{}\n",
		"servers not object": "{\n  \"mcpServers\": []\n}\n",
		"entry not object":   "{\n  \"mcpServers\": {\n    \"casebook\": \"x\"\n  }\n}\n",
		"duplicate key":      "{\n  \"mcpServers\": {},\n  \"mcpServers\": {}\n}\n",
	} {
		t.Run(name, func(t *testing.T) {
			opts, path := desktopFixture(t, []byte(data))
			var refused *RefusedError
			if _, err := SetupClaudeDesktop(testCLI, opts); !errors.As(err, &refused) {
				t.Fatalf("setup err = %v, want a refusal", err)
			}
			if _, err := RemoveClaudeDesktop(opts); !errors.As(err, &refused) {
				t.Fatalf("remove err = %v, want a refusal", err)
			}
			if got := readBytes(t, path); string(got) != data {
				t.Error("a refused file was changed")
			}
			if l := leftovers(t, filepath.Dir(path)); len(l) != 0 {
				t.Errorf("leftovers = %v", l)
			}
		})
	}
}

// A file another program changes while Casebook is writing is left with that
// program's content, and the error says so in a way callers can test.
func TestSetupClaudeDesktopChangedUnderneath(t *testing.T) {
	opts, path := desktopFixture(t, []byte(desktopOrig))
	concurrent := []byte("{\n  \"preferences\": {}\n}\n")
	beforeRename = func(p string) {
		if err := os.WriteFile(p, concurrent, 0o600); err != nil {
			t.Error(err)
		}
	}
	t.Cleanup(func() { beforeRename = nil })
	if _, err := SetupClaudeDesktop(testCLI, opts); !errors.Is(err, ErrChangedUnderneath) {
		t.Fatalf("err = %v, want ErrChangedUnderneath", err)
	}
	if !bytes.Equal(readBytes(t, path), concurrent) {
		t.Error("the concurrent content was not kept")
	}
	if l := leftovers(t, filepath.Dir(path)); len(l) != 0 {
		t.Errorf("leftovers = %v", l)
	}
}

func TestClaudeDesktopConfig(t *testing.T) {
	env := func(k string) string {
		if k == "APPDATA" {
			return filepath.Join("C:", "Users", "ana", "AppData", "Roaming")
		}
		return ""
	}
	none := func(string) string { return "" }
	home := filepath.Join("home", "ana")
	for _, tc := range []struct {
		goos   string
		getenv Getenv
		home   string
		want   string
	}{
		{"windows", env, home, filepath.Join("C:", "Users", "ana", "AppData", "Roaming", "Claude", "claude_desktop_config.json")},
		{"windows", none, home, ""},
		{"darwin", none, home, filepath.Join(home, "Library", "Application Support", "Claude", "claude_desktop_config.json")},
		{"linux", none, home, filepath.Join(home, ".config", "Claude", "claude_desktop_config.json")},
		{"linux", none, "", ""},
	} {
		if got := ClaudeDesktopConfig(tc.goos, tc.getenv, tc.home); got != tc.want {
			t.Errorf("ClaudeDesktopConfig(%s) = %q, want %q", tc.goos, got, tc.want)
		}
	}
	// The location `casebook setup mcp --client claude-desktop` prints is this one.
	s, err := MCPSnippetFor(MCPSnippetOptions{Client: "claude-desktop", CLIPath: testCLI, GOOS: "windows", Home: home, Getenv: env})
	if err != nil || s.Location != ClaudeDesktopConfig("windows", env, home) {
		t.Errorf("printed location = %q, %v", s.Location, err)
	}
}
