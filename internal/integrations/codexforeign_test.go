package integrations

import (
	"bytes"
	"os"
	"strings"
	"testing"

	toml "github.com/pelletier/go-toml/v2"
)

// codexTrust is the table Codex 0.156 appends at the end of config.toml.
const codexTrust = "\n[projects.\"/home/jo/shop\"]\ntrust_level = \"trusted\"\n"

// codexAppendInsideBlock writes what Codex leaves behind when the Casebook
// block ends the file: its new table goes before the trailing END comment.
func codexAppendInsideBlock(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	i := bytes.LastIndex(data, []byte(codexBlockEnd))
	if i < 0 {
		t.Fatal("no END line")
	}
	out := append(append(append([]byte(nil), data[:i]...), codexTrust[1:]...), data[i:]...)
	if err := os.WriteFile(path, out, 0o600); err != nil {
		t.Fatal(err)
	}
}

// wantTrustOutsideBlock checks the trust entry and the root key survive,
// and that the trust table is no longer inside a managed block.
func wantTrustOutsideBlock(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Model    string                       `toml:"model"`
		Projects map[string]map[string]string `toml:"projects"`
	}
	if err := toml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("not TOML: %v\n%s", err, data)
	}
	if doc.Model != "o3" || doc.Projects["/home/jo/shop"]["trust_level"] != "trusted" {
		t.Fatalf("lost user settings: %+v\n%s", doc, data)
	}
	if span, err := findManagedBlock(data); err != nil {
		t.Fatal(err)
	} else if span.Found && bytes.Contains(data[span.Start:span.End], []byte("projects")) {
		t.Fatalf("trust table still inside the block:\n%s", data)
	}
	if foreignInBlock(data) {
		t.Fatal("foreignInBlock still true")
	}
	return data
}

func TestCodexForeignTableInBlock(t *testing.T) {
	cli := "/usr/local/bin/casebook"
	newFile := func(t *testing.T) (CodexOptions, string) {
		t.Helper()
		opts, path := codexFixture(t, []byte("model = \"o3\"\n"))
		if _, err := SetupCodex(cli, opts); err != nil {
			t.Fatal(err)
		}
		codexAppendInsideBlock(t, path)
		data, _ := os.ReadFile(path)
		if !foreignInBlock(data) {
			t.Fatalf("fixture has no foreign table in the block:\n%s", data)
		}
		if st := InspectCodex(opts.Paths, cli); !st.ForeignInBlock || !st.Managed {
			t.Fatalf("InspectCodex = %+v", st)
		}
		return opts, path
	}

	t.Run("setup moves it out", func(t *testing.T) {
		opts, path := newFile(t)
		c, err := SetupCodex(cli, opts)
		if err != nil {
			t.Fatal(err)
		}
		wantCodexChange(t, c, path, ActionUpdated, true)
		first := wantTrustOutsideBlock(t, path)
		// Rerunning changes nothing, and the block no longer ends the file.
		c, err = SetupCodex(cli, opts)
		if err != nil {
			t.Fatal(err)
		}
		wantCodexChange(t, c, path, ActionUnchanged, false)
		span, _ := findManagedBlock(first)
		if span.End == len(first) {
			t.Fatalf("block still ends the file:\n%s", first)
		}
		// Re-setup with another command keeps the trust entry too.
		if _, err := SetupCodex("/opt/casebook", opts); err != nil {
			t.Fatal(err)
		}
		wantTrustOutsideBlock(t, path)
		if st := InspectCodex(opts.Paths, cli); st.ForeignInBlock {
			t.Fatal("InspectCodex still reports a foreign table")
		}
	})

	t.Run("remove keeps it", func(t *testing.T) {
		opts, path := newFile(t)
		c, err := RemoveCodex(opts)
		if err != nil {
			t.Fatal(err)
		}
		wantCodexChange(t, c, path, ActionRemoved, true)
		data := wantTrustOutsideBlock(t, path)
		if bytes.Contains(data, []byte("casebook")) {
			t.Fatalf("block not removed:\n%s", data)
		}
		want := "model = \"o3\"\n\n[projects.\"/home/jo/shop\"]\ntrust_level = \"trusted\"\n"
		if string(data) != want {
			t.Fatalf("after remove:\n%q\nwant\n%q", data, want)
		}
	})

	t.Run("casebook tables stay in the block", func(t *testing.T) {
		opts, path := newFile(t)
		data, _ := os.ReadFile(path)
		i := bytes.Index(data, []byte("[projects."))
		withEnv := append(append(append([]byte(nil), data[:i]...), "[mcp_servers.casebook.env]\nCASEBOOK_LOG_LEVEL = \"info\"\n\n"...), data[i:]...)
		if err := os.WriteFile(path, withEnv, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := SetupCodex(cli, opts); err != nil {
			t.Fatal(err)
		}
		out := wantTrustOutsideBlock(t, path)
		span, _ := findManagedBlock(out)
		if !bytes.Contains(out[span.Start:span.End], []byte("[mcp_servers.casebook.env]")) {
			t.Fatalf("env table left the block:\n%s", out)
		}
	})
}

func TestCasebookHeader(t *testing.T) {
	for h, want := range map[string]bool{
		"[mcp_servers.casebook]":             true,
		"[mcp_servers.casebook.env]":         true,
		`[mcp_servers."casebook"] # managed`: true,
		"[ mcp_servers . casebook ]":         true,
		`[projects."/home/jo/shop"]`:         false,
		"[mcp_servers.other]":                false,
		"[[mcp_servers.casebook]]":           false,
		"[mcp_servers]":                      false,
		strings.Repeat("[", 3) + "broken":    false,
	} {
		if got := casebookHeader(h); got != want {
			t.Errorf("casebookHeader(%q) = %v, want %v", h, got, want)
		}
	}
}
