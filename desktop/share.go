package main

import (
	"path/filepath"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// SharePreview pins the exact reviewed bytes in this desktop process. The
// bounded cache covers current and recently reviewed versions of the dialog.
func (d *Desktop) SharePreview(invRef string, o app.ShareOptions) (app.SharePreview, error) {
	if err := d.ready(); err != nil {
		return app.SharePreview{}, err
	}
	p, err := d.app.SharePreview(d.ctx, invRef, o)
	if err != nil {
		return app.SharePreview{}, mapErr(err)
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	for i := range d.reviewedShares {
		if d.reviewedShares[i].SnapshotID == p.SnapshotID {
			return p, nil
		}
	}
	d.reviewedShares = append(d.reviewedShares, p)
	if len(d.reviewedShares) > 8 {
		d.reviewedShares = d.reviewedShares[len(d.reviewedShares)-8:]
	}
	return p, nil
}

func (d *Desktop) reviewedShare(id string) (app.SharePreview, error) {
	if err := d.ready(); err != nil {
		return app.SharePreview{}, err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, p := range d.reviewedShares {
		if p.SnapshotID == id {
			return p, nil
		}
	}
	return app.SharePreview{}, uiErr(kindConflict, "snapshot_id", "This preview is no longer available. Refresh and review it before sharing.")
}

// CopySharePreview copies the pinned Markdown, without rereading the case.
func (d *Desktop) CopySharePreview(snapshotID string) error {
	p, err := d.reviewedShare(snapshotID)
	if err != nil {
		return err
	}
	return nativeErr("clipboard", d.native.SetClipboard(p.Markdown))
}

// SaveSharePreview saves the pinned Markdown through the native dialog. Even a
// case change while the save dialog is open cannot alter the reviewed bytes.
func (d *Desktop) SaveSharePreview(snapshotID string) (FileResult, error) {
	p, err := d.reviewedShare(snapshotID)
	if err != nil {
		return FileResult{}, err
	}
	path, err := d.native.SaveFile(runtime.SaveDialogOptions{Title: "Save reviewed summary", DefaultFilename: p.InvestigationRef + "-" + p.Mode + ".md", Filters: mdFilter, CanCreateDirectories: true})
	if err != nil {
		return FileResult{}, nativeErr("dialog", err)
	}
	if path == "" {
		return FileResult{Cancelled: true}, nil
	}
	if d.isCasebookFile(path) {
		return FileResult{}, errExportOverData
	}
	if err := writePrivateFile(path, []byte(p.Markdown)); err != nil {
		return FileResult{}, mapErr(err)
	}
	return FileResult{Path: path, FileName: filepath.Base(path)}, nil
}
