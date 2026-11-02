// Command icongen writes the Casebook application icon (D-02): the SVG
// source appicon.svg, appicon.png at 1024 by 1024 (Wails derives the macOS
// icon from it) and windows/icon.ico with PNG entries at 16, 24, 32, 48,
// 64, 128 and 256 pixels. Output is byte-for-byte reproducible.
//
// Run from the repository root:
//
//	go run ./desktop/tools/icongen -out desktop/build
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

var windowsSizes = []int{16, 24, 32, 48, 64, 128, 256}

func main() {
	out := flag.String("out", "desktop/build", "directory that receives appicon.svg, appicon.png and windows/icon.ico")
	flag.Parse()
	if err := writeIcons(*out); err != nil {
		fmt.Fprintln(os.Stderr, "icongen:", err)
		os.Exit(1)
	}
}

// writeIcons renders every output into dir.
func writeIcons(dir string) error {
	pngData, err := encodePNG(renderPNG(1024))
	if err != nil {
		return err
	}
	ico, err := encodeICO(windowsSizes)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(dir, "windows"), 0o755); err != nil {
		return err
	}
	files := []struct {
		name string
		data []byte
	}{
		{"appicon.svg", []byte(renderSVG())},
		{"appicon.png", pngData},
		{filepath.Join("windows", "icon.ico"), ico},
	}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(dir, f.name), f.data, 0o644); err != nil {
			return err
		}
	}
	return nil
}
