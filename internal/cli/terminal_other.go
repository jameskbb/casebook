//go:build !windows

package cli

import "os"

// isTerminal reports whether f is an interactive terminal: a character device
// other than the null device (stdin redirected from /dev/null, as under cron,
// systemd or CI, is a character device too). It needs no dependency; other
// character devices as stdin are not a real case.
func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil || fi.Mode()&os.ModeCharDevice == 0 {
		return false
	}
	if null, err := os.Stat(os.DevNull); err == nil && os.SameFile(fi, null) {
		return false
	}
	return true
}
