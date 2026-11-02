package main

import (
	"context"
	"os"
	"os/exec"
	goruntime "runtime"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// Native is the seam between the Desktop bindings and the Wails runtime
// (D-06, D-09): native file dialogs, the clipboard, quitting and the window
// theme. Tests inject a fake so the bindings run without a Wails window.
//
// The app only ever writes the clipboard; it never reads it.
type Native interface {
	// SaveFile shows a native save dialog. An empty path with a nil error
	// means the user cancelled.
	SaveFile(opts runtime.SaveDialogOptions) (string, error)
	// OpenFile shows a native open dialog. An empty path with a nil error
	// means the user cancelled.
	OpenFile(opts runtime.OpenDialogOptions) (string, error)
	// SetClipboard replaces the clipboard text.
	SetClipboard(text string) error
	// Quit closes the application.
	Quit()
	// SetTheme sets the native window theme: "system", "light" or "dark".
	SetTheme(theme string)
	// OpenFolder shows a folder in the system file manager.
	OpenFolder(dir string) error
	// OpenURL opens url in the person's default browser.
	OpenURL(url string) error
}

// wailsNative implements Native with the Wails runtime. Its ctx is the one
// Wails passes to OnStartup; every runtime call needs it.
type wailsNative struct {
	ctx context.Context
}

func (w *wailsNative) SaveFile(opts runtime.SaveDialogOptions) (string, error) {
	return runtime.SaveFileDialog(w.ctx, opts)
}

func (w *wailsNative) OpenFile(opts runtime.OpenDialogOptions) (string, error) {
	return runtime.OpenFileDialog(w.ctx, opts)
}

func (w *wailsNative) SetClipboard(text string) error {
	return runtime.ClipboardSetText(w.ctx, text)
}

func (w *wailsNative) Quit() {
	runtime.Quit(w.ctx)
}

func (w *wailsNative) SetTheme(theme string) {
	switch theme {
	case "light":
		runtime.WindowSetLightTheme(w.ctx)
	case "dark":
		runtime.WindowSetDarkTheme(w.ctx)
	default:
		runtime.WindowSetSystemDefaultTheme(w.ctx)
	}
}

func (w *wailsNative) OpenURL(url string) error {
	runtime.BrowserOpenURL(w.ctx, url)
	return nil
}

// OpenFolder starts the system file manager on dir (Explorer, Finder or the
// desktop's default through xdg-open) by absolute path (see fileManager),
// without a shell and without waiting for it. dir must be an existing directory; the Wails BrowserOpenURL
// refuses file URLs, so this runs the file manager directly.
func (w *wailsNative) OpenFolder(dir string) error {
	st, err := os.Stat(dir)
	if err != nil {
		return err
	}
	if !st.IsDir() {
		return &os.PathError{Op: "open", Path: dir, Err: os.ErrInvalid}
	}
	cmd := exec.Command(fileManager(goruntime.GOOS, os.Getenv, isFile), dir)
	if err := cmd.Start(); err != nil {
		return err
	}
	// Explorer exits with status 1 even on success; only reap the process.
	go func() { _ = cmd.Wait() }()
	return nil
}

// Absolute paths of the file managers OpenFolder runs, so a writable
// directory early in PATH cannot swap them (the CLI does the same for open).
const (
	windowsDefaultRoot = `C:\Windows`
	macOpen            = "/usr/bin/open"
)

// xdgOpenPaths are the usual absolute locations of xdg-open, tried in order.
var xdgOpenPaths = []string{"/usr/bin/xdg-open", "/bin/xdg-open", "/usr/local/bin/xdg-open"}

// fileManager returns the program OpenFolder runs on goos:
//
//   - Windows: explorer.exe in %SystemRoot% (C:\Windows when it is unset or
//     not an absolute drive path);
//   - macOS: /usr/bin/open;
//   - elsewhere: the first of xdgOpenPaths that is a file, falling back to
//     xdg-open by name for systems that keep it somewhere else (exec then
//     resolves it through PATH and still refuses a match in the current
//     directory).
//
// getenv and isFile are seams so tests resolve every OS's answer without
// touching the machine.
func fileManager(goos string, getenv func(string) string, isFile func(string) bool) string {
	switch goos {
	case "windows":
		root := strings.TrimRight(getenv("SystemRoot"), `\/`)
		if !isDrivePath(root) {
			root = windowsDefaultRoot
		}
		return root + `\explorer.exe`
	case "darwin":
		return macOpen
	default:
		for _, p := range xdgOpenPaths {
			if isFile(p) {
				return p
			}
		}
		return "xdg-open"
	}
}

// isDrivePath reports whether p is an absolute Windows path on a drive, such
// as C:\Windows. It does not use filepath.IsAbs so the rule is the same when
// tests run on another OS.
func isDrivePath(p string) bool {
	if len(p) < 3 || p[1] != ':' || (p[2] != '\\' && p[2] != '/') {
		return false
	}
	c := p[0] | 0x20
	return c >= 'a' && c <= 'z'
}

// isFile reports whether p names a regular file (following symlinks).
func isFile(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.Mode().IsRegular()
}
