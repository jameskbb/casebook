package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/jameskbb/casebook/internal/app"
)

// TestExport also checks the export is stable (ADR-0016): the CLI's App runs
// on the real clock and the seeding App on a fixed one, yet the bytes match.
func TestExport(t *testing.T) {
	a, db, _ := seededApp(t)
	want, err := a.ExportInvestigation(t.Context(), "INV-0001")
	if err != nil {
		t.Fatal(err)
	}

	r := runCLI(t, "", "--db", db, "export")
	wantCode(t, r, 0)
	var head struct {
		Format  string `json:"format"`
		Version int    `json:"version"`
	}
	if err := json.Unmarshal([]byte(r.Stdout), &head); err != nil {
		t.Fatalf("export is not JSON: %v", err)
	}
	if head.Format != "casebook.investigation" || head.Version != app.ExportVersion {
		t.Errorf("head = %+v", head)
	}
	if r.Stdout != string(want) {
		t.Errorf("export stdout differs from app.ExportInvestigation:\ngot:\n%s\nwant:\n%s", r.Stdout, want)
	}

	path := filepath.Join(t.TempDir(), "inv.json")
	r = runCLI(t, "", "--db", db, "export", "inv-1", "-o", path)
	wantCode(t, r, 0)
	wantContains(t, r.Stdout, "Exported INV-0001 to "+path)
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Error("exported file differs from app.ExportInvestigation")
	}
	if runtime.GOOS != "windows" {
		fi, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if mode := fi.Mode().Perm(); mode != 0o600 {
			t.Errorf("export file mode = %o, want 600", mode)
		}
	}

	// A refused overwrite leaves the file untouched.
	if err := os.WriteFile(path, []byte("keep me\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	r = runCLI(t, "", "--db", db, "export", "--output", path)
	wantCode(t, r, 1)
	wantContains(t, r.Stderr, "already exists", "--force")
	if b, _ := os.ReadFile(path); string(b) != "keep me\n" {
		t.Errorf("file changed after a refused export: %q", b)
	}

	r = runCLI(t, "", "--db", db, "--json", "export", "-o", path, "--force")
	wantCode(t, r, 0)
	var res map[string]any
	decodeJSON(t, r.Stdout, &res)
	if res["investigation_ref"] != "INV-0001" || res["path"] != path || res["bytes"] != float64(len(got)) {
		t.Errorf("export --json = %v (file %d bytes)", res, len(got))
	}
	if b, _ := os.ReadFile(path); string(b) != string(want) {
		t.Error("--force did not overwrite the file with the export")
	}

	r = runCLI(t, "", "--db", db, "export", "INV-0099")
	wantCode(t, r, 3)
	if !strings.HasPrefix(r.Stderr, "casebook: not found:") {
		t.Errorf("stderr = %q", r.Stderr)
	}
	r = runCLI(t, "", "--db", db, "export", "INV-0099", "-o", filepath.Join(t.TempDir(), "x.json"))
	wantCode(t, r, 3)

	wantCode(t, runCLI(t, "", "--db", db, "inv", "activate", "--clear"), 0)
	r = runCLI(t, "", "--db", db, "export")
	wantCode(t, r, 3)
	if r.Stderr != noActiveStderr {
		t.Errorf("stderr = %q", r.Stderr)
	}
	r = runCLI(t, "", "--db", db, "export", "-o", filepath.Join(t.TempDir(), "missing-dir", "x.json"), "INV-0001")
	wantCode(t, r, 1)
}

func TestImport(t *testing.T) {
	_, db, _ := seededApp(t)
	path := filepath.Join(t.TempDir(), "inv.json")
	wantCode(t, runCLI(t, "", "--db", db, "export", "INV-0001", "-o", path), 0)

	r := runCLI(t, "", "--db", db, "import", path)
	wantCode(t, r, 0)
	wantContains(t, r.Stdout, "Imported INV-0001 as INV-0002", "4 evidence", "3 claims", "2 hypotheses", "1 questions")

	refs := func(ref string) []string {
		t.Helper()
		r := runCLI(t, "", "--db", db, "--json", "investigation", "show", ref)
		wantCode(t, r, 0)
		var d app.InvestigationDetail
		decodeJSON(t, r.Stdout, &d)
		var out []string
		for _, e := range d.Evidence {
			out = append(out, e.Ref)
		}
		for _, c := range d.Claims {
			out = append(out, c.Ref)
		}
		return out
	}
	if a, b := strings.Join(refs("INV-0001"), ","), strings.Join(refs("INV-0002"), ","); a != b || a == "" {
		t.Errorf("imported refs %q differ from the original %q", b, a)
	}
	// Imported rows are marked in the human view; native ones are not (AR-09).
	if show := runCLI(t, "", "--db", db, "investigation", "show", "INV-0002"); !strings.Contains(show.Stdout, "(imported)") {
		t.Errorf("show INV-0002 does not mark imported rows:\n%s", show.Stdout)
	}
	if show := runCLI(t, "", "--db", db, "investigation", "show", "INV-0001"); strings.Contains(show.Stdout, "(imported)") {
		t.Errorf("show INV-0001 marks native rows as imported:\n%s", show.Stdout)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	r = runCLI(t, string(data), "--db", db, "--json", "import", "-")
	wantCode(t, r, 0)
	var res app.ImportResult
	decodeJSON(t, r.Stdout, &res)
	if res.InvestigationRef != "INV-0003" || res.OriginalRef != "INV-0001" || !res.RefChanged || res.Counts.Evidence != 4 {
		t.Errorf("import result = %+v", res)
	}

	garbage := filepath.Join(t.TempDir(), "garbage.json")
	if err := os.WriteFile(garbage, []byte("this is not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	r = runCLI(t, "", "--db", db, "import", garbage)
	wantCode(t, r, 2)
	if !strings.HasPrefix(r.Stderr, "casebook: invalid data:") {
		t.Errorf("stderr = %q", r.Stderr)
	}

	big := filepath.Join(t.TempDir(), "big.json")
	if err := os.WriteFile(big, make([]byte, app.MaxImportBytes+10), 0o600); err != nil {
		t.Fatal(err)
	}
	r = runCLI(t, "", "--db", db, "import", big)
	wantCode(t, r, 2)
	if !strings.HasPrefix(r.Stderr, "casebook: invalid data:") {
		t.Errorf("oversize stderr = %q", r.Stderr)
	}

	r = runCLI(t, "", "--db", db, "import", filepath.Join(t.TempDir(), "missing.json"))
	wantCode(t, r, 1)
	if !strings.HasPrefix(r.Stderr, "casebook: ") || strings.Count(r.Stderr, "\n") != 1 {
		t.Errorf("stderr = %q", r.Stderr)
	}
	wantCode(t, runCLI(t, "", "--db", db, "import"), 2)
	wantCode(t, runCLI(t, "", "--db", db, "import", path, path), 2)
}

// TestImportRefusesNonRegularFile is S7: like the desktop, the CLI imports
// only a regular file. A directory stands in for devices on every OS; the
// FIFO case is in portability_fifo_test.go.
func TestImportRefusesNonRegularFile(t *testing.T) {
	_, db, _ := seededApp(t)
	r := runCLI(t, "", "--db", db, "import", t.TempDir())
	wantCode(t, r, 2)
	if r.Stderr != "casebook: invalid data: must be a regular file\n" {
		t.Errorf("stderr = %q", r.Stderr)
	}
}

// TestExportForce is the WR-04 regression: --force replaces the file
// atomically with an owner-only one, never targets the live database and
// leaves the old file and no temporary file behind when it fails.
func TestExportForce(t *testing.T) {
	_, db, _ := seededApp(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "inv.json")
	if err := os.WriteFile(path, []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	noTemps := func(t *testing.T) {
		t.Helper()
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), ".casebook-export-") {
				t.Errorf("temporary file %s left behind", e.Name())
			}
		}
	}

	t.Run("replaces with an owner-only file", func(t *testing.T) {
		wantCode(t, runCLI(t, "", "--db", db, "export", "-o", path, "--force"), 0)
		b, err := os.ReadFile(path)
		if err != nil || !strings.Contains(string(b), `"casebook.investigation"`) {
			t.Fatalf("file not replaced with the export: %q %v", b, err)
		}
		if runtime.GOOS != "windows" {
			fi, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if mode := fi.Mode().Perm(); mode != 0o600 {
				t.Errorf("mode = %o, want 600", mode)
			}
		}
		noTemps(t)
	})

	t.Run("refuses the live database", func(t *testing.T) {
		before, err := os.ReadFile(db)
		if err != nil {
			t.Fatal(err)
		}
		targets := []string{db}
		if runtime.GOOS != "windows" {
			link := filepath.Join(dir, "db-link")
			if err := os.Symlink(db, link); err != nil {
				t.Fatal(err)
			}
			targets = append(targets, link)
		}
		for _, target := range targets {
			for _, force := range []bool{false, true} {
				args := []string{"--db", db, "export", "-o", target}
				if force {
					args = append(args, "--force")
				}
				r := runCLI(t, "", args...)
				wantCode(t, r, 1)
				wantContains(t, r.Stderr, "refusing to overwrite the live database")
			}
		}
		after, err := os.ReadFile(db)
		if err != nil {
			t.Fatal(err)
		}
		if string(after) != string(before) {
			t.Error("the database file changed")
		}
		wantCode(t, runCLI(t, "", "--db", db, "export"), 0)
	})

	t.Run("failure keeps the old file", func(t *testing.T) {
		if runtime.GOOS == "windows" || os.Geteuid() == 0 {
			t.Skip("needs a directory the user cannot write")
		}
		if err := os.WriteFile(path, []byte("keep me\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(dir, 0o500); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
		wantCode(t, runCLI(t, "", "--db", db, "export", "-o", path, "--force"), 1)
		if b, _ := os.ReadFile(path); string(b) != "keep me\n" {
			t.Errorf("file changed after a failed export: %q", b)
		}
		noTemps(t)
	})
}

// TestShareThroughRepo is the "share a case through your repo" flow: export
// to .casebook/INV-0001.json with --out, export again unchanged (same
// bytes, even after activating it), import on another database, and import
// again (a separate copy, named as the same case).
func TestShareThroughRepo(t *testing.T) {
	_, db, _ := seededApp(t)
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".casebook"), 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(repo, ".casebook", "INV-0001.json")
	wantCode(t, runCLI(t, "", "--db", db, "export", "INV-0001", "--out", path), 0)
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	wantCode(t, runCLI(t, "", "--db", db, "investigation", "activate", "--clear"), 0)
	wantCode(t, runCLI(t, "", "--db", db, "investigation", "activate", "INV-0001"), 0)
	wantCode(t, runCLI(t, "", "--db", db, "export", "INV-0001", "--out", path, "--force"), 0)
	second, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatalf("re-export of an unchanged case changed the file:\n%s\n---\n%s", first, second)
	}

	teammate := filepath.Join(t.TempDir(), "casebook.db")
	r := runCLI(t, "", "--db", teammate, "import", path)
	wantCode(t, r, 0)
	if strings.Contains(r.Stdout, "same case") {
		t.Fatalf("first import mentions a same case: %q", r.Stdout)
	}
	r = runCLI(t, "", "--db", teammate, "import", path)
	wantCode(t, r, 0)
	wantContains(t, r.Stdout, "Imported INV-0001 as INV-0002")
	wantContains(t, r.Stdout, "The same case is already here as INV-0001; INV-0002 is a separate copy.")
	r = runCLI(t, "", "--db", teammate, "--json", "import", path)
	wantCode(t, r, 0)
	var res app.ImportResult
	decodeJSON(t, r.Stdout, &res)
	if res.InvestigationRef != "INV-0003" || strings.Join(res.SameCaseRefs, ",") != "INV-0001,INV-0002" {
		t.Fatalf("third import = %+v", res)
	}
}
