package cli

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/doctor"
)

// doctorFakes replaces doctorLookPath and doctorMCPCheck for one test: the
// in-process test binary must never be spawned as casebook. mcpErr, when
// non-nil, makes the MCP check fail. It returns the names LookPath was
// asked for.
func doctorFakes(t *testing.T, mcpErr error, found ...string) *[]string {
	t.Helper()
	oldLook, oldMCP := doctorLookPath, doctorMCPCheck
	t.Cleanup(func() { doctorLookPath, doctorMCPCheck = oldLook, oldMCP })
	looked := &[]string{}
	doctorLookPath = func(name string) (string, error) {
		*looked = append(*looked, name)
		for _, f := range found {
			if f == name {
				return "/usr/bin/" + name, nil
			}
		}
		return "", errors.New("not found")
	}
	doctorMCPCheck = func(_ context.Context, _, db string) (doctor.MCPResult, error) {
		if mcpErr != nil {
			return doctor.MCPResult{}, mcpErr
		}
		_, err := os.Stat(db)
		return doctor.MCPResult{ServerName: "casebook", ServerVersion: Version, Tools: 20, TempDB: err != nil}, nil
	}
	return looked
}

func TestDoctorTableAndJSON(t *testing.T) {
	db, _ := seededDB(t)
	d := isolateClients(t, true)
	doctorFakes(t, nil)
	exe := fakeCLIPath(t)

	r := runClientCLI(t, d, exe, "", "doctor", "--db", db)
	wantCode(t, r, 0)
	wantContains(t, r.Stdout, "Casebook doctor (casebook "+Version+")", "data_dir", "mcp_startup", "codex_cli",
		"All checks passed (", "20 tools")
	lines := strings.Split(strings.TrimSpace(r.Stdout), "\n")
	if len(lines) != 1+1+12+1+1 {
		t.Fatalf("table has %d lines, want header, blank, 12 checks, blank, summary:\n%s", len(lines), r.Stdout)
	}

	r = runClientCLI(t, d, exe, "", "doctor", "--db", db, "--json")
	wantCode(t, r, 0)
	var rep doctor.Report
	decodeJSON(t, r.Stdout, &rep)
	if !rep.OK || len(rep.Checks) != 12 || rep.Version != Version {
		t.Fatalf("report = %+v", rep)
	}
}

func TestDoctorExitCodes(t *testing.T) {
	db, _ := seededDB(t)
	d := isolateClients(t, true)
	exe := fakeCLIPath(t)

	// Warnings alone (no client config, clients absent) exit 0.
	doctorFakes(t, nil, "claude")
	r := runClientCLI(t, d, exe, "", "doctor", "--db", db)
	wantCode(t, r, 0)
	wantContains(t, r.Stdout, "claude_config", "warn")

	// One failing check exits 1 with the singular message.
	doctorFakes(t, errors.New("MCP handshake failed: EOF"))
	r = runClientCLI(t, d, exe, "", "doctor", "--db", db)
	wantCode(t, r, 1)
	if strings.TrimSpace(r.Stderr) != "casebook: doctor found 1 failing check" {
		t.Fatalf("stderr = %q", r.Stderr)
	}
	wantContains(t, r.Stdout, "1 check failed.")
	r = runClientCLI(t, d, exe, "", "doctor", "--db", db, "--json")
	wantCode(t, r, 1)
	var rep doctor.Report
	decodeJSON(t, r.Stdout, &rep)
	if rep.OK || rep.Failed() != 1 {
		t.Fatalf("json report ok=%v failed=%d", rep.OK, rep.Failed())
	}

	// Two failures use the plural.
	garbage := filepath.Join(t.TempDir(), "garbage.db")
	if err := os.WriteFile(garbage, []byte(strings.Repeat("garbage ", 100)), 0o600); err != nil {
		t.Fatal(err)
	}
	r = runClientCLI(t, d, exe, "", "doctor", "--db", garbage)
	wantCode(t, r, 1)
	if strings.TrimSpace(r.Stderr) != "casebook: doctor found 2 failing checks" {
		t.Fatalf("stderr = %q", r.Stderr)
	}
}

func TestDoctorMissingDatabase(t *testing.T) {
	d := isolateClients(t, true)
	looked := doctorFakes(t, nil)
	db := filepath.Join(t.TempDir(), "none", "casebook.db")
	r := runClientCLI(t, d, fakeCLIPath(t), "", "doctor", "--db", db, "--json")
	wantCode(t, r, 0)
	var rep doctor.Report
	decodeJSON(t, r.Stdout, &rep)
	for _, c := range rep.Checks {
		switch c.ID {
		case "integrity", "migrations", "write_access":
			if c.Status != doctor.StatusSkip {
				t.Errorf("%s = %s, want skip", c.ID, c.Status)
			}
		case "mcp_startup":
			if c.Status != doctor.StatusOK || !strings.Contains(c.Detail, "temporary database") {
				t.Errorf("mcp_startup = %+v", c)
			}
		}
	}
	if _, err := os.Stat(db); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("doctor created the database (stat %v)", err)
	}
	if strings.Join(*looked, ",") != "claude,codex" {
		t.Fatalf("LookPath calls = %v", *looked)
	}
}

func TestDoctorSecrets(t *testing.T) {
	const (
		sTitle   = "SENTINEL-cli-title-5e1d"
		sSummary = "SENTINEL-cli-summary-c3a9"
		sEnv     = "SENTINEL-cli-claude-env-82f0"
		sOther   = "SENTINEL-cli-other-env-1b4c"
		sCodex   = "SENTINEL-cli-codex-env-d92e"
		sProcess = "SENTINEL-cli-process-6a07"
	)
	ctx := context.Background()
	a, db, _ := seededApp(t)
	inv, err := a.CreateInvestigation(ctx, app.ActorCLI(), app.CreateInvestigationInput{Title: sTitle, MakeActive: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.AddEvidence(ctx, app.ActorCLI(), inv.Ref, app.EvidenceInput{Title: "e", Summary: sSummary, SourceType: "manual_observation"}); err != nil {
		t.Fatal(err)
	}
	d := isolateClients(t, true)
	t.Setenv("DOCTOR_CLI_TOKEN", sProcess)
	doctorFakes(t, nil, "claude", "codex")
	exe := fakeCLIPath(t)

	r := runClientCLI(t, d, exe, "", "setup", "claude", "--hooks", "--db", db)
	wantCode(t, r, 0)
	// Add env entries to the written config: Casebook's and another server's.
	raw, err := os.ReadFile(d.Paths.ClaudeJSON)
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
	if err := os.WriteFile(d.Paths.ClaudeJSON, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	codexWrite(t, d, "[mcp_servers.casebook]\ncommand = \"/opt/casebook\"\nargs = [\"mcp\"]\n\n[mcp_servers.casebook.env]\nTOKEN = \""+sCodex+"\"\n")

	for _, args := range [][]string{{"doctor", "--db", db}, {"doctor", "--db", db, "--json"}} {
		r := runClientCLI(t, d, exe, "", args...)
		for _, s := range []string{sTitle, sSummary, sEnv, sOther, sCodex, sProcess} {
			if strings.Contains(r.Stdout, s) || strings.Contains(r.Stderr, s) {
				t.Errorf("%v output contains %s:\nstdout:\n%s\nstderr:\n%s", args, s, r.Stdout, r.Stderr)
			}
		}
		// The /opt/casebook command does not exist: codex_config fails.
		wantCode(t, r, 1)
		wantContains(t, r.Stdout, "/opt/casebook")
	}
}
