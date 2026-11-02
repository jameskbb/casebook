// Command Casebook is the desktop app (D-01): a Wails v2 window over the
// same internal/app engine and per-user database the CLI and MCP server use.
package main

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/paths"
)

// assets is the built frontend. frontend/dist/.gitkeep is tracked so this
// embed compiles on a fresh clone before the first npm build (D-03).
//
//go:embed all:frontend/dist
var assets embed.FS

func main() {
	cfg, err := parseFlags(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "casebook:", err)
		os.Exit(2)
	}

	if cfg.SeedDemo {
		maybeSeedDemo(cfg)
	}

	// An open failure does not abort: the window still starts and shows the
	// StartupError screen with the reason (D-05).
	a, dbPath, openErr := openApp(cfg)
	d := newDesktop(a, openErr, dbPath, nil)
	d.explicitDB = usesExplicitDB(cfg, os.Getenv)

	err = wails.Run(&options.App{
		Title:            "Casebook",
		Width:            1280,
		Height:           840,
		MinWidth:         960,
		MinHeight:        640,
		BackgroundColour: startupBackground(systemPrefersDark()),
		AssetServer:      &assetserver.Options{Assets: assets},
		OnStartup:        d.startup,
		OnShutdown:       d.shutdown,
		Bind:             []interface{}{d},
		// A file dropped on the window must never navigate the WebView.
		DragAndDrop: &options.DragAndDrop{DisableWebViewDrop: true},
		Windows:     windowsOptions(),
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "casebook:", err)
	}
}

// openApp opens the engine: the per-user database through app.OpenDefault
// (the same file as the CLI, D-01), or <dir>/casebook.db for --data-dir
// (D-08). It returns the database path to display even when opening failed.
func openApp(cfg flags) (*app.App, string, error) {
	dbPath, err := dbPathFor(cfg)
	if err != nil {
		return nil, dbPath, err
	}
	var a *app.App
	if cfg.DataDir == "" {
		a, err = app.OpenDefault()
	} else {
		a, err = app.Open(dbPath)
	}
	if err != nil {
		return nil, dbPath, err
	}
	return a, a.DBPath(), nil
}

// maybeSeedDemo runs the hidden --seed-demo flag (D-08). It seeds only an
// empty database whose resolved path is not the per-user default (seedDemo
// checks both); otherwise it prints one line and the app starts normally.
func maybeSeedDemo(cfg flags) {
	dbPath, err := dbPathFor(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "casebook: --seed-demo:", err)
		return
	}
	if err := seedDemo(dbPath, time.Now(), cfg.SeedDemoStage); err != nil {
		fmt.Fprintln(os.Stderr, "casebook: --seed-demo:", err)
	}
}

// dbPathFor resolves the database file the flags select.
func dbPathFor(cfg flags) (string, error) {
	if cfg.DataDir == "" {
		return paths.DBPath()
	}
	dir, err := filepath.Abs(cfg.DataDir)
	if err != nil {
		return cfg.DataDir, err
	}
	return filepath.Join(dir, paths.DBFileName), nil
}

// Title bar colours (UI-SPEC Color tokens): the Windows title bar uses --bg
// so it reads as one surface with the sidebar and the header bar, the title
// text uses --ink (--ink-muted while inactive) and the border uses --rule.
var (
	lightBar   = windows.RGB(0xF2, 0xF4, 0xF3)
	lightText  = windows.RGB(0x18, 0x21, 0x2B)
	lightMuted = windows.RGB(0x5A, 0x64, 0x72)
	lightRule  = windows.RGB(0xDA, 0xDF, 0xE3)
	darkBar    = windows.RGB(0x12, 0x17, 0x1D)
	darkText   = windows.RGB(0xE4, 0xE8, 0xEC)
	darkMuted  = windows.RGB(0x9A, 0xA5, 0xB1)
	darkRule   = windows.RGB(0x2C, 0x36, 0x41)
)

// startupBackground is the window colour shown before the page paints:
// --bg of the theme the system uses, so dark mode does not flash light.
func startupBackground(dark bool) *options.RGBA {
	if dark {
		return &options.RGBA{R: 0x12, G: 0x17, B: 0x1D, A: 255}
	}
	return &options.RGBA{R: 0xF2, G: 0xF4, B: 0xF3, A: 255}
}

// windowsOptions follows the system theme (the page's theme switch changes
// it at run time through SetWindowTheme) and paints the title bar in the
// app's own colours for both themes.
func windowsOptions() *windows.Options {
	return &windows.Options{
		Theme: windows.SystemDefault,
		CustomTheme: &windows.ThemeSettings{
			LightModeTitleBar:          lightBar,
			LightModeTitleBarInactive:  lightBar,
			LightModeTitleText:         lightText,
			LightModeTitleTextInactive: lightMuted,
			LightModeBorder:            lightRule,
			LightModeBorderInactive:    lightRule,
			DarkModeTitleBar:           darkBar,
			DarkModeTitleBarInactive:   darkBar,
			DarkModeTitleText:          darkText,
			DarkModeTitleTextInactive:  darkMuted,
			DarkModeBorder:             darkRule,
			DarkModeBorderInactive:     darkRule,
		},
	}
}
