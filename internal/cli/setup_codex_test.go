package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jameskbb/casebook/internal/app/apptest"
	"github.com/jameskbb/casebook/internal/integrations"
)

// codexOriginal is a small user config.toml for the CLI tests.
const codexOriginal = "# my codex config\nmodel = \"o3\"\n\n[mcp_servers.other]\ncommand = \"other-server\"\nargs = [\"serve\"]\n"

// codexWrite writes data as the isolated Codex config.toml.
func codexWrite(t *testing.T, d clientDirs, data string) {
	t.Helper()
	if err := os.WriteFile(d.Paths.CodexConfig, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
}

// codexRead returns the isolated Codex config.toml.
func codexRead(t *testing.T, d clientDirs) string {
	t.Helper()
	b, err := os.ReadFile(d.Paths.CodexConfig)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestSetupCodex(t *testing.T) {
	d := isolateClients(t, true)
	exe := fakeCLIPath(t)
	codexWrite(t, d, codexOriginal)

	r := runClientCLI(t, d, exe, "", "setup", "codex")
	wantCode(t, r, 0)
	wantContains(t, r.Stdout, d.Paths.CodexConfig+": added mcp_servers.casebook", "backup: ", "codex mcp list")
	got := codexRead(t, d)
	block := integrations.CodexConfigPreview(exe, integrations.CodexOptions{Paths: d.Paths})
	block = block[strings.Index(block, "\n")+1:]
	if want := codexOriginal + "\n" + block; got != want {
		t.Errorf("config.toml =\n%s\nwant the original, a blank line, then the block:\n%s", got, want)
	}
	if !strings.Contains(got, "command = '"+exe+"'\nargs = ['mcp']\n# END casebook\n") {
		t.Errorf("block does not hold the command and args:\n%s", got)
	}
	bk := backups(t, d.Paths.CodexConfig)
	if len(bk) != 1 {
		t.Fatalf("backups = %v, want one", bk)
	}
	if b, _ := os.ReadFile(bk[0]); string(b) != codexOriginal {
		t.Errorf("backup = %q, want the original bytes", b)
	}

	r = runClientCLI(t, d, exe, "", "setup", "codex")
	wantCode(t, r, 0)
	wantContains(t, r.Stdout, d.Paths.CodexConfig+": unchanged mcp_servers.casebook")
	if strings.Contains(r.Stdout, "backup: ") || len(backups(t, d.Paths.CodexConfig)) != 1 {
		t.Errorf("rerun wrote a backup:\n%s", r.Stdout)
	}
	if codexRead(t, d) != got {
		t.Errorf("rerun changed the file")
	}
}

func TestSetupCodexRefusedUnmanagedCLI(t *testing.T) {
	d := isolateClients(t, true)
	exe := fakeCLIPath(t)
	const hand = "[mcp_servers.casebook]\ncommand = \"/somewhere/casebook\"\nargs = [\"mcp\"]\n"
	codexWrite(t, d, hand)
	r := runClientCLI(t, d, exe, "", "setup", "codex")
	wantCode(t, r, 1)
	if want := integrations.CodexConfigPreview(exe, integrations.CodexOptions{Paths: d.Paths}); r.Stdout != want {
		t.Errorf("stdout = %q, want the manual config %q", r.Stdout, want)
	}
	want := "casebook: refusing to change " + d.Paths.CodexConfig +
		": an existing [mcp_servers.casebook] is not managed by Casebook; edit it by hand or remove it, then rerun; nothing was written\n"
	if r.Stderr != want {
		t.Errorf("stderr = %q, want %q", r.Stderr, want)
	}
	if codexRead(t, d) != hand || len(backups(t, d.Paths.CodexConfig)) != 0 {
		t.Errorf("refusal wrote the file or a backup")
	}
}

func TestSetupCodexPrint(t *testing.T) {
	d := isolateClients(t, true)
	exe := fakeCLIPath(t)
	r := runClientCLI(t, d, exe, "", "setup", "codex", "--print")
	wantCode(t, r, 0)
	if want := integrations.CodexConfigPreview(exe, integrations.CodexOptions{Paths: d.Paths}); r.Stdout != want {
		t.Errorf("stdout:\n%s\nwant exactly the preview:\n%s", r.Stdout, want)
	}
	if names := dirNames(t, d.CodexHome); len(names) != 0 {
		t.Errorf("--print wrote %v", names)
	}

	db := filepath.Join(t.TempDir(), "x.db")
	r = runClientCLI(t, d, exe, "", "setup", "codex", "--print", "--json", "--db", db)
	wantCode(t, r, 0)
	var got setupJSON
	decodeJSON(t, r.Stdout, &got)
	want := integrations.CodexConfigPreview(exe, integrations.CodexOptions{Paths: d.Paths, DBPath: db})
	if !got.DryRun || got.Preview != want || got.Client != "codex" || got.CLIPath != exe || got.Changes == nil || len(got.Changes) != 0 {
		t.Errorf("--print --json = %+v", got)
	}
	if names := dirNames(t, d.CodexHome); len(names) != 0 {
		t.Errorf("--print --json wrote %v", names)
	}
	if _, err := os.Stat(db); !os.IsNotExist(err) {
		t.Errorf("--print opened or created the database (%v)", err)
	}
}

func TestSetupCodexPrintRemove(t *testing.T) {
	d := isolateClients(t, true)
	r := runClientCLI(t, d, fakeCLIPath(t), "", "setup", "codex", "--print", "--remove")
	wantCode(t, r, 2)
	wantContains(t, r.Stderr, "--print cannot be combined with --remove")
	if names := dirNames(t, d.CodexHome); len(names) != 0 {
		t.Errorf("usage error wrote %v", names)
	}
}

func TestSetupCodexRemove(t *testing.T) {
	d := isolateClients(t, true)
	exe := fakeCLIPath(t)
	codexWrite(t, d, codexOriginal)
	wantCode(t, runClientCLI(t, d, exe, "", "setup", "codex"), 0)

	r := runClientCLI(t, d, exe, "", "setup", "codex", "--remove")
	wantCode(t, r, 0)
	wantContains(t, r.Stdout, d.Paths.CodexConfig+": removed mcp_servers.casebook", "backup: ", "Restart Codex sessions to unload Casebook.")
	if got := codexRead(t, d); got != codexOriginal {
		t.Errorf("after remove = %q, want the original", got)
	}
	n := len(backups(t, d.Paths.CodexConfig))

	r = runClientCLI(t, d, exe, "", "setup", "codex", "--remove")
	wantCode(t, r, 0)
	wantContains(t, r.Stdout, d.Paths.CodexConfig+": unchanged mcp_servers.casebook")
	if strings.Contains(r.Stdout, "backup: ") || len(backups(t, d.Paths.CodexConfig)) != n {
		t.Errorf("second remove wrote a backup:\n%s", r.Stdout)
	}
}

func TestSetupCodexRemoveMissingFile(t *testing.T) {
	d := isolateClients(t, true)
	r := runClientCLI(t, d, fakeCLIPath(t), "", "setup", "codex", "--remove")
	wantCode(t, r, 0)
	wantContains(t, r.Stdout, "unchanged mcp_servers.casebook")
	if names := dirNames(t, d.CodexHome); len(names) != 0 {
		t.Errorf("remove created %v", names)
	}
}

func TestSetupCodexJSON(t *testing.T) {
	d := isolateClients(t, true)
	exe := fakeCLIPath(t)
	r := runClientCLI(t, d, exe, "", "--json", "setup", "codex")
	wantCode(t, r, 0)
	var raw any
	decodeJSON(t, r.Stdout, &raw)
	apptest.AssertNoJSONNulls(t, raw)
	var got setupJSON
	decodeJSON(t, r.Stdout, &got)
	if got.Client != "codex" || got.CLIPath != exe || got.DryRun || got.Next == "" || got.Preview != "" {
		t.Errorf("report = %+v", got)
	}
	if len(got.Changes) != 1 || got.Changes[0].Action != "added" || got.Changes[0].File != d.Paths.CodexConfig ||
		got.Changes[0].Backup != "" || strings.Join(got.Changes[0].Entries, ",") != "mcp_servers.casebook" {
		t.Errorf("changes = %+v", got.Changes)
	}

	r = runClientCLI(t, d, exe, "", "--json", "setup", "codex", "--remove")
	wantCode(t, r, 0)
	decodeJSON(t, r.Stdout, &raw)
	apptest.AssertNoJSONNulls(t, raw)
	decodeJSON(t, r.Stdout, &got)
	if len(got.Changes) != 1 || got.Changes[0].Action != "removed" || got.Changes[0].Backup == "" {
		t.Errorf("remove changes = %+v", got.Changes)
	}
}

func TestSetupCodexRefusedJSON(t *testing.T) {
	d := isolateClients(t, true)
	exe := fakeCLIPath(t)
	const hand = "[mcp_servers.casebook]\ncommand = \"/somewhere/casebook\"\n"
	codexWrite(t, d, hand)
	r := runClientCLI(t, d, exe, "", "--json", "setup", "codex")
	wantCode(t, r, 1)
	var raw any
	decodeJSON(t, r.Stdout, &raw)
	apptest.AssertNoJSONNulls(t, raw)
	var got setupJSON
	decodeJSON(t, r.Stdout, &got)
	if got.Client != "codex" || got.Preview != integrations.CodexConfigPreview(exe, integrations.CodexOptions{Paths: d.Paths}) || len(got.Changes) != 0 {
		t.Errorf("refusal report = %+v", got)
	}
	wantContains(t, r.Stderr, "refusing to change "+d.Paths.CodexConfig+": an existing [mcp_servers.casebook] is not managed by Casebook")
	if codexRead(t, d) != hand || len(backups(t, d.Paths.CodexConfig)) != 0 {
		t.Errorf("refusal wrote the file or a backup")
	}
}

func TestSetupCodexRefusedInvalidTOML(t *testing.T) {
	d := isolateClients(t, true)
	const broken = "model = \"o3\"\ntoken = = \"placeholder-secret-zz\"\n"
	codexWrite(t, d, broken)
	r := runClientCLI(t, d, fakeCLIPath(t), "", "setup", "codex")
	wantCode(t, r, 1)
	want := "casebook: refusing to change " + d.Paths.CodexConfig + ": not valid TOML (line 2, column 9); nothing was written\n"
	if r.Stderr != want {
		t.Errorf("stderr = %q, want %q", r.Stderr, want)
	}
	if strings.Contains(r.Stderr, "placeholder-secret-zz") {
		t.Errorf("stderr quotes the file")
	}
	wantContains(t, r.Stdout, "# BEGIN casebook")
	if codexRead(t, d) != broken {
		t.Errorf("file changed")
	}
}

func TestSetupCodexHelp(t *testing.T) {
	d := isolateClients(t, true)
	r := runClientCLI(t, d, fakeCLIPath(t), "", "setup")
	wantCode(t, r, 0)
	wantContains(t, r.Stdout, "codex", "Codex")
	r = runClientCLI(t, d, fakeCLIPath(t), "", "setup", "codex", "--help")
	wantCode(t, r, 0)
	wantContains(t, r.Stdout, "--remove", "remove the Casebook-managed block and nothing else", "--print",
		"print the configuration block that would be added and change nothing", "# BEGIN casebook")
}
