package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/app/apptest"
)

func TestNoArgs(t *testing.T) {
	db, f := seededDB(t)
	r := runCLI(t, "", "--db", db)
	wantCode(t, r, 0)
	for _, want := range []string{"Casebook dev", db, f.InvestigationRef, "September cloud bill up 38%", "Exit codes:", "Usage:"} {
		if !strings.Contains(r.Stdout, want) {
			t.Errorf("stdout does not contain %q:\n%s", want, r.Stdout)
		}
	}
	if !strings.Contains(r.Stdout, "Exit codes: 0 success; 1 runtime failure; 2 usage or invalid input; 3 not found or no active case.") {
		t.Error("help does not document the exit codes")
	}
	if r.Stderr != "" {
		t.Errorf("stderr = %q", r.Stderr)
	}
	if strings.Index(r.Stdout, "Database:") > strings.Index(r.Stdout, "Usage:") {
		t.Error("status block must come before the help")
	}
}

func TestNoArgsJSON(t *testing.T) {
	db, _ := seededDB(t)
	r := runCLI(t, "", "--db", db, "--json")
	wantCode(t, r, 0)
	var info app.StatusInfo
	if err := json.Unmarshal([]byte(r.Stdout), &info); err != nil {
		t.Fatalf("stdout is not StatusInfo JSON: %v\n%s", err, r.Stdout)
	}
	if info.ActiveInvestigationRef != "INV-0001" || info.DBPath != db {
		t.Errorf("info = %+v", info)
	}
	var raw any
	if err := json.Unmarshal([]byte(r.Stdout), &raw); err != nil {
		t.Fatal(err)
	}
	apptest.AssertNoJSONNulls(t, raw)
	if strings.Contains(r.Stdout, "Usage:") {
		t.Error("--json output contains help text")
	}
}

// TestNoArgsMissingDB proves bare casebook never creates the database
// (AR-22), in text and JSON.
func TestNoArgsMissingDB(t *testing.T) {
	db := filepath.Join(t.TempDir(), "nested", "casebook.db")
	r := runCLI(t, "", "--db", db)
	wantCode(t, r, 0)
	for _, want := range []string{"Casebook dev", db, "not created yet", "Usage:"} {
		if !strings.Contains(r.Stdout, want) {
			t.Errorf("stdout does not contain %q:\n%s", want, r.Stdout)
		}
	}
	j := runCLI(t, "", "--db", db, "--json")
	wantCode(t, j, 0)
	var info app.StatusInfo
	if err := json.Unmarshal([]byte(j.Stdout), &info); err != nil || info.DBPath != db || info.SchemaVersion != 0 {
		t.Errorf("json = %q (err %v)", j.Stdout, err)
	}
	if _, err := os.Stat(filepath.Dir(db)); !os.IsNotExist(err) {
		t.Errorf("bare casebook created %s (stat err %v)", filepath.Dir(db), err)
	}
}

func TestNoArgsUnopenableDB(t *testing.T) {
	// A directory exists but is not a database on every OS. (A path through a
	// regular file reads as missing on Windows, which shows "not created yet".)
	r := runCLI(t, "", "--db", t.TempDir())
	wantCode(t, r, 0)
	if !strings.Contains(r.Stdout, "Usage:") || !strings.Contains(r.Stdout, "Exit codes:") {
		t.Errorf("help not printed:\n%s", r.Stdout)
	}
	lines := strings.Split(strings.TrimRight(r.Stderr, "\n"), "\n")
	if len(lines) != 1 || !strings.HasPrefix(lines[0], "casebook: note:") {
		t.Errorf("stderr = %q, want one line starting casebook: note:", r.Stderr)
	}
}

func TestStatus(t *testing.T) {
	db, _ := seededDB(t)
	t.Run("human", func(t *testing.T) {
		r := runCLI(t, "", "status", "--db", db)
		wantCode(t, r, 0)
		for _, want := range []string{"Database:", db, "Data dir:", filepath.Dir(db), "Schema:", "Active:",
			"INV-0001 September cloud bill up 38% (your AI's current case)", "Records:", "1 cases, 4 evidence, 3 claims, 2 hypotheses, 1 questions"} {
			if !strings.Contains(r.Stdout, want) {
				t.Errorf("stdout does not contain %q:\n%s", want, r.Stdout)
			}
		}
		if strings.Contains(r.Stdout, "Usage:") {
			t.Error("status printed help")
		}
	})
	t.Run("json", func(t *testing.T) {
		r := runCLI(t, "", "status", "--json", "--db", db)
		wantCode(t, r, 0)
		var info app.StatusInfo
		if err := json.Unmarshal([]byte(r.Stdout), &info); err != nil {
			t.Fatalf("not JSON: %v\n%s", err, r.Stdout)
		}
		if info.Counts.Evidence != 4 || info.SchemaVersion < 1 {
			t.Errorf("info = %+v", info)
		}
		var raw any
		_ = json.Unmarshal([]byte(r.Stdout), &raw)
		apptest.AssertNoJSONNulls(t, raw)
	})
	t.Run("no active", func(t *testing.T) {
		empty := filepath.Join(t.TempDir(), "casebook.db")
		a, err := app.Open(empty)
		if err != nil {
			t.Fatal(err)
		}
		a.Close()
		r := runCLI(t, "", "status", "--db", empty)
		wantCode(t, r, 0)
		if !strings.Contains(r.Stdout, `none (your AI has no current case; run "casebook case activate <ref>")`) {
			t.Errorf("stdout:\n%s", r.Stdout)
		}
	})
}

// TestStatusMissingDB proves casebook status reports a database that does
// not exist yet instead of creating it (AR-22), in text and JSON, and exits 0
// like bare casebook.
func TestStatusMissingDB(t *testing.T) {
	db := filepath.Join(t.TempDir(), "nested", "casebook.db")
	r := runCLI(t, "", "status", "--db", db)
	wantCode(t, r, 0)
	for _, want := range []string{"Database:", db, "not created yet", "Data dir:", filepath.Dir(db), "Active:"} {
		if !strings.Contains(r.Stdout, want) {
			t.Errorf("stdout does not contain %q:\n%s", want, r.Stdout)
		}
	}
	if strings.Contains(r.Stdout, "Usage:") || r.Stderr != "" {
		t.Errorf("stdout:\n%s\nstderr: %q", r.Stdout, r.Stderr)
	}
	j := runCLI(t, "", "status", "--json", "--db", db)
	wantCode(t, j, 0)
	var info app.StatusInfo
	if err := json.Unmarshal([]byte(j.Stdout), &info); err != nil || info.DBPath != db ||
		info.DataDir != filepath.Dir(db) || info.SchemaVersion != 0 || info.Counts != (app.Counts{}) {
		t.Errorf("json = %q (err %v)", j.Stdout, err)
	}
	var raw any
	_ = json.Unmarshal([]byte(j.Stdout), &raw)
	apptest.AssertNoJSONNulls(t, raw)
	if _, err := os.Stat(filepath.Dir(db)); !os.IsNotExist(err) {
		t.Errorf("status created %s (stat err %v)", filepath.Dir(db), err)
	}

}

func TestVersion(t *testing.T) {
	never := filepath.Join(t.TempDir(), "never")
	r := runCLI(t, "", "version", "--db", filepath.Join(never, "casebook.db"))
	wantCode(t, r, 0)
	if r.Stdout != "casebook dev\n" {
		t.Errorf("stdout = %q", r.Stdout)
	}
	if _, err := os.Stat(never); !os.IsNotExist(err) {
		t.Errorf("version touched the database directory: %v", err)
	}
	f := runCLI(t, "", "--version")
	wantCode(t, f, 0)
	if f.Stdout != "casebook dev\n" || f.Stderr != "" {
		t.Errorf("--version stdout = %q stderr = %q", f.Stdout, f.Stderr)
	}
	j := runCLI(t, "", "version", "--json", "--db", filepath.Join(never, "casebook.db"))
	wantCode(t, j, 0)
	var v map[string]string
	if err := json.Unmarshal([]byte(j.Stdout), &v); err != nil || v["version"] != "dev" || len(v) != 1 {
		t.Errorf("json = %q (%v)", j.Stdout, err)
	}
}

func TestUsageErrors(t *testing.T) {
	db := filepath.Join(t.TempDir(), "casebook.db")
	r := runCLI(t, "", "bogus", "--db", db)
	wantCode(t, r, 2)
	if r.Stderr != "casebook: unknown command \"bogus\" for \"casebook\"\n" {
		t.Errorf("stderr = %q", r.Stderr)
	}
	for _, args := range [][]string{
		{"status", "--nope", "--db", db},
		{"--nope"},
		{"mcp", "extra", "--db", db},
		{"version", "extra"},
		{"status", "extra", "--db", db},
	} {
		r := runCLI(t, "", args...)
		wantCode(t, r, 2)
		lines := strings.Split(strings.TrimRight(r.Stderr, "\n"), "\n")
		if len(lines) != 1 || !strings.HasPrefix(lines[0], "casebook: ") {
			t.Errorf("%v: stderr = %q", args, r.Stderr)
		}
		if r.Stdout != "" {
			t.Errorf("%v: stdout = %q", args, r.Stdout)
		}
	}
}

func TestHelpCommand(t *testing.T) {
	r := runCLI(t, "", "--help")
	wantCode(t, r, 0)
	for _, want := range []string{"Exit codes:", "mcp", "status", "version", "--db", "--json"} {
		if !strings.Contains(r.Stdout, want) {
			t.Errorf("help does not contain %q", want)
		}
	}
}
