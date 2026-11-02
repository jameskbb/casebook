package cli

import (
	"fmt"
	"strings"
	"testing"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/app/apptest"
)

func TestContextFormats(t *testing.T) {
	a, db, _ := seededApp(t)
	ctx := t.Context()
	render := func(format string, o app.ContextOptions) string {
		t.Helper()
		s, err := a.RenderContext(ctx, "INV-0001", format, o)
		if err != nil {
			t.Fatal(err)
		}
		return withNewline(s)
	}
	md := render("markdown", app.ContextOptions{})
	compact := render("markdown", app.ContextOptions{Compact: true})
	js := render("json", app.ContextOptions{})
	capped := render("markdown", app.ContextOptions{MaxBytes: 1500})
	if md == capped || md == js {
		t.Fatalf("fixture renders the same pack for every option (md %d, capped %d bytes); the test would prove nothing", len(md), len(capped))
	}

	cases := []struct {
		name string
		args []string
		want string
	}{
		{"default active", []string{"context"}, md},
		{"explicit ref", []string{"context", "inv-1"}, md},
		{"format md", []string{"context", "--format", "md"}, md},
		{"format markdown", []string{"context", "--format", "markdown"}, md},
		{"format upper", []string{"context", "--format", "MD"}, md},
		{"compact", []string{"context", "INV-0001", "--format", "compact"}, compact},
		{"format json", []string{"context", "--format", "json"}, js},
		{"json flag", []string{"context", "--json"}, js},
		{"json flag and format json", []string{"context", "--json", "--format", "json"}, js},
		{"max bytes", []string{"context", "--max-bytes", "1500"}, capped},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := runCLI(t, "", append([]string{"--db", db}, c.args...)...)
			wantCode(t, r, 0)
			if r.Stdout != c.want {
				t.Errorf("stdout differs from app.RenderContext:\ngot:\n%s\nwant:\n%s", r.Stdout, c.want)
			}
			if r.Stderr != "" {
				t.Errorf("stderr = %q", r.Stderr)
			}
		})
	}
}

// TestContextCompactLarge proves --format compact reaches ContextOptions:
// the fixture fits the 6000 byte compact budget, so a larger investigation
// is needed for compact output to differ from the full Markdown.
func TestContextCompactLarge(t *testing.T) {
	a, db, _ := seededApp(t)
	ctx := t.Context()
	inv, err := a.CreateInvestigation(ctx, apptest.Human, app.CreateInvestigationInput{Title: "Large investigation"})
	if err != nil {
		t.Fatal(err)
	}
	for i := range 8 {
		stmt := fmt.Sprintf("Claim %d: %s", i, strings.Repeat("long statement text ", 90))
		if _, err := a.AddClaim(ctx, apptest.Human, inv.Ref, app.ClaimInput{Statement: stmt}); err != nil {
			t.Fatal(err)
		}
	}
	full, err := a.RenderContext(ctx, inv.Ref, "markdown", app.ContextOptions{})
	if err != nil {
		t.Fatal(err)
	}
	compact, err := a.RenderContext(ctx, inv.Ref, "markdown", app.ContextOptions{Compact: true})
	if err != nil {
		t.Fatal(err)
	}
	if full == compact {
		t.Fatalf("compact equals full Markdown (%d bytes); the test would prove nothing", len(full))
	}
	r := runCLI(t, "", "--db", db, "context", inv.Ref, "--format", "compact")
	wantCode(t, r, 0)
	if r.Stdout != withNewline(compact) {
		t.Errorf("compact stdout differs from app.RenderContext with Compact:\ngot %d bytes, want %d", len(r.Stdout), len(compact))
	}
}

func TestContextErrors(t *testing.T) {
	_, db, _ := seededApp(t)
	cases := []struct {
		name   string
		args   []string
		code   int
		stderr string
	}{
		{"unknown format", []string{"context", "--format", "xml"}, 2, "casebook: invalid format:"},
		{"json conflicts with md", []string{"context", "--json", "--format", "md"}, 2, "casebook: --json conflicts with --format md"},
		{"negative max bytes", []string{"context", "--max-bytes", "-1"}, 2, "casebook: invalid"},
		{"bad max bytes", []string{"context", "--max-bytes", "lots"}, 2, "casebook: "},
		{"not found", []string{"context", "INV-0099"}, 3, "casebook: not found:"},
		{"two refs", []string{"context", "INV-0001", "INV-0002"}, 2, "casebook: "},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := runCLI(t, "", append([]string{"--db", db}, c.args...)...)
			wantCode(t, r, c.code)
			if !strings.HasPrefix(r.Stderr, c.stderr) || strings.Count(r.Stderr, "\n") != 1 {
				t.Errorf("stderr = %q, want one line starting %q", r.Stderr, c.stderr)
			}
			if r.Stdout != "" {
				t.Errorf("stdout = %q, want empty", r.Stdout)
			}
		})
	}
}
