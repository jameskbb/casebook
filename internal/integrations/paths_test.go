package integrations

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolvePaths(t *testing.T) {
	home := filepath.Join(string(filepath.Separator), "home", "u")
	env := func(vars map[string]string) Getenv {
		return func(k string) string { return vars[k] }
	}
	p := ResolvePaths(env(nil), home)
	want := Paths{
		ClaudeConfigDir: filepath.Join(home, ".claude"),
		ClaudeJSON:      filepath.Join(home, ".claude.json"),
		ClaudeSettings:  filepath.Join(home, ".claude", "settings.json"),
		CodexHome:       filepath.Join(home, ".codex"),
		CodexConfig:     filepath.Join(home, ".codex", "config.toml"),
	}
	if p != want {
		t.Errorf("defaults = %+v, want %+v", p, want)
	}

	// Absolute on every OS (a bare \cfg is drive-relative on Windows).
	root := t.TempDir()
	cfg := filepath.Join(root, "cfg")
	codex := filepath.Join(root, "cx")
	p = ResolvePaths(env(map[string]string{"CLAUDE_CONFIG_DIR": cfg, "CODEX_HOME": codex}), home)
	want = Paths{
		ClaudeConfigDir: cfg,
		ClaudeJSON:      filepath.Join(cfg, ".claude.json"),
		ClaudeSettings:  filepath.Join(cfg, "settings.json"),
		CodexHome:       codex,
		CodexConfig:     filepath.Join(codex, "config.toml"),
	}
	if p != want {
		t.Errorf("overrides = %+v, want %+v", p, want)
	}

	p = ResolvePaths(env(map[string]string{"CLAUDE_CONFIG_DIR": "  ", "CODEX_HOME": ""}), home)
	if p.ClaudeJSON != filepath.Join(home, ".claude.json") || p.CodexHome != filepath.Join(home, ".codex") {
		t.Errorf("blank overrides were used: %+v", p)
	}

	p = ResolvePaths(env(map[string]string{"CLAUDE_CONFIG_DIR": "rel/claude", "CODEX_HOME": "rel/codex"}), home)
	wd, _ := os.Getwd()
	if p.ClaudeJSON != filepath.Join(wd, "rel", "claude", ".claude.json") || p.CodexConfig != filepath.Join(wd, "rel", "codex", "config.toml") {
		t.Errorf("relative overrides not made absolute: %+v", p)
	}
}

// touch creates a file with its parent directories.
func touch(t *testing.T, path string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("bin"), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

// noPath is a lookPath that finds nothing.
func noPath(string) (string, error) { return "", errors.New("not found") }

func TestCLIPath(t *testing.T) {
	realDir := func(t *testing.T) string {
		d, err := filepath.EvalSymlinks(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		return d
	}
	t.Run("the CLI itself", func(t *testing.T) {
		dir := realDir(t)
		exe := touch(t, filepath.Join(dir, "casebook"))
		got, err := cliPathFrom(exe, "linux", noPath, filepath.EvalSymlinks, os.Stat)
		if err != nil || got != exe {
			t.Fatalf("cliPathFrom = %s, %v; want %s", got, err, exe)
		}
	})
	t.Run("packaged bin before sibling", func(t *testing.T) {
		dir := realDir(t)
		exe := touch(t, filepath.Join(dir, "Casebook"))
		bin := touch(t, filepath.Join(dir, "bin", "casebook"))
		touch(t, filepath.Join(dir, "casebook"))
		got, err := cliPathFrom(exe, "linux", noPath, filepath.EvalSymlinks, os.Stat)
		if err != nil || got != bin {
			t.Fatalf("cliPathFrom = %s, %v; want %s", got, err, bin)
		}
	})
	t.Run("windows packaged layout", func(t *testing.T) {
		dir := realDir(t)
		exe := touch(t, filepath.Join(dir, "Casebook.exe"))
		bin := touch(t, filepath.Join(dir, "bin", "casebook.exe"))
		got, err := cliPathFrom(exe, "windows", noPath, filepath.EvalSymlinks, os.Stat)
		if err != nil || got != bin {
			t.Fatalf("cliPathFrom = %s, %v; want %s", got, err, bin)
		}
	})
	t.Run("sibling", func(t *testing.T) {
		dir := realDir(t)
		exe := touch(t, filepath.Join(dir, "Casebook"))
		sibling := filepath.Join(dir, "casebook")
		onPath := touch(t, filepath.Join(realDir(t), "casebook"))
		lookPath := func(string) (string, error) { return onPath, nil }
		if _, err := os.Stat(sibling); err == nil {
			// Case-insensitive volume: "casebook" is the desktop binary
			// itself and must never be returned.
			got, err := cliPathFrom(exe, "linux", lookPath, filepath.EvalSymlinks, os.Stat)
			if err != nil || got != onPath {
				t.Fatalf("cliPathFrom = %s, %v; want PATH's %s, not the desktop binary", got, err, onPath)
			}
			return
		}
		touch(t, sibling)
		got, err := cliPathFrom(exe, "linux", lookPath, filepath.EvalSymlinks, os.Stat)
		if err != nil || got != sibling {
			t.Fatalf("cliPathFrom = %s, %v; want %s", got, err, sibling)
		}
	})
	t.Run("bin is a directory", func(t *testing.T) {
		dir := realDir(t)
		exe := touch(t, filepath.Join(dir, "Casebook"))
		if err := os.MkdirAll(filepath.Join(dir, "bin", "casebook"), 0o700); err != nil {
			t.Fatal(err)
		}
		if _, err := cliPathFrom(exe, "linux", noPath, filepath.EvalSymlinks, os.Stat); err == nil {
			t.Fatal("a directory was accepted as the CLI")
		}
	})
	t.Run("PATH", func(t *testing.T) {
		dir := realDir(t)
		exe := touch(t, filepath.Join(dir, "Casebook"))
		onPath := touch(t, filepath.Join(realDir(t), "casebook"))
		var asked string
		lookPath := func(name string) (string, error) { asked = name; return onPath, nil }
		got, err := cliPathFrom(exe, "linux", lookPath, filepath.EvalSymlinks, os.Stat)
		if err != nil || got != onPath || asked != "casebook" {
			t.Fatalf("cliPathFrom = %s, %v (looked up %q); want %s", got, err, asked, onPath)
		}
		_, _ = cliPathFrom(exe, "windows", lookPath, filepath.EvalSymlinks, os.Stat)
		if asked != "casebook.exe" {
			t.Errorf("windows looked up %q, want casebook.exe", asked)
		}
	})
	t.Run("not found", func(t *testing.T) {
		dir := realDir(t)
		exe := touch(t, filepath.Join(dir, "Casebook"))
		_, err := cliPathFrom(exe, "linux", noPath, filepath.EvalSymlinks, os.Stat)
		if err == nil || !strings.Contains(err.Error(), "PATH") || !strings.Contains(err.Error(), exe) {
			t.Fatalf("err = %v, want it to name %s and PATH", err, exe)
		}
	})
	t.Run("symlinks resolved", func(t *testing.T) {
		dir := realDir(t)
		target := touch(t, filepath.Join(realDir(t), "casebook-real"))
		exe := touch(t, filepath.Join(dir, "Casebook"))
		link := filepath.Join(dir, "bin", "casebook")
		if err := os.MkdirAll(filepath.Dir(link), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, link); err != nil {
			t.Skipf("cannot create symlinks here: %v", err)
		}
		got, err := cliPathFrom(exe, "linux", noPath, filepath.EvalSymlinks, os.Stat)
		if err != nil || got != target {
			t.Fatalf("cliPathFrom = %s, %v; want the resolved %s", got, err, target)
		}
		failing := func(string) (string, error) { return "", errors.New("cannot resolve") }
		got, err = cliPathFrom(exe, "linux", noPath, failing, os.Stat)
		if err != nil || got != link {
			t.Fatalf("cliPathFrom with failing resolution = %s, %v; want the unresolved %s", got, err, link)
		}
	})
	t.Run("relative results become absolute", func(t *testing.T) {
		lookPath := func(string) (string, error) { return filepath.Join("rel", "casebook"), nil }
		got, err := cliPathFrom("", "linux", lookPath, func(p string) (string, error) { return p, nil }, os.Stat)
		if err != nil || !filepath.IsAbs(got) {
			t.Fatalf("cliPathFrom = %s, %v; want an absolute path", got, err)
		}
	})
	t.Run("CLIPath", func(t *testing.T) {
		// The test binary is not named casebook; only check the contract
		// shape: an absolute path or an error naming PATH.
		got, err := CLIPath()
		if err == nil && !filepath.IsAbs(got) {
			t.Errorf("CLIPath = %s, want an absolute path", got)
		}
		if err != nil && !strings.Contains(err.Error(), "PATH") {
			t.Errorf("CLIPath error = %v", err)
		}
	})
}
