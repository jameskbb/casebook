package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jameskbb/casebook/internal/app"
)

func TestReportCommand(t *testing.T) {
	a, db, _ := seededApp(t)
	ctx := t.Context()
	want, err := a.Report(ctx, "INV-0001", app.ReportOptions{})
	if err != nil {
		t.Fatal(err)
	}
	short, err := a.Report(ctx, "INV-0001", app.ReportOptions{TimelineLimit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if want == short || !strings.HasPrefix(want, "# INV-0001: ") {
		t.Fatalf("fixture report looks wrong:\n%s", want)
	}

	for _, c := range []struct {
		name string
		args []string
		want string
	}{
		{"active", []string{"report"}, want},
		{"explicit ref", []string{"report", "inv-1"}, want},
		{"timeline", []string{"report", "--timeline", "1"}, short},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := runCLI(t, "", append([]string{"--db", db}, c.args...)...)
			wantCode(t, r, 0)
			if r.Stdout != withNewline(c.want) || r.Stderr != "" {
				t.Errorf("stdout differs from app.Report (stderr %q):\n%s", r.Stderr, r.Stdout)
			}
		})
	}

	r := runCLI(t, "", "--db", db, "report", "--json")
	wantCode(t, r, 0)
	var doc struct {
		InvestigationRef string `json:"investigation_ref"`
		Report           string `json:"report"`
	}
	if err := json.Unmarshal([]byte(r.Stdout), &doc); err != nil || doc.InvestigationRef != "INV-0001" || doc.Report != want {
		t.Fatalf("--json = %+v, %v", doc, err)
	}

	path := filepath.Join(t.TempDir(), "report.md")
	r = runCLI(t, "", "--db", db, "report", "-o", path)
	wantCode(t, r, 0)
	got, err := os.ReadFile(path)
	if err != nil || string(got) != want {
		t.Fatalf("-o file = %q, %v", got, err)
	}
	r = runCLI(t, "", "--db", db, "report", "-o", path)
	if r.Code == 0 {
		t.Fatal("-o overwrote an existing file without --force")
	}
	r = runCLI(t, "", "--db", db, "report", "-o", path, "--force")
	wantCode(t, r, 0)

	r = runCLI(t, "", "--db", db, "report", "INV-0099")
	if r.Code == 0 {
		t.Fatal("unknown investigation accepted")
	}
}

func TestReportBriefCommand(t *testing.T) {
	a, db, _ := seededApp(t)
	p, err := a.SharePreview(t.Context(), "INV-0001", app.ShareOptions{Mode: "brief"})
	if err != nil {
		t.Fatal(err)
	}
	want := p.Markdown
	if !strings.Contains(want, "Leadership brief.") || !strings.Contains(want, "## Scope and omissions") {
		t.Fatalf("fixture brief looks wrong:\n%s", want)
	}

	for _, args := range [][]string{{"report", "--brief"}, {"report", "inv-1", "--brief"}} {
		r := runCLI(t, "", append([]string{"--db", db}, args...)...)
		wantCode(t, r, 0)
		if r.Stdout != withNewline(want) || r.Stderr != "" {
			t.Errorf("%v: stdout differs from the Share summary brief (stderr %q):\n%s", args, r.Stderr, r.Stdout)
		}
	}

	r := runCLI(t, "", "--db", db, "report", "--brief", "--json")
	wantCode(t, r, 0)
	var doc struct {
		InvestigationRef string `json:"investigation_ref"`
		Mode             string `json:"mode"`
		Report           string `json:"report"`
	}
	if err := json.Unmarshal([]byte(r.Stdout), &doc); err != nil || doc.InvestigationRef != "INV-0001" || doc.Mode != "brief" || doc.Report != want {
		t.Fatalf("--brief --json = %+v, %v", doc, err)
	}
	r = runCLI(t, "", "--db", db, "report", "--json")
	wantCode(t, r, 0)
	if err := json.Unmarshal([]byte(r.Stdout), &doc); err != nil || doc.Mode != "report" {
		t.Fatalf("--json mode = %q, %v", doc.Mode, err)
	}

	path := filepath.Join(t.TempDir(), "brief.md")
	r = runCLI(t, "", "--db", db, "report", "--brief", "-o", path)
	wantCode(t, r, 0)
	if !strings.Contains(r.Stdout, "leadership brief") {
		t.Errorf("-o message = %q", r.Stdout)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != want {
		t.Fatalf("-o file = %q, %v", got, err)
	}
	r = runCLI(t, "", "--db", db, "report", "--brief", "-o", path)
	if r.Code == 0 {
		t.Fatal("--brief -o overwrote an existing file without --force")
	}
	r = runCLI(t, "", "--db", db, "report", "--brief", "-o", path, "--force")
	wantCode(t, r, 0)

	r = runCLI(t, "", "--db", db, "report", "--brief", "--timeline", "5")
	wantCode(t, r, 2)
	if !strings.Contains(r.Stderr, "--timeline cannot be used with --brief") {
		t.Errorf("stderr = %q", r.Stderr)
	}
}

func TestEvernoteCommandRemoved(t *testing.T) {
	_, db, _ := seededApp(t)
	for _, args := range [][]string{{"evernote"}, {"evernote", "preview", "--enex", "x.enex"}, {"evernote", "--help"}} {
		r := runCLI(t, "", append([]string{"--db", db}, args...)...)
		wantCode(t, r, 1)
		if r.Stdout != "" || !strings.HasPrefix(r.Stderr, "casebook: the Evernote commands were removed") ||
			!strings.Contains(r.Stderr, "casebook report") || !strings.Contains(r.Stderr, "--brief") ||
			!strings.Contains(r.Stderr, "Share summary") || !strings.Contains(r.Stderr, "save the brief or report into an app it is connected to") || !strings.Contains(r.Stderr, "docs/sharing.md") ||
			strings.Count(r.Stderr, "\n") != 1 {
			t.Errorf("%v: stdout %q stderr %q", args, r.Stdout, r.Stderr)
		}
	}
	r := runCLI(t, "", "--db", db, "--help")
	wantCode(t, r, 0)
	if strings.Contains(strings.ToLower(r.Stdout), "evernote") {
		t.Errorf("help lists the removed evernote command:\n%s", r.Stdout)
	}
}
