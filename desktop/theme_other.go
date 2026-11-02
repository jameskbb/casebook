//go:build !windows

package main

// systemPrefersDark reports light outside Windows: macOS paints the window
// from its own appearance settings, and the page follows the system theme
// as soon as it loads.
func systemPrefersDark() bool { return false }
