package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/app/apptest"
)

// doctorResult is one finished run of the built binary.
type doctorResult struct {
	Stdout, Stderr string
	Code           int
}

// doctorRun runs the built casebook with args in the isolated environment d
// (HOME, CLAUDE_CONFIG_DIR, CODEX_HOME and CASEBOOK_HOME in temp dirs), from
// d.Home as the working directory.
func doctorRun(t *testing.T, d clientSmokeDirs, args ...string) doctorResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary(t), args...)
	cmd.Env = d.Env
	cmd.Dir = d.Home
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	code := 0
	if err != nil {
		var ee *exec.ExitError
		if !errors.As(err, &ee) {
			t.Fatalf("run casebook %s: %v\nstderr:\n%s", strings.Join(args, " "), err, errb.String())
		}
		code = ee.ExitCode()
	}
	return doctorResult{Stdout: out.String(), Stderr: errb.String(), Code: code}
}

// doctorSeeded returns a database file holding the cloud spend
// fixture.
func doctorSeeded(t *testing.T) string {
	t.Helper()
	a, dbPath := openApp(t)
	apptest.SeedCloudSpend(t, a)
	return dbPath
}

// doctorJSONReport mirrors doctor.Report.
type doctorJSONReport struct {
	Version string `json:"version"`
	OK      bool   `json:"ok"`
	Checks  []struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Status string `json:"status"`
		Detail string `json:"detail"`
	} `json:"checks"`
}

// doctorDecode decodes a doctor --json report.
func doctorDecode(t *testing.T, s string) doctorJSONReport {
	t.Helper()
	var r doctorJSONReport
	if err := json.Unmarshal([]byte(s), &r); err != nil {
		t.Fatalf("decode doctor report: %v\n%s", err, s)
	}
	return r
}

// doctorCheck returns the check with id or fails the test.
func doctorCheck(t *testing.T, r doctorJSONReport, id string) (status, detail string) {
	t.Helper()
	for _, c := range r.Checks {
		if c.ID == id {
			return c.Status, c.Detail
		}
	}
	t.Fatalf("doctor report has no %s check: %+v", id, r.Checks)
	return "", ""
}

func TestMCPSelfTest(t *testing.T) {
	db := doctorSeeded(t)
	d := clientSmokeEnv(t)

	r := doctorRun(t, d, "mcp", "--self-test", "--db", db)
	if r.Code != 0 {
		t.Fatalf("self-test exit %d\nstdout:\n%s\nstderr:\n%s", r.Code, r.Stdout, r.Stderr)
	}
	if !strings.Contains(r.Stdout, "ok") || !strings.Contains(r.Stdout, "21 tools") {
		t.Fatalf("self-test output = %q, want ok and 21 tools", r.Stdout)
	}
	if strings.Contains(r.Stdout, "temporary") {
		t.Fatalf("self-test used a temporary database although %s exists: %q", db, r.Stdout)
	}

	var out struct {
		OK            bool   `json:"ok"`
		Server        string `json:"server"`
		ServerVersion string `json:"server_version"`
		Tools         int    `json:"tools"`
		TempDB        bool   `json:"temp_db"`
	}
	r = doctorRun(t, d, "mcp", "--self-test", "--json", "--db", db)
	if r.Code != 0 {
		t.Fatalf("self-test --json exit %d\nstderr:\n%s", r.Code, r.Stderr)
	}
	if err := json.Unmarshal([]byte(r.Stdout), &out); err != nil {
		t.Fatalf("decode self-test json: %v\n%s", err, r.Stdout)
	}
	if !out.OK || out.Tools != 21 || out.TempDB || out.Server != "casebook" || out.ServerVersion != testVersion {
		t.Fatalf("self-test json = %+v, want ok, 21 tools, casebook %s, temp_db false", out, testVersion)
	}

	missing := filepath.Join(t.TempDir(), "missing.db")
	r = doctorRun(t, d, "mcp", "--self-test", "--json", "--db", missing)
	if r.Code != 0 {
		t.Fatalf("self-test on a missing database exit %d\nstderr:\n%s", r.Code, r.Stderr)
	}
	out = struct {
		OK            bool   `json:"ok"`
		Server        string `json:"server"`
		ServerVersion string `json:"server_version"`
		Tools         int    `json:"tools"`
		TempDB        bool   `json:"temp_db"`
	}{}
	if err := json.Unmarshal([]byte(r.Stdout), &out); err != nil {
		t.Fatalf("decode self-test json: %v\n%s", err, r.Stdout)
	}
	if !out.OK || out.Tools != 21 || !out.TempDB {
		t.Fatalf("self-test json on a missing database = %+v, want ok, 21 tools, temp_db true", out)
	}
	if _, err := os.Stat(missing); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("self-test created %s (stat err %v)", missing, err)
	}
}

func TestDoctorTracer(t *testing.T) {
	db := doctorSeeded(t)
	d := clientSmokeEnv(t)

	r := doctorRun(t, d, "doctor", "--db", db, "--json")
	if r.Code != 0 {
		t.Fatalf("doctor exit %d\nstdout:\n%s\nstderr:\n%s", r.Code, r.Stdout, r.Stderr)
	}
	rep := doctorDecode(t, r.Stdout)
	if !rep.OK || rep.Version != testVersion {
		t.Fatalf("report ok=%v version=%q, want ok and %s", rep.OK, rep.Version, testVersion)
	}
	for _, id := range []string{"data_dir", "database", "mcp_startup"} {
		if s, detail := doctorCheck(t, rep, id); s != "ok" {
			t.Errorf("%s = %s (%s), want ok", id, s, detail)
		}
	}
	if _, detail := doctorCheck(t, rep, "mcp_startup"); !strings.Contains(detail, "21 tools") {
		t.Errorf("mcp_startup detail = %q, want 21 tools", detail)
	}
	if _, detail := doctorCheck(t, rep, "database"); !strings.Contains(detail, "INV-0001") {
		t.Errorf("database detail = %q, want the active ref INV-0001", detail)
	}
}

// TestDoctorSecrets: through the real binary, sentinels in record text, in
// Claude Code and Codex env tables and in the process environment never
// reach doctor's output (DIAG-02).
func TestDoctorSecrets(t *testing.T) {
	const (
		sTitle   = "SENTINEL-bin-title-2c8f"
		sSummary = "SENTINEL-bin-summary-9a31"
		sEnv     = "SENTINEL-bin-claude-env-6d0b"
		sOther   = "SENTINEL-bin-other-env-e47c"
		sCodex   = "SENTINEL-bin-codex-env-03fa"
		sProcess = "SENTINEL-bin-process-b815"
	)
	ctx := context.Background()
	a, db := openApp(t)
	apptest.SeedCloudSpend(t, a)
	inv, err := a.CreateInvestigation(ctx, app.ActorCLI(), app.CreateInvestigationInput{Title: sTitle, MakeActive: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.AddEvidence(ctx, app.ActorCLI(), inv.Ref, app.EvidenceInput{Title: "e", Summary: sSummary, SourceType: "manual_observation"}); err != nil {
		t.Fatal(err)
	}
	d := clientSmokeEnv(t)
	d.Env = append(d.Env, "DOCTOR_BIN_TOKEN="+sProcess)

	if r := doctorRun(t, d, "setup", "claude", "--hooks", "--db", db); r.Code != 0 {
		t.Fatalf("setup claude: exit %d\n%s", r.Code, r.Stderr)
	}
	claudeJSON := filepath.Join(d.ClaudeDir, ".claude.json")
	raw, err := os.ReadFile(claudeJSON)
	if err != nil {
		t.Fatal(err)
	}
	var cfg map[string]map[string]map[string]any
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}
	cfg["mcpServers"]["casebook"]["env"] = map[string]string{"TOKEN": sEnv}
	cfg["mcpServers"]["other"] = map[string]any{"command": "x", "env": map[string]string{"KEY": sOther}}
	raw, _ = json.Marshal(cfg)
	if err := os.WriteFile(claudeJSON, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	bin, err := filepath.EvalSymlinks(binary(t))
	if err != nil {
		t.Fatal(err)
	}
	cmdTOML, _ := json.Marshal(bin)
	codexTOML := "[mcp_servers.casebook]\ncommand = " + string(cmdTOML) + "\nargs = [\"mcp\", \"--db\", " + strconvQuote(db) + "]\n\n[mcp_servers.casebook.env]\nTOKEN = \"" + sCodex + "\"\n"
	if err := os.WriteFile(filepath.Join(d.CodexHome, "config.toml"), []byte(codexTOML), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, args := range [][]string{{"doctor", "--db", db}, {"doctor", "--db", db, "--json"}} {
		r := doctorRun(t, d, args...)
		if r.Code != 0 {
			t.Fatalf("%v: exit %d\nstdout:\n%s\nstderr:\n%s", args, r.Code, r.Stdout, r.Stderr)
		}
		for _, s := range []string{sTitle, sSummary, sEnv, sOther, sCodex, sProcess} {
			if strings.Contains(r.Stdout, s) || strings.Contains(r.Stderr, s) {
				t.Errorf("%v output contains %s", args, s)
			}
		}
		for _, want := range []string{"hooks: SessionStart, Stop, PreCompact", "hand-written", "21 tools", "2 investigations"} {
			if !strings.Contains(r.Stdout, want) {
				t.Errorf("%v output lacks %q:\n%s", args, want, r.Stdout)
			}
		}
		if len(args) == 3 {
			t.Logf("doctor table:\n%s", r.Stdout)
		}
	}
}

// strconvQuote encodes s as a JSON string, which is also a valid TOML basic
// string for paths.
func strconvQuote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// TestDoctorExitCode: a garbage database file fails doctor with exit 1.
func TestDoctorExitCode(t *testing.T) {
	d := clientSmokeEnv(t)
	garbage := filepath.Join(t.TempDir(), "casebook.db")
	if err := os.WriteFile(garbage, bytes.Repeat([]byte("not a database\n"), 100), 0o600); err != nil {
		t.Fatal(err)
	}
	r := doctorRun(t, d, "doctor", "--db", garbage, "--json")
	if r.Code != 1 {
		t.Fatalf("exit %d, want 1\nstdout:\n%s\nstderr:\n%s", r.Code, r.Stdout, r.Stderr)
	}
	if !strings.HasPrefix(r.Stderr, "casebook: doctor found ") {
		t.Fatalf("stderr = %q", r.Stderr)
	}
	rep := doctorDecode(t, r.Stdout)
	if rep.OK {
		t.Fatal("report ok on a garbage database")
	}
	if s, _ := doctorCheck(t, rep, "database"); s != "fail" {
		t.Fatalf("database = %s, want fail", s)
	}
	if s, detail := doctorCheck(t, rep, "mcp_startup"); s != "fail" || !strings.Contains(detail, "server said: casebook:") {
		t.Fatalf("mcp_startup = %s %q, want fail quoting the server error line", s, detail)
	}
	if b, err := os.ReadFile(garbage); err != nil || !bytes.Equal(b, bytes.Repeat([]byte("not a database\n"), 100)) {
		t.Fatalf("doctor changed the garbage file (err %v)", err)
	}
}
