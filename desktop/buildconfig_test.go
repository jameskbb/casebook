package main

import (
	"encoding/json"
	"os"
	"regexp"
	"strconv"
	"testing"
)

// TestMacOSMinimumMatchesGoRuntime: the Go runtime needs macOS 11, so the
// bundle must not claim older systems (AR-31); an older Mac then shows the
// system's clean "requires a newer macOS" message instead of crashing.
func TestMacOSMinimumMatchesGoRuntime(t *testing.T) {
	re := regexp.MustCompile(`<key>LSMinimumSystemVersion</key>\s*<string>(\d+)\.`)
	for _, p := range []string{"build/darwin/Info.plist", "build/darwin/Info.dev.plist"} {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		m := re.FindSubmatch(b)
		if m == nil {
			t.Fatalf("%s: no LSMinimumSystemVersion", p)
		}
		if major, _ := strconv.Atoi(string(m[1])); major < 11 {
			t.Errorf("%s: LSMinimumSystemVersion major %d, want at least 11", p, major)
		}
	}
}

// TestWailsInstallsFromLockfile: a local native build must not rewrite
// package-lock.json (AR-26), so the frontend installs with npm ci.
func TestWailsInstallsFromLockfile(t *testing.T) {
	b, err := os.ReadFile("wails.json")
	if err != nil {
		t.Fatal(err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(b, &cfg); err != nil {
		t.Fatal(err)
	}
	if got := cfg["frontend:install"]; got != "npm ci" {
		t.Fatalf("frontend:install = %v, want npm ci", got)
	}
}
