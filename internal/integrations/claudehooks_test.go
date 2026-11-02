package integrations

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// hooksFixture writes .claude.json (typical.json) and settings.json (data;
// nil means no file) in a fresh CLAUDE_CONFIG_DIR and returns options with
// Hooks set, quoting for linux and the fixed clock, plus both paths.
func hooksFixture(t *testing.T, settings []byte) (opts ClaudeOptions, claudePath, settingsPath string) {
	t.Helper()
	opts, claudePath = claudeFixture(t, readFixture(t, "typical.json"))
	opts.Hooks = true
	opts.GOOS = "linux"
	settingsPath = opts.Paths.ClaudeSettings
	if settings != nil {
		if err := os.WriteFile(settingsPath, settings, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return opts, claudePath, settingsPath
}

// settingsChange returns the settings.json change of a two-file run.
func settingsChange(t *testing.T, changes []FileChange, path string) FileChange {
	t.Helper()
	if len(changes) != 2 {
		t.Fatalf("changes = %+v, want .claude.json and settings.json", changes)
	}
	if changes[1].File != path {
		t.Fatalf("second change is %s, want %s", changes[1].File, path)
	}
	return changes[1]
}

// indentedGroup is the desired group for spec as encoded at depth three
// (inside hooks.<Event>), without leading indentation.
func indentedGroup(t *testing.T, spec hookSpec, opts ClaudeOptions) string {
	t.Helper()
	var b bytes.Buffer
	if err := json.Indent(&b, desiredHookGroup(spec, testCLI, opts), "      ", "  "); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

// replaceOnce replaces old with new exactly once in s or fails.
func replaceOnce(t *testing.T, s, old, new string) string {
	t.Helper()
	if strings.Count(s, old) != 1 {
		t.Fatalf("fixture layout changed: %q found %d times", old, strings.Count(s, old))
	}
	return strings.Replace(s, old, new, 1)
}

// hookCommand is the shell-form command string of a Casebook hook handler
// (hookInvocation for a non-Windows system).
func hookCommand(cliPath, dbPath, sub string) string {
	cmd, _ := hookInvocation(cliPath, dbPath, sub, "", false, "linux")
	return cmd
}

// TestHookInvocationQuiet: --quiet-when-inactive lands on the session-start
// handler only, before any --scope flag, in both forms, and both forms are
// recognized as Casebook's.
func TestHookInvocationQuiet(t *testing.T) {
	cases := []struct {
		sub, scope, goos string
		wantCmd          string
		wantArgs         []string
	}{
		{"session-start", "", "linux", `/usr/bin/casebook hook session-start --quiet-when-inactive`, nil},
		{"session-start", ScopeMarked, "darwin", `/usr/bin/casebook hook session-start --quiet-when-inactive --scope marked`, nil},
		{"checkpoint", ScopeMarked, "linux", `/usr/bin/casebook hook checkpoint --scope marked`, nil},
		{"session-start", "", "windows", `/usr/bin/casebook`, []string{"hook", "session-start", "--quiet-when-inactive"}},
		{"session-start", ScopeMarked, "windows", `/usr/bin/casebook`, []string{"hook", "session-start", "--quiet-when-inactive", "--scope", "marked"}},
		{"checkpoint", "", "windows", `/usr/bin/casebook`, []string{"hook", "checkpoint"}},
	}
	for _, c := range cases {
		cmd, args := hookInvocation("/usr/bin/casebook", "", c.sub, c.scope, true, c.goos)
		if cmd != c.wantCmd || strings.Join(args, "\x00") != strings.Join(c.wantArgs, "\x00") {
			t.Errorf("hookInvocation(%q, %q, quiet, %q) = %q, %q; want %q, %q", c.sub, c.scope, c.goos, cmd, args, c.wantCmd, c.wantArgs)
		}
		handler, _ := marshalNoEscape(hookHandler{Type: "command", Command: cmd, Args: args, Timeout: 10})
		if sub, ok := casebookHookSub(handler); !ok || sub != c.sub {
			t.Errorf("casebookHookSub(%s) = %q, %v; want %q, true", handler, sub, ok, c.sub)
		}
	}
}

func TestHookInvocationShellForm(t *testing.T) {
	cases := []struct {
		cli, db, sub, scope, goos, want string
	}{
		{"/opt/case book/casebook", "", "checkpoint", "", "linux", `'/opt/case book/casebook' hook checkpoint`},
		{"/usr/local/bin/casebook", "", "checkpoint", "", "darwin", `/usr/local/bin/casebook hook checkpoint`},
		{"/opt/it's/casebook", "", "checkpoint", "", "linux", `'/opt/it'\''s/casebook' hook checkpoint`},
		{"/opt/a$b&c/casebook", "", "session-start", "", "linux", `'/opt/a$b&c/casebook' hook session-start`},
		{"/opt/$(id)/`id`/100%/casebook", "", "checkpoint", "", "linux", "'/opt/$(id)/`id`/100%/casebook' hook checkpoint"},
		{"/opt/case book/casebook", "/data/my db.db", "session-start", "", "linux", `'/opt/case book/casebook' --db '/data/my db.db' hook session-start`},
		{"/usr/bin/casebook", "/data/c.db", "checkpoint", "", "linux", `/usr/bin/casebook --db /data/c.db hook checkpoint`},
		{"/usr/bin/casebook", "", "checkpoint", ScopeAll, "linux", `/usr/bin/casebook hook checkpoint`},
		{"/usr/bin/casebook", "/data/c.db", "session-start", ScopeMarked, "darwin", `/usr/bin/casebook --db /data/c.db hook session-start --scope marked`},
		{"/opt/case book/casebook", "", "checkpoint", ScopeMarked, "linux", `'/opt/case book/casebook' hook checkpoint --scope marked`},
	}
	for _, c := range cases {
		got, args := hookInvocation(c.cli, c.db, c.sub, c.scope, false, c.goos)
		if got != c.want || args != nil {
			t.Errorf("hookInvocation(%q, %q, %q, %q, %q) = %s, %q; want %s, nil", c.cli, c.db, c.sub, c.scope, c.goos, got, args, c.want)
		}
		handler, _ := marshalNoEscape(hookHandler{Type: "command", Command: got, Timeout: 10})
		if sub, ok := casebookHookSub(handler); !ok || sub != c.sub {
			t.Errorf("casebookHookSub(%s) = %q, %v; want %q, true", handler, sub, ok, c.sub)
		}
	}
}

// TestHookInvocationExecForm: on Windows the handler is the exec form, the
// CLI path as command (never quoted) and the arguments as args, so neither
// Git Bash nor PowerShell ever parses it.
func TestHookInvocationExecForm(t *testing.T) {
	cases := []struct {
		cli, db, sub, scope string
		want                []string
	}{
		{`C:\Program Files\Casebook\casebook.exe`, "", "session-start", "", []string{"hook", "session-start"}},
		{`C:\Casebook\bin\casebook.exe`, `C:\Data\case book.db`, "session-start", "", []string{"--db", `C:\Data\case book.db`, "hook", "session-start"}},
		{`C:\Users\100%user\casebook.exe`, `C:\%TEMP%\c.db`, "checkpoint", "", []string{"--db", `C:\%TEMP%\c.db`, "hook", "checkpoint"}},
		{`C:\$work\a` + "`" + `b\casebook.exe`, "", "checkpoint", ScopeAll, []string{"hook", "checkpoint"}},
		{`C:\Users\O'Neil\casebook.exe`, "", "checkpoint", ScopeMarked, []string{"hook", "checkpoint", "--scope", "marked"}},
		{`C:\Users\O’Brien\casebook.exe`, `D:\x.db`, "session-start", ScopeMarked, []string{"--db", `D:\x.db`, "hook", "session-start", "--scope", "marked"}},
	}
	for _, c := range cases {
		cmd, args := hookInvocation(c.cli, c.db, c.sub, c.scope, false, "windows")
		if cmd != c.cli || strings.Join(args, "\x00") != strings.Join(c.want, "\x00") {
			t.Errorf("hookInvocation(%q, %q, %q, %q, windows) = %q, %q; want %q, %q", c.cli, c.db, c.sub, c.scope, cmd, args, c.cli, c.want)
		}
		handler, _ := marshalNoEscape(hookHandler{Type: "command", Command: cmd, Args: args, Timeout: 10})
		if sub, ok := casebookHookSub(handler); !ok || sub != c.sub {
			t.Errorf("casebookHookSub(%s) = %q, %v; want %q, true", handler, sub, ok, c.sub)
		}
		if !strings.Contains(string(handler), `"command":`) || !strings.Contains(string(handler), `"args":[`) {
			t.Errorf("handler %s lacks command or args", handler)
		}
	}
}

// TestCasebookHookSubForms: every handler form earlier setups wrote, and the
// hand-written variants the docs show, is recognized so a rerun of setup
// replaces it; near misses are the user's.
func TestCasebookHookSubForms(t *testing.T) {
	ours := map[string]string{
		// Shell forms: unquoted, single-quoted (setups since the quoting
		// change), double-quoted (earlier Windows setups).
		`{"type":"command","command":"/usr/bin/casebook hook checkpoint"}`:                                                       "checkpoint",
		`{"type":"command","command":"'C:\\Program Files\\Casebook\\bin\\casebook.exe' hook session-start"}`:                     "session-start",
		`{"type":"command","command":"\"C:\\Program Files\\Casebook\\bin\\casebook.exe\" --db \"C:\\d.db\" hook checkpoint"}`:    "checkpoint",
		`{"type":"command","command":"'/mnt/c/Users/Jo Doe/casebook.exe' hook session-start"}`:                                   "session-start",
		`{"type":"command","command":"/usr/bin/casebook hook session-start --scope marked"}`:                                     "session-start",
		`{"type":"command","command":"/usr/bin/casebook hook checkpoint --scope=all"}`:                                           "checkpoint",
		`{"type":"command","command":"C:\\casebook\\casebook.exe","args":["hook","checkpoint"],"timeout":10}`:                    "checkpoint",
		`{"type":"command","command":"C:\\casebook.exe","args":["--db","C:\\x y.db","hook","session-start","--scope","marked"]}`: "session-start",
		`{"type":"command","command":"C:\\casebook.exe","args":["hook","session-start","--scope=marked"]}`:                       "session-start",
		// The hint flags: the quiet one setup writes, the opt-in one older
		// hook commands carried, by hand after the scope, and in exec form.
		`{"type":"command","command":"/usr/bin/casebook hook session-start --quiet-when-inactive"}`:                                  "session-start",
		`{"type":"command","command":"/usr/bin/casebook hook session-start --hint-when-inactive"}`:                                   "session-start",
		`{"type":"command","command":"/usr/bin/casebook hook session-start --scope marked --quiet-when-inactive"}`:                   "session-start",
		`{"type":"command","command":"C:\\casebook.exe","args":["hook","session-start","--quiet-when-inactive","--scope","marked"]}`: "session-start",
	}
	for h, want := range ours {
		if sub, ok := casebookHookSub(json.RawMessage(h)); !ok || sub != want {
			t.Errorf("casebookHookSub(%s) = %q, %v; want %q, true", h, sub, ok, want)
		}
	}
	for _, h := range []string{
		`{"type":"command","command":"~/bin/greet.sh"}`,
		`{"type":"command","command":"/usr/bin/other hook checkpoint"}`,
		`{"type":"command","command":"casebook hook checkpoint --verbose"}`,
		`{"type":"command","command":"casebook hook checkpoint --scope everywhere"}`,
		`{"type":"command","command":"casebook hook session-start --quiet"}`,
		`{"type":"command","command":7}`,
		`{"type":"prompt","prompt":"casebook hook checkpoint"}`,
		`{"type":"command","command":"casebook","args":["hook","checkpoint","--verbose"]}`,
		`{"type":"command","command":"casebook","args":["checkpoint"]}`,
		`{"type":"command","command":"casebook","args":"hook checkpoint"}`,
		`{"type":"command","command":"casebook","args":["hook",7]}`,
		`{"type":"command","command":"other.exe","args":["hook","checkpoint"]}`,
		`{"type":"command","command":"casebook hook checkpoint","args":[]}`,
		`"casebook hook checkpoint"`,
		`[]`,
	} {
		if sub, ok := casebookHookSub(json.RawMessage(h)); ok {
			t.Errorf("casebookHookSub(%s) = %q, true; want a user handler", h, sub)
		}
	}
}

// TestQuotePOSIXBashLiteral runs every quoted form through bash, the shell
// Claude Code uses for shell-form hooks on macOS and Linux (and that WSL
// users paste setup commands into), and checks that bash passes on exactly
// the original text.
func TestQuotePOSIXBashLiteral(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not on PATH")
	}
	hostile := []string{
		`C:\Program Files\Casebook\bin\casebook.exe`,
		`C:\Users\100%user\%PATH%\casebook.exe`,
		`C:\Users\$HOME\$(echo pwned)\casebook.exe`,
		"C:\\Users\\a`echo pwned`b\\casebook.exe",
		`C:\Users\O'Neil\it''s\casebook.exe`,
		`C:\a;b&c|d<e>f(g)h\casebook.exe`,
		`/opt/a b/!x/*?[c]/~/casebook`,
		"/opt/tab\there/casebook",
		"/usr/bin/casebook",
		"",
	}
	for _, s := range hostile {
		q := QuotePOSIX(s)
		out, err := exec.Command(bash, "--norc", "--noprofile", "-c", "printf '%s' "+q).Output()
		if err != nil {
			t.Errorf("bash -c with %s: %v", q, err)
			continue
		}
		if string(out) != s {
			t.Errorf("QuotePOSIX(%q) = %s; bash read %q", s, q, out)
		}
	}
}

// TestInstallHooksWindowsReplacesShellForms: a Windows setup over the
// shell-form handlers earlier setups wrote (double quotes, then single
// quotes) rewrites each in place to the exec form, keeps the user's own
// handlers, and a rerun changes nothing.
func TestInstallHooksWindowsReplacesShellForms(t *testing.T) {
	const cli = `C:\Program Files\Casebook\bin\casebook.exe`
	old := `{
  "hooks": {
    "SessionStart": [
      {"matcher": "startup", "hooks": [{"type": "command", "command": "\"C:\\Program Files\\Casebook\\bin\\casebook.exe\" hook session-start", "timeout": 20}]}
    ],
    "Stop": [
      {"hooks": [{"type": "command", "command": "notify-send done"}]},
      {"hooks": [{"type": "command", "command": "'C:\\Program Files\\Casebook\\bin\\casebook.exe' hook checkpoint", "timeout": 10}]}
    ]
  }
}
`
	opts, _, path := hooksFixture(t, []byte(old))
	opts.GOOS = "windows"
	opts.HookScope = ScopeMarked
	if _, err := SetupClaude(cli, opts); err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Hooks map[string][]struct {
			Matcher string `json:"matcher"`
			Hooks   []struct {
				Command string   `json:"command"`
				Args    []string `json:"args"`
				Timeout int      `json:"timeout"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(readBytes(t, path), &doc); err != nil {
		t.Fatal(err)
	}
	ss := doc.Hooks["SessionStart"]
	if len(ss) != 1 || ss[0].Matcher != "startup" || len(ss[0].Hooks) != 1 || ss[0].Hooks[0].Command != cli ||
		strings.Join(ss[0].Hooks[0].Args, " ") != "hook session-start --scope marked" || ss[0].Hooks[0].Timeout != 20 {
		t.Errorf("SessionStart = %+v, want the exec form keeping matcher and timeout", ss)
	}
	stop := doc.Hooks["Stop"]
	if len(stop) != 2 || stop[0].Hooks[0].Command != "notify-send done" || stop[0].Hooks[0].Args != nil ||
		stop[1].Hooks[0].Command != cli || strings.Join(stop[1].Hooks[0].Args, " ") != "hook checkpoint --scope marked" {
		t.Errorf("Stop = %+v, want the user group and the exec form", stop)
	}
	if pc := doc.Hooks["PreCompact"]; len(pc) != 1 || pc[0].Hooks[0].Command != cli {
		t.Errorf("PreCompact = %+v", pc)
	}
	first := readBytes(t, path)
	changes, err := SetupClaude(cli, opts)
	if err != nil {
		t.Fatal(err)
	}
	if c := settingsChange(t, changes, path); c.Action != ActionUnchanged || !bytes.Equal(readBytes(t, path), first) {
		t.Errorf("rerun = %+v, want unchanged", c)
	}
	if changes, err := RemoveClaude(opts); err != nil || changes[1].Action != ActionRemoved {
		t.Fatalf("remove = %+v, %v", changes, err)
	}
	if strings.Contains(string(readBytes(t, path)), "casebook.exe") {
		t.Errorf("remove left a Casebook handler:\n%s", readBytes(t, path))
	}
}

// TestMergeHookFormSwitch: rerunning setup on the other form rewrites the
// Casebook handler in place, adding or dropping args, and keeps the user's
// timeout and extra members.
func TestMergeHookFormSwitch(t *testing.T) {
	spec := casebookHookSpecs[1]
	oldShell := json.RawMessage(`{"hooks":[{"type":"command","command":"'C:\\cb\\casebook.exe' hook checkpoint","timeout":30,"statusMessage":"x"}]}`)
	win := ClaudeOptions{GOOS: "windows", HookScope: ScopeMarked}
	got := string(mergedHookGroup(oldShell, spec, `C:\cb\casebook.exe`, win))
	want := `{"hooks":[{"type":"command","command":"C:\\cb\\casebook.exe","timeout":30,"statusMessage":"x","args":["hook","checkpoint","--scope","marked"]}]}`
	if got != want {
		t.Errorf("shell to exec:\n got %s\nwant %s", got, want)
	}
	back := string(mergedHookGroup(json.RawMessage(got), spec, "/usr/bin/casebook", ClaudeOptions{GOOS: "linux"}))
	want = `{"hooks":[{"type":"command","command":"/usr/bin/casebook hook checkpoint","timeout":30,"statusMessage":"x"}]}`
	if back != want {
		t.Errorf("exec to shell:\n got %s\nwant %s", back, want)
	}
}

func TestInstallHooksTypical(t *testing.T) {
	orig := readFixture(t, "settings-typical.json")
	opts, claudePath, path := hooksFixture(t, orig)
	changes, err := SetupClaude(testCLI, opts)
	if err != nil {
		t.Fatal(err)
	}
	if changes[0].File != claudePath || changes[0].Action != ActionAdded {
		t.Errorf("first change = %+v, want .claude.json added", changes[0])
	}
	c := settingsChange(t, changes, path)
	if c.Action != ActionAdded || c.Note != "" ||
		strings.Join(c.Entries, ",") != "hooks.SessionStart,hooks.Stop,hooks.PreCompact" {
		t.Errorf("settings change = %+v", c)
	}
	if c.Backup == "" || !bytes.Equal(readBytes(t, c.Backup), orig) {
		t.Errorf("backup %q does not hold the original bytes", c.Backup)
	}

	// Independent expectation: the original text with one group appended to
	// SessionStart and Stop and a PreCompact array added at the end of hooks.
	want := string(orig)
	want = replaceOnce(t, want, "      }\n    ],\n    \"Stop\": [",
		"      },\n      "+indentedGroup(t, casebookHookSpecs[0], opts)+"\n    ],\n    \"Stop\": [")
	want = replaceOnce(t, want, "      }\n    ],\n    \"PostToolUse\": [",
		"      },\n      "+indentedGroup(t, casebookHookSpecs[1], opts)+"\n    ],\n    \"PostToolUse\": [")
	want = replaceOnce(t, want, "    ]\n  },\n  \"statusLine\"",
		"    ],\n    \"PreCompact\": [\n      "+indentedGroup(t, casebookHookSpecs[2], opts)+"\n    ]\n  },\n  \"statusLine\"")
	got := readBytes(t, path)
	if string(got) != want {
		t.Fatalf("install changed more than the Casebook groups; %s", firstDiff(got, []byte(want)))
	}
	if !strings.Contains(string(got), `"command": "'/opt/Case Book/bin/casebook' hook session-start"`) {
		t.Errorf("quoted command not found in:\n%s", got)
	}
}

func TestInstallHooksIdempotent(t *testing.T) {
	opts, _, path := hooksFixture(t, readFixture(t, "settings-typical.json"))
	if _, err := SetupClaude(testCLI, opts); err != nil {
		t.Fatal(err)
	}
	first := readBytes(t, path)
	before := leftovers(t, filepath.Dir(path))
	changes, err := SetupClaude(testCLI, opts)
	if err != nil {
		t.Fatal(err)
	}
	if c := settingsChange(t, changes, path); c.Action != ActionUnchanged || c.Backup != "" {
		t.Errorf("second run = %+v, want unchanged without backup", c)
	}
	if !bytes.Equal(readBytes(t, path), first) {
		t.Error("second run changed settings.json")
	}
	if after := leftovers(t, filepath.Dir(path)); len(after) != len(before) {
		t.Errorf("second run left %v (before %v)", after, before)
	}
}

func TestInstallHooksMissingFile(t *testing.T) {
	opts, _, path := hooksFixture(t, nil)
	changes, err := SetupClaude(testCLI, opts)
	if err != nil {
		t.Fatal(err)
	}
	if c := settingsChange(t, changes, path); c.Action != ActionAdded || c.Backup != "" {
		t.Errorf("change = %+v, want added without backup", c)
	}
	var doc map[string]map[string][]json.RawMessage
	if err := json.Unmarshal(readBytes(t, path), &doc); err != nil {
		t.Fatal(err)
	}
	for _, s := range casebookHookSpecs {
		if g := doc["hooks"][s.Event]; len(g) != 1 || !sameJSON(g[0], desiredHookGroup(s, testCLI, opts)) {
			t.Errorf("hooks.%s = %s", s.Event, g)
		}
	}
	wantMode(t, path, 0o600)
}

func TestInstallHooksReplacesStale(t *testing.T) {
	stale := func(cmd string) string {
		return `{"hooks": [{"type": "command", "command": "` + cmd + `", "timeout": 10}]}`
	}
	user := `{"hooks": [{"type": "command", "command": "notify-send done"}]}`
	t.Run("in place", func(t *testing.T) {
		doc := `{"hooks": {"Stop": [` + user + `, ` + stale("/old/casebook hook checkpoint") + `, ` + user + `]}}`
		opts, _, path := hooksFixture(t, []byte(doc))
		if _, err := SetupClaude(testCLI, opts); err != nil {
			t.Fatal(err)
		}
		changes, err := SetupClaude(testCLI, opts)
		if err != nil {
			t.Fatal(err)
		}
		if c := settingsChange(t, changes, path); c.Action != ActionUnchanged {
			t.Errorf("rerun = %+v", c)
		}
		var got struct {
			Hooks map[string][]json.RawMessage `json:"hooks"`
		}
		if err := json.Unmarshal(readBytes(t, path), &got); err != nil {
			t.Fatal(err)
		}
		stop := got.Hooks["Stop"]
		if len(stop) != 3 || !sameJSON(stop[0], json.RawMessage(user)) || !sameJSON(stop[2], json.RawMessage(user)) ||
			!sameJSON(stop[1], desiredHookGroup(casebookHookSpecs[1], testCLI, opts)) {
			t.Errorf("Stop = %s, want the stale group replaced at index 1", stop)
		}
	})
	t.Run("updated action", func(t *testing.T) {
		doc := `{"hooks": {"Stop": [` + stale("/old/casebook hook checkpoint") + `]}}`
		opts, _, path := hooksFixture(t, []byte(doc))
		changes, err := SetupClaude(testCLI, opts)
		if err != nil {
			t.Fatal(err)
		}
		if c := settingsChange(t, changes, path); c.Action != ActionUpdated || c.Backup == "" {
			t.Errorf("change = %+v, want updated with a backup", c)
		}
	})
	t.Run("hand-added quiet flag is replaced, not duplicated", func(t *testing.T) {
		doc := `{"hooks": {"SessionStart": [{"matcher": "startup|resume|clear|compact", "hooks": [{"type": "command", "command": "` +
			QuotePOSIX(testCLI) + ` hook session-start --quiet-when-inactive", "timeout": 10}]}]}}`
		opts, _, path := hooksFixture(t, []byte(doc))
		if _, err := SetupClaude(testCLI, opts); err != nil {
			t.Fatal(err)
		}
		var got struct {
			Hooks map[string][]hookGroup `json:"hooks"`
		}
		if err := json.Unmarshal(readBytes(t, path), &got); err != nil {
			t.Fatal(err)
		}
		ss := got.Hooks["SessionStart"]
		if len(ss) != 1 || len(ss[0].Hooks) != 1 || ss[0].Hooks[0].Command != QuotePOSIX(testCLI)+" hook session-start" {
			t.Errorf("SessionStart = %+v, want one default handler", ss)
		}
		quiet := opts
		quiet.HookQuiet = true
		if _, err := SetupClaude(testCLI, quiet); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(readBytes(t, path), &got); err != nil {
			t.Fatal(err)
		}
		ss = got.Hooks["SessionStart"]
		if len(ss) != 1 || len(ss[0].Hooks) != 1 || ss[0].Hooks[0].Command != QuotePOSIX(testCLI)+" hook session-start --quiet-when-inactive" {
			t.Errorf("SessionStart with HookQuiet = %+v, want one quiet handler", ss)
		}
		if stop := got.Hooks["Stop"]; len(stop) != 1 || stop[0].Hooks[0].Command != QuotePOSIX(testCLI)+" hook checkpoint" {
			t.Errorf("Stop with HookQuiet = %+v, want the plain checkpoint handler", stop)
		}
	})
	t.Run("duplicates collapse", func(t *testing.T) {
		doc := `{"hooks": {"Stop": [` + stale("/a/casebook hook checkpoint") + `, ` + user + `, ` +
			stale("/b/casebook hook checkpoint") + `]}}`
		opts, _, path := hooksFixture(t, []byte(doc))
		if _, err := SetupClaude(testCLI, opts); err != nil {
			t.Fatal(err)
		}
		var got struct {
			Hooks map[string][]json.RawMessage `json:"hooks"`
		}
		if err := json.Unmarshal(readBytes(t, path), &got); err != nil {
			t.Fatal(err)
		}
		stop := got.Hooks["Stop"]
		if len(stop) != 2 || !sameJSON(stop[0], desiredHookGroup(casebookHookSpecs[1], testCLI, opts)) ||
			!sameJSON(stop[1], json.RawMessage(user)) {
			t.Errorf("Stop = %s, want one Casebook group then the user group", stop)
		}
	})
}

func TestInstallHooksMixed(t *testing.T) {
	orig := readFixture(t, "settings-mixed.json")
	opts, _, path := hooksFixture(t, orig)
	changes, err := SetupClaude(testCLI, opts)
	if err != nil {
		t.Fatal(err)
	}
	c := settingsChange(t, changes, path)
	if c.Action != ActionUpdated || !strings.Contains(c.Note, "hooks.Stop") {
		t.Errorf("change = %+v, want updated with a note naming hooks.Stop", c)
	}
	got := readBytes(t, path)
	o, err := parseJSONObject(got)
	if err != nil {
		t.Fatal(err)
	}
	hooks, _, err := o.childObject("hooks")
	if err != nil {
		t.Fatal(err)
	}
	stop, _ := hooks.Get("Stop")
	oo, _ := parseJSONObject(orig)
	oh, _, _ := oo.childObject("hooks")
	origStop, _ := oh.Get("Stop")
	if !sameJSON(stop, origStop) {
		t.Errorf("Stop changed from %s to %s", origStop, stop)
	}
	var keys []string
	for _, m := range hooks.Members {
		keys = append(keys, m.Key)
	}
	if strings.Join(keys, ",") != "Stop,SessionStart,PreCompact" {
		t.Errorf("hooks keys = %v", keys)
	}
}

func TestInstallHooksOddShapes(t *testing.T) {
	odd := []string{`"just a string"`, `{"matcher": "x"}`, `{"hooks": "not an array"}`, `{"hooks": []}`, `7`}
	doc := "{\n  \"hooks\": {\n    \"Stop\": [" + strings.Join(odd, ", ") + "]\n  }\n}\n"
	opts, _, path := hooksFixture(t, []byte(doc))
	if _, err := SetupClaude(testCLI, opts); err != nil {
		t.Fatal(err)
	}
	var got struct {
		Hooks map[string][]json.RawMessage `json:"hooks"`
	}
	if err := json.Unmarshal(readBytes(t, path), &got); err != nil {
		t.Fatal(err)
	}
	stop := got.Hooks["Stop"]
	if len(stop) != len(odd)+1 {
		t.Fatalf("Stop = %s", stop)
	}
	for i, o := range odd {
		if !sameJSON(stop[i], json.RawMessage(o)) {
			t.Errorf("Stop[%d] = %s, want %s untouched", i, stop[i], o)
		}
	}
}

func TestInstallHooksRefuses(t *testing.T) {
	for _, c := range []struct{ name, doc, reason string }{
		{"invalid JSON", `{"hooks": {`, "not valid JSON"},
		{"hooks is an array", `{"hooks": []}`, "hooks is not a JSON object"},
		{"Stop is an object", `{"hooks": {"Stop": {"hooks": []}}}`, "hooks.Stop is not an array"},
		{"PreCompact is a string", `{"hooks": {"PreCompact": "casebook hook checkpoint"}}`, "hooks.PreCompact is not an array"},
	} {
		t.Run(c.name, func(t *testing.T) {
			opts, claudePath, path := hooksFixture(t, []byte(c.doc))
			claudeBefore := readBytes(t, claudePath)
			changes, err := SetupClaude(testCLI, opts)
			if changes != nil {
				t.Errorf("changes = %+v on refusal", changes)
			}
			wantRefused(t, err, path, c.doc, c.reason)
			if !bytes.Equal(readBytes(t, claudePath), claudeBefore) {
				t.Error(".claude.json was written although settings.json was refused")
			}
		})
	}
	t.Run("refused .claude.json writes no settings", func(t *testing.T) {
		opts, path := claudeFixture(t, []byte(`{"mcpServers": []}`))
		opts.Hooks = true
		if _, err := SetupClaude(testCLI, opts); err == nil {
			t.Fatal("want a refusal")
		}
		if _, err := os.Lstat(opts.Paths.ClaudeSettings); !os.IsNotExist(err) {
			t.Errorf("settings.json written (%v)", err)
		}
		if l := leftovers(t, filepath.Dir(path)); len(l) != 0 {
			t.Errorf("leftovers = %v", l)
		}
	})
}

func TestRemoveHooks(t *testing.T) {
	t.Run("install then remove keeps everything but Casebook's handlers", func(t *testing.T) {
		orig := readFixture(t, "settings-typical.json")
		opts, claudePath, path := hooksFixture(t, orig)
		// The fixture has no PreCompact event: install adds one and remove
		// leaves it as an empty array (containers are never deleted; WR-03).
		want := []byte(replaceOnce(t, string(orig), "    ]\n  },\n  \"statusLine\"", "    ],\n    \"PreCompact\": []\n  },\n  \"statusLine\""))
		claudeOrig := readBytes(t, claudePath)
		if _, err := SetupClaude(testCLI, opts); err != nil {
			t.Fatal(err)
		}
		changes, err := RemoveClaude(opts)
		if err != nil {
			t.Fatal(err)
		}
		c := settingsChange(t, changes, path)
		if c.Action != ActionRemoved || strings.Join(c.Entries, ",") != "hooks.SessionStart,hooks.Stop,hooks.PreCompact" {
			t.Errorf("settings change = %+v", c)
		}
		if changes[0].File != claudePath || changes[0].Action != ActionRemoved {
			t.Errorf(".claude.json change = %+v", changes[0])
		}
		if got := readBytes(t, path); !bytes.Equal(got, want) {
			t.Fatalf("settings.json not restored; %s", firstDiff(got, want))
		}
		if got := readBytes(t, claudePath); !bytes.Equal(got, claudeOrig) {
			t.Fatalf(".claude.json not restored; %s", firstDiff(got, claudeOrig))
		}
	})
	t.Run("mixed group keeps the user handler", func(t *testing.T) {
		opts, _, path := hooksFixture(t, readFixture(t, "settings-mixed.json"))
		changes, err := RemoveClaude(opts)
		if err != nil {
			t.Fatal(err)
		}
		if c := settingsChange(t, changes, path); c.Action != ActionRemoved || strings.Join(c.Entries, ",") != "hooks.Stop" {
			t.Errorf("change = %+v", c)
		}
		want := "{\n  \"hooks\": {\n    \"Stop\": [\n      {\n        \"matcher\": \"\",\n        \"hooks\": [\n" +
			"          {\n            \"type\": \"command\",\n            \"command\": \"notify-send done\"\n          }\n" +
			"        ]\n      }\n    ]\n  },\n  \"theme\": \"dark\"\n}\n"
		if got := readBytes(t, path); string(got) != want {
			t.Fatalf("settings.json; %s", firstDiff(got, []byte(want)))
		}
	})
	t.Run("only Casebook hooks leaves empty containers", func(t *testing.T) {
		// Casebook cannot tell an event array or hooks object it created from
		// one the user created, so remove never deletes either (WR-03).
		emptied := "  \"hooks\": {\n    \"SessionStart\": [],\n    \"Stop\": [],\n    \"PreCompact\": []\n  }"
		for _, c := range []struct{ orig, want string }{
			{"{\n  \"theme\": \"dark\"\n}\n", "{\n  \"theme\": \"dark\",\n" + emptied + "\n}\n"},
			{"{\n  \"hooks\": {}\n}\n", "{\n" + emptied + "\n}\n"},
			{"{\n  \"hooks\": {\n    \"Stop\": []\n  }\n}\n", "{\n  \"hooks\": {\n    \"Stop\": [],\n    \"SessionStart\": [],\n    \"PreCompact\": []\n  }\n}\n"},
		} {
			opts, _, path := hooksFixture(t, []byte(c.orig))
			if _, err := SetupClaude(testCLI, opts); err != nil {
				t.Fatal(err)
			}
			if _, err := RemoveClaude(opts); err != nil {
				t.Fatal(err)
			}
			if got := string(readBytes(t, path)); got != c.want {
				t.Errorf("%q: settings.json = %q, want %q", c.orig, got, c.want)
			}
		}
	})
	t.Run("user event with a Casebook handler", func(t *testing.T) {
		doc := `{"hooks": {"PostToolUse": [{"hooks": [{"type": "command", "command": "casebook hook checkpoint"}]}], "Stop": []}}`
		opts, _, path := hooksFixture(t, []byte(doc))
		changes, err := RemoveClaude(opts)
		if err != nil {
			t.Fatal(err)
		}
		if c := settingsChange(t, changes, path); strings.Join(c.Entries, ",") != "hooks.PostToolUse" {
			t.Errorf("entries = %v", c.Entries)
		}
		// The user's empty Stop array stays, and so does the emptied
		// PostToolUse array.
		want := "{\n  \"hooks\": {\n    \"PostToolUse\": [],\n    \"Stop\": []\n  }\n}\n"
		if got := string(readBytes(t, path)); got != want {
			t.Errorf("settings.json = %q, want %q", got, want)
		}
	})
	t.Run("missing settings", func(t *testing.T) {
		opts, _, path := hooksFixture(t, nil)
		changes, err := RemoveClaude(opts)
		if err != nil {
			t.Fatal(err)
		}
		if c := settingsChange(t, changes, path); c.Action != ActionUnchanged || len(c.Entries) != 0 {
			t.Errorf("change = %+v", c)
		}
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Errorf("remove created %s", path)
		}
	})
	t.Run("nothing to remove", func(t *testing.T) {
		orig := readFixture(t, "settings-typical.json")
		opts, _, path := hooksFixture(t, orig)
		changes, err := RemoveClaude(opts)
		if err != nil {
			t.Fatal(err)
		}
		if c := settingsChange(t, changes, path); c.Action != ActionUnchanged || c.Backup != "" {
			t.Errorf("change = %+v", c)
		}
		if !bytes.Equal(readBytes(t, path), orig) {
			t.Error("settings.json changed")
		}
	})
	t.Run("refusals write nothing", func(t *testing.T) {
		for _, c := range []struct{ doc, reason string }{
			{`{"hooks": [`, "not valid JSON"},
			{`{"hooks": "x"}`, "hooks is not a JSON object"},
			{`{"hooks": {"SessionStart": {}}}`, "hooks.SessionStart is not an array"},
		} {
			opts, claudePath, path := hooksFixture(t, []byte(c.doc))
			if _, err := SetupClaude(testCLI, ClaudeOptions{Paths: opts.Paths, Now: opts.Now}); err != nil {
				t.Fatal(err)
			}
			claudeBefore := readBytes(t, claudePath)
			for _, l := range leftovers(t, filepath.Dir(path)) {
				os.Remove(filepath.Join(filepath.Dir(path), l))
			}
			changes, err := RemoveClaude(opts)
			if changes != nil {
				t.Errorf("%s: changes = %+v on refusal", c.doc, changes)
			}
			wantRefused(t, err, path, c.doc, c.reason)
			if !bytes.Equal(readBytes(t, claudePath), claudeBefore) {
				t.Errorf("%s: .claude.json was written although settings.json was refused", c.doc)
			}
		}
	})
}

func TestClaudeConfigPreviewHooks(t *testing.T) {
	opts, _, settingsPath := hooksFixture(t, nil)
	base := opts
	base.Hooks = false
	got := ClaudeConfigPreview(testCLI, opts)
	q := `'/opt/Case Book/bin/casebook'`
	want := ClaudeConfigPreview(testCLI, base) + "\n" +
		`Claude Code hooks: merge into the top-level "hooks" object of ` + settingsPath + `
{
  "hooks": {
    "SessionStart": [
      {
        "matcher": "startup|resume|clear|compact",
        "hooks": [
          {
            "type": "command",
            "command": "` + q + ` hook session-start",
            "timeout": 10
          }
        ]
      }
    ],
    "Stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "` + q + ` hook checkpoint",
            "timeout": 10
          }
        ]
      }
    ],
    "PreCompact": [
      {
        "matcher": "manual|auto",
        "hooks": [
          {
            "type": "command",
            "command": "` + q + ` hook checkpoint",
            "timeout": 10
          }
        ]
      }
    ]
  }
}
`
	if got != want {
		t.Errorf("preview:\n%s\nwant:\n%s", got, want)
	}
	if _, err := os.Lstat(settingsPath); !os.IsNotExist(err) {
		t.Errorf("preview wrote %s", settingsPath)
	}
}

// TestClaudeSecondWriteFails covers a two-file run whose settings.json write
// fails after .claude.json was written (WR-07): the .claude.json change is
// returned and the error names it and its backup.
func TestClaudeSecondWriteFails(t *testing.T) {
	failSettings := func(t *testing.T, settingsPath string) {
		beforeRename = func(p string) {
			if p == settingsPath {
				if err := os.WriteFile(p, []byte("{\"concurrent\": true}\n"), 0o600); err != nil {
					t.Error(err)
				}
			}
		}
		t.Cleanup(func() { beforeRename = nil })
	}
	t.Run("setup", func(t *testing.T) {
		opts, claudePath, path := hooksFixture(t, readFixture(t, "settings-typical.json"))
		failSettings(t, path)
		changes, err := SetupClaude(testCLI, opts)
		if err == nil {
			t.Fatal("want an error")
		}
		if len(changes) != 1 || changes[0].File != claudePath || changes[0].Action != ActionAdded || changes[0].Backup == "" {
			t.Fatalf("changes = %+v, want the .claude.json change", changes)
		}
		for _, want := range []string{claudePath + " was added", "backup " + changes[0].Backup, path + " was not changed", "changed while Casebook was updating it"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("err = %q, want it to contain %q", err, want)
			}
		}
	})
	t.Run("remove", func(t *testing.T) {
		opts, claudePath, path := hooksFixture(t, readFixture(t, "settings-typical.json"))
		if _, err := SetupClaude(testCLI, opts); err != nil {
			t.Fatal(err)
		}
		failSettings(t, path)
		changes, err := RemoveClaude(opts)
		if err == nil {
			t.Fatal("want an error")
		}
		if len(changes) != 1 || changes[0].File != claudePath || changes[0].Action != ActionRemoved || changes[0].Backup == "" {
			t.Fatalf("changes = %+v, want the .claude.json change", changes)
		}
		if !strings.Contains(err.Error(), claudePath+" was removed (backup "+changes[0].Backup+")") {
			t.Errorf("err = %q", err)
		}
	})
	t.Run("first file unchanged", func(t *testing.T) {
		opts, _, path := hooksFixture(t, readFixture(t, "settings-typical.json"))
		if _, err := SetupClaude(testCLI, ClaudeOptions{Paths: opts.Paths, Now: opts.Now}); err != nil {
			t.Fatal(err)
		}
		failSettings(t, path)
		changes, err := SetupClaude(testCLI, opts)
		if err == nil || changes != nil || strings.Contains(err.Error(), "was not changed") {
			t.Errorf("changes = %+v, err = %v; want only the settings.json error", changes, err)
		}
	})
}
