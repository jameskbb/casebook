package mcpserver

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/app/apptest"
)

// TestDirectoryActiveInvestigation: a server working in a directory bound
// to INV-0002 resolves omitted refs to it, reports active_dir, and
// casebook_set_active_investigation re-points the binding, not the global
// setting.
func TestDirectoryActiveInvestigation(t *testing.T) {
	ctx := context.Background()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	global, _ := seeded(t)
	if _, err := global.CreateInvestigation(ctx, apptest.Human, app.CreateInvestigationInput{Title: "Second"}); err != nil {
		t.Fatal(err)
	}
	if _, err := global.CreateInvestigation(ctx, apptest.Human, app.CreateInvestigationInput{Title: "Third"}); err != nil {
		t.Fatal(err)
	}
	if _, err := global.BindDirectory(ctx, apptest.Human, dir, "INV-0002"); err != nil {
		t.Fatal(err)
	}
	a := apptest.Reopen(t, global.DBPath(), app.WithWorkDir(dir))
	cs := newTestSession(t, a, sessionOpts{})

	out := decode[activeOutput](t, callTool(t, cs, "casebook_get_active_investigation", nil))
	if out.Investigation == nil || out.Investigation.Ref != "INV-0002" || out.ActiveDir != dir {
		t.Fatalf("get_active = %+v", out)
	}
	res := callTool(t, cs, "casebook_get_context", map[string]any{})
	if !strings.Contains(resultText(res), "INV-0002") {
		t.Fatalf("get_context without a ref did not use the directory's investigation:\n%s", resultText(res))
	}

	callTool(t, cs, "casebook_set_active_investigation", map[string]any{"investigation_ref": "INV-0003"})
	info, err := a.ActiveInvestigationInfo(ctx)
	if err != nil || info.Investigation.Ref != "INV-0003" || info.Dir != dir || info.GlobalRef != "INV-0001" {
		t.Fatalf("after set_active: %+v, %v", info, err)
	}
}
