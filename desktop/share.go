package main

import (
	"path/filepath"
	"strings"

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
	return app.SharePreview{}, uiErr(kindConflict, "snapshot_id", "This is no longer the latest answer. Close this window and choose Share the answer again.")
}

// CopySharePreview copies the pinned answer as plain text with no marks,
// without rereading the question. The page copies rich text itself and uses
// this when it cannot.
func (d *Desktop) CopySharePreview(snapshotID string) error {
	p, err := d.reviewedShare(snapshotID)
	if err != nil {
		return err
	}
	return nativeErr("clipboard", d.native.SetClipboard(p.Text))
}

// SaveSharePreview saves the pinned answer through the native dialog. Even a
// change to the question while the save dialog is open cannot alter what the
// person read.
func (d *Desktop) SaveSharePreview(snapshotID string) (FileResult, error) {
	p, err := d.reviewedShare(snapshotID)
	if err != nil {
		return FileResult{}, err
	}
	return d.saveAnswer(p)
}

var htmlFilter = []runtime.FileFilter{{DisplayName: "Web page (*.html)", Pattern: "*.html"}}

// saveAnswer asks where to save a shared answer and writes it as a web page
// (app.AnswerPage around the HTML of the preview), which opens on a
// double-click on any computer. The page title and the suggested file name
// are the question, the first line of the Markdown.
func (d *Desktop) saveAnswer(p app.SharePreview) (FileResult, error) {
	title, _, _ := strings.Cut(strings.TrimPrefix(p.Markdown, "# "), "\n")
	path, err := d.native.SaveFile(runtime.SaveDialogOptions{Title: "Save the answer", DefaultFilename: answerFileName(title, p.Mode), Filters: htmlFilter, CanCreateDirectories: true})
	if err != nil {
		return FileResult{}, nativeErr("dialog", err)
	}
	if path == "" {
		return FileResult{Cancelled: true}, nil
	}
	if d.isCasebookFile(path) {
		return FileResult{}, errExportOverData
	}
	if err := writePrivateFile(path, []byte(app.AnswerPage(title, p.HTML))); err != nil {
		return FileResult{}, mapErr(err)
	}
	return FileResult{Path: path, FileName: filepath.Base(path)}, nil
}

// answerFileName is the suggested file name of a saved answer: the question
// with the characters a file name cannot hold removed, cut at a word to at
// most 80 bytes ("Why did calls go up 30 percent in October.html"), then
// " (everything)" for the full version. A question with nothing usable left
// is "answer".
// shortcut: a title that is a reserved Windows device name (CON, NUL) is not
// renamed; the save dialog refuses it and the person types another name.
func answerFileName(title, mode string) string {
	clean := strings.Map(func(r rune) rune {
		if r < ' ' || strings.ContainsRune(`<>:"/\|?*`, r) {
			return -1
		}
		return r
	}, strings.ReplaceAll(title, "%", " percent"))
	name := ""
	for _, word := range strings.Fields(clean) {
		if len(name)+1+len(word) > 80 {
			break
		}
		name = strings.TrimSpace(name + " " + word)
	}
	if name = strings.TrimRight(name, ". "); name == "" {
		name = "answer"
	}
	if mode == "report" {
		name += " (everything)"
	}
	return name + ".html"
}
