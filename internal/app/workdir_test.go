package app_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/app/apptest"
)

// repoTree makes <tmp>/repoA (with a .git folder) holding sub/deep, and
// <tmp>/repoB (with a .git file, like a worktree), and returns their paths
// with symlinks resolved.
func repoTree(t *testing.T) (repoA, deep, repoB string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repoA = filepath.Join(root, "repoA")
	deep = filepath.Join(repoA, "sub", "deep")
	repoB = filepath.Join(root, "repoB")
	for _, d := range []string{filepath.Join(repoA, ".git"), deep, repoB} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(repoB, ".git"), []byte("gitdir: elsewhere\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return repoA, deep, repoB
}

func TestProjectRoot(t *testing.T) {
	repoA, deep, repoB := repoTree(t)
	for _, tc := range []struct{ in, want string }{
		{deep, repoA},
		{repoA, repoA},
		{repoB, repoB},
	} {
		got, err := app.ProjectRoot(tc.in)
		if err != nil || got != tc.want {
			t.Errorf("ProjectRoot(%s) = %q, %v; want %q", tc.in, got, err, tc.want)
		}
	}
	plain := t.TempDir()
	if got, err := app.ProjectRoot(plain); err != nil || filepath.Base(got) != filepath.Base(plain) {
		t.Errorf("ProjectRoot(no repo) = %q, %v; want the directory itself", got, err)
	}
	if _, err := app.ProjectRoot(""); err == nil {
		t.Error("ProjectRoot(\"\") succeeded")
	}
}

// TestDirectoryBindingWinsOverGlobal binds repoA to INV-0002 while INV-0001
// is globally active: Apps working inside repoA see INV-0002, Apps elsewhere
// (and the desktop, with no working directory) still see INV-0001.
func TestDirectoryBindingWinsOverGlobal(t *testing.T) {
	ctx := context.Background()
	repoA, deep, repoB := repoTree(t)
	path := filepath.Join(t.TempDir(), "casebook.db")
	global := apptest.Reopen(t, path)
	newInv(t, global, "First")
	newInv(t, global, "Second")
	if _, err := global.SetActiveInvestigation(ctx, apptest.Human, "INV-0001"); err != nil {
		t.Fatal(err)
	}

	inA := apptest.Reopen(t, path, app.WithWorkDir(deep))
	if inA.WorkDir() != deep {
		t.Fatalf("WorkDir = %q, want %q", inA.WorkDir(), deep)
	}
	b, err := inA.BindDirectory(ctx, apptest.Human, "", "inv-2")
	if err != nil {
		t.Fatal(err)
	}
	if b.Dir != repoA || b.InvestigationRef != "INV-0002" || b.Title != "Second" {
		t.Fatalf("BindDirectory = %+v, want %s bound to INV-0002", b, repoA)
	}
	// Rebinding to the same investigation is a no-op with no activity.
	if _, err := inA.BindDirectory(ctx, apptest.Human, "", "INV-0002"); err != nil {
		t.Fatal(err)
	}
	wantActions(t, global, "INV-0002", "investigation.create", "active.bind")

	got, err := inA.GetInvestigation(ctx, "")
	if err != nil || got.Ref != "INV-0002" || !got.IsActive {
		t.Fatalf("GetInvestigation(\"\") in repoA = %+v, %v", got, err)
	}
	if first, _ := inA.GetInvestigation(ctx, "INV-0001"); first.IsActive {
		t.Fatal("INV-0001 reports IsActive inside repoA")
	}
	info, err := inA.ActiveInvestigationInfo(ctx)
	if err != nil || info.Investigation == nil || info.Investigation.Ref != "INV-0002" ||
		info.Scope != app.ActiveScopeDirectory || info.Dir != repoA || info.GlobalRef != "INV-0001" {
		t.Fatalf("ActiveInvestigationInfo in repoA = %+v, %v", info, err)
	}
	sums, err := inA.ListInvestigations(ctx, app.ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range sums {
		if s.IsActive != (s.Ref == "INV-0002") {
			t.Fatalf("summary %s IsActive = %v in repoA", s.Ref, s.IsActive)
		}
	}
	st, err := inA.Status(ctx)
	if err != nil || st.ActiveInvestigationRef != "INV-0002" || st.ActiveDir != repoA {
		t.Fatalf("Status in repoA = %+v, %v", st, err)
	}

	for name, other := range map[string]*app.App{
		"desktop": global,
		"repoB":   apptest.Reopen(t, path, app.WithWorkDir(repoB)),
	} {
		act, err := other.GetActiveInvestigation(ctx)
		if err != nil || act == nil || act.Ref != "INV-0001" {
			t.Fatalf("%s: GetActiveInvestigation = %+v, %v; want INV-0001", name, act, err)
		}
		info, err := other.ActiveInvestigationInfo(ctx)
		if err != nil || info.Scope != app.ActiveScopeGlobal || info.Dir != "" {
			t.Fatalf("%s: ActiveInvestigationInfo = %+v, %v", name, info, err)
		}
	}

	list, err := global.ListDirectoryBindings(ctx)
	if err != nil || len(list) != 1 || list[0].Dir != repoA || list[0].InvestigationRef != "INV-0002" {
		t.Fatalf("ListDirectoryBindings = %+v, %v", list, err)
	}
}

// TestSetActiveRepointsBinding: inside a bound directory, setting the active
// investigation re-points the binding and leaves the global one alone;
// SetGlobalActiveInvestigation changes only the global one.
func TestSetActiveRepointsBinding(t *testing.T) {
	ctx := context.Background()
	repoA, deep, _ := repoTree(t)
	a := apptest.Open(t, app.WithWorkDir(deep))
	newInv(t, a, "First")
	newInv(t, a, "Second")
	newInv(t, a, "Third")

	// No binding yet: set is global.
	if _, err := a.SetActiveInvestigation(ctx, apptest.Human, "INV-0001"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.BindDirectory(ctx, apptest.Human, repoA, "INV-0002"); err != nil {
		t.Fatal(err)
	}
	inv, err := a.SetActiveInvestigation(ctx, apptest.Human, "INV-0003")
	if err != nil || inv.Ref != "INV-0003" || !inv.IsActive {
		t.Fatalf("SetActiveInvestigation = %+v, %v", inv, err)
	}
	info, _ := a.ActiveInvestigationInfo(ctx)
	if info.Investigation.Ref != "INV-0003" || info.Scope != app.ActiveScopeDirectory || info.GlobalRef != "INV-0001" {
		t.Fatalf("after set in bound dir: %+v", info)
	}
	wantActions(t, a, "INV-0003", "investigation.create", "active.bind")

	inv, err = a.SetGlobalActiveInvestigation(ctx, apptest.Human, "INV-0002")
	if err != nil || inv.IsActive {
		t.Fatalf("SetGlobalActiveInvestigation = %+v, %v; want IsActive false under the binding", inv, err)
	}
	info, _ = a.ActiveInvestigationInfo(ctx)
	if info.Investigation.Ref != "INV-0003" || info.GlobalRef != "INV-0002" {
		t.Fatalf("after global set: %+v", info)
	}

	// Creating with MakeActive inside the bound directory re-points it too.
	created, err := a.CreateInvestigation(ctx, apptest.Human, app.CreateInvestigationInput{Title: "Fourth", MakeActive: true})
	if err != nil || !created.IsActive {
		t.Fatalf("CreateInvestigation = %+v, %v", created, err)
	}
	info, _ = a.ActiveInvestigationInfo(ctx)
	if info.Investigation.Ref != "INV-0004" || info.GlobalRef != "INV-0002" {
		t.Fatalf("after create: %+v", info)
	}

	// Unbinding from a subdirectory removes the applying binding.
	dir, err := a.UnbindDirectory(ctx, apptest.Human, "")
	if err != nil || dir != repoA {
		t.Fatalf("UnbindDirectory = %q, %v", dir, err)
	}
	if dir, err := a.UnbindDirectory(ctx, apptest.Human, ""); err != nil || dir != "" {
		t.Fatalf("second UnbindDirectory = %q, %v", dir, err)
	}
	info, _ = a.ActiveInvestigationInfo(ctx)
	if info.Investigation.Ref != "INV-0002" || info.Scope != app.ActiveScopeGlobal {
		t.Fatalf("after unbind: %+v", info)
	}
}

func TestBindDirectoryValidationAndCascade(t *testing.T) {
	ctx := context.Background()
	repoA, _, _ := repoTree(t)
	a := apptest.Open(t)
	newInv(t, a, "First")

	_, err := a.BindDirectory(ctx, apptest.Human, "", "INV-0001")
	wantValidation(t, err, "dir")
	_, err = a.BindDirectory(ctx, apptest.Human, repoA, "")
	wantValidation(t, err, "investigation_ref")
	_, err = a.UnbindDirectory(ctx, apptest.Human, "")
	wantValidation(t, err, "dir")

	if _, err := a.BindDirectory(ctx, apptest.Human, repoA, "INV-0001"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.ArchiveInvestigation(ctx, apptest.Human, "INV-0001"); err != nil {
		t.Fatal(err)
	}
	if err := a.DeleteInvestigation(ctx, apptest.Human, "INV-0001"); err != nil {
		t.Fatal(err)
	}
	list, err := a.ListDirectoryBindings(ctx)
	if err != nil || len(list) != 0 {
		t.Fatalf("bindings after delete = %+v, %v", list, err)
	}
}

// TestDirectoryBindingCaseFolding: where file names are case-insensitive
// (Windows), a binding matches any spelling of its directory but keeps and
// reports the spelling it was made with; rebinding under another spelling
// keeps one row. With exact matching each spelling is its own binding.
func TestDirectoryBindingCaseFolding(t *testing.T) {
	ctx := context.Background()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	// The directories do not exist, so cleaning keeps their spelling.
	shop := filepath.Join(root, "Shop")
	lower := filepath.Join(root, "shop")
	path := filepath.Join(t.TempDir(), "casebook.db")

	restore := app.SetFoldDirCase(true)
	defer restore()
	a := apptest.Reopen(t, path)
	newInv(t, a, "First")
	newInv(t, a, "Second")
	if b, err := a.BindDirectory(ctx, apptest.Human, shop, "INV-0001"); err != nil || b.Dir != shop {
		t.Fatalf("BindDirectory = %+v, %v; want Dir %q", b, err, shop)
	}
	in := apptest.Reopen(t, path, app.WithWorkDir(filepath.Join(root, "SHOP", "sub")))
	info, err := in.ActiveInvestigationInfo(ctx)
	if err != nil || info.Investigation == nil || info.Investigation.Ref != "INV-0001" || info.Dir != shop {
		t.Fatalf("ActiveInvestigationInfo under SHOP/sub = %+v, %v; want INV-0001 for %q", info, err, shop)
	}
	if b, err := a.BindDirectory(ctx, apptest.Human, lower, "INV-0001"); err != nil || b.Dir != shop {
		t.Fatalf("no-op rebind = %+v, %v; want the stored spelling %q", b, err, shop)
	}
	if _, err := a.BindDirectory(ctx, apptest.Human, lower, "INV-0002"); err != nil {
		t.Fatal(err)
	}
	list, err := a.ListDirectoryBindings(ctx)
	if err != nil || len(list) != 1 || list[0].Dir != lower || list[0].InvestigationRef != "INV-0002" {
		t.Fatalf("ListDirectoryBindings after respelled rebind = %+v, %v", list, err)
	}
	if dir, err := a.UnbindDirectory(ctx, apptest.Human, filepath.Join(root, "SHOP")); err != nil || dir != lower {
		t.Fatalf("UnbindDirectory(SHOP) = %q, %v; want %q", dir, err, lower)
	}

	app.SetFoldDirCase(false)
	for i, d := range []string{shop, lower} {
		if _, err := a.BindDirectory(ctx, apptest.Human, d, "INV-0001"); err != nil {
			t.Fatalf("bind %d: %v", i, err)
		}
	}
	if list, err := a.ListDirectoryBindings(ctx); err != nil || len(list) != 2 {
		t.Fatalf("exact matching: bindings = %+v, %v; want 2", list, err)
	}
}
