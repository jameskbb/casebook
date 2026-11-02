package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// clientsDesktop returns a Desktop whose ClientConnections reads a temp
// home and reports cli as the CLI ("" means not found).
func clientsDesktop(t *testing.T, cli string) (*Desktop, string) {
	t.Helper()
	d, _ := newTestDesktop(t)
	home := t.TempDir()
	d.clients = &clientSeams{
		getenv: func(string) string { return "" },
		home:   func() (string, error) { return home, nil },
		cliPath: func() (string, error) {
			if cli == "" {
				return "", errors.New("not found")
			}
			return cli, nil
		},
		goos: "linux",
	}
	return d, home
}

// jsonString encodes s as a JSON string (Windows paths need escaped
// backslashes).
func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func snapshotFiles(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	_ = filepath.Walk(dir, func(p string, fi os.FileInfo, err error) error {
		if err == nil && !fi.IsDir() {
			b, _ := os.ReadFile(p)
			out[p] = string(b)
		}
		return nil
	})
	return out
}

func TestClientConnectionsNotConfigured(t *testing.T) {
	cli := filepath.Join(t.TempDir(), "casebook")
	d, home := clientsDesktop(t, cli)
	got, err := d.ClientConnections()
	if err != nil {
		t.Fatal(err)
	}
	if got.CLIPath != cli || got.CLIName != cli || got.CLISource != cliOnPath {
		t.Fatalf("cli = %q %q %q", got.CLIPath, got.CLIName, got.CLISource)
	}
	if got.Claude.Configured || got.Codex.Configured {
		t.Fatalf("configured with no config files: %+v", got)
	}
	if got.Claude.ConfigPath != filepath.Join(home, ".claude.json") || got.Codex.ConfigPath != filepath.Join(home, ".codex", "config.toml") {
		t.Fatalf("config paths %q %q", got.Claude.ConfigPath, got.Codex.ConfigPath)
	}
	if q := quoteArg("linux", cli); got.Claude.SetupCommand != q+" setup claude --hooks --skill" || got.Codex.SetupCommand != q+" setup codex --skill" {
		t.Fatalf("setup commands %q %q", got.Claude.SetupCommand, got.Codex.SetupCommand)
	}
	if !strings.Contains(got.Claude.Snippet, `"command": `+jsonString(cli)) || !strings.Contains(got.Claude.Snippet, `"mcp"`) {
		t.Fatalf("claude snippet:\n%s", got.Claude.Snippet)
	}
	if !strings.Contains(got.Codex.Snippet, "[mcp_servers.casebook]") || !strings.Contains(got.Codex.Snippet, cli) {
		t.Fatalf("codex snippet:\n%s", got.Codex.Snippet)
	}
	if got.Claude.Hooks == nil || got.Codex.Hooks == nil {
		t.Fatal("hooks must never be nil (JSON null)")
	}
	if files := snapshotFiles(t, home); len(files) != 0 {
		t.Fatalf("ClientConnections wrote files: %v", files)
	}
}

func TestClientConnectionsDetectsConfiguredAndNeverWrites(t *testing.T) {
	dir := t.TempDir()
	cli := filepath.Join(dir, "casebook")
	if err := os.WriteFile(cli, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	d, home := clientsDesktop(t, cli)
	claude := `{"mcpServers":{"casebook":{"type":"stdio","command":` + jsonString(cli) + `,"args":["mcp"],"env":{}}}}`
	if err := os.WriteFile(filepath.Join(home, ".claude.json"), []byte(claude), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".codex", "config.toml"), []byte("not = [valid"), 0o600); err != nil {
		t.Fatal(err)
	}
	before := snapshotFiles(t, home)
	got, err := d.ClientConnections()
	if err != nil {
		t.Fatal(err)
	}
	if !got.Claude.Configured || !got.Claude.MatchesCLI || !got.Claude.CommandExists || got.Claude.Command != cli {
		t.Fatalf("claude status %+v", got.Claude)
	}
	if got.Codex.Configured || !got.Codex.ConfigProblem {
		t.Fatalf("codex status %+v", got.Codex)
	}
	after := snapshotFiles(t, home)
	if len(after) != len(before) {
		t.Fatalf("files changed: before %v after %v", before, after)
	}
	for p, b := range before {
		if after[p] != b {
			t.Fatalf("%s changed", p)
		}
	}
}

func TestClientConnectionsMissingCLIAndExplicitDB(t *testing.T) {
	d, _ := clientsDesktop(t, "")
	d.explicitDB = true
	got, err := d.ClientConnections()
	if err != nil {
		t.Fatal(err)
	}
	if got.CLIPath != "" || got.CLISource != cliMissing || got.CLIName != "casebook" {
		t.Fatalf("cli = %q %q %q", got.CLIPath, got.CLISource, got.CLIName)
	}
	if got.DBPath != d.dbPath {
		t.Fatalf("db = %q, want %q", got.DBPath, d.dbPath)
	}
	want := "casebook --db " + quoteArg("linux", d.dbPath) + " setup claude --hooks --skill"
	if got.Claude.SetupCommand != want {
		t.Fatalf("setup = %q, want %q", got.Claude.SetupCommand, want)
	}
	if !strings.Contains(got.Claude.Snippet, `"--db"`) || !strings.Contains(got.Codex.Snippet, "--db") {
		t.Fatalf("snippets lack --db:\n%s\n%s", got.Claude.Snippet, got.Codex.Snippet)
	}
}

func TestClientConnectionsHomeError(t *testing.T) {
	d, _ := clientsDesktop(t, "")
	d.clients.home = func() (string, error) { return "", errors.New("no home") }
	_, err := d.ClientConnections()
	if e := decodeUIError(t, err); e.Kind != kindOther {
		t.Fatalf("kind = %q", e.Kind)
	}
}

func TestSetupCommandQuoting(t *testing.T) {
	cases := []struct{ goos, cli, db, want string }{
		{"windows", `C:\Casebook\bin\casebook.exe`, "", `C:\Casebook\bin\casebook.exe setup claude --hooks --skill`},
		{"windows", `C:\Program Files\Casebook\bin\casebook.exe`, "", `& 'C:\Program Files\Casebook\bin\casebook.exe' setup claude --hooks --skill`},
		{"windows", `C:\Casebook\bin\casebook.exe`, `C:\My Data\casebook.db`, `C:\Casebook\bin\casebook.exe --db 'C:\My Data\casebook.db' setup claude --hooks --skill`},
		{"darwin", "/Applications/Casebook.app/Contents/MacOS/bin/casebook", "", "/Applications/Casebook.app/Contents/MacOS/bin/casebook setup claude --hooks --skill"},
		{"darwin", "/Users/o'neil/My Apps/casebook", "", `'/Users/o'\''neil/My Apps/casebook' setup claude --hooks --skill`},
		// Hostile paths: $, backtick and % stay literal inside single quotes.
		{"windows", `C:\Users\100%user\casebook.exe`, `C:\$env:TEMP\a` + "`" + `b.db`, "& 'C:\\Users\\100%user\\casebook.exe' --db 'C:\\$env:TEMP\\a`b.db' setup claude --hooks --skill"},
		{"windows", "C:\\Users\\O\u2019Brien\\casebook.exe", "", "& 'C:\\Users\\O\u2019\u2019Brien\\casebook.exe' setup claude --hooks --skill"},
		{"windows", `C:\Users\O'Neil\casebook.exe`, "", `& 'C:\Users\O''Neil\casebook.exe' setup claude --hooks --skill`},
		{"windows", `C:\a,b\casebook.exe`, "", `& 'C:\a,b\casebook.exe' setup claude --hooks --skill`},
		{"linux", "/opt/$(id)/`id`/x\\y/casebook", "", "'/opt/$(id)/`id`/x\\y/casebook' setup claude --hooks --skill"},
		{"linux", "/opt/100%/casebook", "", "/opt/100%/casebook setup claude --hooks --skill"},
	}
	for _, c := range cases {
		if got := setupCommand(c.goos, c.cli, c.db, "claude"); got != c.want {
			t.Errorf("setupCommand(%s, %q, %q) = %q, want %q", c.goos, c.cli, c.db, got, c.want)
		}
	}
}

func TestUsesExplicitDB(t *testing.T) {
	env := map[string]string{}
	get := func(k string) string { return env[k] }
	if usesExplicitDB(flags{}, get) {
		t.Fatal("default reported explicit")
	}
	if !usesExplicitDB(flags{DataDir: "x"}, get) {
		t.Fatal("--data-dir not explicit")
	}
	env["CASEBOOK_HOME"] = "/tmp/x"
	if !usesExplicitDB(flags{}, get) {
		t.Fatal("CASEBOOK_HOME not explicit")
	}
}

func TestAppVersion(t *testing.T) {
	d, _ := newTestDesktop(t)
	if v := d.AppVersion(); v != "dev" {
		t.Fatalf("AppVersion = %q, want dev in an unstamped test build", v)
	}
}

// AR-12: the Program Files path needs the PowerShell call operator, so the
// panel names PowerShell and also offers the Command Prompt form.
func TestCmdSetupCommand(t *testing.T) {
	cases := []struct{ goos, cli, db, want string }{
		{"windows", `C:\Program Files\Casebook\bin\casebook.exe`, "", `"C:\Program Files\Casebook\bin\casebook.exe" setup claude --hooks --skill`},
		{"windows", `C:\Casebook\bin\casebook.exe`, `C:\My Data\casebook.db`, `C:\Casebook\bin\casebook.exe --db "C:\My Data\casebook.db" setup claude --hooks --skill`},
		// Same text in both shells: no second form.
		{"windows", `C:\Casebook\bin\casebook.exe`, "", ""},
		{"darwin", "/Users/o'neil/My Apps/casebook", "", ""},
		// cmd.exe expands %NAME% even inside double quotes: no cmd form.
		{"windows", `C:\Users\100%user\casebook.exe`, "", ""},
		{"windows", `C:\Casebook\bin\casebook.exe`, `C:\%TEMP%\c.db`, ""},
		// $ and backtick mean nothing to cmd.exe; & and ^ are text inside quotes.
		{"windows", `C:\$work\a` + "`" + `b\casebook.exe`, "", `C:\$work\a` + "`" + `b\casebook.exe setup claude --hooks --skill`},
		{"windows", `C:\R&D ^x\casebook.exe`, "", `"C:\R&D ^x\casebook.exe" setup claude --hooks --skill`},
	}
	for _, c := range cases {
		if got := cmdSetupCommand(c.goos, c.cli, c.db, "claude"); got != c.want {
			t.Errorf("cmdSetupCommand(%s, %q, %q) = %q, want %q", c.goos, c.cli, c.db, got, c.want)
		}
	}
}

// hostileArgs are paths with characters some shell would split, expand or
// end a quote at.
var hostileArgs = []string{
	`C:\Program Files\Casebook\bin\casebook.exe`,
	`C:\Users\100%user\%PATH%\casebook.exe`,
	`C:\Users\$HOME\$(echo pwned)\$env:TEMP\casebook.exe`,
	"C:\\Users\\a`echo pwned`b\\casebook.exe",
	`C:\Users\O'Neil\it''s\casebook.exe`,
	"C:\\Users\\O\u2018Brien\u2019s\u201a\u201b\\casebook.exe",
	"C:\\Users\\\u201cquoted\u201d\\casebook.exe",
	`C:\a;b&c|d<e>f(g)h,i@j{k}\casebook.exe`,
	`/opt/a b/!x/*?[c]/~/casebook`,
}

// TestQuoteArgPOSIXShell checks with a real sh that every POSIX-quoted
// hostile path reaches the program unchanged.
func TestQuoteArgPOSIXShell(t *testing.T) {
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("sh not on PATH")
	}
	for _, s := range hostileArgs {
		q := quoteArg("linux", s)
		out, err := exec.Command(sh, "-c", "printf '%s' "+q).Output()
		if err != nil || string(out) != s {
			t.Errorf("quoteArg(linux, %q) = %s; sh read %q (%v)", s, q, out, err)
		}
	}
}

// TestQuoteArgPowerShell checks the same with PowerShell when one is
// reachable (pwsh, or powershell.exe on Windows and from WSL).
func TestQuoteArgPowerShell(t *testing.T) {
	ps := ""
	for _, name := range []string{"pwsh", "powershell.exe"} {
		if p, err := exec.LookPath(name); err == nil {
			ps = p
			break
		}
	}
	if ps == "" {
		t.Skip("no PowerShell on PATH")
	}
	for _, s := range hostileArgs {
		q := quoteArg("windows", s)
		// Base64 of each argument's UTF-8 bytes keeps the comparison free of
		// console encoding.
		script := "[Console]::Out.Write([Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes(" + q + ")))"
		out, err := exec.Command(ps, "-NoProfile", "-NonInteractive", "-Command", script).Output()
		if err != nil {
			t.Errorf("PowerShell with %s: %v", q, err)
			continue
		}
		got, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(out)))
		if err != nil || string(got) != s {
			t.Errorf("quoteArg(windows, %q) = %s; PowerShell read %q (%v)", s, q, got, err)
		}
	}
}

func TestClientConnectionsShell(t *testing.T) {
	for goos, want := range map[string]string{"windows": "PowerShell", "linux": "terminal", "darwin": "terminal"} {
		d, _ := newTestDesktop(t)
		home := t.TempDir()
		d.clients = &clientSeams{
			getenv:  func(string) string { return "" },
			home:    func() (string, error) { return home, nil },
			cliPath: func() (string, error) { return `C:\Program Files\Casebook\bin\casebook.exe`, nil },
			goos:    goos,
		}
		got, err := d.ClientConnections()
		if err != nil {
			t.Fatal(err)
		}
		if got.Shell != want {
			t.Errorf("%s: Shell = %q, want %q", goos, got.Shell, want)
		}
		if (got.Claude.SetupCommandCmd != "") != (goos == "windows") || (got.Codex.SetupCommandCmd != "") != (goos == "windows") {
			t.Errorf("%s: cmd forms = %q / %q", goos, got.Claude.SetupCommandCmd, got.Codex.SetupCommandCmd)
		}
		wantClaude, wantCodex := "", ""
		if goos == "windows" {
			wantClaude = `claude mcp add --scope user casebook -- '/mnt/c/Program Files/Casebook/bin/casebook.exe' mcp`
			wantCodex = `codex mcp add casebook -- '/mnt/c/Program Files/Casebook/bin/casebook.exe' mcp`
		}
		if got.Claude.SetupCommandWSL != wantClaude || got.Codex.SetupCommandWSL != wantCodex {
			t.Errorf("%s: WSL forms = %q / %q", goos, got.Claude.SetupCommandWSL, got.Codex.SetupCommandWSL)
		}
	}
}

// The WSL form needs a drive path: no form for a missing CLI (the bare
// name) or a UNC path, and --db keeps the Windows database path.
func TestWSLSetupCommandDesktop(t *testing.T) {
	cases := []struct{ goos, cli, db, want string }{
		{"windows", `C:\Casebook\bin\casebook.exe`, `D:\My Cases\casebook.db`, `claude mcp add --scope user casebook -- /mnt/c/Casebook/bin/casebook.exe mcp --db 'D:\My Cases\casebook.db'`},
		{"windows", "", "", ""},
		{"windows", `\\server\share\casebook.exe`, "", ""},
		{"linux", `C:\Casebook\bin\casebook.exe`, "", ""},
	}
	for _, c := range cases {
		if got := wslSetupCommand(c.goos, c.cli, c.db, "claude"); got != c.want {
			t.Errorf("wslSetupCommand(%s, %q, %q) = %q, want %q", c.goos, c.cli, c.db, got, c.want)
		}
	}
}
