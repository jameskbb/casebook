package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/app/apptest"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func TestReviewedSharePreservesBytesAfterAIWrite(t *testing.T) {
	d, native := newTestDesktop(t)
	inv, err := d.CreateInvestigation(app.CreateInvestigationInput{Title: "Reviewed answer"})
	if err != nil {
		t.Fatal(err)
	}
	p, err := d.SharePreview(inv.Ref, app.ShareOptions{Mode: "brief"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = d.app.AddClaim(d.ctx, apptest.AI, inv.Ref, app.ClaimInput{Statement: "Added after preview"})
	if err != nil {
		t.Fatal(err)
	}
	if err := d.CopySharePreview(p.SnapshotID); err != nil {
		t.Fatal(err)
	}
	if native.clipboard != p.Markdown || strings.Contains(native.clipboard, "Added after preview") {
		t.Fatal("copy reread changed case")
	}
	native.savePath = filepath.Join(t.TempDir(), "reviewed.md")
	res, err := d.SaveSharePreview(p.SnapshotID)
	if err != nil || res.Cancelled {
		t.Fatalf("save = %+v %v", res, err)
	}
	got, err := os.ReadFile(native.savePath)
	if err != nil || string(got) != p.Markdown {
		t.Fatal("save did not preserve reviewed bytes")
	}
	// Windows permissions are ACLs, not Unix mode bits.
	if runtime.GOOS != "windows" {
		if info, err := os.Stat(native.savePath); err != nil || info.Mode().Perm() != 0600 {
			t.Fatal("saved file is not private")
		}
	}
	refreshed, err := d.SharePreview(inv.Ref, app.ShareOptions{Mode: "brief"})
	if err != nil || refreshed.SnapshotID == p.SnapshotID || !strings.Contains(refreshed.Markdown, "Added after preview") {
		t.Fatal("refresh did not show new case state")
	}
	if err := d.CopySharePreview(p.SnapshotID); err != nil || native.clipboard != p.Markdown {
		t.Fatal("refresh overwrote previously reviewed bytes")
	}
}

func TestReviewedShareNativeSafetyAndBoundedCache(t *testing.T) {
	d, native := newTestDesktop(t)
	inv, err := d.CreateInvestigation(app.CreateInvestigationInput{Title: "Native safety"})
	if err != nil {
		t.Fatal(err)
	}
	p, err := d.SharePreview(inv.Ref, app.ShareOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if err := d.CopySharePreview("made-up"); err == nil {
		t.Fatal("unknown preview copied")
	}
	if _, err := d.SaveSharePreview("made-up"); err == nil || len(native.saveOpts) > 0 {
		t.Fatal("unknown preview opened a dialog")
	}
	native.savePath = ""
	if r, e := d.SaveSharePreview(p.SnapshotID); e != nil || !r.Cancelled {
		t.Fatal("cancelled save wrote data")
	}
	native.savePath = d.dbPath
	if _, err := d.SaveSharePreview(p.SnapshotID); err == nil {
		t.Fatal("save over live data accepted")
	}
	for i := 0; i < 9; i++ {
		_, err := d.app.AddQuestion(d.ctx, apptest.AI, inv.Ref, app.QuestionInput{Question: "New unknown"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := d.SharePreview(inv.Ref, app.ShareOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	if len(d.reviewedShares) > 8 {
		t.Fatal("unbounded preview cache")
	}
	if err := d.CopySharePreview(p.SnapshotID); err == nil {
		t.Fatal("evicted preview silently regenerated")
	}
}

// mutationDuringSave changes the case after the reviewed bytes were selected,
// while the native save dialog is open.
type mutationDuringSave struct {
	*fakeNative
	mutate func()
}

func (n *mutationDuringSave) SaveFile(o wailsruntime.SaveDialogOptions) (string, error) {
	n.mutate()
	return n.fakeNative.SaveFile(o)
}

func TestReviewedShareUnchangedDuringNativeSaveDialog(t *testing.T) {
	d, native := newTestDesktop(t)
	inv, err := d.CreateInvestigation(app.CreateInvestigationInput{Title: "Before save dialog"})
	if err != nil {
		t.Fatal(err)
	}
	p, err := d.SharePreview(inv.Ref, app.ShareOptions{})
	if err != nil {
		t.Fatal(err)
	}
	native.savePath = filepath.Join(t.TempDir(), "reviewed.md")
	d.native = &mutationDuringSave{fakeNative: native, mutate: func() {
		title := "Changed while dialog open"
		if _, err := d.app.UpdateInvestigation(d.ctx, apptest.AI, inv.Ref, app.InvestigationPatch{Title: &title}); err != nil {
			t.Fatal(err)
		}
	}}
	if _, err := d.SaveSharePreview(p.SnapshotID); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(native.savePath)
	if err != nil || string(got) != p.Markdown {
		t.Fatal("native dialog write changed reviewed bytes")
	}
}
