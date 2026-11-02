//go:build windows

package cli

import (
	"os"
	"syscall"
)

// isTerminal reports whether f is an interactive console. NUL is a character
// device too, but only a console handle has a console mode.
func isTerminal(f *os.File) bool {
	var mode uint32
	return syscall.GetConsoleMode(syscall.Handle(f.Fd()), &mode) == nil
}
