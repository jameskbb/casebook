package doctor

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/app/apptest"
	"github.com/jameskbb/casebook/internal/integrations"
)

// wantOrder is the fixed check order of every report.
var wantOrder = []string{
	"data_dir", "database", "integrity", "migrations", "write_access",
	"mcp_startup", "cli_path", "bundle_version", "claude_config", "codex_config", "claude_cli", "codex_cli",
}

// doctorFixture is one isolated doctor setup.
type doctorFixture struct {
	Opts    Options
	Looked  []string // names the fake LookPath was asked for
	MCPRuns int
}

// newDoctorFixture returns options for dbPath with temp client config
// paths, an executable stand-in CLI at <dir>/bin/casebook, a fake MCP check
// that succeeds (with TempDB when dbPath is missing) and a fake LookPath
// that finds every name in found.
func newDoctorFixture(t *testing.T, dbPath string, found ...string) *doctorFixture {
	t.Helper()
	claude, codex := t.TempDir(), t.TempDir()
	getenv := func(k string) string {
		switch k {
		case "CLAUDE_CONFIG_DIR":
			return claude
		case "CODEX_HOME":
			return codex
		}
		return ""
	}
	f := &doctorFixture{}
	f.Opts = Options{
		DBPath:  dbPath,
		CLIPath: doctorCLI(t, t.TempDir()),
		Paths:   integrations.ResolvePaths(getenv, t.TempDir()),
		Version: "v9.9.9-test",
		LookPath: func(name string) (string, error) {
			f.Looked = append(f.Looked, name)
			if slices.Contains(found, name) {
				return "/usr/local/bin/" + name, nil
			}
			return "", errors.New("not found")
		},
		MCPCheck: func(_ context.Context, cliPath, db string) (MCPResult, error) {
			f.MCPRuns++
			_, err := os.Stat(db)
			return MCPResult{ServerName: "casebook", ServerVersion: "v9.9.9-test", Tools: 20, TempDB: err != nil}, nil
		},
	}
	return f
}

// doctorCLI writes an executable stand-in casebook at <root>/bin/casebook
// and returns its symlink-resolved path.
func doctorCLI(t *testing.T, root string) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(root, "bin", "casebook")
	if runtime.GOOS == "windows" {
		p += ".exe"
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

// doctorSeeded returns the path of a seeded database (its App stays open
// for the test, which is supported).
func doctorSeeded(t *testing.T) (string, *app.App) {
	t.Helper()
	a := apptest.Open(t)
	apptest.SeedCloudSpend(t, a)
	return a.DBPath(), a
}

// checkByID returns the check with id or fails.
func checkByID(t *testing.T, r Report, id string) Check {
	t.Helper()
	for _, c := range r.Checks {
		if c.ID == id {
			return c
		}
	}
	t.Fatalf("no %s check in %+v", id, r.Checks)
	return Check{}
}

// wantCheck fails when the check's status differs or its detail lacks any
// of subs.
func wantCheck(t *testing.T, r Report, id string, s Status, subs ...string) {
	t.Helper()
	c := checkByID(t, r, id)
	if c.Status != s {
		t.Errorf("%s = %s (%s), want %s", id, c.Status, c.Detail, s)
	}
	for _, sub := range subs {
		if !strings.Contains(c.Detail, sub) {
			t.Errorf("%s detail = %q, want it to contain %q", id, c.Detail, sub)
		}
	}
}

// doctorSetupClients writes Casebook's Claude (with hooks) and Codex config
// for the fixture's CLI.
func doctorSetupClients(t *testing.T, f *doctorFixture) {
	t.Helper()
	if _, err := integrations.SetupClaude(f.Opts.CLIPath, integrations.ClaudeOptions{Paths: f.Opts.Paths, Hooks: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := integrations.SetupCodex(f.Opts.CLIPath, integrations.CodexOptions{Paths: f.Opts.Paths}); err != nil {
		t.Fatal(err)
	}
}

// doctorWrite writes data to path, creating its directory.
func doctorWrite(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
}

// doctorQuote encodes s as a JSON (and TOML basic) string.
func doctorQuote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func TestRunHealthy(t *testing.T) {
	db, _ := doctorSeeded(t)
	f := newDoctorFixture(t, db, "claude", "codex")
	doctorSetupClients(t, f)
	// An installed layout: the desktop app next to bin/ (AR-20 warns
	// otherwise).
	desktop := "Casebook"
	if runtime.GOOS == "windows" {
		desktop += ".exe"
	}
	// The release stamp matches the CLI's version (bundle_version).
	doctorWrite(t, filepath.Join(filepath.Dir(filepath.Dir(f.Opts.CLIPath)), desktop), "desktop casebook-version=v9.9.9-test\x00")
	r := Run(context.Background(), f.Opts)
	for _, c := range r.Checks {
		if c.Status != StatusOK {
			t.Errorf("%s = %s (%s), want ok", c.ID, c.Status, c.Detail)
		}
	}
	if !r.OK || r.Failed() != 0 || r.Version != "v9.9.9-test" {
		t.Fatalf("report ok=%v failed=%d version=%q", r.OK, r.Failed(), r.Version)
	}
	wantCheck(t, r, "database", StatusOK, "schema ", "1 investigations", "4 evidence", "active INV-0001")
	wantCheck(t, r, "integrity", StatusOK, "integrity_check and foreign_key_check are clean")
	wantCheck(t, r, "write_access", StatusOK, "checked without opening it")
	wantCheck(t, r, "mcp_startup", StatusOK, "casebook v9.9.9-test, 20 tools")
	wantCheck(t, r, "claude_config", StatusOK, f.Opts.Paths.ClaudeJSON, "hooks: SessionStart, Stop, PreCompact")
	wantCheck(t, r, "codex_config", StatusOK, f.Opts.Paths.CodexConfig, "managed block")
	wantCheck(t, r, "claude_cli", StatusOK, "found at /usr/local/bin/claude")
	wantCheck(t, r, "codex_cli", StatusOK, "found at /usr/local/bin/codex")
	if f.MCPRuns != 1 {
		t.Fatalf("MCP check ran %d times, want 1", f.MCPRuns)
	}
}

func TestRunCheckOrder(t *testing.T) {
	for _, tc := range []struct {
		name string
		db   func(t *testing.T) string
	}{
		{"seeded", func(t *testing.T) string { db, _ := doctorSeeded(t); return db }},
		{"missing", func(t *testing.T) string { return filepath.Join(t.TempDir(), "casebook.db") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := Run(context.Background(), newDoctorFixture(t, tc.db(t)).Opts)
			var ids []string
			for _, c := range r.Checks {
				ids = append(ids, c.ID)
				if c.Name == "" || c.Detail == "" {
					t.Errorf("%s has an empty name or detail: %+v", c.ID, c)
				}
			}
			if !slices.Equal(ids, wantOrder) {
				t.Fatalf("order = %v\nwant %v", ids, wantOrder)
			}
		})
	}
}

func TestRunMissingDatabase(t *testing.T) {
	db := filepath.Join(t.TempDir(), "data", "casebook.db")
	f := newDoctorFixture(t, db)
	r := Run(context.Background(), f.Opts)
	wantCheck(t, r, "data_dir", StatusWarn, "does not exist yet")
	wantCheck(t, r, "database", StatusWarn, "does not exist yet")
	for _, id := range []string{"integrity", "migrations", "write_access"} {
		wantCheck(t, r, id, StatusSkip, "no database yet")
	}
	wantCheck(t, r, "mcp_startup", StatusOK, "temporary database")
	if !r.OK {
		t.Fatalf("warnings failed the report: %+v", r.Checks)
	}
	if _, err := os.Stat(db); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("doctor created %s (stat %v)", db, err)
	}
	if _, err := os.Stat(filepath.Dir(db)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("doctor created the data dir (stat %v)", err)
	}
}

func TestRunGarbageDatabase(t *testing.T) {
	db := filepath.Join(t.TempDir(), "casebook.db")
	doctorWrite(t, db, strings.Repeat("not a sqlite database\n", 200))
	r := Run(context.Background(), newDoctorFixture(t, db).Opts)
	wantCheck(t, r, "database", StatusFail)
	for _, id := range []string{"integrity", "migrations", "write_access"} {
		wantCheck(t, r, id, StatusSkip, "database could not be opened")
	}
	if r.OK || r.Failed() != 1 {
		t.Fatalf("ok=%v failed=%d, want one failure", r.OK, r.Failed())
	}
}

func TestRunMigrations(t *testing.T) {
	db, a := doctorSeeded(t)
	st, err := a.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	r := Run(context.Background(), newDoctorFixture(t, db).Opts)
	wantCheck(t, r, "migrations", StatusOK, "schema version "+itoa(st.SchemaVersion)+" (latest for this build;", "applies pending migrations")
}

func TestRunReadOnlyDatabase(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("file permissions do not restrict this user")
	}
	db, a := doctorSeeded(t)
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(db, 0o400); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(db, 0o600) })
	r := Run(context.Background(), newDoctorFixture(t, db).Opts)
	wantCheck(t, r, "write_access", StatusFail, "not writable")
	if r.OK {
		t.Fatal("a read-only database passed")
	}
}

func TestRunClaudeConfig(t *testing.T) {
	gone := filepath.Join(t.TempDir(), "old", "casebook")
	cases := []struct {
		name   string
		found  []string
		config func(t *testing.T, f *doctorFixture)
		status Status
		subs   []string
	}{
		{"absent client", nil, func(*testing.T, *doctorFixture) {}, StatusSkip, []string{"claude is not on PATH"}},
		{"config missing, claude on PATH", []string{"claude"}, func(*testing.T, *doctorFixture) {}, StatusWarn, []string{"casebook setup claude"}},
		{"invalid", nil, func(t *testing.T, f *doctorFixture) { doctorWrite(t, f.Opts.Paths.ClaudeJSON, "{") }, StatusWarn, []string{"not valid JSON"}},
		{"not registered", nil, func(t *testing.T, f *doctorFixture) {
			doctorWrite(t, f.Opts.Paths.ClaudeJSON, `{"mcpServers": {}}`)
		}, StatusWarn, []string{"not registered", "hooks: none"}},
		{"command missing", nil, func(t *testing.T, f *doctorFixture) {
			doctorWrite(t, f.Opts.Paths.ClaudeJSON, `{"mcpServers": {"casebook": {"command": `+doctorQuote(gone)+`}}}`)
		}, StatusFail, []string{gone, "does not exist"}},
		{"other CLI", nil, func(t *testing.T, f *doctorFixture) {
			other := doctorCLI(t, t.TempDir())
			doctorWrite(t, f.Opts.Paths.ClaudeJSON, `{"mcpServers": {"casebook": {"command": `+doctorQuote(other)+`}}}`)
		}, StatusWarn, []string{"not this CLI"}},
		{"registered without hooks", nil, func(t *testing.T, f *doctorFixture) {
			if _, err := integrations.SetupClaude(f.Opts.CLIPath, integrations.ClaudeOptions{Paths: f.Opts.Paths}); err != nil {
				t.Fatal(err)
			}
		}, StatusOK, []string{"matches this CLI", "hooks: none"}},
		{"registered with hooks", nil, func(t *testing.T, f *doctorFixture) {
			if _, err := integrations.SetupClaude(f.Opts.CLIPath, integrations.ClaudeOptions{Paths: f.Opts.Paths, Hooks: true}); err != nil {
				t.Fatal(err)
			}
		}, StatusOK, []string{"hooks: SessionStart, Stop, PreCompact"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newDoctorFixture(t, filepath.Join(t.TempDir(), "casebook.db"), tc.found...)
			tc.config(t, f)
			wantCheck(t, Run(context.Background(), f.Opts), "claude_config", tc.status, tc.subs...)
		})
	}
}

func TestRunCodexConfig(t *testing.T) {
	cases := []struct {
		name   string
		found  []string
		config func(t *testing.T, f *doctorFixture)
		status Status
		subs   []string
	}{
		{"absent client", nil, func(*testing.T, *doctorFixture) {}, StatusSkip, []string{"codex is not on PATH"}},
		{"config missing, codex on PATH", []string{"codex"}, func(*testing.T, *doctorFixture) {}, StatusWarn, []string{"casebook setup codex"}},
		{"invalid TOML", nil, func(t *testing.T, f *doctorFixture) { doctorWrite(t, f.Opts.Paths.CodexConfig, "model = \n") }, StatusWarn, []string{"not valid TOML"}},
		{"not registered", nil, func(t *testing.T, f *doctorFixture) { doctorWrite(t, f.Opts.Paths.CodexConfig, "model = \"o3\"\n") }, StatusWarn, []string{"not registered"}},
		{"managed", nil, func(t *testing.T, f *doctorFixture) {
			if _, err := integrations.SetupCodex(f.Opts.CLIPath, integrations.CodexOptions{Paths: f.Opts.Paths}); err != nil {
				t.Fatal(err)
			}
		}, StatusOK, []string{"managed block", "matches this CLI"}},
		{"Codex table inside the block", nil, func(t *testing.T, f *doctorFixture) {
			if _, err := integrations.SetupCodex(f.Opts.CLIPath, integrations.CodexOptions{Paths: f.Opts.Paths}); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(f.Opts.Paths.CodexConfig)
			if err != nil {
				t.Fatal(err)
			}
			text := strings.Replace(string(data), "# END casebook", "[projects.\"/home/jo/shop\"]\ntrust_level = \"trusted\"\n# END casebook", 1)
			doctorWrite(t, f.Opts.Paths.CodexConfig, text)
		}, StatusWarn, []string{"trust entry", "run casebook setup codex"}},
		{"hand-written", nil, func(t *testing.T, f *doctorFixture) {
			doctorWrite(t, f.Opts.Paths.CodexConfig, "[mcp_servers.casebook]\ncommand = "+doctorQuote(f.Opts.CLIPath)+"\nargs = [\"mcp\"]\n")
		}, StatusOK, []string{"hand-written"}},
		{"command missing", nil, func(t *testing.T, f *doctorFixture) {
			doctorWrite(t, f.Opts.Paths.CodexConfig, "[mcp_servers.casebook]\ncommand = "+doctorQuote(filepath.Join(t.TempDir(), "nope"))+"\n")
		}, StatusFail, []string{"does not exist"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newDoctorFixture(t, filepath.Join(t.TempDir(), "casebook.db"), tc.found...)
			tc.config(t, f)
			wantCheck(t, Run(context.Background(), f.Opts), "codex_config", tc.status, tc.subs...)
		})
	}
}

func TestRunClientCLIs(t *testing.T) {
	f := newDoctorFixture(t, filepath.Join(t.TempDir(), "casebook.db"), "claude")
	r := Run(context.Background(), f.Opts)
	wantCheck(t, r, "claude_cli", StatusOK, "found at /usr/local/bin/claude")
	wantCheck(t, r, "codex_cli", StatusSkip, "not on PATH")
	if !slices.Contains(f.Looked, "claude") || !slices.Contains(f.Looked, "codex") {
		t.Fatalf("LookPath calls = %v, want claude and codex", f.Looked)
	}
}

func TestRunCLIPath(t *testing.T) {
	for _, goos := range []string{"linux", "windows"} {
		t.Run(goos, func(t *testing.T) {
			root := t.TempDir()
			f := newDoctorFixture(t, filepath.Join(t.TempDir(), "casebook.db"))
			f.Opts.CLIPath = doctorCLI(t, root)
			f.Opts.GOOS = goos
			r := Run(context.Background(), f.Opts)
			wantCheck(t, r, "cli_path", StatusWarn, f.Opts.CLIPath, "desktop app: not found next to the CLI", "casebook ui will not work")

			name := "Casebook"
			if goos == "windows" {
				name += ".exe"
			}
			desktop := filepath.Join(filepath.Dir(filepath.Dir(f.Opts.CLIPath)), name)
			doctorWrite(t, desktop, "app")
			r = Run(context.Background(), f.Opts)
			wantCheck(t, r, "cli_path", StatusOK, "desktop app: "+desktop)
		})
	}
	f := newDoctorFixture(t, filepath.Join(t.TempDir(), "casebook.db"))
	f.Opts.CLIPath = ""
	r := Run(context.Background(), f.Opts)
	wantCheck(t, r, "cli_path", StatusWarn, "cannot locate")
	wantCheck(t, r, "mcp_startup", StatusFail, "cannot locate the casebook binary")
}

func TestRunMCPFailure(t *testing.T) {
	db, _ := doctorSeeded(t)
	f := newDoctorFixture(t, db)
	f.Opts.MCPCheck = func(context.Context, string, string) (MCPResult, error) {
		return MCPResult{}, errors.New("MCP handshake failed: EOF; server said: casebook: boom")
	}
	r := Run(context.Background(), f.Opts)
	wantCheck(t, r, "mcp_startup", StatusFail, "server said: casebook: boom")
	if r.Failed() != 1 || r.OK {
		t.Fatalf("failed = %d ok = %v", r.Failed(), r.OK)
	}
}

// TestRunNoMutation: counts and the newest activity id are identical before
// and after a run.
func TestRunNoMutation(t *testing.T) {
	db, a := doctorSeeded(t)
	ctx := context.Background()
	snapshot := func() (app.Counts, int64) {
		st, err := a.Status(ctx)
		if err != nil {
			t.Fatal(err)
		}
		rows, err := a.ListActivity(ctx, app.ActivityFilter{Limit: 1})
		if err != nil || len(rows) == 0 {
			t.Fatalf("activity: %v %d", err, len(rows))
		}
		return st.Counts, rows[0].ID
	}
	c1, id1 := snapshot()
	f := newDoctorFixture(t, db, "claude", "codex")
	doctorSetupClients(t, f)
	Run(ctx, f.Opts)
	Run(ctx, f.Opts)
	c2, id2 := snapshot()
	if c1 != c2 || id1 != id2 {
		t.Fatalf("doctor changed the database: %+v/%d -> %+v/%d", c1, id1, c2, id2)
	}
}

// TestRunSecrets: sentinels in record text and client config never reach
// the report (DIAG-02).
func TestRunSecrets(t *testing.T) {
	const (
		sTitle   = "SENTINEL-title-91c2"
		sSummary = "SENTINEL-summary-4d7e"
		sEnv     = "SENTINEL-claude-env-0b6a"
		sOther   = "SENTINEL-other-env-3f18"
		sCodex   = "SENTINEL-codex-env-a5d0"
		sProcess = "SENTINEL-process-env-77e1"
	)
	ctx := context.Background()
	db, a := doctorSeeded(t)
	inv, err := a.CreateInvestigation(ctx, app.ActorCLI(), app.CreateInvestigationInput{Title: sTitle, MakeActive: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.AddEvidence(ctx, app.ActorCLI(), inv.Ref, app.EvidenceInput{Title: "e", Summary: sSummary, SourceType: "manual_observation"}); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOCTOR_TEST_TOKEN", sProcess)
	f := newDoctorFixture(t, db, "claude", "codex")
	doctorWrite(t, f.Opts.Paths.ClaudeJSON, `{"mcpServers": {
  "casebook": {"type": "stdio", "command": `+doctorQuote(f.Opts.CLIPath)+`, "args": ["mcp"], "env": {"TOKEN": "`+sEnv+`"}},
  "other": {"command": "x", "env": {"KEY": "`+sOther+`"}}
}}`)
	doctorWrite(t, f.Opts.Paths.CodexConfig, "[mcp_servers.casebook]\ncommand = "+doctorQuote(f.Opts.CLIPath)+"\nargs = [\"mcp\"]\n\n[mcp_servers.casebook.env]\nTOKEN = \""+sCodex+"\"\n")
	r := Run(ctx, f.Opts)
	out, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{sTitle, sSummary, sEnv, sOther, sCodex, sProcess} {
		if strings.Contains(string(out), s) {
			t.Errorf("report contains %s:\n%s", s, out)
		}
	}
	wantCheck(t, r, "database", StatusOK, "2 investigations")
}

// itoa formats n in decimal.
func itoa(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}
