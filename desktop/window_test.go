package main

import (
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

// tokenColour reads one hex colour token from the light or dark block of
// frontend/src/styles/tokens.css and returns it as a Windows COLORREF.
func tokenColour(t *testing.T, css, theme, token string) int32 {
	t.Helper()
	start := strings.Index(css, ":root[data-theme='"+theme+"']")
	if start < 0 {
		t.Fatalf("no %s block in tokens.css", theme)
	}
	block := css[start:]
	if end := strings.Index(block, "}"); end >= 0 {
		block = block[:end]
	}
	m := regexp.MustCompile(regexp.QuoteMeta(token) + `:\s*#([0-9a-fA-F]{6});`).FindStringSubmatch(block)
	if m == nil {
		t.Fatalf("%s not found in the %s tokens", token, theme)
	}
	v, err := strconv.ParseUint(m[1], 16, 32)
	if err != nil {
		t.Fatal(err)
	}
	return windows.RGB(uint8(v>>16), uint8(v>>8), uint8(v))
}

// TestTitleBarMatchesTokens keeps the native title bar colours in step with
// the page tokens, so the frame and the sidebar read as one surface.
func TestTitleBarMatchesTokens(t *testing.T) {
	raw, err := os.ReadFile("frontend/src/styles/tokens.css")
	if err != nil {
		t.Fatal(err)
	}
	css := string(raw)
	ct := windowsOptions().CustomTheme
	if windowsOptions().Theme != windows.SystemDefault || ct == nil {
		t.Fatal("window must follow the system theme with custom title bar colours")
	}
	checks := []struct {
		name  string
		got   int32
		theme string
		token string
	}{
		{"light bar", ct.LightModeTitleBar, "light", "--bg"},
		{"light bar inactive", ct.LightModeTitleBarInactive, "light", "--bg"},
		{"light text", ct.LightModeTitleText, "light", "--ink"},
		{"light text inactive", ct.LightModeTitleTextInactive, "light", "--ink-muted"},
		{"light border", ct.LightModeBorder, "light", "--rule"},
		{"dark bar", ct.DarkModeTitleBar, "dark", "--bg"},
		{"dark bar inactive", ct.DarkModeTitleBarInactive, "dark", "--bg"},
		{"dark text", ct.DarkModeTitleText, "dark", "--ink"},
		{"dark text inactive", ct.DarkModeTitleTextInactive, "dark", "--ink-muted"},
		{"dark border", ct.DarkModeBorder, "dark", "--rule"},
	}
	for _, c := range checks {
		if want := tokenColour(t, css, c.theme, c.token); c.got != want {
			t.Errorf("%s = %#06x, want %s %#06x", c.name, c.got, c.token, want)
		}
	}
}

// TestStartupBackgroundMatchesTokens keeps the colour shown before the page
// paints equal to --bg of the system theme, so dark mode has no light flash.
func TestStartupBackgroundMatchesTokens(t *testing.T) {
	raw, err := os.ReadFile("frontend/src/styles/tokens.css")
	if err != nil {
		t.Fatal(err)
	}
	css := string(raw)
	for _, c := range []struct {
		dark  bool
		theme string
	}{{false, "light"}, {true, "dark"}} {
		bg := startupBackground(c.dark)
		if got, want := windows.RGB(bg.R, bg.G, bg.B), tokenColour(t, css, c.theme, "--bg"); got != want || bg.A != 255 {
			t.Errorf("%s startup background = %#06x (alpha %d), want --bg %#06x", c.theme, got, bg.A, want)
		}
	}
	_ = systemPrefersDark() // must not panic on any OS
}
